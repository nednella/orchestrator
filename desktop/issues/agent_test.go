package issues_test

import (
	"slices"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
)

const agentQueue = `    queue_sections:
      - name: Ready
        labels: [ready]
        actions:
          - {name: Work, command: "/work {n}", agent: claude}
          - {name: Opus, command: "/work {n}", agent: claude, model: opus}
`

func TestIssueSessionsGetTheirActionsAgent(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{Queue: agentQueue})
	h.GH.SetIssues(`[
		{"number":7,"title":"By the action","url":"https://x/7","labels":[{"name":"ready"}]},
		{"number":8,"title":"By the label","url":"https://x/8","labels":[{"name":"ready"},{"name":"agent:claude"}]},
		{"number":9,"title":"Unknown label","url":"https://x/9","labels":[{"name":"ready"},{"name":"agent:codex"}]},
		{"number":10,"title":"Model the provider takes","url":"https://x/10","labels":[{"name":"ready"},{"name":"agent:claude"},{"name":"model:sonnet"}]},
		{"number":11,"title":"Model the provider refuses","url":"https://x/11","labels":[{"name":"ready"},{"name":"model:other-model"}]}
	]`)

	tests := []struct {
		name          string
		issue         int
		action        string
		agent         string
		model, effort string
		flag          string
	}{
		{"the action's agent", 7, "", "claude", "", "", ""},
		{"the label over the action", 8, "", "claude", "", "", ""},
		{"a label naming no provider is ignored", 9, "", "claude", "", "", ""},
		{"a model label the provider takes", 10, "", "claude", "sonnet", "", "sonnet"},
		{"a model label the provider refuses is skipped", 11, "", "claude", "", "", ""},
		{"another action's model", 11, "Opus", "claude", "opus", "", "opus"},
	}
	for _, tt := range tests {
		s, err := h.StartIssueWith(tt.issue, tt.action)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if s.Agent != tt.agent || s.Model != tt.model || s.Effort != tt.effort {
			t.Errorf("%s: session runs %s %s %s, want %s %s %s", tt.name, s.Agent, s.Model, s.Effort, tt.agent, tt.model, tt.effort)
		}
		if model, effort := flagsOf(t, h.ClaudeDir(), s.ID); model != tt.flag || effort != "" {
			t.Errorf("%s: %s got --model %q --effort %q, want %q and none", tt.name, tt.agent, model, effort, tt.flag)
		}
		if err := h.KillSession(s.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTheQueueSaysWhichLabelsTheProviderIgnores(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{Queue: agentQueue})
	h.GH.SetIssues(`[
		{"number":7,"title":"Labels claude takes","url":"https://x/7","labels":[{"name":"ready"},{"name":"model:opus"},{"name":"effort:medium"}]},
		{"number":8,"title":"Labels claude refuses","url":"https://x/8","labels":[{"name":"ready"},{"name":"model:other-model"},{"name":"effort:hgih"}]}
	]`)
	list, err := h.Issues(true)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int][]string{
		7: {},
		8: {`model:other-model is ignored: claude has no model "other-model"`, `effort:hgih is ignored: claude takes an effort of low, medium, high, xhigh, max, not "hgih"`},
	}
	for _, is := range list {
		if !slices.Equal(is.Ignored, want[is.Number]) || is.Ignored == nil {
			t.Errorf("#%d ignored = %q, want %q", is.Number, is.Ignored, want[is.Number])
		}
	}
}
