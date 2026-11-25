package sessions_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/project"
)

func sessionNumbered(t *testing.T, h *apptest.Harness, n int) sessions.Session {
	t.Helper()
	list := h.Sessions().List()
	i := slices.IndexFunc(list, func(s sessions.Session) bool { return s.N == n })
	if i < 0 {
		t.Fatalf("no session %d in %+v", n, list)
	}
	return list[i]
}

func TestNewWithAgentClaudeStartsAsWithoutTheFlag(t *testing.T) {
	h := apptest.New(t)
	if resp := h.Ask(t, ctl.Request{Cmd: "new", Args: []string{"flagged"}, Opts: map[string]string{"agent": "claude"}}); !resp.OK {
		t.Fatalf("new --agent claude = %+v", resp)
	}
	if resp := h.Ask(t, ctl.Request{Cmd: "new", Args: []string{"plain"}}); !resp.OK {
		t.Fatalf("new = %+v", resp)
	}
	flagged, bare := sessionNumbered(t, h, 1), sessionNumbered(t, h, 2)
	if flagged.Agent != "claude" || bare.Agent != "claude" {
		t.Errorf("agents = %q and %q, want claude", flagged.Agent, bare.Agent)
	}
	if a, b := claudeArgs(t, h, flagged.ID), claudeArgs(t, h, bare.ID); !slices.Equal(a, b) {
		t.Errorf("--agent claude started\n%q\nwithout the flag\n%q", a, b)
	}
}

func TestNewAndIssueRefuseAnUnknownAgent(t *testing.T) {
	h := newHarness(t)
	for _, cmd := range []string{"new", "issue"} {
		resp := h.Ask(t, ctl.Request{Cmd: cmd, Args: []string{"12"}, Opts: map[string]string{"agent": "codex"}})
		if resp.OK || !strings.Contains(resp.Error, `no agent "codex": the agents are claude`) {
			t.Errorf("%s --agent codex = %+v", cmd, resp)
		}
	}
	if len(h.Sessions().List()) != 0 {
		t.Errorf("a session started: %+v", h.Sessions().List())
	}
}

func TestAgentSettingAppliesToTheNextSession(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{Agent: "bash"})
	if got := h.Settings(); got.Agent != "bash" || !slices.Equal(got.Agents, []string{"claude"}) {
		t.Fatalf("settings = %+v", got)
	}
	plain := plainSession(h, t)
	if plain.Agent != "bash" {
		t.Errorf("agent of a session on the default = %q, want bash", plain.Agent)
	}

	if got, err := h.SetAgent(" claude "); err != nil || got.Agent != "claude" {
		t.Fatalf("SetAgent = %+v, %v", got, err)
	}
	if cfg, err := project.Load(h.Conf); err != nil || cfg.Agent != "claude" {
		t.Errorf("config = %+v, %v", cfg, err)
	}
	claude, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	if claude.Agent != "claude" {
		t.Errorf("agent of a session after the change = %q, want claude", claude.Agent)
	}
	if args := claudeArgs(t, h, claude.ID); !slices.Contains(args, "--settings") {
		t.Errorf("the session did not run the claude provider: %q", args)
	}
	if _, err := h.SetAgent("  "); err == nil {
		t.Error("an empty agent was accepted")
	}
	if got := h.Settings().Agent; got != "claude" {
		t.Errorf("a rejected agent changed the setting to %q", got)
	}
}

func TestSessionKeepsItsAgentOverARestartAndAResume(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{Agent: "bash"})
	if resp := h.Ask(t, ctl.Request{Cmd: "new", Args: []string{"#12", "work"}, Opts: map[string]string{"agent": "claude"}}); !resp.OK {
		t.Fatalf("new --agent claude = %+v", resp)
	}
	started := sessionNumbered(t, h, 1)
	claudeArgs(t, h, started.ID)
	if started.Agent != "claude" {
		t.Fatalf("agent = %q", started.Agent)
	}
	if data, _ := os.ReadFile(h.Conf); !strings.Contains(string(data), "agent_command: bash") {
		t.Fatalf("the default changed:\n%s", data)
	}

	h.App.Stop()
	h = h.Restart(t)
	if got, ok := h.Session(started.ID); !ok || got.Agent != "claude" {
		t.Errorf("after a restart the session is %+v", got)
	}

	fresh := endedWhileClosed(t, h, started.ID)
	var gone sessions.Session
	eventually(t, "the ended row", func() bool { gone, _ = fresh.Session(started.ID); return gone.State == "ended" })
	if gone.Agent != "claude" || !gone.Resumable {
		t.Errorf("ended row = %+v", gone)
	}
	resumed, err := fresh.ResumeSession(started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Agent != "claude" {
		t.Errorf("resumed row = %+v", resumed)
	}
	if args := claudeArgs(t, h, resumed.ID); !slices.Contains(args, "--resume") {
		t.Errorf("the resumed session did not run claude: %q", args)
	}
}

func TestSessionWithAnAgentNotOnPathDoesNotStart(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{Agent: "absent-agent --flag"})
	if _, err := h.Sessions().Create("work", "", false, 0, ""); err == nil || !strings.Contains(err.Error(), "absent-agent is not installed") {
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
