package threads_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nednella/orchestrator/desktop/threads"
)

func write(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestChangesCountCommittedAndUncommittedLines(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	s := threadWithoutIssue(h, t)
	gitIn(t, h.Dir, "checkout", "-b", "work")
	write(t, h.Dir, "a.txt", "1\n2\n3\n")
	gitIn(t, h.Dir, "add", "a.txt")
	gitIn(t, h.Dir, "commit", "-m", "a")
	write(t, h.Dir, "README", "changed\nmore\n")
	write(t, h.Dir, "untracked.txt", "not counted\n")

	got := h.Threads().Changes(s.ID)
	if want := (threads.Changes{Base: "origin/main", Added: 5, Removed: 1}); got != want {
		t.Errorf("changes = %+v, want %+v", got, want)
	}
}

func TestChangesLeaveOutWhatLandedOnTheDefaultBranchSinceTheBranchStarted(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	s := threadWithoutIssue(h, t)
	gitIn(t, h.Dir, "checkout", "-b", "work")
	write(t, h.Dir, "mine.txt", "mine\n")
	gitIn(t, h.Dir, "add", "mine.txt")
	gitIn(t, h.Dir, "commit", "-m", "mine")
	gitIn(t, h.Dir, "checkout", "main")
	write(t, h.Dir, "theirs.txt", "1\n2\n3\n4\n")
	gitIn(t, h.Dir, "add", "theirs.txt")
	gitIn(t, h.Dir, "commit", "-m", "theirs")
	gitIn(t, h.Dir, "push", "origin", "main")
	gitIn(t, h.Dir, "checkout", "work")

	got := h.Threads().Changes(s.ID)
	if want := (threads.Changes{Base: "origin/main", Added: 1}); got != want {
		t.Errorf("changes = %+v, want %+v", got, want)
	}
}

func TestChangesCountABinaryFileAsNoLines(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	s := threadWithoutIssue(h, t)
	gitIn(t, h.Dir, "checkout", "-b", "work")
	write(t, h.Dir, "blob.bin", "a\x00b\x00c")
	gitIn(t, h.Dir, "add", "blob.bin")
	gitIn(t, h.Dir, "commit", "-m", "blob")

	got := h.Threads().Changes(s.ID)
	if want := (threads.Changes{Base: "origin/main"}); got != want {
		t.Errorf("changes = %+v, want %+v", got, want)
	}
}

func TestChangesHaveNoBaseWithoutAnOrigin(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	gitIn(t, h.Dir, "init")
	commitIn(t, h.Dir, "a.txt")
	s := threadWithoutIssue(h, t)

	if got := h.Threads().Changes(s.ID); got != (threads.Changes{}) {
		t.Errorf("changes = %+v, want none", got)
	}
}

func TestChangesHaveNoBaseOutsideARepository(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s := threadWithoutIssue(h, t)

	if got := h.Threads().Changes(s.ID); got != (threads.Changes{}) {
		t.Errorf("changes = %+v, want none", got)
	}
}
