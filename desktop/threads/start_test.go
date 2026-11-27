package threads_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/chat"
	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/bus"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/thread"
)

// listedStarting is the latest threads event's row of the thread, when it says the thread starts.
func listedStarting(h *apptest.Harness, id string) bool {
	for _, row := range h.Rec.LastThreads() {
		if row.ID == id {
			return row.Start == threads.Starting
		}
	}
	return false
}

func mentions(lines []string, text string) int {
	n := 0
	for _, line := range lines {
		if strings.Contains(line, text) {
			n++
		}
	}
	return n
}

func TestNewThreadStartsWithoutWaitingForTheNetwork(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	origin := repoFixture(t, h)
	slowOrigin(t, h, origin, 3*time.Second)
	old := gitIn(t, h.Dir, "rev-parse", "origin/main")
	// Another clone pushes a newer commit, which only a fetch brings in.
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, filepath.Dir(clone), "clone", origin, clone)
	if err := os.WriteFile(filepath.Join(clone, "news"), []byte("news\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, clone, "add", "news")
	gitIn(t, clone, "commit", "-m", "news")
	gitIn(t, clone, "push", "origin", "main")

	began := time.Now()
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > time.Second {
		t.Errorf("the call took %v", took)
	}
	if s.Start != threads.Starting || s.State != "working" || s.FirstMessage != "make it faster" || s.Title != "Fix login" {
		t.Errorf("the row = %+v, want it starting with the first message", s)
	}
	if !listedStarting(h, s.ID) {
		t.Errorf("the threads event does not list %s as starting: %+v", s.ID, h.Rec.LastThreads())
	}
	eventually(t, "the thread to start", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	if took := time.Since(began); took > 2500*time.Millisecond {
		t.Errorf("the thread took %v to start, which the 3 s fetch holds up", took)
	}
	row, _ := h.Thread(s.ID)
	if row.Worktree == "" || !exists(row.Worktree) || row.FirstMessage != "" || row.State == "waiting" || row.Branch != "fix-login" {
		t.Errorf("the started row = %+v", row)
	}
	if head := gitIn(t, row.Worktree, "rev-parse", "HEAD"); head != old {
		t.Errorf("the worktree starts at %s, want the local copy of origin/main, %s", head, old)
	}
	eventually(t, "the first message on the agent's stdin", func() bool { return mentions(userLines(t, h, s.ID), "make it faster") >= 1 })
	time.Sleep(300 * time.Millisecond)
	if n := mentions(userLines(t, h, s.ID), "make it faster"); n != 1 {
		t.Errorf("the first message reached the agent %d times, want once", n)
	}
	eventually(t, "the fetch after the start", func() bool { return gitIn(t, h.Dir, "rev-parse", "origin/main") != old })
}

func TestSlowSetupCommandDoesNotHoldTheAgent(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{SetupCommand: "sleep 30"})
	repoFixture(t, h)
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to start", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	eventually(t, "the first message", func() bool { return mentions(userLines(t, h, s.ID), "make it faster") == 1 })
	row, _ := h.Thread(s.ID)
	if row.Setup != threads.SetupRunning || row.SetupShell == "" {
		t.Errorf("the row = %+v, want the set-up still running", row)
	}
}

func TestSlowAgentShowsAsStartingThenGetsTheFirstMessageOnce(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{ClaudeStartup: "sleep 2"})
	repoFixture(t, h)

	began := time.Now()
	s, err := h.NewThreadLater("Slow one", "hello there")
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > time.Second {
		t.Errorf("the call took %v", took)
	}
	if s.Start != threads.Starting {
		t.Errorf("the row = %+v, want it starting", s)
	}
	eventually(t, "the thread to start", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	eventually(t, "the first message on the agent's stdin", func() bool { return mentions(userLines(t, h, s.ID), "hello there") >= 1 })
	time.Sleep(300 * time.Millisecond)
	if n := mentions(userLines(t, h, s.ID), "hello there"); n != 1 {
		t.Errorf("the first message reached the agent %d times, want once", n)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestFailedStartShowsOnTheThreadAndRetries(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	tmux := newFakeTmux(t)
	tmux.fail(t, "new", true)

	s, err := h.NewThreadLater("Tidy up", "clean the tests")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the start to fail", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == threads.StartFailed
	})
	row, _ := h.Thread(s.ID)
	if !strings.Contains(row.StartError, "no room for a session") || row.FirstMessage != "clean the tests" || row.State != "waiting" || row.Detail != row.StartError {
		t.Errorf("the failed row = %+v", row)
	}
	tree := filepath.Join(h.Dir, "trees", "tidy-up")
	if row.Worktree == "" || !exists(tree) || len(userLines(t, h, s.ID)) != 0 {
		t.Errorf("the failed start has the worktree %q, which should stay for the retry, and reached the agent: %v", row.Worktree, userLines(t, h, s.ID))
	}
	tmux.fail(t, "new", false)
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	row, _ = h.Thread(s.ID)
	if row.Branch != "tidy-up" || row.Worktree == "" || row.StartError != "" || !strings.HasSuffix(row.Worktree, filepath.Join("trees", "tidy-up")) {
		t.Errorf("the retried row = %+v", row)
	}
	eventually(t, "the first message", func() bool { return mentions(userLines(t, h, s.ID), "clean the tests") == 1 })
	if err := h.RetryThreadStart(s.ID); err == nil {
		t.Error("a retry of a thread that runs was taken")
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestFailedStartIsDeleted(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	newFakeTmux(t).fail(t, "new", true)
	s, err := h.NewThreadLater("Doomed", "never mind")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the start to fail", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == threads.StartFailed
	})
	if err := h.Threads().DeleteAndWait(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if rows := h.Threads().List(); len(rows) != 0 {
		t.Errorf("rows = %+v", rows)
	}
	if err := h.RetryThreadStart(s.ID); err == nil {
		t.Error("a deleted thread was retried")
	}
}

func TestDeleteStopsAStartThatRuns(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 3*time.Second)
	s, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Threads().DeleteAndWait(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if rows := h.Threads().List(); len(rows) != 0 {
		t.Errorf("rows = %+v", rows)
	}
	time.Sleep(500 * time.Millisecond)
	if rows := h.Threads().List(); len(rows) != 0 {
		t.Errorf("the start went on after the delete: %+v", rows)
	}
	if exists(filepath.Join(h.Dir, "trees", "issue-7")) {
		t.Error("the worktree of a deleted start is there")
	}
}

func TestIssueThreadIsNotStartedTwiceWhileItStarts(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 2*time.Second)

	first, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	if first.Start != threads.Starting || first.Issue != 7 || first.Branch != "issue-7" {
		t.Errorf("the issue's thread = %+v", first)
	}
	second, err := h.StartIssueLater(7)
	if err != nil || second.ID != first.ID {
		t.Fatalf("a second start = %+v, %v; want thread %s", second, err, first.ID)
	}
	eventually(t, "the thread to start", func() bool {
		row, _ := h.Thread(first.ID)
		return row.Start == ""
	})
	eventually(t, "the action's command", func() bool { return mentions(userLines(t, h, first.ID), "/ship 7") == 1 })
	time.Sleep(300 * time.Millisecond)
	if rows := h.Threads().List(); len(rows) != 1 || mentions(userLines(t, h, first.ID), "/ship 7") != 1 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestRenameWhileTheThreadStarts(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 2*time.Second)
	s, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Threads().Rename(s.ID, "Second name"); err != nil {
		t.Fatal(err)
	}
	if row, _ := h.Thread(s.ID); row.Title != "Second name" || row.Start != threads.Starting {
		t.Errorf("the row = %+v, want the new title while it starts", row)
	}
	eventually(t, "the thread to start", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	h.Threads().Refresh()
	if row, _ := h.Thread(s.ID); row.Title != "Second name" {
		t.Errorf("title after the start = %q", row.Title)
	}
}

func TestListedStartLeavesNoStateFileUntilItsAgentRuns(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 2*time.Second)
	s, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	h.Threads().Refresh()
	if rec, _ := bus.ReadState(h.State, s.ID); rec.Thread != "" {
		t.Errorf("a thread that has not started has a state file: %+v", rec)
	}
	eventually(t, "the thread to start", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	row, _ := h.Thread(s.ID)
	rec, _ := bus.ReadState(h.State, s.ID)
	if rec.Branch != "issue-7" || rec.Path != row.Worktree || !strings.HasPrefix(rec.Title, "#7") {
		t.Errorf("the saved identity = %+v, want the branch and folder of %+v", rec.Identity, row)
	}
}

func TestThreadThatStartsCannotBeSlept(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 2*time.Second)
	s, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Threads().SettleAsOwner(s.ID); err == nil || !strings.Contains(err.Error(), "still starting") {
		t.Errorf("sleep = %v", err)
	}
	if err := h.Threads().SetAccess(s.ID, "read"); err == nil || !strings.Contains(err.Error(), "still starting") {
		t.Errorf("access = %v", err)
	}
	if err := h.Threads().SetFast(s.ID, true); err == nil || !strings.Contains(err.Error(), "still starting") {
		t.Errorf("fast = %v", err)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestRetryKeepsTheBranchTheFirstTryPicked(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	tmux := newFakeTmux(t)
	tmux.fail(t, "new", true)
	s, err := h.NewThreadLater("Tidy up", "clean")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the start to fail", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == threads.StartFailed
	})
	tmux.fail(t, "new", false)
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	if row, _ := h.Thread(s.ID); row.Branch != "tidy-up" {
		t.Errorf("branch after the retry = %q, want tidy-up", row.Branch)
	}
	if got := gitIn(t, h.Dir, "branch", "--list", "tidy-up*"); strings.Contains(got, "tidy-up-2") {
		t.Errorf("the retry made a second branch: %s", got)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestRetryReusesTheWorktreeTheFailedStartMade(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{SetupCommand: "touch ran-setup"})
	repoFixture(t, h)
	tmux := newFakeTmux(t)
	tmux.fail(t, "new", true)
	s, err := h.NewThreadLater("Tidy up", "clean")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the start to fail", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == threads.StartFailed
	})
	failed, _ := h.Thread(s.ID)
	tree := filepath.Join(h.Dir, "trees", "tidy-up")
	// Work the owner did in it between the failure and the retry: git would not remove it, and it must still be the
	// thread's own worktree, with the set-up command to run.
	if err := os.WriteFile(filepath.Join(tree, "notes.txt"), []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tmux.fail(t, "new", false)
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	row, _ := h.Thread(s.ID)
	if row.Worktree != failed.Worktree || row.Branch != "tidy-up" {
		t.Errorf("the retried row = %+v, want the worktree %s of the failed one", row, failed.Worktree)
	}
	if n := strings.Count(gitIn(t, h.Dir, "worktree", "list"), "\n") + 1; n != 2 {
		t.Errorf("git lists %d worktrees, want the project's and the thread's: %s", n, gitIn(t, h.Dir, "worktree", "list"))
	}
	eventually(t, "the set-up command", func() bool { return exists(filepath.Join(tree, "ran-setup")) })
	if !exists(filepath.Join(tree, "notes.txt")) {
		t.Error("the owner's file is gone")
	}
}

func TestMessageSentWhileTheThreadStartsGoesAfterTheFirstOne(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 2*time.Second)
	s, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "second words"); err != nil {
		t.Fatal(err)
	}
	if row, _ := h.Thread(s.ID); len(row.StartQueue) != 1 || row.StartQueue[0] != "second words" || row.Detail != "Setting up the worktree" {
		t.Errorf("the row = %+v, want one message queued and the step", row)
	}
	eventually(t, "both messages", func() bool { return len(userLines(t, h, s.ID)) >= 2 })
	users := userLines(t, h, s.ID)
	if !strings.Contains(users[0], "/ship 7") || !strings.Contains(users[1], "second words") || len(users) != 2 {
		t.Errorf("messages = %v", users)
	}
	eventually(t, "the start to end", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == "" && len(row.StartQueue) == 0
	})
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestSetupCommandStartsAfterTheAgent(t *testing.T) {
	seen := filepath.Join(t.TempDir(), "sessions")
	// The command lists the tmux sessions that exist when it starts.
	h := apptest.NewWith(t, apptest.Options{SetupCommand: "tmux -L {socket} list-sessions -F '#{session_name}' > " + seen})
	repoFixture(t, h)
	newFakeTmux(t).fail(t, "slow-new", true)
	s, err := h.NewThreadIn("Tidy up", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	var data []byte
	eventually(t, "the command to list the sessions", func() bool {
		data, _ = os.ReadFile(seen)
		return len(data) > 0
	})
	if !strings.Contains(string(data), s.ID) {
		t.Errorf("the sessions when the set-up started = %q, want the thread's own, %s", data, s.ID)
	}
}

const startDot = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestMessageWithAnImageDoesNotJumpTheQueueWhileTheThreadStarts(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 2*time.Second)
	s, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	for _, send := range []func() error{
		func() error { _, err := h.ChatSend(s.ID, "text one"); return err },
		func() error {
			_, err := h.ChatSend(s.ID, "with an image", chat.Image{Base64: startDot, Mime: "image/png"})
			return err
		},
		func() error { _, err := h.ChatSend(s.ID, "text two"); return err },
	} {
		if err := send(); err != nil {
			t.Fatal(err)
		}
	}
	if row, _ := h.Thread(s.ID); len(row.StartQueue) != 3 {
		t.Errorf("the row holds %v, want the three messages", row.StartQueue)
	}
	eventually(t, "all four messages", func() bool { return len(userLines(t, h, s.ID)) >= 4 })
	users := userLines(t, h, s.ID)
	for i, want := range []string{"/ship 7", "text one", "with an image", "text two"} {
		if i >= len(users) || !strings.Contains(users[i], want) {
			t.Fatalf("message %d = %q, want it to hold %q; all: %v", i, users[i], want, users)
		}
	}
	if !strings.Contains(users[2], `"media_type":"image/png"`) {
		t.Errorf("the third message = %s, want its image", users[2])
	}
}

