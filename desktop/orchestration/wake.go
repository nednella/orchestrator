package orchestration

import (
	"context"
	"log"
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
	// again.
	retryFirst = time.Second
	retryMost  = time.Minute
)

// report is a child's turn_end, or its exit when it was put to sleep, at the seq end, which may give its parent the
// child's result. waited says that the parent's wait on the child was in flight then, and so gives the parent the
// result.
type report struct {
	child, parent string
	end           int64
	waited        bool
}

// ChildTurnEnded hears of each turn_end a thread's follower takes, and of the exit when the thread settles. When the
// thread is a child and its turn is over, as a wait counts it, or it was put to sleep in the middle of one, the result
// goes to its parent once: by the parent's wait on it when one is in flight, else as a message from the child that
// wakes a parent that sleeps and is sent whatever session_prompt_send says. Then the child goes to sleep, unless a task
// of its own still runs in the background; a later message wakes it, and each later turn's result goes to the parent
// as a later one. A turn that a thread stopped, as stop does, gives none: the stop answered that thread. It runs on the
// follower, so it looks only at the wait here and gives the result in a goroutine of its own, one child at a time, in
// order.
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

// give gives the child's result to its parent, records it given and puts the child to sleep. It gives nothing while
// the child's last turn has no result, or one that went already.
func (c *Commands) give(r report) {
	ctx := c.ctx()
	ch, err := c.threads.ChatOf(r.child)
	if err != nil || ch.Deleting {
		return
	}
	t := c.turnOf(ch)
	end := t.result()
	asleep := end != t.end
	switch {
	case end == 0 || end <= c.threads.Reported(r.child) || c.threads.HoldsTold(r.child):
		return
	case !asleep && (!t.ended || t.asks):
		return
	case !asleep && t.outcome == outcomeStopped && t.stopper != "":
		return
	}
	w := waitOn{r.parent, r.child}
	given := r.waited && c.waitGets(ctx, w, end)
	if ctx.Err() != nil {
		return
	}
	if !given && c.claim(w, end, false) {
		later := c.threads.Reported(r.child) > 0
		text := sessions.FromApp(prompts.ChildDone(c.threads.Title(r.child), r.child, outcomeText(t, asleep), t.reply, later, strings.HasSuffix(t.reply, cutMark)))
		if !c.sendResult(ctx, w, sessions.Told{From: r.child, Text: text, Body: t.reply, Result: true}) {
			return
		}
	}
	if err := c.threads.SetReported(r.child, end); err != nil {
		log.Printf("agentos: %v", err)
	}
	if t.open > 0 || asleep {
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

// sendResult tells the parent its child's result m until the parent takes it, and says whether it did: false once the
// parent is gone or ctx ends. A message held for the parent counts once it goes; one a stop of the parent kept goes at
// the parent's next turn, as the sessions hold it. One the parent could not take, as when its wake was refused, goes
// again after a pause that doubles up to a minute.
func (c *Commands) sendResult(ctx context.Context, w waitOn, m sessions.Told) bool {
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
		if _, gone := c.threads.ChatOf(w.waiter); gone != nil {
			log.Printf("agentos: thread %s did not get the result of its child %s: %v", w.waiter, w.thread, err)
			return false
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

// outcomeText is how a child's turn ended, as its result tells the parent; asleep says the owner put the child to
// sleep before it ended.
func outcomeText(t turn, asleep bool) string {
	switch {
	case asleep:
		return "was put to sleep by the owner"
	case t.outcome == outcomeStopped:
		return "was stopped by the owner"
	case t.outcome == outcomeFailed && t.reason != "":
		return "failed: " + t.reason
	case t.outcome == outcomeFailed:
		return "failed"
	}
	return "completed"
}
