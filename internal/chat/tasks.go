package chat

import (
	"encoding/json"
	"slices"
)

// Task is a task Claude runs in the background, as its task_start says.
type Task struct {
	Call  string // the tool call that started it, "" for a task only Claude's list names so far
	ID    string // Claude's id for it
	Kind  string // TaskShell, or another kind, such as a subagent's
	Agent string // the Agent call of the subagent that started it, "" for the thread's own Claude
}

// Works reports whether the thread works while the task runs. A shell may be a server that never ends, and Claude
// itself counts its session idle while only shells run.
func (t Task) Works() bool { return t.Kind != TaskShell }

// ReportedAlone reports whether Claude tells of the end of the task in a turn of its own: it does for a task of its
// own that was not stopped. A subagent's task wakes that subagent instead, and the end of a stopped one comes with the
// next turn.
func (t Task) ReportedAlone(end Event) bool { return t.Agent == "" && end.Stop != TaskStopped }

// Tasks are the tasks a thread's Claude has open in the background, in the order they started. The zero value has none.
type Tasks struct{ open []Task }

// See follows e: a task_start opens its task, a task_end closes it, Claude's list of its tasks closes those it leaves
// out and opens those it adds, and Claude's exit or a new run's start closes them all. It returns the task a task_end
// closed. A subagent that Claude wakes again starts under the same call. Claude lists a task just before its start, and
// one whose end was missed no longer.
func (t *Tasks) See(e Event) (closed Task, ok bool) {
	switch e.Kind {
	case KindTaskStart:
		i := t.index(e)
		if i < 0 {
			t.open = append(t.open, e.asTask())
		} else if t.open[i].Call == "" {
			t.open[i] = e.asTask()
		}
	case KindTasks:
		var listed []ListedTask
		if json.Unmarshal(e.Input, &listed) != nil {
			break
		}
		t.open = slices.DeleteFunc(t.open, func(o Task) bool {
			return o.ID != "" && !slices.ContainsFunc(listed, func(l ListedTask) bool { return l.ID == o.ID })
		})
		for _, l := range listed {
			if !slices.ContainsFunc(t.open, func(o Task) bool { return o.ID == l.ID }) {
				t.open = append(t.open, t.Named(Event{Kind: KindTaskStart, ID: l.ID, Tool: l.Kind, Agent: l.Agent}).asTask())
			}
		}
	case KindTaskEnd:
		i := t.index(e)
		if i < 0 {
			return Task{}, false
		}
		closed = t.open[i]
		t.open = slices.Delete(t.open, i, i+1)
		return closed, true
	case KindExit, KindStart:
		t.open = nil
	}
	return Task{}, false
}

// Named gives e, a task's start or end or an approval, the Agent call of the subagent that started the task or asks:
// Claude names it by its own task id at a start and in an approval, which stays when that subagent is not one of the
// open tasks, and not at all at an end.
func (t *Tasks) Named(e Event) Event {
	switch e.Kind {
	case KindTaskStart, KindApproval:
		if e.Agent == "" {
			break
		}
		if i := slices.IndexFunc(t.open, func(o Task) bool { return o.ID == e.Agent && o.Call != "" }); i >= 0 {
			e.Agent = t.open[i].Call
		}
	case KindTaskEnd:
		if i := t.index(e); i >= 0 {
			e.Agent = t.open[i].Agent
		}
	}
	return e
}

// index is where the open task that e starts or ends is, -1 when it is not open: found by its call, or by Claude's id
// for one only Claude's list has named.
func (t *Tasks) index(e Event) int {
	return slices.IndexFunc(t.open, func(o Task) bool { return o.Call != "" && o.Call == e.Parent || e.ID != "" && o.ID == e.ID })
}

func (e Event) asTask() Task { return Task{Call: e.Parent, ID: e.ID, Kind: e.Tool, Agent: e.Agent} }

// Open is the open tasks, oldest first.
func (t *Tasks) Open() []Task { return slices.Clone(t.open) }

// Working is how many open tasks the thread works on.
func (t *Tasks) Working() int {
	n := 0
	for _, o := range t.open {
		if o.Works() {
			n++
		}
	}
	return n
}
