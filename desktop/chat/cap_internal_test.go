package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	chatlog "github.com/nednella/orchestrator/internal/chat"
)

func TestShownCutsWhatIsTooLongForTheWindow(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", 3*maxShownText)
	bigInput := func(extra string) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"file_path": "/a/b.go", "content": big, "n": 3, "extra": extra})
		return b
	}
	tests := []struct {
		name   string
		in     chatlog.Event
		cut    bool
		check  func(t *testing.T, got chatlog.Event)
		sameAs bool
	}{
		{name: "a short result", in: chatlog.Event{Kind: chatlog.KindToolResult, Text: "ok"}, sameAs: true},
		{name: "a result at the limit", in: chatlog.Event{Kind: chatlog.KindToolResult, Text: strings.Repeat("y", maxShownText)}, sameAs: true},
		{name: "a long result", in: chatlog.Event{Kind: chatlog.KindToolResult, Text: big}, cut: true, check: func(t *testing.T, got chatlog.Event) {
			if len(got.Text) != maxShownText+len(cutMarker) || !strings.HasSuffix(got.Text, "(cut at 64 KB)") {
				t.Errorf("text is %d bytes and ends %q", len(got.Text), got.Text[len(got.Text)-20:])
			}
		}},
		{name: "a result cut inside a character", in: chatlog.Event{Kind: chatlog.KindToolResult, Text: strings.Repeat("é", maxShownText)}, cut: true, check: func(t *testing.T, got chatlog.Event) {
			if !utf8.ValidString(got.Text) {
				t.Error("the cut text is not valid UTF-8")
			}
		}},
		{name: "a short input", in: chatlog.Event{Kind: chatlog.KindToolUse, Tool: "Bash", Input: json.RawMessage(`{"command":"ls"}`)}, sameAs: true},
		{name: "a long input", in: chatlog.Event{Kind: chatlog.KindToolUse, Tool: "Write", Input: bigInput("")}, cut: true, check: func(t *testing.T, got chatlog.Event) {
			var in map[string]any
			if err := json.Unmarshal(got.Input, &in); err != nil {
				t.Fatal(err)
			}
			content, _ := in["content"].(string)
			if in["file_path"] != "/a/b.go" || in["n"] != float64(3) || len(content) != maxShownText+len(cutMarker) || !strings.HasSuffix(content, "(cut at 64 KB)") {
				t.Errorf("input = file_path %v, n %v, content of %d bytes", in["file_path"], in["n"], len(content))
			}
		}},
		{name: "an input that is no object", in: chatlog.Event{Kind: chatlog.KindToolUse, Input: json.RawMessage(`"` + big + `"`)}, cut: true, check: func(t *testing.T, got chatlog.Event) {
			if !json.Valid(got.Input) || len(got.Input) > maxShownText+100 {
				t.Errorf("input is %d bytes, valid %v", len(got.Input), json.Valid(got.Input))
			}
		}},
		{name: "another kind is left alone", in: chatlog.Event{Kind: chatlog.KindMessage, Text: big}, sameAs: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []chatlog.Event{{Kind: chatlog.KindUser, Text: "hi"}, tt.in}
			got := shown(events)
			if got[1].Cut != tt.cut {
				t.Fatalf("Cut = %v, want %v", got[1].Cut, tt.cut)
			}
			if tt.sameAs && (got[1].Text != tt.in.Text || string(got[1].Input) != string(tt.in.Input)) {
				t.Error("an event that fits was changed")
			}
			if tt.check != nil {
				tt.check(t, got[1])
			}
			if events[1].Cut || (tt.cut && len(events[1].Text)+len(events[1].Input) < maxShownText) {
				t.Error("the log's own event was changed")
			}
		})
	}
}
