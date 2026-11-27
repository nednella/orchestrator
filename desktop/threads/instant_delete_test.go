package threads_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Not parallel: it puts a fake tmux on the process PATH.
func TestDeleteReturnsBeforeTheThreadIsStopped(t *testing.T) {
	h := newHarness(t)
	s, err := h.StartThread("work", "")
	if err != nil {
		t.Fatal(err)
	}
	chatLog, _ := hostFiles(h, s.ID)
	newFakeTmux(t).slowKill(t, "2")

	began := time.Now()
	if err := h.DeleteThreadAsApp(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(began); took > 800*time.Millisecond {
		t.Errorf("the call took %v, which the slow stop of the thread holds up", took)
	}
	if _, listed := h.Thread(s.ID); listed {
		t.Error("the row is still on the list when the call returns")
	}
	for _, row := range h.Rec.LastThreads() {
		if row.ID == s.ID {
			t.Errorf("the threads event still lists the thread: %+v", row)
		}
	}
	if !tmuxHas(h, s.ID) {
		t.Error("the thread was stopped before the call returned; the test cannot tell")
	}
	eventually(t, "the thread to be gone", func() bool { return !tmuxHas(h, s.ID) && !hasState(t, h, s.ID) && !exists(chatLog) })
	if err := h.DeleteThreadAsApp(s.ID, false); err == nil {
		t.Error("a second delete of the gone thread was taken")
	}
}

func TestRefusedDeleteBringsTheRowBack(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	pushWork(t, s.Worktree)
	if err := os.WriteFile(filepath.Join(s.Worktree, "notes.txt"), []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := h.DeleteThreadAsApp(s.ID, true); err != nil {
		t.Fatalf("the call = %v, want it to return and the refusal to come after", err)
	}
	eventually(t, "the refusal to be told", func() bool { return h.Rec.Count("delete:refused") == 1 })
	eventually(t, "the row to come back", func() bool {
		row, listed := h.Thread(s.ID)
		return listed && row.Cleanup == "blocked"
	})
	if !tmuxHas(h, s.ID) || !hasState(t, h, s.ID) || !exists(s.Worktree) {
		t.Errorf("the thread: running %v, state file %v, worktree %v; want it all as it was", tmuxHas(h, s.ID), hasState(t, h, s.ID), exists(s.Worktree))
	}
	// The thread can be deleted again once the work is gone from the worktree.
	if err := h.DeleteThread(s.ID, false); err != nil {
		t.Errorf("a delete without the worktree = %v", err)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestDeleteTheAppDidNotFinishLeavesTheThreadAsItWas(t *testing.T) {
	h := newHarness(t)
	s, err := h.StartThread("work", "")
	if err != nil {
		t.Fatal(err)
	}
	newFakeTmux(t).slowKill(t, "8")
	if err := h.DeleteThreadAsApp(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, listed := h.Thread(s.ID); listed {
		t.Fatal("the row is still listed while the delete runs")
	}
	// The app quits while the stop is still on its way.
	second := h.Restart(t)
	row, listed := second.Thread(s.ID)
	if !listed || row.State == "settled" || !hasState(t, h, s.ID) {
		t.Errorf("after the restart the thread is %+v, listed %v: want it as it was, running", row, listed)
	}
}

func TestDeleteWithTheWorktreeIsNotRefusedForTheThreadsOwnTurn(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, message, state string }{
		{"a reply that ends in a question", "question", "waiting"},
		{"a turn that runs", "tool wait", "working"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			repoFixture(t, h)
			s := issueThread(h, t, 7)
			if _, err := h.ChatSend(s.ID, tt.message); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the thread to be "+tt.state, func() bool { return stateOf(h, s.ID) == tt.state })

			if err := h.DeleteThread(s.ID, true); err != nil {
				t.Fatalf("Delete = %v, want the thread's own turn not to keep its worktree", err)
			}
			if _, listed := h.Thread(s.ID); listed || exists(s.Worktree) {
				t.Errorf("the thread is listed: %v, its worktree is there: %v", listed, exists(s.Worktree))
			}
		})
	}
}
