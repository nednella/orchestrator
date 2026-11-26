package agent

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nednella/agentos/internal/mcp"
	"github.com/nednella/agentos/internal/session"
	"github.com/nednella/agentos/internal/usage"
	"github.com/nednella/agentos/internal/util"
)

// threadTools are the commands every thread may run without asking, at every access: the ones that file an issue for
// the owner's queue or plan one in it.
var threadTools = []string{"Bash(gh issue create:*)", "Bash(gh issue comment:*)", "Bash(gh issue edit:*)"}

// browserTool drives the session's browser tab. A session with a browser may run it without asking.
const browserTool = "Bash(agent-browser:*)"

// toolTimeout is how long, in milliseconds, Claude lets an MCP tool call run: a little over the hour an agentos wait
// may take.
const toolTimeout = "3700000"

var claudeEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"Notification", "Stop", "SessionEnd",
}

// Claude runs Claude Code with hooks that call back into the agentos binary at Exe.
// The hooks come from --settings, so they apply to our sessions only.
// ConfigDir holds the user's own settings.json. Dirs are the app's own folders, which the
// agent may use without asking, so a fresh claude install does not prompt for them.
type Claude struct {
	Exe, ConfigDir string
	Dirs           []string
}

type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

func (c Claude) Name() string { return "claude" }

// headless makes claude read user messages as stream-json on stdin and write its turns as stream-json on stdout,
// partial messages too, echo each user message back when claude takes it in, and ask for each permission with a control request on stdout.
var headless = []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--include-partial-messages", ThinkingDisplay, "summarized", "--permission-prompt-tool", "stdio", "--replay-user-messages"}

// ThinkingDisplay makes claude -p stream the text of its thinking blocks, which it streams empty without it. claude does
// not list it in its help, so a claude that does not know it is started again without it.
const ThinkingDisplay = "--thinking-display"

// WithoutFlag is argv without flag and the value that follows it.
func WithoutFlag(argv []string, flag string) []string {
	i := slices.Index(argv, flag)
	if i < 0 {
		return argv
	}
	return slices.Concat(argv[:i], argv[min(i+2, len(argv)):])
}

// Command is claude's command line, run headless for agentos host.
func (c Claude) Command(l Launch) []string {
	hooks := make(map[string][]hookGroup, len(claudeEvents))
	for _, ev := range claudeEvents {
		hooks[ev] = []hookGroup{{Hooks: []hookCommand{{Type: "command", Command: util.ShellQuote(c.Exe) + " hook " + ev}}}}
	}
	type permissions struct {
		AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	}
	settings, _ := json.Marshal(struct {
		Hooks       map[string][]hookGroup `json:"hooks"`
		Permissions permissions            `json:"permissions"`
	}{hooks, permissions{c.Dirs}})
	argv := append([]string{"claude", "--settings", string(settings)}, headless...)
	if l.Model != "" {
		argv = append(argv, "--model", l.Model)
	}
	if l.Effort != "" {
		argv = append(argv, "--effort", l.Effort)
	}
	if l.Resume != "" {
		argv = append(argv, "--resume", l.Resume)
		if l.ForkAt != "" {
			argv = append(argv, "--fork-session", "--resume-session-at", l.ForkAt)
		}
	}
	allowed := threadTools
	if l.Browser {
		allowed = append(slices.Clip(allowed), browserTool)
	}
	if l.MCPConfig != "" {
		argv = append(argv, "--mcp-config", l.MCPConfig)
		agentos := mcp.ClaudeNames()
		if l.Access == session.Read {
			agentos = mcp.ReadNames()
		}
		allowed = append(slices.Clip(allowed), agentos...)
	}
	argv = append(argv, "--permission-mode", PermissionMode(l.Access), "--allowedTools", strings.Join(allowed, ","))
	if l.Prompt != "" {
		argv = append(argv, "--append-system-prompt", l.Prompt)
	}
	return argv
}

// PermissionMode is claude's mode for the access. A level it does not know is Supervised, never more.
func PermissionMode(a session.Access) string {
	switch a {
	case session.Read:
		return "plan"
	case session.Full:
		return "bypassPermissions"
	}
	return "default"
}

// AccessOfMode is the access claude's permission mode gives, false for a mode agentos does not know.
func AccessOfMode(mode string) (session.Access, bool) {
	switch mode {
	case "plan":
		return session.Read, true
	case "default", "acceptEdits":
		return session.Supervised, true
	case "bypassPermissions":
		return session.Full, true
	}
	return "", false
}

// Env sets claude's temp folder to the session's own, lets an agentos tool call run as long as a wait may, and keeps a
// checkpoint of the files each message's turn edits, which claude records only while it runs with the setting.
func (c Claude) Env(l Launch) []string {
	return []string{"CLAUDE_CODE_TMPDIR=" + l.TmpDir, "MCP_TOOL_TIMEOUT=" + toolTimeout, "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=1"}
}

