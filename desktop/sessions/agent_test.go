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
	"github.com/nednella/agentos/internal/session"
)

func TestThreadStartsWithoutTheCommandOnPath(t *testing.T) {
	t.Setenv("PATH", apptest.PathWithout("agentos"))
	h := newHarness(t)
	if _, err := exec.LookPath("agentos"); err == nil {
		t.Fatal("an agentos command is still on PATH")
	}
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	args := claudeArgs(t, h, s.ID)
	if !strings.Contains(strings.Join(args, " "), apptest.CLIPath(t)+"' hook Stop") {
		t.Errorf("claude's hooks do not run the app's own binary: %q", args)
	}
	h.Hook(t, s.ID, "Stop", `{"last_assistant_message":"done"}`)
	eventually(t, "the hook to reach the app", func() bool {
		got, ok := h.Session(s.ID)
		return ok && got.State == session.Idle
	})
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
	if _, err := h.Sessions().Create("work", "", false, 0, session.Supervised); err == nil || err.Error() != "claude is not installed: it is not on PATH" {
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
