package sessions_test

import (
	"encoding/json"
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
	"github.com/nednella/agentos/desktop/stats"
	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
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

// accessTag is the access the thread's tmux session is tagged with.
func accessTag(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{@agentos-access}").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestNewThreadKeepsItsAccess(t *testing.T) {
	h := newHarness(t)
	read, err := h.NewThread("ask", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	write, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	if read.Access != session.Read || write.Access != session.Supervised {
		t.Errorf("rows = %+v and %+v, want read and supervised", read, write)
	}
	for id, want := range map[string]session.Access{read.ID: session.Read, write.ID: session.Supervised} {
		if got := accessTag(t, h, id); got != string(want) {
			t.Errorf("%s: tagged %q, want %q", id, got, want)
		}
		eventually(t, "the access in the state file", func() bool { rec, _ := bus.ReadState(h.State, id); return rec.Access == want })
	}
	if args := strings.Join(claudeArgs(t, h, read.ID), " "); !strings.Contains(args, "--permission-mode plan") {
		t.Errorf("args of the Read thread = %s", args)
	}
	if args := strings.Join(claudeArgs(t, h, write.ID), " "); !strings.Contains(args, "--permission-mode default") {
		t.Errorf("args of the Supervised thread = %s", args)
	}

	fresh := h.Restart(t)
	eventually(t, "the rows after a restart", func() bool {
		r, _ := fresh.Session(read.ID)
		w, _ := fresh.Session(write.ID)
		return r.Access == session.Read && w.Access == session.Supervised
	})
	if err := fresh.SettleThread(read.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { got, _ := fresh.Session(read.ID); return got.State == "settled" })
	if got, _ := fresh.Session(read.ID); got.Access != session.Read {
		t.Errorf("settled row = %+v, want read", got)
	}
	args := filepath.Join(fresh.ClaudeDir(), strings.ReplaceAll(read.ID, "/", "_")+".args")
	if err := os.Remove(args); err != nil {
		t.Fatal(err)
	}
	woken, err := fresh.WakeThread(read.ID)
	if err != nil {
		t.Fatal(err)
	}
	if woken.Access != session.Read || accessTag(t, fresh, read.ID) != "read" {
		t.Errorf("woken row = %+v, tagged %q", woken, accessTag(t, fresh, read.ID))
	}
	if args := strings.Join(claudeArgs(t, fresh, read.ID), " "); !strings.Contains(args, "--permission-mode plan") {
		t.Errorf("args of the woken Read thread = %s", args)
	}
}

func TestOlderBuildsChatWakesAsRead(t *testing.T) {
	wakesAsRead(t, func(file map[string]any) {
		delete(file, "access")
		file["chat"] = true
	})
}

func TestUnknownAccessWakesAsRead(t *testing.T) {
	wakesAsRead(t, func(file map[string]any) { file["access"] = "root" })
}

// wakesAsRead settles a Supervised thread, edits its state file while the app is stopped, and checks the thread is
// Read after a restart and after a wake.
func wakesAsRead(t *testing.T, edit func(file map[string]any)) {
	t.Helper()
	h := newHarness(t)
	s, err := h.NewSession("ask", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { got, _ := h.Session(s.ID); return got.State == "settled" })
	h.App.Stop()
	path := filepath.Join(h.State, "sessions", s.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	edit(file)
	if data, err = json.Marshal(file); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	fresh := h.Restart(t)
	eventually(t, "the settled row", func() bool { got, _ := fresh.Session(s.ID); return got.State == "settled" })
	if got, _ := fresh.Session(s.ID); got.Access != session.Read {
		t.Errorf("row = %+v, want read", got)
	}
	if _, err := fresh.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}
	if got := accessTag(t, fresh, s.ID); got != "read" {
		t.Errorf("woken thread tagged %q, want read", got)
	}
}

func TestReadThreadCountsInStats(t *testing.T) {
	h := newHarness(t)
	read, err := h.NewThread("ask", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	work, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{read.ID, work.ID} {
		h.Hook(t, id, "UserPromptSubmit", `{"prompt":"a"}`)
		h.Hook(t, id, "Notification", `{"message":"Claude needs your permission","notification_type":"permission_prompt"}`)
		h.Hook(t, id, "Stop", `{}`)
	}
	for _, kind := range []string{activity.Prompt, activity.SessionStart, activity.Worked, activity.Wait} {
		eventually(t, kind+" of the Read thread", func() bool {
			return slices.ContainsFunc(h.Events(t, kind), func(e activity.Event) bool { return e.Session == read.ID })
		})
	}
	eventually(t, "the Read thread's wait", func() bool {
		st, _ := h.Stats(7, "")
		return slices.ContainsFunc(st.Recent, func(w stats.Wait) bool { return w.SessionTitle == read.Title })
	})
	led, err := h.Ledger(30, "")
	if err != nil {
		t.Fatal(err)
	}
	if led.Totals.Prompts != 2 || led.Totals.Sessions != 2 || led.Totals.Chats != 0 {
		t.Errorf("totals = %+v", led.Totals)
	}

	transcript := filepath.Join(t.TempDir(), "conv.jsonl")
	appendLines(t, transcript, assistantLine("msg_1", time.Now().Add(time.Second), 1000, 100))
	h.Hook(t, read.ID, "Stop", fmt.Sprintf(`{"transcript_path":%q}`, transcript))
	eventually(t, "the Read thread's usage", func() bool { return len(h.Events(t, activity.Usage)) == 1 })
}

func TestIssueHasOneThread(t *testing.T) {
	h := newHarness(t)
	plan, err := h.StartIssueWith(7, "Plan")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Access != session.Read {
		t.Fatalf("plan = %+v, want a Read thread", plan)
	}
	work, err := h.StartIssueWith(7, "Work")
	if err != nil {
		t.Fatal(err)
	}
	if work.ID != plan.ID || work.Access != session.Read {
		t.Errorf("work = %+v, want the live Read thread %s", work, plan.ID)
	}
	out, err := exec.Command("tmux", "-L", h.Socket, "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Fatal(err)
	}
	if names := strings.Fields(string(out)); len(names) != 1 || names[0] != plan.ID {
		t.Errorf("tmux sessions = %q, want only %s", names, plan.ID)
	}
	if got := h.Sessions().IssueSessions(); got[7] != plan.ID {
		t.Errorf("issue sessions = %v, want #7 linked to its Read thread", got)
	}
}

func TestReadThreadFollowsItsBranch(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	h.GH.SetPR(prJSON("OPEN", true, "[]", 0, 0))
	read, err := h.StartIssueWith(7, "Plan")
	if err != nil {
		t.Fatal(err)
	}
	if read.Access != session.Read || read.Branch != "issue-7" {
		t.Fatalf("the Plan thread = %+v, want a Read thread on issue-7", read)
	}
	worksIn(h, t, read, read.Worktree)
	eventually(t, "the Read thread's PR", hasPR(h, read.ID))
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
	s, err := h.Sessions().Create("work", "write the queue", false, 0, session.Supervised)
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
	s, err := h.Sessions().Create("work", "", false, 0, session.Supervised)
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
			repoFixture(t, h)
			s, err := h.Sessions().Create("#12 work", "", false, 12, session.Supervised)
			if err != nil {
				t.Fatal(err)
			}
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
			h.RefreshPRs()
			if err := h.SettleThread(s.ID); err != nil {
				t.Fatal(err)
			}
			h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
			h.RefreshPRs()
			var fresh string
			eventually(t, "a new session for the issue", func() bool {
				for _, v := range h.Sessions().List() {
					if v.Issue == 12 && v.State != "settled" {
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
			repoFixture(t, h)
			s, err := h.Sessions().Create("#12 work", "", false, 12, session.Supervised)
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

func TestEveryAccessGetsTheSamePrompt(t *testing.T) {
	h := newHarness(t)
	read, err := h.NewThread("read", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	supervised, err := h.NewSession("supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	full, err := h.NewThread("full", "", "", "", "full")
	if err != nil {
		t.Fatal(err)
	}
	prompt := func(id string) string {
		args := claudeArgs(t, h, id)
		i := slices.Index(args, "--append-system-prompt")
		if i < 0 {
			t.Fatalf("args = %q", args)
		}
		return strings.Join(args[i+1:], "\n")
	}
	want := prompt(supervised.ID)
	for _, other := range []sessions.Session{read, full} {
		if got := prompt(other.ID); got != want {
			t.Errorf("a %s thread and a Supervised thread get different prompts:\n%s\n---\n%s", other.Access, got, want)
		}
	}
}

// TestEveryStartKeepsFileCheckpoints: Claude keeps the checkpoints a rewind of the files needs only when the process
// that edits them runs with the setting, so every start has it: a new thread, a fork and a wake.
func TestEveryStartKeepsFileCheckpoints(t *testing.T) {
	h := apptest.New(t)
	const setting = "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING"
	src, err := h.Sessions().Create("work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	if got := claudeEnv(t, h, src.ID)[setting]; got != "1" {
		t.Errorf("new thread: %s = %q, want 1", setting, got)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	fork, err := h.ForkSession(src.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	if got := claudeEnv(t, h, fork.ID)[setting]; got != "1" {
		t.Errorf("fork: %s = %q, want 1", setting, got)
	}

	settled := talkedThenSettled(t, h)
	for _, ext := range []string{".args", ".env"} {
		if err := os.Remove(filepath.Join(h.ClaudeDir(), strings.ReplaceAll(settled.ID, "/", "_")+ext)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.WakeThread(settled.ID); err != nil {
		t.Fatal(err)
	}
	if got := claudeEnv(t, h, settled.ID)[setting]; got != "1" {
		t.Errorf("wake: %s = %q, want 1", setting, got)
	}
}
