package sessions_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/internal/scoped"
	"github.com/nednella/agentos/desktop/sessions"
)

// shellIDs are the ids of the current project's shells in the snapshot.
func shellIDs(h *apptest.Harness) []string {
	var ids []string
	for _, sh := range h.Snapshot().Shells {
		ids = append(ids, sh.ID)
	}
	return ids
}

// newShell starts a shell of the thread, "" for one of the project, and returns its id.
func newShell(t *testing.T, h *apptest.Harness, thread string) string {
	t.Helper()
	shell, err := h.ShellNew(thread)
	if err != nil {
		t.Fatal(err)
	}
	return shell.ID
}

// shellOf is the current project's shell id as the snapshot gives it.
func shellOf(h *apptest.Harness, id string) (sessions.Shell, bool) {
	shells := h.Snapshot().Shells
	i := slices.IndexFunc(shells, func(sh sessions.Shell) bool { return sh.ID == id })
	if i < 0 {
		return sessions.Shell{}, false
	}
	return shells[i], true
}

// shellFolder is the folder the shell's pane is in now, with symlinks resolved.
func shellFolder(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{pane_current_path}").Output()
	if err != nil {
		t.Fatal(err)
	}
	path, _ := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	return path
}

func resolved(path string) string {
	r, _ := filepath.EvalSymlinks(path)
	return r
}

// typeIn types the line into the shell and presses Enter.
func typeIn(t *testing.T, h *apptest.Harness, id, line string) {
	t.Helper()
	if err := exec.Command("tmux", "-L", h.Socket, "send-keys", "-t", "="+id+":", line, "Enter").Run(); err != nil {
		t.Fatal(err)
	}
}

// pushedThread is the thread of issue 7, with pushed work in its worktree and idle there.
func pushedThread(t *testing.T, h *apptest.Harness) sessions.Session {
	t.Helper()
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	pushWork(t, s.Worktree)
	h.Hook(t, s.ID, "Stop", fmt.Sprintf(`{"cwd":%q}`, s.Worktree))
	eventually(t, "idle", func() bool { return stateOf(h, s.ID) == "idle" })
	return s
}

func TestProjectShellHasNoThread(t *testing.T) {
	h := shellHarness(t)
	id := newShell(t, h, "")
	if got, ok := shellOf(h, id); !ok || got.Thread != "" {
		t.Errorf("the project shell = %+v, %v; want no thread", got, ok)
	}
	if got := shellFolder(t, h, id); got != resolved(h.Dir) {
		t.Errorf("the project shell starts in %s, want the project folder %s", got, h.Dir)
	}
}

func TestThreadShellStartsInItsWorktree(t *testing.T) {
	h := shellHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	id := newShell(t, h, s.ID)
	if got, ok := shellOf(h, id); !ok || got.Thread != s.ID {
		t.Errorf("the thread's shell = %+v, %v; want thread %s", got, ok, s.ID)
	}
	eventually(t, "the shell in the worktree", func() bool { return shellFolder(t, h, id) == resolved(s.Worktree) })

	gitIn(t, h.Dir, "worktree", "remove", "--force", s.Worktree)
	again := newShell(t, h, s.ID)
	if got := shellFolder(t, h, again); got != resolved(h.Dir) {
		t.Errorf("a shell of a thread whose worktree is gone starts in %s, want the project folder %s", got, h.Dir)
	}
	if got, _ := shellOf(h, again); got.Thread != s.ID {
		t.Errorf("that shell = %+v, want thread %s", got, s.ID)
	}
}

func TestShellsAreReadAgainAfterARestart(t *testing.T) {
	h := shellHarness(t)
	thread, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	project := newShell(t, h, "")
	own := newShell(t, h, thread.ID)

	second := h.Restart(t)
	got := second.Snapshot().Shells
	want := []sessions.Shell{{ID: project, Thread: ""}, {ID: own, Thread: thread.ID}}
	if len(got) != len(want) {
		t.Fatalf("shells after a restart = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Thread != want[i].Thread {
			t.Errorf("shell %d after a restart = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestNewShellIsRefusedDuringItsThreadsCleanup(t *testing.T) {
	h := shellHarness(t)
	s := pushedThread(t, h)
	release, done := heldCleanup(t, h, s.ID)

	_, err := h.ShellNew(s.ID)
	if want := fmt.Sprintf("Thread %d is being cleaned up; open a shell when that ends", s.N); err == nil || err.Error() != want {
		t.Errorf("a shell of the thread = %v, want %q", err, want)
	}
	project := newShell(t, h, "")
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !cleanedUp(h, s.ID)() || exists(s.Worktree) {
		t.Errorf("the clean-up did not finish: worktree there %v", exists(s.Worktree))
	}
	if got := shellIDs(h); !slices.Equal(got, []string{project}) {
		t.Errorf("shells = %q, want only the project's %s", got, project)
	}
}

// commandHarness is a shellHarness whose login shell is bash, which names its own process as it is called; macOS's sh
// runs as bash.
func commandHarness(t *testing.T) *apptest.Harness {
	t.Helper()
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("HOME", t.TempDir())
	return newHarness(t)
}

// shellRuns waits until a refresh finds the shell running command, busy or not.
func shellRuns(t *testing.T, h *apptest.Harness, id, command string, busy bool) {
	t.Helper()
	eventually(t, fmt.Sprintf("the shell running %s", command), func() bool {
		h.Sessions().Refresh()
		got, ok := shellOf(h, id)
		return ok && got.Command == command && got.Busy == busy
	})
}

func TestShellReportsItsRunningCommand(t *testing.T) {
	h := commandHarness(t)
	id := newShell(t, h, "")
	shellRuns(t, h, id, "bash", false)
	typeIn(t, h, id, "sleep 30")
	shellRuns(t, h, id, "sleep", true)
	if err := exec.Command("tmux", "-L", h.Socket, "send-keys", "-t", "="+id+":", "C-c").Run(); err != nil {
		t.Fatal(err)
	}
	shellRuns(t, h, id, "bash", false)
}

func TestChangedCommandIsSentAgain(t *testing.T) {
	h := commandHarness(t)
	id := newShell(t, h, "")
	shellRuns(t, h, id, "bash", false)
	sent := func() []sessions.Shell {
		list, _ := h.Rec.Last("shells").(scoped.List[sessions.Shell])
		if list.Project != "main" {
			return nil
		}
		return list.Items
	}
	if got := sent(); len(got) != 1 || got[0].ID != id || got[0].Busy {
		t.Fatalf("the shells sent = %+v, want %s idle", got, id)
	}
	before := h.Rec.Count("shells")
	h.Sessions().Refresh()
	if got := h.Rec.Count("shells"); got != before {
		t.Errorf("a refresh with nothing changed sent the shells again (%d events, was %d)", got, before)
	}

	typeIn(t, h, id, "sleep 30")
	shellRuns(t, h, id, "sleep", true)
	if got := sent(); len(got) != 1 || got[0].Command != "sleep" || !got[0].Busy {
		t.Errorf("the shells sent once the command changed = %+v, want sleep, busy", got)
	}
}
