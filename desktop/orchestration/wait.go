package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/nednella/agentos/desktop/chat"
	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
	ctl "github.com/nednella/agentos/internal/control"
)

const (
	// sliceMost is the longest one call of wait waits, in milliseconds: the agentos command waits in slices, so no
	// control call outlasts control.MaxTimeout.
	sliceMost = 60000
	// waitMost is the longest a whole wait lasts, as the agentos command limits it.
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
	// TimeoutSeconds is the agentos command's: it stops calling once it passes.
	TimeoutSeconds int `json:"timeout_seconds"`
}

// waited is what one slice of a wait answers. Text is what the agent is given: the thread's last reply once its turn
// ended, else a sentence that says how it stands.
type waited struct {
	Done bool   `json:"done"`
	Text string `json:"text"`
}

// waitOn is a wait: the thread waiter waits on thread.
type waitOn struct{ waiter, thread string }

// Waits says whether waiter waits on thread, so a result the wait answers it with is not also sent. A wait lasts from
// its first slice until a slice answers it done, the waiter's turn ends, or the deadline the agentos command gave
// passes; between two slices it still waits.
func (c *Commands) Waits(waiter, thread string) bool {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	w := waitOn{waiter, thread}
	if time.Now().Before(c.waiting[w]) {
		return true
	}
	delete(c.waiting, w)
	return false
}

// hold keeps the wait until the time until, or a later one it already has.
func (c *Commands) hold(w waitOn, until time.Time) {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	if until.After(c.waiting[w]) {
		c.waiting[w] = until
	}
}

func (c *Commands) release(w waitOn) {
	c.waitMu.Lock()
	defer c.waitMu.Unlock()
	delete(c.waiting, w)
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
	// until_ms is when the agentos command stops calling; without it the wait lasts the slice.
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
	w := waitOn{caller, in.Thread}
	c.hold(w, until)
	ctx, cancel := context.WithTimeout(ctx, slice)
	defer cancel()
	var callerEnded atomic.Bool
	go c.cancelAtTurnEnd(ctx, cancel, caller, &callerEnded)
	got := c.waitFor(ctx, ch)
	if got.Done || callerEnded.Load() {
		c.release(w)
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

// cancelAtTurnEnd sets ended and calls cancel once the caller's running turn ends or the caller stops running.
func (c *Commands) cancelAtTurnEnd(ctx context.Context, cancel context.CancelFunc, caller string, ended *atomic.Bool) {
	ch, err := c.threads.ChatOf(caller)
	if err != nil {
		log.Printf("agentos: the wait of %s does not follow its turn: %v", caller, err)
		return
	}
	events, err := chatlog.ReadLog(ch.Log, 0)
	if err != nil {
		log.Printf("agentos: reading the chat log of %s: %v", caller, err)
		return
	}
	var newest int64
	if len(events) > 0 {
		newest = events[len(events)-1].Seq
	}
	_, err = c.chats.AwaitTurn(ctx, caller, newest)
	if errors.Is(err, chat.ErrThreadGone) {
		if _, runs, err := c.stillRuns(caller); err == nil && runs {
			return
		}
	}
	if err == nil || errors.Is(err, chat.ErrThreadGone) {
		ended.Store(true)
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
	// ended says the last turn_end comes after the last user message and no queued message waits to be taken in.
	ended   bool
	outcome string // completed, failed or stopped, once ended
	reason  string // why a failed turn failed, as Claude said
	reply   string // the agent's last reply after the last user message, cut as read cuts it
	ran     bool   // Claude exited after its last start: this run is over
	settled bool   // that exit was the thread settling
	asks    bool   // an approval waits on the owner's answer
}

// turnOf reads the thread's last turn from its log; a log that cannot be read is an empty one.
func (c *Commands) turnOf(ch sessions.Chat) turn {
	events, err := chatlog.ReadLog(ch.Log, 0)
	if err != nil {
		log.Printf("agentos: reading the chat log of %s: %v", ch.ID, err)
	}
	return turnIn(events)
}

func turnIn(events []chatlog.Event) turn {
	var t turn
	var user, interrupt int64
	var end chatlog.Event
	var approvals []chatlog.Event
	var tasks chatlog.Tasks
	queued := map[string]bool{}
	for _, e := range events {
		t.newest = e.Seq
		approvals = chatlog.Waiting(approvals, e, &tasks)
		tasks.See(e)
		switch e.Kind {
		case chatlog.KindUser:
			user = e.Seq
			delete(queued, e.ID)
		case chatlog.KindQueued:
			queued[e.ID] = true
		case chatlog.KindUnsent:
			delete(queued, e.ID)
		case chatlog.KindInterrupt:
			interrupt = e.Seq
		case chatlog.KindTurnEnd:
			end = e
		case chatlog.KindStart:
			// A message queued in an earlier run never reaches this one.
			clear(queued)
			t.ran, t.settled = false, false
		case chatlog.KindExit:
			clear(queued)
			t.ran, t.settled = true, e.Stop == chatlog.StopSettle
		}
	}
	t.asks = len(approvals) > 0
	t.ended = end.Seq > user && len(queued) == 0
	switch {
	case interrupt > user && interrupt < end.Seq:
		t.outcome = outcomeStopped
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

// answer is the slice's answer for the thread id with its turn t and the outcome, "" while it runs.
func answer(id string, t turn, outcome string) waited {
	w := waited{Done: outcome != ""}
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
