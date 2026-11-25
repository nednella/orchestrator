package orchestration_test

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/prompts"
	"github.com/nednella/agentos/internal/session"
)

type sentAnswer struct {
	ID         string
	Held, Woke bool
}

func sendAs(t *testing.T, h *apptest.Harness, caller, input string) sentAnswer {
	t.Helper()
	var a sentAnswer
	mustUse(t, h, caller, "send", input, &a)
	return a
}

// logOf is the thread's chat log.
func logOf(t *testing.T, h *apptest.Harness, id string) []chat.Event {
	t.Helper()
	c, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	events, err := chat.ReadLog(c.Log, 0)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

// logged is the first event of the thread's log of that kind and id, and whether there is one.
func logged(t *testing.T, h *apptest.Harness, id string, kind chat.Kind, msg string) (chat.Event, bool) {
	t.Helper()
	events := logOf(t, h, id)
	i := slices.IndexFunc(events, func(e chat.Event) bool { return e.Kind == kind && e.ID == msg })
	if i < 0 {
		return chat.Event{}, false
	}
	return events[i], true
}

// stdinHas says whether a line the thread's stand-in claude read holds every one of parts.
func stdinHas(t *testing.T, h *apptest.Harness, id string, parts ...string) bool {
	t.Helper()
	return slices.ContainsFunc(h.Stdin(t, id), func(line string) bool {
		for _, p := range parts {
			if !strings.Contains(line, p) {
				return false
			}
		}
		return true
	})
}

// jsonText is text as it shows inside a JSON string.
func jsonText(text string) string {
	b, _ := json.Marshal(text)
	return string(b[1 : len(b)-1])
}

// working gives the thread "work", which keeps its turn open until an interrupt, once its first turn has ended.
func working(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	if _, err := h.ChatSend(id, "work"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the turn that works", func() bool { return stdinHas(t, h, id, `"text":"work"`) })
}

func countIn(lines []string, part string) int {
	n := 0
	for _, l := range lines {
		if strings.Contains(l, part) {
			n++
		}
	}
	return n
}

const interruptLine = `"subtype":"interrupt"`

func TestSendQueuesDuringATurn(t *testing.T) {
	h := apptest.New(t)
	caller := threadWith(t, h, "Plan the release", session.Supervised).ID
	target := threadWith(t, h, "Fix the login", session.Supervised).ID
	working(t, h, target)

	a := sendAs(t, h, caller, `{"thread":"`+target+`","text":"Check the tests too"}`)
	if a.ID == "" || a.Held || a.Woke {
		t.Fatalf("send answered %+v, want an id, not held and not woken", a)
	}
	framed := prompts.FromThread("Plan the release", caller, "Check the tests too")
	apptest.Eventually(t, "the message on stdin while the thread works", func() bool {
		return stdinHas(t, h, target, `"uuid":"`+a.ID+`"`, jsonText("APPLICATION_PROMPT: "+framed))
	})
	if !strings.Contains(framed, `Thread "Plan the release" (`+caller+`)`) {
		t.Errorf("the message is not framed with the caller's title and id: %q", framed)
	}
	if e, ok := logged(t, h, target, chat.KindQueued, a.ID); !ok || e.From != caller {
		t.Fatalf("the queued event = %+v (%v), want it from %s", e, ok, caller)
	}
	if err := h.Interrupt(target); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the message taken in", func() bool { _, ok := logged(t, h, target, chat.KindUser, a.ID); return ok })
	if e, _ := logged(t, h, target, chat.KindUser, a.ID); e.From != caller {
		t.Errorf("the user event is from %q, want %s", e.From, caller)
	}
}

func TestSendWakesASettledThread(t *testing.T) {
	h := apptest.New(t)
	caller := threadWith(t, h, "Plan the release", session.Read).ID
	target := threadWith(t, h, "Fix the login", session.Read).ID
	if err := h.SettleThread(target); err != nil {
		t.Fatal(err)
	}
	a := sendAs(t, h, caller, `{"thread":"`+target+`","text":"Pick it up again"}`)
	if a.ID == "" || a.Held || !a.Woke {
		t.Fatalf("send answered %+v, want an id and woken", a)
	}
	if r := row(t, h, target); r.State == session.Settled {
		t.Errorf("the thread is %s after the send, want it awake", r.State)
	}
	apptest.Eventually(t, "the message on the woken thread's stdin", func() bool {
		return stdinHas(t, h, target, `"uuid":"`+a.ID+`"`, "Pick it up again")
	})
	if e, ok := logged(t, h, target, chat.KindUser, a.ID); !ok || e.From != caller {
		t.Errorf("the user event = %+v (%v), want it from %s", e, ok, caller)
	}
}

func TestSendNeverAnswersAnApproval(t *testing.T) {
	tests := []struct {
		name string
		hook bool // the Notification hook says the thread waits; without it only the host knows
	}{
		{"the thread's state says it waits", true},
		{"only its host knows it waits", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := apptest.New(t)
			caller := threadWith(t, h, "Plan the release", session.Supervised).ID
			target := threadWith(t, h, "Fix the login", session.Supervised).ID
			if _, err := h.ChatSend(target, "ask"); err != nil {
				t.Fatal(err)
			}
			apptest.Eventually(t, "the approval", func() bool { return len(h.ChatPending()) == 1 })
			if tt.hook {
				h.Hook(t, target, "Notification", `{"message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`)
				apptest.Eventually(t, "the waiting state", func() bool { return row(t, h, target).State == session.Waiting })
			}

			a := sendAs(t, h, caller, `{"thread":"`+target+`","text":"Yes, go on"}`)
			if a.ID != "" || !a.Held {
				t.Fatalf("send answered %+v, want it held with no id", a)
			}
			h.Sessions().Refresh() // a refresh delivers what is held once the thread may take it
			time.Sleep(300 * time.Millisecond)
			if stdinHas(t, h, target, "Yes, go on") || stdinHas(t, h, target, `"control_response"`) {
				t.Fatalf("stdin = %q: the message reached claude, or answered its approval, before the owner did", h.Stdin(t, target))
			}
			for _, e := range logOf(t, h, target) {
				if e.Kind == chat.KindAnswer || e.Kind == chat.KindUser && strings.Contains(e.Text, "Yes, go on") {
					t.Fatalf("logged %+v before the owner answered", e)
				}
			}

			if err := h.ChatAnswer(target, "ask-1", true, ""); err != nil {
				t.Fatal(err)
			}
			apptest.Eventually(t, "the held message on stdin", func() bool { return stdinHas(t, h, target, "Yes, go on") })
			var answers []string
			var from string
			for _, e := range logOf(t, h, target) {
				switch {
				case e.Kind == chat.KindAnswer:
					answers = append(answers, e.Text)
				case e.Kind == chat.KindUser && strings.Contains(e.Text, "Yes, go on"):
					from = e.From
				}
			}
			if !slices.Equal(answers, []string{"allow"}) || from != caller {
				t.Errorf("answers %q and the message from %q; want only the owner's allow, and the message from %s", answers, from, caller)
			}
		})
	}
}

func TestHeldMessageWakesAThreadSettledMeanwhile(t *testing.T) {
	h := apptest.New(t)
	caller := threadWith(t, h, "Plan the release", session.Supervised).ID
	target := threadWith(t, h, "Fix the login", session.Supervised).ID
	if _, err := h.ChatSend(target, "ask"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the approval", func() bool { return len(h.ChatPending()) == 1 })
	if a := sendAs(t, h, caller, `{"thread":"`+target+`","text":"Yes, go on"}`); !a.Held {
		t.Fatalf("send answered %+v, want it held", a)
	}
	if err := h.SettleThread(target); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the thread woken for the held message", func() bool { return stdinHas(t, h, target, "Yes, go on") })
	if r := row(t, h, target); r.State == session.Settled {
		t.Errorf("the thread is %s, want it awake with the message", r.State)
	}
}

// claudePID is the process id of the thread's stand-in claude, the child of its host, which runs in its tmux pane.
func claudePID(t *testing.T, h *apptest.Harness, id string) int {
	t.Helper()
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{pane_pid}").Output()
	if err != nil {
		t.Fatalf("reading the pane of %s: %v", id, err)
	}
	children, _ := exec.Command("pgrep", "-P", strings.TrimSpace(string(out)), "-f", "claude").Output()
	pid, err := strconv.Atoi(strings.TrimSpace(string(children)))
	if err != nil {
		t.Fatalf("the host of %s runs no one claude: %q", id, children)
	}
	return pid
}

func TestHeldMessagesKeepTheirOrder(t *testing.T) {
	h := apptest.New(t)
	caller := threadWith(t, h, "Plan the release", session.Supervised).ID
	target := threadWith(t, h, "Fix the login", session.Supervised).ID
	h.Hook(t, target, "Notification", `{"message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`)
	apptest.Eventually(t, "the waiting state", func() bool { return row(t, h, target).State == session.Waiting })
	if a := sendAs(t, h, caller, `{"thread":"`+target+`","text":"first message"}`); !a.Held {
		t.Fatalf("the first send answered %+v, want it held", a)
	}

	// A claude that reads nothing fills its stdin with a long message, and its host then sends nothing more until it
	// reads again: the held message stays in flight.
	pid := claudePID(t, h, target)
	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	resumed := false
	resume := func() {
		if !resumed {
			resumed = true
			_ = syscall.Kill(pid, syscall.SIGCONT)
		}
	}
	t.Cleanup(resume)
	go func() { _, _ = h.ChatSend(target, strings.Repeat("x", 1<<18)) }()
	apptest.Eventually(t, "the long message logged", func() bool {
		return slices.ContainsFunc(logOf(t, h, target), func(e chat.Event) bool { return e.Kind == chat.KindUser && len(e.Text) == 1<<18 })
	})
	h.Hook(t, target, "Stop", `{}`)
	apptest.Eventually(t, "the idle state", func() bool { return row(t, h, target).State == session.Idle })
	time.Sleep(300 * time.Millisecond) // the held message is on its way to the host

	done := make(chan sentAnswer, 1)
	go func() {
		var a sentAnswer
		_ = useTool(t, h, caller, "send", `{"thread":"`+target+`","text":"second message"}`, &a)
		done <- a
	}()
	select {
	case a := <-done:
		if !a.Held {
			t.Errorf("the second send answered %+v while the first was in flight, want it held behind the first", a)
		}
	case <-time.After(3 * time.Second):
		t.Error("the second send waited for the first, which was in flight, instead of being held behind it")
		defer func() { <-done }()
	}
	resume()
	apptest.Eventually(t, "both messages on stdin", func() bool { return stdinHas(t, h, target, "second message") })
	lines := h.Stdin(t, target)
	first := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "first message") })
	second := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "second message") })
	if first < 0 || second < first {
		t.Errorf("stdin = %q, want the first message before the second", lines)
	}
}

