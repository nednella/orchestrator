package gittest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/internal/gittest"
)

// A machine whose owner ignores .env everywhere must not change what git says in a test.
func TestIsolateKeepsTheOwnersIgnoreFileAndConfigOut(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config", "git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "git", "ignore"), []byte(".env\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[core]\n\texcludesfile = "+filepath.Join(home, "global-ignore")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	defer gittest.Isolate()()

	repo := t.TempDir()
	gittest.Run(t, repo, "init")
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := gittest.Run(t, repo, "status", "--porcelain"); got != "?? .env" {
		t.Errorf("status = %q, want the file shown as untracked", got)
	}
	// The app's own git runs in the process's environment.
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	if out, err := cmd.Output(); err != nil || strings.TrimSpace(string(out)) != "?? .env" {
		t.Errorf("git in the process's environment says %q, %v; want the file shown as untracked", out, err)
	}
}
