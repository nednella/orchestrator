package chat

import (
	"encoding/json"
	"slices"
)

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
	// KindReply is Claude's reply to a control request of the host's, by its request ID: Input is what it answered, or
	// Error and Text say that it failed and why. The host consumes it and never logs it.
	KindReply Kind = "reply"
	// KindTaskStart is a task Claude runs in the background starting, beside its turn and outliving it: Parent is the tool
	// call that started it, ID Claude's id for it, Tool its kind (TaskAgent for a subagent, TaskShell for a shell, or
	// another), Text what it does, and Agent the Agent call of the subagent that started it, "" for the thread's own
	// Claude. Parse gives that subagent by Claude's id for it, which the host turns into its Agent call.
	KindTaskStart Kind = "task_start"
	// KindTaskEnd is a task Claude ran beside its turn ending, as a subagent or a shell in the background: Parent is the
	// tool call that started it, Text its report, Error set when it failed or was stopped, and Stop TaskStopped when it
	// was stopped. A subagent in the background outlives its call's result and the turn that made it, so only this says
	// that it is done.
	KindTaskEnd Kind = "task_end"
	// KindRewind is Claude's conversation taken back to before the user's message ID: the chat drops that message and
	// everything after it, up to the rewind.
	KindRewind Kind = "rewind"
	KindExit   Kind = "exit"
	// KindTasks is the whole list of the tasks Claude runs in the background, which Claude sends at each change: Input
	// is a list of ListedTask. It corrects the open tasks that the starts and ends say, should one of those be missed.
	KindTasks Kind = "tasks"
	// KindCancel is Claude withdrawing the approval it asked for under ID, as it does at an interrupt: it no longer
	// waits on an answer.
	KindCancel Kind = "cancel"
)

// The ways the host stops Claude itself, which an exit's Stop names.
const (
	StopSettle  = "settle"  // the thread settled
	StopRestart = "restart" // the thread starts again with a new access, which the exit's Text names
)

// TaskStopped is the Stop of a task_end for a task that was stopped.
const TaskStopped = "stopped"

// TaskAgent is the kind of a subagent Claude runs as a task, and TaskShell the kind of a shell it runs in the background.
const (
	TaskAgent = "local_agent"
	TaskShell = "local_bash"
)

// ListedTask is one task of a tasks event: Claude's id for it, its kind, and the task id of the subagent that started
// it, "" for the thread's own Claude.
type ListedTask struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Agent string `json:"agent,omitempty"`
}

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
	// Agent is the id of the Agent tool call whose subagent sent the event, asked for the approval or started or ended
	// the task, "" for the thread's own Claude.
	Agent string `json:"agent,omitempty"`
	// From is the thread that sent a user or queued message, or that asked for an interrupt; "" for the owner or the
	// app.
	From string `json:"from,omitempty"`
	// Body is the bare words of a user or queued message from a thread, which Text frames for Claude, and Result marks
	// one that is a child's result.
	Body   string `json:"body,omitempty"`
	Result bool   `json:"result,omitempty"`
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

// Prompts folds e into ids, the user's messages that Claude's conversation holds, oldest first. A message that only
// denied a tool is not one of them, nor is one that a Claude that failed at its start never read; a rewind takes out
// the message it names and every one after it.
func Prompts(ids []string, e Event) []string {
	switch {
	case e.Kind == KindUser && e.Parent == "" && e.ID != "":
		return append(ids, e.ID)
	case e.Kind == KindUnsent && e.Error:
		return slices.DeleteFunc(ids, func(id string) bool { return id == e.ID })
	case e.Kind == KindRewind:
		if i := slices.Index(ids, e.ID); i >= 0 {
			return ids[:i]
		}
	}
	return ids
}
