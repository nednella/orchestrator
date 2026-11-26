// Package chat serves each thread's chat to the front end: its history from the chat log, and its live events and
// waiting approvals from its host.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"slices"
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
	startWait      = 3 * time.Second  // for a watch that comes before Start
	hostStartWait  = 10 * time.Second // for the host of a thread that is starting, before its log is read
)

// ErrThreadGone is a thread's follower stopping while a turn is awaited: the thread settled or was deleted, its host
// died, or the app stops.
var ErrThreadGone = errors.New("the thread no longer runs")

// Threads are the sessions whose agents run under a host. Wake starts a settled one again under its id, and
// AccessChanged follows the permission mode its Claude reports, saying whether the owner approved its plan just before.
// FlagRefused says that a thread's Claude did not know flag, so its host started it without. FilesRewind says where a
// rewind may put a thread's files back, as Sessions.FilesRewind does. BackgroundChanged says what a thread's Claude
// does beside its turns.
type Threads interface {
	ChatOf(id string) (sessions.Chat, error)
	Chats() []sessions.Chat
	TurnEnded(id string, e chatlog.Event)
	BackgroundChanged(id string, b sessions.Background)
	AccessChanged(id, mode string, planApproved bool)
	FlagRefused(flag string)
	NextTurn(id string)
	Wake(id string) (sessions.Chat, error)
	FilesRewind(id string) (dir string, inProject bool, reason string)
}

// Pending is the approval a thread waits on: the oldest one Claude asked for that has no answer yet.
type Pending struct {
	ID      string `json:"id"`      // the thread's
	Request string `json:"request"` // the approval's, to answer it by
	Tool    string `json:"tool"`
	Summary string `json:"summary"` // what the tool would do, in one line
	Kind    string `json:"kind"`    // "tool", "question" or "plan"
	// Input is the tool's input, for a card that answers the approval away from its chat; none when it is over
	// maxPendingInput, as a whole file an Edit or a Write carries can be, and InputTooLarge says so.
	Input         json.RawMessage `json:"input"`
	InputTooLarge bool            `json:"inputTooLarge"`
}

// maxPendingInput is the most input a Pending carries: each change of what a thread waits on sends it to the front end.
const maxPendingInput = 64 << 10

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
// tells the front end which approval each thread waits on, sends the events of the threads it watches and keeps each
// project's latest command list.
type Chats struct {
	threads  Threads
	emit     func(string, any)
	commands *commandLists
	// turnEnded hears of each turn_end a follower takes, after the threads do, and of each exit of a thread that
	// settled; OnTurnEnd sets it before Start.
	turnEnded func(id string, e chatlog.Event)
	// since is when this run of the app began, in unix milliseconds: a flag Claude refused before it was an earlier
	// run's, which may have run another claude.
	since int64

	started chan struct{} // closed by Start

	mu        sync.Mutex
	ctx       context.Context // Start's, nil until it starts
	followers map[string]*follower
	watched   map[string]bool
}

// New follows the threads t; it keeps each project's command list under dir, in the project's own folder.
func New(t Threads, dir string, emit func(string, any)) *Chats {
	return &Chats{threads: t, emit: emit, commands: newCommandLists(dir), since: time.Now().UnixMilli(), started: make(chan struct{}), followers: map[string]*follower{}, watched: map[string]bool{}, turnEnded: func(string, chatlog.Event) {}}
}

// OnTurnEnd has fn hear of each turn_end a follower takes, after Threads.TurnEnded and before an AwaitTurn does: the
// latest one of the log when the follower starts, and each one the host streams after it. fn also hears of the exit
// the host streams when the thread settles, which may end a turn with no turn_end. fn runs on the follower, so it must
// not block. It comes before Start.
func (c *Chats) OnTurnEnd(fn func(id string, e chatlog.Event)) { c.turnEnded = fn }

// Start follows the live threads' hosts until ctx ends. Each live thread has its follower when Start returns.
func (c *Chats) Start(ctx context.Context) {
	c.mu.Lock()
	c.ctx = ctx
	c.mu.Unlock()
	close(c.started)
	c.reconcile()
	go func() {
		tick := time.NewTicker(reconcileEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				c.reconcile()
			}
		}
	}()
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
		if c.followers[id] == nil {
			c.follow(ch)
		}
	}
}

// follow starts the thread's follower. It needs mu, and Start to have run.
func (c *Chats) follow(ch sessions.Chat) *follower {
	ctx, stop := context.WithCancel(c.ctx)
	f := &follower{chats: c, chat: ch, stop: stop, read: make(chan struct{})}
	c.followers[ch.ID] = f
	go f.run(ctx)
	return f
}

