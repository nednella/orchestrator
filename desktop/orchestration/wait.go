package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/nednella/orchestrator/desktop/chat"
	"github.com/nednella/orchestrator/desktop/sessions"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	ctl "github.com/nednella/orchestrator/internal/control"
)

const (
	// sliceMost is the longest one call of wait waits, in milliseconds: the orchestrator command waits in slices, so no
	// control call outlasts control.MaxTimeout.
	sliceMost = 60000
	// waitMost is the longest a whole wait lasts, as the orchestrator command limits it.
	waitMost = time.Hour
)

// The ways a wait ends, each with its own answer.
const (
	outcomeCompleted = "completed"
	outcomeFailed    = "failed"
	outcomeStopped   = "stopped"
	outcomeSettled   = "settled"
	outcomeDeleted   = "deleted"
)

type waitInput struct {
	Thread string `json:"thread"`
	// TimeoutSeconds is the orchestrator command's: it stops calling once it passes.
	TimeoutSeconds int `json:"timeout_seconds"`
}

// waited is what one slice of a wait answers. Text is what the agent is given: the thread's last reply once its turn
// ended, else a sentence that says how it stands. end is the seq that ends the turn whose result it gives, as
// turn.result says, which the agent is not given.
type waited struct {
	Done bool   `json:"done"`
	Text string `json:"text"`
	end  int64
}

// waitOn is a wait: the thread waiter waits on thread.
type waitOn struct{ waiter, thread string }

// held is how long a wait lasts: until the deadline the orchestrator command gave, and while the turn of the waiter's that
// called it runs, the turn that had logged up to seq from when the wait began.
type held struct {
	until time.Time
	from  int64
}

// Waits says whether waiter waits on thread, so a result the wait answers it with is not also sent. A wait lasts from
// its first slice until a slice answers it done, the waiter's turn ends or it stops running, or the deadline passes;
// between two slices it still waits.
func (c *Commands) Waits(waiter, thread string) bool {
	w := waitOn{waiter, thread}
	c.waitMu.Lock()
	h, ok := c.waiting[w]
	c.waitMu.Unlock()
	if !ok {
		return false
	}
	if time.Now().Before(h.until) && c.inTurn(waiter, h.from) {
		return true
	}
	c.waitMu.Lock()
	if c.waiting[w] == h {
		delete(c.waiting, w)
	}
	c.waitMu.Unlock()
	return false
}

// inTurn says whether the thread runs and its log has no turn's end, nor Claude's exit or start, after the seq from: as
// its follower has taken them, which a child's turn end asks at once, else as the log says.
func (c *Commands) inTurn(id string, from int64) bool {
	ch, err := c.threads.ChatOf(id)
	if err != nil || !ch.Live {
		return false
	}
	if over, known := c.chats.TurnOver(id, from); known {
		return !over
	}
	events, err := chatlog.ReadLog(ch.Log, from)
	if err != nil {
		log.Printf("orchestrator: reading the chat log of %s: %v", id, err)
		return false
	}
	return !slices.ContainsFunc(events, func(e chatlog.Event) bool {
		return e.Kind == chatlog.KindTurnEnd || e.Kind == chatlog.KindExit || e.Kind == chatlog.KindStart
	})
}

// hold keeps the wait until the time until, or a later one it already has. A wait that no longer lasts starts again
// from the seq newest of the waiter's log.
func (c *Commands) hold(w waitOn, until time.Time, newest int64) {
	lasts := c.Waits(w.waiter, w.thread)
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	h := c.waiting[w]
	if !lasts {
		h = held{from: newest}
	}
	if until.After(h.until) {
		h.until = until
	}
	c.waiting[w] = h
}

// givenBy is a child's result that went to its parent, or is going: the seq that ends the turn it is of, and whether a
// wait gave it, not a message.
type givenBy struct {
	end    int64
	byWait bool
}

