package chat

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nednella/agentos/desktop/internal/run"
	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
)

func TestRankFiles(t *testing.T) {
	paths := []string{"src/app/main.go", "readme.md", "cmd/domain.go", "main.go", "mxaxixn.c"}
	cases := []struct {
		query string
		limit int
		want  []string
	}{
		{"main", 30, []string{"main.go", "src/app/main.go", "cmd/domain.go", "mxaxixn.c"}},
		{"MAIN", 30, []string{"main.go", "src/app/main.go", "cmd/domain.go", "mxaxixn.c"}},
		{"", 2, []string{"main.go", "readme.md"}},
		{"zzz", 30, []string{}},
	}
	for _, c := range cases {
		if got := rankFiles(paths, c.query, c.limit); !slices.Equal(got, c.want) {
			t.Errorf("rankFiles(%q) = %q, want %q", c.query, got, c.want)
		}
	}
}

func TestChatFilesListsTrackedAndUntracked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	git(t, dir, "init", "-q")
	for name, text := range map[string]string{"a.go": "package a", "b.go": "package b", "c.log": "noise"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"), []byte("*.log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.go")

	svc := NewService(New(folderThread(dir), func(string, any) {}), run.Exec, context.Background)
	got, err := svc.ChatFiles("main/abcdefgh", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go", "b.go"}; !slices.Equal(got, want) {
		t.Errorf("ChatFiles = %q, want %q", got, want)
	}
}

func TestChatFilesOutsideGitIsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewService(New(folderThread(dir), func(string, any) {}), run.Exec, context.Background)
	got, err := svc.ChatFiles("main/abcdefgh", "")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("ChatFiles = %q, %v; want an empty list", got, err)
	}
}

func TestFilesDropsAPathCutShort(t *testing.T) {
	cut := func(context.Context, string, string, ...string) ([]byte, error) {
		return []byte("a.go\x00src/b.go\x00src/c.g"), nil
	}
	got, err := newFiles(cut).list(context.Background(), t.TempDir())
	if want := []string{"a.go", "src/b.go"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("list = %q, %v; want %q", got, err, want)
	}
}

// folderThread is one ended thread that works in dir.
type folderThread string

func (f folderThread) ChatOf(id string) (sessions.Chat, error) {
	return sessions.Chat{ID: id, Project: "main", Dir: string(f)}, nil
}
func (folderThread) Chats() []sessions.Chat               { return nil }
func (folderThread) TurnEnded(id string, e chatlog.Event) {}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
