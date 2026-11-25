package sessions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/chat"
	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/internal/bus"
	chatlog "github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/host"
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

			// No hook has said the thread works yet: its host knows the turn runs.
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

func TestSetAccessAfterAReplyThatAsks(t *testing.T) {
	h := apptest.New(t)
	s, err := h.NewThread("ask", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "hi"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the turn's end", func() bool { return turnEnds(h, s.ID) == 1 })
	h.Hook(t, s.ID, "Stop", `{"last_assistant_message":"Shall I change the file?"}`)
	eventually(t, "the thread waiting on the owner", func() bool { got, _ := h.Session(s.ID); return got.State == session.Waiting })

	if err := h.SetAccess(s.ID, "supervised"); err != nil {
		t.Fatal(err)
	}
	if got := modeRequests(t, h, s.ID); len(got) != 1 || !strings.Contains(got[0], `"mode":"default"`) {
		t.Fatalf("stdin = %q, want one request for default", got)
	}
	eventually(t, "the access in the row and the state file", accessIs(h, s.ID, session.Supervised))
}

func TestSetAccessKeepsTheAccessWhenTheWakeFails(t *testing.T) {
	h := apptest.New(t)
	s, err := h.Sessions().Create("work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "hi"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the turn's end", func() bool { return turnEnds(h, s.ID) == 1 })
	h.Hook(t, s.ID, "Stop", fmt.Sprintf(`{"session_id":"fake-session","transcript_path":%q}`, writtenTranscript(t)))
	path := os.Getenv("PATH")
	t.Setenv("PATH", apptest.PathWithout("claude"))

	if err := h.SetAccess(s.ID, "full"); err == nil || !strings.Contains(err.Error(), "claude is not installed") {
		t.Fatalf("err = %v, want the wake's", err)
	}
	if row, _ := h.Session(s.ID); row.State != "settled" {
		t.Errorf("row = %+v, want it settled", row)
	}
	eventually(t, "the access in the row and the state file", accessIs(h, s.ID, session.Full))

	t.Setenv("PATH", path)
	if _, err := h.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "claude started as full", func() bool {
		return strings.Contains(strings.Join(h.Args(t, s.ID), " "), "--permission-mode bypassPermissions")
	})
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
	if err := h.SetAccess(s.ID, ""); err == nil || err.Error() != "Pick an access level" {
		t.Errorf("blank: err = %v", err)
	}
}

// askToExitPlan has the Read thread ask to exit plan mode and waits until it asks.
func askToExitPlan(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	if _, err := h.ChatSend(id, "plan"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the plan to approve", func() bool {
		return slices.ContainsFunc(h.ChatPending(), func(p chat.Pending) bool { return p.ID == id && p.Kind == "plan" })
	})
}

// modeLogged says whether the thread's chat log has the mode.
func modeLogged(h *apptest.Harness, id, mode string) func() bool {
	return func() bool { return chatLogHas(h, id, "mode", mode) }
}

