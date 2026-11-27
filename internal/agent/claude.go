package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/nednella/orchestrator/internal/mcp"
	"github.com/nednella/orchestrator/internal/session"
	"github.com/nednella/orchestrator/internal/usage"
)

// threadTools are the commands every thread may run without asking, at every access: the ones that file an issue for
// the owner's queue or plan one in it.
var threadTools = []string{"Bash(gh issue create:*)", "Bash(gh issue comment:*)", "Bash(gh issue edit:*)"}

// browserTool drives the session's browser tab. A session with a browser may run it without asking.
const browserTool = "Bash(agent-browser:*)"

// toolTimeout is how long, in milliseconds, Claude lets an MCP tool call run: a little over the hour an orchestrator wait
// may take.
const toolTimeout = "3700000"

// Claude runs Claude Code. ConfigDir holds the user's own settings.json and the transcripts. Dirs are the app's own
// folders, which the agent may use without asking, so a fresh claude install does not prompt for them.
type Claude struct {
	ConfigDir string
	Dirs      []string
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

// Ultracode is the effort that runs claude at xhigh with its multi-agent workflow on. claude takes it as a setting, not
// as an effort, so everywhere else it is an effort like the others.
const Ultracode = "ultracode"

// effortArg is the effort claude's --effort flag gets for effort.
func effortArg(effort string) string {
	if effort == Ultracode {
		return "xhigh"
	}
	return effort
}

// FastSettings is what a running claude is told to turn its fast mode on or off. Claude takes the setting out to turn
// it off.
func FastSettings(on bool) map[string]any {
	if on {
		return map[string]any{"fastMode": true}
	}
	return map[string]any{"fastMode": nil}
}

// EffortSettings is what a running claude is told to run at effort.
func EffortSettings(effort string) map[string]any {
	return map[string]any{"effortLevel": effortArg(effort), "ultracode": effort == Ultracode}
}

// Command is claude's command line, run headless for orchestrator host.
func (c Claude) Command(l Launch) []string {
	type permissions struct {
		AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	}
	settings, _ := json.Marshal(struct {
		Permissions permissions `json:"permissions"`
		FastMode    bool        `json:"fastMode,omitempty"`
		Ultracode   bool        `json:"ultracode,omitempty"`
	}{permissions{c.Dirs}, l.Fast, l.Effort == Ultracode})
	argv := append([]string{"claude", "--settings", string(settings)}, headless...)
	if l.Model != "" {
		argv = append(argv, "--model", modelArg(l.Model))
	}
	if l.Effort != "" {
		argv = append(argv, "--effort", effortArg(l.Effort))
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
		orchestrator := mcp.ClaudeNames()
		if l.Access == session.Read {
			orchestrator = mcp.ReadNames()
		}
		allowed = append(slices.Clip(allowed), orchestrator...)
	}
	argv = append(argv, "--permission-mode", PermissionMode(l.Access), "--allowedTools", strings.Join(allowed, ","))
	if l.Prompt != "" {
		argv = append(argv, "--append-system-prompt", l.Prompt)
	}
	return argv
}

// modelArg is the model argument claude gets. Claude Code answers in plan mode with Sonnet for the alias "haiku", as
// it does for "opusplan", and shows haiku in its init line all the same. A thread can change to plan mode while it
// runs, so the alias is the full id for every access.
func modelArg(model string) string {
	if model == "haiku" {
		return usage.Haiku
	}
	return model
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

// AccessOfMode is the access claude's permission mode gives, false for a mode orchestrator does not know.
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

// Env sets claude's temp folder to the session's own, lets an orchestrator tool call run as long as a wait may, and keeps a
// checkpoint of the files each message's turn edits, which claude records only while it runs with the setting.
func (c Claude) Env(l Launch) []string {
	return []string{"CLAUDE_CODE_TMPDIR=" + l.TmpDir, "MCP_TOOL_TIMEOUT=" + toolTimeout, "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=1"}
}

// claudeAliases are the names claude takes for its latest models besides their full ids.
var claudeAliases = []string{"default", "best", "fable", "opus", "sonnet", "haiku", "opusplan"}

// claudeEfforts are the efforts claude takes, for every model. Which model gives fast mode and ultracode its effect
// is for claude to say, and the front end lists the models that do.
var claudeEfforts = []string{"low", "medium", "high", "xhigh", "max", Ultracode}

// Models are claude's aliases. claude has no command that lists its models, so a full id, which starts with
// "claude-", is taken without a list: a new model works before orchestrator knows of it.
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

// maxFolderName is how long the name of a project's folder in claude's config may be before claude cuts it.
const maxFolderName = 200

// Transcript is the file claude writes the conversation to, "" with no config folder. The folder is named from the
// folder claude started in, not the one it works in now, which a cd changes.
func (c Claude) Transcript(origin, conversation string) string {
	if c.ConfigDir == "" || origin == "" || conversation == "" {
		return ""
	}
	return filepath.Join(c.ConfigDir, "projects", folderName(origin), conversation+".jsonl")
}

// folderName is how claude names the folder of the transcripts of a project: each character that is not a letter or a
// digit becomes "-", and the name of a long path is cut and followed by the hash of the path, as claude does it. Claude
// works in UTF-16 units, so a character outside the basic plane counts for two.
func folderName(path string) string {
	var b strings.Builder
	for _, r := range path {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r > 0xFFFF:
			b.WriteString("--")
		default:
			b.WriteByte('-')
		}
	}
	name := b.String()
	if len(name) <= maxFolderName {
		return name
	}
	var hash int32
	for _, unit := range utf16.Encode([]rune(path)) {
		hash = hash<<5 - hash + int32(unit)
	}
	abs := int64(hash)
	if abs < 0 {
		abs = -abs
	}
	return name[:maxFolderName] + "-" + strconv.FormatInt(abs, 36)
}

type claudeSettings struct {
	Model         string `json:"model"`
	EffortLevel   string `json:"effortLevel"`
	ModelSettings map[string]struct {
		EffortLevel string `json:"effortLevel"`
	} `json:"modelSettings"`
}

// modelEffort is the effort set for a model in modelSettings, which names models by full id. An alias
// stands for the newest id of its family, so it takes the newest id that starts with claude-<alias>-.
func (s claudeSettings) modelEffort(model string) string {
	model = strings.TrimSuffix(model, "[1m]")
	var exact, newest string
	found := false
	for key, m := range s.ModelSettings {
		id := strings.TrimSuffix(key, "[1m]")
		if id == model {
			exact = m.EffortLevel
			found = true
		}
		if strings.HasPrefix(id, "claude-"+model+"-") && (newest == "" || newerModelID(id, strings.TrimSuffix(newest, "[1m]"))) {
			newest = key
		}
	}
	if found || !slices.Contains(claudeAliases, model) {
		return exact
	}
	return s.ModelSettings[newest].EffortLevel
}

// newerModelID reports whether id a is newer than b: segments split on "-" compare as numbers when both
// are numbers, else as strings, and an id that extends another one is newer.
func newerModelID(a, b string) bool {
	as, bs := strings.Split(a, "-"), strings.Split(b, "-")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if as[i] == bs[i] {
			continue
		}
		x, errX := strconv.Atoi(as[i])
		y, errY := strconv.Atoi(bs[i])
		if errX == nil && errY == nil {
			return x > y
		}
		return as[i] > bs[i]
	}
	return len(as) > len(bs)
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
			l.Effort = s.modelEffort(l.Model)
		}
	}
	for _, s := range files {
		if l.Effort == "" {
			l.Effort = s.EffortLevel
		}
	}
	return l
}
