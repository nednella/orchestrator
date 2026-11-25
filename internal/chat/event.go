package chat

import "encoding/json"

type Kind string

const (
	// KindStart is a host starting a run of Claude, the first event it logs for that run: the events before it are an
	// earlier run's, also when that run's host died before it logged its exit. A start with Text is the same run's Claude
	// started again without the flag Text names, which it did not know.
	KindStart      Kind = "start"
	KindInit       Kind = "init"
	KindUser       Kind = "user"
	KindDelta      Kind = "delta"
	KindMessage    Kind = "message"
	KindToolUse    Kind = "tool_use"
	KindToolResult Kind = "tool_result"
	KindApproval   Kind = "approval"
	KindAnswer     Kind = "answer"
	KindTurnEnd    Kind = "turn_end"
	KindDraft      Kind = "draft"
	KindQueued     Kind = "queued"
	KindInterrupt  Kind = "interrupt"
	KindCommands   Kind = "commands"
	// KindCompact is Claude compacting the conversation: a resume cannot reach a message from before it.
	KindCompact Kind = "compact"
	// KindThinking is a thinking block starting, with no Text, or a piece of its text as it streams; it is never logged.
	KindThinking Kind = "thinking"
	// KindThought is a thinking block that has ended, with its whole text.
	KindThought Kind = "thought"
	// KindEcho is claude taking a user message in; the host consumes it and never logs it.
	KindEcho Kind = "echo"
	// KindStillQueued is claude's reply to the interrupt request ID: Input lists the uuids of the user messages it
	// still holds. The host consumes it and never logs it.
	KindStillQueued Kind = "still_queued"
	// KindUnsent is a message that will not reach Claude: a queued one the host dropped because Claude no longer holds it,
	// or, with Error, one sent before Claude's first line to a Claude that then failed and was not started again.
	KindUnsent Kind = "unsent"
	// KindMode is Claude's permission mode, in Text, once it changes: at the owner's ask, or by Claude itself, as when
	// the owner approves its plan.
	KindMode Kind = "mode"
	// KindReply is Claude's reply to a control request of the host's, by its request ID; Error and Text say that it
	// failed and why. The host consumes it and never logs it.
	KindReply Kind = "reply"
	// KindTaskEnd is a task Claude ran beside its turn ending, as a subagent or a shell in the background: Parent is the
	// tool call that started it, Text its report, Error set when it failed or was stopped, and Stop TaskStopped when it
	// was stopped. A subagent in the background outlives its call's result and the turn that made it, so only this says
	// that it is done.
	KindTaskEnd Kind = "task_end"
	KindExit    Kind = "exit"
)

// The ways the host stops Claude itself, which an exit's Stop names.
const (
	StopSettle  = "settle"  // the thread settled
	StopRestart = "restart" // the thread starts again with a new access, which the exit's Text names
)

// TaskStopped is the Stop of a task_end for a task that was stopped.
const TaskStopped = "stopped"

// Event is one entry of a chat's log. At is unix milliseconds.
type Event struct {
	Seq    int64           `json:"seq"`
	At     int64           `json:"at"`
	Kind   Kind            `json:"kind"`
	ID     string          `json:"id,omitempty"`
	UUID   string          `json:"uuid,omitempty"`
	Parent string          `json:"parent,omitempty"`
	Text   string          `json:"text,omitempty"`
	Tool   string          `json:"tool,omitempty"`
	Input  json.RawMessage `json:"input,omitempty"`
	Error  bool            `json:"error,omitempty"`
	Cost   float64         `json:"cost,omitempty"`
	Files  []string        `json:"files,omitempty"`
	// Stop is set on an exit the host made itself, which is no error: StopSettle, or StopRestart; and on a task_end,
	// TaskStopped.
	Stop string `json:"stop,omitempty"`
	// Agent is the id of the Agent tool call whose subagent sent the event, "" for the thread's own Claude.
	Agent string `json:"agent,omitempty"`
}

// Persisted reports whether the event belongs in the log. A delta or a piece of thinking only feeds the live view, and
// so does a thought with no text, as Claude sends when it does not show its thinking: it only ends the live block.
func (e Event) Persisted() bool {
	switch e.Kind {
	case KindDelta, KindThinking:
		return false
	case KindThought:
		return e.Text != ""
	}
	return true
}
