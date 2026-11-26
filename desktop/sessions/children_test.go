package sessions_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
)

// childOf starts a child of the live thread parent, with parent's access, working where says. Its turn stays open, so
// it never goes to sleep by itself once its result reaches its parent: only a test's putToSleep or sleepsByItself puts
// it to sleep.
func childOf(t *testing.T, h *apptest.Harness, parent sessions.Session, title, where string) sessions.Session {
	t.Helper()
	c, err := h.Sessions().CreateChild(context.Background(), sessions.ChildOptions{Parent: parent.ID, Task: "keep working", Title: title, Access: parent.Access, Where: where})
	if err != nil {
		t.Fatal(err)
	}
	if c.Parent != parent.ID {
		t.Fatalf("the child = %+v, want parent %s", c, parent.ID)
	}
	return c
}

// putToSleep puts the thread to sleep as the owner does and waits until its row says so. No sleep of its own races it:
// a child of childOf keeps its turn open, and a thread with no parent has no result to give.
func putToSleep(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	if err := h.SettleThread(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread asleep", func() bool { return stateOf(h, id) == "settled" })
}

// sleepsByItself ends the child's open turn, so its result goes to its parent and the child goes to sleep by itself, and
// waits until its row and its log say so. It is the child's first sleep.
func sleepsByItself(t *testing.T, h *apptest.Harness, id string) {
	t.Helper()
	if err := h.Interrupt(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the child asleep by itself", func() bool { return stateOf(h, id) == "settled" })
	assertExits(t, h, id, chat.Event{Kind: chat.KindExit, Stop: chat.StopSettle})
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

	t.Run("without the worktree, children in its folder", func(t *testing.T) {
		h := newHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		child := childOf(t, h, parent, "Helper", sessions.WhereFolder)
		putToSleep(t, h, child.ID)
		if got := h.DeletePlan(parent.ID); got.Worktrees != 1 {
			t.Errorf("DeletePlan = %+v, want one worktree", got)
		}
		if err := h.DeleteThread(parent.ID, false); err != nil {
			t.Fatal(err)
		}
		gone(t, h, []sessions.Session{parent, child})
		if !exists(parent.Worktree) {
			t.Error("the parent's worktree went")
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
	if want := fmt.Sprintf("Thread %d keeps its worktree: the worktree of %s has uncommitted changes. Untick the box to keep the worktrees, or clean Thread %d up first", child.N, child.Branch, child.N); err == nil || err.Error() != want {
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
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	commitIn(t, child.Worktree, "work.txt")

	putToSleep(t, h, child.ID)
	entry := cleanupOf(t, h, child)
	if want := child.Branch + " has a commit that is not on origin"; entry.Status != "blocked" || entry.Reason != want || len(entry.Removed) != 0 {
		t.Errorf("the clean-up = %+v, want it kept because %q", entry, want)
	}
	if !exists(filepath.Join(child.Worktree, "work.txt")) {
		t.Error("the worktree went")
	}
	if got, _ := h.Session(child.ID); got.Cleanup != "" {
		t.Errorf("the child's clean-up = %q, want it left to the rules of its pull request", got.Cleanup)
	}
}

func TestChildWorktreeWithPushedCommitsGoes(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	commitIn(t, child.Worktree, "work.txt")
	gitIn(t, child.Worktree, "push", "-u", "origin", "HEAD")

	putToSleep(t, h, child.ID)
	if entry := cleanupOf(t, h, child); entry.Status != "done" || exists(child.Worktree) {
		t.Errorf("the clean-up = %+v, worktree there %v; want it removed", entry, exists(child.Worktree))
	}
	if !hasBranch(t, h.Dir, child.Branch) {
		t.Errorf("%s is gone", child.Branch)
	}
}

func TestChildOnItsParentsUnpushedWorkGoesWithoutAFetch(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	commitIn(t, parent.Worktree, "parent.txt")
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	if !exists(filepath.Join(child.Worktree, "parent.txt")) {
		t.Fatal("the child does not start at its parent's commit")
	}
	fetched := filepath.Join(t.TempDir(), "fetched")
	gitIn(t, h.Dir, "config", "remote.origin.uploadpack", fmt.Sprintf("touch %s; git-upload-pack", fetched))

	putToSleep(t, h, child.ID)
	if entry := cleanupOf(t, h, child); entry.Status != "done" || exists(child.Worktree) {
		t.Errorf("the clean-up = %+v, worktree there %v; want it removed", entry, exists(child.Worktree))
	}
	if exists(fetched) {
		t.Error("the clean-up fetched origin for a child with no commit of its own")
	}
}

func TestDeleteTakesAChildWorktreeOnItsParentsUnpushedWork(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	commitIn(t, h.Dir, "parent.txt")
	if _, err := h.SetThreadPlace("project"); err != nil {
		t.Fatal(err)
	}
	parent, err := h.Sessions().Create("Look around", "", false, 0, session.Read)
	if err != nil {
		t.Fatal(err)
	}
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	if !exists(filepath.Join(child.Worktree, "parent.txt")) {
		t.Fatal("the child does not start at its parent's commit")
	}
	// A grandchild asleep in the child's worktree keeps it at the child's sleep.
	checker := childOf(t, h, child, "Checker", sessions.WhereFolder)
	putToSleep(t, h, checker.ID)
	putToSleep(t, h, child.ID)
	cleanupOf(t, h, child)

	if err := h.DeleteThread(parent.ID, true); err != nil {
		t.Fatal(err)
	}
	if exists(child.Worktree) || !hasBranch(t, h.Dir, child.Branch) {
		t.Errorf("the child's worktree there %v, its branch there %v; want only the branch", exists(child.Worktree), hasBranch(t, h.Dir, child.Branch))
	}
	if !exists(filepath.Join(h.Dir, "parent.txt")) {
		t.Error("the project folder lost the parent's commit")
	}
}

func TestChildWorktreeWithChangesStays(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	notes := filepath.Join(child.Worktree, "notes.txt")
	if err := os.WriteFile(notes, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	putToSleep(t, h, child.ID)
	if want := fmt.Sprintf("the worktree of %s has uncommitted changes", child.Branch); cleanupOf(t, h, child).Reason != want || !exists(notes) {
		t.Errorf("the clean-up = %+v, notes there %v; want it kept because %q", cleanupOf(t, h, child), exists(notes), want)
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

// childWithShell is a child of the thread of issue 7 in its own empty worktree, with a shell of its own open there.
func childWithShell(t *testing.T, h *apptest.Harness) (child sessions.Session, shell string) {
	t.Helper()
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child = childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	shell = newShell(t, h, child.ID)
	eventually(t, "the shell in the worktree", func() bool { return shellFolder(t, h, shell) == resolved(child.Worktree) })
	return child, shell
}

func TestChildSleepKeepsItsWorktreeWhileItsShellIsOpen(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
	}{
		{name: "idle"},
		{name: "busy", command: "sleep"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := shellHarness(t)
			child, shell := childWithShell(t, h)
			if tc.command != "" {
				typeIn(t, h, shell, tc.command+" 60")
			}
			shellRuns(t, h, shell, tc.command, tc.command != "")

			sleepsByItself(t, h, child.ID)
			if entry := cleanupOf(t, h, child); entry.Status != "blocked" || entry.Reason != "a shell of this thread is open" || len(entry.Removed) != 0 {
				t.Errorf("the clean-up = %+v, want it kept for the child's shell", entry)
			}
			if !exists(child.Worktree) || !tmuxHas(h, shell) {
				t.Fatalf("worktree there %v, the child's shell runs %v; want both", exists(child.Worktree), tmuxHas(h, shell))
			}
			shellRuns(t, h, shell, tc.command, tc.command != "")

			if err := h.ShellClose(shell); err != nil {
				t.Fatal(err)
			}
			if _, err := h.WakeThread(child.ID); err != nil {
				t.Fatal(err)
			}
			putToSleep(t, h, child.ID)
			eventually(t, "the worktree gone at the next sleep", func() bool { return !exists(child.Worktree) })
		})
	}
}

func TestChildWithNoShellLosesItsEmptyWorktree(t *testing.T) {
	h := shellHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	others := []string{newShell(t, h, parent.ID), newShell(t, h, "")}

	sleepsByItself(t, h, child.ID)
	if entry := cleanupOf(t, h, child); entry.Status != "done" || !slices.Equal(entry.Removed, []string{"worktree " + child.Worktree}) {
		t.Errorf("the clean-up = %+v, want the worktree removed", entry)
	}
	if exists(child.Worktree) {
		t.Error("the worktree is still there")
	}
	for _, id := range others {
		if !tmuxHas(h, id) {
			t.Errorf("the shell %s went with the child's worktree", id)
		}
	}
}

func TestDeleteClosesTheShellsOfAChildItsShellKept(t *testing.T) {
	for _, withWorktree := range []bool{false, true} {
		t.Run(fmt.Sprintf("worktree %v", withWorktree), func(t *testing.T) {
			h := shellHarness(t)
			child, shell := childWithShell(t, h)
			putToSleep(t, h, child.ID)
			if entry := cleanupOf(t, h, child); entry.Status != "blocked" {
				t.Fatalf("the clean-up = %+v, want it kept for the child's shell", entry)
			}

			if err := h.DeleteThread(child.ID, withWorktree); err != nil {
				t.Fatal(err)
			}
			if tmuxHas(h, shell) {
				t.Error("the deleted child's shell still runs")
			}
			if exists(child.Worktree) != !withWorktree {
				t.Errorf("worktree there %v after a delete with the worktree %v", exists(child.Worktree), withWorktree)
			}
		})
	}
}

func TestDeleteGivesAChildsWorktreeToAForkOfIt(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	pushWork(t, parent.Worktree)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	withConversation(t, h, child.ID)
	writeChat(t, h, child.ID, oneReply...)
	fork, err := h.ForkSession(child.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	putToSleep(t, h, fork.ID)
	putToSleep(t, h, child.ID)
	cleanupOf(t, h, child)

	if err := h.DeleteThread(parent.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(child.ID); ok || !exists(child.Worktree) || exists(parent.Worktree) {
		t.Fatalf("the child's row there %v, its worktree there %v, the parent's there %v; want only the child's worktree", ok, exists(child.Worktree), exists(parent.Worktree))
	}
	if got := h.OwnWorktree(fork.ID); got != child.Worktree {
		t.Errorf("OwnWorktree of the fork = %q, want the child's %s", got, child.Worktree)
	}
	if err := h.DeleteThread(fork.ID, true); err != nil {
		t.Fatal(err)
	}
	if exists(child.Worktree) || !hasBranch(t, h.Dir, child.Branch) {
		t.Errorf("the worktree there %v, its branch there %v; want only the branch", exists(child.Worktree), hasBranch(t, h.Dir, child.Branch))
	}
}

// recordsOf are the paths of the worktree records of the thread id.
func recordsOf(t *testing.T, h *apptest.Harness, id string) []string {
	t.Helper()
	var paths []string
	for _, w := range madeRecords(t, h) {
		if w.Thread == id {
			paths = append(paths, w.Path)
		}
	}
	return paths
}

func TestDeleteGivesAChildsWorktreeOnlyToAForkWithNoneOfItsOwn(t *testing.T) {
	t.Run("a thread with a worktree of its own", func(t *testing.T) {
		h := newHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		pushWork(t, parent.Worktree)
		child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
		other := issueThread(h, t, 8)
		h.Hook(t, other.ID, "PreToolUse", fmt.Sprintf(`{"cwd":%q}`, child.Worktree))
		eventually(t, "the other thread seen in the child's worktree", func() bool { return h.OwnWorktree(child.ID) == "" })
		putToSleep(t, h, child.ID)
		cleanupOf(t, h, child)

		if err := h.DeleteThread(parent.ID, false); err != nil {
			t.Fatal(err)
		}
		if got := recordsOf(t, h, other.ID); !slices.Equal(got, []string{other.Worktree}) {
			t.Errorf("the other thread's records = %q, want only its own %s", got, other.Worktree)
		}
		if got := h.OwnWorktree(other.ID); got != other.Worktree {
			t.Errorf("OwnWorktree of the other thread = %q, want %s", got, other.Worktree)
		}
		if !exists(child.Worktree) {
			t.Error("the child's worktree went")
		}
	})

	t.Run("a fork with a worktree of its own", func(t *testing.T) {
		h := newHarness(t)
		repoFixture(t, h)
		first := issueThread(h, t, 7)
		pushWork(t, first.Worktree)
		builder := childOf(t, h, first, "Builder", sessions.WhereWorktree)
		withConversation(t, h, builder.ID)
		writeChat(t, h, builder.ID, oneReply...)
		fork, err := h.ForkSession(builder.ID, "r1b")
		if err != nil {
			t.Fatal(err)
		}
		putToSleep(t, h, builder.ID)
		cleanupOf(t, h, builder)
		if err := h.DeleteThread(first.ID, false); err != nil {
			t.Fatal(err)
		}
		if got := recordsOf(t, h, fork.ID); !slices.Equal(got, []string{builder.Worktree}) {
			t.Fatalf("the fork's records = %q, want the builder's %s", got, builder.Worktree)
		}

		second := issueThread(h, t, 8)
		pushWork(t, second.Worktree)
		tester := childOf(t, h, second, "Tester", sessions.WhereWorktree)
		h.Hook(t, fork.ID, "PreToolUse", fmt.Sprintf(`{"cwd":%q}`, tester.Worktree))
		eventually(t, "the fork seen in the tester's worktree", func() bool { return h.OwnWorktree(tester.ID) == "" })
		putToSleep(t, h, tester.ID)
		cleanupOf(t, h, tester)
		if err := h.DeleteThread(second.ID, false); err != nil {
			t.Fatal(err)
		}
		if got := recordsOf(t, h, fork.ID); !slices.Equal(got, []string{builder.Worktree}) {
			t.Errorf("the fork's records = %q, want only the builder's %s", got, builder.Worktree)
		}
		if got := h.OwnWorktree(fork.ID); got != builder.Worktree {
			t.Errorf("OwnWorktree of the fork = %q, want %s", got, builder.Worktree)
		}
	})
}

func TestFamilyDeleteRefusesAChildWorktreeSharedOutsideTheFamily(t *testing.T) {
	t.Run("a thread", func(t *testing.T) {
		h := newHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		pushWork(t, parent.Worktree)
		child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
		other := issueThread(h, t, 8)
		h.Hook(t, other.ID, "PreToolUse", fmt.Sprintf(`{"cwd":%q}`, child.Worktree))
		eventually(t, "the other thread seen in the child's worktree", func() bool { return h.OwnWorktree(child.ID) == "" })
		putToSleep(t, h, child.ID)
		cleanupOf(t, h, child)
		if got := h.DeletePlan(parent.ID).Worktrees; got != 2 {
			t.Errorf("the box offers %d worktrees, want the 2 the refusal counts", got)
		}

		err := h.DeleteThread(parent.ID, true)
		if want := fmt.Sprintf("Thread %d keeps its worktree: Thread %s also works in this worktree. Untick the box to keep the worktrees, or clean Thread %d up first", child.N, other.Title, child.N); err == nil || err.Error() != want {
			t.Errorf("Delete = %v, want %q", err, want)
		}
		for _, s := range []sessions.Session{parent, child} {
			if _, ok := h.Session(s.ID); !ok || !exists(s.Worktree) {
				t.Errorf("thread %s: row there %v, worktree there %v; want both", s.Title, ok, exists(s.Worktree))
			}
		}
	})

	t.Run("a shell", func(t *testing.T) {
		h := shellHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		pushWork(t, parent.Worktree)
		child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
		h.Hook(t, child.ID, "PreToolUse", fmt.Sprintf(`{"cwd":%q}`, child.Worktree))
		shell := newShell(t, h, "")
		t.Cleanup(func() { _ = h.ShellClose(shell) })
		cdShell(t, h, shell, child.Worktree)
		eventually(t, "the shell in the child's worktree", func() bool { return h.OwnWorktree(child.ID) == "" })
		putToSleep(t, h, child.ID)
		cleanupOf(t, h, child)

		err := h.DeleteThread(parent.ID, true)
		if want := fmt.Sprintf("Thread %d keeps its worktree: a shell is open in this worktree. Untick the box to keep the worktrees, or clean Thread %d up first", child.N, child.N); err == nil || err.Error() != want {
			t.Errorf("Delete = %v, want %q", err, want)
		}
		if _, ok := h.Session(child.ID); !ok || !exists(child.Worktree) || !exists(parent.Worktree) {
			t.Errorf("the child's row there %v, its worktree there %v, the parent's there %v; want all", ok, exists(child.Worktree), exists(parent.Worktree))
		}
	})
}

func TestDeletePlanCountsAWorktreeAnAncestorWorksIn(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	pushWork(t, parent.Worktree)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	h.Hook(t, parent.ID, "Stop", fmt.Sprintf(`{"cwd":%q}`, child.Worktree))
	eventually(t, "the parent idle in the child's worktree", func() bool { return stateOf(h, parent.ID) == "idle" && h.OwnWorktree(child.ID) == "" })
	putToSleep(t, h, child.ID)
	cleanupOf(t, h, child)

	if got := h.DeletePlan(parent.ID).Worktrees; got != 2 {
		t.Errorf("the box offers %d worktrees, want both", got)
	}
	if err := h.DeleteThread(parent.ID, true); err != nil {
		t.Fatal(err)
	}
	if exists(parent.Worktree) || exists(child.Worktree) {
		t.Errorf("worktrees there: the parent's %v, the child's %v; want neither", exists(parent.Worktree), exists(child.Worktree))
	}
}

func TestDeletePlanOffersNoBoxWhenTheThreadsOwnWorktreeIsShared(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	commitIn(t, child.Worktree, "work.txt")
	putToSleep(t, h, child.ID)
	cleanupOf(t, h, child)
	other := issueThread(h, t, 8)
	h.Hook(t, other.ID, "Stop", fmt.Sprintf(`{"cwd":%q}`, parent.Worktree))
	eventually(t, "the other thread seen in the parent's worktree", func() bool { return h.OwnWorktree(parent.ID) == "" })

	if !exists(child.Worktree) {
		t.Fatal("the child's worktree went at its sleep")
	}
	if got := h.DeletePlan(parent.ID).Worktrees; got != 0 {
		t.Errorf("the box offers %d worktrees, want none", got)
	}
}

func TestDeleteGivesAChildsWorktreeToItsNewestFork(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	for issue := 7; issue < 10; issue++ {
		parent := issueThread(h, t, issue)
		pushWork(t, parent.Worktree)
		child := childOf(t, h, parent, fmt.Sprintf("Builder %d", issue), sessions.WhereWorktree)
		withConversation(t, h, child.ID)
		writeChat(t, h, child.ID, oneReply...)
		var forks []sessions.Session
		for range 2 {
			fork, err := h.ForkSession(child.ID, "r1b")
			if err != nil {
				t.Fatal(err)
			}
			forks = append(forks, fork)
		}
		for _, f := range forks {
			putToSleep(t, h, f.ID)
		}
		putToSleep(t, h, child.ID)
		cleanupOf(t, h, child)

		if err := h.DeleteThread(parent.ID, false); err != nil {
			t.Fatal(err)
		}
		if older, newer := recordsOf(t, h, forks[0].ID), recordsOf(t, h, forks[1].ID); len(older) != 0 || !slices.Equal(newer, []string{child.Worktree}) {
			t.Errorf("#%d: the older fork's records %q, the newer's %q; want the child's worktree for the newer only", issue, older, newer)
		}
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

// holdGitDirs holds the app's reads of the git folders of the worktree wt, the nth at holds[n-1]: each signals its
// reached channel and waits until its release channel closes. Reads past the last hold run at once.
func holdGitDirs(t *testing.T, h *apptest.Harness, wt string, holds int) (reached, release []chan struct{}) {
	t.Helper()
	for range holds {
		reached, release = append(reached, make(chan struct{})), append(release, make(chan struct{}))
	}
	want := realDir(t, wt)
	var mu sync.Mutex
	n := 0
	h.GH.SetBefore(func(dir, name string, args []string) {
		if name != "git" || !slices.Contains(args, "--git-common-dir") {
			return
		}
		if real, err := filepath.EvalSymlinks(dir); err != nil || real != want {
			return
		}
		mu.Lock()
		i := n
		n++
		mu.Unlock()
		if i < holds {
			close(reached[i])
			<-release[i]
		}
	})
	t.Cleanup(func() { h.GH.SetBefore(nil) })
	return reached, release
}

func TestWakeDuringAChildsSleepKeepsItsWorktree(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	wt := child.Worktree
	// The first read is the clean-up's, as it starts; the second the wake's, as it puts the worktree back.
	reached, release := holdGitDirs(t, h, wt, 2)

	putToSleep(t, h, child.ID)
	<-reached[0]
	woke := make(chan error, 1)
	go func() { _, err := h.WakeThread(child.ID); woke <- err }()
	<-reached[1]
	close(release[0])
	time.Sleep(500 * time.Millisecond) // a clean-up that does not wait for the wake removes the worktree here
	close(release[1])

	if err := <-woke; err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if !exists(wt) || stateOf(h, child.ID) == "settled" || realDir(t, panePath(t, h, child.ID)) != realDir(t, wt) {
		t.Errorf("the woken child: worktree there %v, state %s, runs in %s", exists(wt), stateOf(h, child.ID), panePath(t, h, child.ID))
	}
	if log := h.Cleanups(); len(log) != 0 {
		t.Errorf("log = %+v, want no clean-up of a woken child", log)
	}
}

func TestWakeWaitsForAChildsSleepCleanup(t *testing.T) {
	hold := t.TempDir()
	reached, released := filepath.Join(hold, "reached"), filepath.Join(hold, "release")
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: fmt.Sprintf("touch %s; while [ ! -e %s ]; do sleep 0.05; done; %s", reached, released, project.DefaultCleanupCommand)})
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	wt := child.Worktree

	putToSleep(t, h, child.ID)
	eventually(t, "the clean-up held in its command", func() bool { return exists(reached) })
	_, err := h.WakeThread(child.ID)
	if want := fmt.Sprintf("Thread %d is being cleaned up; wake it when that ends", child.N); err == nil || err.Error() != want {
		t.Errorf("wake = %v, want %q", err, want)
	}
	if err := os.WriteFile(released, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if entry := cleanupOf(t, h, child); entry.Status != "done" || exists(wt) {
		t.Fatalf("the clean-up = %+v, worktree there %v", entry, exists(wt))
	}
	if _, err := h.WakeThread(child.ID); err != nil {
		t.Fatal(err)
	}
	if !exists(wt) {
		t.Error("the wake did not put the worktree back")
	}
}

func TestChildKeptAgainForTheSameReasonLogsNothing(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	child := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	if err := os.WriteFile(filepath.Join(child.Worktree, "notes.txt"), []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Each clean-up reads the worktree's changes once it has marked itself running; a wake waits for it to end.
	checked := make(chan struct{}, 8)
	h.GH.SetBefore(func(dir, name string, args []string) {
		if name == "git" && slices.Contains(args, "--porcelain") && slices.Contains(args, child.Worktree) {
			checked <- struct{}{}
		}
	})
	t.Cleanup(func() { h.GH.SetBefore(nil) })
	sleepAndWake := func() {
		t.Helper()
		putToSleep(t, h, child.ID)
		<-checked
		eventually(t, "the clean-up's end", func() bool { got, _ := h.Session(child.ID); return got.Cleanup != "pending" })
		if _, err := h.WakeThread(child.ID); err != nil {
			t.Fatal(err)
		}
	}

	sleepAndWake()
	sleepAndWake()
	if log, events := h.Cleanups(), h.Events(t, activity.Cleanup); len(log) != 1 || len(events) != 1 {
		t.Errorf("after a second sleep kept for the same reason: log %+v, events %+v; want one of each", log, events)
	}

	gitIn(t, child.Worktree, "add", "notes.txt")
	gitIn(t, child.Worktree, "commit", "-m", "notes")
	putToSleep(t, h, child.ID)
	eventually(t, "the clean-up kept for the new reason", func() bool { return len(h.Cleanups()) == 2 })
	if want := child.Branch + " has a commit that is not on origin"; h.Cleanups()[0].Reason != want {
		t.Errorf("the newest entry = %+v, want %q", h.Cleanups()[0], want)
	}
	if events := h.Events(t, activity.Cleanup); len(events) != 2 {
		t.Errorf("events = %+v, want two", events)
	}
}

func TestFamilyDeleteRemovesNoWorktreeUnlessEveryOneMayGo(t *testing.T) {
	// freeChild is a child whose worktree its sleep kept, for a commit origin lacked, and which may go now it is pushed.
	freeChild := func(t *testing.T, h *apptest.Harness, parent sessions.Session) sessions.Session {
		t.Helper()
		c := childOf(t, h, parent, "Pusher", sessions.WhereWorktree)
		commitIn(t, c.Worktree, "pushed.txt")
		putToSleep(t, h, c.ID)
		cleanupOf(t, h, c)
		gitIn(t, c.Worktree, "push", "-u", "origin", "HEAD")
		return c
	}

	t.Run("a child keeps its worktree", func(t *testing.T) {
		h := newHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		pushWork(t, parent.Worktree)
		kept := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
		if err := os.WriteFile(filepath.Join(kept.Worktree, "notes.txt"), []byte("mine\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		putToSleep(t, h, kept.ID)
		cleanupOf(t, h, kept)
		free := freeChild(t, h, parent)

		err := h.DeleteThread(parent.ID, true)
		if want := fmt.Sprintf("Thread %d keeps its worktree: the worktree of %s has uncommitted changes. Untick the box to keep the worktrees, or clean Thread %d up first", kept.N, kept.Branch, kept.N); err == nil || err.Error() != want {
			t.Errorf("Delete = %v, want %q", err, want)
		}
		for _, s := range []sessions.Session{parent, kept, free} {
			if _, ok := h.Session(s.ID); !ok || !exists(s.Worktree) {
				t.Errorf("thread %s: row there %v, worktree there %v; want both", s.Title, ok, exists(s.Worktree))
			}
		}
		if !tmuxHas(h, parent.ID) {
			t.Error("the parent was put to sleep")
		}
	})

	t.Run("the parent changes its worktree as it stops", func(t *testing.T) {
		h := newHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		pushWork(t, parent.Worktree)
		free := freeChild(t, h, parent)
		late := filepath.Join(parent.Worktree, "late.txt")
		// A fetch once the parent has stopped finds a file its agent wrote as it stopped.
		gitIn(t, h.Dir, "config", "remote.origin.uploadpack", fmt.Sprintf("if ! tmux -L %s has-session -t =%s 2>/dev/null; then echo late > %s; fi; git-upload-pack", h.Socket, parent.ID, late))

		err := h.DeleteThread(parent.ID, true)
		if want := fmt.Sprintf("Thread %d keeps its worktree: the worktree of issue-7 has uncommitted changes. It is asleep now. Untick the box to keep the worktrees, or clean Thread %d up first", parent.N, parent.N); err == nil || err.Error() != want {
			t.Errorf("Delete = %v, want %q", err, want)
		}
		if !exists(late) || !exists(free.Worktree) || !exists(parent.Worktree) {
			t.Errorf("the late file there %v, the child's worktree there %v, the parent's there %v; want all", exists(late), exists(free.Worktree), exists(parent.Worktree))
		}
		if got, ok := h.Session(parent.ID); !ok || got.State != "settled" {
			t.Errorf("the parent = %+v, there %v; want it asleep", got, ok)
		}
	})
}

func TestDeleteWithoutTheWorktreesKeepsEveryFolderOfTheFamily(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	if _, err := h.SetThreadPlace("project"); err != nil {
		t.Fatal(err)
	}
	parent, err := h.Sessions().Create("Look around", "", false, 0, session.Read)
	if err != nil {
		t.Fatal(err)
	}
	if parent.Worktree != "" {
		t.Fatalf("the parent has a worktree: %+v", parent)
	}
	unpushed := childOf(t, h, parent, "Builder", sessions.WhereWorktree)
	commitIn(t, unpushed.Worktree, "work.txt")
	checker := childOf(t, h, unpushed, "Checker", sessions.WhereFolder)
	putToSleep(t, h, checker.ID)
	putToSleep(t, h, unpushed.ID)
	cleanupOf(t, h, unpushed)
	family := []sessions.Session{parent, unpushed, checker}
	if got := h.DeletePlan(parent.ID); got.Worktrees != 1 {
		t.Errorf("DeletePlan of the parent = %+v, want one worktree", got)
	}

	err = h.DeleteThread(parent.ID, true)
	if want := fmt.Sprintf("Thread %d keeps its worktree: %s has a commit that is not on origin. Untick the box to keep the worktree, or clean Thread %d up first", unpushed.N, unpushed.Branch, unpushed.N); err == nil || err.Error() != want {
		t.Errorf("Delete with the worktrees = %v, want %q", err, want)
	}
	for _, s := range family {
		if _, ok := h.Session(s.ID); !ok {
			t.Errorf("thread %s went", s.Title)
		}
	}

	if err := h.DeleteThread(parent.ID, false); err != nil {
		t.Fatal(err)
	}
	for _, s := range family {
		if _, ok := h.Session(s.ID); ok {
			t.Errorf("thread %s is still there", s.Title)
		}
	}
	if !exists(filepath.Join(unpushed.Worktree, "work.txt")) || !hasBranch(t, h.Dir, unpushed.Branch) {
		t.Errorf("the child's worktree there %v, branch there %v; want both", exists(unpushed.Worktree), hasBranch(t, h.Dir, unpushed.Branch))
	}
	if records := madeRecords(t, h); len(records) != 0 {
		t.Errorf("records = %+v, want every thread's gone", records)
	}
	if !exists(filepath.Join(h.Dir, "README")) {
		t.Error("the project folder lost its files")
	}
}

func TestDeletePlanNamesTheAsleepDescendants(t *testing.T) {
	h := apptest.New(t)
	parent := talked(t, h)
	if got := h.DeletePlan(parent.ID).Children; got == nil || len(got) != 0 {
		t.Errorf("Children of a thread with none = %#v, want []", got)
	}
	helper := childOf(t, h, parent, "Helper", sessions.WhereFolder)
	checker := childOf(t, h, helper, "Checker", sessions.WhereFolder)
	childOf(t, h, parent, "Runner", sessions.WhereFolder)
	putToSleep(t, h, checker.ID)
	putToSleep(t, h, helper.ID)

	if got, want := h.DeletePlan(parent.ID).Children, []string{"Helper", "Checker"}; !slices.Equal(got, want) {
		t.Errorf("Children = %q, want the asleep ones, nearest first: %q", got, want)
	}
}

func TestFamilyDeleteThatFailsAfterTheStopSaysTheThreadIsAsleep(t *testing.T) {
	// The clean-up command, which runs once the delete has put the parent to sleep, turns the child's state file into a
	// folder that holds a file, so the child's removal after it fails.
	broken := filepath.Join(t.TempDir(), "break")
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: fmt.Sprintf(`f=$(cat %q); rm -f "$f"; mkdir -p "$f/x"; %s`, broken, project.DefaultCleanupCommand)})
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	pushWork(t, parent.Worktree)
	child := childOf(t, h, parent, "Checker", sessions.WhereFolder)
	putToSleep(t, h, child.ID)
	if err := os.WriteFile(broken, []byte(filepath.Join(h.State, "sessions", child.ID+".json")), 0o600); err != nil {
		t.Fatal(err)
	}

	err := h.DeleteThread(parent.ID, true)
	if want := fmt.Sprintf(". Thread %d is asleep now", parent.N); err == nil || !strings.HasSuffix(err.Error(), want) {
		t.Errorf("Delete = %v, want an error that ends %q", err, want)
	}
	if got, ok := h.Session(parent.ID); !ok || got.State != "settled" {
		t.Errorf("the parent = %+v, there %v; want it asleep", got, ok)
	}
}