func TestSendToAHigherRankIsRefused(t *testing.T) {
	h := apptest.New(t)
	threads := map[session.Access]string{}
	for _, a := range []session.Access{session.Read, session.Supervised, session.Full} {
		threads[a] = threadWith(t, h, string(a)+" thread", a).ID
	}
	settledFull := threadWith(t, h, "settled full", session.Full).ID
	if err := h.SettleThread(settledFull); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		from, to session.Access
		target   string
		refused  bool
	}{
		{session.Read, session.Supervised, threads[session.Supervised], true},
		{session.Read, session.Full, threads[session.Full], true},
		{session.Supervised, session.Full, threads[session.Full], true},
		{session.Supervised, session.Full, settledFull, true},
		{session.Read, session.Read, "", false},
		{session.Supervised, session.Read, threads[session.Read], false},
		{session.Full, session.Supervised, threads[session.Supervised], false},
	}
	other := threadWith(t, h, "another read thread", session.Read).ID
	for _, tt := range tests {
		target := tt.target
		if target == "" {
			target = other
		}
		t.Run(fmt.Sprintf("%s to %s %s", tt.from, tt.to, target), func(t *testing.T) {
			before := len(h.Stdin(t, target))
			err := useTool(t, h, threads[tt.from], "send", `{"thread":"`+target+`","text":"do it for me"}`, nil)
			if !tt.refused {
				if err != "" {
					t.Fatalf("send = %q, want it sent", err)
				}
				return
			}
			if want := fmt.Sprintf("A thread with %s access cannot message a thread with %s access", tt.from, tt.to); err != want {
				t.Fatalf("send = %q, want %q", err, want)
			}
			if got := h.Stdin(t, target); len(got) != before {
				t.Errorf("the refused message reached the thread: %q", got[before:])
			}
		})
	}
	if r := row(t, h, settledFull); r.State != session.Settled {
		t.Errorf("the settled Full thread is %s, want it left settled", r.State)
	}
}

