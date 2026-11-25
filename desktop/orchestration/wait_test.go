package orchestration_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/internal/chat"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/session"
)

// waitAnswer is what one slice of wait answers.
type waitAnswer struct {
	Done                         bool
	Text, Outcome, State, Result string
	WaitingOnOwner               bool
}

// waitCall runs one slice of wait over the control socket with the caller's token, as the agentos command does. It
// gives the answer, or the error the tool gave; it calls no t method, so it may run in its own goroutine.
func waitCall(h *apptest.Harness, token, thread string, slice time.Duration) (waitAnswer, string, error) {
	ms := slice.Milliseconds()
	resp, err := ctl.Call(context.Background(), ctl.SocketPath(h.State), ctl.Request{
		Cmd: "orchestrate", Token: token, Args: []string{"wait"}, TimeoutMs: int(ms) + 5000,
		Opts: map[string]string{"input": `{"thread":"` + thread + `","timeout_seconds":600}`, "slice_ms": strconv.FormatInt(ms, 10)},
	})
	if err != nil {
		return waitAnswer{}, "", err
	}
	if !resp.OK {
		return waitAnswer{}, resp.Error, nil
	}
	var a waitAnswer
	err = json.Unmarshal([]byte(resp.Out), &a)
	return a, "", err
}

// waitOn is waitCall for a slice that must answer.
func waitOn(t *testing.T, h *apptest.Harness, caller, thread string, slice time.Duration) waitAnswer {
	t.Helper()
	a, refused, err := waitCall(h, tokenOf(t, h, caller), thread, slice)
	if err != nil || refused != "" {
		t.Fatalf("wait on %s: %v %s", thread, err, refused)
	}
	return a
}

type waitResult struct {
	a       waitAnswer
	refused string
	err     error
}

// waitInBackground starts one slice of wait and gives its answer on the channel it returns, once the slice is in
// flight.
func waitInBackground(t *testing.T, h *apptest.Harness, caller, thread string, slice time.Duration) <-chan waitResult {
	t.Helper()
	token := tokenOf(t, h, caller)
	got := make(chan waitResult, 1)
	go func() {
		a, refused, err := waitCall(h, token, thread, slice)
		got <- waitResult{a, refused, err}
	}()
	time.Sleep(300 * time.Millisecond)
	return got
}

func waitResultOf(t *testing.T, got <-chan waitResult, within time.Duration) waitResult {
	t.Helper()
	select {
	case r := <-got:
		return r
	case <-time.After(within):
		t.Fatalf("the wait did not return within %v", within)
		return waitResult{}
	}
}

// turnEnded says whether the thread's chat log holds a turn_end.
func turnEnded(t *testing.T, h *apptest.Harness, id string) bool {
	t.Helper()
	return slices.ContainsFunc(logOf(t, h, id), func(e chat.Event) bool { return e.Kind == chat.KindTurnEnd })
}

// busyChild is a child of a new thread that, once its task is done, keeps a turn open until an interrupt.
func busyChild(t *testing.T, h *apptest.Harness) (parent, id string) {
	t.Helper()
	parent = newThread(t, h, "Plan the release").ID
	id = mustDelegate(t, h, parent, `{"task":"Count the files","access":"read"}`).ID
	apptest.Eventually(t, "the task done", func() bool { return turnEnded(t, h, id) })
	working(t, h, id)
	return parent, id
}

