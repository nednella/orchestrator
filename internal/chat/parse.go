package chat

import (
	"encoding/json"
	"fmt"
	"strings"
)

type line struct {
	Type           string          `json:"type"`
	Subtype        string          `json:"subtype"`
	SessionID      string          `json:"session_id"`
	UUID           string          `json:"uuid"`
	APIMessageID   string          `json:"api_message_id"`
	Event          json.RawMessage `json:"event"`
	Message        json.RawMessage `json:"message"`
	RequestID      string          `json:"request_id"`
	Request        json.RawMessage `json:"request"`
	IsError        bool            `json:"is_error"`
	TotalCost      float64         `json:"total_cost_usd"`
	IsReplay       bool            `json:"isReplay"`
	Response       json.RawMessage `json:"response"`
	Errors         []string        `json:"errors"`
	PermissionMode string          `json:"permissionMode"`
	// ParentToolUseID is the Agent call whose subagent wrote the line, on an assistant, user or stream event.
	ParentToolUseID string `json:"parent_tool_use_id"`
	// ToolUseID, Status and Summary are a task's end: the call that started it, completed, failed or stopped, and its
	// report.
	ToolUseID string `json:"tool_use_id"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	// TaskID, TaskType, Description, IsBackgrounded and ParentTaskID are a task's start: Claude's id for it, its kind,
	// what it does, whether it runs beside the turn, and the id of the subagent's task that started it.
	TaskID         string `json:"task_id"`
	TaskType       string `json:"task_type"`
	Description    string `json:"description"`
	IsBackgrounded bool   `json:"is_backgrounded"`
	ParentTaskID   string `json:"parent_task_id"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// Parse turns one line of claude's stream-json stdout into events. A line of a type the chat does not show gives none.
func Parse(data []byte) ([]Event, error) {
	var l line
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parse claude line: %w", err)
	}
	switch l.Type {
	case "system":
		switch l.Subtype {
		case "init":
			return []Event{{Kind: KindInit, ID: l.SessionID}}, nil
		case "compact_boundary":
			return []Event{{Kind: KindCompact}}, nil
		case "status":
			if l.PermissionMode != "" {
				return []Event{{Kind: KindMode, Text: l.PermissionMode}}, nil
			}
		case "task_started":
			return parseTaskStart(l), nil
		case "task_notification":
			return parseTaskEnd(l), nil
		}
	case "stream_event", "assistant", "user":
		events, err := parseTurn(l)
		for i := range events {
			events[i].Agent = l.ParentToolUseID
		}
		return events, err
	case "control_response":
		return parseControlResponse(l)
	case "control_request":
		return parseControlRequest(l)
	case "result":
		return []Event{{Kind: KindTurnEnd, Cost: l.TotalCost, Error: l.IsError, Text: strings.Join(l.Errors, "\n")}}, nil
	}
	return nil, nil
}

// parseTurn reads a line of the conversation: a streamed piece of a message, a whole message, or the user's side.
func parseTurn(l line) ([]Event, error) {
	switch l.Type {
	case "stream_event":
		return parseStreamEvent(l)
	case "assistant":
		return parseAssistant(l)
	}
	return parseUser(l)
}

// parseTaskStart reads the start of a task Claude runs in the background. A task Claude waits on in its turn, as a
// subagent it does not run in the background, ends within the turn and gives none.
func parseTaskStart(l line) []Event {
	if l.ToolUseID == "" || !l.IsBackgrounded {
		return nil
	}
	return []Event{{Kind: KindTaskStart, ID: l.TaskID, Parent: l.ToolUseID, Tool: l.TaskType, Text: l.Description, Agent: l.ParentTaskID}}
}

// parseTaskEnd reads the end of a task Claude runs beside its turn. Only a task a tool call started can be shown at
// its call.
func parseTaskEnd(l line) []Event {
	if l.ToolUseID == "" {
		return nil
	}
	e := Event{Kind: KindTaskEnd, ID: l.TaskID, Parent: l.ToolUseID, Text: l.Summary, Error: l.Status != "completed"}
	if l.Status == TaskStopped {
		e.Stop = TaskStopped
	}
	return []Event{e}
}