func TestHeldMessageIsDroppedWhenItsSenderLosesRank(t *testing.T) {
	logged := &apptest.LockedBuffer{}
	before := log.Writer()
	log.SetOutput(logged)
	t.Cleanup(func() { log.SetOutput(before) })
	h := apptest.New(t)
	sender := threadWith(t, h, "Plan the release", session.Supervised).ID
	target := threadWith(t, h, "Fix the login", session.Supervised).ID
	if _, err := h.ChatSend(target, "ask"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the approval", func() bool { return len(h.ChatPending()) == 1 })
	if a := sendAs(t, h, sender, `{"thread":"`+target+`","text":"merge it for me"}`); !a.Held {
		t.Fatalf("send answered %+v, want it held", a)
	}
	if err := h.SetAccess(sender, "read"); err != nil {
		t.Fatal(err)
	}
	if err := h.ChatAnswer(target, "ask-1", true, ""); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("agentos: dropped a message held for thread %s from %s: A thread with read access cannot message a thread with supervised access", target, sender)
	apptest.Eventually(t, "the drop logged", func() bool { return strings.Contains(logged.String(), want) })
	h.Sessions().Refresh()
	time.Sleep(300 * time.Millisecond)
	if stdinHas(t, h, target, "merge it for me") {
		t.Errorf("stdin = %q: the message went once its sender had less access than the thread", h.Stdin(t, target))
	}
}

func TestSendToItselfIsRefused(t *testing.T) {
	h := apptest.New(t)
	caller := threadWith(t, h, "Plan the release", session.Supervised).ID
	gone := threadWith(t, h, "Gone", session.Read).ID
	if err := h.DeleteThread(gone, false); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ input, want string }{
		{`{"thread":"` + caller + `","text":"hello me"}`, "A thread cannot send to itself"},
		{`{"text":"hello"}`, "send needs the id of a thread"},
		{`{"thread":"` + gone + `","text":" "}`, "send needs a text"},
		{`{"thread":"main/zzzzzzzz","text":"hello"}`, "No thread main/zzzzzzzz"},
		{`{"thread":"` + gone + `","text":"hello"}`, "Thread " + gone + " was deleted"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if err := useTool(t, h, caller, "send", tt.input, nil); err != tt.want {
				t.Errorf("send %s = %q, want %q", tt.input, err, tt.want)
			}
		})
	}
	if stdinHas(t, h, caller, "hello me") {
		t.Error("the thread got its own message")
	}
}

