package sessions_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/prompts"
)

func claudeArgs(t *testing.T, h *apptest.Harness, id string) []string {
	t.Helper()
	var args []string
	eventually(t, "claude started", func() bool {
		args = h.Args(t, id)
		return len(args) > 0
	})
	return args
}

// claudeEnv is what of the session's environment the stand-in claude was started with, by name.
func claudeEnv(t *testing.T, h *apptest.Harness, id string) map[string]string {
	t.Helper()
	claudeArgs(t, h, id) // the env file comes first
	data, err := os.ReadFile(filepath.Join(h.ClaudeDir(), strings.ReplaceAll(id, "/", "_")+".env"))
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		k, v, _ := strings.Cut(line, "=")
		env[k] = v
	}
	return env
}

func TestNewChatStartsClaudeInPlanMode(t *testing.T) {
	h := apptest.New(t)

	tests := []struct {
		name, title, wantTitle string
	}{
		{"a title", "what does the queue do", "what does the queue do"},
		{"no title", "  ", "chat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := h.NewChat(tt.title)
			if err != nil {
				t.Fatal(err)
			}
			if !c.Chat || c.Title != tt.wantTitle || c.Issue != 0 || c.State != "idle" {
				t.Errorf("chat = %+v", c)
			}
			args := claudeArgs(t, h, c.ID)
			if i := slices.Index(args, "--permission-mode"); i < 0 || args[i+1] != "plan" {
				t.Errorf("args = %q", args)
			}
			if i := slices.Index(args, "--allowedTools"); i < 0 || !strings.Contains(args[i+1], "Bash(agentos new:*)") {
				t.Errorf("args = %q", args)
			}
			if joined := strings.Join(args, "\n"); !strings.Contains(joined, prompts.Chat()) || strings.Contains(joined, prompts.Task()) {
				t.Errorf("system prompt of the chat = %q", joined)
			}
		})
	}

	work, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	if work.Chat {
		t.Error("a session is flagged as a chat")
	}
	args := claudeArgs(t, h, work.ID)
	if slices.Contains(args, "--permission-mode") || !strings.Contains(strings.Join(args, "\n"), prompts.Task()) {
		t.Errorf("args of a session = %q", args)
	}
}

