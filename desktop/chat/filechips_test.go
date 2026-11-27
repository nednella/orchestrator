package chat

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestFileChipsSayWhatIsAtEachPath(t *testing.T) {
	t.Parallel()
	parent, elsewhere := t.TempDir(), t.TempDir()
	dir := filepath.Join(parent, "issue-7")
	writeFiles(t, dir, map[string]string{"src/a.go": "package a", "docs/plan.md": "# Plan"})
	writeFiles(t, elsewhere, map[string]string{"page.html": "<p>hi</p>"})
	symlink(t, filepath.Join(elsewhere, "page.html"), filepath.Join(dir, "leak.html"))
	symlink(t, "src/a.go", filepath.Join(dir, "alias.go"))
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := readService(t, dir, dir)
	got, err := svc.ChatFileChips("main/abcdefgh", []string{
		"src/a.go", "./docs/../src/a.go", filepath.Join(dir, "docs/plan.md"), "docs", ".", "alias.go",
		"missing.go", "src/a.go/x", filepath.Join(elsewhere, "page.html"), elsewhere, "leak.html", "pipe", "", "../x",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []FileChip{
		{Path: "src/a.go", Exists: true, Size: 9, Abs: filepath.Join(dir, "src/a.go"), Inside: true, Rel: "src/a.go"},
		{Path: "./docs/../src/a.go", Exists: true, Size: 9, Abs: filepath.Join(dir, "src/a.go"), Inside: true, Rel: "src/a.go"},
		{Path: filepath.Join(dir, "docs/plan.md"), Exists: true, Size: 6, Abs: filepath.Join(dir, "docs/plan.md"), Inside: true, Rel: "docs/plan.md"},
		{Path: "docs", Exists: true, Dir: true, Abs: filepath.Join(dir, "docs"), Inside: true, Rel: "docs"},
		{Path: ".", Exists: true, Dir: true, Abs: dir, Inside: true},
		{Path: "alias.go", Exists: true, Size: 9, Abs: filepath.Join(dir, "alias.go"), Inside: true, Rel: "src/a.go"},
		{Path: "missing.go", Abs: filepath.Join(dir, "missing.go")},
		{Path: "src/a.go/x", Abs: filepath.Join(dir, "src/a.go/x")},
		{Path: filepath.Join(elsewhere, "page.html"), Exists: true, Size: 9, Abs: filepath.Join(elsewhere, "page.html")},
		{Path: elsewhere, Exists: true, Dir: true, Abs: elsewhere},
		{Path: "leak.html", Exists: true, Size: 9, Abs: filepath.Join(dir, "leak.html")},
		{Path: "pipe", Abs: filepath.Join(dir, "pipe")},
		{Path: ""},
		{Path: "../x", Abs: filepath.Join(parent, "x")},
	}
	if !reflect.DeepEqual(got, want) {
		for i := range want {
			if !reflect.DeepEqual(got[i], want[i]) {
				t.Errorf("chip %q = %+v, want %+v", want[i].Path, got[i], want[i])
			}
		}
	}
}

func TestFileChipsRefuseTooManyPathsAndAnUnknownThread(t *testing.T) {
	t.Parallel()
	svc := readService(t, t.TempDir(), t.TempDir())
	if _, err := svc.ChatFileChips("main/abcdefgh", make([]string, maxChips+1)); err == nil {
		t.Error("ChatFileChips of too many paths = nil, want an error")
	}
	if got, err := svc.ChatFileChips("main/abcdefgh", nil); err != nil || got == nil || len(got) != 0 {
		t.Errorf("ChatFileChips of no paths = %v, %v; want an empty list", got, err)
	}
}

func TestReadOutsideReadsAnyRegularFileReadOnly(t *testing.T) {
	t.Parallel()
	dir, elsewhere := t.TempDir(), t.TempDir()
	writeFiles(t, elsewhere, map[string]string{"page.html": "<p>hi</p>", "pic.png": "png", "big.txt": strings.Repeat("a", maxShown+10)})
	symlink(t, filepath.Join(elsewhere, "page.html"), filepath.Join(dir, "link.html"))
	if err := os.Mkdir(filepath.Join(elsewhere, "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(elsewhere, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := readService(t, dir, dir)
	for _, path := range []string{filepath.Join(elsewhere, "page.html"), filepath.Join(dir, "link.html")} {
		if got, err := svc.ChatReadOutside("main/abcdefgh", path); err != nil || got.Kind != "text" || got.Text != "<p>hi</p>" {
			t.Errorf("ChatReadOutside(%q) = %+v, %v; want the page", path, got, err)
		}
	}
	if got, err := svc.ChatReadOutside("main/abcdefgh", filepath.Join(elsewhere, "pic.png")); err != nil || got.Kind != "image" || got.Mime != "image/png" {
		t.Errorf("ChatReadOutside(pic.png) = %+v, %v; want an image", got, err)
	}
	if got, err := svc.ChatReadOutside("main/abcdefgh", filepath.Join(elsewhere, "big.txt")); err != nil || !got.Truncated || len(got.Text) != maxShown {
		t.Errorf("ChatReadOutside(big.txt) = %d bytes, truncated %v, %v; want the size limit", len(got.Text), got.Truncated, err)
	}
	for path, want := range map[string]string{
		filepath.Join(elsewhere, "folder"):  "This is not a file",
		filepath.Join(elsewhere, "pipe"):    "This is not a file",
		filepath.Join(elsewhere, "gone.md"): "This file does not exist",
		"page.html":                         "This file is not at an absolute path",
		"../page.html":                      "This file is not at an absolute path",
	} {
		if got, err := svc.ChatReadOutside("main/abcdefgh", path); err == nil || err.Error() != want {
			t.Errorf("ChatReadOutside(%q) = %+v, %v; want %q", path, got, err, want)
		}
	}
}
