package host

// The socket takes one JSON request line and answers one JSON reply line. A subscribe reply is followed by one chat.Event per line until the client hangs up.

const (
	opSend      = "send"
	opDraft     = "draft"
	opInterrupt = "interrupt"
	opAnswer    = "answer"
	opSubscribe = "subscribe"
)

type request struct {
	Op      string `json:"op"`
	Text    string `json:"text,omitempty"`
	ID      string `json:"id,omitempty"`
	Allow   bool   `json:"allow,omitempty"`
	Message string `json:"message,omitempty"`
	After   int64  `json:"after,omitempty"`
}

type reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

const maxRequest = 1 << 20

// maxSocketPath is the longest path a unix socket can bind on macOS: sun_path holds 104 bytes with the closing NUL.
const maxSocketPath = 103
