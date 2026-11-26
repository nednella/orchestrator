package chat

import (
	"encoding/json"
	"strings"

	"github.com/nednella/orchestrator/internal/prompts"
)

type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type imageBlock struct {
	Type   string      `json:"type"`
	Source imageSource `json:"source"`
}

type imageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`
}

type userLine struct {
	Type            string      `json:"type"`
	Message         chatMessage `json:"message"`
	ParentToolUseID *string     `json:"parent_tool_use_id"`
	SessionID       string      `json:"session_id"`
	UUID            string      `json:"uuid,omitempty"`
}

type controlRequest struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	Request   any    `json:"request"`
}

type requestBody struct {
	Subtype              string `json:"subtype"`
	Mode                 string `json:"mode,omitempty"`
	SpareBackgroundTasks bool   `json:"spare_background_tasks,omitempty"`
}

type rewindFilesBody struct {
	Subtype       string `json:"subtype"`
	UserMessageID string `json:"user_message_id"`
	DryRun        bool   `json:"dry_run"`
}

type rewindConversationBody struct {
	Subtype           string `json:"subtype"`
	TargetMessageUUID string `json:"target_message_uuid"`
	LastSeenUserUUID  string `json:"last_seen_user_message_uuid"`
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
	return UserMessageWith("", text, nil)
}

// Image is one picture of a user turn: its media type and its base64 data.
type Image struct{ Mime, Data string }

// UserMessageWith is a user turn that carries id, which claude echoes back, and images before the text.
func UserMessageWith(id, text string, images []Image) []byte {
	content := make([]any, 0, len(images)+1)
	for _, im := range images {
		content = append(content, imageBlock{Type: "image", Source: imageSource{Type: "base64", MediaType: im.Mime, Data: im.Data}})
	}
	if text != "" || len(images) == 0 {
		content = append(content, textBlock{Type: "text", Text: text})
	}
	return encodeLine(userLine{
		Type:            "user",
		Message:         chatMessage{Role: "user", Content: content},
		ParentToolUseID: nil,
		SessionID:       "default",
		UUID:            id,
	})
}

func Initialize(id string) []byte {
	return encodeLine(controlRequest{Type: "control_request", RequestID: id, Request: requestBody{Subtype: "initialize"}})
}

// Interrupt stops Claude's turn and leaves the tasks it runs in the background going, as Esc does in Claude's terminal:
// without spare_background_tasks, an interrupt in stream-json mode stops them too.
func Interrupt(id string) []byte {
	return encodeLine(controlRequest{Type: "control_request", RequestID: id, Request: requestBody{Subtype: "interrupt", SpareBackgroundTasks: true}})
}

// SetPermissionMode asks claude to change its permission mode, such as plan or default, as it runs.
func SetPermissionMode(id, mode string) []byte {
	return encodeLine(controlRequest{Type: "control_request", RequestID: id, Request: requestBody{Subtype: "set_permission_mode", Mode: mode}})
}

// GetUsage asks claude for the plan's usage and limits, which it reads from the account with no turn.
func GetUsage(id string) []byte {
	return encodeLine(controlRequest{Type: "control_request", RequestID: id, Request: requestBody{Subtype: "get_usage"}})
}

// RewindFiles asks claude to put the files that its turns edited back as they were before the user message: with
// dryRun it only says what that would change.
func RewindFiles(id, message string, dryRun bool) []byte {
	return encodeLine(controlRequest{Type: "control_request", RequestID: id, Request: rewindFilesBody{Subtype: "rewind_files", UserMessageID: message, DryRun: dryRun}})
}

// RewindConversation asks claude to take its conversation back to before the user message target. latest is the
// newest user message the asker knows of: claude refuses the rewind when it holds a later one.
func RewindConversation(id, target, latest string) []byte {
	return encodeLine(controlRequest{Type: "control_request", RequestID: id, Request: rewindConversationBody{Subtype: "rewind_conversation", TargetMessageUUID: target, LastSeenUserUUID: latest}})
}

// Allow answers a can_use_tool request. Claude needs an object for updatedInput, so an absent input becomes {}.
func Allow(id string, input json.RawMessage) []byte {
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	return permissionAnswer(id, allowBody{Behavior: "allow", UpdatedInput: input})
}

// Deny answers a can_use_tool request with a no. Claude turns message into an error tool result, and the API refuses one
// that is empty, so a deny with no reason gets a default one.
func Deny(id, message string) []byte {
	if strings.TrimSpace(message) == "" {
		message = prompts.DeniedWithoutReason()
	}
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
