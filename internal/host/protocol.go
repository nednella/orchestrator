package host

import "encoding/json"

// The socket takes one JSON request line and answers one JSON reply line. A subscribe reply is followed by one chat.Event per line until the client hangs up.

const (
	opSend      = "send"
	opSendNow   = "send_now"
	opDraft     = "draft"
	opInterrupt = "interrupt"
	opAnswer    = "answer"
	opSubscribe = "subscribe"
)

type request struct {
	Op      string          `json:"op"`
	Text    string          `json:"text,omitempty"`
	Files   []string        `json:"files,omitempty"`
	ID      string          `json:"id,omitempty"`
	Allow   bool            `json:"allow,omitempty"`
	Message string          `json:"message,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	After   int64           `json:"after,omitempty"`
}

type reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	ID    string `json:"id,omitempty"`
}

const maxRequest = 1 << 20

// MaxSocketPath is the longest path a unix socket can bind on macOS: sun_path holds 104 bytes with the closing NUL.
const MaxSocketPath = 103
