package sessions_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/internal/project"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

var (
	eventually = apptest.Eventually
	exists     = apptest.Exists
	prJSON     = apptest.PRJSON
)

// newHarness is a project with the default clean-up command, which removes the worktree and keeps the branch.
func newHarness(t *testing.T) *apptest.Harness { return apptest.New(t) }

// noteCleanups is the default clean-up command, after a line in the file ran with the {worktree} it runs for.
func noteCleanups(ran string) string {
	return fmt.Sprintf("echo {worktree} >> %q && ", ran) + project.DefaultCleanupCommand
}

// writtenTranscript is a transcript file of a conversation, which Claude writes at the conversation's first prompt.
func writtenTranscript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
