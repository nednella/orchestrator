package chat

import "slices"

// Task is a task Claude runs in the background, as its task_start says.
type Task struct {
	Call  string // the tool call that started it
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

// See follows e: a task_start opens its task, a task_end closes it, and Claude's exit or a new run's start closes them
// all. It returns the task a task_end closed. A subagent that Claude wakes again starts under the same call.
func (t *Tasks) See(e Event) (closed Task, ok bool) {
	switch e.Kind {
	case KindTaskStart:
		if !slices.ContainsFunc(t.open, func(o Task) bool { return o.Call == e.Parent }) {
			t.open = append(t.open, Task{Call: e.Parent, ID: e.ID, Kind: e.Tool, Agent: e.Agent})
		}
	case KindTaskEnd:
		i := slices.IndexFunc(t.open, func(o Task) bool { return o.Call == e.Parent })
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

// Named gives e, a task's start or end, the Agent call of the subagent that started the task: Claude names it by its
// own task id at the start, which stays when that subagent is not one of the open tasks, and not at all at the end.
func (t *Tasks) Named(e Event) Event {
	switch e.Kind {
	case KindTaskStart:
		if e.Agent == "" {
			break
		}
		if i := slices.IndexFunc(t.open, func(o Task) bool { return o.ID == e.Agent }); i >= 0 {
			e.Agent = t.open[i].Call
		}
	case KindTaskEnd:
		if i := slices.IndexFunc(t.open, func(o Task) bool { return o.Call == e.Parent }); i >= 0 {
			e.Agent = t.open[i].Agent
		}
	}
	return e
}

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
