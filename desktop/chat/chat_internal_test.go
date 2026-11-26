package chat

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/sessions"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/session"
)

func TestToolSummaryMatchesPending(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, tool, input, want string
	}{
		{"a command", "Bash", `{"command":"go test ./...\ngo vet ./...","description":"run the tests"}`, "go test ./..."},
		{"a file", "Read", `{"file_path":"/repo/main.go","limit":20}`, "/repo/main.go"},
		{"a URL", "WebFetch", `{"url":"https://example.com/docs","prompt":"read it"}`, "https://example.com/docs"},
		{"a question", "AskUserQuestion", `{"questions":[{"question":"Which one?"},{"question":"Why?"}]}`, "Which one?"},
		{"a plan", "ExitPlanMode", `{"plan":"\n  Add made.txt\nThen test"}`, "Add made.txt"},
		{"no field it knows", "mcp__orchestrator__threads", `{"project":"all"}`, ""},
		{"input that is not JSON", "Bash", `not json`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := json.RawMessage(tt.input)
			got := ToolSummary(tt.tool, input)
			if got != tt.want {
				t.Errorf("ToolSummary = %q, want %q", got, tt.want)
			}
			p := pendingOf("main/aaaaaaaa", chatlog.Event{Kind: chatlog.KindApproval, ID: "r1", Tool: tt.tool, Input: input})
			if p.Summary != got {
				t.Errorf("pendingOf's summary = %q, ToolSummary's %q", p.Summary, got)
			}
			if string(p.Input) != tt.input {
				t.Errorf("pendingOf's input = %s, want the approval's %s", p.Input, tt.input)
			}
		})
	}
}

func TestPendingInputIsCapped(t *testing.T) {
	t.Parallel()
	inputOf := func(size int) json.RawMessage {
		frame := `{"file_path":"a.go","content":""}`
		return json.RawMessage(strings.Replace(frame, `""`, `"`+strings.Repeat("a", size-len(frame))+`"`, 1))
	}
	tests := []struct {
		name  string
		input json.RawMessage
		kept  bool
	}{
		{"at the cap", inputOf(maxPendingInput), true},
		{"one byte over", inputOf(maxPendingInput + 1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pendingOf("main/aaaaaaaa", chatlog.Event{Kind: chatlog.KindApproval, ID: "r1", Tool: "Write", Input: tt.input})
			if tt.kept && (string(p.Input) != string(tt.input) || p.InputTooLarge) {
				t.Errorf("pending carries %d bytes of input, too large %v; want all %d, not too large", len(p.Input), p.InputTooLarge, len(tt.input))
			}
			if !tt.kept && (p.Input != nil || !p.InputTooLarge) {
				t.Errorf("pending carries %d bytes of input, too large %v; want none, too large", len(p.Input), p.InputTooLarge)
			}
			if p.Summary != "a.go" {
				t.Errorf("summary = %q, want the file either way", p.Summary)
			}
		})
	}
}

// modeThread is a live thread whose AccessChanged calls are kept.
type modeThread struct {
	chat  sessions.Chat
	modes chan string
}

func (m modeThread) ChatOf(string) (sessions.Chat, error)          { return m.chat, nil }
func (m modeThread) Chats() []sessions.Chat                        { return []sessions.Chat{m.chat} }
func (m modeThread) Observe(string, ...session.Event)              {}
func (m modeThread) TranscriptPath(string, string) string          { return "" }
func (m modeThread) BackgroundChanged(string, sessions.Background) {}
func (m modeThread) AccessChanged(_, mode string, _ bool)          { m.modes <- mode }
func (m modeThread) FlagRefused(string)                            {}
func (m modeThread) NextTurn(string)                               {}
func (m modeThread) Wake(string) (sessions.Chat, error)            { return m.chat, nil }
func (m modeThread) FilesRewind(string) (string, bool, string)     { return "", true, "" }

func TestFollowerHandsOnTheLogsModeOnlyOfAHostThatListens(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		listening bool
	}{
		{"a host that listens", true},
		{"a host that does not", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "aos")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })
			logPath, socket := filepath.Join(dir, "chat.jsonl"), filepath.Join(dir, "host.sock")
			l, err := chatlog.OpenLog(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range []chatlog.Event{{Kind: chatlog.KindStart}, {Kind: chatlog.KindMode, Text: "plan"}} {
				if _, err := l.Append(e); err != nil {
					t.Fatal(err)
				}
			}
			l.Close()
			if tt.listening {
				ln, err := net.Listen("unix", socket)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { ln.Close() })
				go func() {
					for {
						conn, err := ln.Accept()
						if err != nil {
							return
						}
						conn.Close()
					}
				}()
			}
			thread := modeThread{chat: sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: socket, Live: true}, modes: make(chan string, 4)}
			chats := New(thread, t.TempDir(), func(string, any) {})
			chats.hostWait = 300 * time.Millisecond
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			chats.Start(ctx)
			select {
			case mode := <-thread.modes:
				if !tt.listening {
					t.Errorf("the mode %q of a log whose host does not listen was handed on", mode)
				}
			case <-time.After(2 * time.Second):
				if tt.listening {
					t.Error("the mode of the log was not handed on")
				}
			}
		})
	}
}
