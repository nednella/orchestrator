package project

import (
	"errors"
	"slices"
	"testing"

	"github.com/nednella/orchestrator/internal/agent"
)

func TestPick(t *testing.T) {
	plan := Action{Name: "Plan", Model: "opus"}
	claude := agent.Claude{}.ValidModel
	noHaiku := func(model, effort string) error {
		if model == "haiku" {
			return errors.New("no haiku")
		}
		return claude(model, effort)
	}
	tests := []struct {
		name    string
		action  Action
		labels  []string
		valid   func(model, effort string) error
		want    Model
		ignored []string
	}{
		{"nothing set passes nothing", Action{}, nil, claude, Model{}, nil},
		{"the action's", plan, nil, claude, Model{Model: "opus", Effort: ""}, nil},
		{"labels over action, field by field", Action{Name: "Plan", Model: "opus", Effort: "low"}, []string{"type:bug", "model:sonnet", "effort:max"}, claude, Model{Model: "sonnet", Effort: "max"}, nil},
		{"a label fills what the action leaves unset", plan, []string{"effort:high"}, claude, Model{Model: "opus", Effort: "high"}, nil},
		{"first label of a kind wins", Action{}, []string{"model:opus", "model:haiku"}, claude, Model{Model: "opus", Effort: ""}, nil},
		{"bad labels are skipped", plan, []string{"model:--x", "effort:huge"}, claude, Model{Model: "opus", Effort: ""}, []string{
			`model:--x is ignored: model "--x" is not a model name`,
			`effort:huge is ignored: claude takes an effort of low, medium, high, xhigh, max, ultracode, not "huge"`,
		}},
		{"a label the provider rejects is skipped", plan, []string{"model:haiku", "effort:high"}, noHaiku, Model{Model: "opus", Effort: "high"}, []string{"model:haiku is ignored: no haiku"}},
		{"a model of another provider is skipped", plan, []string{"model:other-model"}, claude, Model{Model: "opus", Effort: ""}, []string{`model:other-model is ignored: claude has no model "other-model"`}},
	}
	for _, tt := range tests {
		got, ignored := Pick(tt.action, tt.labels, tt.valid)
		if got != tt.want {
			t.Errorf("%s: Pick = %+v, want %+v", tt.name, got, tt.want)
		}
		if !slices.Equal(ignored, tt.ignored) {
			t.Errorf("%s: ignored = %q, want %q", tt.name, ignored, tt.ignored)
		}
	}
}

func TestFastModeIsKeptOverABase(t *testing.T) {
	if got := (Model{Fast: true}).over(Model{Model: "opus"}); got != (Model{Model: "opus", Fast: true}) {
		t.Errorf("over = %+v, want opus, fast", got)
	}
	if got := (Model{Model: "sonnet"}).over(Model{Fast: true}); got != (Model{Model: "sonnet", Fast: true}) {
		t.Errorf("over = %+v, want sonnet, fast from the base", got)
	}
}
