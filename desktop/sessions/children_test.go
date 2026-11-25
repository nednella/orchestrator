package sessions_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
)

// childOf starts a child of the live thread parent, with parent's access, working where says.
func childOf(t *testing.T, h *apptest.Harness, parent sessions.Session, title, where string) sessions.Session {
	t.Helper()
	c, err := h.Sessions().CreateChild(context.Background(), sessions.ChildOptions{Parent: parent.ID, Task: "Do the work", Title: title, Access: parent.Access, Where: where})
	if err != nil {
		t.Fatal(err)
	}
	if c.Parent != parent.ID {
		t.Fatalf("the child = %+v, want parent %s", c, parent.ID)
	}
	return c
}

// putToSleep puts the thread to sleep and waits until its row says so.
func putToSleep(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	if err := h.SettleThread(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread asleep", func() bool { return stateOf(h, id) == "settled" })
}

// cleanupOf waits until the thread's clean-up has ended with an entry in the log, and returns the entry.
func cleanupOf(t *testing.T, h *apptest.Harness, s sessions.Session) sessions.Cleanup {
	t.Helper()
	var entry sessions.Cleanup
	eventually(t, "the clean-up of "+s.Title, func() bool {
		log := h.Cleanups()
		i := slices.IndexFunc(log, func(c sessions.Cleanup) bool { return c.SessionTitle == s.Title })
		if i >= 0 {
			entry = log[i]
		}
		got, _ := h.Session(s.ID)
		return i >= 0 && got.Cleanup != "pending"
	})
	return entry
}

// hasBranch says whether the repo at dir has the local branch.
func hasBranch(t *testing.T, dir, branch string) bool {
	t.Helper()
	return gitIn(t, dir, "branch", "--list", branch) != ""
}

func TestDeleteIsRefusedWhileAGrandchildRuns(t *testing.T) {
	h := apptest.New(t)
	parent := talked(t, h)
	child := childOf(t, h, parent, "Helper", sessions.WhereFolder)
	grandchild := childOf(t, h, child, "Checker", sessions.WhereFolder)
	putToSleep(t, h, child.ID)

	want := fmt.Sprintf("Thread %d has a running child, Checker; stop it first", parent.N)
	for _, worktree := range []bool{false, true} {
		if err := h.DeleteThread(parent.ID, worktree); err == nil || err.Error() != want {
			t.Errorf("Delete with the worktree %v = %v, want %q", worktree, err, want)
		}
	}
	for _, id := range []string{parent.ID, child.ID, grandchild.ID} {
		if _, ok := h.Session(id); !ok || !hasState(t, h, id) {
			t.Errorf("thread %s: row there %v, state file there %v", id, ok, hasState(t, h, id))
		}
	}
	if !tmuxHas(h, parent.ID) || !tmuxHas(h, grandchild.ID) {
		t.Errorf("running: the parent %v, the grandchild %v", tmuxHas(h, parent.ID), tmuxHas(h, grandchild.ID))
	}
}

func TestDeleteTakesAsleepChildrenWithIt(t *testing.T) {
	// The parent works in its issue's worktree, a child in a worktree of its own, and a grandchild in the child's.
	family := func(t *testing.T) (h *apptest.Harness, threads []sessions.Session) {
		h = newHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		pushWork(t, parent.Worktree)
		child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
		if child.Worktree == "" || child.Branch == "" {
			t.Fatalf("the child has no worktree: %+v", child)
		}
		grandchild := childOf(t, h, child, "Checker", sessions.WhereFolder)
		putToSleep(t, h, grandchild.ID)
		putToSleep(t, h, child.ID)
		if got := cleanupOf(t, h, child); got.Status != "blocked" || !exists(child.Worktree) {
			t.Fatalf("the child's clean-up at its sleep = %+v, worktree there %v; want it kept", got, exists(child.Worktree))
		}
		return h, []sessions.Session{parent, child, grandchild}
	}
	gone := func(t *testing.T, h *apptest.Harness, threads []sessions.Session) {
		t.Helper()
		for _, s := range threads {
			chatLog, _ := hostFiles(h, s.ID)
			if _, ok := h.Session(s.ID); ok || tmuxHas(h, s.ID) || hasState(t, h, s.ID) || exists(chatLog) {
				t.Errorf("thread %s: row there %v, running %v, state file there %v, chat log there %v", s.Title, ok, tmuxHas(h, s.ID), hasState(t, h, s.ID), exists(chatLog))
			}
		}
		if records := madeRecords(t, h); len(records) != 0 {
			t.Errorf("records = %+v, want every thread's gone", records)
		}
	}

	t.Run("with the worktrees", func(t *testing.T) {
		h, threads := family(t)
		parent, child := threads[0], threads[1]
		if got := h.OwnWorktree(parent.ID); got != parent.Worktree {
			t.Errorf("OwnWorktree of the parent = %q, want %s", got, parent.Worktree)
		}
		if got := h.OwnWorktree(child.ID); got != child.Worktree {
			t.Errorf("OwnWorktree of the child, whose child works in it = %q, want %s", got, child.Worktree)
		}
		if err := h.DeleteThread(parent.ID, true); err != nil {
			t.Fatal(err)
		}
		gone(t, h, threads)
		if exists(parent.Worktree) || exists(child.Worktree) {
			t.Errorf("worktrees there: the parent's %v, the child's %v", exists(parent.Worktree), exists(child.Worktree))
		}
		if !hasBranch(t, h.Dir, "issue-7") || !hasBranch(t, h.Dir, child.Branch) {
			t.Errorf("branches: %s", gitIn(t, h.Dir, "branch", "--list"))
		}
	})

	t.Run("without the worktrees", func(t *testing.T) {
		h, threads := family(t)
		parent, child := threads[0], threads[1]
		if err := h.DeleteThread(parent.ID, false); err != nil {
			t.Fatal(err)
		}
		gone(t, h, threads)
		if !exists(parent.Worktree) || !exists(child.Worktree) {
			t.Errorf("worktrees there: the parent's %v, the child's %v; want both", exists(parent.Worktree), exists(child.Worktree))
		}
	})
}

func TestDeleteStopsAtAChildsKeptWorktree(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	pushWork(t, parent.Worktree)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	notes := filepath.Join(child.Worktree, "notes.txt")
	if err := os.WriteFile(notes, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	putToSleep(t, h, child.ID)
	cleanupOf(t, h, child)

	err := h.DeleteThread(parent.ID, true)
	if want := fmt.Sprintf("Thread %d keeps its worktree: the worktree of %s has uncommitted changes. Delete it without the worktree, or clean it up first", child.N, child.Branch); err == nil || err.Error() != want {
		t.Errorf("Delete = %v, want %q", err, want)
	}
	if got, ok := h.Session(parent.ID); !ok || got.State == "settled" || !tmuxHas(h, parent.ID) || !exists(parent.Worktree) {
		t.Errorf("the parent = %+v, there %v; want it running in its worktree", got, ok)
	}
	if got, ok := h.Session(child.ID); !ok || got.State != "settled" || !exists(notes) {
		t.Errorf("the child = %+v, there %v, notes there %v", got, ok, exists(notes))
	}
}

func TestSleepingParentsChildGoesOn(t *testing.T) {
	h := apptest.New(t)
	parent := talked(t, h)
	child := childOf(t, h, parent, "Helper", sessions.WhereFolder)

	putToSleep(t, h, parent.ID)
	time.Sleep(300 * time.Millisecond)
	if !tmuxHas(h, child.ID) || stateOf(h, child.ID) == "settled" {
		t.Errorf("the child: running %v, state %s; want it running", tmuxHas(h, child.ID), stateOf(h, child.ID))
	}
	if got, _ := h.Session(child.ID); got.Parent != parent.ID {
		t.Errorf("the child's parent = %q, want %s", got.Parent, parent.ID)
	}
}

func TestEmptyChildWorktreeGoesWhenItSleeps(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	wt := child.Worktree
	if wt == "" || !exists(wt) {
		t.Fatalf("the child has no worktree: %+v", child)
	}

	putToSleep(t, h, child.ID)
	entry := cleanupOf(t, h, child)
	if entry.Status != "done" || !slices.Equal(entry.Removed, []string{"worktree " + wt}) {
		t.Errorf("the clean-up = %+v, want the worktree removed", entry)
	}
	if exists(wt) || worktreesOf(t, h.Dir)[child.Branch] != "" {
		t.Errorf("the worktree is still there: on disk %v, worktrees %v", exists(wt), worktreesOf(t, h.Dir))
	}
	if !hasBranch(t, h.Dir, child.Branch) {
		t.Errorf("%s is gone", child.Branch)
	}
	if got, ok := h.Session(child.ID); !ok || got.State != "settled" || got.Cleanup != "" {
		t.Errorf("the child = %+v, there %v; want it asleep", got, ok)
	}
	if !exists(parent.Worktree) {
		t.Error("the parent's worktree went")
	}

	if _, err := h.WakeThread(child.ID); err != nil {
		t.Fatal(err)
	}
	if !exists(wt) || realDir(t, panePath(t, h, child.ID)) != realDir(t, wt) {
		t.Errorf("the woken child: worktree there %v, runs in %s", exists(wt), panePath(t, h, child.ID))
	}
}

func TestChildWorktreeWithCommitsStays(t *testing.T) {
	for _, c := range []struct {
		name, reason string
		pushed       bool
	}{
		{"pushed", "%s has a commit that origin/main lacks", true},
		{"not pushed", "%s has a commit that is not on origin", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			repoFixture(t, h)
			parent := issueThread(h, t, 7)
			child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
			commitIn(t, child.Worktree, "work.txt")
			if c.pushed {
				gitIn(t, child.Worktree, "push", "-u", "origin", "HEAD")
			}

			putToSleep(t, h, child.ID)
			entry := cleanupOf(t, h, child)
			if want := fmt.Sprintf(c.reason, child.Branch); entry.Status != "blocked" || entry.Reason != want || len(entry.Removed) != 0 {
				t.Errorf("the clean-up = %+v, want it kept because %q", entry, want)
			}
			if !exists(filepath.Join(child.Worktree, "work.txt")) {
				t.Error("the worktree went")
			}
			if got, _ := h.Session(child.ID); got.Cleanup != "" {
				t.Errorf("the child's clean-up = %q, want it left to the rules of its pull request", got.Cleanup)
			}
		})
	}
}

func TestChildInTheParentsFolderRemovesNothing(t *testing.T) {
	ran := filepath.Join(t.TempDir(), "ran")
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: noteCleanups(ran)})
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Helper", sessions.WhereFolder)
	if child.Worktree != "" {
		t.Fatalf("the child has a worktree: %+v", child)
	}

	putToSleep(t, h, child.ID)
	time.Sleep(500 * time.Millisecond)
	if runs, log := cleanupRuns(ran), h.Cleanups(); len(runs) != 0 || len(log) != 0 {
		t.Errorf("clean-up runs %q, log %+v; want none", runs, log)
	}
	if !exists(parent.Worktree) || !tmuxHas(h, parent.ID) {
		t.Errorf("the parent: worktree there %v, running %v", exists(parent.Worktree), tmuxHas(h, parent.ID))
	}
}

func TestChildWorktreeUsedByAForkStays(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	withConversation(t, h, child.ID)
	writeChat(t, h, child.ID, oneReply...)
	fork, err := h.ForkSession(child.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	putToSleep(t, h, fork.ID)

	putToSleep(t, h, child.ID)
	entry := cleanupOf(t, h, child)
	if want := fmt.Sprintf("Thread %s also works in this worktree", fork.Title); entry.Status != "blocked" || entry.Reason != want {
		t.Errorf("the clean-up = %+v, want it kept because %q", entry, want)
	}
	if !exists(child.Worktree) {
		t.Error("the worktree went")
	}
}

func TestDeleteWithoutTheWorktreeKeepsAnEmptyChildWorktree(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)

	if err := h.DeleteThread(child.ID, false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if _, ok := h.Session(child.ID); ok || !exists(child.Worktree) {
		t.Errorf("row there %v, worktree there %v; want the row gone and the worktree left", ok, exists(child.Worktree))
	}
	if log := h.Cleanups(); len(log) != 0 {
		t.Errorf("log = %+v, want none", log)
	}
}
