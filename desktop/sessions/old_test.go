package sessions_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
)

const (
	oldDetail = "Ran in a terminal before chats"
	oldChat   = "This thread ran in a terminal before agentos showed chats"
)

func TestOldTerminalSessionIsLeftAlone(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n"})
	// A session from a release before the host, on the harness's own tmux server: it has the old tags and no hosted tag.
	id := "main/aaaaaaaa"
	tmuxRun(t, h, "-f", "/dev/null", "new-session", "-d", "-s", id, "-c", h.Dir, "sleep", "600")
	tmuxRun(t, h, "set-option", "-t", id, "@agentos-issue", "12", ";", "set-option", "-t", id, "@agentos-name", "#12 work")
	h.Sessions().Refresh()
	row, ok := h.Session(id)
	if !ok || row.State != session.Settled || row.Hosted || row.Detail != oldDetail || row.Title != "#12 work" || row.Resumable {
		t.Fatalf("old terminal session's row = %+v (%v), want settled, not hosted, %q", row, ok, oldDetail)
	}

	h.Hook(t, id, "UserPromptSubmit", `{"prompt":"go","cwd":"`+h.Dir+`"}`)
	h.Hook(t, id, "PostToolUse", `{"tool_name":"Bash","cwd":"`+h.Dir+`"}`)
	h.Hook(t, id, "Notification", `{"message":"needs permission","notification_type":"permission_prompt","cwd":"`+h.Dir+`"}`)
	h.Hook(t, id, "UserPromptSubmit", `{"prompt":"yes","cwd":"`+h.Dir+`"}`)
	h.Hook(t, id, "Stop", `{"cwd":"`+h.Dir+`"}`)
	time.Sleep(500 * time.Millisecond)
	h.Sessions().Refresh()
	if got, _ := h.Session(id); got.State != session.Settled || got.Detail != oldDetail || got.Branch != "" {
		t.Errorf("row after its hooks = %+v, want it unchanged", got)
	}
	for _, state := range []string{"waiting", "replied", "opened", "pr"} {
		if n := h.Rec.CountAttentionFor(id, state); n != 0 {
			t.Errorf("its hooks raised %d %q alerts", n, state)
		}
	}
	if st, _ := h.Stats(7, ""); st.Total != 0 {
		t.Errorf("its hooks opened %d waits", st.Total)
	}
	if slices.Contains(h.Rec.All("awake"), any(true)) {
		t.Error("its hooks held the Mac awake")
	}

	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(prJSON("OPEN", false, "[]", 1, 0))
	h.RefreshPRs()
	time.Sleep(500 * time.Millisecond)
	if pane := h.Pane(t, id); strings.Contains(pane, "address-review") {
		t.Errorf("a PR review was typed into the old session's pane:\n%s", pane)
	}
	if got, _ := h.Session(id); got.PR != nil || got.PRAttention != "" {
		t.Errorf("row after a PR review = %+v, want no PR", got)
	}

	if err, want := h.TypeInto(id, "hello"), "Thread "+id+" runs in an old terminal"; err == nil || err.Error() != want {
		t.Errorf("TypeInto = %v, want %q", err, want)
	}
	if err, want := h.SettleThread(id), "Thread "+id+" runs in an old terminal; end it there"; err == nil || err.Error() != want {
		t.Errorf("Settle = %v, want %q", err, want)
	}
	if err := h.Interrupt(id); err == nil || err.Error() != oldChat {
		t.Errorf("Interrupt = %v, want %q", err, oldChat)
	}
	if _, err := h.ChatHistory(id, 0); err == nil || err.Error() != oldChat {
		t.Errorf("ChatHistory = %v, want %q", err, oldChat)
	}

	if err := h.DismissSession(id); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(id); ok {
		t.Error("a dismissed old session still has a row")
	}
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{pane_current_command}").Output()
	if err != nil || strings.TrimSpace(string(out)) != "sleep" {
		t.Errorf("old session's pane runs %q (%v), want sleep", out, err)
	}
}

func TestEndedOldSessionIsSettled(t *testing.T) {
	h := newHarness(t)
	id := "main/bbbbbbbb"
	rec := session.Record{Session: id, State: session.Idle, Event: "Stop", At: time.Now().Add(-time.Hour), Identity: session.Identity{Title: "old work"}}
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	row, ok := h.Session(id)
	if !ok || row.State != session.Settled || row.Hosted || row.Detail != oldDetail || row.Title != "old work" {
		t.Fatalf("ended old session's row = %+v (%v), want settled, not hosted, %q", row, ok, oldDetail)
	}
	if _, err := h.ChatHistory(id, 0); err == nil || err.Error() != oldChat {
		t.Errorf("ChatHistory = %v, want %q", err, oldChat)
	}
}
