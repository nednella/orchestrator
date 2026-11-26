package sessions_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/activity"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/session"
)

// ended is a session that had the conversation abc and has settled.
func ended(t *testing.T, h *apptest.Harness, s sessions.Session) sessions.Session {
	t.Helper()
	h.Observe(t, s.ID, "Stop", fmt.Sprintf(`{"session_id":"abc","transcript_path":%q}`, writtenTranscript(t)))
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	var got sessions.Session
	eventually(t, "the session settled", func() bool { got, _ = h.Session(s.ID); return got.State == "settled" })
	return got
}

// talkedThenSettled is a thread that had one turn, "hi", in the conversation fake-session, the stand-in's, and has
// settled.
func talkedThenSettled(t *testing.T, h *apptest.Harness) sessions.Session {
	t.Helper()
	s, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "hi"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first turn's end", func() bool { return turnEnds(h, s.ID) == 1 })
	h.Observe(t, s.ID, "Stop", fmt.Sprintf(`{"session_id":"fake-session","transcript_path":%q}`, writtenTranscript(t)))
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	var got sessions.Session
	eventually(t, "the thread settled", func() bool { got, _ = h.Session(s.ID); return got.State == "settled" })
	return got
}

func turnEnds(h *apptest.Harness, id string) int {
	chatLog, _ := hostFiles(h, id)
	events, _ := chat.ReadLog(chatLog, 0)
	n := 0
	for _, e := range events {
		if e.Kind == chat.KindTurnEnd {
			n++
		}
	}
	return n
}

// resumes says whether the thread's claude was last started to resume the conversation.
func resumes(t *testing.T, h *apptest.Harness, id, conversation string) func() bool {
	return func() bool {
		args := h.Args(t, id)
		i := slices.Index(args, "--resume")
		return i >= 0 && i+1 < len(args) && args[i+1] == conversation
	}
}

