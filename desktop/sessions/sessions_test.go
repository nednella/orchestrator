package sessions_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/session"
)

func TestSessionLifecycle(t *testing.T) {
	h := newHarness(t)

	first, err := h.NewSession("first", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.NewSession("second", "")
	if err != nil {
		t.Fatal(err)
	}
	snap := h.Snapshot()
	if len(snap.Sessions) != 2 || snap.Project.Dir != h.Dir || snap.Sessions[0].History == nil {
		t.Fatalf("snapshot = %+v", snap)
	}
	if first.N != 1 || second.N != 2 || first.State != "idle" || first.CreatedAt == 0 {
		t.Errorf("sessions = %+v %+v", first, second)
	}

	t.Run("hooks", func(t *testing.T) {
		before := h.Rec.Count("attention")
		h.Hook(t, second.ID, "Notification", `{"message":"Claude needs your permission"}`)
		eventually(t, "waiting", func() bool {
			s := h.Rec.LastSessions()
			return len(s) == 2 && s[0].ID == second.ID && s[0].State == "waiting"
		})
		s := h.Rec.LastSessions()
		if s[0].Detail != "Claude needs your permission" || len(s[0].History) != 2 || s[0].LastEventAt == 0 {
			t.Errorf("waiting session = %+v", s[0])
		}
		eventually(t, "attention event", func() bool { return h.Rec.Count("attention") == before+1 })
		name, payload := h.Rec.LastEvent()
		if got, _ := json.Marshal(payload); name != "attention" || string(got) != fmt.Sprintf(`{"id":%q,"state":"waiting"}`, second.ID) {
			t.Errorf("last event = %s %s", name, got)
		}

		h.Hook(t, first.ID, "UserPromptSubmit", `{"prompt":"go"}`)
		eventually(t, "working", func() bool {
			s := h.Rec.LastSessions()
			return len(s) == 2 && s[1].ID == first.ID && s[1].State == "working"
		})
		repliedBefore := h.Rec.CountAttention("replied")
		h.Hook(t, first.ID, "Stop", `{}`)
		eventually(t, "idle after the reply", func() bool {
			s := h.Rec.LastSessions()
			return len(s) == 2 && s[1].ID == first.ID && s[1].State == "idle"
		})
		eventually(t, "replied attention for a turn that ended", func() bool { return h.Rec.CountAttention("replied") == repliedBefore+1 })
		h.Hook(t, second.ID, "PreToolUse", `{"tool_name":"Edit","tool_input":{"file_path":"a.tsx"}}`)
		eventually(t, "idle then working order", func() bool {
			s := h.Rec.LastSessions()
			return len(s) == 2 && s[0].ID == first.ID && s[1].State == "working" && s[1].Detail == "Edit a.tsx"
		})
	})

	t.Run("rename and kill", func(t *testing.T) {
		if err := h.RenameSession(second.ID, "renamed"); err != nil {
			t.Fatal(err)
		}
		eventually(t, "rename", func() bool {
			s := h.Rec.LastSessions()
			return len(s) == 2 && s[1].Title == "renamed"
		})
		if err := h.KillSession(first.ID); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the killed session to show as ended", func() bool {
			s := h.Rec.LastSessions()
			return len(s) == 2 && s[1].ID == first.ID && s[1].State == "ended" && s[1].EndedAt > 0
		})
		if got := h.Snapshot().Sessions; len(got) != 2 || got[0].ID != second.ID || got[1].State != "ended" || got[1].Title != "first" {
			t.Errorf("sessions after kill = %+v", got)
		}
		if out, err := exec.Command("tmux", "-L", h.Socket, "has-session", "-t", first.ID).CombinedOutput(); err == nil {
			t.Errorf("the killed tmux session is still there: %s", out)
		}
		if err := h.KillSession(first.ID); err == nil {
			t.Error("killing an ended session succeeded")
		}
		if err := h.DismissSession(second.ID); err == nil {
			t.Error("dismissed a session that is running")
		}
		if err := h.DismissSession(first.ID); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the dismissed row to go", func() bool { return len(h.Rec.LastSessions()) == 1 })
		time.Sleep(2500 * time.Millisecond) // a poll must not bring it back
		if got := h.Snapshot().Sessions; len(got) != 1 || got[0].ID != second.ID {
			t.Errorf("sessions after dismiss and a poll = %+v", got)
		}
	})

	t.Run("session ends on its own", func(t *testing.T) {
		if err := exec.Command("tmux", "-L", h.Socket, "kill-session", "-t", second.ID).Run(); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the row to turn ended", func() bool {
			s := h.Rec.LastSessions()
			return len(s) == 1 && s[0].State == "ended" && s[0].EndedAt > 0 && s[0].Title == "renamed"
		})
		if h.Snapshot().Projects[0].Sessions != 0 {
			t.Error("an ended session counts as a running one")
		}

		// A new session does not take the ended one's number.
		third, err := h.NewSession("third", "")
		if err != nil || third.ID == second.ID {
			t.Errorf("new session = %+v, %v", third, err)
		}
		if err := h.DismissSession(second.ID); err != nil {
			t.Fatal(err)
		}
	})
}

