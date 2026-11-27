package threads_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/thread"
)

func TestFilesRewindOnlyInTheThreadsOwnWorktree(t *testing.T) {
	t.Parallel()
	h := shellHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	if dir, inProject, reason := h.Threads().FilesRewind(s.ID); !samePath(dir, wt) || inProject || reason != "" {
		t.Fatalf("FilesRewind = %q, %v, %q; want the thread's worktree", dir, inProject, reason)
	}

	reader, err := h.Threads().CreateThread("look around", threads.StartOptions{Access: thread.Read, Place: threads.InProject})
	if err != nil {
		t.Fatal(err)
	}
	if dir, inProject, reason := h.Threads().FilesRewind(reader.ID); dir != "" || !inProject || reason != "" {
		t.Errorf("a thread in the project folder: FilesRewind = %q, %v, %q", dir, inProject, reason)
	}

	withConversation(t, h, s.ID)
	writeChat(t, h, s.ID, oneReply...)
	fork, err := h.ForkThread(s.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, reason := h.Threads().FilesRewind(s.ID); reason != fmt.Sprintf("Thread %s also works in this worktree", fork.Title) {
		t.Errorf("with a live fork in the worktree: reason = %q", reason)
	}
	if dir, inProject, reason := h.Threads().FilesRewind(fork.ID); dir != "" || inProject || reason != "Files are rewound only in a worktree orchestrator made for this thread" {
		t.Errorf("the fork: FilesRewind = %q, %v, %q", dir, inProject, reason)
	}
	if err := h.SettleThread(fork.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fork settled", func() bool { return stateOf(h, fork.ID) == "settled" })
	if _, _, reason := h.Threads().FilesRewind(s.ID); reason != fmt.Sprintf("Thread %s also works in this worktree", fork.Title) {
		t.Errorf("with the fork asleep: reason = %q", reason)
	}
	if err := h.DeleteThread(fork.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, _, reason := h.Threads().FilesRewind(s.ID); reason != "" {
		t.Errorf("with the fork deleted: reason = %q", reason)
	}

	h.Observe(t, s.ID, "Stop", fmt.Sprintf(`{"cwd":%q}`, wt))
	own, err := h.ShellNew(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread's shell in the worktree", func() bool { return shellFolder(t, h, own.ID) == resolved(wt) })
	if _, _, reason := h.Threads().FilesRewind(s.ID); reason != "" {
		t.Errorf("with an idle shell in the worktree: reason = %q", reason)
	}
	typeIn(t, h, own.ID, "sleep 30")
	eventually(t, "the shell's command keeps the files", func() bool {
		_, _, reason := h.Threads().FilesRewind(s.ID)
		return reason == "a command is running in a terminal in this worktree"
	})
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}