func TestMessageHeldWithNoFirstMessageWaitsForTheHost(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{})
	slowOrigin(t, h, repoFixture(t, h), 1*time.Second)
	s, err := h.NewThreadLater("Fix login", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "only message"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the held message", func() bool { return mentions(userLines(t, h, s.ID), "only message") == 1 })
	eventually(t, "the start to end", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == "" && len(row.StartQueue) == 0
	})
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestSetupCommandThatCannotStartShowsAsFailed(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{SetupCommand: "true"})
	repoFixture(t, h)
	newFakeTmux(t).fail(t, "new-shell", true)
	s, err := h.NewThreadIn("Tidy up", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the set-up to show as failed", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Setup == threads.SetupFailed
	})
	row, _ := h.Thread(s.ID)
	if !strings.Contains(row.SetupProblem, "no room for a shell") || row.SetupShell != "" || row.Start != "" || row.State == "settled" {
		t.Errorf("the row = %+v, want the thread running and the set-up failed with why", row)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestQuitDuringTheStartBringsTheThreadBackAsAFailedStart(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{SetupCommand: "touch ran-setup"})
	repoFixture(t, h)
	tmux := newFakeTmux(t)
	tmux.fail(t, "slow-new", true)
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "and the signup form"); err != nil {
		t.Fatal(err)
	}
	var placed threads.Thread
	eventually(t, "the start to reach the agent", func() bool {
		placed, _ = h.Thread(s.ID)
		return placed.Worktree != "" && placed.Detail == "Starting Claude"
	})
	h.BeginQuit()
	h.App.Stop()
	tmux.fail(t, "slow-new", false)

	second := h.Restart(t)
	row, ok := second.Thread(s.ID)
	if !ok || row.Start != threads.StartFailed || row.StartError != "The app quit while the thread started" || row.State != "waiting" {
		t.Fatalf("the row after the restart = %+v, %v; want a failed start", row, ok)
	}
	if row.FirstMessage != "make it faster" || len(row.StartQueue) != 1 || row.StartQueue[0] != "and the signup form" || row.Title != "Fix login" || row.N != s.N || row.Worktree != placed.Worktree || row.Branch != "fix-login" {
		t.Errorf("the row after the restart = %+v, want the messages, the number and the worktree of %+v", row, placed)
	}
	if tmuxHas(second, s.ID) {
		t.Error("a session of the thread runs")
	}
	if err := second.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := second.Thread(s.ID)
		return row.Start == ""
	})
	eventually(t, "both messages", func() bool { return len(userLines(t, second, s.ID)) >= 2 })
	users := userLines(t, second, s.ID)
	if len(users) != 2 || !strings.Contains(users[0], "make it faster") || !strings.Contains(users[1], "and the signup form") {
		t.Errorf("messages = %v", users)
	}
	row, _ = second.Thread(s.ID)
	if row.Worktree != placed.Worktree || strings.Count(gitIn(t, h.Dir, "worktree", "list"), "\n") != 1 {
		t.Errorf("the retried row = %+v, want the worktree %s and no second one: %s", row, placed.Worktree, gitIn(t, h.Dir, "worktree", "list"))
	}
	eventually(t, "the set-up command", func() bool { return exists(filepath.Join(placed.Worktree, "ran-setup")) })
	if left, _ := filepath.Glob(filepath.Join(second.State, "data", "main", "starts", "*")); len(left) != 0 {
		t.Errorf("the saved start is still there: %v", left)
	}
}

