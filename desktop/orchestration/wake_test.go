package orchestration_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/thread"
)

// resultText is the line of a child's result as the parent's Claude reads it.
func resultText(title, child, outcome, result string, later bool) string {
	return jsonText("APPLICATION_PROMPT: " + prompts.ChildDone(title, child, outcome, result, later, false))
}

// resultLines are the user lines the parent's stand-in claude read that hold a result of the child.
func resultLines(t *testing.T, h *apptest.Harness, parent, child string) []string {
	t.Helper()
	var out []string
	for _, l := range h.Stdin(t, parent) {
		if strings.Contains(l, `"type":"user"`) && strings.Contains(l, "in thread "+child) {
			out = append(out, l)
		}
	}
	return out
}

// results are the user events of the parent's log that are a result of the child.
func results(t *testing.T, h *apptest.Harness, parent, child string) []chat.Event {
	t.Helper()
	var out []chat.Event
	for _, e := range logOf(t, h, parent) {
		if e.Kind == chat.KindUser && e.From == child && e.Result {
			out = append(out, e)
		}
	}
	return out
}

// asleep says whether the thread's row has gone to sleep.
func asleep(t *testing.T, h *apptest.Harness, id string) bool {
	t.Helper()
	s, ok := h.Thread(id)
	return ok && s.State == thread.Settled
}

// quiet waits until each child has gone to sleep, as it does once its result went to its parent, and its parent's turn
// on that result has ended: a message to a child then wakes it.
func quiet(t *testing.T, h *apptest.Harness, children ...string) {
	t.Helper()
	apptest.Eventually(t, "the children asleep and their parents idle", func() bool {
		for _, c := range children {
			if !asleep(t, h, c) || turnOpen(t, h, h.Threads().ParentOf(c)) {
				return false
			}
		}
		return true
	})
}