func TestWaitReturnsTheResult(t *testing.T) {
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"Count the files in src","access":"read"}`)
	a := waitOn(t, h, parent, c.ID, 10*time.Second)
	if !a.Done || a.Outcome != "completed" || a.Result != "done" || a.Text != "done" || a.WaitingOnOwner {
		t.Errorf("wait answered %+v, want done and completed with the reply done", a)
	}
}

func TestWaitRightAfterDelegateWaitsForTheTask(t *testing.T) {
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	// The stand-in takes a second over a task that ends in slow.
	c := mustDelegate(t, h, parent, `{"task":"slow","access":"read"}`)
	if turnEnded(t, h, c.ID) {
		t.Fatal("the child's task ended before the wait began")
	}
	if r := row(t, h, c.ID); r.State != session.Idle {
		t.Logf("the child's row is %s before its first turn", r.State)
	}
	a := waitOn(t, h, parent, c.ID, 10*time.Second)
	if !a.Done || a.Outcome != "completed" || a.Result != "done" {
		t.Errorf("wait answered %+v, want the task's reply", a)
	}
	if !turnEnded(t, h, c.ID) {
		t.Error("the wait returned before the child's turn ended")
	}
}

func TestWaitOnAnIdleThreadReturnsAtOnce(t *testing.T) {
	h := apptest.New(t)
	caller := newThread(t, h, "Plan the release").ID
	target := newThread(t, h, "Fix the login").ID
	if _, err := h.ChatSend(target, "hello"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the turn ended", func() bool { return turnEnded(t, h, target) })
	begin := time.Now()
	a := waitOn(t, h, caller, target, time.Minute)
	if took := time.Since(begin); took > 2*time.Second {
		t.Errorf("the wait took %v, want it at once", took)
	}
	if !a.Done || a.Outcome != "completed" || a.Text != "done" {
		t.Errorf("wait answered %+v, want done with the reply", a)
	}
}

func TestWaitReturnsAtItsLimit(t *testing.T) {
	h := apptest.New(t)
	parent, c := busyChild(t, h)
	begin := time.Now()
	a := waitOn(t, h, parent, c, time.Second)
	if took := time.Since(begin); took < time.Second || took > 2*time.Second {
		t.Errorf("the slice of 1 s took %v", took)
	}
	if a.Done || a.Outcome != "" || a.Text != "Thread "+c+" still runs." {
		t.Errorf("wait answered %+v, want not done and the thread still running", a)
	}
}

func TestWaitReturnsWhenTheChildSettles(t *testing.T) {
	// A delete settles the thread first, so the wait may see either.
	for _, tc := range []struct {
		name  string
		texts map[string]string // by outcome
		end   func(h *apptest.Harness, id string) error
	}{
		{"settled", map[string]string{"settled": "Thread %s has settled."}, func(h *apptest.Harness, id string) error { return h.SettleThread(id) }},
		{"deleted", map[string]string{"settled": "Thread %s has settled.", "deleted": "Thread %s was deleted."}, func(h *apptest.Harness, id string) error { return h.DeleteThread(id, false) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := apptest.New(t)
			parent, c := busyChild(t, h)
			got := waitInBackground(t, h, parent, c, 30*time.Second)
			if err := tc.end(h, c); err != nil {
				t.Fatal(err)
			}
			r := waitResultOf(t, got, 5*time.Second)
			text, ok := tc.texts[r.a.Outcome]
			if want := strings.Replace(text, "%s", c, 1); r.err != nil || r.refused != "" || !r.a.Done || !ok || r.a.Text != want {
				t.Errorf("wait answered %+v, want done with one of %v", r, tc.texts)
			}
		})
	}
}

func TestWaitReturnsWhenTheHostDies(t *testing.T) {
	h := apptest.New(t)
	parent, c := busyChild(t, h)
	got := waitInBackground(t, h, parent, c, 30*time.Second)
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+c+":", "#{pane_pid}").Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("pane pid %q: %v", out, err)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	r := waitResultOf(t, got, 15*time.Second)
	if r.err == nil && r.refused == "" && (!r.a.Done || r.a.Outcome != "settled") {
		t.Errorf("wait answered %+v, want settled or an error", r)
	}
}

func TestWaitOnAnAncestorIsRefused(t *testing.T) {
	h := apptest.New(t)
	top := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, top, `{"task":"Look around","access":"read"}`).ID
	g := mustDelegate(t, h, c, `{"task":"Look closer","access":"read"}`).ID
	for _, tc := range []struct{ name, caller, thread, want string }{
		{"the parent", g, c, "Thread " + c + " started this thread; waiting on it would wait forever"},
		{"the grandparent", g, top, "Thread " + top + " started this thread; waiting on it would wait forever"},
		{"itself", g, g, "A thread cannot wait on itself"},
		{"no thread", g, "", "wait needs the id of a thread"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, refused, err := waitCall(h, tokenOf(t, h, tc.caller), tc.thread, time.Second)
			if err != nil || refused != tc.want {
				t.Errorf("wait answered %v %q, want %q", err, refused, tc.want)
			}
		})
	}
	if a := waitOn(t, h, top, g, 10*time.Second); !a.Done || a.Outcome != "completed" {
		t.Errorf("a wait on a grandchild answered %+v, want it done", a)
	}
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, top), Args: []string{"wait"}, Opts: map[string]string{"input": `{"thread":"` + g + `"}`, "slice_ms": "60001"}})
	if resp.OK || resp.Error != `slice_ms is 1 to 60000, not "60001"` {
		t.Errorf("a slice over a minute answered %+v", resp)
	}
}

func TestWaitReportsWaitingOnOwner(t *testing.T) {
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"Count the files","access":"supervised"}`).ID
	apptest.Eventually(t, "the task done", func() bool { return turnEnded(t, h, c) })
	if _, err := h.ChatSend(c, "ask"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the approval", func() bool { _, ok := logged(t, h, c, chat.KindApproval, "ask-1"); return ok })
	a := waitOn(t, h, parent, c, time.Second)
	if a.Done || !a.WaitingOnOwner || a.Text != "Thread "+c+" still runs: it waits for the owner to answer an approval." {
		t.Errorf("wait answered %+v, want not done and waiting on the owner", a)
	}
	got := waitInBackground(t, h, parent, c, 30*time.Second)
	if err := h.ChatAnswer(c, "ask-1", true, ""); err != nil {
		t.Fatal(err)
	}
	if r := waitResultOf(t, got, 5*time.Second); r.err != nil || !r.a.Done || r.a.Outcome != "completed" || r.a.WaitingOnOwner {
		t.Errorf("wait after the answer = %+v, want done", r)
	}
}