func TestOwnersPromptIsSentOnceTheAgentIsReady(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("", "printf 'ran-%s' prompt\nignored second line")
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "printf 'ran-%s' prompt" {
		t.Errorf("title = %q, want the first line of the prompt", s.Title)
	}
	eventually(t, "the prompt submitted", func() bool { return strings.Contains(h.Pane(t, s.ID), "ran-prompt") })
}

func TestThreadFromAPromptGetsItsTitleModelAndEffort(t *testing.T) {
	dir := apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	long := strings.Repeat("é", 80)

	tests := []struct {
		name, title, prompt, model, effort string
		chat                               bool
		wantTitle                          string
	}{
		{"task", "", "Fix the cart\nwith detail", "opus", "high", false, "Fix the cart"},
		{"chat", "", "What is this?", "haiku", "", true, "What is this?"},
		{"long first line", "", long, "", "low", false, strings.Repeat("é", 60)},
		{"given title", "mine", "do it", "", "", false, "mine"},
		{"empty chat", "", "", "", "", true, "chat"},
	}
	for _, tt := range tests {
		var s sessions.Session
		var err error
		if tt.chat {
			s, err = h.NewChatWith(tt.title, tt.prompt, tt.model, tt.effort)
		} else {
			s, err = h.NewSessionWith(tt.title, tt.prompt, tt.model, tt.effort)
		}
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if s.Title != tt.wantTitle {
			t.Errorf("%s: title = %q, want %q", tt.name, s.Title, tt.wantTitle)
		}
		args := claudeArgs(t, dir, s.ID)
		flag := func(name string) string {
			if i := slices.Index(args, name); i >= 0 {
				return args[i+1]
			}
			return ""
		}
		if flag("--model") != tt.model || flag("--effort") != tt.effort {
			t.Errorf("%s: claude got --model %q --effort %q, want %q %q", tt.name, flag("--model"), flag("--effort"), tt.model, tt.effort)
		}
		if tt.prompt != "" {
			text, _ := json.Marshal(tt.prompt)
			eventually(t, tt.name+": the prompt on claude's stdin", func() bool {
				users := userLines(t, dir, s.ID)
				return len(users) == 1 && strings.Contains(users[0], `"text":`+string(text))
			})
		}
	}
}

func TestThreadWithAnUnknownModelDoesNotStart(t *testing.T) {
	apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	if s, err := h.NewSessionWith("", "hi", "nope", ""); err == nil {
		t.Errorf("started %+v with a model claude does not take", s)
	}
	if s, err := h.NewChatWith("", "hi", "", "huge"); err == nil {
		t.Errorf("started %+v with an effort claude does not take", s)
	}
}

// hostFolder is the folder of the hosts' sockets of the project with that key.
func hostFolder(h *apptest.Harness, key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(h.State, "hosts", hex.EncodeToString(sum[:4]))
}

