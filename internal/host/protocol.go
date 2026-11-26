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
	opRelease   = "release"
	opStop      = "stop"
	opSubscribe = "subscribe"
	// opRewindPreview says what a rewind of the files to before a user message would change; opRewind rewinds.
	opRewindPreview = "rewind_preview"
	opRewind        = "rewind"
	// opUsage asks Claude for the plan's usage and limits.
	opUsage = "usage"
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
	From    string          `json:"from,omitempty"`   // the send and interrupt ops': the thread that sends or stops, "" for the owner or the app
	Body    string          `json:"body,omitempty"`   // the send op's: the bare words that Text frames for Claude
	Result  bool            `json:"result,omitempty"` // the send op's: the message is a child's result
	// Within is the rewind op's: the folder that holds every file the rewind puts back, "" to leave the files as they are;
	// the rewind_preview op's: the folder outside which a file is a reason not to rewind the files.
	Within string `json:"within,omitempty"`
}

type reply struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	ID    string `json:"id,omitempty"`
	Busy  bool   `json:"busy,omitempty"` // the hold op's answer: the host was busy, so it is not held
	// Unsupported says that the Claude Code the host runs does not know a control request the op needs.
	Unsupported bool            `json:"unsupported,omitempty"`
	Changes     *FileChanges    `json:"changes,omitempty"` // the rewind_preview op's answer
	Prefill     string          `json:"prefill,omitempty"` // the rewind op's answer: the text of the message rewound to
	Usage       json.RawMessage `json:"usage,omitempty"`   // the usage op's answer: Claude's answer to get_usage
	// Approval is the send op's refusal of a message from a thread while an approval waits, which the message would deny.
	Approval bool `json:"approval,omitempty"`
	// Restarting is a refusal while the host is held for a restart, as for a change of access.
	Restarting bool `json:"restarting,omitempty"`
}

// FileChanges is what a rewind of the files would change: the files, by absolute path, and the lines it would add and
// take out. Reason says why Claude cannot rewind them, "" when it can.
type FileChanges struct {
	Files      []string `json:"files"`
	Insertions int      `json:"insertions"`
	Deletions  int      `json:"deletions"`
	Reason     string   `json:"reason,omitempty"`
}

const maxRequest = 1 << 20

// MaxSocketPath is the longest path a unix socket can bind on macOS: sun_path holds 104 bytes with the closing NUL.
const MaxSocketPath = 103
