package sessions_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/evidence"
	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/desktop/stats"
	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
)

// tmpOf is the thread's own temp folder.
func tmpOf(h *apptest.Harness, id string) string {
	name, _ := session.ParseName(id)
	return filepath.Join(h.State, "data", name.Project, "tmp", name.Token)
}

// handoverOf is where the thread's handover is kept.
func handoverOf(h *apptest.Harness, id string) string {
	name, _ := session.ParseName(id)
	return filepath.Join(h.State, "data", name.Project, "handovers", name.Token+".md")
}

// hasState says whether the thread has a state file.
func hasState(t *testing.T, h *apptest.Harness, id string) bool {
	t.Helper()
	rec, err := bus.ReadState(h.State, id)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Session != ""
}

// talked is a thread whose chat log has the message hello.
func talked(t *testing.T, h *apptest.Harness) sessions.Session {
	t.Helper()
	s, err := h.Sessions().Create("work", "hello", true, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	chatLog, _ := hostFiles(h, s.ID)
	eventually(t, "the chat log", func() bool {
		data, _ := os.ReadFile(chatLog)
		return strings.Contains(string(data), `"kind":"user"`)
	})
	return s
}

func inList(list []sessions.Session, id string) bool {
	return slices.ContainsFunc(list, func(s sessions.Session) bool { return s.ID == id })
}

func TestDeleteRemovesTheThreadAndItsChat(t *testing.T) {
	h := apptest.New(t)
	s, _, err := h.Sessions().CreateIssue("#12 work", "hello", 12, project.Action{Name: "Work"}, nil, "what came before", sessions.StartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	chatLog, socket := hostFiles(h, s.ID)
	eventually(t, "the chat log", func() bool { data, _ := os.ReadFile(chatLog); return strings.Contains(string(data), `"kind":"user"`) })
	other, err := h.NewSession("other", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := evidence.New(filepath.Join(h.State, "data"), stats.NewLog(filepath.Join(h.State, "data"), "testtest")).AddText(s.ID, "proof", "", "agent"); err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { got, _ := h.Session(s.ID); return got.State == "settled" })
	// A host that dies without its shutdown leaves its socket behind.
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{tmpOf(h, s.ID), handoverOf(h, s.ID), evidenceDir(h, s.ID)} {
		if !exists(path) {
			t.Fatalf("%s is not there before the delete", path)
		}
	}
	started := len(h.Events(t, activity.SessionStart))

	if err := h.DeleteThread(s.ID, false); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the row gone from the sessions event", func() bool {
		list := h.Rec.LastSessions()
		return !inList(list, s.ID) && inList(list, other.ID)
	})
	if hasState(t, h, s.ID) {
		t.Error("the state file is still there")
	}
	for _, path := range []string{chatLog, evidenceDir(h, s.ID), handoverOf(h, s.ID), tmpOf(h, s.ID), socket} {
		if exists(path) {
			t.Errorf("%s is still there", path)
		}
	}
	if got := len(h.Events(t, activity.SessionStart)); got != started {
		t.Errorf("start events = %d, want the %d from before the delete kept", got, started)
	}
	time.Sleep(2500 * time.Millisecond) // a poll must not bring it back
	if _, ok := h.Session(s.ID); ok {
		t.Error("a poll brought the deleted thread back")
	}
	if err, want := h.DeleteThread(s.ID, false), "Thread "+s.ID+" was deleted"; err == nil || err.Error() != want {
		t.Errorf("a second Delete = %v, want %q", err, want)
	}
}

func TestDeleteSurvivesARestart(t *testing.T) {
	h := apptest.New(t)
	s := talked(t, h)
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { got, _ := h.Session(s.ID); return got.State == "settled" })
	if err := h.DeleteThread(s.ID, false); err != nil {
		t.Fatal(err)
	}

	fresh := h.Restart(t)
	fresh.Sessions().Refresh()
	fresh.Sessions().Refresh()
	if _, ok := fresh.Session(s.ID); ok {
		t.Error("the deleted thread came back after a restart")
	}
	if hasState(t, fresh, s.ID) {
		t.Error("the restart wrote a state file for the deleted thread")
	}
	if _, err := fresh.WakeThread(s.ID); err == nil {
		t.Error("WakeThread after a restart woke the deleted thread")
	}
}