// hostFiles are the chat log and the socket of a session's host.
func hostFiles(h *apptest.Harness, id string) (chatLog, socket string) {
	name, _ := session.ParseName(id)
	return filepath.Join(h.State, "data", name.Project, "chats", name.Token+".jsonl"), filepath.Join(hostFolder(h, name.Project), name.Token+".sock")
}

// userLines are the user messages the stand-in claude read on its stdin.
func userLines(t *testing.T, dir, id string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, strings.ReplaceAll(id, "/", "_")+".stdin"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	var users []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.HasPrefix(line, `{"type":"user"`) {
			users = append(users, line)
		}
	}
	return users
}

func TestClaudeRunsUnderTheHost(t *testing.T) {
	dir := apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	chatLog, socket := hostFiles(h, s.ID)
	if len(socket) > 103 {
		t.Fatalf("socket path %s is %d bytes, over the 103 a unix socket takes", socket, len(socket))
	}
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", s.ID, "#{pane_start_command}").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := apptest.CLIPath(t) + " host --log " + chatLog + " --socket " + socket + " -- claude "
	if got := strings.TrimSpace(string(out)); !strings.HasPrefix(got, want) {
		t.Errorf("pane runs %s\nwant it to start with %s", got, want)
	}
	args := claudeArgs(t, dir, s.ID)
	joined := strings.Join(args, " ")
	for _, flag := range []string{"-p", "--input-format stream-json", "--output-format stream-json", "--verbose", "--include-partial-messages", "--permission-prompt-tool stdio"} {
		if !strings.Contains(" "+joined+" ", " "+flag+" ") {
			t.Errorf("claude lacks %s: %q", flag, args)
		}
	}
	if slices.Contains(args, "--") {
		t.Errorf("claude got a start prompt: %q", args)
	}
	eventually(t, "the host's socket", func() bool { return exists(socket) })
	info, err := os.Stat(filepath.Dir(socket))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("socket folder = %v, %v; want mode 700", info.Mode(), err)
	}
}

func TestTheHostIsTheAppsOwnBinary(t *testing.T) {
	dir := apptest.ClaudeOnPath(t)
	older := filepath.Join(t.TempDir(), "agentos")
	if err := os.WriteFile(older, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(older)+string(os.PathListSeparator)+os.Getenv("PATH"))
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", s.ID, "#{pane_start_command}").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(out)), apptest.CLIPath(t)+" host "; !strings.HasPrefix(got, want) {
		t.Errorf("pane runs %s\nwant it to start with %s", got, want)
	}
	if args := strings.Join(claudeArgs(t, dir, s.ID), " "); !strings.Contains(args, older+"' hook Stop") {
		t.Errorf("the hooks do not call the agentos on PATH: %q", args)
	}
}

func TestAWideSocketFolderIsMadePrivate(t *testing.T) {
	apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	folder := hostFolder(h, "main")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	_, socket := hostFiles(h, s.ID)
	eventually(t, "the host's socket", func() bool { return exists(socket) })
	if info, err := os.Stat(folder); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("socket folder = %v, %v; want mode 700", info.Mode(), err)
	}
}

func TestALongProjectKeyStartsItsHost(t *testing.T) {
	apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	dir := filepath.Join(t.TempDir(), strings.Repeat("a-long-project-name-", 5))
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := h.AddProjectDir(dir); err != nil {
		t.Fatal(err)
	}
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if name, _ := session.ParseName(s.ID); len(name.Project) < 90 {
		t.Fatalf("project key %q is not long", name.Project)
	}
	_, socket := hostFiles(h, s.ID)
	eventually(t, "the host's socket", func() bool { return exists(socket) })
}

