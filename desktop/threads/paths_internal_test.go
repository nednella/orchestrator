package threads

import (
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/nednella/orchestrator/internal/term"
)

func TestResolveLink(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home folder")
	}
	for _, c := range []struct{ dir, path, want string }{
		{"/work/shop", "src/a.ts", "/work/shop/src/a.ts"},
		{"/work/shop", "./src/../a.ts", "/work/shop/a.ts"},
		{"/work/shop", "../other/a.ts", "/work/other/a.ts"},
		{"/work/shop", "/tmp/x.html", "/tmp/x.html"},
		{"/work/shop", "/tmp/a/../x.html", "/tmp/x.html"},
		{"/work/shop", "~/notes.md", filepath.Join(home, "notes.md")},
		{"/work/shop", "~", home},
		{"/work/shop", "~other/x", "/work/shop/~other/x"},
		{"", "src/a.ts", ""},
		{"work", "src/a.ts", ""},
		{"", "/tmp/x", "/tmp/x"},
		{"/work/shop", "", ""},
	} {
		if got := ResolveLink(c.dir, c.path); got != c.want {
			t.Errorf("ResolveLink(%q, %q) = %q, want %q", c.dir, c.path, got, c.want)
		}
	}
}

func TestOpenAbsoluteReadsOnlyARegularFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	file := write("a.txt")
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "link.txt"} {
		f, got, err := OpenAbsolute(filepath.Join(dir, name), "test")
		if err != nil || got != real {
			t.Errorf("OpenAbsolute(%s) = %q, %v; want the real path %q", name, got, err, real)
		}
		if f != nil {
			f.Close()
		}
	}
	for path, want := range map[string]string{
		filepath.Join(dir, "folder"):   "This is not a file",
		filepath.Join(dir, "pipe"):     "This is not a file",
		filepath.Join(dir, "gone"):     "This file does not exist",
		filepath.Join(dir, "dangling"): "This file does not exist",
		filepath.Join(file, "x"):       "This file does not exist",
		"a.txt":                        "This file is not at an absolute path",
		"":                             "This file is not at an absolute path",
	} {
		if f, _, err := OpenAbsolute(path, "test"); err == nil || err.Error() != want {
			t.Errorf("OpenAbsolute(%q) = %v, want %q", path, err, want)
		} else if f != nil {
			t.Errorf("OpenAbsolute(%q) gave a file with its error", path)
		}
	}
}

func TestOpenFileInEditorOpensAFileAtALineWhereTheEditorTakesOne(t *testing.T) {
	t.Parallel()
	wt, elsewhere := t.TempDir(), t.TempDir()
	for _, p := range []string{filepath.Join(wt, "main.go"), filepath.Join(elsewhere, "page.html")} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(elsewhere, "page.html")
	for _, c := range []struct {
		app, path string
		line      int
		want      []string
	}{
		{"", "main.go", 12, []string{"open", "-a", "Visual Studio Code", "vscode://file" + filepath.Join(wt, "main.go") + ":12"}},
		{"", "main.go", 0, []string{"open", "-a", "Visual Studio Code", filepath.Join(wt, "main.go")}},
		{"Cursor", outside, 3, []string{"open", "-a", "Cursor", "cursor://file" + outside + ":3"}},
		{"Zed", "main.go", 5, []string{"open", "-a", "Zed", "zed://file" + filepath.Join(wt, "main.go") + ":5"}},
		{"Sublime Text", "main.go", 9, []string{"open", "-a", "Sublime Text", filepath.Join(wt, "main.go")}},
		{"", wt, 4, []string{"open", "-a", "Visual Studio Code", wt}},
	} {
		o := &opened{}
		app := func() string { return c.app }
		if c.app == "" {
			app = nil
		}
		s, name := editorThreads(t, o, app, t.TempDir())
		s.info = []term.Info{{Name: name, Path: wt}}
		if err := s.OpenFileInEditor(name.String(), c.path, c.line); err != nil {
			t.Fatalf("OpenFileInEditor(%q, %d): %v", c.path, c.line, err)
		}
		if want := [][]string{c.want}; !reflect.DeepEqual(o.calls, want) {
			t.Errorf("OpenFileInEditor(%q, %d) ran %q, want %q", c.path, c.line, o.calls, want)
		}
	}
}

func TestOpenFileInEditorRefusesAPathThatIsNotThere(t *testing.T) {
	t.Parallel()
	wt := t.TempDir()
	o := &opened{}
	s, name := editorThreads(t, o, nil, t.TempDir())
	s.info = []term.Info{{Name: name, Path: wt}}
	for _, path := range []string{"gone.go", "/tmp/orchestrator-no-such-file", ""} {
		if err := s.OpenFileInEditor(name.String(), path, 1); err == nil || err.Error() != "This file does not exist" {
			t.Errorf("OpenFileInEditor(%q) = %v, want it does not exist", path, err)
		}
	}
	if len(o.calls) != 0 {
		t.Errorf("ran %q, want nothing", o.calls)
	}
}

func TestRevealInFinderShowsTheFileOrFolder(t *testing.T) {
	t.Parallel()
	wt := t.TempDir()
	if err := os.Mkdir(filepath.Join(wt, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	o := &opened{}
	s, name := editorThreads(t, o, nil, t.TempDir())
	s.info = []term.Info{{Name: name, Path: wt}}
	if err := s.RevealInFinder(name.String(), "docs"); err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"open", "-R", filepath.Join(wt, "docs")}}; !reflect.DeepEqual(o.calls, want) {
		t.Errorf("ran %q, want %q", o.calls, want)
	}
	if err := s.RevealInFinder(name.String(), "gone"); err == nil || err.Error() != "This file does not exist" {
		t.Errorf("RevealInFinder(gone) = %v", err)
	}
}