// claim takes the child's result that the seq end ends for the way byWait names, and says whether it did: false when
// it, or a later one, was taken already. It takes waitMu.
func (c *Commands) claim(w waitOn, end int64, byWait bool) bool {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	return c.claimLocked(w, end, byWait)
}

func (c *Commands) claimLocked(w waitOn, end int64, byWait bool) bool {
	if c.given[w].end >= end {
		return false
	}
	c.given[w] = givenBy{end: end, byWait: byWait}
	return true
}

// ThreadDeleted forgets the waits and the results given to or by the thread id, which was deleted.
func (c *Commands) ThreadDeleted(id string) {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	maps.DeleteFunc(c.given, func(w waitOn, _ givenBy) bool { return w.waiter == id || w.thread == id })
	maps.DeleteFunc(c.waiting, func(w waitOn, _ held) bool { return w.waiter == id || w.thread == id })
}

// givenByWait says whether a wait of the parent's on the child took the result that the seq end ends, or a later one.
func (c *Commands) givenByWait(w waitOn, end int64) bool {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	g := c.given[w]
	return g.byWait && g.end >= end
}

// release ends the wait once a slice answered it done with got. A parent's wait on its child takes the child's result
// for got's turn, unless a message to the parent took it first: then the answer says so, and gives the parent no
// second copy of it.
func (c *Commands) release(w waitOn, got waited) waited {
	parent := c.threads.ParentOf(w.thread) == w.waiter
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	delete(c.waiting, w)
	if !parent || got.end == 0 {
		return got
	}
	g := c.given[w]
	switch {
	case g.end >= got.end && g.byWait:
	case g.end >= got.end, c.threads.Reported(w.thread) >= got.end:
		return waited{Done: true, Text: fmt.Sprintf("Thread %s's result of this turn is sent to you as a message instead.", w.thread), end: got.end}
	default:
		c.claimLocked(w, got.end, true)
	}
	return got
}

// wait answers one slice of the wait tool: it returns once the thread's turn has ended, it settles or is deleted, or
// the slice ends. It ends at once too when the caller's own turn ends, as when the owner interrupts it, or the caller
// stops running: nothing then waits for the answer.
func (c *Commands) wait(ctx context.Context, caller string, req ctl.Request) (string, error) {
	var in waitInput
	if err := decode(req.Opts["input"], &in); err != nil {
		return "", err
	}
	ms, err := strconv.Atoi(req.Opts["slice_ms"])
	if err != nil || ms < 1 || ms > sliceMost {
		return "", fmt.Errorf("slice_ms is 1 to %d, not %q", sliceMost, req.Opts["slice_ms"])
	}
	now, slice := time.Now(), time.Duration(ms)*time.Millisecond
	// until_ms is when the orchestrator command stops calling; without it the wait lasts the slice.
	until := now.Add(slice)
	if s := req.Opts["until_ms"]; s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return "", fmt.Errorf("until_ms is a time in unix milliseconds, not %q", s)
		}
		if asked := time.UnixMilli(n); asked.After(until) {
			until = asked
		}
		if last := now.Add(waitMost); until.After(last) {
			until = last
		}
	}
	switch {
	case in.Thread == "":
		return "", errors.New("wait needs the id of a thread")
	case in.Thread == caller:
		return "", errors.New("A thread cannot wait on itself")
	case c.isAncestor(in.Thread, caller):
		return "", fmt.Errorf("Thread %s started this thread; waiting on it would wait forever", in.Thread)
	}
	ch, err := c.threads.ChatOf(in.Thread)
	if err != nil {
		return "", err
	}
	callerChat, err := c.threads.ChatOf(caller)
	if err != nil {
		return "", err
	}
	newest := c.turnOf(callerChat).newest
	w := waitOn{caller, in.Thread}
	c.hold(w, until, newest)
	ctx, cancel := context.WithTimeout(ctx, slice)
	defer cancel()
	go c.cancelAtTurnEnd(ctx, cancel, caller, newest)
	got := c.waitFor(ctx, ch)
	if got.Done {
		got = c.release(w, got)
	}
	data, err := json.Marshal(got)
	if err != nil {
		return "", fmt.Errorf("writing the answer: %w", err)
	}
	return string(data), nil
}