func TestSentPromptReachesClaudeAsAMessage(t *testing.T) {
	dir := apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	s, err := h.Sessions().Create("#7 work", "/work 7; and a ';' at the end;", true, 7, "")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the prompt on claude's stdin", func() bool { return len(userLines(t, dir, s.ID)) > 0 })
	time.Sleep(300 * time.Millisecond)
	users := userLines(t, dir, s.ID)
	if len(users) != 1 || !strings.Contains(users[0], `"text":"/work 7; and a ';' at the end;"`) {
		t.Errorf("user lines = %q, want one with the prompt", users)
	}
	chatLog, _ := hostFiles(h, s.ID)
	eventually(t, "the prompt and the reply in the chat log", func() bool {
		data, _ := os.ReadFile(chatLog)
		return loggedUser(chatLog, "/work 7; and a ';' at the end;") && strings.Contains(string(data), `"kind":"turn_end"`)
	})
}

// loggedUser reports whether the chat log at path holds the user message text.
func loggedUser(path, text string) bool {
	events, _ := chat.ReadLog(path, 0)
	return slices.ContainsFunc(events, func(e chat.Event) bool { return e.Kind == chat.KindUser && e.Text == text })
}

func TestAppPromptKeepsItsMark(t *testing.T) {
	dir := apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	if _, err := h.Sessions().CreateBriefed("set up", "write the queue", "brief"); err != nil {
		t.Fatal(err)
	}
	id := h.Sessions().List()[0].ID
	eventually(t, "the prompt on claude's stdin", func() bool { return len(userLines(t, dir, id)) == 1 })
	if users := userLines(t, dir, id); !strings.Contains(users[0], `"text":"APPLICATION_PROMPT: write the queue"`) {
		t.Errorf("user line = %s", users[0])
	}
}

func TestManualPromptIsSentToAHostedSession(t *testing.T) {
	dir := apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude", ProjectExtra: "    session_prompt_send: manual\n"})
	if _, err := h.Sessions().CreateBriefed("set up", "write the queue", "brief"); err != nil {
		t.Fatal(err)
	}
	id := h.Sessions().List()[0].ID
	eventually(t, "the prompt on claude's stdin", func() bool { return len(userLines(t, dir, id)) == 1 })
	if users := userLines(t, dir, id); !strings.Contains(users[0], `"text":"APPLICATION_PROMPT: write the queue"`) {
		t.Errorf("user line = %s", users[0])
	}
}

func TestDismissRemovesTheChatLog(t *testing.T) {
	apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	s, err := h.Sessions().Create("work", "hello", true, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	chatLog, socket := hostFiles(h, s.ID)
	eventually(t, "the chat log", func() bool {
		data, _ := os.ReadFile(chatLog)
		return strings.Contains(string(data), `"kind":"user"`)
	})
	if err := h.KillSession(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the killed session to show as ended", func() bool { got, _ := h.Session(s.ID); return got.State == "ended" })
	if got, _ := h.Session(s.ID); got.Detail != "" {
		t.Errorf("ended row = %+v, want no failure shown", got)
	}
	// A host that dies without its shutdown leaves its socket behind.
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !exists(chatLog) {
		t.Fatal("the chat log is gone before the dismiss")
	}
	if err := h.DismissSession(s.ID); err != nil {
		t.Fatal(err)
	}
	if exists(chatLog) || exists(socket) {
		t.Errorf("after dismiss, chat log there: %v, socket there: %v", exists(chatLog), exists(socket))
	}
}

func TestInterruptReachesTheHost(t *testing.T) {
	dir := apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	_, socket := hostFiles(h, s.ID)
	eventually(t, "the host's socket", func() bool { return exists(socket) })
	if err := h.Interrupt(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the interrupt on claude's stdin", func() bool {
		data, _ := os.ReadFile(filepath.Join(dir, strings.ReplaceAll(s.ID, "/", "_")+".stdin"))
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, `"type":"control_request"`) && strings.Contains(line, `"subtype":"interrupt"`) {
				return true
			}
		}
		return false
	})
}