// claudeAliases are the names claude takes for its latest models besides their full ids.
var claudeAliases = []string{"default", "best", "fable", "opus", "sonnet", "haiku", "opusplan"}

// claudeEfforts are the efforts claude takes, for every model.
var claudeEfforts = []string{"low", "medium", "high", "xhigh", "max"}

// Models are claude's aliases. claude has no command that lists its models, so a full id, which starts with
// "claude-", is taken without a list: a new model works before agentos knows of it.
func (c Claude) Models() []Model {
	models := make([]Model, len(claudeAliases))
	for i, alias := range claudeAliases {
		models[i] = Model{ID: alias, Efforts: claudeEfforts}
	}
	return models
}

// ValidModel takes an alias or a full id, either with the [1m] that asks for the long context.
func (c Claude) ValidModel(model, effort string) error {
	model = strings.TrimSuffix(model, "[1m]")
	models := c.Models()
	if strings.HasPrefix(model, "claude-") {
		models = append(models, Model{ID: model, Efforts: claudeEfforts})
	}
	return checkModel("claude", models, model, effort)
}

func (c Claude) Prepare(Launch) error { return nil }

func (c Claude) Usage(from usage.Mark) usage.Reader { return usage.NewCursor(from) }

type payload struct {
	NotificationType     string         `json:"notification_type"`
	Message              string         `json:"message"`
	ToolName             string         `json:"tool_name"`
	ToolInput            map[string]any `json:"tool_input"`
	Prompt               string         `json:"prompt"`
	LastAssistantMessage string         `json:"last_assistant_message"`
	Cwd                  string         `json:"cwd"`
	SessionID            string         `json:"session_id"`
	TranscriptPath       string         `json:"transcript_path"`
}

// ParseEvent reads the hook JSON from r. A body that is not JSON still gives a
// usable event, because the event name alone moves the state. Claude reads nothing from a hook that only reports.
func (c Claude) ParseEvent(name string, r io.Reader) (session.Event, string, bool) {
	var p payload
	_ = json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&p)
	ev := session.Event{Name: name, NotificationType: p.NotificationType, Message: p.Message, Cwd: p.Cwd, Conversation: p.SessionID, Transcript: p.TranscriptPath}
	switch name {
	case "Notification":
		ev.Detail = p.Message
	case "PreToolUse", "PostToolUse":
		ev.Detail = toolDetail(p)
	case "UserPromptSubmit":
		ev.Detail = p.Prompt
	case "Stop":
		ev.Detail = p.LastAssistantMessage
		ev.Asks = asks(p.LastAssistantMessage)
	}
	ev.Detail = oneLine(ev.Detail, 120)
	if name == "PreToolUse" || name == "PostToolUse" {
		ev.Tool = p.ToolName
		if cmd, ok := p.ToolInput["command"].(string); ok {
			ev.Command = oneLine(cmd, 200)
		}
	}
	return ev, "", true
}

// asks says whether a reply ends in a question to the owner.
func asks(reply string) bool { return strings.HasSuffix(strings.TrimRight(reply, " \t\n*_"), "?") }

func toolDetail(p payload) string {
	for _, key := range []string{"command", "file_path", "pattern", "url", "description"} {
		if v, ok := p.ToolInput[key].(string); ok && v != "" {
			return p.ToolName + " " + v
		}
	}
	return p.ToolName
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

type claudeSettings struct {
	Model         string `json:"model"`
	EffortLevel   string `json:"effortLevel"`
	ModelSettings map[string]struct {
		EffortLevel string `json:"effortLevel"`
	} `json:"modelSettings"`
}

// Fill reads claude's settings files, the project's local and shared ones before the user's, so the first
// that sets a value wins. A model's own effort in modelSettings wins over effortLevel. A file that is
// missing or not JSON is skipped.
func (c Claude) Fill(dir string, l Launch) Launch {
	paths := []string{filepath.Join(dir, ".claude", "settings.local.json"), filepath.Join(dir, ".claude", "settings.json")}
	if c.ConfigDir != "" {
		paths = append(paths, filepath.Join(c.ConfigDir, "settings.json"))
	}
	var files []claudeSettings
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var s claudeSettings
		if json.Unmarshal(data, &s) == nil {
			files = append(files, s)
		}
	}
	for _, s := range files {
		if l.Model == "" {
			l.Model = s.Model
		}
	}
	for _, s := range files {
		if l.Effort == "" {
			l.Effort = s.ModelSettings[l.Model].EffortLevel
		}
	}
	for _, s := range files {
		if l.Effort == "" {
			l.Effort = s.EffortLevel
		}
	}
	return l
}