// assertOneChat checks that the thread's history counts from seq 1 with no seq missing or twice, and that its user
// messages are users, in order.
func assertOneChat(t *testing.T, h *apptest.Harness, id string, users ...string) {
	t.Helper()
	events, err := h.ChatHistory(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	var said []string
	for i, e := range events {
		if e.Seq != int64(i+1) {
			t.Fatalf("seq %d at %d of the history, want %d: %+v", e.Seq, i, i+1, events)
		}
		if e.Kind == chat.KindUser {
			said = append(said, e.Text)
		}
	}
	if !slices.Equal(said, users) {
		t.Errorf("user messages = %q, want %q", said, users)
	}
}

func TestWakeKeepsTheThreadID(t *testing.T) {
	h := apptest.New(t)
	before := talkedThenSettled(t, h)

	if _, err := h.ChatSend(before.ID, "again"); err != nil {
		t.Fatal(err)
	}
	woken, ok := h.Session(before.ID)
	if !ok || woken.State == "settled" || woken.N != before.N || woken.CreatedAt != before.CreatedAt || woken.WokeAt <= before.CreatedAt {
		t.Fatalf("woken row = %+v (%v), from %+v", woken, ok, before)
	}
	if list := h.Sessions().List(); len(list) != 1 {
		t.Errorf("rows = %+v, want the one thread", list)
	}
	eventually(t, "claude resuming the conversation", resumes(t, h, before.ID, "fake-session"))
	eventually(t, "the second turn's end", func() bool { return turnEnds(h, before.ID) == 2 })
	assertOneChat(t, h, before.ID, "hi", "again")
}

func TestWakeAfterTheAppRestarts(t *testing.T) {
	h := apptest.New(t)
	before := talkedThenSettled(t, h)
	saved, err := bus.ReadState(h.State, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	h.App.Stop()

	second := h.Restart(t)
	eventually(t, "the settled row", func() bool { got, _ := second.Session(before.ID); return got.State == "settled" })
	if _, err := second.ChatSend(before.ID, "again"); err != nil {
		t.Fatal(err)
	}
	if woken, _ := second.Session(before.ID); woken.State == "settled" || woken.N != before.N || woken.CreatedAt != before.CreatedAt {
		t.Fatalf("woken row = %+v, from %+v", woken, before)
	}
	eventually(t, "claude resuming the conversation", resumes(t, second, before.ID, "fake-session"))
	eventually(t, "the second turn's end", func() bool { return turnEnds(second, before.ID) == 2 })
	assertOneChat(t, second, before.ID, "hi", "again")

	rec, err := bus.ReadState(second.State, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Session != before.ID || rec.Title != "work" || rec.Created != saved.Created || rec.Woke <= rec.Created || rec.Conversation != "fake-session" {
		t.Errorf("state file = %+v, before the wake %+v", rec, saved)
	}
	if n := len(saved.Changes); len(rec.Changes) <= n || !slices.Equal(rec.Changes[:n-1], saved.Changes[:n-1]) || rec.Changes[len(rec.Changes)-1].State != session.Idle {
		t.Errorf("history = %+v, want the one before the wake %+v, then idle", rec.Changes, saved.Changes)
	}
}

// settledWhileClosed quits the app, ends the session as a computer restart would, and opens the app again a minute later.
func settledWhileClosed(t *testing.T, h *apptest.Harness, id string) *apptest.Harness {
	t.Helper()
	h.Observe(t, id, "Stop", fmt.Sprintf(`{"session_id":"abc","transcript_path":%q}`, writtenTranscript(t)))
	h.App.Stop()
	if err := exec.Command("tmux", "-L", h.Socket, "kill-session", "-t", "="+id).Run(); err != nil {
		t.Fatal(err)
	}
	rec, err := bus.ReadState(h.State, id)
	if err != nil {
		t.Fatal(err)
	}
	rec.At = rec.At.Add(-time.Minute)
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}
	return h.Restart(t)
}

func TestThreadThatSettledWhileTheAppWasClosedWakesAsItself(t *testing.T) {
	for _, tt := range []struct {
		name  string
		start func(h *apptest.Harness) (sessions.Session, error)
	}{
		{"read", func(h *apptest.Harness) (sessions.Session, error) { return h.NewThread("ask", "", "", "", "read") }},
		{"issue task", func(h *apptest.Harness) (sessions.Session, error) {
			return h.Sessions().Create(h.Sessions().Current(), "#12 work", "", false, 12, session.Supervised)
		}},
		{"renamed task", func(h *apptest.Harness) (sessions.Session, error) {
			s, err := h.NewSession("work", "")
			if err != nil {
				return s, err
			}
			if err := h.RenameSession(s.ID, "better"); err != nil {
				return s, err
			}
			s, _ = h.Session(s.ID)
			return s, nil
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := apptest.New(t)
			old, err := tt.start(h)
			if err != nil {
				t.Fatal(err)
			}
			fresh := settledWhileClosed(t, h, old.ID)
			var gone sessions.Session
			eventually(t, "the settled row", func() bool { gone, _ = fresh.Session(old.ID); return gone.State == "settled" })
			if gone.Title != old.Title || gone.Issue != old.Issue || gone.Access != old.Access {
				t.Errorf("settled row %+v of %+v", gone, old)
			}
			woken, err := fresh.WakeThread(old.ID)
			if err != nil {
				t.Fatal(err)
			}
			if woken.ID != old.ID || woken.Title != old.Title || woken.Issue != old.Issue || woken.Access != old.Access || woken.State == "settled" {
				t.Errorf("woken %+v from %+v", woken, old)
			}
			eventually(t, "claude resuming the conversation", resumes(t, fresh, old.ID, "abc"))
			if args := fresh.Args(t, old.ID); (old.Access == session.Read) != slices.Contains(args, "plan") {
				t.Errorf("args of a woken %s thread: %q", old.Access, args)
			}
		})
	}
}

func TestWakeWithoutAConversationStartsAfresh(t *testing.T) {
	h := apptest.New(t)
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	claudeArgs(t, h, s.ID)
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { got, _ := h.Session(s.ID); return got.State == "settled" })
	args := filepath.Join(h.ClaudeDir(), strings.ReplaceAll(s.ID, "/", "_")+".args")
	if err := os.Remove(args); err != nil {
		t.Fatal(err)
	}
	if _, err := h.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "claude started again", func() bool { return exists(args) })
	if args := claudeArgs(t, h, s.ID); slices.Contains(args, "--resume") {
		t.Errorf("args = %q, want no conversation to resume", args)
	}
}