// isAncestor says whether thread started id, or started a thread that did, and so on.
func (c *Commands) isAncestor(thread, id string) bool {
	seen := map[string]bool{id: true}
	for p := c.threads.ParentOf(id); p != "" && !seen[p]; p = c.threads.ParentOf(p) {
		if p == thread {
			return true
		}
		seen[p] = true
	}
	return false
}

// cancelAtTurnEnd calls cancel once the caller's turn running at its log's seq newest ends, or the caller stops running.
func (c *Commands) cancelAtTurnEnd(ctx context.Context, cancel context.CancelFunc, caller string, newest int64) {
	_, err := c.chats.AwaitTurn(ctx, caller, newest)
	if errors.Is(err, chat.ErrThreadGone) {
		if _, runs, err := c.stillRuns(caller); err == nil && runs {
			return
		}
	}
	if err == nil || errors.Is(err, chat.ErrThreadGone) {
		cancel()
	}
}

// stillRuns is how the thread's turn stands once its follower stopped, and whether the thread runs on: a follower also
// stops when the app quits, and the thread's turn may then still end. It errs for a thread that was deleted.
func (c *Commands) stillRuns(id string) (turn, bool, error) {
	ch, err := c.threads.ChatOf(id)
	if err != nil {
		return turn{}, false, err
	}
	t := c.turnOf(ch)
	return t, ch.Live && !t.ran, nil
}

// waitFor is the answer once the thread's turn ends, it no longer runs or ctx ends. Each turn_end its follower takes
// has the log read again, since a message queued meanwhile, or held for it while it waited on the owner, starts a turn
// of its own.
func (c *Commands) waitFor(ctx context.Context, ch sessions.Chat) waited {
	for {
		t := c.turnOf(ch)
		switch {
		case !ch.Live || t.ran:
			return answer(ch.ID, t, outcomeSettled)
		case t.ended && !c.threads.HoldsTold(ch.ID):
			return answer(ch.ID, t, t.outcome)
		}
		_, err := c.chats.AwaitTurn(ctx, ch.ID, t.newest)
		switch {
		case errors.Is(err, chat.ErrThreadGone):
			t, runs, err := c.stillRuns(ch.ID)
			switch {
			case err != nil:
				return answer(ch.ID, turn{}, outcomeDeleted)
			case runs:
				return answer(ch.ID, t, "")
			}
			return answer(ch.ID, t, outcomeSettled)
		case err != nil:
			return answer(ch.ID, c.turnOf(ch), "")
		}
	}
}

// turn is how a thread's last turn stands in its chat log.
type turn struct {
	newest int64 // the seq of the log's last event
	end    int64 // the seq of the log's last turn_end
	user   int64 // the seq of the log's last user message
	exit   int64 // the seq of Claude's last exit
	// ended says the last turn_end comes after the last user message, no queued message waits to be taken in, no
	// subagent works in the background and no turn Claude runs to tell of a task's end is due: Claude runs one after
	// a task of its own ends.
	ended   bool
	outcome string // completed, failed or stopped, once ended
	reason  string // why a failed turn failed, as Claude said
	stopper string // the thread that asked for the interrupt that stopped the turn, "" for the owner
	reply   string // the agent's last reply after the last user message, cut as read cuts it
	ran     bool   // Claude exited after its last start: this run is over
	settled bool   // that exit was the thread settling
	byOwner bool   // the owner put the thread to sleep there
	asks    bool   // an approval waits on the owner's answer
	open    int    // the tasks Claude has open in the background, shells among them
}

