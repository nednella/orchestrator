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

	"github.com/nednella/agentos/desktop/evidence"
	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/desktop/stats"
	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
)

// tmpOf is the thread's own temp folder.
func tmpOf(h *apptest.Harness, id string) string {
	name, _ := session.ParseName(id)
	return filepath.Join(h.State, "data", name.Project, "tmp", name.Token)
}

// archived says whether the thread's state file says it is archived.
func archived(t *testing.T, h *apptest.Harness, id string) bool {
	t.Helper()
	rec, err := bus.ReadState(h.State, id)
	if err != nil {
		t.Fatal(err)
	}
	return rec.State == session.Archived
}

// talked is a thread whose chat log has the message hello.
func talked(t *testing.T, h *apptest.Harness) sessions.Session {
	t.Helper()
	s, err := h.Sessions().Create("work", "hello", true, 0)
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

func TestArchiveHidesTheThread(t *testing.T) {
	h := apptest.New(t)
	s := talked(t, h)
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
	chatLog, socket := hostFiles(h, s.ID)
	// A host that dies without its shutdown leaves its socket behind.
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !exists(tmpOf(h, s.ID)) {
		t.Fatal("the thread has no temp folder before the archive")
	}

	if err := h.ArchiveThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the row gone from the sessions event", func() bool {
		list := h.Rec.LastSessions()
		return !inList(list, s.ID) && inList(list, other.ID)
	})
	if !archived(t, h, s.ID) {
		t.Error("the state file does not say archived")
	}
	if !exists(chatLog) || !exists(evidenceDir(h, s.ID)) {
		t.Errorf("chat log there: %v, evidence there: %v; want both kept", exists(chatLog), exists(evidenceDir(h, s.ID)))
	}
	if exists(tmpOf(h, s.ID)) || exists(socket) {
		t.Errorf("temp folder there: %v, host socket there: %v; want both gone", exists(tmpOf(h, s.ID)), exists(socket))
	}
	time.Sleep(2500 * time.Millisecond) // a poll must not bring it back
	if _, ok := h.Session(s.ID); ok {
		t.Error("a poll brought the archived thread back")
	}
	if err, want := h.ArchiveThread(s.ID), "Thread "+s.ID+" is archived"; err == nil || err.Error() != want {
		t.Errorf("a second Archive = %v, want %q", err, want)
	}
}

func TestArchiveSurvivesARestart(t *testing.T) {
	h := apptest.New(t)
	s := talked(t, h)
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { got, _ := h.Session(s.ID); return got.State == "settled" })
	if err := h.ArchiveThread(s.ID); err != nil {
		t.Fatal(err)
	}

	fresh := h.Restart(t)
	fresh.Sessions().Refresh()
	fresh.Sessions().Refresh()
	if _, ok := fresh.Session(s.ID); ok {
		t.Error("the archived thread came back after a restart")
	}
	if !archived(t, fresh, s.ID) {
		t.Error("the restart wrote over the archived state file")
	}
	chatLog, _ := hostFiles(fresh, s.ID)
	if !exists(chatLog) {
		t.Error("the chat log went at the restart")
	}
	if _, err := fresh.WakeThread(s.ID); err == nil || err.Error() != "Thread "+s.ID+" is archived" {
		t.Errorf("WakeThread after a restart = %v, want it refused as archived", err)
	}
}

func TestArchivedThreadRefusesMessages(t *testing.T) {
	t.Run("the chat and a wake", func(t *testing.T) {
		h := apptest.New(t)
		s := talked(t, h)
		if err := h.SettleThread(s.ID); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the thread settled", func() bool { got, _ := h.Session(s.ID); return got.State == "settled" })
		if err := h.ArchiveThread(s.ID); err != nil {
			t.Fatal(err)
		}
		want := "Thread " + s.ID + " is archived"
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
			t.Error("the archived thread runs again")
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
		if err := h.ArchiveThread(s.ID); err != nil {
			t.Fatal(err)
		}
		h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
		h.RefreshPRs()
		time.Sleep(500 * time.Millisecond)
		if tmuxHas(h, s.ID) || len(h.Sessions().List()) != 0 {
			t.Errorf("a PR review woke the archived thread: running %v, rows %+v", tmuxHas(h, s.ID), h.Sessions().List())
		}
		if !archived(t, h, s.ID) {
			t.Error("the state file does not say archived after the PR review")
		}
	})

	t.Run("a start of its issue makes a new thread", func(t *testing.T) {
		h := apptest.New(t)
		s, err := h.StartIssue(7)
		if err != nil {
			t.Fatal(err)
		}
		ended(t, h, s)
		if err := h.ArchiveThread(s.ID); err != nil {
			t.Fatal(err)
		}
		got, err := h.StartIssue(7)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID == s.ID || got.State == "settled" || got.Issue != 7 {
			t.Errorf("start of an archived thread's issue = %+v, want a new thread", got)
		}
		if tmuxHas(h, s.ID) || !archived(t, h, s.ID) {
			t.Error("the start of its issue took the archived thread back")
		}
	})
}

