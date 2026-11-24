package chat

import (
	"encoding/json"
	"fmt"
	"strings"
)

type line struct {
	Type         string          `json:"type"`
	Subtype      string          `json:"subtype"`
	SessionID    string          `json:"session_id"`
	UUID         string          `json:"uuid"`
	APIMessageID string          `json:"api_message_id"`
	Event        json.RawMessage `json:"event"`
	Message      json.RawMessage `json:"message"`
	RequestID    string          `json:"request_id"`
	Request      json.RawMessage `json:"request"`
	IsError      bool            `json:"is_error"`
	TotalCost    float64         `json:"total_cost_usd"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
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
		if l.Subtype == "init" {
			return []Event{{Kind: KindInit, ID: l.SessionID}}, nil
		}
	case "stream_event":
		return parseStreamEvent(l)
	case "assistant":
		return parseAssistant(l)
	case "user":
		return parseUser(l)
	case "control_request":
		return parseControlRequest(l)
	case "result":
		return []Event{{Kind: KindTurnEnd, Cost: l.TotalCost, Error: l.IsError}}, nil
	}
	return nil, nil
}

func parseStreamEvent(l line) ([]Event, error) {
	var ev struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	}
	if err := json.Unmarshal(l.Event, &ev); err != nil {
		return nil, fmt.Errorf("parse stream event: %w", err)
	}
	if ev.Type != "content_block_delta" || ev.Delta.Type != "text_delta" {
		return nil, nil
	}
	return []Event{{Kind: KindDelta, ID: l.APIMessageID, Text: ev.Delta.Text}}, nil
}

func blocks(raw json.RawMessage) (id string, content []block, err error) {
	var m struct {
		ID      string  `json:"id"`
		Content []block `json:"content"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", nil, fmt.Errorf("parse message: %w", err)
	}
	return m.ID, m.Content, nil
}

func parseAssistant(l line) ([]Event, error) {
	id, content, err := blocks(l.Message)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, b := range content {
		switch b.Type {
		case "text":
			events = append(events, Event{Kind: KindMessage, ID: id, UUID: l.UUID, Text: b.Text})
		case "tool_use":
			events = append(events, Event{Kind: KindToolUse, ID: b.ID, UUID: l.UUID, Tool: b.Name, Input: b.Input})
		}
	}
	return events, nil
}

func parseUser(l line) ([]Event, error) {
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
	var req struct {
		Subtype  string          `json:"subtype"`
		ToolName string          `json:"tool_name"`
		Input    json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(l.Request, &req); err != nil {
		return nil, fmt.Errorf("parse control request: %w", err)
	}
	if req.Subtype != "can_use_tool" {
		return nil, nil
	}
	return []Event{{Kind: KindApproval, ID: l.RequestID, Tool: req.ToolName, Input: req.Input}}, nil
}