func TestChatFlagSurvivesARestart(t *testing.T) {
	h := newHarness(t)
	live, err := h.NewChat("live")
	if err != nil {
		t.Fatal(err)
	}
	gone, err := h.NewChat("gone")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := h.NewSession("plain", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.KillSession(gone.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the chat ended", func() bool { got, _ := h.Session(gone.ID); return got.State == "ended" })

	fresh := h.Restart(t)
	for id, want := range map[string]bool{live.ID: true, gone.ID: true, plain.ID: false} {
		eventually(t, "the row after a restart", func() bool {
			got, ok := fresh.Session(id)
			return ok && got.Chat == want
		})
	}
	if got, _ := fresh.Session(gone.ID); got.State != "ended" {
		t.Errorf("ended chat = %+v", got)
	}
}

func TestChatStaysOutOfTheStats(t *testing.T) {
	h := newHarness(t)
	chat, err := h.NewChat("ask")
	if err != nil {
		t.Fatal(err)
	}
	work, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{chat.ID, work.ID} {
		h.Hook(t, id, "UserPromptSubmit", `{"prompt":"a"}`)
		h.Hook(t, id, "Notification", `{"message":"Claude needs your permission","notification_type":"permission_prompt"}`)
		h.Hook(t, id, "Stop", `{}`)
	}
	eventually(t, "the session's prompt", func() bool { return len(h.Events(t, activity.Prompt)) == 1 })
	eventually(t, "the session's wait", func() bool { st, _ := h.Stats(7, ""); return st.Total > 0 })

	for _, kind := range []string{activity.Prompt, activity.SessionStart, activity.Worked, activity.Wait} {
		for _, e := range h.Events(t, kind) {
			if e.Session == chat.ID {
				t.Errorf("%s event of the chat: %+v", kind, e)
			}
		}
	}
	st, _ := h.Stats(7, "")
	for _, w := range st.Recent {
		if w.SessionTitle == chat.Title {
			t.Errorf("wait of the chat: %+v", w)
		}
	}
	led, err := h.Ledger(30, "")
	if err != nil {
		t.Fatal(err)
	}
	if starts := h.Events(t, activity.ChatStart); len(starts) != 1 || starts[0].Session != chat.ID {
		t.Errorf("chat_start events = %+v", starts)
	}
	if led.Totals.Prompts != 1 || led.Totals.Sessions != 1 || led.Totals.Chats != 1 {
		t.Errorf("totals = %+v", led.Totals)
	}

	transcript := filepath.Join(t.TempDir(), "conv.jsonl")
	appendLines(t, transcript, assistantLine("msg_1", time.Now().Add(time.Second), 1000, 100))
	h.Hook(t, chat.ID, "Stop", fmt.Sprintf(`{"transcript_path":%q}`, transcript))
	eventually(t, "the chat's usage", func() bool { return len(h.Events(t, activity.ChatUsage)) == 1 })
	if got := h.Events(t, activity.Usage); len(got) != 0 {
		t.Errorf("usage events = %+v, want the chat's under chat_usage only", got)
	}

	if err := h.KillSession(chat.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the chat ended", func() bool { got, _ := h.Session(chat.ID); return got.State == "ended" })
	if got := h.Events(t, activity.SessionEnd); len(got) != 0 {
		t.Errorf("session_end events = %+v", got)
	}
}

func TestChatHasNoBranchOrPR(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	h.GH.SetPR(prJSON("OPEN", true, "[]", 0, 0))

	chat, err := h.NewChat("ask")
	if err != nil {
		t.Fatal(err)
	}
	work := plainSession(h, t)
	madeBranch(h, t, work)
	worksIn(h, t, chat, h.Dir)
	worksIn(h, t, work, h.Dir)
	eventually(t, "the session's branch", func() bool { got, _ := h.Session(work.ID); return got.Branch == "my-fix" })
	eventually(t, "the session's PR", hasPR(h, work.ID))

	got, _ := h.Session(chat.ID)
	if got.Branch != "" || got.PR != nil || got.Worktree != "" {
		t.Errorf("chat = %+v", got)
	}
	if err := h.Cleanup(chat.ID, true); err == nil {
		t.Error("cleaning up a chat did not fail")
	}
}

func chatLogHas(h *apptest.Harness, id, kind, text string) bool {
	chatLog, _ := hostFiles(h, id)
	data, _ := os.ReadFile(chatLog)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, `"kind":"`+kind+`"`) && strings.Contains(line, text) {
			return true
		}
	}
	return false
}

func TestUnsentPromptIsADraft(t *testing.T) {
	h := apptest.New(t)
	s, err := h.Sessions().Create("work", "write the queue", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the draft in the chat log", func() bool { return chatLogHas(h, s.ID, "draft", "write the queue") })
	if users := userLines(t, h, s.ID); len(users) != 0 {
		t.Errorf("user lines on stdin = %v, want none", users)
	}
}

func TestManualPromptIsADraftOnAHostedSession(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    session_prompt_send: manual\n"})
	if _, err := h.Sessions().CreateBriefed("set up", "write the queue", "brief"); err != nil {
		t.Fatal(err)
	}
	id := h.Sessions().List()[0].ID
	eventually(t, "the draft in the chat log", func() bool { return chatLogHas(h, id, "draft", "write the queue") })
	if users := userLines(t, h, id); len(users) != 0 {
		t.Errorf("user lines on stdin = %v, want none", users)
	}
}

func TestTypeIntoIsADraft(t *testing.T) {
	h := apptest.New(t)
	s, err := h.Sessions().Create("work", "", false, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.TypeInto(s.ID, "Address the review comments on PR #12."); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the draft in the chat log", func() bool {
		return chatLogHas(h, s.ID, "draft", "Address the review comments on PR #12.")
	})
	if users := userLines(t, h, s.ID); len(users) != 0 {
		t.Errorf("user lines on stdin = %v, want none", users)
	}
}

func TestWakeOfAnEndedHostedSession(t *testing.T) {
	for _, tt := range []struct {
		name, extra string
		draft       bool
	}{
		{"auto sends", "", false},
		{"manual drafts", "    session_prompt_send: manual\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n" + tt.extra})
			s, err := h.Sessions().Create("#12 work", "", false, 12, "")
			if err != nil {
				t.Fatal(err)
			}
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
			h.RefreshPRs()
			if err := h.KillSession(s.ID); err != nil {
				t.Fatal(err)
			}
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
			h.RefreshPRs()
			var fresh string
			eventually(t, "a new session for the issue", func() bool {
				for _, v := range h.Sessions().List() {
					if v.Issue == 12 && v.State != "ended" {
						fresh = v.ID
						return true
					}
				}
				return false
			})
			if tt.draft {
				eventually(t, "the draft", func() bool { return chatLogHas(h, fresh, "draft", "/address-review 12") })
				time.Sleep(300 * time.Millisecond)
				if users := userLines(t, h, fresh); len(users) != 0 {
					t.Errorf("user lines on stdin = %v, want none", users)
				}
				return
			}
			eventually(t, "the prompt on stdin", func() bool { return len(userLines(t, h, fresh)) == 1 })
			if chatLogHas(h, fresh, "draft", "/address-review 12") {
				t.Error("an auto wake was left as a draft")
			}
		})
	}
}

func TestManualWakeIsADraft(t *testing.T) {
	for _, tt := range []struct {
		name string
		work bool
	}{{"live", false}, {"held while working", true}} {
		t.Run(tt.name, func(t *testing.T) {
			h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n    session_prompt_send: manual\n"})
			s, err := h.Sessions().Create("#12 work", "", false, 12, "")
			if err != nil {
				t.Fatal(err)
			}
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
			h.RefreshPRs()
			if tt.work {
				h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
				eventually(t, "working", func() bool { return stateOf(h, s.ID) == "working" })
			}
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
			h.RefreshPRs()
			eventually(t, "the draft", func() bool { return chatLogHas(h, s.ID, "draft", "/address-review 12") })
			time.Sleep(300 * time.Millisecond)
			if users := userLines(t, h, s.ID); len(users) != 0 {
				t.Errorf("user lines on stdin = %v, want none", users)
			}
		})
	}
}
