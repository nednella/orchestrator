package orchestration

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/prompts"
)

const (
	// reportTick is how often a result in hand looks again at the wait it may go by.
	reportTick = 100 * time.Millisecond
	// retryFirst and retryMost are the first and the longest pause before a result the parent could not take goes
	// again, and how often a dropped one looks for the parent's next turn.
	retryFirst = time.Second
	retryMost  = time.Minute
)

// report is a child's turn that ended at the seq end, whose result goes to its parent. waited says that the parent's
// wait on the child was in flight when the turn ended, and so gives the parent the result.
type report struct {
	child, parent string
	end           int64
	waited        bool
}

// ChildTurnEnded hears of each turn_end a thread's follower takes. When the thread is a child and its turn is over, as
// a wait counts it, the result goes to its parent once: by the parent's wait on it when one is in flight, else as a
// message from the child that wakes a parent that sleeps and is sent whatever session_prompt_send says. Then the child
// goes to sleep, unless a task of its own still runs in the background; a later message wakes it, and each later turn's
// result goes to the parent as a later one. A turn that a thread stopped, as stop does, gives none: the stop answered
// that thread. It runs on the follower, so it looks only at the wait here and gives the result in a goroutine of its
// own, one child at a time, in order.
func (c *Commands) ChildTurnEnded(id string, e chatlog.Event) {
	parent := c.threads.ParentOf(id)
	if parent == "" || e.Seq <= c.threads.Reported(id) {
		return
	}
	w := waitOn{parent, id}
	r := report{child: id, parent: parent, end: e.Seq, waited: c.Waits(parent, id) || c.givenByWait(w, e.Seq)}
	c.reportMu.Lock()
	defer c.reportMu.Unlock()
	if c.reporting[id] {
		c.reportNext[id] = r
		return
	}
	c.reporting[id] = true
	go c.reportAll(r)
}

// reportAll gives the result r, then each next one that came for the same child meanwhile. Only the latest counts: a
// turn that ended before another one is over is no result.
func (c *Commands) reportAll(r report) {
	for {
		c.give(r)
		c.reportMu.Lock()
		next, ok := c.reportNext[r.child]
		delete(c.reportNext, r.child)
		if !ok {
			delete(c.reporting, r.child)
			c.reportMu.Unlock()
			return
		}
		c.reportMu.Unlock()
		r = next
	}
}

// give gives the child's result to its parent, records it given and puts the child to sleep. It gives nothing for a
// turn that is not the child's last, or not over yet.
func (c *Commands) give(r report) {
	ctx := c.ctx()
	ch, err := c.threads.ChatOf(r.child)
	if err != nil {
		return
	}
	t := c.turnOf(ch)
	if !t.ended || t.end != r.end || t.asks || c.threads.HoldsTold(r.child) {
		return
	}
	if t.outcome == outcomeStopped && t.stopper != "" {
		return
	}
	w := waitOn{r.parent, r.child}
	given := r.waited && c.waitGets(ctx, w, r.end)
	if ctx.Err() != nil {
		return
	}
	if !given && c.claim(w, r.end, false) {
		later := c.threads.Reported(r.child) > 0
		text := sessions.FromApp(prompts.ChildDone(c.threads.Title(r.child), r.child, outcomeText(t), t.reply, later, strings.HasSuffix(t.reply, cutMark)))
		if !c.sendResult(ctx, w, r.end, sessions.Told{From: r.child, Text: text, Body: t.reply, Result: true}) {
			return
		}
	}
	if err := c.threads.SetReported(r.child, r.end); err != nil {
		log.Printf("agentos: %v", err)
	}
	if t.open > 0 {
		return
	}
	if now, err := c.threads.ChatOf(r.child); err != nil || !now.Live {
		return
	}
	// A message that came for the child while its result went keeps it awake for the turn it starts.
	if _, err := c.threads.SettleIdle(r.child); err != nil {
		log.Printf("agentos: child %s did not go to sleep once its result went: %v", r.child, err)
	}
}

// waitGets says whether the wait w took the child's result that the seq end ends: it waits while the wait lasts, and
// says no once it no longer does without it, or ctx ends.
func (c *Commands) waitGets(ctx context.Context, w waitOn, end int64) bool {
	tick := time.NewTicker(reportTick)
	defer tick.Stop()
	for {
		if c.givenByWait(w, end) {
			return true
		}
		if !c.Waits(w.waiter, w.thread) {
			return c.givenByWait(w, end)
		}
		select {
		case <-ctx.Done():
			return false
		case <-tick.C:
		}
	}
}

// sendResult tells the parent its child's result m, which ends at the seq end, until the parent takes it, and says
// whether it did: false once the parent is gone or ctx ends. A message held for the parent counts once it goes. One
// dropped because the parent was stopped goes again once the parent's next turn or run starts, unless a wait of the
// parent's took the result meanwhile; one the parent could not take, as when its wake was refused, goes again after a
// pause that doubles up to a minute.
func (c *Commands) sendResult(ctx context.Context, w waitOn, end int64, m sessions.Told) bool {
	pause := retryFirst
	for {
		done := make(chan error, 1)
		m.Done = done
		id, err := c.threads.Tell(w.waiter, m)
		if err == nil && id == "" {
			select {
			case err = <-done:
			case <-ctx.Done():
				return false
			}
		}
		if err == nil {
			return true
		}
		ch, gone := c.threads.ChatOf(w.waiter)
		if gone != nil {
			log.Printf("agentos: thread %s did not get the result of its child %s: %v", w.waiter, w.thread, err)
			return false
		}
		if errors.Is(err, sessions.ErrToldStopped) {
			c.unclaim(w, end)
			if !c.untilRuns(ctx, ch) {
				return false
			}
			if !c.claim(w, end, false) {
				return true
			}
			continue
		}
		log.Printf("agentos: thread %s did not get the result of its child %s, trying again in %v: %v", w.waiter, w.thread, pause, err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(pause):
		}
		pause = min(2*pause, retryMost)
	}
}

// untilRuns waits until the thread's log has a user message or a run's start after what it holds now: the thread
// stopped at another thread's ask takes nothing until then. It says false once the thread is gone or ctx ends.
func (c *Commands) untilRuns(ctx context.Context, ch sessions.Chat) bool {
	from := c.turnOf(ch).newest
	tick := time.NewTicker(retryFirst)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-tick.C:
		}
		if _, err := c.threads.ChatOf(ch.ID); err != nil {
			return false
		}
		events, err := chatlog.ReadLog(ch.Log, from)
		if err != nil {
			log.Printf("agentos: reading the chat log of %s: %v", ch.ID, err)
			continue
		}
		if slices.ContainsFunc(events, func(e chatlog.Event) bool { return e.Kind == chatlog.KindUser || e.Kind == chatlog.KindStart }) {
			return true
		}
	}
}

// outcomeText is how a child's turn ended, as its result tells the parent.
func outcomeText(t turn) string {
	switch t.outcome {
	case outcomeStopped:
		return "was stopped by the owner"
	case outcomeFailed:
		if t.reason != "" {
			return "failed: " + t.reason
		}
		return "failed"
	}
	return "completed"
}
