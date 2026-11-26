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

// waitAnswer is what one slice of wait answers, and all it answers.
type waitAnswer struct {
	Done bool   `json:"done"`
	Text string `json:"text"`
}

// waitCall runs one slice of wait over the control socket with the caller's token, as the agentos command does. It
// gives the answer, or the error the tool gave; it calls no t method, so it may run in its own goroutine.
func waitCall(h *apptest.Harness, token, thread string, slice time.Duration) (waitAnswer, string, error) {
	return waitCallUntil(h, token, thread, slice, time.Time{})
}

// waitCallUntil is waitCall for a wait whose agentos command stops calling at until, which it sends unless it is zero.
func waitCallUntil(h *apptest.Harness, token, thread string, slice time.Duration, until time.Time) (waitAnswer, string, error) {
	ms := slice.Milliseconds()
	opts := map[string]string{"input": `{"thread":"` + thread + `","timeout_seconds":600}`, "slice_ms": strconv.FormatInt(ms, 10)}
	if !until.IsZero() {
		opts["until_ms"] = strconv.FormatInt(until.UnixMilli(), 10)
	}
	resp, err := ctl.Call(context.Background(), ctl.SocketPath(h.State), ctl.Request{
		Cmd: "orchestrate", Token: token, Args: []string{"wait"}, TimeoutMs: int(ms) + 5000, Opts: opts,
	})
	if err != nil {
		return waitAnswer{}, "", err
	}
	if !resp.OK {
		return waitAnswer{}, resp.Error, nil
	}
	var a waitAnswer
	d := json.NewDecoder(strings.NewReader(resp.Out))
	d.DisallowUnknownFields()
	err = d.Decode(&a)
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
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, parent), Args: []string{"wait"}, TimeoutMs: 15000, Opts: map[string]string{"input": `{"thread":"` + c.ID + `"}`, "slice_ms": "10000"}})
	if want := `{"done":true,"text":"done"}`; !resp.OK || resp.Out != want {
		t.Errorf("wait answered %+v, want %s", resp, want)
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
	if !a.Done || a.Text != "done" {
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
	if !a.Done || a.Text != "done" {
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
	if a.Done || a.Text != "Thread "+c+" still runs." {
		t.Errorf("wait answered %+v, want not done and the thread still running", a)
	}
}

func TestWaitReturnsWhenTheChildSettles(t *testing.T) {
	// A delete settles the thread first, so the wait may see either.
	for _, tc := range []struct {
		name  string
		texts []string
		end   func(h *apptest.Harness, id string) error
	}{
		{"settled", []string{"Thread %s has settled."}, func(h *apptest.Harness, id string) error { return h.SettleThread(id) }},
		{"deleted", []string{"Thread %s has settled.", "Thread %s was deleted."}, func(h *apptest.Harness, id string) error { return h.DeleteThread(id, false) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := apptest.New(t)
			parent, c := busyChild(t, h)
			got := waitInBackground(t, h, parent, c, 30*time.Second)
			if err := tc.end(h, c); err != nil {
				t.Fatal(err)
			}
			r := waitResultOf(t, got, 5*time.Second)
			known := slices.ContainsFunc(tc.texts, func(text string) bool { return r.a.Text == strings.Replace(text, "%s", c, 1) })
			if r.err != nil || r.refused != "" || !r.a.Done || !known {
				t.Errorf("wait answered %+v, want done with one of %q", r, tc.texts)
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
	if want := "Thread " + c + " stopped running: its host or its Claude exited."; r.err == nil && r.refused == "" && (!r.a.Done || r.a.Text != want) {
		t.Errorf("wait answered %+v, want done with %q, or an error", r, want)
	}
}

func TestWaitGoesOnWhenTheAppStops(t *testing.T) {
	h := apptest.New(t)
	parent, c := busyChild(t, h)
	got := waitInBackground(t, h, parent, c, 30*time.Second)
	// The quit ends the followers before the control socket closes.
	h.BeginQuit()
	if r := waitResultOf(t, got, 5*time.Second); r.err != nil || r.refused != "" || r.a.Done {
		t.Errorf("wait answered %+v while the app stopped, want not done", r)
	}
	if !slices.Contains(tmuxSessions(t, h), c) {
		t.Errorf("thread %s does not run after the app stopped", c)
	}
}

func TestWaitWaitsForAHeldMessage(t *testing.T) {
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"Count the files","access":"read"}`).ID
	apptest.Eventually(t, "the task done", func() bool { return turnEnded(t, h, c) })
	sibling := newThread(t, h, "Fix the login").ID
	h.Hook(t, c, "Notification", `{"message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`)
	apptest.Eventually(t, "the waiting state", func() bool { return row(t, h, c).State == session.Waiting })
	if a := sendAs(t, h, sibling, `{"thread":"`+c+`","text":"Count the tests too"}`); !a.Held {
		t.Fatalf("send answered %+v, want it held", a)
	}

	got := waitInBackground(t, h, parent, c, 30*time.Second)
	select {
	case r := <-got:
		t.Fatalf("wait answered %+v while a message was held for the thread", r)
	case <-time.After(time.Second):
	}
	h.Hook(t, c, "Stop", `{}`)
	r := waitResultOf(t, got, 10*time.Second)
	if r.err != nil || !r.a.Done || r.a.Text != "done" {
		t.Errorf("wait answered %+v, want the held message's turn done", r)
	}
	if !slices.ContainsFunc(logOf(t, h, c), func(e chat.Event) bool { return e.Kind == chat.KindUser && e.From == sibling }) {
		t.Error("the wait returned before the held message was taken in")
	}
}

// sliceIn runs one slice of a wait that lasts until the time given, in its own goroutine.
func sliceIn(h *apptest.Harness, token, thread string, length time.Duration, until time.Time) <-chan waitResult {
	got := make(chan waitResult, 1)
	go func() {
		a, refused, err := waitCallUntil(h, token, thread, length, until)
		got <- waitResult{a, refused, err}
	}()
	return got
}

// busyAgain gives the thread "work" once more and returns once its stand-in claude has read it.
func busyAgain(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	turns := countIn(h.Stdin(t, id), `"text":"work"`)
	if _, err := h.ChatSend(id, "work"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the next turn that works", func() bool { return countIn(h.Stdin(t, id), `"text":"work"`) > turns })
}

func TestAWaitIsKeptUntilItEnds(t *testing.T) {
	h := apptest.New(t)
	parent, c := busyChild(t, h)
	token := tokenOf(t, h, parent)
	waits := func() bool { return h.App.Tools().Waits(parent, c) }
	slice := func(length time.Duration, until time.Time) <-chan waitResult {
		return sliceIn(h, token, c, length, until)
	}

	if r := waitResultOf(t, slice(time.Second, time.Now().Add(time.Minute)), 5*time.Second); r.err != nil || r.refused != "" || r.a.Done {
		t.Fatalf("the first slice answered %+v, want not done", r)
	}
	if !waits() {
		t.Error("between two slices the wait is not kept")
	}

	got := slice(30*time.Second, time.Now().Add(time.Minute))
	time.Sleep(300 * time.Millisecond)
	if err := h.Interrupt(c); err != nil {
		t.Fatal(err)
	}
	if r := waitResultOf(t, got, 5*time.Second); !r.a.Done {
		t.Fatalf("the slice answered %+v, want done", r)
	}
	if waits() {
		t.Error("the wait is kept after a slice answered it done")
	}

	busyAgain(t, h, c)
	if r := waitResultOf(t, slice(time.Second, time.Now().Add(1500*time.Millisecond)), 5*time.Second); r.a.Done || !waits() {
		t.Fatalf("a slice before the deadline answered %+v, kept %v; want not done and kept", r, waits())
	}
	time.Sleep(time.Second)
	if waits() {
		t.Error("the wait is kept after its deadline")
	}

	working(t, h, parent)
	got = slice(30*time.Second, time.Now().Add(time.Minute))
	time.Sleep(300 * time.Millisecond)
	if err := h.Interrupt(parent); err != nil {
		t.Fatal(err)
	}
	if r := waitResultOf(t, got, 5*time.Second); r.a.Done {
		t.Fatalf("the slice answered %+v, want not done", r)
	}
	if waits() {
		t.Error("the wait is kept after the waiter's turn ended")
	}
}

func TestAWaitEndsWithTheWaitersTurn(t *testing.T) {
	for _, tc := range []struct {
		name string
		end  func(h *apptest.Harness, id string) error
	}{
		{"interrupted", func(h *apptest.Harness, id string) error { return h.Interrupt(id) }},
		{"settled", func(h *apptest.Harness, id string) error { return h.SettleThread(id) }},
	} {
		t.Run(tc.name+" between two slices", func(t *testing.T) {
			h := apptest.New(t)
			parent, c := busyChild(t, h)
			busyAgain(t, h, parent)
			if r := waitResultOf(t, sliceIn(h, tokenOf(t, h, parent), c, time.Second, time.Now().Add(time.Minute)), 5*time.Second); r.err != nil || r.a.Done {
				t.Fatalf("the slice answered %+v, want not done", r)
			}
			if !h.App.Tools().Waits(parent, c) {
				t.Fatal("the wait is not kept between two slices")
			}
			if err := tc.end(h, parent); err != nil {
				t.Fatal(err)
			}
			apptest.Eventually(t, "the wait ended with the waiter's turn", func() bool { return !h.App.Tools().Waits(parent, c) })
		})
	}
	t.Run("interrupted as a slice ends", func(t *testing.T) {
		h := apptest.New(t)
		parent, c := busyChild(t, h)
		token := tokenOf(t, h, parent)
		for _, offset := range []time.Duration{-40 * time.Millisecond, -10 * time.Millisecond, 0, 10 * time.Millisecond, 40 * time.Millisecond} {
			busyAgain(t, h, parent)
			got := sliceIn(h, token, c, 400*time.Millisecond, time.Now().Add(time.Minute))
			time.Sleep(400*time.Millisecond + offset)
			if err := h.Interrupt(parent); err != nil {
				t.Fatal(err)
			}
			waitResultOf(t, got, 5*time.Second)
			apptest.Eventually(t, "the wait ended with the waiter's turn", func() bool { return !h.App.Tools().Waits(parent, c) })
		}
	})
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
	if a := waitOn(t, h, top, g, 10*time.Second); !a.Done || a.Text != "done" {
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
	if a.Done || a.Text != "Thread "+c+" still runs: it waits for the owner to answer an approval." {
		t.Errorf("wait answered %+v, want not done and waiting on the owner", a)
	}
	got := waitInBackground(t, h, parent, c, 30*time.Second)
	if err := h.ChatAnswer(c, "ask-1", true, ""); err != nil {
		t.Fatal(err)
	}
	if r := waitResultOf(t, got, 5*time.Second); r.err != nil || !r.a.Done || r.a.Text != "done" {
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
	if r.err != nil || !r.a.Done || r.a.Text != "done" {
		t.Errorf("wait answered %+v, want the queued message's turn completed", r)
	}
	if _, ok := logged(t, h, c, chat.KindUser, sent.ID); !ok {
		t.Error("the wait returned before the queued message was taken in")
	}
}
