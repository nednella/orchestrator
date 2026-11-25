package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/internal/prompts"
	"github.com/nednella/agentos/internal/session"
)

// headlessFlags are what claude needs to run under agentos host.
var headlessFlags = []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--thinking-display", "summarized", "--permission-prompt-tool", "stdio", "--replay-user-messages"}

func TestClaudeCommand(t *testing.T) {
	argv := Claude{Exe: "/opt/it's here/agentos"}.Command(Launch{})
	if len(argv) != 3+len(headlessFlags)+4 || argv[0] != "claude" || argv[1] != "--settings" {
		t.Fatalf("argv = %q", argv)
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct{ Type, Command string }
		}
	}
	if err := json.Unmarshal([]byte(argv[2]), &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings.Hooks) != len(claudeEvents) {
		t.Errorf("hooks for %d events, want %d", len(settings.Hooks), len(claudeEvents))
	}
	got := settings.Hooks["Stop"][0].Hooks[0]
	if got.Type != "command" || !strings.HasSuffix(got.Command, " hook Stop") || !strings.HasPrefix(got.Command, `'/opt/it'\''s here/agentos'`) {
		t.Errorf("Stop hook = %+v", got)
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
	argv := Claude{Exe: "/x", Dirs: []string{"/data", "/local"}}.Command(Launch{})
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

const tools = "Bash(agentos issue:*),Bash(agentos new:*),Bash(agentos note:*),Bash(agentos refresh:*),Bash(agentos browser:*),Bash(gh issue create:*),Bash(gh issue comment:*),Bash(gh issue edit:*)"

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
		{"read", Launch{Access: session.Read}, []string{"--permission-mode", "plan", "--allowedTools", tools}},
		{"browser", Launch{Browser: true}, []string{"--permission-mode", "default", "--allowedTools", tools + ",Bash(agent-browser:*)"}},
		{"prompt", Launch{Prompt: "where you run"}, append(slices.Clone(supervised), "--append-system-prompt", "where you run")},
		{"all, in order", Launch{Access: session.Read, Browser: true, Prompt: "p"},
			[]string{"--permission-mode", "plan", "--allowedTools", tools + ",Bash(agent-browser:*)", "--append-system-prompt", "p"}},
	}
	for _, tt := range tests {
		argv := Claude{Exe: "/x"}.Command(tt.l)
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
		argv := Claude{Exe: "/x"}.Command(l)
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
	if argv := (Claude{Exe: "/x"}).Command(Launch{Access: "write"}); !slices.Contains(argv, "default") || slices.Contains(argv, "bypassPermissions") {
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
		t.Errorf("a mode agentos does not know gave %q", got)
	}
}

func TestClaudeForkFlags(t *testing.T) {
	argv := Claude{Exe: "/x"}.Command(Launch{Resume: "abc", ForkAt: "1203a261-38c2-4e41-b43b-07f6c4fdb1bd", Prompt: "p"})
	want := append(append([]string{"--resume", "abc", "--fork-session", "--resume-session-at", "1203a261-38c2-4e41-b43b-07f6c4fdb1bd"}, supervised...), "--append-system-prompt", "p")
	if got := argv[3+len(headlessFlags):]; !slices.Equal(got, want) {
		t.Errorf("flags = %q, want %q", got, want)
	}
	if argv := (Claude{Exe: "/x"}).Command(Launch{ForkAt: "1203a261"}); slices.Contains(argv, "--fork-session") {
		t.Errorf("a fork with no conversation to resume: argv = %q", argv)
	}
}

func TestClaudeEnv(t *testing.T) {
	if got := (Claude{}).Env(Launch{TmpDir: "/tmp/s"}); !slices.Equal(got, []string{"CLAUDE_CODE_TMPDIR=/tmp/s", "MCP_TOOL_TIMEOUT=3700000", "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=1"}) {
		t.Errorf("env = %q", got)
	}
}

func TestMCPConfigFlag(t *testing.T) {
	agentos := "mcp__agentos__delegate,mcp__agentos__threads,mcp__agentos__read,mcp__agentos__send,mcp__agentos__wait,mcp__agentos__stop"
	for _, access := range []session.Access{session.Read, session.Supervised, session.Full} {
		argv := Claude{Exe: "/x"}.Command(Launch{Access: access, MCPConfig: "/state/hosts/ab/tok.mcp.json"})
		if n := slices.Index(argv, "--mcp-config"); n < 0 || argv[n+1] != "/state/hosts/ab/tok.mcp.json" || slices.Index(argv[n+1:], "--mcp-config") >= 0 {
			t.Errorf("%s: argv = %q, want --mcp-config and its path once", access, argv)
		}
		if i := slices.Index(argv, "--allowedTools"); i < 0 || argv[i+1] != tools+","+agentos {
			t.Errorf("%s: --allowedTools = %q, want the thread tools and the six agentos tools", access, argv[i+1])
		}
	}
	argv := Claude{Exe: "/x"}.Command(Launch{})
	if slices.Contains(argv, "--mcp-config") || strings.Contains(strings.Join(argv, " "), "mcp__agentos__") {
		t.Errorf("without a config: argv = %q", argv)
	}
}

