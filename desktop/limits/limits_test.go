package limits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/host"
)

// threads are the sessions with the live ones given.
type threads []sessions.Chat

func (t threads) Chats() []sessions.Chat { return t }

var oneLive = threads{{ID: "main/abcdefgh", Socket: "/tmp/h.sock", Live: true}}

// clock is a time the test moves.
type clock struct {
	mu sync.Mutex
	at time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

// asker answers each ask with its answer and error, and counts the asks.
type asker struct {
	mu      sync.Mutex
	asked   []string
	answer  string
	err     error
	onceErr error // the first ask's error, before err
}

func (a *asker) ask(_ context.Context, socket string) (json.RawMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.asked = append(a.asked, socket)
	if a.onceErr != nil {
		err := a.onceErr
		a.onceErr = nil
		return nil, err
	}
	return json.RawMessage(a.answer), a.err
}

func (a *asker) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.asked)
}

const weekly96 = `{"rate_limits":{"five_hour":null,"seven_day":{"utilization":96,"resets_at":"2026-11-30T22:59:59.804611+00:00"}}}`

// tracker is a Tracker on threads, ask and start, at a clock the test moves, that records what it emits.
func tracker(t *testing.T, th Threads, ask Asker, start Starter) (*Tracker, *clock, func() []Limits) {
	t.Helper()
	var mu sync.Mutex
	var emitted []Limits
	tr := New(th, ask, start, t.TempDir(), func(name string, payload any) {
		if name != "limits" {
			t.Errorf("emitted %q", name)
		}
		mu.Lock()
		defer mu.Unlock()
		emitted = append(emitted, payload.(Limits))
	})
	c := &clock{at: time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)}
	tr.now = c.now
	return tr, c, func() []Limits {
		mu.Lock()
		defer mu.Unlock()
		return append([]Limits(nil), emitted...)
	}
}

