package mcp

import (
	"strings"
	"testing"
)

func TestToolsAreNamedForTheServer(t *testing.T) {
	if Server != "orchestrator" {
		t.Errorf("Server = %q, want orchestrator", Server)
	}
	names := ClaudeNames()
	if len(names) == 0 {
		t.Fatal("no tool names")
	}
	for _, name := range names {
		if !strings.HasPrefix(name, "mcp__orchestrator__") {
			t.Errorf("%q does not start with mcp__orchestrator__", name)
		}
	}
}