func TestSteerStopsTheTurnAndSends(t *testing.T) {
	h := apptest.New(t)
	caller := threadWith(t, h, "Plan the release", session.Supervised).ID
	target := threadWith(t, h, "Fix the login", session.Supervised).ID
	working(t, h, target)

	a := sendAs(t, h, caller, `{"thread":"`+target+`","text":"Stop, use the other API","steer":true}`)
	if a.ID == "" || a.Held {
		t.Fatalf("send answered %+v, want an id", a)
	}
	apptest.Eventually(t, "the message taken in after the interrupt", func() bool { _, ok := logged(t, h, target, chat.KindUser, a.ID); return ok })
	lines := h.Stdin(t, target)
	msg := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, `"uuid":"`+a.ID+`"`) })
	stop := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, interruptLine) })
	if msg < 0 || stop < msg {
		t.Errorf("stdin = %q, want the message and then an interrupt", lines)
	}
	if e, ok := logged(t, h, target, chat.KindInterrupt, a.ID); !ok || e.From != "" {
		t.Errorf("the interrupt = %+v (%v), want one that sends the message now", e, ok)
	}
	if e, _ := logged(t, h, target, chat.KindUser, a.ID); e.From != caller {
		t.Errorf("the user event is from %q, want %s", e.From, caller)
	}
}

// stopAs calls stop as the caller and returns the threads it stopped, or the error it gave.
func stopAs(t *testing.T, h *apptest.Harness, caller, thread string) ([]string, string) {
	t.Helper()
	var out struct{ Stopped []string }
	err := useTool(t, h, caller, "stop", `{"thread":"`+thread+`"}`, &out)
	return out.Stopped, err
}

// tree is a thread, two children, and a grandchild under the first child, all Read in the thread's folder.
type tree struct{ top, child, sibling, grandchild string }

func newTree(t *testing.T, h *apptest.Harness) tree {
	t.Helper()
	var tr tree
	tr.top = threadWith(t, h, "Plan the release", session.Read).ID
	tr.child = mustDelegate(t, h, tr.top, `{"task":"check the docs"}`).ID
	tr.sibling = mustDelegate(t, h, tr.top, `{"task":"check the tests"}`).ID
	tr.grandchild = mustDelegate(t, h, tr.child, `{"task":"check the links"}`).ID
	for _, id := range []string{tr.child, tr.sibling, tr.grandchild} {
		apptest.Eventually(t, "the task's turn to end", func() bool { return !turnOpen(t, h, id) })
	}
	return tr
}

