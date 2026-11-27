package threads_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/bus"
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
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "sleep 30"})
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
	if exists(filepath.Join(h.Dir, "trees", "tidy-up")) || len(userLines(t, h, s.ID)) != 0 {
		t.Error("the failed start left a worktree or reached the agent")
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
	if row.Branch != "tidy-up" || row.Worktree == "" || row.StartError != "" {
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
	if err := h.Threads().Delete(s.ID, true); err != nil {
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
	if err := h.Threads().Delete(s.ID, true); err != nil {
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
	if exists(filepath.Join(h.Dir, "trees", "tidy-up")) {
		t.Error("the worktree of the failed start is still there")
	}
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
