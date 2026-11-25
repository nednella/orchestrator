package chat

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTasksFollowTheirStartsAndEnds(t *testing.T) {
	agent := Event{Kind: KindTaskStart, ID: "a1", Parent: "toolu_a", Tool: "local_agent"}
	shell := Event{Kind: KindTaskStart, ID: "b1", Parent: "toolu_b", Tool: TaskShell}
	for _, tt := range []struct {
		name    string
		events  []Event
		open    []string
		working int
	}{
		{"none", nil, nil, 0},
		{"a subagent and a shell", []Event{agent, shell}, []string{"toolu_a", "toolu_b"}, 1},
		{"a start seen twice", []Event{agent, agent}, []string{"toolu_a"}, 1},
		{"an end", []Event{agent, shell, {Kind: KindTaskEnd, Parent: "toolu_a"}}, []string{"toolu_b"}, 0},
		{"a subagent woken again", []Event{agent, {Kind: KindTaskEnd, Parent: "toolu_a"}, agent}, []string{"toolu_a"}, 1},
		{"the end of a task the turn waited on", []Event{agent, {Kind: KindTaskEnd, Parent: "toolu_c"}}, []string{"toolu_a"}, 1},
		{"Claude's exit", []Event{agent, shell, {Kind: KindExit}}, nil, 0},
		{"a new run", []Event{agent, {Kind: KindStart}}, nil, 0},
	} {
		var tasks Tasks
		for _, e := range tt.events {
			tasks.See(e)
		}
		var open []string
		for _, o := range tasks.Open() {
			open = append(open, o.Call)
		}
		if !reflect.DeepEqual(open, tt.open) || tasks.Working() != tt.working {
			t.Errorf("%s: open %v, working %d; want %v, %d", tt.name, open, tasks.Working(), tt.open, tt.working)
		}
	}
}

func TestTasksSeeReturnsTheTaskAnEndCloses(t *testing.T) {
	var tasks Tasks
	tasks.See(Event{Kind: KindTaskStart, ID: "a1", Parent: "toolu_a", Tool: "local_agent", Agent: "toolu_p"})
	closed, ok := tasks.See(Event{Kind: KindTaskEnd, Parent: "toolu_a"})
	if !ok || closed != (Task{Call: "toolu_a", ID: "a1", Kind: "local_agent", Agent: "toolu_p"}) {
		t.Fatalf("closed %+v, %v", closed, ok)
	}
	if _, ok := tasks.See(Event{Kind: KindTaskEnd, Parent: "toolu_a"}); ok {
		t.Error("a second end closed a task")
	}
}

func TestTasksNameTheSubagentThatStartedATask(t *testing.T) {
	var tasks Tasks
	tasks.See(Event{Kind: KindTaskStart, ID: "a1", Parent: "toolu_a", Tool: "local_agent"})
	for _, tt := range []struct {
		name      string
		e         Event
		wantAgent string
	}{
		{"Claude's own", Event{Kind: KindTaskStart, ID: "b1", Parent: "toolu_b", Tool: TaskShell}, ""},
		{"an open subagent's", Event{Kind: KindTaskStart, ID: "b2", Parent: "toolu_c", Tool: TaskShell, Agent: "a1"}, "toolu_a"},
		{"a subagent that is not open", Event{Kind: KindTaskStart, ID: "b3", Parent: "toolu_d", Tool: TaskShell, Agent: "a9"}, "a9"},
	} {
		if got := tasks.Named(tt.e).Agent; got != tt.wantAgent {
			t.Errorf("%s: agent %q, want %q", tt.name, got, tt.wantAgent)
		}
		tasks.See(tasks.Named(tt.e))
	}
	if got := tasks.Named(Event{Kind: KindApproval, Parent: "toolu_w", Agent: "a1"}).Agent; got != "toolu_a" {
		t.Errorf("approval of an open subagent: agent %q, want toolu_a", got)
	}
	if got := tasks.Named(Event{Kind: KindTaskEnd, Parent: "toolu_c"}).Agent; got != "toolu_a" {
		t.Errorf("end of the subagent's shell: agent %q, want toolu_a", got)
	}
	if got := tasks.Named(Event{Kind: KindTaskEnd, Parent: "toolu_x"}).Agent; got != "" {
		t.Errorf("end of a task that is not open: agent %q", got)
	}
}

func TestATaskIsReportedAloneWhenItIsClaudesOwnAndNotStopped(t *testing.T) {
	for _, tt := range []struct {
		name string
		task Task
		end  Event
		want bool
	}{
		{"Claude's own subagent", Task{Kind: "local_agent"}, Event{Kind: KindTaskEnd}, true},
		{"Claude's own shell, failed", Task{Kind: TaskShell}, Event{Kind: KindTaskEnd, Error: true}, true},
		{"stopped", Task{Kind: "local_agent"}, Event{Kind: KindTaskEnd, Error: true, Stop: TaskStopped}, false},
		{"a subagent's shell", Task{Kind: TaskShell, Agent: "toolu_a"}, Event{Kind: KindTaskEnd}, false},
	} {
		if got := tt.task.ReportedAlone(tt.end); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestClaudesListOfItsTasksCorrectsTheOpenTasks(t *testing.T) {
	list := func(tasks ...ListedTask) Event {
		t.Helper()
		input, err := json.Marshal(tasks)
		if err != nil {
			t.Fatal(err)
		}
		return Event{Kind: KindTasks, Input: input}
	}
	agent := Event{Kind: KindTaskStart, ID: "a1", Parent: "toolu_a", Tool: "local_agent"}
	listedAgent := ListedTask{ID: "a1", Kind: "local_agent"}
	for _, tt := range []struct {
		name    string
		events  []Event
		open    []Task
		working int
	}{
		{"listed just before its start", []Event{list(listedAgent), agent}, []Task{{Call: "toolu_a", ID: "a1", Kind: "local_agent"}}, 1},
		{"an end that was missed", []Event{agent, list()}, nil, 0},
		{"a task only the list names", []Event{list(ListedTask{ID: "b1", Kind: TaskShell}, ListedTask{ID: "a2", Kind: "local_agent"})}, []Task{{ID: "b1", Kind: TaskShell}, {ID: "a2", Kind: "local_agent"}}, 1},
		{"a subagent's shell", []Event{agent, list(listedAgent, ListedTask{ID: "b2", Kind: TaskShell, Agent: "a1"})}, []Task{{Call: "toolu_a", ID: "a1", Kind: "local_agent"}, {ID: "b2", Kind: TaskShell, Agent: "toolu_a"}}, 1},
		{"the end of a task only the list named", []Event{list(ListedTask{ID: "a2", Kind: "local_agent"}), {Kind: KindTaskEnd, ID: "a2", Parent: "toolu_x"}}, nil, 0},
		{"a list that cannot be read", []Event{agent, {Kind: KindTasks, Input: json.RawMessage(`{}`)}}, []Task{{Call: "toolu_a", ID: "a1", Kind: "local_agent"}}, 1},
	} {
		var tasks Tasks
		for _, e := range tt.events {
			tasks.See(e)
		}
		if got := tasks.Open(); len(got)+len(tt.open) > 0 && !reflect.DeepEqual(got, tt.open) || tasks.Working() != tt.working {
			t.Errorf("%s: open %+v, working %d; want %+v, %d", tt.name, got, tasks.Working(), tt.open, tt.working)
		}
	}
}
