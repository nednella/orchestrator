package sessions_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/session"
)

func TestFilesRewindOnlyInTheThreadsOwnWorktree(t *testing.T) {
	h := shellHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	if dir, inProject, reason := h.Sessions().FilesRewind(s.ID); !samePath(dir, wt) || inProject || reason != "" {
		t.Fatalf("FilesRewind = %q, %v, %q; want the thread's worktree", dir, inProject, reason)
	}

	reader, err := h.Sessions().CreateThread("look around", sessions.StartOptions{Access: session.Read, Place: sessions.InProject})
	if err != nil {
		t.Fatal(err)
	}
	if dir, inProject, reason := h.Sessions().FilesRewind(reader.ID); dir != "" || !inProject || reason != "" {
		t.Errorf("a thread in the project folder: FilesRewind = %q, %v, %q", dir, inProject, reason)
	}

	withConversation(t, h, s.ID)
	writeChat(t, h, s.ID, oneReply...)
	fork, err := h.ForkSession(s.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, reason := h.Sessions().FilesRewind(s.ID); reason != fmt.Sprintf("Thread %s also works in this worktree", fork.Title) {
		t.Errorf("with a live fork in the worktree: reason = %q", reason)
	}
	if dir, inProject, reason := h.Sessions().FilesRewind(fork.ID); dir != "" || inProject || reason != "Files are rewound only in a worktree agentos made for this thread" {
		t.Errorf("the fork: FilesRewind = %q, %v, %q", dir, inProject, reason)
	}
	if err := h.SettleThread(fork.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the fork settled", func() bool { return stateOf(h, fork.ID) == "settled" })
	if _, _, reason := h.Sessions().FilesRewind(s.ID); reason != fmt.Sprintf("Thread %s also works in this worktree", fork.Title) {
		t.Errorf("with the fork asleep: reason = %q", reason)
	}
	if err := h.DeleteThread(fork.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, _, reason := h.Sessions().FilesRewind(s.ID); reason != "" {
		t.Errorf("with the fork deleted: reason = %q", reason)
	}

	h.Hook(t, s.ID, "Stop", fmt.Sprintf(`{"cwd":%q}`, wt))
	shell, err := h.ShellNew(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	realWt, _ := filepath.EvalSymlinks(wt)
	eventually(t, "the shell in the worktree", func() bool {
		out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+shell.ID+":", "#{pane_current_path}").Output()
		path, _ := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
		return err == nil && path == realWt
	})
	if _, _, reason := h.Sessions().FilesRewind(s.ID); reason != "a shell is open in this worktree" {
		t.Errorf("with a shell in the worktree: reason = %q", reason)
	}
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}
