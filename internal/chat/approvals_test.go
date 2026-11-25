package chat

import (
	"reflect"
	"testing"
)

func TestApprovalsWaitUntilAnsweredOrDone(t *testing.T) {
	own := Event{Kind: KindApproval, ID: "r1", Parent: "toolu_1"}
	sub := Event{Kind: KindApproval, ID: "r2", Parent: "toolu_2", Agent: "toolu_a"}
	older := Event{Kind: KindApproval, ID: "r3"}
	for _, tt := range []struct {
		name string
		then Event
		want []string
	}{
		{"nothing yet", Event{Kind: KindMessage}, []string{"r1", "r2", "r3"}},
		{"an answer", Event{Kind: KindAnswer, ID: "r2"}, []string{"r1", "r3"}},
		{"Claude withdrawing it", Event{Kind: KindCancel, ID: "r3"}, []string{"r1", "r2"}},
		{"the call's result", Event{Kind: KindToolResult, Parent: "toolu_2"}, []string{"r1", "r3"}},
		{"the foreground subagent's result", Event{Kind: KindToolResult, Parent: "toolu_a"}, []string{"r1", "r3"}},
		{"a result for no call", Event{Kind: KindToolResult}, []string{"r1", "r2", "r3"}},
		{"the subagent's end", Event{Kind: KindTaskEnd, Parent: "toolu_a"}, []string{"r1", "r3"}},
		{"another task's end", Event{Kind: KindTaskEnd, Parent: "toolu_b"}, []string{"r1", "r2", "r3"}},
		{"a turn's end", Event{Kind: KindTurnEnd}, []string{"r1", "r2", "r3"}},
		{"Claude's exit", Event{Kind: KindExit}, nil},
		{"a new run", Event{Kind: KindStart}, nil},
	} {
		var waiting []Event
		for _, e := range []Event{own, sub, older, tt.then} {
			waiting = Waiting(waiting, e)
		}
		var ids []string
		for _, a := range waiting {
			ids = append(ids, a.ID)
		}
		if !reflect.DeepEqual(ids, tt.want) {
			t.Errorf("after %s: %v wait, want %v", tt.name, ids, tt.want)
		}
	}
}
