package sessions_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/internal/scoped"
	"github.com/nednella/orchestrator/desktop/sessions"
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

// cdShell moves the shell into dir and waits until tmux sees it there.
func cdShell(t *testing.T, h *apptest.Harness, id, dir string) {
	t.Helper()
	typeIn(t, h, id, fmt.Sprintf("cd %q", dir))
	eventually(t, "the shell in "+dir, func() bool { return shellFolder(t, h, id) == resolved(dir) })
}

// pushedThread is the thread of issue 7, with pushed work in its worktree and idle there.
func pushedThread(t *testing.T, h *apptest.Harness) sessions.Session {
	t.Helper()
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	pushWork(t, s.Worktree)
	h.Observe(t, s.ID, "Stop", fmt.Sprintf(`{"cwd":%q}`, s.Worktree))
	eventually(t, "idle", func() bool { return stateOf(h, s.ID) == "idle" })
	return s
}

func TestShellFromNoThreadStartsInTheProjectFolder(t *testing.T) {
	t.Parallel()
	h := shellHarness(t)
	id := newShell(t, h, "")
	if got, ok := shellOf(h, id); !ok || resolved(got.Folder) != resolved(h.Dir) {
		t.Errorf("the project shell = %+v, %v; want the folder %s", got, ok, h.Dir)
	}
	if got := shellFolder(t, h, id); got != resolved(h.Dir) {
		t.Errorf("the project shell starts in %s, want the project folder %s", got, h.Dir)
	}
}

func TestShellFromAThreadStartsInItsWorktreeAndBelongsToTheProject(t *testing.T) {
	t.Parallel()
	h := shellHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	id := newShell(t, h, s.ID)
	if got, ok := shellOf(h, id); !ok || resolved(got.Folder) != resolved(s.Worktree) {
		t.Errorf("the shell = %+v, %v; want the folder %s", got, ok, s.Worktree)
	}
	eventually(t, "the shell in the worktree", func() bool { return shellFolder(t, h, id) == resolved(s.Worktree) })

	gitIn(t, h.Dir, "worktree", "remove", "--force", s.Worktree)
	again := newShell(t, h, s.ID)
	if got := shellFolder(t, h, again); got != resolved(h.Dir) {
		t.Errorf("a shell from a thread whose worktree is gone starts in %s, want the project folder %s", got, h.Dir)
	}
	if got, ok := shellOf(h, again); !ok || resolved(got.Folder) != resolved(h.Dir) {
		t.Errorf("that shell = %+v, %v; want the folder %s", got, ok, h.Dir)
	}
}