func parseStreamEvent(l line) ([]Event, error) {
	if len(l.Event) == 0 {
		return nil, nil
	}
	var ev struct {
		Type         string `json:"type"`
		ContentBlock struct {
			Type string `json:"type"`
		} `json:"content_block"`
		Delta struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"delta"`
	}
	if err := json.Unmarshal(l.Event, &ev); err != nil {
		return nil, fmt.Errorf("parse stream event: %w", err)
	}
	switch {
	case ev.Type == "content_block_start" && ev.ContentBlock.Type == "thinking":
		return []Event{{Kind: KindThinking, ID: l.APIMessageID}}, nil
	case ev.Type != "content_block_delta":
		return nil, nil
	case ev.Delta.Type == "text_delta":
		return []Event{{Kind: KindDelta, ID: l.APIMessageID, Text: ev.Delta.Text}}, nil
	// A thinking block Claude does not show streams deltas with no text, which say nothing.
	case ev.Delta.Type == "thinking_delta" && ev.Delta.Thinking != "":
		return []Event{{Kind: KindThinking, ID: l.APIMessageID, Text: ev.Delta.Thinking}}, nil
	}
	return nil, nil
}

func isArray(raw json.RawMessage) bool {
	return len(raw) > 0 && raw[0] == '['
}

func isObject(raw json.RawMessage) bool {
	return len(raw) > 0 && raw[0] == '{'
}

// blocks reads a message's content blocks; content that is a plain string has none.
func blocks(raw json.RawMessage) (id string, content []block, err error) {
	var m struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", nil, fmt.Errorf("parse message: %w", err)
	}
	if !isArray(m.Content) {
		return m.ID, nil, nil
	}
	if err := json.Unmarshal(m.Content, &content); err != nil {
		return "", nil, fmt.Errorf("parse message content: %w", err)
	}
	return m.ID, content, nil
}

func parseAssistant(l line) ([]Event, error) {
	id, content, err := blocks(l.Message)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, b := range content {
		switch b.Type {
		case "thinking":
			events = append(events, Event{Kind: KindThought, ID: id, Text: b.Thinking})
		case "text":
			events = append(events, Event{Kind: KindMessage, ID: id, UUID: l.UUID, Text: b.Text})
		case "tool_use":
			events = append(events, Event{Kind: KindToolUse, ID: b.ID, UUID: l.UUID, Tool: b.Name, Input: b.Input})
		}
	}
	return events, nil
}

func parseUser(l line) ([]Event, error) {
	if l.IsReplay {
		return []Event{{Kind: KindEcho, ID: l.UUID}}, nil
	}
	_, content, err := blocks(l.Message)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, b := range content {
		if b.Type == "tool_result" {
			events = append(events, Event{Kind: KindToolResult, Parent: b.ToolUseID, Text: resultText(b.Content), Error: b.IsError})
		}
	}
	return events, nil
}

// resultText reads a tool result's content, which is a string or a list of text blocks.
func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []block
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var texts []string
	for _, p := range parts {
		if p.Type == "text" {
			texts = append(texts, p.Text)
		}
	}
	return strings.Join(texts, "\n")
}

func parseControlRequest(l line) ([]Event, error) {
	if len(l.Request) == 0 {
		return nil, nil
	}
	var req struct {
		Subtype   string          `json:"subtype"`
		ToolName  string          `json:"tool_name"`
		Input     json.RawMessage `json:"input"`
		ToolUseID string          `json:"tool_use_id"`
	}
	if err := json.Unmarshal(l.Request, &req); err != nil {
		return nil, fmt.Errorf("parse control request: %w", err)
	}
	if req.Subtype != "can_use_tool" {
		return nil, nil
	}
	return []Event{{Kind: KindApproval, ID: l.RequestID, Parent: req.ToolUseID, Tool: req.ToolName, Input: req.Input}}, nil
}

// parseControlResponse reads the reply to initialize, which lists the slash commands, the reply to an interrupt, which
// lists the messages claude still holds, and any other reply as a KindReply.
func parseControlResponse(l line) ([]Event, error) {
	if len(l.Response) == 0 {
		return nil, nil
	}
	var r struct {
		Subtype   string          `json:"subtype"`
		RequestID string          `json:"request_id"`
		Error     string          `json:"error"`
		Response  json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(l.Response, &r); err != nil {
		return nil, fmt.Errorf("parse control response: %w", err)
	}
	var lists struct {
		Commands    json.RawMessage `json:"commands"`
		StillQueued json.RawMessage `json:"still_queued"`
	}
	if len(r.Response) > 0 {
		if err := json.Unmarshal(r.Response, &lists); err != nil {
			return nil, fmt.Errorf("parse control response: %w", err)
		}
	}
	switch {
	case isArray(lists.Commands):
		return []Event{{Kind: KindCommands, Input: lists.Commands}}, nil
	case isArray(lists.StillQueued):
		return []Event{{Kind: KindStillQueued, ID: r.RequestID, Input: lists.StillQueued}}, nil
	}
	reply := Event{Kind: KindReply, ID: r.RequestID, Error: r.Subtype == "error", Text: r.Error}
	if isObject(r.Response) {
		reply.Input = r.Response
	}
	return []Event{reply}, nil
}
