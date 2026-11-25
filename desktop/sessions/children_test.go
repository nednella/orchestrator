package sessions_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
