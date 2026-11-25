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
          - {name: Work, command: "/work {n}"}
          - {name: Opus, command: "/work {n}", model: opus}
`

func TestIssueSessionsGetTheModelTheirLabelsAndActionPick(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{Queue: agentQueue})
	h.GH.SetIssues(`[
		{"number":7,"title":"No labels","url":"https://x/7","labels":[{"name":"ready"}]},
		{"number":9,"title":"An old agent label","url":"https://x/9","labels":[{"name":"ready"},{"name":"agent:codex"}]},
		{"number":10,"title":"Model claude takes","url":"https://x/10","labels":[{"name":"ready"},{"name":"model:sonnet"}]},
		{"number":11,"title":"Model claude refuses","url":"https://x/11","labels":[{"name":"ready"},{"name":"model:other-model"}]}
	]`)

	tests := []struct {
		name   string
		issue  int
		action string
		model  string
	}{
		{"no labels", 7, "", ""},
		{"an old agent label is ignored", 9, "", ""},
		{"a model label claude takes", 10, "", "sonnet"},
		{"a model label claude refuses is skipped", 11, "", ""},
		{"another action's model", 11, "Opus", "opus"},
	}
	for _, tt := range tests {
		s, err := h.StartIssueWith(tt.issue, tt.action)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if s.Model != tt.model || s.Effort != "" {
			t.Errorf("%s: session runs %s %s, want %s", tt.name, s.Model, s.Effort, tt.model)
		}
		if model, effort := flagsOf(t, h.ClaudeDir(), s.ID); model != tt.model || effort != "" {
			t.Errorf("%s: claude got --model %q --effort %q, want %q and none", tt.name, model, effort, tt.model)
		}
		if err := h.SettleThread(s.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTheQueueSaysWhichLabelsClaudeIgnores(t *testing.T) {
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
