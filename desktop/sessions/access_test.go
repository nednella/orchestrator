package sessions_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
)

// paneOf is the thread's tmux session and the process its pane runs, the host.
func paneOf(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{session_id} #{pane_pid}").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func modeRequests(t *testing.T, h *apptest.Harness, id string) []string {
	t.Helper()
	var out []string
	for _, l := range h.Stdin(t, id) {
		if strings.Contains(l, `"subtype":"set_permission_mode"`) {
			out = append(out, l)
		}
	}
	return out
}

// accessIs says whether the thread's row and state file hold the access.
func accessIs(h *apptest.Harness, id string, want session.Access) func() bool {
	return func() bool {
		row, _ := h.Session(id)
		rec, _ := bus.ReadState(h.State, id)
		return row.Access == want && rec.Access == want
	}
}

func TestSetAccessReadToSupervisedIsLive(t *testing.T) {
	h := apptest.New(t)
	s, err := h.NewThread("ask", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	args := claudeArgs(t, h, s.ID)
	pane := paneOf(t, h, s.ID)

	if err := h.SetAccess(s.ID, "supervised"); err != nil {
		t.Fatal(err)
	}
	if got := modeRequests(t, h, s.ID); len(got) != 1 || !strings.Contains(got[0], `"mode":"default"`) {
		t.Fatalf("stdin = %q, want one request for default", got)
	}
	if got := accessTag(t, h, s.ID); got != "supervised" {
		t.Errorf("tagged %q, want supervised", got)
	}
	eventually(t, "the access in the row and the state file", accessIs(h, s.ID, session.Supervised))
	if got := paneOf(t, h, s.ID); got != pane {
		t.Errorf("tmux session and host %q, want the same as before, %q", got, pane)
	}
	if got := h.Args(t, s.ID); strings.Join(got, " ") != strings.Join(args, " ") {
		t.Errorf("claude started again with %q", got)
	}

	if err := h.SetAccess(s.ID, "read"); err != nil {
		t.Fatal(err)
	}
	if got := modeRequests(t, h, s.ID); len(got) != 2 || !strings.Contains(got[1], `"mode":"plan"`) {
		t.Fatalf("stdin = %q, want a second request, for plan", got)
	}
	eventually(t, "the access back to read", accessIs(h, s.ID, session.Read))
}

func TestSetAccessToFullRestarts(t *testing.T) {
	h := apptest.New(t)
	s, err := h.Sessions().Create("work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "hi"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first turn's end", func() bool { return turnEnds(h, s.ID) == 1 })
	h.Hook(t, s.ID, "Stop", fmt.Sprintf(`{"session_id":"fake-session","transcript_path":%q}`, writtenTranscript(t)))
	pane := paneOf(t, h, s.ID)

	if err := h.SetAccess(s.ID, "full"); err != nil {
		t.Fatal(err)
	}
	if got := paneOf(t, h, s.ID); got == pane {
		t.Errorf("tmux session and host %q, want a new host", got)
	}
	eventually(t, "claude resuming the conversation", resumes(t, h, s.ID, "fake-session"))
	if args := strings.Join(h.Args(t, s.ID), " "); !strings.Contains(args, "--permission-mode bypassPermissions") {
		t.Errorf("args = %s, want bypassPermissions", args)
	}
	if got := modeRequests(t, h, s.ID); len(got) != 0 {
		t.Errorf("stdin = %q, want no mode request", got)
	}
	row, _ := h.Session(s.ID)
	if row.ID != s.ID || row.N != s.N || row.State == "settled" || accessTag(t, h, s.ID) != "full" {
		t.Errorf("row = %+v, tagged %q, from %+v", row, accessTag(t, h, s.ID), s)
	}
	eventually(t, "the access in the row and the state file", accessIs(h, s.ID, session.Full))
	if list := h.Sessions().List(); len(list) != 1 {
		t.Errorf("rows = %+v, want the one thread", list)
	}

	if _, err := h.ChatSend(s.ID, "again"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the second turn's end", func() bool { return turnEnds(h, s.ID) == 2 })
	assertOneChat(t, h, s.ID, "hi", "again")

	if err := h.SetAccess(s.ID, "supervised"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "claude started again as supervised", func() bool {
		return strings.Contains(strings.Join(h.Args(t, s.ID), " "), "--permission-mode default")
	})
	eventually(t, "the access back to supervised", accessIs(h, s.ID, session.Supervised))
}

func TestSetAccessIsRefusedDuringATurn(t *testing.T) {
	for _, to := range []string{"supervised", "full"} {
		t.Run(to, func(t *testing.T) {
			h := apptest.New(t)
			s, err := h.NewThread("ask", "", "", "", "read")
			if err != nil {
				t.Fatal(err)
			}
			claudeArgs(t, h, s.ID)
			pane := paneOf(t, h, s.ID)
			if _, err := h.ChatSend(s.ID, "slow"); err != nil {
				t.Fatal(err)
			}
			h.Hook(t, s.ID, "UserPromptSubmit", `{"session_id":"fake-session","prompt":"slow"}`)
			eventually(t, "the thread working", func() bool { got, _ := h.Session(s.ID); return got.State == session.Working })

			if err := h.SetAccess(s.ID, to); err == nil || err.Error() != "Change access when the turn ends" {
				t.Fatalf("err = %v, want the turn's refusal", err)
			}
			if got := paneOf(t, h, s.ID); got != pane {
				t.Errorf("tmux session and host %q, want the same as before, %q", got, pane)
			}
			if got := modeRequests(t, h, s.ID); len(got) != 0 {
				t.Errorf("stdin = %q, want no mode request", got)
			}
			if got := accessTag(t, h, s.ID); got != "read" {
				t.Errorf("tagged %q, want read", got)
			}
		})
	}
}

func TestSetAccessDuringAHostTurnIsRefused(t *testing.T) {
	h := apptest.New(t)
	s, err := h.NewThread("ask", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	claudeArgs(t, h, s.ID)
	if _, err := h.ChatSend(s.ID, "slow"); err != nil {
		t.Fatal(err)
	}
	// No hook has said the thread works yet; its host knows the turn runs.
	if err := h.SetAccess(s.ID, "supervised"); err == nil || err.Error() != "Change access when the turn ends" {
		t.Fatalf("err = %v, want the host's refusal", err)
	}
	if got := accessTag(t, h, s.ID); got != "read" {
		t.Errorf("tagged %q, want read", got)
	}
}

func TestSetAccessOfASettledThread(t *testing.T) {
	h := apptest.New(t)
	before := talkedThenSettled(t, h)
	args := h.ClaudeDir() + "/" + strings.ReplaceAll(before.ID, "/", "_") + ".args"
	if err := os.Remove(args); err != nil {
		t.Fatal(err)
	}

	if err := h.SetAccess(before.ID, "full"); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("tmux", "-L", h.Socket, "has-session", "-t", "="+before.ID).Run(); err == nil {
		t.Error("a host started for the access change")
	}
	if exists(args) {
		t.Error("claude started for the access change")
	}
	eventually(t, "the access in the row and the state file", accessIs(h, before.ID, session.Full))
	if row, _ := h.Session(before.ID); row.State != "settled" {
		t.Errorf("row = %+v, want it still settled", row)
	}

	woken, err := h.WakeThread(before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if woken.Access != session.Full || accessTag(t, h, before.ID) != "full" {
		t.Errorf("woken row = %+v, tagged %q", woken, accessTag(t, h, before.ID))
	}
	if args := strings.Join(claudeArgs(t, h, before.ID), " "); !strings.Contains(args, "--permission-mode bypassPermissions") || !strings.Contains(args, "--resume fake-session") {
		t.Errorf("args of the woken thread = %s", args)
	}
}

func TestSetAccessRefusesWhatCannotChange(t *testing.T) {
	h := apptest.New(t)
	s := talkedThenSettled(t, h)
	if err := h.ArchiveThread(s.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.SetAccess(s.ID, "read"); err == nil || !strings.Contains(err.Error(), "archived") {
		t.Errorf("archived: err = %v", err)
	}
	if err := h.SetAccess(s.ID, "write"); err == nil {
		t.Error("an access agentos does not know was taken")
	}
}

func TestClaudesOwnModeChangeIsFollowed(t *testing.T) {
	h := apptest.New(t)
	s, err := h.NewThread("plan it", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.ChatWatch(s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "leave plan"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the access Claude left plan mode for", accessIs(h, s.ID, session.Supervised))
	if got := accessTag(t, h, s.ID); got != "supervised" {
		t.Errorf("tagged %q, want supervised", got)
	}

	fresh := h.Restart(t)
	eventually(t, "the access after a restart", func() bool { row, _ := fresh.Session(s.ID); return row.Access == session.Supervised })
}