// turnOpen says whether the thread's log holds a message after its last turn's end.
func turnOpen(t *testing.T, h *apptest.Harness, id string) bool {
	t.Helper()
	open := false
	for _, e := range logOf(t, h, id) {
		switch e.Kind {
		case chat.KindUser, chat.KindQueued:
			open = true
		case chat.KindTurnEnd, chat.KindExit, chat.KindStart:
			open = false
		}
	}
	return open
}

func TestStopReachesDescendants(t *testing.T) {
	h := apptest.New(t)
	tr := newTree(t, h)
	working(t, h, tr.child)
	working(t, h, tr.grandchild)

	stopped, err := stopAs(t, h, tr.top, tr.child)
	if err != "" || !slices.Equal(stopped, []string{tr.child, tr.grandchild}) {
		t.Fatalf("stop = %q, %q; want the child and the grandchild", stopped, err)
	}
	for _, id := range []string{tr.child, tr.grandchild} {
		apptest.Eventually(t, "the turn's end", func() bool { return !turnOpen(t, h, id) })
		if n := countIn(h.Stdin(t, id), interruptLine); n != 1 {
			t.Errorf("%s: %d interrupts on stdin, want 1", id, n)
		}
		if e, ok := logged(t, h, id, chat.KindInterrupt, ""); !ok || e.From != tr.top {
			t.Errorf("%s: the interrupt = %+v (%v), want it logged as from %s", id, e, ok, tr.top)
		}
	}
	if n := countIn(h.Stdin(t, tr.sibling), interruptLine); n != 0 {
		t.Errorf("the sibling got %d interrupts, want none", n)
	}

	if stopped, err := stopAs(t, h, tr.top, tr.child); err != "" || len(stopped) != 0 {
		t.Errorf("a stop with no turn open = %q, %q; want nothing stopped", stopped, err)
	}
	if n := countIn(h.Stdin(t, tr.child), interruptLine); n != 1 {
		t.Errorf("a stop with no turn open wrote an interrupt: %d on stdin", n)
	}
}

func TestStopDropsTheMessagesHeldForAThread(t *testing.T) {
	h := apptest.New(t)
	tr := newTree(t, h)
	if _, err := h.ChatSend(tr.child, "ask"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the approval", func() bool { return len(h.ChatPending()) == 1 })
	if a := sendAs(t, h, tr.sibling, `{"thread":"`+tr.child+`","text":"also check the index"}`); !a.Held {
		t.Fatalf("send answered %+v, want it held", a)
	}

	var out struct {
		Stopped []string
		Dropped int
	}
	mustUse(t, h, tr.top, "stop", `{"thread":"`+tr.child+`"}`, &out)
	if !slices.Equal(out.Stopped, []string{tr.child}) || out.Dropped != 1 {
		t.Fatalf("stop = %+v, want the child stopped and one message dropped", out)
	}
	apptest.Eventually(t, "the turn's end", func() bool { return !turnOpen(t, h, tr.child) })
	h.Sessions().Refresh() // a refresh delivers what is held once the thread may take it
	time.Sleep(time.Second)
	if stdinHas(t, h, tr.child, "also check the index") || turnOpen(t, h, tr.child) {
		t.Errorf("stdin = %q: the stopped thread got the held message and started a turn for it", h.Stdin(t, tr.child))
	}
}

func TestStopRefusesAThreadThatIsNotADescendant(t *testing.T) {
	h := apptest.New(t)
	tr := newTree(t, h)
	stranger := threadWith(t, h, "Someone else", session.Read).ID
	working(t, h, tr.top)
	working(t, h, tr.sibling)
	working(t, h, stranger)
	tests := []struct{ name, caller, target string }{
		{"a sibling", tr.child, tr.sibling},
		{"the caller's parent", tr.child, tr.top},
		{"the caller's grandparent", tr.grandchild, tr.top},
		{"the caller itself", tr.child, tr.child},
		{"a thread outside the family", tr.top, stranger},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopped, err := stopAs(t, h, tt.caller, tt.target)
			if want := "Thread " + tt.target + " is not a child of this thread"; err != want || stopped != nil {
				t.Errorf("stop = %q, %q; want %q", stopped, err, want)
			}
		})
	}
	for _, id := range []string{tr.top, tr.sibling, stranger} {
		if n := countIn(h.Stdin(t, id), interruptLine); n != 0 || !turnOpen(t, h, id) {
			t.Errorf("%s got %d interrupts, turn open %v; want its turn left running", id, n, turnOpen(t, h, id))
		}
	}
}
