// Package limits reads the owner's Claude plan limits: Claude's answer to get_usage, through a live thread's host or a
// short probe when no thread runs, and the rate_limit events a turn streams.
package limits

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/host"
)

const (
	// every is how often a live thread's host is asked without the owner asking, and how long a good answer stands
	// before the owner's ask with no live thread starts a probe.
	every = 5 * time.Minute
	// checkEvery is how often the loop looks whether the last ask is older than every.
	checkEvery = time.Minute
)

// Window is one plan limit: Name is five_hour, seven_day, model:<name> or spend, Left the percent of it left, from 0
// to 100, and ResetsAt when it starts again, in unix milliseconds, 0 when Claude does not say.
type Window struct {
	Name     string  `json:"name"`
	Label    string  `json:"label"`
	Left     float64 `json:"left"`
	ResetsAt int64   `json:"resetsAt"`
}

// Limits are the plan limits as last read. At is when a value last came, in unix milliseconds, 0 before any.
// Unsupported says that Claude Code does not answer get_usage, so the windows are only what turns told of; Error is
// why the latest read failed, "" when it did not.
type Limits struct {
	Windows     []Window `json:"windows"`
	At          int64    `json:"at"`
	Unsupported bool     `json:"unsupported"`
	Error       string   `json:"error"`
}

// Threads are the sessions; a live one's host is asked for the usage.
type Threads interface {
	Chats() []sessions.Chat
}

// Asker asks the host at socket for Claude's get_usage answer, as host.Usage does.
type Asker func(ctx context.Context, socket string) (json.RawMessage, error)

// Tracker keeps the latest plan limits and tells the front end of each change as the limits event.
type Tracker struct {
	threads Threads
	ask     Asker
	start   Starter
	dir     string // the folder every probe runs in
	emit    func(string, any)
	now     func() time.Time

	// reading holds one token per read in flight: one read, and so one probe, at a time.
	reading chan struct{}
	// emitting is taken before mu and held through the emit, so the limits events go out in the order their values
	// were taken.
	emitting sync.Mutex

	mu      sync.Mutex
	limits  Limits
	windows map[string]Window
	fetched time.Time // when the latest good get_usage answer came
	asked   time.Time // when a live thread's host was last asked
}

// New tracks the limits: it asks a live thread's host with ask, and with no live thread runs a probe with start in
// dir, which it empties before each probe.
func New(threads Threads, ask Asker, start Starter, dir string, emit func(string, any)) *Tracker {
	return &Tracker{threads: threads, ask: ask, start: start, dir: dir, emit: emit, now: time.Now, reading: make(chan struct{}, 1), windows: map[string]Window{}}
}

