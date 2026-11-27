package thread

import (
	"cmp"
	"time"
)

// Event is a change an agent's host reports, in Claude Code's hook names.
type Event struct {
	Name             string
	NotificationType string
	Message          string
	Detail           string
	Tool             string    // the tool of a PreToolUse or PostToolUse
	Command          string    // that tool's shell command, when it has one
	Cwd              string    // where the agent was working
	Conversation     string    // the agent's id for its conversation
	Transcript       string    // the file the agent writes its conversation to
	Model            string    // the model the agent says it runs, for an agent whose transcript names none
	Asks             bool      // a Stop whose reply ends in a question to the owner
	At               time.Time // when it happened; zero for now
	Seq              int64     // the seq of the chat log entry it comes from, 0 for none
}

// Init is the event of the agent telling its conversation, folder and model, which comes after a turn's first message:
// it changes neither the state nor what the row shows of the turn.
const Init = "ThreadInit"

// TurnError is the event of a turn that ended with an error, an interrupt among them.
const TurnError = "TurnError"

// Next is the state reducer: the state after ev, given the state before it.
func Next(cur State, ev Event) State {
	switch ev.Name {
	case "SessionStart":
		return Idle
	case "Stop":
		// A thread that asks in text, not with AskUserQuestion, still waits on the owner.
		if ev.Asks {
			return Waiting
		}
		return Idle
	case TurnError:
		return Idle
	case "UserPromptSubmit", "PreToolUse", "PostToolUse":
		return Working
	case "Notification":
		return Waiting
	}
	return cur
}

// Apply builds the record ev leaves on top of the previous record.
func Apply(name string, prev Record, ev Event, now time.Time) Record {
	if prev.State == "" {
		prev = Record{Thread: name, State: Idle, Event: ev.Name, At: now, Identity: prev.Identity, Observed: prev.Observed}
	}
	if ev.Name == Init {
		prev.At, prev.Cwd = now, cmp.Or(ev.Cwd, prev.Cwd)
		prev.Conversation, prev.Transcript = cmp.Or(ev.Conversation, prev.Conversation), cmp.Or(ev.Transcript, prev.Transcript)
		prev.AgentModel, prev.Observed = cmp.Or(ev.Model, prev.AgentModel), max(prev.Observed, ev.Seq)
		return prev
	}
	rec := Record{Thread: name, State: Next(prev.State, ev), Event: ev.Name, At: now, Detail: ev.Detail, Notify: ev.NotificationType, Cwd: cmp.Or(ev.Cwd, prev.Cwd), Conversation: cmp.Or(ev.Conversation, prev.Conversation), Transcript: cmp.Or(ev.Transcript, prev.Transcript), AgentModel: cmp.Or(ev.Model, prev.AgentModel), Changes: prev.Changes, Identity: prev.Identity, Observed: max(prev.Observed, ev.Seq)}
	// A record with no changes, the first or one saved before records kept them, starts its timeline here.
	if rec.State != prev.State || len(prev.Changes) == 0 {
		rec.Changes = AddChange(prev.Changes, Change{State: rec.State, At: now.UnixMilli()})
	}
	switch ev.Name {
	case "PreToolUse", "PostToolUse":
		rec.Tool, rec.Command = ev.Tool, ev.Command
	case "Notification", "Stop", TurnError:
		// The prompt that follows a tool call is about that tool.
		rec.Tool, rec.Command = prev.Tool, prev.Command
	}
	return rec
}
