package chat

import "slices"

// Waiting is the approvals that wait, oldest first, once e has happened. An approval waits until it is answered, Claude
// withdraws it, its call has its result, the subagent that asks for it ends, or Claude exits or a new run starts. A
// turn's end takes none: a subagent in the background asks past the turn that started it, and Claude withdraws the
// thread's own at an interrupt and rejects its call.
func Waiting(approvals []Event, e Event) []Event {
	switch e.Kind {
	case KindApproval:
		return append(approvals, e)
	case KindAnswer, KindCancel:
		return slices.DeleteFunc(approvals, func(a Event) bool { return a.ID == e.ID })
	case KindToolResult:
		// A foreground subagent's Agent call has its result when the subagent ends.
		return slices.DeleteFunc(approvals, func(a Event) bool { return e.Parent != "" && (a.Parent == e.Parent || a.Agent == e.Parent) })
	case KindTaskEnd:
		return slices.DeleteFunc(approvals, func(a Event) bool { return a.Agent != "" && a.Agent == e.Parent })
	case KindExit, KindStart:
		return nil
	}
	return approvals
}
