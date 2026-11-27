package issues_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	chatlog "github.com/nednella/orchestrator/internal/chat"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

var (
	eventually = apptest.Eventually
	exists     = apptest.Exists
)

func newHarness(t *testing.T) *apptest.Harness { return apptest.New(t) }

// gitRepo makes h.Dir a git repo with one commit on main, so an issue thread gets a worktree and branch of its own.
func gitRepo(t *testing.T, h *apptest.Harness) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(h.Dir, "README"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init"}, {"add", "README"}, {"commit", "-m", "first"}} {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
		cmd.Dir = h.Dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// sent says whether the thread's claude read a user message with the text.
func sent(t *testing.T, h *apptest.Harness, id, text string) bool {
	t.Helper()
	quoted, _ := json.Marshal(text)
	return slices.ContainsFunc(h.Stdin(t, id), func(line string) bool {
		return strings.HasPrefix(line, `{"type":"user"`) && strings.Contains(line, `"text":`+string(quoted))
	})
}

// drafted says whether the thread's chat log holds a draft of the text.
func drafted(h *apptest.Harness, id, text string) bool {
	events, _ := h.ChatHistory(id, 0)
	return slices.ContainsFunc(events, func(e chatlog.Event) bool { return e.Kind == chatlog.KindDraft && e.Text == text })
}
