package chat

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/run"
	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
)

// noProject knows no project.
func noProject(string) string { return "" }

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

	svc := NewService(New(folderThread(dir), t.TempDir(), func(string, any) {}), run.Exec, noProject, context.Background)
	got, err := svc.ChatFiles("main/abcdefgh", "")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go", "b.go"}; !slices.Equal(got, want) {
		t.Errorf("ChatFiles = %q, want %q", got, want)
	}
}

func TestProjectFilesListsTheProjectFolder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	git(t, dir, "init", "-q")
	for _, name := range []string{"main.go", "docs/readme.md"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	projectDir := func(key string) string {
		if key == "demo" {
			return dir
		}
		return ""
	}
	svc := NewService(New(folderThread(t.TempDir()), t.TempDir(), func(string, any) {}), run.Exec, projectDir, context.Background)
	got, err := svc.ProjectFiles("demo", "read")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"docs/readme.md"}; !slices.Equal(got, want) {
		t.Errorf("ProjectFiles = %q, want %q", got, want)
	}
	if _, err := svc.ProjectFiles("gone", ""); err == nil {
		t.Error("ProjectFiles of a project that is not configured gave no error")
	}
}

func TestChatFilesOutsideGitIsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := NewService(New(folderThread(dir), t.TempDir(), func(string, any) {}), run.Exec, noProject, context.Background)
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

func TestFilesListsAFolderOnceForCallsAtOnce(t *testing.T) {
	var runs atomic.Int32
	release := make(chan struct{})
	slow := func(context.Context, string, string, ...string) ([]byte, error) {
		runs.Add(1)
		<-release
		return []byte("a.go\x00"), nil
	}
	f := newFiles(slow)
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			if got, err := f.list(context.Background(), "/repo"); err != nil || !slices.Equal(got, []string{"a.go"}) {
				t.Errorf("list = %q, %v", got, err)
			}
		})
	}
	for runs.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // the other calls reach the listing that runs
	close(release)
	wg.Wait()
	if n := runs.Load(); n != 1 {
		t.Errorf("git ran %d times, want once", n)
	}
}

func TestFilesForgetsOldFolders(t *testing.T) {
	listed := func(context.Context, string, string, ...string) ([]byte, error) { return []byte("a.go\x00"), nil }
	f := newFiles(listed)
	for _, dir := range []string{"/one", "/two"} {
		if _, err := f.list(context.Background(), dir); err != nil {
			t.Fatal(err)
		}
	}
	f.byDir["/one"].at = time.Now().Add(-filesFor)
	if _, err := f.list(context.Background(), "/three"); err != nil {
		t.Fatal(err)
	}
	if _, kept := f.byDir["/one"]; kept || len(f.byDir) != 2 {
		t.Errorf("folders kept = %v, want /two and /three", slices.Collect(maps.Keys(f.byDir)))
	}
}

// folderThread is one ended thread that works in dir.
type folderThread string

func (f folderThread) ChatOf(id string) (sessions.Chat, error) {
	return sessions.Chat{ID: id, Project: "main", Dir: string(f)}, nil
}
func (folderThread) Chats() []sessions.Chat                    { return nil }
func (folderThread) TurnEnded(id string, e chatlog.Event)      {}
func (folderThread) AccessChanged(string, string, bool)        {}
func (folderThread) FlagRefused(string)                        {}
func (folderThread) Wake(string) (sessions.Chat, error)        { return sessions.Chat{}, nil }
func (folderThread) FilesRewind(string) (string, bool, string) { return "", true, "" }

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
