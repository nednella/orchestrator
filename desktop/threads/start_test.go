package threads_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/bus"
)

const addWorktree = "git worktree add -b {branch} {worktree} {base}"

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

func TestNewThreadIsListedBeforeItsWorktreeIsMade(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "sleep 3 && " + addWorktree})
	repoFixture(t, h)

	began := time.Now()
	s, err := h.NewThreadLater("Fix login", "make it faster")
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > time.Second {
		t.Errorf("the call took %v, which the worktree command holds up", took)
	}
	if s.Start != threads.Starting || s.State != "working" || s.FirstMessage != "make it faster" || s.Title != "Fix login" {
		t.Errorf("the row = %+v, want it starting with the first message", s)
	}
	if !listedStarting(h, s.ID) {
		t.Errorf("the threads event does not list %s as starting: %+v", s.ID, h.Rec.LastThreads())
	}
	if exists(filepath.Join(h.Dir, "trees", "fix-login")) {
		t.Error("the worktree was made before the call returned")
	}
	if got, err := h.NewThreadLater("Fix login", "again"); err != nil || got.ID == s.ID {
		t.Errorf("a second start = %+v, %v", got, err)
	}

	eventually(t, "the thread to start", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == ""
	})
	row, _ := h.Thread(s.ID)
	if row.Worktree == "" || !exists(row.Worktree) || row.FirstMessage != "" || row.State == "waiting" {
		t.Errorf("the started row = %+v", row)
	}
	eventually(t, "the first message on the agent's stdin", func() bool { return mentions(userLines(t, h, s.ID), "make it faster") >= 1 })
	time.Sleep(300 * time.Millisecond)
	if n := mentions(userLines(t, h, s.ID), "make it faster"); n != 1 {
		t.Errorf("the first message reached the agent %d times, want once", n)
	}
	if row.Branch != "fix-login" {
		t.Errorf("branch = %q", row.Branch)
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

func TestFailedStartShowsOnTheThreadAndRetries(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), "failed-once")
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "if [ -e " + marker + " ]; then " + addWorktree + "; else touch " + marker + "; echo no room on the disk >&2; exit 3; fi"})
	repoFixture(t, h)

	s, err := h.NewThreadLater("Tidy up", "clean the tests")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the start to fail", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Start == threads.StartFailed
	})
	row, _ := h.Thread(s.ID)
	if !strings.Contains(row.StartError, "no room on the disk") || row.FirstMessage != "clean the tests" || row.State != "waiting" || row.Detail != row.StartError {
		t.Errorf("the failed row = %+v", row)
	}
	if exists(filepath.Join(h.Dir, "trees", "tidy-up")) || len(userLines(t, h, s.ID)) != 0 {
		t.Error("the failed start left a worktree or reached the agent")
	}
	if err := h.RetryThreadStart(s.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.RetryThreadStart(s.ID); err == nil {
		t.Error("a retry of a start that runs was taken")
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

func TestFailedStartIsDeleted(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "exit 3"})
	repoFixture(t, h)
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
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "sleep 3 && " + addWorktree})
	repoFixture(t, h)
	s, err := h.NewThreadLater("Changed my mind", "oops")
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
	if exists(filepath.Join(h.Dir, "trees", "changed-my-mind")) {
		t.Error("the worktree of a deleted start is there")
	}
}

func TestIssueThreadIsNotStartedTwiceWhileItStarts(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "sleep 2 && " + addWorktree})
	repoFixture(t, h)

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
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "sleep 2 && " + addWorktree})
	repoFixture(t, h)
	s, err := h.NewThreadLater("First name", "hello")
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
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "sleep 2 && " + addWorktree})
	repoFixture(t, h)
	s, err := h.NewThreadLater("Fix login", "hello")
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
	if rec.Branch != "fix-login" || rec.Path != row.Worktree || rec.Title != "Fix login" {
		t.Errorf("the saved identity = %+v, want the branch and folder of %+v", rec.Identity, row)
	}
}

func TestThreadThatStartsCannotBeSlept(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "sleep 2 && " + addWorktree})
	repoFixture(t, h)
	s, err := h.NewThreadLater("Fix login", "hello")
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