func TestAnInterruptedTurnEndsWhenTheHostSaysSo(t *testing.T) {
	for _, tt := range []struct {
		name, state string
		hooks       [][2]string
	}{
		{"working", "working", nil},
		{"waiting", "waiting", [][2]string{{"PreToolUse", `{"tool_name":"Bash"}`}, {"Notification", `{"message":"needs permission","notification_type":"permission_prompt"}`}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := apptest.ClaudeOnPath(t)
			h := apptest.NewWith(t, apptest.Options{Agent: "claude", ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n"})
			s, err := h.Sessions().Create("#12 work", "", false, 12, "")
			if err != nil {
				t.Fatal(err)
			}
			transcript := filepath.Join(t.TempDir(), "conv.jsonl")
			h.Hook(t, s.ID, "UserPromptSubmit", fmt.Sprintf(`{"prompt":"go","transcript_path":%q}`, transcript))
			appendLines(t, transcript, assistantLine("msg_1", time.Now().Add(100*time.Millisecond), 1000, 100))
			for _, hook := range tt.hooks {
				h.Hook(t, s.ID, hook[0], hook[1])
			}
			eventually(t, tt.state, func() bool { return stateOf(h, s.ID) == tt.state })
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
			h.RefreshPRs()
			h.GH.SetPR(prJSON("OPEN", false, "[]", 1, 0))
			h.RefreshPRs()
			time.Sleep(300 * time.Millisecond)
			if users := userLines(t, dir, s.ID); len(users) > 0 {
				t.Fatalf("a prompt was sent in the middle of the turn: %q", users)
			}

			if err := h.Interrupt(s.ID); err != nil {
				t.Fatal(err)
			}
			eventually(t, "idle after the interrupt", func() bool { return stateOf(h, s.ID) == "idle" })
			if rec, err := bus.ReadState(h.State, s.ID); err != nil || rec.Event != session.TurnError {
				t.Errorf("state after the interrupt = %+v, %v; want a %s", rec, err, session.TurnError)
			}
			eventually(t, "the held prompt once the turn ends", func() bool {
				users := userLines(t, dir, s.ID)
				return len(users) == 1 && strings.Contains(users[0], "/address-review 12")
			})
			eventually(t, "the turn's usage in the event log", func() bool {
				events := h.Events(t, "usage")
				return len(events) == 1 && events[0].Tokens.Input == 1000 && events[0].Tokens.Output == 100
			})
		})
	}
}

func TestOnlyATurnEndWithNoErrorAlertsAReply(t *testing.T) {
	for _, tt := range []struct {
		name    string
		end     func(h *apptest.Harness, id string) error
		replied int
	}{
		{"a reply", func(h *apptest.Harness, id string) error { return h.TypeInto(id, "next") }, 1},
		{"an interrupt", func(h *apptest.Harness, id string) error { return h.Interrupt(id) }, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			apptest.ClaudeOnPath(t)
			h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
			s, err := h.Sessions().Create("work", "", false, 0, "")
			if err != nil {
				t.Fatal(err)
			}
			_, socket := hostFiles(h, s.ID)
			eventually(t, "the host's socket", func() bool { return exists(socket) })
			h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
			eventually(t, "working", func() bool { return stateOf(h, s.ID) == "working" })
			if err := tt.end(h, s.ID); err != nil {
				t.Fatal(err)
			}
			eventually(t, "idle after the host's turn_end", func() bool { return stateOf(h, s.ID) == "idle" })
			time.Sleep(200 * time.Millisecond)
			if n := h.Rec.CountAttentionFor(s.ID, "replied"); n != tt.replied {
				t.Errorf("replied %d times, want %d", n, tt.replied)
			}
		})
	}
}

func TestATurnTheStopHookEndedEndsOnce(t *testing.T) {
	for _, tt := range []struct {
		name, stop, state string
	}{
		{"a reply", `{"last_assistant_message":"Done."}`, "idle"},
		{"a question", `{"last_assistant_message":"Which branch should I use?"}`, "waiting"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			apptest.ClaudeOnPath(t)
			h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
			s, err := h.Sessions().Create("work", "", false, 0, "")
			if err != nil {
				t.Fatal(err)
			}
			transcript := filepath.Join(t.TempDir(), "conv.jsonl")
			h.Hook(t, s.ID, "UserPromptSubmit", fmt.Sprintf(`{"prompt":"go","transcript_path":%q}`, transcript))
			appendLines(t, transcript, assistantLine("msg_1", time.Now().Add(100*time.Millisecond), 1000, 100))
			h.Hook(t, s.ID, "Stop", tt.stop)
			stopped, err := bus.ReadState(h.State, s.ID)
			if err != nil || stopped.Event != "Stop" {
				t.Fatalf("state after Stop = %+v, %v", stopped, err)
			}

			if err := h.TypeInto(s.ID, "next"); err != nil {
				t.Fatal(err)
			}
			chatLog, _ := hostFiles(h, s.ID)
			eventually(t, "the host's turn_end after the Stop", func() bool {
				data, _ := os.ReadFile(chatLog)
				return strings.Contains(string(data), `"kind":"turn_end"`)
			})
			time.Sleep(1500 * time.Millisecond) // past the wait for the turn's last reply
			if rec, err := bus.ReadState(h.State, s.ID); err != nil || !rec.At.Equal(stopped.At) {
				t.Errorf("state after the host's turn_end = %+v, %v; want the Stop's, at %v", rec, err, stopped.At)
			}
			if got := stateOf(h, s.ID); got != tt.state {
				t.Errorf("state = %s, want %s", got, tt.state)
			}
			if events := h.Events(t, "usage"); len(events) != 1 || events[0].Tokens.Input != 1000 {
				t.Errorf("usage events = %+v, want the turn's once", events)
			}
		})
	}
}