func TestArchiveOfALiveThreadSettlesItFirst(t *testing.T) {
	h := apptest.New(t)
	s := talked(t, h)
	transcript := filepath.Join(t.TempDir(), "conv.jsonl")
	appendLines(t, transcript, assistantLine("msg_1", time.Now().Add(100*time.Millisecond), 1000, 100))
	h.Hook(t, s.ID, "UserPromptSubmit", fmt.Sprintf(`{"prompt":"go","transcript_path":%q}`, transcript))
	eventually(t, "the turn's tokens on the row", func() bool { row, _ := h.Session(s.ID); return row.Tokens == 1100 })
	chatLog, socket := hostFiles(h, s.ID)

	if err := h.ArchiveThread(s.ID); err != nil {
		t.Fatal(err)
	}
	if tmuxHas(h, s.ID) {
		t.Error("the thread still runs after the archive")
	}
	if _, ok := h.Session(s.ID); ok {
		t.Error("the archived thread still has a row")
	}
	rec, err := bus.ReadState(h.State, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.State != session.Archived || rec.SettledAt == 0 || len(rec.Changes) == 0 || rec.Changes[len(rec.Changes)-1].State != session.Settled || rec.Title != "work" {
		t.Errorf("state file = %+v, want it archived after it settled, with its identity", rec)
	}
	if !exists(chatLog) || exists(tmpOf(h, s.ID)) || exists(socket) {
		t.Errorf("chat log there: %v, temp folder there: %v, socket there: %v", exists(chatLog), exists(tmpOf(h, s.ID)), exists(socket))
	}
	time.Sleep(300 * time.Millisecond)
	if used := h.Events(t, activity.Usage); len(used) != 1 || used[0].Tokens.Total() != 1100 {
		t.Errorf("usage events = %+v, want the turn's usage recorded once", used)
	}
}

func TestArchiveRefusesAShell(t *testing.T) {
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
	if err, want := h.ArchiveThread("main/shell"), "the shell is not a session"; err == nil || err.Error() != want {
		t.Errorf("Archive of a shell's row = %v, want %q", err, want)
	}

	shell, err := h.ShellOpen("")
	if err != nil {
		t.Fatal(err)
	}
	if err, want := h.ArchiveThread(shell.ID), "the shell is not a session"; err == nil || err.Error() != want {
		t.Errorf("Archive of a live shell = %v, want %q", err, want)
	}
	time.Sleep(100 * time.Millisecond)
	if !tmuxHas(h, shell.ID) {
		t.Error("the shell stopped")
	}
	if !exists(filepath.Join(project, "tmp")) || !exists(tmpOf(h, thread.ID)) || !exists(image) || !exists(filepath.Join(project, "chats")) {
		t.Error("the project's tmp, a thread's temp folder, the chat images or the chat logs went with a shell's archive")
	}
}

func TestArchiveLeavesAnOldTerminalAlone(t *testing.T) {
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

	if err := h.ArchiveThread(id); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(id); ok {
		t.Error("an archived old session still has a row")
	}
	paneRuns := func(h *apptest.Harness) bool {
		out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{pane_current_command}#{@agentos-name}").Output()
		return err == nil && strings.TrimSpace(string(out)) == "sleep#12 work"
	}
	if !paneRuns(h) {
		t.Error("the old session's pane does not run sleep under its name")
	}
	if !exists(scratch) {
		t.Error("the old session's own temp folder went")
	}
	if !archived(t, h, id) {
		t.Error("the old session's state file does not say archived")
	}

	fresh := h.Restart(t)
	fresh.Sessions().Refresh()
	if _, ok := fresh.Session(id); ok {
		t.Error("the archived old session came back after a restart")
	}
	if !paneRuns(fresh) || !exists(scratch) {
		t.Error("the restart touched the old session")
	}
}
