package chat

import "encoding/json"

type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type chatMessage struct {
	Role    string      `json:"role"`
	Content []textBlock `json:"content"`
}

type userLine struct {
	Type            string      `json:"type"`
	Message         chatMessage `json:"message"`
	ParentToolUseID *string     `json:"parent_tool_use_id"`
	SessionID       string      `json:"session_id"`
}

type controlRequest struct {
	Type      string      `json:"type"`
	RequestID string      `json:"request_id"`
	Request   requestBody `json:"request"`
}

type requestBody struct {
	Subtype string `json:"subtype"`
}

type controlResponse struct {
	Type     string `json:"type"`
	Response reply  `json:"response"`
}

type reply struct {
	Subtype   string `json:"subtype"`
	RequestID string `json:"request_id"`
	Response  any    `json:"response"`
}

type allowBody struct {
	Behavior     string          `json:"behavior"`
	UpdatedInput json.RawMessage `json:"updatedInput"`
}

type denyBody struct {
	Behavior string `json:"behavior"`
	Message  string `json:"message"`
}

// UserMessage is one user turn for claude's stdin.
func UserMessage(text string) []byte {
	return encodeLine(userLine{
		Type:            "user",
		Message:         chatMessage{Role: "user", Content: []textBlock{{Type: "text", Text: text}}},
		ParentToolUseID: nil,
		SessionID:       "default",
	})
}

func Initialize(id string) []byte {
	return encodeLine(controlRequest{Type: "control_request", RequestID: id, Request: requestBody{Subtype: "initialize"}})
}

func Interrupt(id string) []byte {
	return encodeLine(controlRequest{Type: "control_request", RequestID: id, Request: requestBody{Subtype: "interrupt"}})
}

// Allow answers a can_use_tool request. Claude needs an object for updatedInput, so an absent input becomes {}.
func Allow(id string, input json.RawMessage) []byte {
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	return permissionAnswer(id, allowBody{Behavior: "allow", UpdatedInput: input})
}

func Deny(id, message string) []byte {
	return permissionAnswer(id, denyBody{Behavior: "deny", Message: message})
}

func permissionAnswer(id string, body any) []byte {
	return encodeLine(controlResponse{Type: "control_response", Response: reply{Subtype: "success", RequestID: id, Response: body}})
}

// encodeLine panics only when a RawMessage is not valid JSON, which a control_request input never is.
func encodeLine(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}
