package gittest

import (
	"path/filepath"
	"testing"
)

// Not parallel: it sets the process environment, as a git hook and git rebase --exec do.
func TestRunActsOnItsFolderWhateverGitDirSays(t *testing.T) {
	outer := t.TempDir()
	Run(t, outer, "init")
	Run(t, outer, "commit", "--allow-empty", "-m", "first")
	t.Cleanup(Unchanged(t, outer))
	t.Setenv("GIT_DIR", filepath.Join(outer, ".git"))
	t.Setenv("GIT_WORK_TREE", outer)

	dir, err := filepath.EvalSymlinks(t.TempDir()) // git prints the real path
	if err != nil {
		t.Fatal(err)
	}
	Run(t, dir, "init")
	Run(t, dir, "commit", "--allow-empty", "-m", "first")
	Run(t, dir, "branch", "work")
	Run(t, dir, "worktree", "add", filepath.Join(dir, "trees", "work"), "work")
	if got, want := Run(t, dir, "rev-parse", "--absolute-git-dir"), filepath.Join(dir, ".git"); got != want {
		t.Errorf("git worked in %s, want %s", got, want)
	}
}
