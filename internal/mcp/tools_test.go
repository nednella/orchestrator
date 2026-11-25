package mcp

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestSixToolsWithDescriptions(t *testing.T) {
	var names []string
	readOnly := map[string]bool{}
	for _, tool := range Tools() {
		names = append(names, tool.Name)
		readOnly[tool.Name] = tool.ReadOnly
		if tool.Description == "" {
			t.Errorf("%s has no description", tool.Name)
		}
	}
	if want := []string{"delegate", "threads", "read", "send", "wait", "stop"}; !slices.Equal(names, want) {
		t.Errorf("tools = %v, want %v", names, want)
	}
	for name, want := range map[string]bool{"delegate": false, "threads": true, "read": true, "send": false, "wait": true, "stop": false} {
		if readOnly[name] != want {
			t.Errorf("%s read-only = %v, want %v", name, readOnly[name], want)
		}
	}
}

func TestSchemasAreValidJSON(t *testing.T) {
	fields := map[string][]string{
		"delegate": {"task", "title", "access", "model", "effort", "project", "where", "issue"},
		"threads":  {"project"},
		"read":     {"thread", "view", "before", "limit"},
		"send":     {"thread", "text", "steer"},
		"wait":     {"thread", "timeout_seconds"},
		"stop":     {"thread"},
	}
	for _, tool := range Tools() {
		var s struct {
			Type                 string                     `json:"type"`
			Properties           map[string]json.RawMessage `json:"properties"`
			AdditionalProperties *bool                      `json:"additionalProperties"`
		}
		if err := json.Unmarshal(tool.Schema, &s); err != nil {
			t.Errorf("%s: %v", tool.Name, err)
			continue
		}
		var got []string
		for name := range s.Properties {
			got = append(got, name)
		}
		slices.Sort(got)
		want := slices.Sorted(slices.Values(fields[tool.Name]))
		if !slices.Equal(got, want) {
			t.Errorf("%s names %v, want %v", tool.Name, got, want)
		}
		if s.Type != "object" || s.AdditionalProperties == nil || *s.AdditionalProperties {
			t.Errorf("%s is not a closed object: %s", tool.Name, tool.Schema)
		}
	}
}

func TestDelegateStatesTheAccessRule(t *testing.T) {
	for _, tool := range Tools() {
		if tool.Name == "delegate" && !strings.Contains(tool.Description, "never gets more than your own access") {
			t.Errorf("delegate lacks the access rule: %s", tool.Description)
		}
	}
}
