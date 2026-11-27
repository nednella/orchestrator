package chat

import (
	"encoding/json"
	"strings"
	"time"

	chatlog "github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/thread"
)

// turn is what stateEvent keeps of the events of a run before the one it reads: the tools called and not yet answered,
// the approvals that wait, and the thread's own latest reply in the turn.
type turn struct {
	transcript func(origin, conversation string) string
	tools      map[string]call // by the tool use's id
	approvals  map[string]call // the thread's own approvals that wait, by their request's id
	reply      string
}

// call is a tool call: the tool, the shell command it runs when it has one, and what it does in a line.
type call struct{ tool, command, detail string }

func callOf(tool string, input json.RawMessage) call {
	return call{tool, commandOf(input), oneLine(toolDetail(tool, input), 120)}
}

func newTurn(transcript func(origin, conversation string) string) *turn {
	return &turn{transcript: transcript, tools: map[string]call{}, approvals: map[string]call{}}
}

func (t *turn) reset() {
	clear(t.tools)
	clear(t.approvals)
	t.reply = ""
}

// stateEvent is the change of the thread's state that the logged event e makes, in the names of Claude's hooks. It
// reports false for an event that changes none. A subagent's approval is not one: Background tells of those.
func stateEvent(e chatlog.Event, t *turn) (thread.Event, bool) {
	ev := thread.Event{At: time.UnixMilli(e.At), Seq: e.Seq}
	switch e.Kind {
	case chatlog.KindStart:
		t.reset()
		ev.Name = "SessionStart"
	case chatlog.KindInit:
		ev.Name = thread.Init
		ev.Conversation, ev.Cwd, ev.Model = e.ID, e.Cwd, e.Model
		ev.Transcript = t.transcript(e.Origin, e.ID)
	case chatlog.KindUser:
		// A message that only denies an approval starts no turn.
		if e.Parent != "" {
			return thread.Event{}, false
		}
		ev.Name, ev.Detail = "UserPromptSubmit", oneLine(e.Text, 120)
	case chatlog.KindToolUse:
		c := callOf(e.Tool, e.Input)
		t.tools[e.ID] = c
		ev.Name, ev.Tool, ev.Command, ev.Detail = "PreToolUse", c.tool, c.command, c.detail
	case chatlog.KindToolResult:
		c := t.tools[e.Parent]
		delete(t.tools, e.Parent)
		ev.Name, ev.Tool, ev.Command, ev.Detail = "PostToolUse", c.tool, c.command, c.detail
	case chatlog.KindApproval:
		if e.Agent != "" {
			return thread.Event{}, false
		}
		t.approvals[e.ID] = callOf(e.Tool, e.Input)
		ev.Name, ev.NotificationType = "Notification", "permission_prompt"
		ev.Message = oneLine(strings.TrimSpace(e.Tool+" "+ToolSummary(e.Tool, e.Input)), 120)
		ev.Detail = ev.Message
	case chatlog.KindAnswer:
		// The tool goes on, or Claude goes on without it: the thread works again once no approval waits.
		c, ok := t.approvals[e.ID]
		delete(t.approvals, e.ID)
		if !ok || len(t.approvals) > 0 {
			return thread.Event{}, false
		}
		ev.Name, ev.Tool, ev.Command, ev.Detail = "PreToolUse", c.tool, c.command, c.detail
	case chatlog.KindCancel:
		delete(t.approvals, e.ID)
		return thread.Event{}, false
	case chatlog.KindMessage:
		if e.Agent == "" {
			t.reply = e.Text
		}
		return thread.Event{}, false
	case chatlog.KindTurnEnd:
		reply := t.reply
		t.reset()
		if e.Error {
			ev.Name = thread.TurnError
			break
		}
		ev.Name, ev.Detail, ev.Asks = "Stop", oneLine(reply, 120), asks(reply)
	case chatlog.KindExit:
		t.reset()
		ev.Name = "SessionEnd"
	default:
		return thread.Event{}, false
	}
	return ev, true
}

// asks says whether a reply ends in a question to the owner.
func asks(reply string) bool { return strings.HasSuffix(strings.TrimRight(reply, " \t\n*_"), "?") }

// commandOf is the shell command a tool call's input holds, on one line, "" for a tool that runs none.
func commandOf(input json.RawMessage) string {
	var in struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(input, &in)
	return oneLine(in.Command, 200)
}

// toolDetail says what a tool call does, by the first of its input's fields that tells.
func toolDetail(tool string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	for _, key := range []string{"command", "file_path", "pattern", "url", "description"} {
		if v, ok := in[key].(string); ok && v != "" {
			return tool + " " + v
		}
	}
	return tool
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}
