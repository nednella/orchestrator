package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/run"
)

// Not parallel: it sets the process environment.
func TestFileTreeListsWhatGitKnowsAndWhatItIgnores(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	git(t, dir, "init", "-q")
	writeFiles(t, dir, map[string]string{
		".gitignore": "node_modules/\n*.log\n", "README.md": "# R", "src/main.go": "", "src/old.go": "", "src/new.go": "",
		"debug.log": "", "node_modules/pkg/index.js": "", "empty/.keep": "",
	})
	git(t, dir, "add", "README.md", "src/main.go", "src/old.go")
	git(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "x")
	if err := os.Remove(filepath.Join(dir, "src", "old.go")); err != nil {
		t.Fatal(err)
	}
	svc := NewService(New(folderThread(dir), t.TempDir(), func(string, any) {}), run.Exec, func(key string) string {
		if key == "demo" {
			return dir
		}
		return ""
	}, context.Background)
	want := []FileEntry{
		{Path: ".gitignore"}, {Path: "README.md"}, {Path: "debug.log", Ignored: true}, {Path: "empty/.keep"},
		{Path: "node_modules", Dir: true, Ignored: true}, {Path: "src/main.go"}, {Path: "src/new.go"},
	}
	for name, get := range map[string]func() (FileListing, error){
		"ChatFileTree":    func() (FileListing, error) { return svc.ChatFileTree("main/abcdefgh") },
		"ProjectFileTree": func() (FileListing, error) { return svc.ProjectFileTree("demo") },
	} {
		got, err := get()
		if err != nil || got.Truncated || !slices.Equal(got.Entries, want) {
			t.Errorf("%s = %+v, %v; want %+v", name, got, err, want)
		}
	}
	if _, err := svc.ProjectFileTree("gone"); err == nil {
		t.Error("ProjectFileTree of a project that is not configured gave no error")
	}
}

// Not parallel: it sets the process environment.
func TestFileTreeOfAFolderOutsideGitListsEverythingButGit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	writeFiles(t, dir, map[string]string{"a.txt": "", "docs/b.md": "", ".git/config": ""})
	if err := os.Mkdir(filepath.Join(dir, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	symlink(t, "docs", filepath.Join(dir, "link"))
	got, err := readService(t, dir, dir, "").ChatFileTree("main/abcdefgh")
	want := []FileEntry{{Path: "a.txt"}, {Path: "docs", Dir: true}, {Path: "docs/b.md"}, {Path: "empty", Dir: true}, {Path: "link"}}
	if err != nil || !slices.Equal(got.Entries, want) {
		t.Errorf("ChatFileTree = %+v, %v; want %+v", got, err, want)
	}
}

func TestFileTreeStopsAtTheCap(t *testing.T) {
	t.Parallel()
	var listed strings.Builder
	for i := range maxTreeEntries + 5 {
		fmt.Fprintf(&listed, "f/%06d.txt\x00", i)
	}
	many := func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
		if slices.Contains(args, "-c") {
			return []byte(listed.String()), nil
		}
		return nil, nil
	}
	svc := NewService(New(folderThread(t.TempDir()), t.TempDir(), func(string, any) {}), many, noProject, context.Background)
	got, err := svc.ChatFileTree("main/abcdefgh")
	if err != nil || !got.Truncated || len(got.Entries) != maxTreeEntries {
		t.Errorf("ChatFileTree = %d entries, truncated %v, %v; want %d, cut short", len(got.Entries), got.Truncated, err, maxTreeEntries)
	}
}

func TestFileDirListsOneLevelOfAFolderInside(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"node_modules/a/index.js": "", "node_modules/b.json": "", "node_modules/.git/x": "", ".git/config": ""})
	svc := readService(t, dir, dir, "")
	want := []FileEntry{{Path: "node_modules/.git", Dir: true, Ignored: true}, {Path: "node_modules/a", Dir: true, Ignored: true}, {Path: "node_modules/b.json", Ignored: true}}
	if got, err := svc.ChatFileDir("main/abcdefgh", "node_modules"); err != nil || !slices.Equal(got.Entries, want) {
		t.Errorf("ChatFileDir(node_modules) = %+v, %v; want %+v", got, err, want)
	}
	if got, err := svc.ProjectFileDir("demo", "node_modules/a"); err != nil || !slices.Equal(got.Entries, []FileEntry{{Path: "node_modules/a/index.js", Ignored: true}}) {
		t.Errorf("ProjectFileDir(node_modules/a) = %+v, %v", got, err)
	}
	if got, err := svc.ChatFileDir("main/abcdefgh", ""); err != nil || slices.ContainsFunc(got.Entries, func(e FileEntry) bool { return e.Path == ".git" }) {
		t.Errorf("ChatFileDir of the root = %+v, %v; want no .git", got, err)
	}
}

func TestFileDirRefusesAFolderOutside(t *testing.T) {
	t.Parallel()
	parent, elsewhere := t.TempDir(), t.TempDir()
	dir := filepath.Join(parent, "issue-7")
	writeFiles(t, parent, map[string]string{"x/secret": "", "issue-7/in/a": ""})
	writeFiles(t, elsewhere, map[string]string{"y": ""})
	symlink(t, elsewhere, filepath.Join(dir, "out"))
	symlink(t, "../x", filepath.Join(dir, "up"))
	svc := readService(t, dir, dir, "")
	for _, path := range []string{"..", "../x", filepath.Join(parent, "x"), "in/../../x", "out", "up", "out/y"} {
		got, err := svc.ChatFileDir("main/abcdefgh", path)
		if want := "This folder is outside thread 7's folder"; err == nil || err.Error() != want {
			t.Errorf("ChatFileDir(%q) = %+v, %v; want %q", path, got, err, want)
		}
	}
	if _, err := svc.ChatFileDir("main/abcdefgh", "gone"); err == nil || err.Error() != "This folder does not exist" {
		t.Errorf("ChatFileDir(gone) = %v, want it does not exist", err)
	}
}