func TestARestartOverManyTurnsLeavesTheRunningOneAlone(t *testing.T) {
	apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(t.TempDir(), "conv.jsonl")
	chatLog, _ := hostFiles(h, s.ID)
	const turns = 10
	for i := range turns {
		h.Hook(t, s.ID, "UserPromptSubmit", fmt.Sprintf(`{"prompt":"turn %d","transcript_path":%q}`, i, transcript))
		appendLines(t, transcript, assistantLine(fmt.Sprintf("msg_%d", i), time.Now().Add(100*time.Millisecond), 100, 10))
		h.Hook(t, s.ID, "Stop", "{}")
		if err := h.TypeInto(s.ID, "next"); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the turn's end in the chat log", func() bool {
			data, _ := os.ReadFile(chatLog)
			return strings.Count(string(data), `"kind":"turn_end"`) == i+1
		})
	}
	h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"last"}`)
	eventually(t, "working", func() bool { return stateOf(h, s.ID) == "working" })
	time.Sleep(1500 * time.Millisecond) // past the wait for the last Stop's usage
	usage := len(h.Events(t, "usage"))
	running, err := bus.ReadState(h.State, s.ID)
	if err != nil {
		t.Fatal(err)
	}

	second := h.Restart(t)
	time.Sleep(time.Second) // the new app has read the chat log again by now
	if got, _ := second.Session(s.ID); got.State != "working" {
		t.Errorf("state after a restart = %s, want working: an earlier turn's end ended this one", got.State)
	}
	if rec, err := bus.ReadState(h.State, s.ID); err != nil || !rec.At.Equal(running.At) {
		t.Errorf("state file after a restart = %+v, %v; want the running turn's, at %v", rec, err, running.At)
	}
	if got := len(second.Events(t, "usage")); got != usage {
		t.Errorf("%d usage events after a restart, want %d", got, usage)
	}
}

func TestAPlainSessionTakesNoInterrupt(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "Thread " + s.ID + " has no chat: its agent runs in its terminal"
	if err := h.Interrupt(s.ID); err == nil || err.Error() != want {
		t.Errorf("Interrupt = %v, want %q", err, want)
	}
}

func TestAnEndedSessionTakesNoTextOrInterrupt(t *testing.T) {
	apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.KillSession(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the killed session to show as ended", func() bool { got, _ := h.Session(s.ID); return got.State == "ended" })
	if err, want := h.Interrupt(s.ID), "Thread "+s.ID+" has ended"; err == nil || err.Error() != want {
		t.Errorf("Interrupt = %v, want %q", err, want)
	}
	want := "session " + s.ID + " has ended"
	if err := h.TypeInto(s.ID, "hello"); err == nil || err.Error() != want {
		t.Errorf("TypeInto = %v, want %q", err, want)
	}
}

func TestTypeIntoAHostedSessionSendsIt(t *testing.T) {
	dir := apptest.ClaudeOnPath(t)
	h := apptest.NewWith(t, apptest.Options{Agent: "claude"})
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.TypeInto(s.ID, "Address the review comments on PR #12."); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the text on claude's stdin", func() bool { return len(userLines(t, dir, s.ID)) == 1 })
	if users := userLines(t, dir, s.ID); !strings.Contains(users[0], `"text":"Address the review comments on PR #12."`) {
		t.Errorf("user line = %s", users[0])
	}
}

func TestSessionsSurviveWithoutALocale(t *testing.T) {
	for _, name := range []string{"LANG", "LC_ALL", "LC_CTYPE"} {
		t.Setenv(name, "")
	}
	h := newHarness(t)
	s, err := h.NewSession("naïve", "")
	if err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	list := h.Sessions().List()
	if len(list) != 1 || list[0].ID != s.ID || list[0].Title != "naïve" {
		t.Errorf("sessions after a refresh = %+v", list)
	}
}

func TestMacStaysAwakeWhileASessionWorks(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("busy", "")
	if err != nil {
		t.Fatal(err)
	}
	awake := func() any { return h.Rec.Last("awake") }

	h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
	eventually(t, "the assertion held", func() bool { return awake() == true })
	h.Hook(t, s.ID, "Stop", `{}`)
	eventually(t, "the assertion released", func() bool { return awake() == false })
}

func TestIdleReminders(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("r", "")
	if err != nil {
		t.Fatal(err)
	}
	state := func() string { got, _ := h.Session(s.ID); return string(got.State) }
	waits := func() int { st, _ := h.Stats(7, ""); return st.Total }

	h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
	eventually(t, "working", func() bool { return state() == "working" })
	replied := h.Rec.CountAttention("replied")
	h.Hook(t, s.ID, "Notification", `{"message":"Claude is waiting for your input","notification_type":"idle_prompt"}`)
	eventually(t, "a turn killed by an error to stop showing as working", func() bool { return state() == "idle" })
	if h.Rec.CountAttention("replied") != replied || waits() != 0 {
		t.Errorf("an idle reminder raised attention (%d) or opened a wait (%d)", h.Rec.CountAttention("replied")-replied, waits())
	}

	h.Hook(t, s.ID, "Notification", `{"message":"needs permission","notification_type":"permission_prompt"}`)
	eventually(t, "waiting", func() bool { return state() == "waiting" })
	h.Hook(t, s.ID, "Notification", `{"message":"Claude is waiting for your input","notification_type":"idle_prompt"}`)
	time.Sleep(300 * time.Millisecond)
	if state() != "waiting" {
		t.Errorf("an idle reminder changed a waiting session to %s", state())
	}

	h.Hook(t, s.ID, "PostToolUse", `{"tool_name":"Edit"}`)
	eventually(t, "working again", func() bool { return state() == "working" })
	replied = h.Rec.CountAttention("replied")
	h.Hook(t, s.ID, "Stop", `{"last_assistant_message":"I fixed the test."}`)
	eventually(t, "the reply", func() bool { return h.Rec.CountAttention("replied") > replied })
	if st, _ := h.Stats(7, ""); st.Total != 1 || st.ByCause[0].Kind != "permission" {
		t.Errorf("a finished turn counted as an interruption: %+v", st.ByCause)
	}
}

func TestReplyEndingInAQuestionWaits(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("q", "")
	if err != nil {
		t.Fatal(err)
	}
	state := func() string { got, _ := h.Session(s.ID); return string(got.State) }

	h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
	eventually(t, "working", func() bool { return state() == "working" })
	h.Hook(t, s.ID, "Stop", `{"last_assistant_message":"Which branch should I use?"}`)
	eventually(t, "waiting", func() bool { return state() == "waiting" })
	h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"main"}`)
	eventually(t, "working again", func() bool { return state() == "working" })
	if st, _ := h.Stats(7, ""); st.Total != 1 || st.ByCause[0].Label != "Question in reply" {
		t.Errorf("the question did not count as a wait: %+v", st.ByCause)
	}
}