func TestDeletedThreadRefusesMessages(t *testing.T) {
	t.Run("the chat and a wake", func(t *testing.T) {
		h := apptest.New(t)
		s := talked(t, h)
		if err := h.SettleThread(s.ID); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the thread settled", func() bool { got, _ := h.Session(s.ID); return got.State == "settled" })
		if err := h.DeleteThread(s.ID, false); err != nil {
			t.Fatal(err)
		}
		want := "Thread " + s.ID + " was deleted"
		if _, err := h.ChatSend(s.ID, "again"); err == nil || err.Error() != want {
			t.Errorf("ChatSend = %v, want %q", err, want)
		}
		if err := h.ChatDraft(s.ID, "again"); err == nil || err.Error() != want {
			t.Errorf("ChatDraft = %v, want %q", err, want)
		}
		if _, err := h.WakeThread(s.ID); err == nil || err.Error() != want {
			t.Errorf("WakeThread = %v, want %q", err, want)
		}
		if err := h.TypeInto(s.ID, "again"); err == nil || err.Error() != want {
			t.Errorf("TypeInto = %v, want %q", err, want)
		}
		if tmuxHas(h, s.ID) {
			t.Error("the deleted thread runs again")
		}
		chatLog, _ := hostFiles(h, s.ID)
		if exists(chatLog) {
			t.Error("a refused message made the chat log again")
		}
	})

	t.Run("a PR review wakes nothing", func(t *testing.T) {
		h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n"})
		s := issueSession(h, t, 12)
		h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
		h.RefreshPRs()
		if err := h.SettleThread(s.ID); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the thread settled", func() bool { got, _ := h.Session(s.ID); return got.State == "settled" })
		if err := h.DeleteThread(s.ID, false); err != nil {
			t.Fatal(err)
		}
		h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
		h.RefreshPRs()
		time.Sleep(500 * time.Millisecond)
		if tmuxHas(h, s.ID) || len(h.Sessions().List()) != 0 {
			t.Errorf("a PR review woke the deleted thread: running %v, rows %+v", tmuxHas(h, s.ID), h.Sessions().List())
		}
		if hasState(t, h, s.ID) {
			t.Error("the PR review wrote a state file for the deleted thread")
		}
	})

	t.Run("a start of its issue makes a new thread", func(t *testing.T) {
		h := apptest.New(t)
		s, err := h.StartIssue(7)
		if err != nil {
			t.Fatal(err)
		}
		ended(t, h, s)
		if err := h.DeleteThread(s.ID, false); err != nil {
			t.Fatal(err)
		}
		got, err := h.StartIssue(7)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID == s.ID || got.State == "settled" || got.Issue != 7 {
			t.Errorf("start of a deleted thread's issue = %+v, want a new thread", got)
		}
		if tmuxHas(h, s.ID) || hasState(t, h, s.ID) {
			t.Error("the start of its issue took the deleted thread back")
		}
	})
}

