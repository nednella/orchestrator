package terminal_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

var eventually = apptest.Eventually

func newHarness(t *testing.T) *apptest.Harness { return apptest.New(t) }

// newShell is the project's shell session, on /bin/sh with a throwaway home so no profile of the owner runs: a pane
// that runs what the test types.
func newShell(t *testing.T) (*apptest.Harness, string) {
	t.Helper()
	h := apptest.NewWith(t, apptest.Options{Env: []string{"SHELL=/bin/sh", "HOME=" + t.TempDir()}})
	shell, err := h.ShellNew("")
	if err != nil {
		t.Fatal(err)
	}
	// What is typed before the shell reads it is shown twice, once by the terminal and once by the shell's line editor,
	// and a shell on a slow machine starts late: the tests type once it has run a command.
	// The command names the file by a path with no digits, which the tests look for in what they read.
	ready := filepath.Join(h.Dir, ".shell-ready")
	if err := exec.Command("tmux", "-L", h.Socket, "send-keys", "-t", "="+shell.ID+":", "touch .shell-ready", "Enter").Run(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the shell to run a command", func() bool { return apptest.Exists(ready) })
	return h, shell.ID
}
