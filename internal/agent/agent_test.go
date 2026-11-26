package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/session"
)

// headlessFlags are what claude needs to run under orchestrator host.
var headlessFlags = []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--thinking-display", "summarized", "--permission-prompt-tool", "stdio", "--replay-user-messages"}

func TestClaudeCommand(t *testing.T) {
	argv := Claude{}.Command(Launch{})
	if len(argv) != 3+len(headlessFlags)+4 || argv[0] != "claude" || argv[1] != "--settings" {
		t.Fatalf("argv = %q", argv)
	}
}

func TestNoHookInTheSettings(t *testing.T) {
	argv := Claude{Dirs: []string{"/d"}}.Command(Launch{Access: session.Full, Browser: true, MCPConfig: "/m.json", Prompt: "p"})
	var settings map[string]json.RawMessage
	if err := json.Unmarshal([]byte(argv[2]), &settings); err != nil {
		t.Fatal(err)
	}
	if _, ok := settings["hooks"]; ok || len(settings) != 1 {
		t.Errorf("the settings are %s, want only permissions", argv[2])
	}
	for _, arg := range argv {
		if strings.Contains(arg, " hook ") {
			t.Errorf("argument %q runs a hook", arg)
		}
	}
}

func TestWithoutFlagDropsItsValue(t *testing.T) {
	argv := Claude{}.Command(Launch{})
	got := WithoutFlag(argv, ThinkingDisplay)
	if slices.Contains(got, ThinkingDisplay) || slices.Contains(got, "summarized") || len(got) != len(argv)-2 {
		t.Errorf("WithoutFlag = %q", got)
	}
	if got := WithoutFlag([]string{"claude", "-p"}, ThinkingDisplay); !slices.Equal(got, []string{"claude", "-p"}) {
		t.Errorf("without the flag, argv = %q", got)
	}
}

func TestClaudeGrantsDirs(t *testing.T) {
	argv := Claude{Dirs: []string{"/data", "/local"}}.Command(Launch{})
	var settings struct {
		Permissions struct{ AdditionalDirectories []string }
	}
	if err := json.Unmarshal([]byte(argv[2]), &settings); err != nil {
		t.Fatal(err)
	}
	if got := settings.Permissions.AdditionalDirectories; !slices.Equal(got, []string{"/data", "/local"}) {
		t.Errorf("additionalDirectories = %q", got)
	}
}

// supervised is what a launch that sets nothing ends with: Supervised, and the tools every thread may run.
var supervised = []string{"--permission-mode", "default", "--allowedTools", tools}

const tools = "Bash(gh issue create:*),Bash(gh issue comment:*),Bash(gh issue edit:*)"