func TestParseUsage(t *testing.T) {
	for _, tt := range []struct {
		name   string
		answer string
		want   []Window
		err    bool
	}{
		{"the recorded answer", weekly96, []Window{{Name: "seven_day", Label: "Weekly", Left: 4, ResetsAt: 1796079599804}}, false},
		{"every window", `{"rate_limits":{
			"five_hour":{"utilization":25.5,"resets_at":"2026-11-26T15:00:00Z"},
			"seven_day":{"utilization":40,"resets_at":"2026-11-30T23:00:00Z"},
			"model_scoped":[{"display_name":"Fable","utilization":10,"resets_at":"2026-11-30T23:00:00Z"},{"display_name":"","utilization":5}],
			"extra_usage":{"is_enabled":true,"utilization":30}}}`, []Window{
			{Name: "five_hour", Label: "Session", Left: 74.5, ResetsAt: 1795705200000},
			{Name: "seven_day", Label: "Weekly", Left: 60, ResetsAt: 1796079600000},
			{Name: "model:Fable", Label: "Weekly · Fable", Left: 90, ResetsAt: 1796079600000},
			{Name: "spend", Label: "Monthly spend", Left: 70},
		}, false},
		{"spend that is off", `{"rate_limits":{"extra_usage":{"is_enabled":false,"utilization":null}}}`, nil, false},
		{"over the limit", `{"rate_limits":{"seven_day":{"utilization":104,"resets_at":"bad"}}}`, []Window{{Name: "seven_day", Label: "Weekly", Left: 0}}, false},
		{"no plan limits", `{"rate_limits_available":false}`, nil, false},
		{"not JSON", `{`, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUsage(json.RawMessage(tt.answer))
			if (err != nil) != tt.err {
				t.Fatalf("err = %v, want an error: %v", err, tt.err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("windows = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRateLimitEventUpdatesAWindow(t *testing.T) {
	session := Window{Name: "five_hour", Label: "Session", Left: 80, ResetsAt: 1795705200000}
	weekly := Window{Name: "seven_day", Label: "Weekly", Left: 60, ResetsAt: 1796079600000}
	for _, tt := range []struct {
		name string
		info string
		want []Window
	}{
		{"the recorded event", `{"status":"allowed_warning","resetsAt":1796079600,"rateLimitType":"seven_day","utilization":0.96,"unifiedWindows":{"seven_day":{"utilization":0.96,"resetsAt":1796079600,"status":"allowed_warning"}}}`,
			[]Window{session, {Name: "seven_day", Label: "Weekly", Left: 4, ResetsAt: 1796079600000}}},
		{"no unified windows", `{"rateLimitType":"five_hour","utilization":0.5,"resetsAt":1796140800}`,
			[]Window{{Name: "five_hour", Label: "Session", Left: 50, ResetsAt: 1796140800000}, weekly}},
		{"a window the modal does not show", `{"rateLimitType":"overage","utilization":0.5,"resetsAt":1796140800}`, nil},
		{"no utilization", `{"rateLimitType":"seven_day","resetsAt":1796140800}`, nil},
		{"not JSON", `{`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &asker{answer: `{"rate_limits":{"five_hour":{"utilization":20,"resets_at":"2026-11-26T15:00:00Z"},"seven_day":{"utilization":40,"resets_at":"2026-11-30T23:00:00Z"}}}`}
			tr, c, emitted := tracker(t, oneLive, a.ask, nil)
			if _, err := tr.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			read, before := tr.Limits(), len(emitted())
			c.add(time.Minute)
			tr.Seen("main/abcdefgh", chat.Event{Kind: chat.KindRateLimit, Input: json.RawMessage(tt.info)})
			got := tr.Limits()
			if tt.want == nil {
				if !reflect.DeepEqual(got, read) || len(emitted()) != before {
					t.Errorf("limits = %+v, want them as read, %+v, and no event", got, read)
				}
				return
			}
			if !reflect.DeepEqual(got.Windows, tt.want) {
				t.Errorf("windows = %+v, want %+v", got.Windows, tt.want)
			}
			if n := len(emitted()) - before; n != 1 || got.At != c.now().UnixMilli() {
				t.Errorf("emitted %d, at %d; want one limits event, at the event's time", n, got.At)
			}
		})
	}
}

func TestLimitsRefreshAtMostEveryFiveMinutes(t *testing.T) {
	a := &asker{answer: weekly96}
	tr, c, _ := tracker(t, oneLive, a.ask, nil)
	ctx := context.Background()
	steps := []struct {
		name  string
		after time.Duration
		owner bool
		asks  int
	}{
		{"the loop's first look", 0, false, 1},
		{"a minute on", time.Minute, false, 1},
		{"just under five minutes on", 4*time.Minute - time.Second, false, 1},
		{"five minutes on", time.Second, false, 2},
		{"the owner's ask while fresh", time.Second, true, 3},
		{"the loop after the owner's ask", time.Minute, false, 3},
	}
	for _, s := range steps {
		c.add(s.after)
		if err := tr.read(ctx, s.owner); err != nil {
			t.Fatal(err)
		}
		if got := a.count(); got != s.asks {
			t.Fatalf("%s: %d asks, want %d", s.name, got, s.asks)
		}
	}
	if a.asked[0] != "/tmp/h.sock" {
		t.Errorf("asked %q, want the live thread's host", a.asked[0])
	}
}

func TestAFailedAskWaitsFiveMinutesToo(t *testing.T) {
	for _, tt := range []struct {
		name   string
		answer string
		err    error
	}{
		{"an error", "", errors.New("refused")},
		{"no plan limits", `{"rate_limits_available":false}`, nil},
		{"no get_usage", "", host.ErrNoUsage},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &asker{answer: tt.answer, err: tt.err}
			tr, c, _ := tracker(t, oneLive, a.ask, nil)
			for range 5 {
				if err := tr.read(context.Background(), false); err != nil {
					t.Fatal(err)
				}
				c.add(time.Minute - time.Second)
			}
			if a.count() != 1 {
				t.Errorf("%d asks in under five minutes, want one", a.count())
			}
		})
	}
}

func TestRunAsksAtOnceAndStopsWithItsContext(t *testing.T) {
	a := &asker{answer: weekly96}
	tr, _, _ := tracker(t, oneLive, a.ask, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		tr.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for a.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop with its context")
	}
	if a.count() != 1 {
		t.Errorf("%d asks, want one at the start", a.count())
	}
}

func TestAReadThatFailsKeepsTheWindows(t *testing.T) {
	for _, tt := range []struct {
		name        string
		err         error
		answer      string
		unsupported bool
		errText     string
	}{
		{"Claude does not know get_usage", fmt.Errorf("host at /tmp/h.sock: %w", host.ErrNoUsage), "", true, ""},
		{"the host cannot be reached", errors.New("reach host at /tmp/h.sock: refused"), "", false, "reach host at /tmp/h.sock: refused"},
		{"an answer that is not JSON", nil, "{", false, "read Claude's usage answer: unexpected end of JSON input"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := &asker{answer: tt.answer, err: tt.err}
			tr, _, _ := tracker(t, oneLive, a.ask, nil)
			tr.Seen("main/abcdefgh", chat.Event{Kind: chat.KindRateLimit, Input: json.RawMessage(`{"rateLimitType":"seven_day","utilization":0.5,"resetsAt":1796079600}`)})
			got, err := tr.Refresh(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := []Window{{Name: "seven_day", Label: "Weekly", Left: 50, ResetsAt: 1796079600000}}
			if got.Unsupported != tt.unsupported || got.Error != tt.errText || !reflect.DeepEqual(got.Windows, want) {
				t.Errorf("limits = %+v, want unsupported %v, error %q and the stream's window", got, tt.unsupported, tt.errText)
			}
			if _, err := tr.Refresh(context.Background()); err != nil || a.count() != 2 {
				t.Errorf("%d asks, %v; want the next refresh to ask again", a.count(), err)
			}
		})
	}
}

func TestAGoodReadClearsAnError(t *testing.T) {
	a := &asker{answer: weekly96, onceErr: errors.New("refused")}
	tr, _, _ := tracker(t, oneLive, a.ask, nil)
	if l, _ := tr.Refresh(context.Background()); l.Error != "refused" {
		t.Fatalf("limits = %+v, want the error", l)
	}
	if l, _ := tr.Refresh(context.Background()); l.Error != "" || len(l.Windows) != 1 || l.Windows[0].Name != "seven_day" {
		t.Errorf("limits = %+v, want the windows and no error", l)
	}
}

func TestRefreshWaitsOnlyAsLongAsItsContext(t *testing.T) {
	tr, _, _ := tracker(t, threads{}, nil, nil)
	tr.reading <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := tr.Refresh(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want the context's", err)
	}
}

func TestLimitsAreNeverNil(t *testing.T) {
	tr, _, _ := tracker(t, threads{}, nil, nil)
	if got := tr.Limits(); got.Windows == nil || len(got.Windows) != 0 {
		t.Errorf("windows = %#v, want []", got.Windows)
	}
}
