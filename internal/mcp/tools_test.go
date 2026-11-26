package mcp

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
)

// schemaOf is a tool's schema, read as the tests need it.
type schemaOf struct {
	Type                 string                     `json:"type"`
	Properties           map[string]json.RawMessage `json:"properties"`
	Required             []string                   `json:"required"`
	AdditionalProperties *bool                      `json:"additionalProperties"`
}

func TestThirteenToolsWithDescriptions(t *testing.T) {
	var names []string
	readOnly := map[string]bool{}
	for _, tool := range Tools() {
		names = append(names, tool.Name)
		readOnly[tool.Name] = tool.ReadOnly
		if tool.Description == "" {
			t.Errorf("%s has no description", tool.Name)
		}
	}
	want := []string{"delegate", "threads", "read", "send", "wait", "stop", "note", "issue", "new_thread", "show", "track", "refresh", "browser"}
	if !slices.Equal(names, want) {
		t.Errorf("tools = %v, want %v", names, want)
	}
	for name, want := range map[string]bool{"delegate": false, "threads": true, "read": true, "send": false, "wait": true, "stop": false} {
		if readOnly[name] != want {
			t.Errorf("%s read-only = %v, want %v", name, readOnly[name], want)
		}
	}
}

func TestAgentToolsAreListed(t *testing.T) {
	tests := []struct {
		name     string
		fields   []string
		required []string
		readOnly bool
	}{
		{"note", []string{"text"}, []string{"text"}, true},
		{"issue", []string{"number", "prompt"}, []string{"number"}, true},
		{"new_thread", []string{"title", "prompt", "access"}, []string{"title", "prompt"}, true},
		{"show", []string{"file", "text", "caption"}, nil, false},
		{"track", []string{"branch"}, []string{"branch"}, false},
		{"refresh", nil, nil, true},
		{"browser", []string{"action", "url", "front", "caption", "full"}, []string{"action"}, true},
	}
	listed := map[string]Tool{}
	for _, tool := range Tools() {
		listed[tool.Name] = tool
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := listed[tt.name]
			if !ok {
				t.Fatalf("%s is not listed", tt.name)
			}
			if tool.ReadOnly != tt.readOnly {
				t.Errorf("read-only = %v, want %v", tool.ReadOnly, tt.readOnly)
			}
			var s schemaOf
			if err := json.Unmarshal(tool.Schema, &s); err != nil {
				t.Fatal(err)
			}
			if got, want := slices.Sorted(maps.Keys(s.Properties)), slices.Sorted(slices.Values(tt.fields)); !slices.Equal(got, want) {
				t.Errorf("fields = %v, want %v", got, want)
			}
			if !slices.Equal(s.Required, tt.required) {
				t.Errorf("required = %v, want %v", s.Required, tt.required)
			}
		})
	}
}

func TestAccessAndActionsAreEnums(t *testing.T) {
	for _, tt := range []struct{ tool, field, want string }{
		{"new_thread", "access", `["read","supervised","full"]`},
		{"browser", "action", `["open","screenshot","close"]`},
	} {
		for _, tool := range Tools() {
			if tool.Name != tt.tool {
				continue
			}
			var s schemaOf
			if err := json.Unmarshal(tool.Schema, &s); err != nil {
				t.Fatal(err)
			}
			var field struct{ Enum json.RawMessage }
			if err := json.Unmarshal(s.Properties[tt.field], &field); err != nil || string(field.Enum) != tt.want {
				t.Errorf("%s.%s enum = %s, %v; want %s", tt.tool, tt.field, field.Enum, err, tt.want)
			}
		}
	}
}

func TestSchemasAreValidJSON(t *testing.T) {
	fields := map[string][]string{
		"delegate":   {"task", "title", "access", "model", "effort", "project", "where", "issue"},
		"threads":    {"project"},
		"read":       {"thread", "view", "before", "limit"},
		"send":       {"thread", "text", "steer"},
		"wait":       {"thread", "timeout_seconds"},
		"stop":       {"thread"},
		"note":       {"text"},
		"issue":      {"number", "prompt"},
		"new_thread": {"title", "prompt", "access"},
		"show":       {"file", "text", "caption"},
		"track":      {"branch"},
		"refresh":    {},
		"browser":    {"action", "url", "front", "caption", "full"},
	}
	for _, tool := range Tools() {
		var s schemaOf
		if err := json.Unmarshal(tool.Schema, &s); err != nil {
			t.Errorf("%s: %v", tool.Name, err)
			continue
		}
		got := slices.Sorted(maps.Keys(s.Properties))
		want := slices.Sorted(slices.Values(fields[tool.Name]))
		if !slices.Equal(got, want) {
			t.Errorf("%s names %v, want %v", tool.Name, got, want)
		}
		if s.Type != "object" || s.AdditionalProperties == nil || *s.AdditionalProperties {
			t.Errorf("%s is not a closed object: %s", tool.Name, tool.Schema)
		}
	}
}

func TestReadNamesLeaveOutShowAndTrack(t *testing.T) {
	read := ReadNames()
	for _, name := range ClaudeNames() {
		want := name != ToolPrefix+"show" && name != ToolPrefix+"track"
		if slices.Contains(read, name) != want {
			t.Errorf("%s in ReadNames = %v, want %v", name, !want, want)
		}
	}
	if len(ClaudeNames()) != 13 || len(read) != 11 {
		t.Errorf("ClaudeNames = %v, ReadNames = %v", ClaudeNames(), read)
	}
}

func TestDelegateStatesTheAccessRule(t *testing.T) {
	for _, tool := range Tools() {
		if tool.Name == "delegate" && !strings.Contains(tool.Description, "never gets more than your own access") {
			t.Errorf("delegate lacks the access rule: %s", tool.Description)
		}
	}
}
