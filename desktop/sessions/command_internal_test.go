package sessions

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/internal/agent"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/prompts"
	"github.com/nednella/agentos/internal/session"
)

type fakeBrowsers struct{ ready bool }

func (f fakeBrowsers) AgentReady() bool                          { return f.ready }
func (f fakeBrowsers) AgentEnv(string, string) ([]string, error) { return nil, nil }
func (f fakeBrowsers) Has(string) bool                           { return false }
func (f fakeBrowsers) Close(context.Context, string)             {}
func (f fakeBrowsers) Forget(context.Context, string)            {}
func (f fakeBrowsers) PIDs() []int                               { return nil }

// commandFor is the argv claude starts with.
func (s *Sessions) commandFor(proj project.Project, model project.Model, conversation, brief, handover string, access session.Access) []string {
	return agent.Claude{Exe: "/x"}.Command(launchFor(model, conversation, brief, handover, access, s.hasBrowser(proj, brief), forkStart{}, workTree{}, ""))
}

// allowedTools is the --allowedTools value in argv, "" when there is none.
func allowedTools(argv []string) string {
	if i := slices.Index(argv, "--allowedTools"); i >= 0 {
		return argv[i+1]
	}
	return ""
}

func TestClaudeGetsTheBrowserToolsAndPrompt(t *testing.T) {
	off := false
	tests := []struct {
		name    string
		proj    project.Project
		browser bool
		want    bool
	}{
		{"claude with a browser", project.Project{Name: "p"}, true, true},
		{"project turns it off", project.Project{Name: "p", Browser: &off}, true, false},
		{"no browser or agent-browser", project.Project{Name: "p"}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sessions{browsers: fakeBrowsers{tt.browser}}
			argv := s.commandFor(tt.proj, project.Model{}, "", "", "", session.Supervised)
			if strings.Contains(allowedTools(argv), "Bash(agent-browser:*)") != tt.want {
				t.Fatalf("argv = %q", argv)
			}
			prompt := slices.Index(argv, "--append-system-prompt")
			if prompt < 0 {
				t.Fatalf("argv = %q", argv)
			}
			if !strings.HasPrefix(argv[prompt+1], prompts.Session()+"\n\n"+prompts.Thread()) {
				t.Errorf("system prompt = %q", argv[prompt+1])
			}
			if strings.Contains(argv[prompt+1], prompts.Browser()) != tt.want {
				t.Errorf("system prompt = %q", argv[prompt+1])
			}
		})
	}
}

func TestCommandResumesAConversation(t *testing.T) {
	s := &Sessions{browsers: fakeBrowsers{}}
	argv := s.commandFor(project.Project{Name: "p"}, project.Model{}, "abc", "", "", session.Supervised)
	if i := slices.Index(argv, "--resume"); i < 0 || argv[i+1] != "abc" {
		t.Errorf("argv = %q", argv)
	}
}

func TestConversationOfARunningAndAnEndedSession(t *testing.T) {
	written := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(written, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Sessions{
		records: map[string]session.Record{"p/1": {Conversation: "live", Transcript: written}, "p/4": {Conversation: "unwritten", Transcript: written + ".none"}},
		ended:   map[string]*endedSession{"p/2": {rec: session.Record{Conversation: "gone", Transcript: written}}},
	}
	if got := s.conversationOf("p/1"); got != "live" {
		t.Errorf("running: %q", got)
	}
	if got := s.conversationOf("p/2"); got != "gone" {
		t.Errorf("ended: %q", got)
	}
	if got := s.conversationOf("p/3"); got != "" {
		t.Errorf("unknown: %q", got)
	}
	if got := s.conversationOf("p/4"); got != "" {
		t.Errorf("no transcript: %q", got)
	}
}

func TestCommandGivesABriefedSessionOnlyTheBrief(t *testing.T) {
	s := &Sessions{browsers: fakeBrowsers{ready: true}}
	argv := s.commandFor(project.Project{Name: "p"}, project.Model{}, "", "set it up", "", session.Supervised)
	if i := slices.Index(argv, "--append-system-prompt"); i < 0 || argv[i+1] != "set it up" || strings.Contains(allowedTools(argv), "agent-browser") {
		t.Errorf("argv = %q", argv)
	}
}

func TestHostedRefusesASocketPathTooLong(t *testing.T) {
	name := session.Name{Project: "p", Token: session.NewToken()}
	fits := &Sessions{stateDir: "/" + strings.Repeat("s", 73), agents: agent.Registry{Exe: "/x"}}
	if got := len(fits.hostSocket(name)); got != 103 {
		t.Fatalf("socket path is %d bytes, want 103", got)
	}
	long := &Sessions{stateDir: filepath.Join(t.TempDir(), strings.Repeat("s", 100)), agents: agent.Registry{Exe: "/x"}}
	_, err := long.hosted(name, []string{"claude"})
	if err == nil || !strings.HasPrefix(err.Error(), "the session socket path is too long: ") {
		t.Errorf("err = %v", err)
	}
	if _, statErr := os.Stat(long.stateDir); !os.IsNotExist(statErr) {
		t.Errorf("the refused session made its folder: %v", statErr)
	}
}

func TestCommandEndsATasksPromptWithTheHandover(t *testing.T) {
	s := &Sessions{browsers: fakeBrowsers{}}
	argv := s.commandFor(project.Project{Name: "p"}, project.Model{}, "", "", "the patch is in /tmp/x", session.Supervised)
	want := prompts.Session() + "\n\n" + prompts.Thread() + "\n\n" + prompts.Handover("the patch is in /tmp/x")
	if i := slices.Index(argv, "--append-system-prompt"); i < 0 || argv[i+1] != want {
		t.Errorf("argv = %q", argv)
	}
}

func TestCommandStartsAReadThreadInPlanModeWithTheBrowser(t *testing.T) {
	s := &Sessions{browsers: fakeBrowsers{ready: true}}
	argv := s.commandFor(project.Project{Name: "p", Dir: "/work/p"}, project.Model{}, "", "", "", session.Read)
	if i := slices.Index(argv, "--permission-mode"); i < 0 || argv[i+1] != "plan" {
		t.Errorf("no plan mode: %q", argv)
	}
	const tools = "Bash(agentos issue:*),Bash(agentos new:*),Bash(agentos note:*),Bash(agentos refresh:*),Bash(agentos browser:*),Bash(gh issue create:*),Bash(gh issue comment:*),Bash(gh issue edit:*),Bash(agent-browser:*)"
	if got := allowedTools(argv); got != tools {
		t.Errorf("allowed tools = %q", got)
	}
	want := prompts.Session() + "\n\n" + prompts.Thread() + "\n\n" + prompts.Browser()
	if i := slices.Index(argv, "--append-system-prompt"); i < 0 || argv[i+1] != want {
		t.Errorf("system prompt = %q", argv)
	}

	write := s.commandFor(project.Project{Name: "p", Dir: "/work/p"}, project.Model{}, "", "", "", session.Supervised)
	if i := slices.Index(write, "--permission-mode"); i < 0 || write[i+1] != "default" || allowedTools(write) != tools {
		t.Errorf("a Supervised thread: %q", write)
	}
}
