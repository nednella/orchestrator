package host

import "encoding/json"

// The socket takes one JSON request line and answers one JSON reply line. A subscribe reply is followed by one chat.Event per line until the client hangs up.

const (
	opSend      = "send"
	opSendNow   = "send_now"
	opDraft     = "draft"
	opInterrupt = "interrupt"
	opAnswer    = "answer"
	opMode      = "mode"
	opHold      = "hold"
	opStop      = "stop"
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
	Mode    string          `json:"mode,omitempty"`
	Reason  string          `json:"reason,omitempty"` // the stop op's: chat.StopSettle or chat.StopRestart
	Access  string          `json:"access,omitempty"` // the stop op's, for a restart: the access the thread starts again with
}

type reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	ID    string `json:"id,omitempty"`
	Busy  bool   `json:"busy,omitempty"` // the hold op's answer: the host was busy, so it is not held
}

const maxRequest = 1 << 20

// MaxSocketPath is the longest path a unix socket can bind on macOS: sun_path holds 104 bytes with the closing NUL.
const MaxSocketPath = 103
