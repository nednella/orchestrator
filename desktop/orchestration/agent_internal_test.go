package orchestration

import (
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/mcp"
)

func TestAgentRequestMakesTheCommandsRequest(t *testing.T) {
	t.Parallel()
	const caller, project = "main/abcdefgh", "main"
	ask := func(cmd string, args []string, opts map[string]string) ctl.Request {
		return ctl.Request{Cmd: cmd, Thread: caller, Project: project, Args: args, Opts: opts}
	}
	tests := []struct {
		tool, input string
		want        ctl.Request
	}{
		{"note", `{"text":"the build is slow"}`, ask("note", []string{"the build is slow"}, map[string]string{})},
		{"issue", `{"number":7,"prompt":" the patch is in /tmp/x "}`, ask("issue", []string{"7"}, map[string]string{"prompt": "the patch is in /tmp/x"})},
		{"issue", `{"number":8}`, ask("issue", []string{"8"}, map[string]string{"prompt": ""})},
		{"new_thread", `{"title":" Fix it ","prompt":"the login fails","access":"read"}`, ask("new", []string{"Fix it"}, map[string]string{"prompt": "the login fails", "access": "read"})},
		{"show", `{"text":"done","caption":"the build"}`, ask("show", nil, map[string]string{"text": "done", "caption": "the build"})},
		{"show", `{"file":"/tmp/a/../shot.png"}`, ask("show", []string{"/tmp/shot.png"}, map[string]string{"text": "", "caption": ""})},
		{"track", `{"branch":" feature-x "}`, ask("track", nil, map[string]string{"branch": "feature-x"})},
		{"refresh", `{}`, ask("refresh", nil, map[string]string{})},
		{"browser", `{"action":"open","url":"http://127.0.0.1:18001/","front":true}`, ask("browser", []string{"open", "http://127.0.0.1:18001/"}, map[string]string{"caption": "", "front": "1"})},
		{"browser", `{"action":"screenshot","caption":"the page","full":true}`, ask("browser", []string{"screenshot"}, map[string]string{"caption": "the page", "full": "1"})},
		{"browser", `{"action":"close"}`, ask("browser", []string{"close"}, map[string]string{"caption": ""})},
	}
	for _, tt := range tests {
		t.Run(tt.tool+" "+tt.input, func(t *testing.T) {
			got, err := agentRequest(tt.tool, json.RawMessage(tt.input), caller, project)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("agentRequest = %+v, %v; want %+v", got, err, tt.want)
			}
		})
	}
}

func TestAgentRequestRefusesABadInput(t *testing.T) {
	t.Parallel()
	tests := []struct{ tool, input, err string }{
		{"note", `{"text":""}`, "The note is empty"},
		{"note", `{"text":"x","thread":"main/bbbbbbbb"}`, "The tool's input is not what its schema asks for"},
		{"note", ``, "The tool's input is not what its schema asks for"},
		{"note", `{"text":"` + strings.Repeat("x", maxNote+1) + `"}`, "The note is 65537 bytes, more than the 65536 a note takes"},
		{"show", `{"text":"x","caption":"` + strings.Repeat("x", maxCaption+1) + `"}`, "The caption is 1025 bytes, more than the 1024 a caption takes"},
		{"show", `{"file":"/a.png","caption":"` + strings.Repeat("x", maxCaption+1) + `"}`, "The caption is 1025 bytes, more than the 1024 a caption takes"},
		{"issue", `{"number":-1}`, "number is an issue's number, 1 or more"},
		{"issue", `{"number":7,"prompt":"` + strings.Repeat("x", maxPrompt+1) + `"}`, "The prompt is 6145 bytes"},
		{"new_thread", `{"title":"x","prompt":""}`, "The new thread needs a title and a prompt"},
		{"new_thread", `{"title":"x","prompt":"` + strings.Repeat("x", maxPrompt+1) + `"}`, "The prompt is 6145 bytes"},
		{"show", `{}`, "Give file or text, not both"},
		{"show", `{"file":"a.png","text":"x"}`, "Give file or text, not both"},
		{"show", `{"file":"shots/a.png"}`, "shots/a.png is not an absolute path"},
		{"track", `{"branch":"  "}`, "track needs a branch"},
		{"refresh", `{"project":"other"}`, "The tool's input is not what its schema asks for"},
		{"browser", `{"action":"open","url":" "}`, "open needs a url"},
		{"browser", `{"action":"help"}`, `action is open, screenshot or close, not "help"`},
		{"delegate", `{"task":"x"}`, `orchestrator has no tool "delegate"`},
	}
	for _, tt := range tests {
		t.Run(tt.tool+" "+tt.input, func(t *testing.T) {
			if got, err := agentRequest(tt.tool, json.RawMessage(tt.input), "main/abcdefgh", "main"); err == nil || !strings.HasPrefix(err.Error(), tt.err) {
				t.Errorf("agentRequest = %+v, %v; want %q", got, err, tt.err)
			}
		})
	}
}

func TestEveryAgentToolIsServed(t *testing.T) {
	t.Parallel()
	var served []string
	for _, tool := range mcp.Tools() {
		served = append(served, tool.Name)
	}
	for name, cmd := range agentTools() {
		if !slices.Contains(served, name) {
			t.Errorf("the MCP server does not list %s", name)
		}
		want := name
		if name == "new_thread" {
			want = "new"
		}
		if cmd != want {
			t.Errorf("%s sends %q, want %q", name, cmd, want)
		}
	}
	if got := slices.Sorted(maps.Keys(agentTools())); !slices.Equal(got, []string{"browser", "issue", "new_thread", "note", "refresh", "show", "track"}) {
		t.Errorf("agent tools = %q", got)
	}
}
