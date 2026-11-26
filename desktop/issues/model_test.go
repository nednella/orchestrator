package issues_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/usage"
)

// argsOf is the arguments a session's claude was started with, one per line.
func argsOf(t *testing.T, dir, id string) string {
	t.Helper()
	var data []byte
	eventually(t, "claude started", func() bool {
		var err error
		data, err = os.ReadFile(filepath.Join(dir, strings.ReplaceAll(id, "/", "_")+".args"))
		return err == nil && len(data) > 0
	})
	return strings.TrimSpace(string(data))
}

// flagsOf is the --model and --effort values a session's claude was started with.
// claudeModel is the --model argument claude gets for a model the thread was given.
func claudeModel(model string) string {
	if model == "haiku" {
		return usage.Haiku
	}
	return model
}

func flagsOf(t *testing.T, dir, id string) (model, effort string) {
	t.Helper()
	args := strings.Split(argsOf(t, dir, id), "\n")
	for _, flag := range []string{"--model", "--effort"} {
		v := ""
		if i := slices.Index(args, flag); i >= 0 {
			v = args[i+1]
		}
		if flag == "--model" {
			model = v
		} else {
			effort = v
		}
	}
	return model, effort
}

func TestIssueSessionsGetTheirActionsModel(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)

	tests := []struct {
		name          string
		issue         int
		action        string
		model, effort string
	}{
		{"an action with no model passes none", 11, "", "", ""},
		{"an action's model and effort", 7, "Plan", "opus", "high"},
		{"the default action", 8, "", "opus", ""},
		{"labels win", 12, "", "haiku", "high"},
	}
	for _, tt := range tests {
		s, err := h.StartIssueWith(tt.issue, tt.action)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if s.Model != tt.model || s.Effort != tt.effort {
			t.Errorf("%s: session runs %s %s, want %s %s", tt.name, s.Model, s.Effort, tt.model, tt.effort)
		}
		if model, effort := flagsOf(t, h.ClaudeDir(), s.ID); model != claudeModel(tt.model) || effort != tt.effort {
			t.Errorf("%s: claude got --model %q --effort %q, want %q %q", tt.name, model, effort, tt.model, tt.effort)
		}
	}
}

func TestSessionWithoutAnIssueGetsNoModelFlags(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.NewSession("plain", "")
	if err != nil {
		t.Fatal(err)
	}
	if model, effort := flagsOf(t, h.ClaudeDir(), s.ID); model != "" || effort != "" || s.Model != "" || s.Effort != "" {
		t.Errorf("claude got %q %q, session %q %q, want none", model, effort, s.Model, s.Effort)
	}
}

func TestSessionsGetClaudesOwnSettings(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	user := h.ClaudeConfigDir()
	if err := os.MkdirAll(user, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(user, "settings.json"), []byte(`{"model": "sonnet", "effortLevel": "low"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		start         func() (sessions.Session, error)
		model, effort string
	}{
		{"a session with no issue", func() (sessions.Session, error) { return h.NewSession("plain", "") }, "sonnet", "low"},
		{"the action's model wins", func() (sessions.Session, error) { return h.StartIssueWith(8, "") }, "opus", "low"},
		{"labels win", func() (sessions.Session, error) { return h.StartIssueWith(12, "") }, "haiku", "high"},
	}
	for _, tt := range tests {
		s, err := tt.start()
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if model, effort := flagsOf(t, h.ClaudeDir(), s.ID); model != claudeModel(tt.model) || effort != tt.effort || s.Model != tt.model || s.Effort != tt.effort {
			t.Errorf("%s: claude got %q %q, session %q %q, want %s %s", tt.name, model, effort, s.Model, s.Effort, tt.model, tt.effort)
		}
	}
}

func TestModelSurvivesARestartAndReachesTheTally(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.StartIssue(8)
	if err != nil {
		t.Fatal(err)
	}
	h.Observe(t, s.ID, "Notification", `{"message":"Claude needs your permission"}`)
	eventually(t, "a wait with the model", func() bool {
		st, _ := h.Stats(7, "")
		return len(st.Recent) == 1 && st.Recent[0].Model == "opus"
	})

	again := h.Restart(t)
	got, ok := again.Session(s.ID)
	if !ok || got.Model != "opus" || got.Effort != "" {
		t.Errorf("after a restart: %+v, found %v", got, ok)
	}

	if err := again.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the ended row keeps its model", func() bool {
		ended, ok := again.Session(s.ID)
		return ok && ended.State == "settled" && ended.Model == "opus"
	})
}

func TestOwnersPromptFollowsTheActionsCommandAndTheirModelWins(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)

	s, err := h.StartIssueOwn(7, "", "be careful", "haiku", "low")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the command then the prompt on claude's stdin", func() bool { return sent(t, h, s.ID, "/ship 7 be careful") })
	if model, effort := flagsOf(t, h.ClaudeDir(), s.ID); model != usage.Haiku || effort != "low" {
		t.Errorf("claude got --model %q --effort %q, want haiku low", model, effort)
	}

	plan, err := h.StartIssueOwn(8, "", "", "", "max")
	if err != nil {
		t.Fatal(err)
	}
	if model, effort := flagsOf(t, h.ClaudeDir(), plan.ID); model != "opus" || effort != "max" {
		t.Errorf("claude got --model %q --effort %q, want the action's model and the owner's effort", model, effort)
	}
}

func TestOwnersPromptDoesNotReachALiveSession(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	first, err := h.StartIssueOwn(7, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	again, err := h.StartIssueOwn(7, "", "type this", "haiku", "")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID {
		t.Fatalf("started %s, want the live session %s", again.ID, first.ID)
	}
	time.Sleep(500 * time.Millisecond)
	events, _ := h.ChatHistory(first.ID, 0)
	if stdin := strings.Join(h.Stdin(t, first.ID), "\n"); strings.Contains(stdin, "type this") || slices.ContainsFunc(events, func(e chatlog.Event) bool { return strings.Contains(e.Text, "type this") }) {
		t.Errorf("the prompt reached the live session: stdin %q, chat %+v", stdin, events)
	}
}