func TestDeleteOfALiveThreadStopsIt(t *testing.T) {
	h := apptest.New(t)
	s := talked(t, h)
	transcript := filepath.Join(t.TempDir(), "conv.jsonl")
	appendLines(t, transcript, assistantLine("msg_1", time.Now().Add(100*time.Millisecond), 1000, 100))
	h.Hook(t, s.ID, "UserPromptSubmit", fmt.Sprintf(`{"prompt":"go","transcript_path":%q}`, transcript))
	eventually(t, "the turn's tokens on the row", func() bool { row, _ := h.Session(s.ID); return row.Tokens == 1100 })
	chatLog, socket := hostFiles(h, s.ID)

	if err := h.DeleteThread(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if tmuxHas(h, s.ID) {
		t.Error("the thread still runs after the delete")
	}
	if _, ok := h.Session(s.ID); ok {
		t.Error("the deleted thread still has a row")
	}
	time.Sleep(300 * time.Millisecond) // the host's exit and a late usage count come after the stop
	if hasState(t, h, s.ID) || exists(chatLog) || exists(tmpOf(h, s.ID)) || exists(socket) {
		t.Errorf("state file there: %v, chat log there: %v, temp folder there: %v, socket there: %v", hasState(t, h, s.ID), exists(chatLog), exists(tmpOf(h, s.ID)), exists(socket))
	}
	if used := h.Events(t, activity.Usage); len(used) != 1 || used[0].Tokens.Total() != 1100 {
		t.Errorf("usage events = %+v, want the turn's usage recorded once", used)
	}
}

func TestDeleteWithTheWorktreeRemovesItAndKeepsTheBranch(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	head := gitIn(t, wt, "rev-parse", "HEAD")
	chatLog, _ := hostFiles(h, s.ID)
	if got := h.OwnWorktree(s.ID); got != wt {
		t.Fatalf("OwnWorktree = %q, want %s", got, wt)
	}

	if err := h.DeleteThread(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if exists(wt) || strings.Contains(gitIn(t, h.Dir, "worktree", "list", "--porcelain"), wt) {
		t.Errorf("the worktree is still there: on disk %v, git lists %s", exists(wt), gitIn(t, h.Dir, "worktree", "list", "--porcelain"))
	}
	if got := gitIn(t, h.Dir, "rev-parse", "refs/heads/issue-7"); got != head {
		t.Errorf("issue-7 is at %q, want %s", got, head)
	}
	if _, ok := h.Session(s.ID); ok || tmuxHas(h, s.ID) || hasState(t, h, s.ID) || exists(chatLog) {
		t.Errorf("the thread: row there %v, running %v, state file there %v, chat log there %v", ok, tmuxHas(h, s.ID), hasState(t, h, s.ID), exists(chatLog))
	}
	if records := madeRecords(t, h); len(records) != 0 {
		t.Errorf("records = %+v, want the thread's gone with it", records)
	}
	log := h.Cleanups()
	if len(log) != 1 || log[0].Status != "done" || log[0].SessionTitle != "#7 work" {
		t.Fatalf("log = %+v", log)
	}
	if want := []string{"worktree " + wt, "thread put to sleep", "temp folder"}; !slices.Equal(log[0].Removed, want) {
		t.Errorf("removed = %q, want %q", log[0].Removed, want)
	}
	if got := h.Events(t, activity.Cleanup); len(got) != 1 || got[0].Label != "done" || got[0].Session != s.ID {
		t.Errorf("cleanup events = %+v", got)
	}
}

func TestDeleteWithTheWorktreeIsRefusedWhileItHoldsWork(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	notes := filepath.Join(wt, "notes.txt")
	if err := os.WriteFile(notes, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	chatLog, _ := hostFiles(h, s.ID)

	err := h.DeleteThread(s.ID, true)
	if want := fmt.Sprintf("Thread %d keeps its worktree: the worktree of issue-7 has uncommitted changes. Untick the box to keep the worktree, or clean Thread %d up first", s.N, s.N); err == nil || err.Error() != want {
		t.Errorf("Delete = %v, want %q", err, want)
	}
	if got, ok := h.Session(s.ID); !ok || got.State == "settled" || got.Worktree != wt {
		t.Errorf("the row = %+v, there %v; want it running, with its worktree", got, ok)
	}
	if !tmuxHas(h, s.ID) || !hasState(t, h, s.ID) || !exists(chatLog) {
		t.Errorf("the thread: running %v, state file there %v, chat log there %v", tmuxHas(h, s.ID), hasState(t, h, s.ID), exists(chatLog))
	}
	if !exists(notes) || worktreesOf(t, h.Dir)["issue-7"] != realDir(t, wt) {
		t.Errorf("the worktree: notes there %v, worktrees %v", exists(notes), worktreesOf(t, h.Dir))
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Removed != 0 {
		t.Errorf("records = %+v", records)
	}
	if log := h.Cleanups(); len(log) != 1 || log[0].Status != "blocked" || len(log[0].Removed) != 0 {
		t.Errorf("log = %+v", log)
	}
}

func TestDeleteWithoutTheWorktreeLeavesItOnDisk(t *testing.T) {
	ran := filepath.Join(t.TempDir(), "ran")
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: noteCleanups(ran)})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	commitIn(t, wt, "unpushed.txt")
	head := gitIn(t, wt, "rev-parse", "HEAD")
	notes := filepath.Join(wt, "notes.txt")
	if err := os.WriteFile(notes, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := h.DeleteThread(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(s.ID); ok || tmuxHas(h, s.ID) {
		t.Errorf("the thread: row there %v, running %v", ok, tmuxHas(h, s.ID))
	}
	if !exists(notes) || worktreesOf(t, h.Dir)["issue-7"] != realDir(t, wt) {
		t.Errorf("the worktree: notes there %v, worktrees %v", exists(notes), worktreesOf(t, h.Dir))
	}
	if got := gitIn(t, wt, "rev-parse", "HEAD"); got != head {
		t.Errorf("the worktree is at %s, want %s", got, head)
	}
	if records := madeRecords(t, h); len(records) != 0 {
		t.Errorf("records = %+v, want the thread's gone with it", records)
	}
	if runs, log := cleanupRuns(ran), h.Cleanups(); len(runs) != 0 || len(log) != 0 {
		t.Errorf("clean-up runs %q, log %+v; want none", runs, log)
	}

	// No longer agentos's: a new thread of the issue runs in it as in one a human made, and no clean-up removes it.
	again := issueSession(h, t, 7)
	if again.Branch != "issue-7" || again.Worktree != "" || realDir(t, panePath(t, h, again.ID)) != realDir(t, wt) {
		t.Errorf("the new thread of #7 = %+v in %s, want it in %s, unrecorded", again, panePath(t, h, again.ID), wt)
	}
	if err := h.Cleanup(again.ID, true); err != nil {
		t.Fatal(err)
	}
	if !exists(notes) || len(cleanupRuns(ran)) != 0 {
		t.Errorf("a forced clean-up of the new thread: notes there %v, runs %q", exists(notes), cleanupRuns(ran))
	}
}

func TestSharedWorktreeIsNotOffered(t *testing.T) {
	t.Run("a fork, running and asleep", func(t *testing.T) {
		h, wt, ran, src, fork := forkHarness(t)
		wantShared := fmt.Sprintf("Thread %d keeps its worktree: Thread #7 work (fork) also works in this worktree. Delete it without the worktree", src.N)
		for _, state := range []string{"running", "asleep"} {
			if state == "asleep" {
				if err := h.SettleThread(fork.ID); err != nil {
					t.Fatal(err)
				}
				eventually(t, "the fork settled", func() bool { return stateOf(h, fork.ID) == "settled" })
			}
			if got := h.OwnWorktree(src.ID); got != "" {
				t.Errorf("%s: OwnWorktree = %q, want none", state, got)
			}
			if err := h.DeleteThread(src.ID, true); err == nil || err.Error() != wantShared {
				t.Errorf("%s: Delete = %v, want %q", state, err, wantShared)
			}
			if _, ok := h.Session(src.ID); !ok || !exists(wt) || len(cleanupRuns(ran)) != 0 {
				t.Errorf("%s: the source's row there %v, worktree there %v, clean-up runs %q", state, ok, exists(wt), cleanupRuns(ran))
			}
		}
		if h.OwnWorktree(fork.ID) != "" {
			t.Error("the fork is offered the source's worktree")
		}

		if err := h.DeleteThread(src.ID, false); err != nil {
			t.Fatal(err)
		}
		if !exists(wt) || len(cleanupRuns(ran)) != 0 {
			t.Errorf("worktree there %v, clean-up runs %q", exists(wt), cleanupRuns(ran))
		}
		if got, ok := h.Session(fork.ID); !ok || got.ForkOf != src.ID {
			t.Errorf("the fork = %+v, %v", got, ok)
		}
	})

	t.Run("a shell", func(t *testing.T) {
		h := shellHarness(t)
		repoFixture(t, h)
		s := issueThread(h, t, 7)
		wt := s.Worktree
		pushWork(t, wt)
		h.Hook(t, s.ID, "Stop", fmt.Sprintf(`{"cwd":%q}`, wt))
		eventually(t, "idle", func() bool { return stateOf(h, s.ID) == "idle" })
		shell, err := h.ShellNew(s.ID)
		if err != nil {
			t.Fatal(err)
		}
		eventually(t, "the shell in the worktree", func() bool { return h.OwnWorktree(s.ID) == "" })
		if err := h.ShellClose(shell.ID); err != nil {
			t.Fatal(err)
		}
		if got := h.OwnWorktree(s.ID); got != wt {
			t.Errorf("OwnWorktree once the shell closed = %q, want %s", got, wt)
		}
	})
}

func TestDeleteOfAThreadInTheProjectFolderTouchesNoFolder(t *testing.T) {
	h, s, ran := projectFolderThread(t, "")
	if err := os.WriteFile(filepath.Join(h.Dir, "notes.txt"), []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := h.OwnWorktree(s.ID); got != "" {
		t.Errorf("OwnWorktree = %q, want none", got)
	}
	if err := h.DeleteThread(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(s.ID); ok {
		t.Error("the thread still has a row")
	}
	projectFolderIntact(t, h, "notes.txt")
	if runs, log := cleanupRuns(ran), h.Cleanups(); len(runs) != 0 || len(log) != 0 {
		t.Errorf("clean-up runs %q, log %+v; want none", runs, log)
	}
}

func TestDeleteIsRefusedDuringACleanup(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	pushWork(t, s.Worktree)
	release, done := heldCleanup(t, h, s.ID)

	for _, worktree := range []bool{false, true} {
		err := h.DeleteThread(s.ID, worktree)
		if want := fmt.Sprintf("Thread %d is being cleaned up; delete it when that ends", s.N); err == nil || err.Error() != want {
			t.Errorf("Delete with the worktree %v = %v, want %q", worktree, err, want)
		}
	}
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !cleanedUp(h, s.ID)() || exists(s.Worktree) || !hasState(t, h, s.ID) {
		t.Errorf("the clean-up did not finish with the thread kept: worktree there %v", exists(s.Worktree))
	}
}

func TestDeleteRefusesAShell(t *testing.T) {
	h := shellHarness(t)
	thread, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(h.State, "data", "main")
	image := filepath.Join(project, "chat-media", "k3x9q0ab", "0123456789abcdef.png")
	if err := os.MkdirAll(filepath.Dir(image), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	long := time.Now().Add(-8 * 24 * time.Hour)
	// A shell's name has no token: an older build wrote a state file for one.
	rec := session.Record{Session: "main/shell", State: session.Settled, Event: "SessionEnd", At: long, Identity: session.Identity{Title: "old"}, SettledAt: long.UnixMilli()}
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	if _, ok := h.Session("main/shell"); ok {
		t.Error("a shell's state file made a row")
	}
	if err, want := h.DeleteThread("main/shell", false), "the shell is not a session"; err == nil || err.Error() != want {
		t.Errorf("Delete of a shell's row = %v, want %q", err, want)
	}

	shell, err := h.ShellNew("")
	if err != nil {
		t.Fatal(err)
	}
	if err, want := h.DeleteThread(shell.ID, false), "the shell is not a session"; err == nil || err.Error() != want {
		t.Errorf("Delete of a live shell = %v, want %q", err, want)
	}
	time.Sleep(100 * time.Millisecond)
	if !tmuxHas(h, shell.ID) {
		t.Error("the shell stopped")
	}
	if !exists(filepath.Join(project, "tmp")) || !exists(tmpOf(h, thread.ID)) || !exists(image) || !exists(filepath.Join(project, "chats")) {
		t.Error("the project's tmp, a thread's temp folder, the chat images or the chat logs went with a shell's delete")
	}
}

func TestDeleteLeavesAnOldTerminalAlone(t *testing.T) {
	h := newHarness(t)
	id := "main/aaaaaaaa"
	tmuxRun(t, h, "-f", "/dev/null", "new-session", "-d", "-s", id, "-c", h.Dir, "sleep", "600")
	tmuxRun(t, h, "set-option", "-t", id, "@agentos-issue", "12", ";", "set-option", "-t", id, "@agentos-name", "#12 work")
	// What the old session keeps of its own: a temp folder an older build gave it.
	scratch := filepath.Join(tmpOf(h, id), "notes.txt")
	if err := os.MkdirAll(filepath.Dir(scratch), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scratch, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	if _, ok := h.Session(id); !ok {
		t.Fatal("the old session has no row")
	}

	for _, worktree := range []bool{false, true} {
		if err, want := h.DeleteThread(id, worktree), "Thread "+id+" runs in an old terminal; end it there"; err == nil || err.Error() != want {
			t.Errorf("Delete of a live old session, with the worktree %v = %v, want %q", worktree, err, want)
		}
	}
	h.Sessions().Refresh()
	if _, ok := h.Session(id); !ok {
		t.Error("the refused delete took the old session's row")
	}
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{pane_current_command}#{@agentos-name}").Output()
	if err != nil || strings.TrimSpace(string(out)) != "sleep#12 work" {
		t.Errorf("old session's pane runs and is named %q (%v), want sleep#12 work", out, err)
	}
	if !exists(scratch) {
		t.Error("the old session's own temp folder went")
	}

	got, did, err := h.Sessions().CreateIssue("#12 work", "/ship 12", 12, project.Action{Name: "Work"}, nil, "", sessions.StartOptions{})
	if err != nil || did != sessions.Kept || got.ID != id {
		t.Errorf("CreateIssue = %+v, %v, %v; want the old session %s kept", got, did, err, id)
	}
	if names := tmuxNames(t, h); names != id {
		t.Errorf("tmux sessions = %q, want only the old one", names)
	}
}

func TestDeleteOfAnEndedOldSessionRemovesIt(t *testing.T) {
	h := newHarness(t)
	id := "main/bbbbbbbb"
	rec := session.Record{Session: id, State: session.Idle, Event: "Stop", At: time.Now().Add(-time.Hour), Identity: session.Identity{Title: "old work"}}
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(tmpOf(h, id), "notes.txt")
	if err := os.MkdirAll(filepath.Dir(scratch), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scratch, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	if row, ok := h.Session(id); !ok || row.State != session.Settled || row.Hosted {
		t.Fatalf("ended old session's row = %+v (%v), want settled, not hosted", row, ok)
	}

	if err := h.DeleteThread(id, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(id); ok {
		t.Error("the deleted old session still has a row")
	}
	if exists(tmpOf(h, id)) || hasState(t, h, id) {
		t.Errorf("temp folder there %v, state file there %v; want both gone", exists(tmpOf(h, id)), hasState(t, h, id))
	}
}

func TestDeleteWaitingOnAHostThatDoesNotStopHoldsUpNoNewThread(t *testing.T) {
	h := apptest.New(t)
	s := talked(t, h)
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread asleep", func() bool { return stateOf(h, s.ID) == "settled" })
	_, socket := hostFiles(h, s.ID)
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	stuck, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stuck.Close() })
	go func() {
		for {
			conn, err := stuck.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	done := make(chan error, 1)
	go func() { done <- h.DeleteThread(s.ID, false) }()
	eventually(t, "the row gone", func() bool { _, ok := h.Session(s.ID); return !ok })
	if _, err := h.NewSession("other", ""); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatalf("the delete ended (%v) before the new thread started, so it did not wait on the host", err)
	default:
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestDeleteIsRefusedWhileAChildRuns(t *testing.T) {
	h := apptest.New(t)
	parent := talked(t, h)
	child, err := h.Sessions().CreateChild(context.Background(), sessions.ChildOptions{Parent: parent.ID, Task: "Read the code", Title: "Helper", Access: parent.Access, Where: sessions.WhereFolder})
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("Thread %d has a running child, Helper; stop it first", parent.N)
	for _, worktree := range []bool{false, true} {
		if err := h.DeleteThread(parent.ID, worktree); err == nil || err.Error() != want {
			t.Errorf("Delete with the worktree %v = %v, want %q", worktree, err, want)
		}
	}
	if _, ok := h.Session(parent.ID); !ok || !tmuxHas(h, parent.ID) || !hasState(t, h, parent.ID) {
		t.Errorf("the parent: row there %v, running %v, state file there %v", ok, tmuxHas(h, parent.ID), hasState(t, h, parent.ID))
	}

	if err := h.SettleThread(child.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the child asleep", func() bool { return stateOf(h, child.ID) == "settled" })
	if err := h.DeleteThread(parent.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(child.ID); ok || hasState(t, h, child.ID) {
		t.Errorf("the asleep child: row there %v, state file there %v; want it deleted with its parent", ok, hasState(t, h, child.ID))
	}
}

func TestThreadBeingDeletedIsNotWoken(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	pushWork(t, s.Worktree)
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread asleep", func() bool { return stateOf(h, s.ID) == "settled" })
	hold := t.TempDir()
	reached, released := filepath.Join(hold, "reached"), filepath.Join(hold, "release")
	gitIn(t, h.Dir, "config", "remote.origin.uploadpack", fmt.Sprintf("touch %s; while [ ! -e %s ]; do sleep 0.05; done; git-upload-pack", reached, released))
	done := make(chan error, 1)
	go func() { done <- h.DeleteThread(s.ID, true) }()
	eventually(t, "the delete held in its clean-up", func() bool { return exists(reached) })

	_, err := h.WakeThread(s.ID)
	if want := fmt.Sprintf("Thread %d is being deleted", s.N); err == nil || err.Error() != want {
		t.Errorf("wake = %v, want %q", err, want)
	}
	if err := os.WriteFile(released, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(s.ID); ok || tmuxHas(h, s.ID) || exists(s.Worktree) {
		t.Errorf("after the delete: row there %v, running %v, worktree there %v", ok, tmuxHas(h, s.ID), exists(s.Worktree))
	}
}

func TestDeleteKeptAfterItsSettleSaysTheThreadIsAsleep(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	late, fetched := filepath.Join(wt, "late.txt"), filepath.Join(t.TempDir(), "fetched")
	// The second checks fetch origin after the settle, so a file written at that fetch is one the agent wrote as it stopped.
	gitIn(t, h.Dir, "config", "remote.origin.uploadpack", fmt.Sprintf("if [ -e %s ]; then echo late > %s; fi; touch %s; git-upload-pack", fetched, late, fetched))

	err := h.DeleteThread(s.ID, true)
	if want := fmt.Sprintf("Thread %d keeps its worktree: the worktree of issue-7 has uncommitted changes. It is asleep now. Untick the box to keep the worktree, or clean Thread %d up first", s.N, s.N); err == nil || err.Error() != want {
		t.Errorf("Delete = %v, want %q", err, want)
	}
	if got, ok := h.Session(s.ID); !ok || got.State != "settled" || got.Cleanup != "blocked" {
		t.Errorf("the row = %+v, there %v; want it asleep, its clean-up kept", got, ok)
	}
	if !hasState(t, h, s.ID) || !exists(late) {
		t.Errorf("state file there %v, the late file there %v", hasState(t, h, s.ID), exists(late))
	}
}

func TestCleanedUpThreadIsNotOfferedAWorktree(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	if err := h.Cleanup(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if !cleanedUp(h, s.ID)() || exists(wt) {
		t.Fatalf("the clean-up: done %v, worktree there %v", cleanedUp(h, s.ID)(), exists(wt))
	}
	// A record that lost its removal, with a worktree back at its path, as only a hand edit makes.
	gitIn(t, h.Dir, "worktree", "add", wt, "issue-7")
	records := madeRecords(t, h)
	records[0].Removed = 0
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.State, "data", "main", "worktrees.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := h.Restart(t)

	if got := fresh.OwnWorktree(s.ID); got != "" {
		t.Errorf("OwnWorktree = %q, want none", got)
	}
	if err := fresh.DeleteThread(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := fresh.Session(s.ID); ok || !exists(wt) {
		t.Errorf("row there %v, worktree there %v; want the row gone and the worktree left", ok, exists(wt))
	}
}
