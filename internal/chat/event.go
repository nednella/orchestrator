package chat

import "encoding/json"

type Kind string

const (
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
	// KindEcho is claude taking a user message in; the host consumes it and never logs it.
	KindEcho Kind = "echo"
	// KindStillQueued is claude's reply to the interrupt request ID: Input lists the uuids of the user messages it
	// still holds. The host consumes it and never logs it.
	KindStillQueued Kind = "still_queued"
	// KindUnsent is a queued message the host dropped because Claude no longer holds it: it will not be sent.
	KindUnsent Kind = "unsent"
	// KindMode is Claude's permission mode, in Text, once it changes: at the owner's ask, or by Claude itself, as when
	// the owner approves its plan.
	KindMode Kind = "mode"
	// KindReply is Claude's reply to a control request of the host's, by its request ID; Error and Text say that it
	// failed and why. The host consumes it and never logs it.
	KindReply Kind = "reply"
	KindExit  Kind = "exit"
)

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
}

// Persisted reports whether the event belongs in the log; a delta only feeds the live view.
func (e Event) Persisted() bool {
	return e.Kind != KindDelta
}