// withConversation gives the thread a conversation and a transcript, so a wake resumes it: one that a message of the
// thread's names, and the one the stand-in claude's next init names, which Claude writes at its first prompt.
func withConversation(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h.Observe(t, id, "SessionStart", fmt.Sprintf(`{"session_id":"conversation-1","transcript_path":%q}`, transcript))
	ch, err := h.Threads().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	folder, err := filepath.EvalSymlinks(ch.Dir)
	if err != nil {
		t.Fatal(err)
	}
	standIn := agent.Claude{ConfigDir: h.ClaudeConfigDir()}.Transcript(folder, "fake-session")
	if err := os.MkdirAll(filepath.Dir(standIn), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(standIn, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// noMoreResults fails the test when the parent gets another result of the child within a second.
func noMoreResults(t *testing.T, h *apptest.Harness, parent, child string, want int) {
	t.Helper()
	time.Sleep(time.Second)
	if got := resultLines(t, h, parent, child); len(got) != want {
		t.Errorf("the parent read %d results of the child, want %d: %q", len(got), want, got)
	}
}

func TestChildResultWakesItsParent(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"Count the files","title":"Count files","access":"read"}`).ID
	apptest.Eventually(t, "the result on the parent's stdin", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	if lines := resultLines(t, h, parent, c); len(lines) != 1 || !strings.Contains(lines[0], resultText("Count files", c, "completed", "done", false)) {
		t.Errorf("the parent read %q, want one line with %s", lines, resultText("Count files", c, "completed", "done", false))
	}
	apptest.Eventually(t, "the result in the parent's log", func() bool { return len(results(t, h, parent, c)) > 0 })
	if got := results(t, h, parent, c); len(got) != 1 || got[0].Body != "done" || !strings.HasPrefix(got[0].Text, "APPLICATION_PROMPT: Task Count files in thread "+c+" completed:") {
		t.Errorf("the parent's log has %+v, want one result from the child with the body done", got)
	}
	apptest.Eventually(t, "the parent's turn on the result", func() bool { return turnEnded(t, h, parent) })
	noMoreResults(t, h, parent, c, 1)
	if !slices.ContainsFunc(logOf(t, h, parent), func(e chat.Event) bool { return e.Kind == chat.KindMessage }) {
		t.Error("the parent did not reply to the result")
	}
}

func TestChildResultWakesASettledParent(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	// The stand-in takes a second over a task that ends in slow, and the parent sleeps meanwhile.
	c := mustDelegate(t, h, parent, `{"task":"slow","title":"Count files","access":"read"}`).ID
	withConversation(t, h, parent)
	if err := h.SettleThread(parent); err != nil {
		t.Fatal(err)
	}
	if turnEnded(t, h, c) {
		t.Fatal("the child's turn ended before its parent slept")
	}
	apptest.Eventually(t, "the result on the woken parent's stdin", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	if lines := resultLines(t, h, parent, c); len(lines) != 1 || !strings.Contains(lines[0], resultText("Count files", c, "completed", "done", false)) {
		t.Errorf("the parent read %q", lines)
	}
	if r := row(t, h, parent); r.State == thread.Settled || !slices.Contains(tmuxSessions(t, h), parent) {
		t.Errorf("the parent is %s, running %v; want it awake under its own id", r.State, slices.Contains(tmuxSessions(t, h), parent))
	}
	if !slices.Contains(claudeArgs(t, h, parent), "--resume") {
		t.Errorf("the parent woke with %q, want its conversation resumed", claudeArgs(t, h, parent))
	}
	noMoreResults(t, h, parent, c, 1)
}

func TestChildResultIsSentOnceAcrossARestart(t *testing.T) {
	t.Parallel()
	t.Run("between the turn end and the send", func(t *testing.T) {
		h := apptest.New(t)
		parent := newThread(t, h, "Plan the release").ID
		c := mustDelegate(t, h, parent, `{"task":"slow","title":"Count files","access":"read"}`).ID
		h.App.Stop()
		apptest.Eventually(t, "the child's turn to end while the app is closed", func() bool { return turnEnded(t, h, c) })
		if n := len(resultLines(t, h, parent, c)); n != 0 {
			t.Fatalf("the parent read %d results before the restart", n)
		}
		fresh := h.Restart(t)
		apptest.Eventually(t, "the result after the restart", func() bool { return len(resultLines(t, fresh, parent, c)) > 0 })
		noMoreResults(t, fresh, parent, c, 1)
	})
	t.Run("after the send", func(t *testing.T) {
		h := apptest.New(t)
		parent := newThread(t, h, "Plan the release").ID
		c := mustDelegate(t, h, parent, `{"task":"Count the files","title":"Count files","access":"read"}`).ID
		quiet(t, h, c)
		end := slices.IndexFunc(logOf(t, h, c), func(e chat.Event) bool { return e.Kind == chat.KindTurnEnd })
		if got, want := h.Threads().Reported(c), logOf(t, h, c)[end].Seq; got != want {
			t.Errorf("Reported = %d, want the seq of the turn's end, %d", got, want)
		}
		h.App.Stop()
		fresh := h.Restart(t)
		if got := fresh.Threads().Reported(c); got == 0 {
			t.Error("the restart lost Reported")
		}
		// A woken child's follower starts at its last turn_end, whose result went already.
		if _, err := fresh.WakeThread(c); err != nil {
			t.Fatal(err)
		}
		noMoreResults(t, fresh, parent, c, 1)
		if asleep(t, fresh, c) {
			t.Error("the woken child went to sleep again with no turn of its own")
		}
	})
}

func TestWaitingParentGetsNoSecondCopy(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"slow","title":"Count files","access":"read"}`).ID
	if a := waitOn(t, h, parent, c, 30*time.Second); !a.Done || a.Text != "done" {
		t.Fatalf("wait answered %+v, want the result", a)
	}
	apptest.Eventually(t, "the child asleep", func() bool { return asleep(t, h, c) })
	noMoreResults(t, h, parent, c, 0)
	if got := results(t, h, parent, c); len(got) != 0 {
		t.Errorf("the parent's log has %+v, want no result: the wait gave it", got)
	}
	if h.Threads().Reported(c) == 0 {
		t.Error("the result the wait gave is not recorded as given")
	}
}

func TestFailedChildSaysSo(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"this one fails","title":"Count files","access":"read"}`).ID
	want := resultText("Count files", c, "failed: API Error: 500", "half done", false)
	apptest.Eventually(t, "the failure on the parent's stdin", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	if lines := resultLines(t, h, parent, c); len(lines) != 1 || !strings.Contains(lines[0], want) {
		t.Errorf("the parent read %q, want one line with %s", lines, want)
	}
	apptest.Eventually(t, "the child asleep", func() bool { return asleep(t, h, c) })
}

func TestStopTheParentAskedForWakesNoOne(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"keep working","access":"read"}`).ID
	apptest.Eventually(t, "the child's task taken in", func() bool { return stdinHas(t, h, c, "keep working") })
	if stopped, err := stopAs(t, h, parent, c); err != "" || !slices.Equal(stopped, []string{c}) {
		t.Fatalf("stop = %q, %q", stopped, err)
	}
	apptest.Eventually(t, "the child's turn to end", func() bool { return turnEnded(t, h, c) })
	noMoreResults(t, h, parent, c, 0)
	if n := countIn(h.Stdin(t, parent), `"type":"user"`); n != 0 || turnEnded(t, h, parent) {
		t.Errorf("the parent read %d messages and ran a turn %v, want none", n, turnEnded(t, h, parent))
	}
	if asleep(t, h, c) {
		t.Error("the child the parent stopped went to sleep; it is the parent's to send to again")
	}
}