func TestEndedSessionsSurviveARestart(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("keep me", "")
	if err != nil {
		t.Fatal(err)
	}
	h.Hook(t, s.ID, "SessionEnd", `{}`)
	eventually(t, "ended by its hook", func() bool { got, _ := h.Session(s.ID); return got.State == "ended" })
	if err := h.KillSession(s.ID); err != nil && !strings.Contains(err.Error(), "ended") {
		t.Fatal(err)
	}
	eventually(t, "ended after the process is gone", func() bool { got, ok := h.Session(s.ID); return ok && got.State == "ended" && got.EndedAt > 0 })

	fresh := h.Restart(t)
	var got sessions.Session
	for _, v := range fresh.Sessions().List() {
		if v.ID == s.ID {
			got = v
		}
	}
	if got.State != "ended" || got.Title != "keep me" || got.EndedAt == 0 {
		t.Errorf("after a restart = %+v", got)
	}
	if err := fresh.DismissSession(s.ID); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(h.State, "sessions", s.ID+".json")) {
		t.Error("the state file of a dismissed session is still there")
	}
}

func TestProjectCountsFollowTheSessions(t *testing.T) {
	h := newHarness(t)
	counts := func() sessions.Project { return h.Snapshot().Project }
	a, err := h.NewSession("a", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := h.NewSession("b", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := counts(); got.Sessions != 2 || got.NeedsYou != 0 || got.Working != 0 {
		t.Errorf("fresh counts = %+v", got)
	}
	h.Hook(t, a.ID, "Notification", `{"message":"needs you","notification_type":"permission_prompt"}`)
	h.Hook(t, b.ID, "PreToolUse", `{"tool_name":"Edit"}`)
	eventually(t, "counts to follow the hooks", func() bool {
		got := counts()
		return got.Sessions == 2 && got.NeedsYou == 1 && got.Working == 1
	})
	if h.Rec.Count("projects") == 0 {
		t.Error("no projects event")
	}
	h.Hook(t, b.ID, "Stop", `{}`)
	eventually(t, "a replied session to count as idle, not needing you", func() bool {
		got := counts()
		return got.NeedsYou == 1 && got.Working == 0
	})
	h.Hook(t, a.ID, "SessionEnd", `{}`)
	eventually(t, "an ended session to leave the counts", func() bool { return counts().Sessions == 1 })
}

func TestRefreshReadsAStateFileAgainOnlyOnceItChanges(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	path := filepath.Join(h.State, "sessions", s.ID+".json")
	at := time.Now().Truncate(time.Second)
	write := func(detail string, at time.Time) {
		t.Helper()
		if err := bus.WriteState(h.State, session.Record{Session: s.ID, State: session.Working, Event: "PreToolUse", At: at, Detail: detail}); err != nil {
			t.Fatal(err)
		}
	}
	detail := func() string { row, _ := h.Session(s.ID); return row.Detail }

	write("first", at.Add(time.Second))
	h.Sessions().Refresh()
	if got := detail(); got != "first" {
		t.Fatalf("detail after a changed file = %q, want first", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	write("other", at.Add(2*time.Second)) // as long as the first, so only the time tells them apart
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	if got := detail(); got != "first" {
		t.Errorf("detail after an unchanged file = %q: the file was read again", got)
	}
	if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	if got := detail(); got != "other" {
		t.Errorf("detail after the file changed = %q, want other", got)
	}
}
