package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/nednella/orchestrator/internal/session"
	"github.com/nednella/orchestrator/internal/usage"
)

// Launch is what a session is started with besides its name. An empty field leaves the choice to the agent.
// Resume is a conversation to continue. ForkAt is the message of that conversation a new conversation forks at: it
// continues the conversation up to there under a new id. Access is what the agent may do without asking. Prompt is
// the text orchestrator adds to the agent's system prompt. Browser says the session drives its browser tab with
// agent-browser. TmpDir is the session's own temp folder. MCPConfig is the file that starts the orchestrator tools' server
// with the thread's token, "" for none.
type Launch struct {
	Model, Effort, Resume string
	ForkAt                string
	Access                session.Access
	Browser               bool
	Prompt                string
	TmpDir                string
	MCPConfig             string
}

// Provider is one agent CLI as orchestrator runs it: the command and environment a session starts with, and the way its
// transcripts are read.
type Provider interface {
	Name() string
	Command(l Launch) []string
	// Env is what the agent needs in its environment besides what every session gets.
	Env(l Launch) []string
	// Fill sets the model and effort l leaves empty to what the agent's own settings for dir choose.
	Fill(dir string, l Launch) Launch
	// Models are the models the agent takes, each with the efforts it takes for it; nil when the agent cannot say.
	Models() []Model
	// ValidModel reports a model or effort the agent would not accept; an empty value is accepted.
	ValidModel(model, effort string) error
	// Prepare makes what the agent needs before it starts and its command line cannot carry; nothing for most.
	Prepare(l Launch) error
	// Transcript is the file the agent writes the conversation to, for an agent started in the folder origin; "" when
	// it cannot say.
	Transcript(origin, conversation string) string
	// Usage reads what the agent's transcripts say it used after the mark's time, from the mark's offsets.
	Usage(from usage.Mark) usage.Reader
}

// CheckInstalled reports a provider whose command is not on PATH, naming the command.
func CheckInstalled(p Provider) error {
	program := p.Command(Launch{})[0]
	if _, err := exec.LookPath(program); err != nil {
		return fmt.Errorf("%s is not installed: it is not on PATH", program)
	}
	return nil
}

// Model is a model an agent takes, with the efforts it takes for it.
type Model struct {
	ID      string
	Efforts []string
}

// modelName keeps a value from being read as a flag when it reaches the agent's command line.
var modelName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\[\]-]*$`)

// checkModel reports a model that models leave out, or an effort the model does not take; with no model, an effort
// that none of them takes, named in the order the models give them. An empty value passes.
func checkModel(agent string, models []Model, model, effort string) error {
	if model != "" && !modelName.MatchString(model) {
		return fmt.Errorf("model %q is not a model name", model)
	}
	var efforts []string
	if model == "" {
		for _, m := range models {
			at := 0
			for _, e := range m.Efforts {
				if i := slices.Index(efforts, e); i >= 0 {
					at = i + 1
					continue
				}
				efforts = slices.Insert(efforts, at, e)
				at++
			}
		}
	} else {
		i := slices.IndexFunc(models, func(m Model) bool { return m.ID == model })
		if i < 0 {
			return fmt.Errorf("%s has no model %q", agent, model)
		}
		efforts = models[i].Efforts
	}
	if effort != "" && !slices.Contains(efforts, effort) {
		return fmt.Errorf("%s takes an effort of %s, not %q", agent, strings.Join(efforts, ", "), effort)
	}
	return nil
}

// Registry makes the provider each session starts with. Dirs are the app's folders the agent may use.
type Registry struct {
	Dirs []string
}

// Provider is claude.
func (r Registry) Provider() Provider {
	return Claude{ConfigDir: claudeConfigDir(), Dirs: r.Dirs}
}

// claudeConfigDir is where claude keeps the user's settings, "" when there is no home.
func claudeConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}
