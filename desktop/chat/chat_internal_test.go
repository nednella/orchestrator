package chat

import (
	"encoding/json"
	"testing"

	chatlog "github.com/nednella/agentos/internal/chat"
)

func TestToolSummaryMatchesPending(t *testing.T) {
	tests := []struct {
		name, tool, input, want string
	}{
		{"a command", "Bash", `{"command":"go test ./...\ngo vet ./...","description":"run the tests"}`, "go test ./..."},
		{"a file", "Read", `{"file_path":"/repo/main.go","limit":20}`, "/repo/main.go"},
		{"a URL", "WebFetch", `{"url":"https://example.com/docs","prompt":"read it"}`, "https://example.com/docs"},
		{"a question", "AskUserQuestion", `{"questions":[{"question":"Which one?"},{"question":"Why?"}]}`, "Which one?"},
		{"a plan", "ExitPlanMode", `{"plan":"\n  Add made.txt\nThen test"}`, "Add made.txt"},
		{"no field it knows", "mcp__agentos__threads", `{"project":"all"}`, ""},
		{"input that is not JSON", "Bash", `not json`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := json.RawMessage(tt.input)
			got := ToolSummary(tt.tool, input)
			if got != tt.want {
				t.Errorf("ToolSummary = %q, want %q", got, tt.want)
			}
			p := pendingOf("main/aaaaaaaa", chatlog.Event{Kind: chatlog.KindApproval, ID: "r1", Tool: tt.tool, Input: input})
			if p.Summary != got {
				t.Errorf("pendingOf's summary = %q, ToolSummary's %q", p.Summary, got)
			}
			if string(p.Input) != tt.input {
				t.Errorf("pendingOf's input = %s, want the approval's %s", p.Input, tt.input)
			}
		})
	}
}
