// Package chat serves each thread's chat to the front end: its history from the chat log, and its live events and
// waiting approvals from its host.
package chat

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/host"
)

const (
	reconcileEvery = time.Second
	batchEvery     = 33 * time.Millisecond // about one frame at 30 per second
	retryFirst     = 100 * time.Millisecond
	retryMost      = 2 * time.Second
)

// Threads are the sessions whose agents run under a host.
type Threads interface {
	ChatOf(id string) (sessions.Chat, error)
	Chats() []sessions.Chat
	TurnEnded(id string, e chatlog.Event)
}

// Pending is the approval a thread waits on: the oldest one Claude asked for that has no answer yet.
type Pending struct {
	ID      string `json:"id"`      // the thread's
	Request string `json:"request"` // the approval's, to answer it by
	Tool    string `json:"tool"`
	Summary string `json:"summary"` // what the tool would do, in one line
	Kind    string `json:"kind"`    // "tool", "question" or "plan"
}

// Update is the payload of a chat:<id> event: the thread's events since the last one, deltas among them.
type Update struct {
	ID     string          `json:"id"`
	Events []chatlog.Event `json:"events"`
}

// PendingUpdate is the payload of a chat:pending event; Pending is nil once nothing waits.
type PendingUpdate struct {
	ID      string   `json:"id"`
	Pending *Pending `json:"pending"`
}

// Chats follows the host of each live thread, one subscription per host: it hands each turn's end to the sessions,
// tells the front end which approval each thread waits on and sends the events of the threads it watches.
type Chats struct {
	threads Threads
	emit    func(string, any)

	mu        sync.Mutex
	ctx       context.Context // Run's, nil until it runs
	followers map[string]*follower
	watched   map[string]bool
}

func New(t Threads, emit func(string, any)) *Chats {
	return &Chats{threads: t, emit: emit, followers: map[string]*follower{}, watched: map[string]bool{}}
}

