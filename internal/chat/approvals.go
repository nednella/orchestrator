package chat

import "slices"

// Waiting is the approvals that wait, oldest first, once e has happened. An approval waits until it is answered, its
// call has its result, the subagent that asks for it ends, or Claude exits or a new run starts. A subagent in the
// background asks after the turn that started it ended, so a turn's end takes only the approvals of the thread's own
// Claude, which cannot outlive its turn.
func Waiting(approvals []Event, e Event) []Event {
	switch e.Kind {
	case KindApproval:
		return append(approvals, e)
	case KindAnswer:
		return slices.DeleteFunc(approvals, func(a Event) bool { return a.ID == e.ID })
	case KindToolResult:
		// A foreground subagent's Agent call has its result when the subagent ends.
		return slices.DeleteFunc(approvals, func(a Event) bool { return e.Parent != "" && (a.Parent == e.Parent || a.Agent == e.Parent) })
	case KindTaskEnd:
		return slices.DeleteFunc(approvals, func(a Event) bool { return a.Agent != "" && a.Agent == e.Parent })
	case KindTurnEnd:
		return slices.DeleteFunc(approvals, func(a Event) bool { return a.Agent == "" })
	case KindExit, KindStart:
		return nil
	}
	return approvals
}