func TestOwnerStopWakesTheParent(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"keep working","title":"Count files","access":"read"}`).ID
	apptest.Eventually(t, "the child's task taken in", func() bool { return stdinHas(t, h, c, "keep working") })
	if err := h.Interrupt(c); err != nil {
		t.Fatal(err)
	}
	want := resultText("Count files", c, "was stopped by the owner", "", false)
	apptest.Eventually(t, "the stop on the parent's stdin", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	if lines := resultLines(t, h, parent, c); len(lines) != 1 || !strings.Contains(lines[0], want) {
		t.Errorf("the parent read %q, want one line with %s", lines, want)
	}
	apptest.Eventually(t, "the child asleep", func() bool { return asleep(t, h, c) })
}

func TestFinishedChildSettles(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"slow","title":"Count files","access":"read"}`).ID
	withConversation(t, h, c)
	quiet(t, h, c)
	if err := os.Remove(filepath.Join(h.ClaudeDir(), strings.ReplaceAll(c, "/", "_")+".args")); err != nil {
		t.Fatal(err)
	}
	if a := sendAs(t, h, parent, `{"thread":"`+c+`","text":"Count the tests too"}`); !a.Woke {
		t.Fatalf("send answered %+v, want the child woken", a)
	}
	if !slices.Contains(claudeArgs(t, h, c), "--resume") {
		t.Errorf("the child woke with %q, want its conversation resumed", claudeArgs(t, h, c))
	}
	want := resultText("Count files", c, "completed", "done", true)
	apptest.Eventually(t, "the later result", func() bool { return len(resultLines(t, h, parent, c)) == 2 })
	if lines := resultLines(t, h, parent, c); !strings.Contains(lines[1], want) {
		t.Errorf("the second result = %s, want %s", lines[1], want)
	}
	quiet(t, h, c)
	noMoreResults(t, h, parent, c, 2)
}