// Run follows the live threads' hosts until ctx ends.
func (c *Chats) Run(ctx context.Context) {
	c.mu.Lock()
	c.ctx = ctx
	c.mu.Unlock()
	tick := time.NewTicker(reconcileEvery)
	defer tick.Stop()
	for {
		c.reconcile()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// reconcile follows each live thread that has no follower and stops the follower of each thread that is gone.
func (c *Chats) reconcile() {
	live := map[string]sessions.Chat{}
	for _, ch := range c.threads.Chats() {
		live[ch.ID] = ch
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx == nil || c.ctx.Err() != nil {
		return
	}
	for id, f := range c.followers {
		if _, ok := live[id]; !ok {
			f.stop()
			delete(c.followers, id)
			delete(c.watched, id)
		}
	}
	for id, ch := range live {
		if c.followers[id] != nil {
			continue
		}
		ctx, stop := context.WithCancel(c.ctx)
		f := &follower{chats: c, chat: ch, stop: stop, read: make(chan struct{})}
		c.followers[id] = f
		go f.run(ctx)
	}
}

// watch sends the thread's events from where its follower read the log to. It returns once the follower has read it,
// so a history read after it misses nothing.
func (c *Chats) watch(id string) {
	c.mu.Lock()
	c.watched[id] = true
	c.mu.Unlock()
	c.reconcile()
	c.mu.Lock()
	f, ctx := c.followers[id], c.ctx
	c.mu.Unlock()
	if f == nil {
		return
	}
	select {
	case <-f.read:
	case <-ctx.Done():
	}
}

func (c *Chats) unwatch(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.watched, id)
}

func (c *Chats) isWatched(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.watched[id]
}

// pending is what each followed thread waits on.
func (c *Chats) pending() []Pending {
	c.mu.Lock()
	followers := make([]*follower, 0, len(c.followers))
	for _, f := range c.followers {
		followers = append(followers, f)
	}
	c.mu.Unlock()
	out := []Pending{}
	for _, f := range followers {
		if p := f.waiting(); p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// follower follows one thread's host.
type follower struct {
	chats *Chats
	chat  sessions.Chat
	stop  context.CancelFunc
	read  chan struct{} // closed once the follower has read the log

	mu        sync.Mutex
	approvals []chatlog.Event // waiting, oldest first
	batch     []chatlog.Event // for the next chat:<id> event
	flushing  bool            // a flush of batch is due
	sent      time.Time       // when the last chat:<id> event went out
}

// run reads the log once, for the approvals that wait and where it ends, then follows the host's stream from there,
// subscribing again after the last event it had when the stream ends.
func (f *follower) run(ctx context.Context) {
	logged, err := chatlog.ReadLog(f.chat.Log, 0)
	if err != nil {
		log.Printf("agentos: reading the chat log of %s: %v", f.chat.ID, err)
	}
	var last int64
	var turnEnd *chatlog.Event
	for _, e := range logged {
		last = e.Seq
		f.track(e)
		if e.Kind == chatlog.KindTurnEnd {
			turnEnd = &e
		}
	}
	close(f.read)
	// A turn interrupted while the app was closed ends now; the earlier turn_ends would change nothing after it.
	if turnEnd != nil {
		f.chats.threads.TurnEnded(f.chat.ID, *turnEnd)
	}
	f.announce(nil)
	defer f.announceGone()

	wait := retryFirst
	for {
		got := false
		_ = host.Subscribe(ctx, f.chat.Socket, last, func(e chatlog.Event) {
			got = true
			if e.Persisted() {
				if e.Seq <= last {
					return
				}
				last = e.Seq
			}
			f.take(e)
		})
		if got {
			wait = retryFirst
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, retryMost)
	}
}

// take handles one event from the host's stream.
func (f *follower) take(e chatlog.Event) {
	before := f.waiting()
	f.track(e)
	f.announce(before)
	if e.Kind == chatlog.KindTurnEnd {
		f.chats.threads.TurnEnded(f.chat.ID, e)
	}
	if f.chats.isWatched(f.chat.ID) {
		f.add(e)
	}
}

// track keeps the approvals that wait: an answer takes one, and a turn's end or Claude's exit takes them all.
func (f *follower) track(e chatlog.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch e.Kind {
	case chatlog.KindApproval:
		f.approvals = append(f.approvals, e)
	case chatlog.KindAnswer:
		for i, a := range f.approvals {
			if a.ID == e.ID {
				f.approvals = append(f.approvals[:i], f.approvals[i+1:]...)
				break
			}
		}
	case chatlog.KindTurnEnd, chatlog.KindExit:
		f.approvals = nil
	}
}

func (f *follower) waiting() *Pending {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.approvals) == 0 {
		return nil
	}
	p := pendingOf(f.chat.ID, f.approvals[0])
	return &p
}

// announce sends chat:pending when what the thread waits on is no longer before.
func (f *follower) announce(before *Pending) {
	now := f.waiting()
	if now == before || (now != nil && before != nil && *now == *before) {
		return
	}
	f.chats.emit("chat:pending", PendingUpdate{ID: f.chat.ID, Pending: now})
}

// announceGone says the thread waits on nothing once it is no longer followed.
func (f *follower) announceGone() {
	before := f.waiting()
	f.mu.Lock()
	f.approvals = nil
	f.mu.Unlock()
	f.announce(before)
}

// add puts e in the next chat:<id> event, which goes out at most once per batchEvery.
func (f *follower) add(e chatlog.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batch = append(f.batch, e)
	if f.flushing {
		return
	}
	f.flushing = true
	time.AfterFunc(max(0, batchEvery-time.Since(f.sent)), f.flush)
}

// flush sends the batch, under mu so that two batches never go out of order. A thread no longer watched gets none.
func (f *follower) flush() {
	f.mu.Lock()
	defer f.mu.Unlock()
	events := f.batch
	f.batch, f.flushing, f.sent = nil, false, time.Now()
	if len(events) == 0 || !f.chats.isWatched(f.chat.ID) {
		return
	}
	f.chats.emit("chat:"+f.chat.ID, Update{ID: f.chat.ID, Events: events})
}

// pendingOf describes the approval e of the thread id.
func pendingOf(id string, e chatlog.Event) Pending {
	p := Pending{ID: id, Request: e.ID, Tool: e.Tool, Kind: "tool"}
	var input struct {
		Command   string `json:"command"`
		FilePath  string `json:"file_path"`
		Path      string `json:"path"`
		Pattern   string `json:"pattern"`
		URL       string `json:"url"`
		Plan      string `json:"plan"`
		Questions []struct {
			Question string `json:"question"`
		} `json:"questions"`
	}
	_ = json.Unmarshal(e.Input, &input)
	switch e.Tool {
	case "AskUserQuestion":
		p.Kind = "question"
		if len(input.Questions) > 0 {
			p.Summary = input.Questions[0].Question
		}
	case "ExitPlanMode":
		p.Kind = "plan"
		p.Summary = firstLine(input.Plan)
	default:
		for _, s := range []string{input.Command, input.FilePath, input.Path, input.Pattern, input.URL} {
			if s != "" {
				p.Summary = firstLine(s)
				break
			}
		}
	}
	return p
}

func firstLine(s string) string {
	for line := range strings.Lines(s) {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
