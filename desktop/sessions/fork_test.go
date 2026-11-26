package sessions_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/session"
)

// withConversation tells the app, as Claude's hook did, that the thread's conversation is abc, and gives the
// conversation the stand-in claude's next init names a transcript, as Claude writes it at its first prompt.
func withConversation(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	h.Observe(t, id, "Stop", fmt.Sprintf(`{"session_id":"abc","transcript_path":%q}`, writtenTranscript(t)))
	if ch, err := h.Sessions().ChatOf(id); err == nil {
		if folder, err := filepath.EvalSymlinks(ch.Dir); err == nil {
			standIn := agent.Claude{ConfigDir: h.ClaudeConfigDir()}.Transcript(folder, "fake-session")
			if err := os.MkdirAll(filepath.Dir(standIn), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(standIn, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	eventually(t, "the thread's conversation", func() bool { got, _ := h.Session(id); return got.State == "idle" })
}

// writeChat adds the events to the thread's chat log once its host has opened it.
func writeChat(t *testing.T, h *apptest.Harness, id string, events ...chat.Event) {
	t.Helper()
	path, socket := hostFiles(h, id)
	eventually(t, "the thread's host", func() bool { return exists(socket) })
	l, err := chat.OpenLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, e := range events {
		if _, err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
}

// oneReply is a chat of one turn, whose reply has two blocks: the second's uuid is r1b.
var oneReply = []chat.Event{
	{Kind: chat.KindUser, ID: "u1", Text: "first"},
	{Kind: chat.KindMessage, ID: "msg_1", UUID: "r1a", Text: "one"},
	{Kind: chat.KindMessage, ID: "msg_1", UUID: "r1b", Text: "two"},
	{Kind: chat.KindTurnEnd},
}

func kindsAndTexts(events []chat.Event) []string {
	var out []string
	for _, e := range events {
		out = append(out, string(e.Kind)+":"+e.Text)
	}
	return out
}

func TestForkCopiesTheChatUpToTheMessage(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	src, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	srcChat, err := h.Sessions().ChatOf(src.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A resumed thread's log names the folder of the thread that sent the image; the image is in its own.
	shown, later := filepath.Join(filepath.Dir(srcChat.Media), "zz22yy33", "0123456789abcdef.png"), filepath.Join(srcChat.Media, "fedcba9876543210.png")
	for _, file := range []string{filepath.Join(srcChat.Media, filepath.Base(shown)), later} {
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("image "+filepath.Base(file)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeChat(t, h, src.ID,
		chat.Event{Kind: chat.KindInit, ID: "abc"},
		chat.Event{Kind: chat.KindUser, ID: "u1", Text: "first", Files: []string{shown}},
		chat.Event{Kind: chat.KindQueued, ID: "u2", Text: "also"},
		chat.Event{Kind: chat.KindDraft, Text: "half typed"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_1", UUID: "r1a", Text: "one"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_1", UUID: "r1b", Text: "two", At: 5000},
		chat.Event{Kind: chat.KindUser, ID: "u2", Text: "also"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_2", UUID: "r2", Text: "three"},
		chat.Event{Kind: chat.KindTurnEnd},
		chat.Event{Kind: chat.KindUser, ID: "u3", Text: "second", Files: []string{later}},
	)

	fork, err := h.ForkSession(src.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	args := claudeArgs(t, h, fork.ID)
	if i := slices.Index(args, "--resume"); i < 0 || !slices.Equal(args[i:i+5], []string{"--resume", "abc", "--fork-session", "--resume-session-at", "r1b"}) {
		t.Errorf("args = %q", args)
	}
	forkLog, _ := hostFiles(h, fork.ID)
	events, err := chat.ReadLog(forkLog, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"init:", "user:first", "message:one", "message:two", "turn_end:", "start:"}
	if got := kindsAndTexts(events); !slices.Equal(got, want) {
		t.Errorf("fork's chat = %q, want %q", got, want)
	}
	for i, e := range events {
		if e.Seq != int64(i+1) {
			t.Errorf("event %d has seq %d", i, e.Seq)
		}
	}
	if end := events[len(events)-2]; end.At != 5000 {
		t.Errorf("the fork's turn ends at %d, not at its last block", end.At)
	}

	forkChat, err := h.Sessions().ChatOf(fork.ID)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(forkChat.Media, filepath.Base(shown))); string(data) != "image 0123456789abcdef.png" {
		t.Errorf("the fork's copy of the image = %q", data)
	}
	if exists(filepath.Join(forkChat.Media, filepath.Base(later))) {
		t.Error("the fork has an image sent after the message")
	}
	if !exists(filepath.Join(srcChat.Media, filepath.Base(shown))) {
		t.Error("the source lost its image")
	}
	if srcEvents, _ := chat.ReadLog(srcChat.Log, 0); len(srcEvents) != 11 {
		t.Errorf("the source's chat changed: %q", kindsAndTexts(srcEvents))
	}

	for _, at := range []string{"nope", ""} {
		if _, err := h.ForkSession(src.ID, at); err == nil || err.Error() != "That message is not in the thread's chat" {
			t.Errorf("fork at %q: %v", at, err)
		}
	}
	fresh, err := h.Sessions().Create(h.Sessions().Current(), "fresh", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ForkSession(fresh.ID, "r1b"); err == nil || err.Error() != "This thread has no conversation to fork yet" {
		t.Errorf("fork of a thread with no conversation: %v", err)
	}
}

func TestForkStartsOnlyAfterTheLastCompaction(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	src, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, append(slices.Clone(oneReply),
		chat.Event{Kind: chat.KindUser, ID: "u2", Text: "/compact"},
		chat.Event{Kind: chat.KindCompact},
		chat.Event{Kind: chat.KindTurnEnd},
		chat.Event{Kind: chat.KindUser, ID: "u3", Text: "second"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_2", UUID: "r2", Text: "three"},
		chat.Event{Kind: chat.KindTurnEnd},
	)...)

	_, err = h.ForkSession(src.ID, "r1b")
	if want := "This message is from before the chat was compacted, so a fork cannot start there"; err == nil || err.Error() != want {
		t.Errorf("fork before the compaction: %v, want %q", err, want)
	}
	if n := len(h.Sessions().List()); n != 1 {
		t.Errorf("%d threads after the refused fork, want only the source", n)
	}

	fork, err := h.ForkSession(src.ID, "r2")
	if err != nil {
		t.Fatal(err)
	}
	args := claudeArgs(t, h, fork.ID)
	if i := slices.Index(args, "--resume-session-at"); i < 0 || args[i+1] != "r2" {
		t.Errorf("args = %q, want a fork at r2", args)
	}
}

func TestForkKeepsFolderAccessAndModel(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	for access, mode := range map[session.Access]string{session.Read: "plan", session.Supervised: "default", session.Full: "bypassPermissions"} {
		t.Run(string(access), func(t *testing.T) {
			src, err := h.Sessions().CreateThread("ask", sessions.StartOptions{Model: project.Model{Model: "haiku", Effort: "low"}, Access: access})
			if err != nil {
				t.Fatal(err)
			}
			withConversation(t, h, src.ID)
			writeChat(t, h, src.ID, oneReply...)
			fork, err := h.ForkSession(src.ID, "r1b")
			if err != nil {
				t.Fatal(err)
			}
			if fork.Title != "ask (fork)" || fork.Access != access || fork.Model != "haiku" || fork.Effort != "low" || fork.ForkOf != src.ID || fork.State == "settled" {
				t.Errorf("fork = %+v", fork)
			}
			srcChat, _ := h.Sessions().ChatOf(src.ID)
			forkChat, _ := h.Sessions().ChatOf(fork.ID)
			if forkChat.Dir != srcChat.Dir || forkChat.Dir == "" {
				t.Errorf("fork works in %q, its source in %q", forkChat.Dir, srcChat.Dir)
			}
			args := strings.Join(claudeArgs(t, h, fork.ID), " ")
			if !strings.Contains(args, "--model claude-haiku-5-5 --effort low") || !strings.Contains(args, "--permission-mode "+mode) {
				t.Errorf("args = %s", args)
			}
		})
	}
}

func TestForkDoesNotTakeOverThePR(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n"})
	repoFixture(t, h)
	src, err := h.Sessions().Create(h.Sessions().Current(), "#12 work", "", false, 12, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	eventually(t, "the source's PR", hasPR(h, src.ID))

	fork, err := h.ForkSession(src.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	if fork.Issue != 12 || fork.Title != "#12 work (fork)" {
		t.Errorf("fork = %+v", fork)
	}
	if got := h.Sessions().IssueSessions()[12]; got != src.ID {
		t.Errorf("the issue's session is %s, want the source %s", got, src.ID)
	}
	if err := h.Sessions().Track(fork.ID, "issue-12"); err == nil {
		t.Error("the fork tracked the branch")
	}

	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
	h.RefreshPRs()
	eventually(t, "the review on the source's stdin", func() bool {
		users := userLines(t, h, src.ID)
		return len(users) == 1 && strings.Contains(users[0], "/address-review 12")
	})
	time.Sleep(300 * time.Millisecond)
	if users := userLines(t, h, fork.ID); len(users) != 0 {
		t.Errorf("the fork got %q", users)
	}
	if got, _ := h.Session(fork.ID); got.PR != nil || got.PRAttention != "" {
		t.Errorf("fork = %+v", got)
	}
}

func TestForkOfAnEndedThread(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	src, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	writeChat(t, h, src.ID, oneReply...)
	ended(t, h, src)

	fork, err := h.ForkSession(src.ID, "r1a")
	if err != nil {
		t.Fatal(err)
	}
	if fork.State == "settled" || fork.ForkOf != src.ID {
		t.Errorf("fork = %+v", fork)
	}
	if i := slices.Index(claudeArgs(t, h, fork.ID), "--resume-session-at"); i < 0 {
		t.Error("the fork did not fork")
	}
	forkLog, _ := hostFiles(h, fork.ID)
	events, _ := chat.ReadLog(forkLog, 0)
	if got, want := kindsAndTexts(events), []string{"user:first", "message:one", "turn_end:", "start:"}; !slices.Equal(got, want) {
		t.Errorf("fork's chat = %q, want %q", got, want)
	}
	if got, ok := h.Session(src.ID); !ok || got.State != "settled" {
		t.Errorf("the source after the fork = %+v, %v", got, ok)
	}
}

func TestForkStaysAFork(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	src, err := h.Sessions().Create(h.Sessions().Current(), "#12 work", "", false, 12, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	fork, err := h.ForkSession(src.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}

	withConversation(t, h, fork.ID)

	fresh := h.Restart(t)
	eventually(t, "the fork after a restart", func() bool { got, ok := fresh.Session(fork.ID); return ok && got.ForkOf == src.ID })
	if err := fresh.SettleThread(fork.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fork settled", func() bool { got, _ := fresh.Session(fork.ID); return got.State == "settled" && got.ForkOf == src.ID })
	woken, err := fresh.WakeThread(fork.ID)
	if err != nil {
		t.Fatal(err)
	}
	if woken.ID != fork.ID || woken.ForkOf != src.ID || woken.Issue != 12 {
		t.Errorf("woken fork = %+v", woken)
	}
	eventually(t, "the woken fork resuming its own conversation", func() bool {
		args := fresh.Args(t, fork.ID)
		i := slices.Index(args, "--resume")
		return i >= 0 && args[i+1] == "abc" && !slices.Contains(args, "--fork-session")
	})
	if got := fresh.Sessions().IssueSessions()[12]; got != src.ID {
		t.Errorf("the issue's session is %s, want the source %s", got, src.ID)
	}
}

func TestForkRemembersWhereItForked(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	src, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	fork, err := h.ForkSession(src.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	if fork.ForkAt != "r1b" {
		t.Errorf("fork cut at %q, want r1b", fork.ForkAt)
	}
	withConversation(t, h, fork.ID)
	if got, _ := h.Session(fork.ID); got.ForkAt != "r1b" {
		t.Errorf("after its first turn the fork says %q", got.ForkAt)
	}

	fresh := h.Restart(t)
	eventually(t, "the fork after a restart", func() bool { got, ok := fresh.Session(fork.ID); return ok && got.ForkAt == "r1b" })
	if err := fresh.SettleThread(fork.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the settled fork", func() bool { got, _ := fresh.Session(fork.ID); return got.State == "settled" && got.ForkAt == "r1b" })
	woken, err := fresh.WakeThread(fork.ID)
	if err != nil {
		t.Fatal(err)
	}
	if woken.ForkAt != "r1b" {
		t.Errorf("woken fork cut at %q", woken.ForkAt)
	}
	if got, _ := fresh.Session(src.ID); got.ForkAt != "" {
		t.Errorf("the source says it was cut at %q", got.ForkAt)
	}
}

// forkHarness is a project whose issue 7 has a pushed worktree, worked on by a thread with a fork. Its clean-up
// command notes each branch it is run for in the file ran names.
func forkHarness(t *testing.T) (h *apptest.Harness, wt, ran string, src, fork sessions.Session) {
	t.Helper()
	ran = filepath.Join(t.TempDir(), "cleanups")
	h = apptest.NewWith(t, apptest.Options{CleanupCommand: fmt.Sprintf("echo {branch} >> %q && ", ran) + project.DefaultCleanupCommand})
	repoFixture(t, h)
	src, err := h.Sessions().Create(h.Sessions().Current(), "#7 work", "", false, 7, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	wt = src.Worktree
	pushWork(t, wt)
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	if fork, err = h.ForkSession(src.ID, "r1b"); err != nil {
		t.Fatal(err)
	}
	return h, wt, ran, src, fork
}

// cleanupRuns are the branches the clean-up command ran for.
func cleanupRuns(ran string) []string {
	data, _ := os.ReadFile(ran)
	return strings.Fields(string(data))
}

func TestForkIsNeverCleanedUp(t *testing.T) {
	t.Parallel()
	h, wt, ran, src, fork := forkHarness(t)
	if err := h.Cleanup(fork.ID, true); err == nil {
		t.Error("a forced clean-up of the fork did not fail")
	}
	if err := h.SettleThread(fork.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fork settled", func() bool { return stateOf(h, fork.ID) == "settled" })
	if err := h.DeleteThread(fork.ID, false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if runs := cleanupRuns(ran); len(runs) != 0 || !exists(wt) {
		t.Errorf("the clean-up command ran for %q; worktree there: %v", runs, exists(wt))
	}
	if got, ok := h.Session(src.ID); !ok || got.State == "settled" {
		t.Errorf("the source = %+v, %v", got, ok)
	}
}

func TestForkIsRefusedDuringACleanup(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	src := issueThread(h, t, 7)
	pushWork(t, src.Worktree)
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	release, done := heldCleanup(t, h, src.ID)

	_, err := h.ForkSession(src.ID, "r1b")
	if want := fmt.Sprintf("Thread %d is being cleaned up; fork it when that ends", src.N); err == nil || err.Error() != want {
		t.Errorf("fork = %v, want %q", err, want)
	}
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !cleanedUp(h, src.ID)() || exists(src.Worktree) {
		t.Errorf("the clean-up did not finish: worktree there %v", exists(src.Worktree))
	}
	if list := h.Sessions().List(); len(list) != 1 {
		t.Errorf("a fork started: %+v", list)
	}
}

func TestEndedForkInTheWorktreeDoesNotBlock(t *testing.T) {
	t.Parallel()
	h, wt, ran, src, fork := forkHarness(t)
	h.Observe(t, fork.ID, "PreToolUse", fmt.Sprintf(`{"cwd":%q}`, wt))
	eventually(t, "the fork at work", func() bool { return stateOf(h, fork.ID) == "working" })
	if err := h.SettleThread(fork.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fork settled", func() bool { return stateOf(h, fork.ID) == "settled" })

	openThenMerge(h)
	eventually(t, "the source cleaned up", cleanedUp(h, src.ID))
	if runs := cleanupRuns(ran); !slices.Equal(runs, []string{"issue-7"}) || exists(wt) {
		t.Errorf("the clean-up command ran for %q; worktree there: %v", runs, exists(wt))
	}
}

func TestForkIsToldNotToTrack(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	src, err := h.Sessions().Create(h.Sessions().Current(), "Source work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	fork, err := h.ForkSession(src.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	args := claudeArgs(t, h, fork.ID)
	if !strings.Contains(strings.Join(args, "\n"), prompts.Fork("Source work")) {
		t.Errorf("the fork's system prompt has no fork text: %q", args)
	}
	if got := claudeArgs(t, h, src.ID); strings.Contains(strings.Join(got, "\n"), prompts.Fork("Source work")) {
		t.Error("the source is told it is a fork")
	}
}

func TestForkOfAThreadWhoseFolderIsGoneIsRefused(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	repoFixture(t, h)
	src, err := h.NewThreadIn("work", "", "supervised", "worktree")
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	ended(t, h, src)
	if err := os.RemoveAll(src.Worktree); err != nil {
		t.Fatal(err)
	}

	_, err = h.ForkSession(src.ID, "r1b")
	if want := src.Worktree + " is gone, so this thread cannot be forked there"; err == nil || err.Error() != want {
		t.Errorf("fork = %v, want %q", err, want)
	}
	if rows := h.Sessions().List(); len(rows) != 1 {
		t.Errorf("rows = %+v, want only the source", rows)
	}
}