func TestPRReviewWakesASettledThread(t *testing.T) {
	for _, tt := range []struct {
		name, extra string
		draft       bool
	}{
		{"auto sends", "", false},
		{"manual drafts", "    session_prompt_send: manual\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n" + tt.extra})
			repoFixture(t, h)
			s := issueSession(h, t, 12)
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
			h.RefreshPRs()
			if err := h.SettleThread(s.ID); err != nil {
				t.Fatal(err)
			}
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
			h.RefreshPRs()
			eventually(t, "the thread woken", func() bool { return stateOf(h, s.ID) == "idle" })
			if list := h.Sessions().List(); len(list) != 1 {
				t.Errorf("rows = %+v, want the one thread", list)
			}
			if tt.draft {
				eventually(t, "the draft", func() bool { return chatLogHas(h, s.ID, "draft", "/address-review 12") })
				time.Sleep(300 * time.Millisecond)
				if users := userLines(t, h, s.ID); len(users) != 0 {
					t.Errorf("user lines on stdin = %v, want none", users)
				}
				return
			}
			eventually(t, "the prompt on stdin", func() bool {
				users := userLines(t, h, s.ID)
				return len(users) == 1 && strings.Contains(users[0], "/address-review 12")
			})
			if chatLogHas(h, s.ID, "draft", "/address-review 12") {
				t.Error("an auto wake was left as a draft")
			}
		})
	}
}

func TestIssueStartWakesTheSettledThread(t *testing.T) {
	h := apptest.New(t)
	s, err := h.StartIssue(7)
	if err != nil {
		t.Fatal(err)
	}
	ships := func() int {
		n := 0
		for _, line := range userLines(t, h, s.ID) {
			if strings.Contains(line, "/ship 7") {
				n++
			}
		}
		return n
	}
	eventually(t, "the action's command on stdin", func() bool { return ships() == 1 })
	ended(t, h, s)

	got, err := h.StartIssue(7)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != s.ID || got.State == "settled" {
		t.Errorf("start of a settled thread's issue = %+v, want %s woken", got, s.ID)
	}
	eventually(t, "the action's command on the woken thread's stdin", func() bool { return ships() == 2 })
	if list := h.Sessions().List(); len(list) != 1 {
		t.Errorf("rows = %+v, want the one thread", list)
	}
}

func TestPRBeforeTheLastWakeIsAnEarlierWakes(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	ended(t, h, s)
	closed := time.Now()
	time.Sleep(2 * time.Millisecond) // the wake must come after the close to the millisecond
	if _, err := h.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}

	h.GH.SetPR(closedAt(prJSON("CLOSED", false, "[]", 0, 0), closed))
	h.RefreshPRs()
	h.RefreshPRs()
	if got, _ := h.Session(s.ID); got.Cleanup != "" || got.PR != nil || got.State == "settled" || !exists(wt) || len(h.Cleanups()) != 0 {
		t.Errorf("woken thread = %+v, worktree there %v, log %+v", got, exists(wt), h.Cleanups())
	}
}

func TestWokenThreadIsCleanedUpAgain(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	pushWork(t, s.Worktree)
	openThenMerge(h)
	eventually(t, "the first clean-up", cleanedUp(h, s.ID))

	if _, err := h.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}
	h.GH.SetPR(strings.Replace(prJSON("OPEN", false, "[]", 0, 0), `"number":12`, `"number":13`, 1))
	h.RefreshPRs()
	if got, _ := h.Session(s.ID); got.PR == nil || got.PR.Number != 13 || got.State == "settled" {
		t.Fatalf("woken thread with a new PR = %+v", got)
	}
	h.GH.SetPR(strings.Replace(prJSON("MERGED", false, "[]", 0, 0), `"number":12`, `"number":13`, 1))
	h.RefreshPRs()
	eventually(t, "the second clean-up", func() bool { return len(h.Cleanups()) == 2 && cleanedUp(h, s.ID)() })
	if log := h.Cleanups(); log[0].PR != 13 || log[0].Status != "done" {
		t.Errorf("log = %+v", log)
	}
}

