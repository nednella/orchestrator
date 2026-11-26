package terminal_test

import (
	"os"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

var eventually = apptest.Eventually

func newHarness(t *testing.T) *apptest.Harness { return apptest.New(t) }

// newShell is the project's shell session, on /bin/sh with a throwaway home so no profile of the owner runs: a pane
// that runs what the test types.
func newShell(t *testing.T) (*apptest.Harness, string) {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("HOME", t.TempDir())
	h := newHarness(t)
	shell, err := h.ShellNew("")
	if err != nil {
		t.Fatal(err)
	}
	return h, shell.ID
}