func TestWaitEndsWithTheCallersTurn(t *testing.T) {
	for _, tc := range []struct {
		name string
		end  func(h *apptest.Harness, id string) error
	}{
		{"interrupted", func(h *apptest.Harness, id string) error { return h.Interrupt(id) }},
		{"settled", func(h *apptest.Harness, id string) error { return h.SettleThread(id) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := apptest.New(t)
			parent, c := busyChild(t, h)
			working(t, h, parent)
			got := waitInBackground(t, h, parent, c, 30*time.Second)
			if err := tc.end(h, parent); err != nil {
				t.Fatal(err)
			}
			if r := waitResultOf(t, got, 3*time.Second); r.err != nil || r.refused != "" || r.a.Done {
				t.Errorf("wait answered %+v, want not done", r)
			}
		})
	}
}

func TestWaitWaitsForAQueuedMessage(t *testing.T) {
	h := apptest.New(t)
	parent, c := busyChild(t, h)
	sent, err := h.ChatSend(c, "and the tests")
	if err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the message queued", func() bool { _, ok := logged(t, h, c, chat.KindQueued, sent.ID); return ok })
	got := waitInBackground(t, h, parent, c, 30*time.Second)
	// The interrupt ends the turn, and the queued message then runs as a turn of its own.
	if err := h.Interrupt(c); err != nil {
		t.Fatal(err)
	}
	r := waitResultOf(t, got, 5*time.Second)
	if r.err != nil || !r.a.Done || r.a.Outcome != "completed" || r.a.Result != "done" {
		t.Errorf("wait answered %+v, want the queued message's turn completed", r)
	}
	if _, ok := logged(t, h, c, chat.KindUser, sent.ID); !ok {
		t.Error("the wait returned before the queued message was taken in")
	}
}