func TestClaudeLaunch(t *testing.T) {
	tests := []struct {
		name string
		l    Launch
		want []string
	}{
		{"none", Launch{}, supervised},
		{"model", Launch{Model: "opus"}, append([]string{"--model", "opus"}, supervised...)},
		{"effort", Launch{Effort: "high"}, append([]string{"--effort", "high"}, supervised...)},
		{"both", Launch{Model: "sonnet", Effort: "medium"}, append([]string{"--model", "sonnet", "--effort", "medium"}, supervised...)},
		{"resume", Launch{Resume: "abc"}, append([]string{"--resume", "abc"}, supervised...)},
		{"read haiku is the full id", Launch{Model: "haiku", Access: session.Read}, []string{"--model", "claude-haiku-5-5", "--permission-mode", "plan", "--allowedTools", tools}},
		{"haiku supervised is the full id", Launch{Model: "haiku"}, append([]string{"--model", "claude-haiku-5-5"}, supervised...)},
		{"haiku full is the full id", Launch{Model: "haiku", Access: session.Full}, []string{"--model", "claude-haiku-5-5", "--permission-mode", "bypassPermissions", "--allowedTools", tools}},
		{"read", Launch{Access: session.Read}, []string{"--permission-mode", "plan", "--allowedTools", tools}},
		{"browser", Launch{Browser: true}, []string{"--permission-mode", "default", "--allowedTools", tools + ",Bash(agent-browser:*)"}},
		{"prompt", Launch{Prompt: "where you run"}, append(slices.Clone(supervised), "--append-system-prompt", "where you run")},
		{"all, in order", Launch{Access: session.Read, Browser: true, Prompt: "p"},
			[]string{"--permission-mode", "plan", "--allowedTools", tools + ",Bash(agent-browser:*)", "--append-system-prompt", "p"}},
	}
	for _, tt := range tests {
		argv := Claude{}.Command(tt.l)
		if !slices.Equal(argv[3:3+len(headlessFlags)], headlessFlags) {
			t.Errorf("%s: argv = %q, want the headless flags after --settings", tt.name, argv)
		}
		if got := argv[3+len(headlessFlags):]; !slices.Equal(got, tt.want) {
			t.Errorf("%s: flags = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAccessFlags(t *testing.T) {
	modes := map[session.Access]string{session.Read: "plan", session.Supervised: "default", session.Full: "bypassPermissions"}
	base := Launch{Model: "opus", Effort: "high", Resume: "abc", Browser: true, Prompt: "p", TmpDir: "/tmp/s"}
	without := func(argv []string) []string {
		i := slices.Index(argv, "--permission-mode")
		if i < 0 {
			t.Fatalf("argv = %q, want a --permission-mode", argv)
		}
		return slices.Delete(slices.Clone(argv), i, i+2)
	}
	var rest []string
	for access, mode := range modes {
		l := base
		l.Access = access
		argv := Claude{}.Command(l)
		if i := slices.Index(argv, "--permission-mode"); argv[i+1] != mode {
			t.Errorf("%s: --permission-mode %q, want %q", access, argv[i+1], mode)
		}
		if i := slices.Index(argv, "--allowedTools"); i < 0 || argv[i+1] != tools+",Bash(agent-browser:*)" {
			t.Errorf("%s: argv = %q, want the same --allowedTools as every access", access, argv)
		}
		if rest == nil {
			rest = without(argv)
		} else if got := without(argv); !slices.Equal(got, rest) {
			t.Errorf("%s: argv besides the mode = %q, want %q", access, got, rest)
		}
		if env := (Claude{}).Env(l); !slices.Equal(env, []string{"CLAUDE_CODE_TMPDIR=/tmp/s", "MCP_TOOL_TIMEOUT=3700000", "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=1"}) {
			t.Errorf("%s: env = %q", access, env)
		}
	}
	if argv := (Claude{}).Command(Launch{Access: "write"}); !slices.Contains(argv, "default") || slices.Contains(argv, "bypassPermissions") {
		t.Errorf("a level claude does not know: argv = %q, want Supervised", argv)
	}
}

func TestAccessOfModeUndoesPermissionMode(t *testing.T) {
	for _, a := range []session.Access{session.Read, session.Supervised, session.Full} {
		if got, ok := AccessOfMode(PermissionMode(a)); !ok || got != a {
			t.Errorf("%s: AccessOfMode(%q) = %q, %v", a, PermissionMode(a), got, ok)
		}
	}
	if got, ok := AccessOfMode("acceptEdits"); !ok || got != session.Supervised {
		t.Errorf("acceptEdits = %q, %v, want Supervised", got, ok)
	}
	if got, ok := AccessOfMode("auto"); ok {
		t.Errorf("a mode orchestrator does not know gave %q", got)
	}
}

func TestClaudeForkFlags(t *testing.T) {
	argv := Claude{}.Command(Launch{Resume: "abc", ForkAt: "1203a261-38c2-4e41-b43b-07f6c4fdb1bd", Prompt: "p"})
	want := append(append([]string{"--resume", "abc", "--fork-session", "--resume-session-at", "1203a261-38c2-4e41-b43b-07f6c4fdb1bd"}, supervised...), "--append-system-prompt", "p")
	if got := argv[3+len(headlessFlags):]; !slices.Equal(got, want) {
		t.Errorf("flags = %q, want %q", got, want)
	}
	if argv := (Claude{}).Command(Launch{ForkAt: "1203a261"}); slices.Contains(argv, "--fork-session") {
		t.Errorf("a fork with no conversation to resume: argv = %q", argv)
	}
}

func TestClaudeEnv(t *testing.T) {
	if got := (Claude{}).Env(Launch{TmpDir: "/tmp/s"}); !slices.Equal(got, []string{"CLAUDE_CODE_TMPDIR=/tmp/s", "MCP_TOOL_TIMEOUT=3700000", "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=1"}) {
		t.Errorf("env = %q", got)
	}
}

// allowedTools is the --allowedTools of argv, one rule each.
func allowedTools(t *testing.T, argv []string) []string {
	t.Helper()
	i := slices.Index(argv, "--allowedTools")
	if i < 0 || i+1 >= len(argv) {
		t.Fatalf("argv = %q, want --allowedTools", argv)
	}
	return strings.Split(argv[i+1], ",")
}

func TestMCPConfigFlag(t *testing.T) {
	const orchestration = "mcp__orchestrator__delegate,mcp__orchestrator__threads,mcp__orchestrator__read,mcp__orchestrator__send,mcp__orchestrator__wait,mcp__orchestrator__stop"
	const readTools = orchestration + ",mcp__orchestrator__note,mcp__orchestrator__issue,mcp__orchestrator__new_thread,mcp__orchestrator__refresh,mcp__orchestrator__browser"
	const writeTools = orchestration + ",mcp__orchestrator__note,mcp__orchestrator__issue,mcp__orchestrator__new_thread,mcp__orchestrator__show,mcp__orchestrator__track,mcp__orchestrator__refresh,mcp__orchestrator__browser"
	for access, orchestrator := range map[session.Access]string{session.Read: readTools, session.Supervised: writeTools, session.Full: writeTools} {
		argv := Claude{}.Command(Launch{Access: access, MCPConfig: "/state/hosts/ab/tok.mcp.json"})
		if n := slices.Index(argv, "--mcp-config"); n < 0 || argv[n+1] != "/state/hosts/ab/tok.mcp.json" || slices.Index(argv[n+1:], "--mcp-config") >= 0 {
			t.Errorf("%s: argv = %q, want --mcp-config and its path once", access, argv)
		}
		if got := strings.Join(allowedTools(t, argv), ","); got != tools+","+orchestrator {
			t.Errorf("%s: --allowedTools = %q, want the thread tools and %s", access, got, orchestrator)
		}
	}
	argv := Claude{}.Command(Launch{})
	if slices.Contains(argv, "--mcp-config") || strings.Contains(strings.Join(argv, " "), "mcp__orchestrator__") {
		t.Errorf("without a config: argv = %q", argv)
	}
}

func TestReadThreadGetsOnlyTheReadTools(t *testing.T) {
	tests := []struct {
		access     session.Access
		showTrack  bool
		agentTools int
	}{
		{session.Read, false, 11},
		{session.Supervised, true, 13},
		{session.Full, true, 13},
	}
	for _, tt := range tests {
		t.Run(string(tt.access), func(t *testing.T) {
			allowed := allowedTools(t, Claude{}.Command(Launch{Access: tt.access, Browser: true, MCPConfig: "/m.json"}))
			for _, tool := range []string{"mcp__orchestrator__show", "mcp__orchestrator__track"} {
				if slices.Contains(allowed, tool) != tt.showTrack {
					t.Errorf("%s allowed = %v, want %v: %q", tool, !tt.showTrack, tt.showTrack, allowed)
				}
			}
			n := 0
			for _, rule := range allowed {
				if strings.HasPrefix(rule, "mcp__orchestrator__") {
					n++
				}
			}
			if n != tt.agentTools || !slices.Contains(allowed, "mcp__orchestrator__note") || !slices.Contains(allowed, "Bash(agent-browser:*)") {
				t.Errorf("allowed = %q, want %d orchestrator tools, note among them, and agent-browser", allowed, tt.agentTools)
			}
		})
	}
}

func TestNoAllowRuleNamesTheCommand(t *testing.T) {
	for _, access := range []session.Access{session.Read, session.Supervised, session.Full} {
		for _, l := range []Launch{{Access: access}, {Access: access, Browser: true, MCPConfig: "/m.json"}} {
			for _, rule := range allowedTools(t, Claude{}.Command(l)) {
				if strings.Contains(rule, "orchestrator ") || strings.Contains(rule, "orchestrator:") {
					t.Errorf("%s: the allow rule %q runs the orchestrator command", access, rule)
				}
			}
		}
	}
}

func TestCheckInstalled(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if err := CheckInstalled(Claude{}); err == nil || err.Error() != "claude is not installed: it is not on PATH" {
		t.Errorf("without claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CheckInstalled(Claude{}); err != nil {
		t.Errorf("with claude: %v", err)
	}
}

func TestRegistryProvider(t *testing.T) {
	got := Registry{Dirs: []string{"/d"}}.Provider()
	if want := (Claude{ConfigDir: claudeConfigDir(), Dirs: []string{"/d"}}); !reflect.DeepEqual(got, want) {
		t.Errorf("Provider() = %#v, want %#v", got, want)
	}
}

func TestClaudeTranscript(t *testing.T) {
	// The hash is the one claude's own function gives for this path.
	long := "/" + strings.Repeat("a/", 120)
	tests := []struct {
		name            string
		configDir       string
		origin, session string
		want            string
	}{
		{"a folder", "/c", "/tmp/rec", "8a2bd559", "/c/projects/-tmp-rec/8a2bd559.jsonl"},
		{"characters outside letters and digits", "/c", "/Users/a b/my_dir.v2/x", "s1", "/c/projects/-Users-a-b-my-dir-v2-x/s1.jsonl"},
		{"a character beyond the basic plane counts twice", "/c", "/a\U0001F600b", "s1", "/c/projects/-a--b/s1.jsonl"},
		{"a long path is cut and hashed", "/c", long, "s1", "/c/projects/" + strings.Repeat("-a", 100) + "-nkcon3/s1.jsonl"},
		{"no config folder", "", "/tmp/rec", "s1", ""},
		{"no folder", "/c", "", "s1", ""},
		{"no conversation", "/c", "/tmp/rec", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Claude{ConfigDir: tt.configDir}).Transcript(tt.origin, tt.session); got != tt.want {
				t.Errorf("Transcript(%q, %q) = %q, want %q", tt.origin, tt.session, got, tt.want)
			}
		})
	}
}

func TestBrowserPromptNamesTheTool(t *testing.T) {
	for _, want := range []string{"The `browser` tool's text has the rules", "`open`", "`front`", "`screenshot`", "`close`"} {
		if !strings.Contains(prompts.Browser(), want) {
			t.Errorf("the prompt lacks %q", want)
		}
	}
}

func TestClaudeHeadless(t *testing.T) {
	argv := ClaudeHeadless.Argv("find things")
	if want := []string{"claude", "-p", "find things", "--allowedTools", "WebSearch WebFetch"}; !slices.Equal(argv, want) {
		t.Errorf("argv = %q, want %q", argv, want)
	}
	env := ClaudeHeadless.Env([]string{"PATH=/bin", "ANTHROPIC_API_KEY=k", "DEPLOY_TOKEN=s", "ORCHESTRATOR_SESSION=main/9"})
	if want := []string{"PATH=/bin", "ANTHROPIC_API_KEY=k"}; !slices.Equal(env, want) {
		t.Errorf("env = %q, want %q", env, want)
	}
}

func TestClaudeFill(t *testing.T) {
	write := func(t *testing.T, path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name                string
		user, shared, local string
		l, want             Launch
	}{
		{"no settings", "", "", "", Launch{}, Launch{}},
		{"the user's", `{"model": "opus", "effortLevel": "medium"}`, "", "", Launch{}, Launch{Model: "opus", Effort: "medium"}},
		{"the project's over the user's", `{"model": "opus", "effortLevel": "medium"}`, `{"model": "sonnet"}`, "", Launch{}, Launch{Model: "sonnet", Effort: "medium"}},
		{"local over shared", "", `{"model": "sonnet"}`, `{"model": "haiku"}`, Launch{}, Launch{Model: "haiku"}},
		{"a set value stays", `{"model": "opus", "effortLevel": "medium"}`, "", "", Launch{Effort: "max", Resume: "abc"}, Launch{Model: "opus", Effort: "max", Resume: "abc"}},
		{"the model's own effort", `{"model": "claude-opus-5-5", "effortLevel": "medium", "modelSettings": {"claude-opus-5-5": {"effortLevel": "high"}}}`, "", "", Launch{}, Launch{Model: "claude-opus-5-5", Effort: "high"}},
		{"another model's effort is not used", `{"model": "opus", "effortLevel": "medium", "modelSettings": {"claude-sonnet-5-5": {"effortLevel": "high"}}}`, "", "", Launch{}, Launch{Model: "opus", Effort: "medium"}},
		{"a broken file is skipped", `{"model": "opus"}`, `{not json`, "", Launch{}, Launch{Model: "opus"}},
	}
	for _, tt := range tests {
		user, dir := t.TempDir(), t.TempDir()
		for path, body := range map[string]string{
			filepath.Join(user, "settings.json"):                 tt.user,
			filepath.Join(dir, ".claude", "settings.json"):       tt.shared,
			filepath.Join(dir, ".claude", "settings.local.json"): tt.local,
		} {
			if body != "" {
				write(t, path, body)
			}
		}
		if got := (Claude{ConfigDir: user}).Fill(dir, tt.l); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: Fill = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestClaudeValidModel(t *testing.T) {
	tests := []struct {
		name          string
		model, effort string
		ok            bool
	}{
		{"an alias", "opus", "max", true},
		{"an alias with the long context", "sonnet[1m]", "", true},
		{"a full id claude has no list for", "claude-opus-5-5", "high", true},
		{"a full id with the long context", "claude-opus-5-5[1m]", "", true},
		{"another provider's model", "other-model", "", false},
		{"an effort alone", "", "xhigh", true},
		{"a misspelt effort", "", "hgih", false},
		{"a flag", "--x", "", false},
		{"nothing", "", "", true},
	}
	for _, tt := range tests {
		if err := (Claude{}).ValidModel(tt.model, tt.effort); (err == nil) != tt.ok {
			t.Errorf("%s: ValidModel(%q, %q) = %v", tt.name, tt.model, tt.effort, err)
		}
	}
}
