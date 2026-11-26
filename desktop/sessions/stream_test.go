package sessions_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
)

// say sends text to the thread as the owner's message.
func say(t *testing.T, h *apptest.Harness, id, text string) {
	t.Helper()
	if _, err := h.ChatSend(id, text); err != nil {
		t.Fatal(err)
	}
}

func TestBranchIsReadAgainAfterAToolCall(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	if got := gitStatus(t, h, s.ID); got != (sessions.BranchStatus{Base: "main", Ahead: 1}) {
		t.Fatalf("a pushed branch = %+v", got)
	}

	say(t, h, s.ID, "tool wait")
	eventually(t, "the tool call", func() bool { return stateOf(h, s.ID) == "working" })
	if err := os.WriteFile(filepath.Join(wt, "README"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.Proceed(t, s.ID)
	// The turn goes on, so only the tool's result can have the row read again.
	eventually(t, "the tool call's change in the row", func() bool { got, _ := h.Session(s.ID); return got.Git != nil && got.Git.Uncommitted == 1 })
	if got := stateOf(h, s.ID); got != "working" {
		t.Errorf("state = %s, want the turn to run still", got)
	}
	h.Finish(t, s.ID)
	eventually(t, "the turn's end", func() bool { return stateOf(h, s.ID) == "idle" })
}

func TestPRIsLookedUpWhenATurnEnds(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := worktreeThread(h, t)
	say(t, h, s.ID, "tool wait")
	eventually(t, "the tool call", func() bool { return stateOf(h, s.ID) == "working" })
	h.GH.SetPR(prJSON("OPEN", true, "[]", 0, 0)) // the agent opens it during the turn
	h.Proceed(t, s.ID)
	time.Sleep(500 * time.Millisecond)
	if got, _ := h.Session(s.ID); got.PR != nil {
		t.Fatalf("the PR shows before the turn ends: %+v", got.PR)
	}
	h.Finish(t, s.ID)
	eventually(t, "the PR the agent opened during the turn", hasPR(h, s.ID))
}

func TestPromptIsRecordedInTheActivity(t *testing.T) {
	h := newHarness(t)
	s, err := h.Sessions().Create("#4 work", "", false, 4, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	say(t, h, s.ID, "first")
	eventually(t, "the first prompt in the ledger", func() bool {
		led, err := h.Ledger(30, "")
		return err == nil && led.Totals.Prompts == 1 && stateOf(h, s.ID) == "idle"
	})
	say(t, h, s.ID, "second")
	eventually(t, "two prompts in the ledger", func() bool {
		led, err := h.Ledger(30, "")
		return err == nil && led.Totals.Prompts == 2
	})
}

func TestRestartObservesNoEventTwice(t *testing.T) {
	h := newHarness(t)
	s, err := h.Sessions().Create("work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	say(t, h, s.ID, "tool")
	eventually(t, "the first turn", func() bool {
		led, err := h.Ledger(30, "")
		return err == nil && led.Totals.Prompts == 1 && stateOf(h, s.ID) == "idle"
	})
	before, err := bus.ReadState(h.State, s.ID)
	if err != nil || before.Observed == 0 {
		t.Fatalf("state = %+v, %v; want the seq it took in", before, err)
	}

	// The app is closed while the thread takes a message, and open again for the rest of that turn and the next.
	h.App.Stop()
	if err := sendToHost(h, s.ID, "while closed"); err != nil {
		t.Fatal(err)
	}
	chatLog, _ := hostFiles(h, s.ID)
	eventually(t, "the turn that ran while the app was closed", func() bool {
		data, err := os.ReadFile(chatLog)
		return err == nil && strings.Count(string(data), `"kind":"turn_end"`) == 2
	})
	second := h.Restart(t)
	eventually(t, "the missed turn taken in", func() bool {
		led, err := second.Ledger(30, "")
		rec, _ := bus.ReadState(h.State, s.ID)
		return err == nil && led.Totals.Prompts == 2 && rec.Observed > before.Observed
	})
	time.Sleep(time.Second) // a follower that took in an event twice has done so by now
	led, err := second.Ledger(30, "")
	if err != nil || led.Totals.Prompts != 2 {
		t.Errorf("prompts = %+v, %v; want each counted once", led.Totals, err)
	}
	after, err := bus.ReadState(h.State, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The missed turn is a prompt that works and a reply that ends it; the earlier turns add nothing.
	if added := len(after.Changes) - len(before.Changes); added != 2 {
		t.Errorf("the state timeline grew by %d changes, want 2: %+v", added, after.Changes)
	}
	if got := stateOf(second, s.ID); got != "idle" {
		t.Errorf("state = %s, want idle", got)
	}
}