func TestClaudesOwnModeChangeIsFollowed(t *testing.T) {
	h := apptest.New(t)
	s, err := h.NewThread("plan it", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	askToExitPlan(t, h, s.ID)
	if err := h.ChatAnswer(s.ID, "plan-1", true, ""); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the access Claude left plan mode for", accessIs(h, s.ID, session.Supervised))
	if got := accessTag(t, h, s.ID); got != "supervised" {
		t.Errorf("tagged %q, want supervised", got)
	}

	fresh := h.Restart(t)
	eventually(t, "the access after a restart", func() bool { row, _ := fresh.Session(s.ID); return row.Access == session.Supervised })
}

func TestModeWithoutAnApprovedPlanGivesNoMoreAccess(t *testing.T) {
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
	eventually(t, "the mode in the log", modeLogged(h, s.ID, "default"))
	eventually(t, "the turn's end", func() bool { return turnEnds(h, s.ID) == 1 })
	time.Sleep(300 * time.Millisecond) // the follower takes the turn's end after the mode
	if row, _ := h.Session(s.ID); row.Access != session.Read || accessTag(t, h, s.ID) != "read" {
		t.Errorf("row = %+v, tagged %q, want read", row, accessTag(t, h, s.ID))
	}
}

func TestModeLoggedWhileTheAppWasClosedIsFollowed(t *testing.T) {
	h := apptest.New(t)
	approved, err := h.NewThread("approved", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	left, err := h.NewThread("left", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	askToExitPlan(t, h, approved.ID)
	claudeArgs(t, h, left.ID)
	h.App.Stop()

	_, socket := hostFiles(h, approved.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := host.Answer(ctx, socket, "plan-1", true, ""); err != nil {
		t.Fatal(err)
	}
	if err := sendToHost(h, left.ID, "leave plan"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the approved thread's mode in the log", modeLogged(h, approved.ID, "default"))
	eventually(t, "the other thread's mode in the log", modeLogged(h, left.ID, "default"))

	fresh := h.Restart(t)
	eventually(t, "the approved plan's access after the restart", accessIs(fresh, approved.ID, session.Supervised))
	if row, _ := fresh.Session(left.ID); row.Access != session.Read {
		t.Errorf("row of the thread with no approved plan = %+v, want read", row)
	}
}

// diedMidRun leaves the settled thread's chat log as a host that was killed leaves it: no exit, with events after the
// run's start.
func diedMidRun(t *testing.T, h *apptest.Harness, id string, events ...chatlog.Event) {
	t.Helper()
	path, _ := hostFiles(h, id)
	logged, err := chatlog.ReadLog(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	var kept bytes.Buffer
	enc := json.NewEncoder(&kept)
	for _, e := range logged {
		if e.Kind != chatlog.KindExit {
			enc.Encode(e)
		}
	}
	if err := os.WriteFile(path, kept.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := chatlog.OpenLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, e := range events {
		if _, err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
}

// followedATurn waits until the thread's follower has handed on a turn, which it does only after its replay of the log.
func followedATurn(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	if err := h.ChatWatch(id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(id, "hi"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a turn's end in a chat event", func() bool {
		for _, p := range h.Rec.All("chat:" + id) {
			if slices.ContainsFunc(p.(chat.Update).Events, func(e chatlog.Event) bool { return e.Kind == chatlog.KindTurnEnd }) {
				return true
			}
		}
		return false
	})
}

func TestAModeOfAKilledRunIsNotFollowedAfterAWake(t *testing.T) {
	approvedPlan := []chatlog.Event{
		{Kind: chatlog.KindApproval, ID: "plan-1", Tool: "ExitPlanMode", Input: json.RawMessage(`{"plan":"p"}`)},
		{Kind: chatlog.KindAnswer, ID: "plan-1", Text: "allow"},
		{Kind: chatlog.KindMode, Text: "default"},
	}
	for _, c := range []struct {
		name   string
		access session.Access
		run    []chatlog.Event
	}{
		{"an approved plan does not raise a thread woken at read", session.Read, approvedPlan},
		{"a plan mode does not lower a thread woken at full", session.Full, []chatlog.Event{{Kind: chatlog.KindMode, Text: "plan"}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := apptest.New(t)
			s := talkedThenSettled(t, h)
			diedMidRun(t, h, s.ID, c.run...)
			if err := h.SetAccess(s.ID, string(c.access)); err != nil {
				t.Fatal(err)
			}
			if _, err := h.WakeThread(s.ID); err != nil {
				t.Fatal(err)
			}
			followedATurn(t, h, s.ID)
			if row, _ := h.Session(s.ID); row.Access != c.access || accessTag(t, h, s.ID) != string(c.access) {
				t.Errorf("row = %+v, tagged %q; want %s, as it was woken", row, accessTag(t, h, s.ID), c.access)
			}
		})
	}
}

func TestASendWhileTheThreadRestartsIsRefused(t *testing.T) {
	h := apptest.New(t)
	s, err := h.NewThread("restarting", "", "", "", "supervised")
	if err != nil {
		t.Fatal(err)
	}
	_, socket := hostFiles(h, s.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := host.Ready(ctx, socket); err != nil {
		t.Fatal(err)
	}
	if idle, err := host.Hold(ctx, socket); err != nil || !idle {
		t.Fatalf("Hold = %v, %v; want idle", idle, err)
	}
	if _, err := h.ChatSend(s.ID, "keep this"); err == nil || err.Error() != "This thread is restarting" {
		t.Fatalf("ChatSend = %v, want the host's refusal, so the composer keeps the text", err)
	}
	if strings.Contains(sentTo(t, h, s.ID), "keep this") {
		t.Error("claude got the message sent while the thread restarts")
	}
}