// watch sends the live thread's events from where its follower read the log to. It returns once the follower has
// read it, starting the follower if it has none, so a history read after it misses nothing.
func (c *Chats) watch(ch sessions.Chat) error {
	select {
	case <-c.started:
	case <-time.After(startWait):
		return errors.New("The chat service has not started")
	}
	c.mu.Lock()
	ctx := c.ctx
	if ctx.Err() != nil {
		c.mu.Unlock()
		return errors.New("The chat service has stopped")
	}
	c.watched[ch.ID] = true
	f := c.followers[ch.ID]
	if f == nil {
		f = c.follow(ch)
	}
	c.mu.Unlock()
	select {
	case <-f.read:
	case <-ctx.Done():
	}
	return nil
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

// AwaitTurn is the thread's next turn_end after the seq after, as its follower takes it, or the latest one when the
// follower has taken one after it already. It is ErrThreadGone once the thread's Claude exits after after or its follower
// stops, and ctx.Err() when ctx ends first. A live thread with no follower gets one.
func (c *Chats) AwaitTurn(ctx context.Context, id string, after int64) (chatlog.Event, error) {
	f, err := c.followerOf(ctx, id)
	if err != nil {
		return chatlog.Event{}, err
	}
	a := &await{after: after, got: make(chan chatlog.Event, 1)}
	f.await(a)
	defer f.forget(a)
	select {
	case e := <-a.got:
		if e.Kind != chatlog.KindTurnEnd {
			return chatlog.Event{}, ErrThreadGone
		}
		return e, nil
	case <-ctx.Done():
		return chatlog.Event{}, ctx.Err()
	}
}

// TurnOver says whether the thread's follower has taken a turn_end, Claude's exit or a run's start after the seq after,
// or has stopped. known is false while the thread has no follower that has read its log, which then holds the answer.
func (c *Chats) TurnOver(id string, after int64) (over, known bool) {
	c.mu.Lock()
	f := c.followers[id]
	c.mu.Unlock()
	if f == nil || !f.hasRead() {
		return false, false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ended || f.turnEnd.Seq > after || f.exit.Seq > after || f.start > after, true
}

// followerOf is the thread's follower, started if the thread is live and has none.
func (c *Chats) followerOf(ctx context.Context, id string) (*follower, error) {
	select {
	case <-c.started:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(startWait):
		return nil, errors.New("The chat service has not started")
	}
	c.mu.Lock()
	f := c.followers[id]
	c.mu.Unlock()
	if f != nil {
		return f, nil
	}
	// ChatOf takes the sessions' lock, which reconcile never holds with mu, so it is read before mu.
	ch, err := c.threads.ChatOf(id)
	if err != nil || !ch.Live {
		return nil, ErrThreadGone
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx.Err() != nil {
		return nil, ErrThreadGone
	}
	if f := c.followers[id]; f != nil {
		return f, nil
	}
	return c.follow(ch), nil
}

// pending is what each live thread waits on: as its follower knows it, or from the log until the follower has read it.
func (c *Chats) pending() []Pending {
	out := []Pending{}
	for _, ch := range c.threads.Chats() {
		c.mu.Lock()
		f := c.followers[ch.ID]
		c.mu.Unlock()
		var p *Pending
		if f != nil && f.hasRead() {
			p = f.waiting()
		} else {
			p = waitingIn(ch)
		}
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// waitingIn reads what the thread waits on from its log.
func waitingIn(ch sessions.Chat) *Pending {
	logged, err := chatlog.ReadLog(ch.Log, 0)
	if err != nil {
		log.Printf("agentos: reading the chat log of %s: %v", ch.ID, err)
		return nil
	}
	var approvals []chatlog.Event
	var tasks chatlog.Tasks
	for _, e := range logged {
		approvals = chatlog.Waiting(approvals, e, &tasks)
		tasks.See(e)
	}
	if len(approvals) == 0 {
		return nil
	}
	p := pendingOf(ch.ID, approvals[0])
	return &p
}

// follower follows one thread's host.
type follower struct {
	chats *Chats
	chat  sessions.Chat
	stop  context.CancelFunc
	read  chan struct{} // closed once the follower has read the log

	mu        sync.Mutex
	approvals []chatlog.Event // waiting, oldest first
	// planApproved is set from the owner allowing an ExitPlanMode until the mode Claude then reports, or the turn's end.
	planApproved bool
	tasks        chatlog.Tasks
	// reporting is set from the end of a task of Claude's own until the end of the turn in which Claude tells of it.
	reporting bool
	batch     []chatlog.Event // for the next chat:<id> event
	flushing  bool            // a flush of batch is due
	sent      time.Time       // when the last chat:<id> event went out
	// turnEnd and exit are the latest of each the follower has taken, and ended says its run has returned: an await
	// that comes after one of them is answered from them. start is the seq of the latest run's start.
	turnEnd, exit chatlog.Event
	start         int64
	ended         bool
	awaits        []*await
}

// await is one AwaitTurn waiting for a turn_end or an exit after the seq after. got holds one event, so the follower
// never blocks on it.
type await struct {
	after int64
	got   chan chatlog.Event
}

// run reads the log once, for the approvals that wait and where it ends, then follows the host's stream from there,
// subscribing again after the last event it had when the stream ends. It reads the log once the host listens, or has
// had hostStartWait to: the host logs its run's start before it listens, so the log then holds the current run's start.
func (f *follower) run(ctx context.Context) {
	defer f.end()
	ready, cancel := context.WithTimeout(ctx, hostStartWait)
	if err := host.Ready(ready, f.chat.Socket); err != nil {
		log.Printf("agentos: the host of thread %s does not listen; reading its chat log: %v", f.chat.ID, err)
	}
	cancel()
	logged, err := chatlog.ReadLog(f.chat.Log, 0)
	if err != nil {
		log.Printf("agentos: reading the chat log of %s: %v", f.chat.ID, err)
	}
	var last int64
	var turnEnd, exit, commands *chatlog.Event
	// mode is the latest mode Claude reported after the last run start: an earlier run's mode, also one whose host was
	// killed before it logged its exit, is not this run's, and a log with no start has no run to follow.
	var mode struct {
		started      bool
		text         string
		planApproved bool
	}
	for _, e := range logged {
		last = e.Seq
		text, approved := f.see(e)
		switch e.Kind {
		case chatlog.KindTurnEnd:
			turnEnd = &e
		case chatlog.KindExit:
			exit = &e
		case chatlog.KindCommands:
			commands = &e
		case chatlog.KindStart:
			// An exit before the run's start was an earlier run's.
			exit = nil
			mode.started, mode.text = true, ""
			if e.Text != "" && e.At >= f.chats.since {
				f.chats.threads.FlagRefused(e.Text)
			}
		case chatlog.KindMode:
			if mode.started {
				mode.text, mode.planApproved = text, approved
			}
		}
	}
	// TurnOver answers from these once the log is read.
	f.mu.Lock()
	if turnEnd != nil {
		f.turnEnd = *turnEnd
	}
	if exit != nil {
		f.exit = *exit
	}
	f.mu.Unlock()
	close(f.read)
	f.chats.threads.BackgroundChanged(f.chat.ID, f.backgroundNow())
	// A follower starts for a thread that runs: at the app's start, or once it woke.
	f.chats.threads.NextTurn(f.chat.ID)
	if commands != nil {
		f.chats.commands.keep(f.chat.Project, *commands)
	}
	// A turn interrupted while the app was closed ends now; the earlier turn_ends would change nothing after it.
	if turnEnd != nil {
		f.chats.threads.TurnEnded(f.chat.ID, *turnEnd)
		f.chats.turnEnded(f.chat.ID, *turnEnd)
		f.reached(*turnEnd)
	}
	if exit != nil {
		// A thread put to sleep as its follower started has that exit in the log already.
		if exit.Stop == chatlog.StopSettle {
			f.chats.turnEnded(f.chat.ID, *exit)
		}
		f.reached(*exit)
	}
	// A mode Claude reported while the app was closed is followed now, by the same rule as a live one.
	if mode.text != "" {
		f.chats.threads.AccessChanged(f.chat.ID, mode.text, mode.planApproved)
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
	before, background := f.waiting(), f.backgroundNow()
	mode, approved := f.see(e)
	f.announce(before)
	if now := f.backgroundNow(); now != background {
		now.At = time.UnixMilli(e.At)
		f.chats.threads.BackgroundChanged(f.chat.ID, now)
	}
	switch e.Kind {
	case chatlog.KindTurnEnd:
		// The sessions, and then OnTurnEnd's fn, hear of the turn's end before an await does, so they see a wait on it
		// still in flight.
		f.chats.threads.TurnEnded(f.chat.ID, e)
		f.chats.turnEnded(f.chat.ID, e)
		f.reached(e)
	case chatlog.KindExit:
		if e.Stop == chatlog.StopSettle {
			f.chats.turnEnded(f.chat.ID, e)
		}
		f.reached(e)
	case chatlog.KindMode:
		f.chats.threads.AccessChanged(f.chat.ID, mode, approved)
	case chatlog.KindCommands:
		f.chats.commands.keep(f.chat.Project, e)
	case chatlog.KindStart:
		if e.Text != "" {
			f.chats.threads.FlagRefused(e.Text)
		}
		f.chats.threads.NextTurn(f.chat.ID)
	case chatlog.KindUser:
		f.chats.threads.NextTurn(f.chat.ID)
	}
	if f.chats.isWatched(f.chat.ID) {
		f.add(e)
	}
}

// see keeps the approvals that wait, as chatlog.Waiting says, and the tasks Claude runs in the background. For a mode
// event it returns the mode and whether the owner allowed an ExitPlanMode since the mode before it, in the same turn.
func (f *follower) see(e chatlog.Event) (mode string, planApproved bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e.Kind == chatlog.KindAnswer && e.Text == "allow" && slices.ContainsFunc(f.approvals, func(a chatlog.Event) bool { return a.ID == e.ID && a.Tool == "ExitPlanMode" }) {
		f.planApproved = true
	}
	f.approvals = chatlog.Waiting(f.approvals, e, &f.tasks)
	if task, ok := f.tasks.See(e); ok && task.ReportedAlone(e) {
		f.reporting = true
	}
	switch e.Kind {
	case chatlog.KindMode:
		mode, planApproved = e.Text, f.planApproved
		f.planApproved = false
	case chatlog.KindStart:
		f.start = e.Seq
		f.planApproved, f.reporting = false, false
	case chatlog.KindTurnEnd, chatlog.KindExit:
		f.planApproved, f.reporting = false, false
	}
	return mode, planApproved
}

// backgroundNow is what the thread's Claude does beside its turns, with no time.
func (f *follower) backgroundNow() sessions.Background {
	f.mu.Lock()
	defer f.mu.Unlock()
	return sessions.Background{Subagents: f.tasks.Working(), Reporting: f.reporting, Asks: len(f.approvals) > 0}
}

func (f *follower) hasRead() bool {
	select {
	case <-f.read:
		return true
	default:
		return false
	}
}

// reached answers each await before e, a turn_end or an exit, and keeps e for the awaits to come.
func (f *follower) reached(e chatlog.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e.Kind == chatlog.KindTurnEnd {
		f.turnEnd = e
	} else {
		f.exit = e
	}
	f.awaits = slices.DeleteFunc(f.awaits, func(a *await) bool {
		if a.after >= e.Seq {
			return false
		}
		a.got <- e
		return true
	})
}

// await adds a, answering it at once from what the follower has taken already; an exit after its seq wins over a
// turn_end, since the thread no longer runs.
func (f *follower) await(a *await) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case f.ended:
		a.got <- chatlog.Event{Kind: chatlog.KindExit}
	case f.exit.Seq > a.after:
		a.got <- f.exit
	case f.turnEnd.Seq > a.after:
		a.got <- f.turnEnd
	default:
		f.awaits = append(f.awaits, a)
	}
}

func (f *follower) forget(a *await) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.awaits = slices.DeleteFunc(f.awaits, func(b *await) bool { return b == a })
}

// end answers every await with the thread gone once the follower stops.
func (f *follower) end() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ended = true
	for _, a := range f.awaits {
		a.got <- chatlog.Event{Kind: chatlog.KindExit}
	}
	f.awaits = nil
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
	if now == before || (now != nil && before != nil && now.ID == before.ID && now.Request == before.Request) {
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
	p := Pending{ID: id, Request: e.ID, Tool: e.Tool, Kind: "tool", Summary: ToolSummary(e.Tool, e.Input), Input: e.Input}
	if len(e.Input) > maxPendingInput {
		p.Input, p.InputTooLarge = nil, true
	}
	switch e.Tool {
	case "AskUserQuestion":
		p.Kind = "question"
	case "ExitPlanMode":
		p.Kind = "plan"
	}
	return p
}

// ToolSummary is what a call of tool with input would do, in one line: a question's first question, a plan's first
// line, else the command, path, pattern or URL it acts on; "" when the input has none of them.
func ToolSummary(tool string, input json.RawMessage) string {
	var in struct {
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
	_ = json.Unmarshal(input, &in)
	switch tool {
	case "AskUserQuestion":
		if len(in.Questions) > 0 {
			return in.Questions[0].Question
		}
		return ""
	case "ExitPlanMode":
		return firstLine(in.Plan)
	}
	for _, s := range []string{in.Command, in.FilePath, in.Path, in.Pattern, in.URL} {
		if s != "" {
			return firstLine(s)
		}
	}
	return ""
}

func firstLine(s string) string {
	for line := range strings.Lines(s) {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