func TestShellsAreReadAgainAfterARestart(t *testing.T) {
	t.Parallel()
	h := shellHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	project := newShell(t, h, "")
	own := newShell(t, h, s.ID)

	got := h.Restart(t).Snapshot().Shells
	want := []sessions.Shell{{ID: project, Folder: h.Dir}, {ID: own, Folder: s.Worktree}}
	if len(got) != len(want) {
		t.Fatalf("shells after a restart = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].ID != want[i].ID || resolved(got[i].Folder) != resolved(want[i].Folder) {
			t.Errorf("shell %d after a restart = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDeleteKeepsEveryShell(t *testing.T) {
	t.Parallel()
	t.Run("without the worktree", func(t *testing.T) {
		h := shellHarness(t)
		gone, err := h.NewSession("gone", "")
		if err != nil {
			t.Fatal(err)
		}
		project := newShell(t, h, "")
		fromGone := []string{newShell(t, h, gone.ID), newShell(t, h, gone.ID)}

		if err := h.DeleteThread(gone.ID, false); err != nil {
			t.Fatal(err)
		}
		left := append([]string{project}, fromGone...)
		for _, id := range left {
			if !tmuxHas(h, id) {
				t.Errorf("the shell %s went with the thread", id)
			}
		}
		if got := shellIDs(h); !slices.Equal(got, left) {
			t.Errorf("shells after the delete = %q, want %q", got, left)
		}
	})

	t.Run("a shell in the worktree keeps it", func(t *testing.T) {
		h := shellHarness(t)
		s := pushedThread(t, h)
		own := newShell(t, h, s.ID)
		eventually(t, "the shell in the worktree", func() bool { return shellFolder(t, h, own) == resolved(s.Worktree) })

		if err := h.DeleteThread(s.ID, true); err == nil {
			t.Fatal("the delete went through with a shell open in the worktree")
		}
		if !exists(s.Worktree) || !tmuxHas(h, own) {
			t.Errorf("a refused delete: worktree there %v, shell runs %v; want both", exists(s.Worktree), tmuxHas(h, own))
		}

		if err := h.ShellClose(own); err != nil {
			t.Fatal(err)
		}
		project := newShell(t, h, "")
		if err := h.DeleteThread(s.ID, true); err != nil {
			t.Fatal(err)
		}
		if exists(s.Worktree) || !tmuxHas(h, project) {
			t.Errorf("worktree there %v, project shell runs %v; want it gone and the shell kept", exists(s.Worktree), tmuxHas(h, project))
		}
	})

	t.Run("a refused delete", func(t *testing.T) {
		h := shellHarness(t)
		s := pushedThread(t, h)
		own := newShell(t, h, s.ID)
		if err := os.WriteFile(filepath.Join(s.Worktree, "unsaved.txt"), []byte("work\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := h.DeleteThread(s.ID, true); err == nil {
			t.Fatal("the delete went through with changes in the worktree")
		}
		if !tmuxHas(h, own) {
			t.Error("a refused delete closed the shell")
		}
	})
}

func TestCleanupWaitsForAShellInTheWorktreeAndKeepsEveryShell(t *testing.T) {
	t.Parallel()
	h := shellHarness(t)
	s := pushedThread(t, h)
	own := newShell(t, h, s.ID)
	eventually(t, "the shell in the worktree", func() bool { return shellFolder(t, h, own) == resolved(s.Worktree) })
	project := newShell(t, h, "")

	openThenMerge(h)
	keptFor(t, h, s.ID, s.Worktree, "a shell is open in this worktree")
	if !tmuxHas(h, own) || !tmuxHas(h, project) {
		t.Error("a kept clean-up closed a shell")
	}

	cdShell(t, h, own, h.Dir)
	if err := h.Cleanup(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if !cleanedUp(h, s.ID)() || exists(s.Worktree) {
		t.Errorf("the clean-up once the shell left: cleaned up %v, worktree there %v", cleanedUp(h, s.ID)(), exists(s.Worktree))
	}
	if got := shellIDs(h); !slices.Equal(got, []string{own, project}) {
		t.Errorf("shells after the clean-up = %q, want %q", got, []string{own, project})
	}
}

func TestNewShellIsRefusedDuringItsThreadsCleanup(t *testing.T) {
	t.Parallel()
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
		t.Errorf("shells = %q, want only %s", got, project)
	}
}

// commandHarness is an app whose shells run sh, which macOS names bash once it runs: a shell is busy by its terminal,
// never by its name.
func commandHarness(t *testing.T) *apptest.Harness {
	t.Helper()
	return apptest.NewWith(t, apptest.Options{Env: []string{"SHELL=/bin/sh", "HOME=" + t.TempDir()}})
}

// shellRuns waits until a refresh finds the shell running command, busy or not. An empty command is the shell itself.
func shellRuns(t *testing.T, h *apptest.Harness, id, command string, busy bool) {
	t.Helper()
	eventually(t, fmt.Sprintf("the shell running %q", command), func() bool {
		h.Sessions().Refresh()
		got, ok := shellOf(h, id)
		return ok && got.Busy == busy && (got.Command == command || command == "" && got.Command != "sleep")
	})
}

func TestShellReportsItsRunningCommand(t *testing.T) {
	t.Parallel()
	h := commandHarness(t)
	id := newShell(t, h, "")
	shellRuns(t, h, id, "", false)
	typeIn(t, h, id, "sleep 30")
	shellRuns(t, h, id, "sleep", true)
	if err := exec.Command("tmux", "-L", h.Socket, "send-keys", "-t", "="+id+":", "C-c").Run(); err != nil {
		t.Fatal(err)
	}
	shellRuns(t, h, id, "", false)
}

func TestChangedCommandIsSentAgain(t *testing.T) {
	t.Parallel()
	h := commandHarness(t)
	id := newShell(t, h, "")
	shellRuns(t, h, id, "", false)
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