// turnOf reads the thread's last turn from its log; a log that cannot be read is an empty one.
func (c *Commands) turnOf(ch sessions.Chat) turn {
	events, err := chatlog.ReadLog(ch.Log, 0)
	if err != nil {
		log.Printf("orchestrator: reading the chat log of %s: %v", ch.ID, err)
	}
	return turnIn(events)
}

func turnIn(events []chatlog.Event) turn {
	var t turn
	var user int64
	var end, interrupt chatlog.Event
	var approvals []chatlog.Event
	var tasks chatlog.Tasks
	reporting := false
	queued := map[string]bool{}
	for _, e := range events {
		t.newest = e.Seq
		approvals = chatlog.Waiting(approvals, e, &tasks)
		if task, ok := tasks.See(e); ok && task.ReportedAlone(e) {
			reporting = true
		}
		switch e.Kind {
		case chatlog.KindUser:
			user = e.Seq
			delete(queued, e.ID)
		case chatlog.KindQueued:
			queued[e.ID] = true
		case chatlog.KindUnsent:
			delete(queued, e.ID)
		case chatlog.KindInterrupt:
			interrupt = e
		case chatlog.KindTurnEnd:
			end, reporting = e, false
		case chatlog.KindStart:
			// A message queued in an earlier run never reaches this one.
			clear(queued)
			t.ran, t.settled, t.byOwner, reporting = false, false, false, false
		case chatlog.KindExit:
			clear(queued)
			t.ran, t.settled, reporting = true, e.Stop == chatlog.StopSettle, false
			t.byOwner = t.settled && e.Text == chatlog.ByOwner
			t.exit = e.Seq
		}
	}
	t.asks, t.open, t.end, t.user = len(approvals) > 0, len(tasks.Open()), end.Seq, user
	t.ended = end.Seq > user && len(queued) == 0 && tasks.Working() == 0 && !reporting
	switch {
	case interrupt.Seq > user && interrupt.Seq < end.Seq:
		t.outcome, t.stopper = outcomeStopped, interrupt.From
	case end.Error:
		t.outcome, t.reason = outcomeFailed, end.Text
	default:
		t.outcome = outcomeCompleted
	}
	for _, e := range entries(events, false) {
		if e.Seq > user && e.From == fromAgent {
			t.reply = e.Text
		}
	}
	return t
}

// result is the seq that ends the last turn as its result: its turn_end, or the exit of a thread put to sleep before
// the turn ended. It is 0 while the last turn has neither.
func (t turn) result() int64 {
	switch {
	case t.end > t.user:
		return t.end
	case t.user > 0 && t.ran && t.settled && t.exit > t.user:
		return t.exit
	}
	return 0
}

// answer is the slice's answer for the thread id with its turn t and the outcome, "" while it runs.
func answer(id string, t turn, outcome string) waited {
	w := waited{Done: outcome != "", end: t.result()}
	var head string
	switch outcome {
	case "":
		w.Text = fmt.Sprintf("Thread %s still runs.", id)
		if t.asks {
			w.Text = fmt.Sprintf("Thread %s still runs: it waits for the owner to answer an approval.", id)
		}
		return w
	case outcomeCompleted:
		if t.reply != "" {
			w.Text = t.reply
			return w
		}
		head = fmt.Sprintf("Thread %s ended its turn with no reply.", id)
	case outcomeFailed:
		head = fmt.Sprintf("Thread %s's turn failed.", id)
		if t.reason != "" {
			head = fmt.Sprintf("Thread %s's turn failed: %s", id, t.reason)
		}
	case outcomeStopped:
		head = fmt.Sprintf("Thread %s's turn was stopped.", id)
	case outcomeSettled:
		head = fmt.Sprintf("Thread %s has settled.", id)
		if !t.settled {
			head = fmt.Sprintf("Thread %s stopped running: its host or its Claude exited.", id)
		}
	case outcomeDeleted:
		head = fmt.Sprintf("Thread %s was deleted.", id)
	}
	w.Text = head
	if t.reply != "" {
		w.Text += " Its last reply:\n\n" + t.reply
	}
	return w
}