func TestCheckInstalled(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if err := CheckInstalled(Claude{Exe: "/x"}); err == nil || err.Error() != "claude is not installed: it is not on PATH" {
		t.Errorf("without claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CheckInstalled(Claude{Exe: "/x"}); err != nil {
		t.Errorf("with claude: %v", err)
	}
}

func TestClaudeParseEvent(t *testing.T) {
	tests := []struct {
		name  string
		event string
		body  string
		want  session.Event
	}{
		{"permission notification", "Notification", `{"message":"Claude needs your permission to use Bash","notification_type":"permission_prompt"}`,
			session.Event{Name: "Notification", NotificationType: "permission_prompt", Message: "Claude needs your permission to use Bash", Detail: "Claude needs your permission to use Bash"}},
		{"tool with command", "PreToolUse", `{"tool_name":"Bash","tool_input":{"command":"echo hi","description":"Print hi"}}`,
			session.Event{Name: "PreToolUse", Detail: "Bash echo hi", Tool: "Bash", Command: "echo hi"}},
		{"tool without known input", "PostToolUse", `{"tool_name":"Task","tool_input":{"x":1}}`,
			session.Event{Name: "PostToolUse", Detail: "Task", Tool: "Task"}},
		{"prompt on many lines", "UserPromptSubmit", `{"prompt":"fix\n  the   bug"}`,
			session.Event{Name: "UserPromptSubmit", Detail: "fix the bug"}},
		{"working directory", "Stop", `{"cwd":"/tmp/p"}`, session.Event{Name: "Stop", Cwd: "/tmp/p"}},
		{"conversation", "Stop", `{"session_id":"abc"}`, session.Event{Name: "Stop", Conversation: "abc"}},
		{"transcript", "Stop", `{"transcript_path":"/t/abc.jsonl"}`, session.Event{Name: "Stop", Transcript: "/t/abc.jsonl"}},
		{"not json", "Stop", `garbage`, session.Event{Name: "Stop"}},
		{"reply ends in a question", "Stop", `{"last_assistant_message":"Done.\n\n**Do you want me to push?**\n"}`,
			session.Event{Name: "Stop", Detail: "Done. **Do you want me to push?**", Asks: true}},
		{"question inside the reply", "Stop", `{"last_assistant_message":"Why? The test was flaky. I fixed it."}`,
			session.Event{Name: "Stop", Detail: "Why? The test was flaky. I fixed it."}},
		{"empty", "SessionStart", ``, session.Event{Name: "SessionStart"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reply, ok := (Claude{}).ParseEvent(tt.event, strings.NewReader(tt.body))
			if reply != "" {
				t.Errorf("reply = %q, want none for claude", reply)
			}
			if !ok || got != tt.want {
				t.Errorf("got %+v, %v, want %+v", got, ok, tt.want)
			}
		})
	}
}

func TestRegistryProvider(t *testing.T) {
	got, err := Registry{Exe: "/x", Dirs: []string{"/d"}}.Provider()
	if want := (Claude{Exe: "/x", ConfigDir: claudeConfigDir(), Dirs: []string{"/d"}}); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Provider() = %#v, %v, want %#v", got, err, want)
	}
}

func TestRegistryWithoutAgentosRefuses(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p, err := Registry{Dirs: []string{"/d"}}.Provider()
	if p != nil || err == nil || err.Error() != "The agentos command is not on PATH, so a thread's hooks cannot report its state" {
		t.Errorf("Provider() = %#v, %v", p, err)
	}
}

func TestRegistryFindsAgentosInstalledLater(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	r := Registry{Dirs: []string{"/d"}}
	if _, err := r.Provider(); err == nil {
		t.Fatal("a provider without agentos")
	}
	exe := filepath.Join(bin, "agentos")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := r.Provider()
	if want := (Claude{Exe: exe, ConfigDir: claudeConfigDir(), Dirs: []string{"/d"}}); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Provider() = %#v, %v, want %#v", got, err, want)
	}
}

func TestBrowserPromptNamesTheCommands(t *testing.T) {
	for _, want := range []string{"agentos browser help", "agentos browser open", "agentos browser screenshot", "agentos browser close"} {
		if !strings.Contains(prompts.Browser(), want) {
			t.Errorf("the prompt lacks %q", want)
		}
	}
}

func TestClaudeHeadless(t *testing.T) {
	argv := ClaudeHeadless.Argv("find things")
	if want := []string{"claude", "-p", "find things", "--allowedTools", "WebSearch WebFetch Bash(agentos digest add:*)"}; !slices.Equal(argv, want) {
		t.Errorf("argv = %q, want %q", argv, want)
	}
	env := ClaudeHeadless.Env([]string{"PATH=/bin", "ANTHROPIC_API_KEY=k", "DEPLOY_TOKEN=s", "AGENTOS_SESSION=main/9"})
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