func TestLaterTurnOfAChildIsALaterResult(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	// The child's turn ends while its shell runs in the background, and the shell's end runs a turn of its own.
	c := mustDelegate(t, h, parent, `{"task":"sleep in a background shell","title":"Sleep","access":"read"}`).ID
	if a := waitOn(t, h, parent, c, 30*time.Second); !a.Done || a.Text != "done" {
		t.Fatalf("wait answered %+v, want the first turn's result", a)
	}
	if asleep(t, h, c) {
		t.Fatal("the child went to sleep while its shell ran")
	}
	want := resultText("Sleep", c, "completed", "shell done", true)
	apptest.Eventually(t, "the later turn's result", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	if lines := resultLines(t, h, parent, c); len(lines) != 1 || !strings.Contains(lines[0], want) {
		t.Errorf("the parent read %q, want only the later result %s", lines, want)
	}
	apptest.Eventually(t, "the child asleep once its shell ended", func() bool { return asleep(t, h, c) })
}

func TestChildTurnRaisesNoRepliedToast(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"keep working","access":"read"}`).ID
	for _, id := range []string{c, parent} {
		h.Observe(t, id, "UserPromptSubmit", `{"prompt":"go"}`)
		apptest.Eventually(t, "the turn working", func() bool { return row(t, h, id).State == thread.Working })
		h.Observe(t, id, "Stop", `{}`)
		apptest.Eventually(t, "the turn's end", func() bool { return row(t, h, id).State == thread.Idle })
	}
	if n := h.Rec.CountAttentionFor(c, "replied"); n != 0 {
		t.Errorf("the child's reply raised %d replied alerts, want none: its parent reads it", n)
	}
	if n := h.Rec.CountAttentionFor(parent, "replied"); n != 1 {
		t.Errorf("the parent's reply raised %d replied alerts, want 1", n)
	}
	h.Observe(t, c, "Notification", `{"message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`)
	apptest.Eventually(t, "the child's waiting alert", func() bool { return h.Rec.CountAttentionFor(c, "waiting") > 0 })
}

func TestChildResultIgnoresManualSend(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    prompt_send: manual\n"})
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"Count the files","title":"Count files","access":"read"}`).ID
	apptest.Eventually(t, "the result on the parent's stdin", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	if slices.ContainsFunc(logOf(t, h, parent), func(e chat.Event) bool { return e.Kind == chat.KindDraft }) {
		t.Error("the result was left as a draft")
	}
	if got := results(t, h, parent, c); len(got) != 1 {
		t.Errorf("the parent's log has %d results, want 1 sent", len(got))
	}
}

func TestChildResultWaitsForTheParentsSubagentApproval(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	if _, err := h.ChatSend(parent, "subagent asks"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the subagent's approval after the turn's end", func() bool {
		_, asks := logged(t, h, parent, chat.KindApproval, "sub-1")
		return asks && turnEnded(t, h, parent)
	})
	c := mustDelegate(t, h, parent, `{"task":"Count the files","title":"Count files","access":"read"}`).ID
	apptest.Eventually(t, "the child's turn to end", func() bool { return turnEnded(t, h, c) })
	time.Sleep(time.Second)
	if lines := resultLines(t, h, parent, c); len(lines) != 0 || !h.Threads().HoldsTold(parent) {
		t.Fatalf("the parent read %q while its subagent's approval waited, held %v; want the result held", lines, h.Threads().HoldsTold(parent))
	}
	if asleep(t, h, c) {
		t.Error("the child went to sleep before its result went")
	}
	if err := h.ChatAnswer(parent, "sub-1", true, ""); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the result once the approval is answered", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	apptest.Eventually(t, "the child asleep", func() bool { return asleep(t, h, c) })
	noMoreResults(t, h, parent, c, 1)
}

func TestChildResultWaitsOutAnAccessChangeOfTheParent(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"slow","title":"Count files","access":"read"}`).ID
	ch, err := h.Threads().ChatOf(parent)
	if err != nil {
		t.Fatal(err)
	}
	// The parent's host is held, as a change of access holds it before it restarts the thread, when the child's turn ends.
	if idle, err := host.Hold(context.Background(), ch.Socket); err != nil || !idle {
		t.Fatalf("Hold = %v, %v, want the parent's host held", idle, err)
	}
	apptest.Eventually(t, "the child's turn to end", func() bool { return turnEnded(t, h, c) })
	apptest.Eventually(t, "the result held for the parent", func() bool { return h.Threads().HoldsTold(parent) })
	if lines := resultLines(t, h, parent, c); len(lines) != 0 {
		t.Fatalf("the parent read %q through a held host", lines)
	}
	if err := h.Threads().SetAccess(parent, thread.Full); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the result once the parent runs again", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	if mode := permissionMode(t, h, parent); mode != "bypassPermissions" {
		t.Errorf("the parent runs in %s, want the new access", mode)
	}
	noMoreResults(t, h, parent, c, 1)
}

func TestChildResultRetriesARefusedWakeOfItsParent(t *testing.T) {
	t.Parallel()
	// The tries come at 0, 0.1, 0.3, 0.7 and 1.5 s: the pause starts at retry and doubles.
	const retry = 100 * time.Millisecond
	for _, tt := range []struct {
		name   string
		delete bool // the child is deleted while its result waits
		want   int
	}{
		{"until the parent wakes", false, 1},
		{"until the child is deleted", true, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := apptest.NewWith(t, apptest.Options{RetryFirst: retry})
			repoFixture(t, h)
			parent := threadWith(t, h, "Plan the release", thread.Supervised)
			if parent.Worktree == "" {
				t.Fatalf("the parent has no worktree: %+v", parent)
			}
			c := mustDelegate(t, h, parent.ID, `{"task":"slow","title":"Count files","access":"read","where":"worktree"}`).ID
			if err := h.SettleThread(parent.ID); err != nil {
				t.Fatal(err)
			}
			// A folder that is not the parent's worktree at its path refuses the parent's wake.
			if err := os.RemoveAll(parent.Worktree); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(parent.Worktree, 0o755); err != nil {
				t.Fatal(err)
			}
			apptest.Eventually(t, "the child's turn to end", func() bool { return turnEnded(t, h, c) })
			time.Sleep(1500 * time.Millisecond) // past the first tries
			if lines := resultLines(t, h, parent.ID, c); len(lines) != 0 || !asleep(t, h, parent.ID) {
				t.Fatalf("the parent read %q, asleep %v; want its wake refused", lines, asleep(t, h, parent.ID))
			}
			if h.Threads().Reported(c) != 0 {
				t.Fatal("the result is recorded as given while the parent could not wake for it")
			}
			if tt.delete {
				if err := h.DeleteThread(c, false); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(parent.Worktree); err != nil {
				t.Fatal(err)
			}
			if tt.want > 0 {
				apptest.Eventually(t, "the result once the parent can wake", func() bool { return len(resultLines(t, h, parent.ID, c)) > 0 })
			} else {
				time.Sleep(4 * time.Second) // past the next try, which comes by 1.5 s
				if !asleep(t, h, parent.ID) {
					t.Error("the parent woke for the result of a deleted child")
				}
			}
			noMoreResults(t, h, parent.ID, c, tt.want)
		})
	}
}

