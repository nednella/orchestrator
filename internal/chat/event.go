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
	KindEcho       Kind = "echo"
	KindExit       Kind = "exit"
)

// KindEcho is claude taking a user message in; the host consumes it and never logs it.

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
