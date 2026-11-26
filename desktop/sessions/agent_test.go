package sessions_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/mcp"
	"github.com/nednella/agentos/internal/session"
)

// mcpServerOf is the command and the environment the thread's MCP config starts the agentos tools' server with.
func mcpServerOf(t *testing.T, args []string) (command string, env map[string]string) {
	t.Helper()
	i := slices.Index(args, "--mcp-config")
	if i < 0 || i+1 >= len(args) {
		t.Fatalf("claude has no MCP config: %q", args)
	}
	data, err := os.ReadFile(args[i+1])
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Servers map[string]struct {
			Command string
			Env     map[string]string
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	server := config.Servers["agentos"]
	return server.Command, server.Env
}

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
	if command, _ := mcpServerOf(t, args); command != apptest.CLIPath(t) {
		t.Errorf("the tools' server runs %q, want the app's own binary %q", command, apptest.CLIPath(t))
	}
	// The stream tells the app the thread waits: no command runs for it.
	if _, err := h.ChatSend(s.ID, "ask"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the stream to reach the app", func() bool {
		got, ok := h.Session(s.ID)
		return ok && got.State == session.Waiting
	})
}

func TestNoHookInTheSettings(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	args := claudeArgs(t, h, s.ID)
	i := slices.Index(args, "--settings")
	if i < 0 {
		t.Fatalf("claude has no settings: %q", args)
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal([]byte(args[i+1]), &settings); err != nil {
		t.Fatal(err)
	}
	if _, ok := settings["hooks"]; ok {
		t.Errorf("claude's settings hold hooks: %s", args[i+1])
	}
	if joined := strings.Join(args, "\n"); strings.Contains(joined, " hook ") {
		t.Errorf("an argument of claude runs a hook command: %q", args)
	}
}

func TestNoBusSocket(t *testing.T) {
	h := newHarness(t)
	if _, err := h.NewSession("work", ""); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(h.State)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Type()&os.ModeSocket != 0 && e.Name() != "control.sock" {
			t.Errorf("%s is a socket in the state folder, which a hook could send a state to", e.Name())
		}
		if strings.HasSuffix(e.Name(), ".sock") && e.Name() != "control.sock" {
			t.Errorf("the state folder holds %s", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(h.State, "agentos.sock")); err == nil {
		t.Error("the state folder holds the bus socket")
	}
}

func TestMCPConfigNamesTheControlSocket(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	_, env := mcpServerOf(t, claudeArgs(t, h, s.ID))
	if want := filepath.Join(h.State, "control.sock"); env[mcp.ControlEnv] != want || env[mcp.TokenEnv] == "" {
		t.Errorf("the tools' server gets %v, want the control socket %s and a token", env, want)
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