func TestRestartOfAnAgentThatNeverGotItsFirstMessageOffersTheRetry(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.Threads().Create(h.Threads().Current(), "work", "", false, 0, thread.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	// The record a quit left when the agent ran and had not taken the first message.
	token := strings.TrimPrefix(s.ID, "main/")
	saved := fmt.Sprintf(`{"name":%q,"number":%d,"rawTitle":"work","title":"work","text":"first words","send":true,"model":{},"access":"supervised","place":"project","created":%d,"placed":true,"launched":true,"delivered":false,"queue":[{"Text":"second words"}],"status":"starting"}`, s.ID, s.N, s.CreatedAt)
	dir := filepath.Join(h.State, "data", "main", "starts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, token+".json"), []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}
	h.BeginQuit()
	h.App.Stop()

	second := h.Restart(t)
	eventually(t, "the thread to be listed as a failed start", func() bool {
		row, _ := second.Thread(s.ID)
		return row.Start == threads.StartFailed
	})
	row, _ := second.Thread(s.ID)
	if row.FirstMessage != "first words" || len(row.StartQueue) != 1 || row.StartError != "The app quit while the thread started" || row.State != "waiting" {
		t.Errorf("the row = %+v", row)
	}
	if err := second.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "both messages", func() bool { return len(userLines(t, second, s.ID)) >= 2 })
	eventually(t, "the start to end", func() bool {
		row, _ := second.Thread(s.ID)
		return row.Start == "" && len(row.StartQueue) == 0
	})
	users := userLines(t, second, s.ID)
	if len(users) != 2 || !strings.Contains(users[0], "first words") || !strings.Contains(users[1], "second words") {
		t.Errorf("messages = %v", users)
	}
	if _, err := os.Stat(filepath.Join(dir, token+".json")); err == nil {
		t.Error("the saved start is still there")
	}
}

func cancelled(h *apptest.Harness, id string) bool {
	row, _ := h.Thread(id)
	return row.Start == threads.StartCancelled
}

func TestCancelStopsAStartThatWaitsForOriginAndRetryGoesOn(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 3*time.Second)
	s, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "and be quick"); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	if err := h.CancelThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > time.Second {
		t.Errorf("the cancel took %v, which the 3 s fetch holds up", took)
	}
	row, _ := h.Thread(s.ID)
	if row.Start != threads.StartCancelled || row.StartError != "Start cancelled" || row.State != "waiting" || row.FirstMessage == "" || len(row.StartQueue) != 1 {
		t.Errorf("the row = %+v, want a cancelled start with its messages kept", row)
	}
	if exists(filepath.Join(h.Dir, "trees", "issue-7")) || len(userLines(t, h, s.ID)) != 0 {
		t.Error("the cancelled start left a worktree or reached the agent")
	}
	if err := h.CancelThreadStart(s.ID); err == nil {
		t.Error("a start that is not under way was cancelled")
	}
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	eventually(t, "both messages", func() bool { return len(userLines(t, h, s.ID)) >= 2 })
	users := userLines(t, h, s.ID)
	if len(users) != 2 || !strings.Contains(users[0], "/ship 7") || !strings.Contains(users[1], "and be quick") {
		t.Errorf("messages = %v", users)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestCancelDuringTheAgentsStartRemovesTheWorktreeItMade(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	tmux := newFakeTmux(t)
	tmux.fail(t, "slow-new", true)
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "and the signup form"); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(h.Dir, "trees", "fix-login")
	eventually(t, "the start to reach the agent", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Worktree != "" && row.Detail == "Starting Claude"
	})
	if err := h.CancelThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	row, _ := h.Thread(s.ID)
	if row.Start != threads.StartCancelled || row.StartError != "Start cancelled" || row.Worktree != "" || row.FirstMessage != "make it faster" || len(row.StartQueue) != 1 {
		t.Errorf("the row = %+v, want a cancelled start with no worktree and its messages kept", row)
	}
	if exists(tree) || tmuxHas(h, s.ID) {
		t.Error("the cancelled start left its worktree or a session")
	}
	if got := gitIn(t, h.Dir, "branch", "--list", "fix-login"); got == "" {
		t.Error("the branch went with the worktree, which only the thread's own delete may take")
	}
	tmux.fail(t, "slow-new", false)
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	eventually(t, "both messages", func() bool { return len(userLines(t, h, s.ID)) >= 2 })
	row, _ = h.Thread(s.ID)
	users := userLines(t, h, s.ID)
	if len(users) != 2 || !strings.Contains(users[0], "make it faster") || !strings.Contains(users[1], "and the signup form") || row.Branch != "fix-login" || !exists(tree) {
		t.Errorf("messages = %v, row = %+v; want the messages once, in order, on the same branch", users, row)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestCancelKeepsAWorktreeWithChangesAndSaysWhy(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	tmux := newFakeTmux(t)
	tmux.fail(t, "slow-new", true)
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(h.Dir, "trees", "fix-login")
	eventually(t, "the start to reach the agent", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Worktree != "" && row.Detail == "Starting Claude"
	})
	if err := os.WriteFile(filepath.Join(tree, "notes.txt"), []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.CancelThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	row, _ := h.Thread(s.ID)
	if row.Start != threads.StartCancelled || !strings.HasPrefix(row.StartError, "Start cancelled. The worktree stays: ") || row.Worktree == "" || !exists(filepath.Join(tree, "notes.txt")) {
		t.Errorf("the row = %+v, want a cancelled start that says why the worktree stays", row)
	}
	tmux.fail(t, "slow-new", false)
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	if after, _ := h.Thread(s.ID); after.Worktree != row.Worktree || !exists(filepath.Join(tree, "notes.txt")) {
		t.Errorf("the retried row = %+v, want the worktree %s with the file", after, row.Worktree)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestCancelStopsTheSetupCommandAndRetryRunsItAgain(t *testing.T) {
	runs := filepath.Join(t.TempDir(), "runs")
	h := apptest.NewWith(t, apptest.Options{SetupCommand: "echo run >> " + runs + "; sleep 60"})
	repoFixture(t, h)
	tmux := newFakeTmux(t)
	tmux.fail(t, "slow-shell", true)
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	// The set-up command's terminal takes a second to open, with the agent running already: the start is still under way.
	eventually(t, "the agent to run", func() bool { ch, _ := h.Threads().ChatOf(s.ID); return ch.Live })
	if err := h.CancelThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	row, _ := h.Thread(s.ID)
	if row.Start != threads.StartCancelled || row.Setup != "" || len(h.Threads().Shells()) != 0 {
		t.Errorf("the row = %+v, shells = %+v; want the set-up stopped and its terminal closed", row, h.Threads().Shells())
	}
	if !exists(row.Worktree) {
		t.Error("the worktree of a thread whose agent runs went")
	}
	tmux.fail(t, "slow-shell", false)
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread and its set-up", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == "" && row.Setup == threads.SetupRunning
	})
	if shells := h.Threads().Shells(); len(shells) != 1 {
		t.Errorf("shells = %+v, want the one terminal of the set-up that runs again", shells)
	}
	eventually(t, "the set-up command's own mark", func() bool { return strings.Contains(readFile(runs), "run") })
	if n := mentions(userLines(t, h, s.ID), "make it faster"); n != 1 {
		t.Errorf("the first message reached the agent %d times, want once", n)
	}
}

func readFile(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

func notesOf(t *testing.T, h *apptest.Harness, id string) []string {
	t.Helper()
	chatLog, _ := hostFiles(h, id)
	events, err := chatlog.ReadLog(chatLog, 0, chatlog.KindNote)
	if err != nil {
		t.Fatal(err)
	}
	var notes []string
	for _, e := range events {
		notes = append(notes, e.Text)
	}
	return notes
}

func TestIssueStartFallsBackToANewBranchFromTheLocalMainWhenOriginIsOutOfReach(t *testing.T) {
	t.Parallel()
	for name, url := range map[string]string{
		"a remote that cannot be reached":    "file:///nowhere/at/all.git",
		"a host that refuses the connection": "ssh://localhost:1/refused.git",
	} {
		t.Run(name, func(t *testing.T) {
			h := apptest.New(t)
			repoFixture(t, h)
			head := gitIn(t, h.Dir, "rev-parse", "main")
			gitIn(t, h.Dir, "remote", "set-url", "origin", url)
			s, err := h.StartIssueWith(7, "")
			if err != nil {
				t.Fatalf("the start = %v, want it to go on from main as last fetched", err)
			}
			if s.Branch != "issue-7" || s.Worktree == "" || gitIn(t, s.Worktree, "rev-parse", "HEAD") != head {
				t.Errorf("the row = %+v, want the new branch issue-7 at the local main %s", s, head)
			}
			want := []string{"Could not reach origin; started from main as last fetched"}
			if got := notesOf(t, h, s.ID); !slices.Equal(got, want) {
				t.Errorf("notes in the chat = %q, want %q", got, want)
			}
		})
	}
}

func TestIssueStartAsksOriginWithoutANoteWhenItHasNoSuchBranchOrThereIsNoOrigin(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	repoFixture(t, h)
	s, err := h.StartIssueWith(7, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := notesOf(t, h, s.ID); len(got) != 0 || s.Branch != "issue-7" {
		t.Errorf("origin has no issue-7: notes = %q, branch = %q; want a new branch and no note", got, s.Branch)
	}
	gitIn(t, h.Dir, "remote", "remove", "origin")
	other, err := h.StartIssueWith(8, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := notesOf(t, h, other.ID); len(got) != 0 || other.Branch != "issue-8" {
		t.Errorf("no origin: notes = %q, branch = %q; want a new branch and no note", got, other.Branch)
	}
}

func TestTwoStartsOfTheSameTitleAtOnceGetDifferentBranches(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	repoFixture(t, h)
	const n = 3
	ids := make(chan string, n)
	for range n {
		go func() {
			s, err := h.NewThreadLater("Fix login", "make it faster")
			if err != nil {
				t.Error(err)
				ids <- ""
				return
			}
			ids <- s.ID
		}()
	}
	branches, trees := map[string]bool{}, map[string]bool{}
	for range n {
		id := <-ids
		if id == "" {
			continue
		}
		eventually(t, "the thread to start", func() bool {
			row, _ := h.Thread(id)
			return row.Start == ""
		})
		row, _ := h.Thread(id)
		branches[row.Branch], trees[row.Worktree] = true, true
		if !exists(row.Worktree) {
			t.Errorf("the worktree %s of %s is not there", row.Worktree, id)
		}
	}
	for _, want := range []string{"fix-login", "fix-login-2", "fix-login-3"} {
		if !branches[want] {
			t.Errorf("branches = %v, want %s among three different ones", branches, want)
		}
	}
	if len(trees) != n {
		t.Errorf("worktrees = %v, want %d different ones", trees, n)
	}
}

func TestStartInAProjectThatIsNoRepositoryRunsInTheProjectFolder(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to start", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	row, _ := h.Thread(s.ID)
	ch, err := h.Threads().ChatOf(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Worktree != "" || row.Branch != "" || exists(filepath.Join(h.Dir, "trees")) || realDir(t, ch.Dir) != realDir(t, h.Dir) {
		t.Errorf("the row = %+v, folder %q; want the project folder %q, no worktree and no branch", row, ch.Dir, h.Dir)
	}
	eventually(t, "the first message", func() bool { return mentions(userLines(t, h, s.ID), "make it faster") == 1 })
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestCancelWhenTheWorktreeIsGoneLetsTheRetryMakeItAgain(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	tmux := newFakeTmux(t)
	tmux.fail(t, "slow-new", true)
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(h.Dir, "trees", "fix-login")
	eventually(t, "the start to reach the agent", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Worktree != "" && row.Detail == "Starting Claude"
	})
	if err := os.RemoveAll(tree); err != nil {
		t.Fatal(err)
	}
	if err := h.CancelThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	if row, _ := h.Thread(s.ID); row.Start != threads.StartCancelled || row.StartError != "Start cancelled" || row.Worktree != "" {
		t.Errorf("the row = %+v, want a plain cancelled start", row)
	}
	tmux.fail(t, "slow-new", false)
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	if row, _ := h.Thread(s.ID); row.Branch != "fix-login" || !exists(tree) {
		t.Errorf("the retried row = %+v, want the worktree made again on fix-login", row)
	}
}

func TestStopDuringTheFetchOfAnIssueBranchNeverMakesALocalOneThatShadowsOrigins(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	origin := repoFixture(t, h)
	// Another clone pushed issue-7, which this clone has no copy of.
	clone := filepath.Join(t.TempDir(), "clone")
	gitIn(t, filepath.Dir(clone), "clone", origin, clone)
	gitIn(t, clone, "checkout", "-b", "issue-7")
	if err := os.WriteFile(filepath.Join(clone, "work.txt"), []byte("work\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, clone, "add", "work.txt")
	gitIn(t, clone, "commit", "-m", "work")
	gitIn(t, clone, "push", "origin", "issue-7")
	want := gitIn(t, clone, "rev-parse", "HEAD")
	slowOrigin(t, h, origin, 2*time.Second)

	s, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if err := h.CancelThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	if got := gitIn(t, h.Dir, "branch", "--list", "issue-7"); got != "" || exists(filepath.Join(h.Dir, "trees", "issue-7")) {
		t.Fatalf("the stop left the local branch %q or a worktree: it would shadow origin's", got)
	}
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	row, _ := h.Thread(s.ID)
	if head := gitIn(t, row.Worktree, "rev-parse", "HEAD"); head != want {
		t.Errorf("the worktree is at %s, want origin's issue-7, %s", head, want)
	}
}

// quitWithAStartThatWasMakingItsWorktree leaves what a quit does when it comes while git makes the worktree of a new
// thread: the worktree is there, no record names it, and the saved start says only what it was about to make, if making.
// It returns the app that starts again, the thread's id and the worktree.
func quitWithAStartThatWasMakingItsWorktree(t *testing.T, making bool) (*apptest.Harness, string, string) {
	t.Helper()
	h := apptest.New(t)
	repoFixture(t, h)
	// The repository ignores .env itself, so the file is no untracked change that keeps the tree.
	exclude, err := os.OpenFile(filepath.Join(h.Dir, ".git", "info", "exclude"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exclude.WriteString(".env\n"); err != nil {
		t.Fatal(err)
	}
	exclude.Close()
	tree := filepath.Join(h.Dir, "trees", "fix-login")
	gitIn(t, h.Dir, "worktree", "add", "-b", "fix-login", tree)
	if err := os.WriteFile(filepath.Join(tree, ".env"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	place := ""
	if making {
		place = realDir(t, tree)
	}
	const id = "main/aaaaaaaa"
	saved := fmt.Sprintf(`{"name":%q,"number":1,"rawTitle":"Fix login","title":"Fix login","text":"make it faster","send":true,"model":{},"access":"supervised","place":"worktree","created":%d,"branch":"fix-login","making":%q,"status":"starting"}`, id, time.Now().UnixMilli(), place)
	dir := filepath.Join(h.State, "data", "main", "starts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "aaaaaaaa.json"), []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}
	h.BeginQuit()
	h.App.Stop()
	return h.Restart(t), id, tree
}

func TestDeleteWithTheBoxTakesTheWorktreeAQuitLeftUnrecordedMidMake(t *testing.T) {
	t.Parallel()
	h, id, tree := quitWithAStartThatWasMakingItsWorktree(t, true)
	if row, ok := h.Thread(id); !ok || row.Start != threads.StartFailed {
		t.Fatalf("the row = %+v, %v; want the failed start", row, ok)
	}
	if err := h.Threads().DeleteAndWait(id, true); err != nil {
		t.Fatal(err)
	}
	if exists(tree) {
		t.Error("the worktree the start was making is still there after a delete with the box")
	}
}

func TestRetryAndDeleteLeaveAWorktreeNoStartSaidItWasMaking(t *testing.T) {
	t.Parallel()
	h, id, tree := quitWithAStartThatWasMakingItsWorktree(t, false)
	if err := h.RetryThreadStart(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retry to start the thread", func() bool {
		row, _ := h.Thread(id)
		return row.Start == ""
	})
	if records := madeRecords(t, h); len(records) != 0 {
		t.Errorf("records = %+v, want none: the worktree is not the start's", records)
	}
	if err := h.Threads().DeleteAndWait(id, true); err != nil {
		t.Logf("delete: %v", err)
	}
	if !exists(filepath.Join(tree, ".env")) {
		t.Error("the worktree and its ignored file went, which no start made")
	}
}

func TestRestartOfAnAgentThatTookTheFirstMessageShowsItsRealChat(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.Threads().Create(h.Threads().Current(), "work", "", false, 0, thread.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	// The host took the message, and the quit came before the app heard.
	if err := sendToHost(h, s.ID, "first words"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the message with the agent", func() bool { return mentions(userLines(t, h, s.ID), "first words") == 1 })
	token := strings.TrimPrefix(s.ID, "main/")
	saved := fmt.Sprintf(`{"name":%q,"number":%d,"rawTitle":"work","title":"work","text":"first words","send":true,"model":{},"access":"supervised","place":"project","created":%d,"placed":true,"launched":true,"delivered":false,"sending":"first","sentBefore":0,"queue":[],"status":"starting"}`, s.ID, s.N, s.CreatedAt)
	dir := filepath.Join(h.State, "data", "main", "starts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, token+".json"), []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}
	h.BeginQuit()
	h.App.Stop()

	second := h.Restart(t)
	if row, ok := second.Thread(s.ID); !ok || row.Start != "" || row.FirstMessage != "" {
		t.Errorf("the row = %+v, %v; want the thread running, with its real chat", row, ok)
	}
	if _, err := os.Stat(filepath.Join(dir, token+".json")); err == nil {
		t.Error("the saved start is still there")
	}
	if n := mentions(userLines(t, second, s.ID), "first words"); n != 1 {
		t.Errorf("the first message reached the agent %d times, want once", n)
	}
}

func TestStopEndsAStartThatWaitsForAnotherStartsWorktree(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	slowOrigin(t, h, repoFixture(t, h), 3*time.Second)
	first, err := h.StartIssueLater(7)
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.StartIssueLater(8)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	began := time.Now()
	if err := h.CancelThreadStart(second.ID); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > time.Second {
		t.Errorf("the stop took %v, which the start in front of it holds up", took)
	}
	if row, _ := h.Thread(second.ID); row.Start != threads.StartCancelled {
		t.Errorf("the row = %+v, want a cancelled start", row)
	}
	eventually(t, "the first start to end", func() bool {
		row, _ := h.Thread(first.ID)
		return row.Start == ""
	})
	if err := h.RetryThreadStart(second.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the retried start to end", func() bool {
		row, _ := h.Thread(second.ID)
		return row.Start == "" && row.Branch == "issue-8"
	})
}
