// Package gittest runs git for tests. A test that a git hook or git rebase --exec runs inherits GIT_DIR and its kin,
// and they point every git the test runs at the repo the tests run in, whatever folder it is given.
package gittest

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/internal/environ"
)

// parent names what a parent git may set that changes which repo git finds.
var parent = slices.Concat(environ.GitRepo, []string{"GIT_CEILING_DIRECTORIES"})

// Env is the process's environment without what a parent git set to point git at its repo.
func Env() []string { return environ.Without(os.Environ(), parent...) }

// Command is git with args in dir, in Env, with no global or system config.
func Command(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = environ.With(Env(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	return cmd
}

// Run runs Command as the user t, with main as the first branch and commits unsigned, and returns what git printed,
// trimmed. A git that fails fails the test.
func Run(t testing.TB, dir string, args ...string) string {
	t.Helper()
	out, err := Command(dir, append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// Unchanged reads every file under dir, and gives back a check that fails the test, naming each file that is new, gone
// or different, if dir no longer holds the same.
func Unchanged(t testing.TB, dir string) (check func()) {
	t.Helper()
	before := files(t, dir)
	return func() {
		t.Helper()
		after := files(t, dir)
		var changed []string
		for path, text := range before {
			if now, ok := after[path]; !ok || now != text {
				changed = append(changed, path)
			}
		}
		for path := range after {
			if _, ok := before[path]; !ok {
				changed = append(changed, path)
			}
		}
		if len(changed) > 0 {
			slices.Sort(changed)
			t.Errorf("%s changed: %s", dir, strings.Join(changed, ", "))
		}
	}
}

func files(t testing.TB, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		rel, _ := filepath.Rel(dir, path)
		out[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