func TestWokenThreadKeepsItsCost(t *testing.T) {
	h := newHarness(t)
	s, err := h.Sessions().Create(h.Sessions().Current(), "#4 work", "", false, 4, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	transcript := writtenTranscript(t)
	stop := fmt.Sprintf(`{"session_id":"abc","transcript_path":%q}`, transcript)
	appendLines(t, transcript, assistantLine("msg_1", time.Now().Add(100*time.Millisecond), 1000, 100))
	h.Observe(t, s.ID, "Stop", stop)
	eventually(t, "the first turn's usage in the event log", func() bool { return len(h.Events(t, activity.Usage)) == 1 })
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { return stateOf(h, s.ID) == "settled" })

	if _, err := h.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}
	appendLines(t, transcript, assistantLine("msg_2", time.Now().Add(time.Second), 10, 1))
	h.Observe(t, s.ID, "UserPromptSubmit", `{"prompt":"more"}`)
	h.Observe(t, s.ID, "Stop", stop)
	eventually(t, "both turns' tokens on the row", func() bool { row, _ := h.Session(s.ID); return row.Tokens == 1111 })
	eventually(t, "the wake's usage in the event log", func() bool { return len(h.Events(t, activity.Usage)) == 2 })
	h.App.Stop()

	second := h.Restart(t)
	eventually(t, "both turns' tokens after a restart", func() bool { row, _ := second.Session(s.ID); return row.Tokens == 1111 })
	led, err := second.Ledger(30, "")
	if err != nil {
		t.Fatal(err)
	}
	if led.Usage.Totals.Tokens != 1111 || len(led.Work) != 1 || led.Work[0].Tokens != 1111 {
		t.Errorf("usage = %+v, work = %+v: a turn was counted twice or lost", led.Usage.Totals, led.Work)
	}
}

func TestWakeOfALiveThreadFails(t *testing.T) {
	h := apptest.New(t)
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "Thread " + s.ID + " is not asleep"
	if _, err := h.WakeThread(s.ID); err == nil || err.Error() != want {
		t.Errorf("wake of a live thread = %v, want %q", err, want)
	}
	if list := h.Sessions().List(); len(list) != 1 || list[0].State == "settled" {
		t.Errorf("rows = %+v", list)
	}
}

func TestWokenThreadShowsItsPRAtOnce(t *testing.T) {
	h := apptest.New(t)
	repoFixture(t, h)
	s, err := h.Sessions().Create(h.Sessions().Current(), "#12 work", "", false, 12, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	ended(t, h, s)
	woken, err := h.WakeThread(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if woken.PR == nil || woken.PR.Number != 12 || woken.Branch != "issue-12" {
		t.Errorf("woken row = %+v, want its PR", woken)
	}
}

func TestIssueStartReportsACommandTheWokenThreadRefused(t *testing.T) {
	h := apptest.New(t)
	s, err := h.StartIssue(7)
	if err != nil {
		t.Fatal(err)
	}
	ended(t, h, s)

	tooLong := strings.Repeat("x", 1<<20) // over what the host reads as one request
	_, did, err := h.Sessions().CreateIssue(h.Sessions().Current(), "#7 work", "/ship 7", 7, project.Action{Name: "Work"}, nil, "", sessions.StartOptions{Prompt: tooLong})
	want := "Thread " + s.ID + " woke but did not take the issue's command"
	if err == nil || err.Error() != want || did != sessions.Woken {
		t.Errorf("CreateIssue = %v, %v; want %q", did, err, want)
	}
	if got, _ := h.Session(s.ID); got.State == "settled" {
		t.Errorf("the thread did not wake: %+v", got)
	}
}

func TestTwoMessagesAtOnceWakeTheThreadOnce(t *testing.T) {
	h := apptest.New(t)
	s := talkedThenSettled(t, h)

	errs := make(chan error, 2)
	for _, text := range []string{"one", "two"} {
		go func() {
			_, err := h.ChatSend(s.ID, text)
			errs <- err
		}()
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	eventually(t, "both messages on stdin", func() bool {
		users := strings.Join(userLines(t, h, s.ID), "\n")
		return strings.Contains(users, `"text":"one"`) && strings.Contains(users, `"text":"two"`)
	})
	if starts := h.Events(t, activity.SessionStart); len(starts) != 2 {
		t.Errorf("session_start = %+v, want the first start and one wake", starts)
	}
	if list := h.Sessions().List(); len(list) != 1 || list[0].State == "settled" {
		t.Errorf("rows = %+v, want the one thread awake", list)
	}
}

func TestPRReviewWakesASettledThreadWithNoIssue(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n"})
	repoFixture(t, h)
	s, err := h.NewThreadIn("my work", "", "supervised", "worktree")
	if err != nil {
		t.Fatal(err)
	}
	if s.Issue != 0 || s.Branch != "my-work" {
		t.Fatalf("thread = %+v, want one with no issue on my-work", s)
	}
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	eventually(t, "the PR on the thread", hasPR(h, s.ID))
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
	h.RefreshPRs()
	eventually(t, "the thread woken", func() bool { return stateOf(h, s.ID) == "idle" })
	eventually(t, "the prompt on stdin", func() bool {
		users := userLines(t, h, s.ID)
		return len(users) == 1 && strings.Contains(users[0], "/address-review 12")
	})
}
