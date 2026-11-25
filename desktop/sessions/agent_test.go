package sessions_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
)

func TestNewThreadWithoutAgentosFails(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{NoAgentos: true})
	want := "The agentos command is not on PATH, so a thread's hooks cannot report its state"
	if _, err := h.NewSession("work", "hello"); err == nil || err.Error() != want {
		t.Fatalf("NewSession = %v, want %q", err, want)
	}
	if list := h.Sessions().List(); len(list) != 0 {
		t.Errorf("the failed start left rows: %+v", list)
	}
	if out, err := exec.Command("tmux", "-L", h.Socket, "list-sessions", "-F", "#{session_name}").CombinedOutput(); err == nil && strings.TrimSpace(string(out)) != "" {
		t.Errorf("the failed start left tmux sessions: %s", out)
	}
}

func TestOldAgentKeysAreIgnored(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{
		TopExtra: "agent_command: bash\n",
		Queue:    "    queue_sections:\n      - name: Ready\n        labels: [ready]\n        actions:\n          - {name: Work, command: \"/ship {n}\", agent: agy}\n",
	})
	s, err := h.StartIssue(7)
	if err != nil {
		t.Fatal(err)
	}
	if args := claudeArgs(t, h, s.ID); !slices.Contains(args, "--settings") {
		t.Errorf("the session did not run claude: %q", args)
	}
	if data, _ := os.ReadFile(h.Conf); !strings.Contains(string(data), "agent_command: bash") {
		t.Errorf("the config lost its old key:\n%s", data)
	}
}

func TestSessionWithClaudeNotOnPathDoesNotStart(t *testing.T) {
	h := newHarness(t)
	t.Setenv("PATH", apptest.PathWithout("claude"))
	if _, err := h.Sessions().Create("work", "", false, 0); err == nil || err.Error() != "claude is not installed: it is not on PATH" {
		t.Fatalf("start error = %v", err)
	}
	if list := h.Sessions().List(); len(list) != 0 {
		t.Errorf("the failed start left rows: %+v", list)
	}
}

func TestDefaultModelIsWhatClaudesSettingsGive(t *testing.T) {
	h := apptest.New(t)
	if got := h.DefaultModel(""); got != (sessions.ModelChoice{}) {
		t.Errorf("with no settings: %+v, want none", got)
	}

	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json"), `{"model": "sonnet", "effortLevel": "low", "modelSettings": {"opus": {"effortLevel": "max"}}}`)
	tests := []struct {
		name, pick string
		want       sessions.ModelChoice
	}{
		{"the user's settings", "", sessions.ModelChoice{Model: "sonnet", Effort: "low"}},
		{"a picked model's own effort", "opus", sessions.ModelChoice{Model: "opus", Effort: "max"}},
		{"a picked model without its own effort", "haiku", sessions.ModelChoice{Model: "haiku", Effort: "low"}},
	}
	for _, tt := range tests {
		if got := h.DefaultModel(tt.pick); got != tt.want {
			t.Errorf("%s: %+v, want %+v", tt.name, got, tt.want)
		}
	}

	write(filepath.Join(h.Dir, ".claude", "settings.local.json"), `{"model": "fable"}`)
	if got, want := h.DefaultModel(""), (sessions.ModelChoice{Model: "fable", Effort: "low"}); got != want {
		t.Errorf("the project's local settings: %+v, want %+v", got, want)
	}
}