func TestDroppedResultGoesAtTheParentsNextTurn(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	top := newThread(t, h, "Plan the release").ID
	parent := mustDelegate(t, h, top, `{"task":"keep working","title":"Check the build"}`).ID
	apptest.Eventually(t, "the parent's task taken in", func() bool { return stdinHas(t, h, parent, "keep working") })
	// The parent waits on the owner in the middle of its turn, so the result is held for it. It waits before the child
	// starts: a child that ended first would give its result to a parent that still takes it.
	h.Observe(t, parent, "Notification", `{"message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`)
	apptest.Eventually(t, "the parent waiting", func() bool { return row(t, h, parent).State == thread.Waiting })
	c := mustDelegate(t, h, parent, `{"task":"Count the files","title":"Count files","access":"read"}`).ID
	apptest.Eventually(t, "the result held for the parent", func() bool { return turnEnded(t, h, c) && h.Threads().HoldsTold(parent) })

	if stopped, err := stopAs(t, h, top, parent); err != "" || !slices.Contains(stopped, parent) {
		t.Fatalf("stop = %q, %q", stopped, err)
	}
	apptest.Eventually(t, "the parent's stopped turn to end", func() bool { return turnEnded(t, h, parent) })
	time.Sleep(time.Second)
	if lines := resultLines(t, h, parent, c); len(lines) != 0 {
		t.Fatalf("the parent read %q after the stop dropped the result", lines)
	}
	if got := h.Threads().Reported(c); got != 0 {
		t.Fatalf("Reported = %d after the result was dropped, want 0", got)
	}
	if asleep(t, h, c) {
		t.Error("the child went to sleep before its result went")
	}

	if a := sendAs(t, h, top, `{"thread":"`+parent+`","text":"Carry on"}`); a.ID == "" {
		t.Fatalf("send answered %+v, want the message sent", a)
	}
	apptest.Eventually(t, "the result at the parent's next turn", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	apptest.Eventually(t, "the result recorded as given", func() bool { return h.Threads().Reported(c) > 0 })
	noMoreResults(t, h, parent, c, 1)
}

func TestWaitAfterTheResultWentSaysSo(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"Count the files","title":"Count files","access":"read"}`).ID
	quiet(t, h, c)
	if n := len(resultLines(t, h, parent, c)); n != 1 {
		t.Fatalf("the parent read %d results, want 1", n)
	}
	a := waitOn(t, h, parent, c, 5*time.Second)
	if want := "Thread " + c + "'s result of this turn is sent to you as a message instead."; !a.Done || a.Text != want {
		t.Errorf("wait answered %+v, want done with %q", a, want)
	}
	other := newThread(t, h, "Watch the release").ID
	if a := waitOn(t, h, other, c, 5*time.Second); !a.Done || !strings.Contains(a.Text, "done") {
		t.Errorf("another thread's wait answered %+v, want the child's last reply", a)
	}
}

func TestWaitWhileTheResultIsHeldSaysSo(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	if _, err := h.ChatSend(parent, "subagent asks"); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the subagent's approval after the turn's end", func() bool {
		_, asks := logged(t, h, parent, chat.KindApproval, "sub-1")
		return asks && turnEnded(t, h, parent)
	})
	c := mustDelegate(t, h, parent, `{"task":"Count the files","title":"Count files","access":"read"}`).ID
	apptest.Eventually(t, "the result held for the parent", func() bool { return turnEnded(t, h, c) && h.Threads().HoldsTold(parent) })
	// The message took the result before the wait came, so the wait does not give it too.
	a := waitOn(t, h, parent, c, 5*time.Second)
	if want := "Thread " + c + "'s result of this turn is sent to you as a message instead."; !a.Done || a.Text != want {
		t.Errorf("wait answered %+v, want done with %q", a, want)
	}
	if err := h.ChatAnswer(parent, "sub-1", true, ""); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the result once the approval is answered", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	noMoreResults(t, h, parent, c, 1)
}

func TestChildPutToSleepMidTurnTellsItsParent(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release").ID
	c := mustDelegate(t, h, parent, `{"task":"keep working","title":"Count files","access":"read"}`).ID
	apptest.Eventually(t, "the child's task taken in", func() bool { return stdinHas(t, h, c, "keep working") })
	// The owner has the child open, so its chat is followed when they put it to sleep.
	if err := h.ChatWatch(c); err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(c); err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the sleep on the parent's stdin", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
	if lines := resultLines(t, h, parent, c); len(lines) != 1 || !strings.Contains(lines[0], "Task Count files in thread "+c+" was put to sleep by the owner:") {
		t.Errorf("the parent read %q, want one result that says the owner put the child to sleep", lines)
	}
	if h.Threads().Reported(c) == 0 {
		t.Error("the result of the sleep is not recorded as given")
	}
	noMoreResults(t, h, parent, c, 1)
}

func TestChildPutToSleepUnfollowedTellsItsParent(t *testing.T) {
	t.Parallel()
	t.Run("before its follower starts", func(t *testing.T) {
		h := apptest.New(t)
		parent := newThread(t, h, "Plan the release").ID
		c := mustDelegate(t, h, parent, `{"task":"keep working","title":"Count files","access":"read"}`).ID
		apptest.Eventually(t, "the child's task taken in", func() bool { return stdinHas(t, h, c, "keep working") })
		// The app's own settle, as at the end of a PR, says no more than that.
		if err := h.Threads().Settle(c); err != nil {
			t.Fatal(err)
		}
		apptest.Eventually(t, "the sleep on the parent's stdin", func() bool { return len(resultLines(t, h, parent, c)) > 0 })
		if lines := resultLines(t, h, parent, c); len(lines) != 1 || !strings.Contains(lines[0], "Task Count files in thread "+c+" was put to sleep:") {
			t.Errorf("the parent read %q, want one result that says the child was put to sleep, not by whom", lines)
		}
		noMoreResults(t, h, parent, c, 1)
	})
	t.Run("while the app is closed", func(t *testing.T) {
		h := apptest.New(t)
		parent := newThread(t, h, "Plan the release").ID
		c := mustDelegate(t, h, parent, `{"task":"keep working","title":"Count files","access":"read"}`).ID
		apptest.Eventually(t, "the child's task taken in", func() bool { return stdinHas(t, h, c, "keep working") })
		ch, err := h.Threads().ChatOf(c)
		if err != nil {
			t.Fatal(err)
		}
		h.App.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := host.PlanStop(ctx, ch.Socket, chat.StopSettle, ""); err != nil {
			t.Fatal(err)
		}
		if err := exec.Command("tmux", "-L", h.Socket, "kill-session", "-t", "="+c).Run(); err != nil {
			t.Fatal(err)
		}
		if err := host.Gone(ctx, ch.Socket); err != nil {
			t.Fatal(err)
		}
		// The app takes a gone thread's state file in as asleep once it is not fresh.
		rec, err := bus.ReadState(h.State, c)
		if err != nil {
			t.Fatal(err)
		}
		rec.At = rec.At.Add(-time.Minute)
		if err := bus.WriteState(h.State, rec); err != nil {
			t.Fatal(err)
		}
		if n := len(resultLines(t, h, parent, c)); n != 0 {
			t.Fatalf("the parent read %d results before the restart", n)
		}
		fresh := h.Restart(t)
		apptest.Eventually(t, "the sleep on the parent's stdin after the restart", func() bool { return len(resultLines(t, fresh, parent, c)) > 0 })
		if lines := resultLines(t, fresh, parent, c); len(lines) != 1 || !strings.Contains(lines[0], "Task Count files in thread "+c+" was put to sleep:") {
			t.Errorf("the parent read %q, want one result that says the child was put to sleep, not by whom", lines)
		}
		noMoreResults(t, fresh, parent, c, 1)
		fresh.App.Stop()
		noMoreResults(t, fresh.Restart(t), parent, c, 1)
	})
}