// Run asks a live thread's host for the usage at once and then whenever its last ask is 5 minutes old, until ctx ends.
// It starts no probe: with no thread running nothing is on screen to keep fresh.
func (t *Tracker) Run(ctx context.Context) {
	tick := time.NewTicker(checkEvery)
	defer tick.Stop()
	for {
		t.read(ctx, false)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Refresh is the owner's ask: a live thread's host is asked at once; with none running, a probe runs unless the values
// are under 5 minutes old. The error is only ctx's, should it end before the read could start.
func (t *Tracker) Refresh(ctx context.Context) (Limits, error) {
	if err := t.read(ctx, true); err != nil {
		return t.Limits(), fmt.Errorf("read the plan limits: %w", err)
	}
	return t.Limits(), nil
}

// read asks for the usage, one read at a time. Without the owner's ask a live thread's host is asked at most every 5
// minutes, whatever it answered, and no probe runs. A host from before the usage op is passed over for the next live
// thread's. The owner's ask with no live thread, or none whose host knows the op, probes only once the latest good
// answer is 5 minutes old.
func (t *Tracker) read(ctx context.Context, owner bool) error {
	select {
	case t.reading <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-t.reading }()
	if sockets := t.liveSockets(); len(sockets) > 0 {
		if !owner && t.within(&t.asked) {
			return nil
		}
		t.mu.Lock()
		t.asked = t.now()
		t.mu.Unlock()
		for _, socket := range sockets {
			raw, err := t.ask(ctx, socket)
			if errors.Is(err, host.ErrHostTooOld) {
				continue
			}
			t.take(raw, err)
			return nil
		}
	}
	if !owner || t.within(&t.fetched) {
		return nil
	}
	raw, err := probe(ctx, t.start, t.dir)
	t.take(raw, err)
	return nil
}

// liveSockets are the host sockets of the live threads.
func (t *Tracker) liveSockets() []string {
	var out []string
	for _, c := range t.threads.Chats() {
		if c.Live && !c.Deleting && c.Socket != "" {
			out = append(out, c.Socket)
		}
	}
	return out
}

// within says whether the time at is set and under 5 minutes ago.
func (t *Tracker) within(at *time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !at.IsZero() && t.now().Sub(*at) < every
}

// take keeps what a read gave. An answer with windows replaces the windows held and counts as fresh. An answer with
// none, as when Claude gives every window as null, keeps the windows turns told of and is not fresh; nor are a failure
// and a Claude that does not know get_usage, which keep them too. What is not fresh is read again at the next ask.
func (t *Tracker) take(raw json.RawMessage, err error) {
	var windows []Window
	if err == nil {
		windows, err = parseUsage(raw)
	}
	t.change(func(now time.Time) bool {
		switch {
		case errors.Is(err, host.ErrNoUsage):
			t.limits.Unsupported, t.limits.Error = true, ""
		case err != nil:
			t.limits.Error = err.Error()
		default:
			t.limits.Unsupported, t.limits.Error = false, ""
			if len(windows) == 0 {
				break
			}
			clear(t.windows)
			for _, w := range windows {
				t.windows[w.Name] = w
			}
			t.limits.At, t.fetched = now.UnixMilli(), now
		}
		return true
	})
}

// change runs fn under mu with the time, then sends the limits event when fn says they changed. The events go out in
// the order the changes were made.
func (t *Tracker) change(fn func(now time.Time) bool) {
	t.emitting.Lock()
	defer t.emitting.Unlock()
	t.mu.Lock()
	changed := fn(t.now())
	l := t.limitsLocked()
	t.mu.Unlock()
	if changed {
		t.emit("limits", l)
	}
}

// Seen takes a rate_limit event a thread's turn streamed and updates the windows it names.
func (t *Tracker) Seen(_ string, e chat.Event) {
	windows := parseRateLimit(e.Input)
	if len(windows) == 0 {
		return
	}
	t.change(func(now time.Time) bool {
		for _, w := range windows {
			t.windows[w.Name] = w
		}
		t.limits.At = now.UnixMilli()
		return true
	})
}

// Limits are the plan limits as last read.
func (t *Tracker) Limits() Limits {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.limitsLocked()
}

// limitsLocked needs mu. The windows come session, weekly, each model's weekly by name, then spend.
func (t *Tracker) limitsLocked() Limits {
	l := t.limits
	l.Windows = make([]Window, 0, len(t.windows))
	for _, w := range t.windows {
		l.Windows = append(l.Windows, w)
	}
	slices.SortFunc(l.Windows, func(a, b Window) int {
		return cmp.Or(cmp.Compare(rank(a.Name), rank(b.Name)), strings.Compare(a.Name, b.Name))
	})
	return l
}

func rank(name string) int {
	switch {
	case name == "five_hour":
		return 0
	case name == "seven_day":
		return 1
	case strings.HasPrefix(name, "model:"):
		return 2
	}
	return 3
}

// usageWindow is a window of get_usage's answer: utilization from 0 to 100, and its reset as an RFC 3339 time.
type usageWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

// parseUsage reads Claude's get_usage answer into its windows. A window Claude gives as null is not in use.
func parseUsage(raw json.RawMessage) ([]Window, error) {
	var a struct {
		RateLimits struct {
			FiveHour    *usageWindow `json:"five_hour"`
			SevenDay    *usageWindow `json:"seven_day"`
			ModelScoped []struct {
				DisplayName string `json:"display_name"`
				usageWindow
			} `json:"model_scoped"`
			ExtraUsage *struct {
				IsEnabled   bool     `json:"is_enabled"`
				Utilization *float64 `json:"utilization"`
			} `json:"extra_usage"`
		} `json:"rate_limits"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("read Claude's usage answer: %w", err)
	}
	r := a.RateLimits
	var out []Window
	add := func(name, label string, w *usageWindow) {
		if w == nil || w.Utilization == nil {
			return
		}
		out = append(out, Window{Name: name, Label: label, Left: left(*w.Utilization), ResetsAt: resetTime(w.ResetsAt)})
	}
	add("five_hour", "Session", r.FiveHour)
	add("seven_day", "Weekly", r.SevenDay)
	for _, m := range r.ModelScoped {
		if m.DisplayName != "" {
			add("model:"+m.DisplayName, "Weekly · "+m.DisplayName, &m.usageWindow)
		}
	}
	if x := r.ExtraUsage; x != nil && x.IsEnabled && x.Utilization != nil {
		out = append(out, Window{Name: "spend", Label: "Monthly spend", Left: left(*x.Utilization)})
	}
	return out, nil
}

// rateWindow is a window of a rate_limit event: utilization from 0 to 1, and its reset in unix seconds.
type rateWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    int64    `json:"resetsAt"`
}

// labels are the windows a rate_limit event names that the modal shows, by name.
var labels = map[string]string{"five_hour": "Session", "seven_day": "Weekly"}

// parseRateLimit reads a rate_limit event's rate_limit_info: each window of its unifiedWindows, else the one it names.
func parseRateLimit(raw json.RawMessage) []Window {
	var info struct {
		RateLimitType  string                `json:"rateLimitType"`
		UnifiedWindows map[string]rateWindow `json:"unifiedWindows"`
		rateWindow
	}
	if json.Unmarshal(raw, &info) != nil {
		return nil
	}
	windows := info.UnifiedWindows
	if len(windows) == 0 {
		windows = map[string]rateWindow{info.RateLimitType: info.rateWindow}
	}
	var out []Window
	for name, w := range windows {
		label, ok := labels[name]
		if !ok || w.Utilization == nil {
			continue
		}
		out = append(out, Window{Name: name, Label: label, Left: left(*w.Utilization * 100), ResetsAt: w.ResetsAt * 1000})
	}
	return out
}

// left is the percent left of a window used to utilization percent, to a hundredth.
func left(utilization float64) float64 {
	return min(100, max(0, math.Round((100-utilization)*100)/100))
}

// resetTime is an RFC 3339 time in unix milliseconds, 0 when it is not one.
func resetTime(s string) int64 {
	at, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return at.UnixMilli()
}
