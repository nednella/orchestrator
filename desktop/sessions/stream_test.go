package sessions_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/session"
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

// afterStart waits until the thread's follower has taken in its start, and gives a time after it.
func afterStart(t *testing.T, h *apptest.Harness, id string) time.Time {
	t.Helper()
	eventually(t, "the thread's start taken in", func() bool {
		rec, err := bus.ReadState(h.State, id)
		return err == nil && rec.Observed > 0
	})
	var rec session.Record
	eventually(t, "a time after the record's", func() bool {
		rec, _ = bus.ReadState(h.State, id)
		return time.Now().After(rec.At.Add(time.Millisecond))
	})
	return time.Now()
}

// steady waits until get gives the same value for a while, and returns it.
func steady(t *testing.T, get func() int64) int64 {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	last, same := get(), 0
	for same < 25 {
		if time.Now().After(deadline) {
			t.Fatal("the value never settled")
		}
		time.Sleep(10 * time.Millisecond)
		if now := get(); now == last {
			same++
		} else {
			last, same = now, 0
		}
	}
	return last
}

// observe gives the thread the events as its follower does.
func observe(h *apptest.Harness, id string, evs ...session.Event) { h.Sessions().Observe(id, evs...) }

func TestAnEventOlderThanTheRecordChangesNothing(t *testing.T) {
	h := newHarness(t)
	s, err := h.Sessions().Create("work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	at := afterStart(t, h, s.ID)
	observe(h, s.ID, session.Event{Name: "UserPromptSubmit", Seq: 100, At: at})
	observe(h, s.ID, session.Event{Name: "Stop", Seq: 101, At: at.Add(10 * time.Millisecond)})
	eventually(t, "idle", func() bool { return stateOf(h, s.ID) == "idle" })

	// A tool call the log dates before the turn's end comes late: it must not make the thread work again.
	observe(h, s.ID, session.Event{Name: "PreToolUse", Seq: 102, At: at.Add(5 * time.Millisecond), Tool: "Bash"})
	rec, err := bus.ReadState(h.State, s.ID)
	if err != nil || rec.Event != "Stop" || rec.State != session.Idle || rec.Observed != 101 || stateOf(h, s.ID) != "idle" {
		t.Errorf("record = %+v, %v, row %s; want the Stop's, idle, and the seq 101", rec, err, stateOf(h, s.ID))
	}
	// One at the same time as the latest is taken in.
	observe(h, s.ID, session.Event{Name: "UserPromptSubmit", Seq: 103, At: at.Add(10 * time.Millisecond)})
	if got := stateOf(h, s.ID); got != "working" {
		t.Errorf("state = %s after an event at the record's time, want working", got)
	}
}

func TestAnEventWithNoSeqIsTakenInAndTakesNoneBack(t *testing.T) {
	h := newHarness(t)
	s, err := h.Sessions().Create("work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	at := afterStart(t, h, s.ID)
	observe(h, s.ID, session.Event{Name: "UserPromptSubmit", Seq: 100, At: at})
	observe(h, s.ID, session.Event{Name: "Stop", Seq: 101, At: at.Add(10 * time.Millisecond)})
	eventually(t, "idle", func() bool { return stateOf(h, s.ID) == "idle" })

	before, err := bus.ReadState(h.State, s.ID)
	if err != nil || before.Observed != 101 {
		t.Fatalf("record = %+v, %v; want the seq 101", before, err)
	}
	// An event with no time is dated now, which must come after the Stop.
	eventually(t, "a time after the record's", func() bool { return time.Now().After(before.At.Add(time.Millisecond)) })
	observe(h, s.ID, session.Event{Name: "PreToolUse", Tool: "Edit"})
	eventually(t, "working at an event with no seq and no time", func() bool { return stateOf(h, s.ID) == "working" })
	if rec, err := bus.ReadState(h.State, s.ID); err != nil || rec.Observed != before.Observed || rec.Tool != "Edit" {
		t.Errorf("record = %+v, %v; want Observed %d as before the event, and the tool", rec, err, before.Observed)
	}
	// An event the record holds already, by its seq, changes nothing.
	observe(h, s.ID, session.Event{Name: "Notification", NotificationType: "permission_prompt", Seq: 101, At: time.Now()})
	if got := stateOf(h, s.ID); got != "working" {
		t.Errorf("state = %s after an event the record holds, want working", got)
	}
}

func TestABatchOfEventsReadsUsageOnce(t *testing.T) {
	h := newHarness(t)
	s, err := h.Sessions().Create("work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	transcript := writtenTranscript(t)
	at := afterStart(t, h, s.ID)
	batch := []session.Event{{Name: "UserPromptSubmit", Seq: 200, At: at}, {Name: session.Init, Seq: 201, At: at, Conversation: "c1", Transcript: transcript}}
	for i := range 6 {
		batch = append(batch, session.Event{Name: "PreToolUse", Seq: int64(202 + 2*i), At: at.Add(time.Duration(i) * time.Millisecond), Tool: "Bash"},
			session.Event{Name: "PostToolUse", Seq: int64(203 + 2*i), At: at.Add(time.Duration(i) * time.Millisecond), Tool: "Bash"})
	}
	before := h.Sessions().UsageReads()
	observe(h, s.ID, batch...)
	eventually(t, "the usage read", func() bool { return h.Sessions().UsageReads() > before })
	if got := steady(t, h.Sessions().UsageReads) - before; got != 1 {
		t.Errorf("%d events read the usage %d times, want once", len(batch), got)
	}
	if rec, err := bus.ReadState(h.State, s.ID); err != nil || rec.Observed != int64(213) || rec.Transcript != transcript {
		t.Errorf("record = %+v, %v; want every event of the batch taken in", rec, err)
	}
}

func TestABatchThatEndsATurnAndStartsAnotherRecordsItsUsageAndLooksUpThePR(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	transcript := writtenTranscript(t)
	appendLines(t, transcript, assistantLine("msg_1", time.Now().Add(100*time.Millisecond), 1000, 100))
	h.GH.SetPRFor("issue-7", "[]")
	at := afterStart(t, h, s.ID)
	batch := []session.Event{
		{Name: "UserPromptSubmit", Seq: 300, At: at},
		{Name: session.Init, Seq: 301, At: at, Conversation: "c1", Transcript: transcript},
		{Name: "Stop", Seq: 302, At: at.Add(time.Millisecond)},
		{Name: "UserPromptSubmit", Seq: 303, At: at.Add(2 * time.Millisecond)},
	}
	before := h.GH.Calls("pr list --head issue-7")
	observe(h, s.ID, batch...)
	eventually(t, "the turn's usage in the event log", func() bool {
		events := h.Events(t, "usage")
		return len(events) == 1 && events[0].Tokens.Input == 1000
	})
	eventually(t, "the PR looked up", func() bool { return h.GH.Calls("pr list --head issue-7") > before })
	steady(t, func() int64 { return int64(h.GH.Calls("pr list --head issue-7")) })
	if got := h.GH.Calls("pr list --head issue-7") - before; got != 1 {
		t.Errorf("the PR was looked up %d times, want once", got)
	}
	if got := stateOf(h, s.ID); got != "working" {
		t.Errorf("state = %s, want the next turn's work", got)
	}
}

func TestABatchWithAToolResultReadsTheBranch(t *testing.T) {
	for _, tt := range []struct {
		name string
		then session.Event
	}{
		{"then the turn's end", session.Event{Name: "Stop", Seq: 402}},
		{"then the next prompt", session.Event{Name: "UserPromptSubmit", Seq: 402}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			repoFixture(t, h)
			s := issueThread(h, t, 7)
			pushWork(t, s.Worktree)
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
			h.RefreshPRs()
			if got := gitStatus(t, h, s.ID); got != (sessions.BranchStatus{Base: "main", Ahead: 1}) {
				t.Fatalf("a pushed branch = %+v", got)
			}
			if err := os.WriteFile(filepath.Join(s.Worktree, "README"), []byte("changed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			at := afterStart(t, h, s.ID)
			tt.then.At = at.Add(time.Millisecond)
			observe(h, s.ID,
				session.Event{Name: "PreToolUse", Seq: 400, At: at, Tool: "Edit"},
				session.Event{Name: "PostToolUse", Seq: 401, At: at, Tool: "Edit"},
				tt.then)
			eventually(t, "the branch read again", func() bool { got, _ := h.Session(s.ID); return got.Git != nil && got.Git.Uncommitted == 1 })
		})
	}
}
