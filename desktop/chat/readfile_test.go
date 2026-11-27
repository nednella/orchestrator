package chat

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/threads"
)

// fileThread is one ended thread, number 7, that works in its folder.
type fileThread struct {
	folderThread
}

func (f fileThread) ChatOf(id string) (threads.Chat, error) {
	return threads.Chat{ID: id, Project: "main", Dir: string(f.folderThread), N: 7}, nil
}

// readService is a service for one thread in dir and the project demo in projectDir.
func readService(t *testing.T, dir, projectDir string) *Service {
	t.Helper()
	known := func(key string) string {
		if key == "demo" {
			return projectDir
		}
		return ""
	}
	return NewService(New(fileThread{folderThread(dir)}, t.TempDir(), func(string, any) {}), run.Exec, known, context.Background)
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestReadFileInsideTheFolder(t *testing.T) {
	t.Parallel()
	dir, projectDir := t.TempDir(), t.TempDir()
	writeFiles(t, dir, map[string]string{"docs/plan.md": "# Plan", "README.markdown": "# Readme"})
	writeFiles(t, projectDir, map[string]string{"AGENTS.md": "# Agents"})
	symlink(t, filepath.Join("docs", "plan.md"), filepath.Join(dir, "linked.md"))
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := readService(t, dir, projectDir)
	for path, want := range map[string]string{
		"docs/plan.md":                      "# Plan",
		"docs/../docs/plan.md":              "# Plan",
		filepath.Join(dir, "docs/plan.md"):  "# Plan",
		"README.markdown":                   "# Readme",
		"linked.md":                         "# Plan",
		filepath.Join(dir, "linked.md"):     "# Plan",
		"./docs/plan.md":                    "# Plan",
		filepath.Join(real, "docs/plan.md"): "# Plan",
	} {
		if got, err := svc.ChatReadFile("main/abcdefgh", path); err != nil || got.Kind != "text" || got.Text != want || got.Size != int64(len(want)) || got.Truncated {
			t.Errorf("ChatReadFile(%q) = %+v, %v; want the text %q", path, got, err, want)
		}
	}
	if got, err := svc.ProjectReadFile("demo", "AGENTS.md"); err != nil || got.Text != "# Agents" {
		t.Errorf("ProjectReadFile = %+v, %v; want the project's AGENTS.md", got, err)
	}
}

func TestReadFileRefusesOutside(t *testing.T) {
	t.Parallel()
	parent, elsewhere := t.TempDir(), t.TempDir()
	dir := filepath.Join(parent, "issue-7")
	writeFiles(t, parent, map[string]string{"x.md": "# Secret", "issue-7/in.md": "# In"})
	writeFiles(t, elsewhere, map[string]string{"y.md": "# Elsewhere"})
	symlink(t, filepath.Join(parent, "x.md"), filepath.Join(dir, "out.md"))
	symlink(t, elsewhere, filepath.Join(dir, "docs"))
	symlink(t, filepath.Join(parent, "gone.md"), filepath.Join(dir, "dangling.md"))
	symlink(t, "../x.md", filepath.Join(dir, "up.md"))
	svc := readService(t, dir, dir)
	// A path out of the folder gets the same answer whether its file is there or not.
	for _, path := range []string{"../x.md", "../gone.md", "../issue-7/../x.md", filepath.Join(parent, "x.md"), filepath.Join(parent, "gone.md"), filepath.Join(elsewhere, "y.md"), "out.md", "dangling.md", "up.md", "docs/y.md", "docs/gone.md"} {
		got, err := svc.ChatReadFile("main/abcdefgh", path)
		if want := "This file is outside thread 7's folder"; err == nil || err.Error() != want {
			t.Errorf("ChatReadFile(%q) = %+v, %v; want %q", path, got, err, want)
		}
	}
	if _, err := svc.ProjectReadFile("demo", "../x.md"); err == nil || err.Error() != "This file is outside project demo's folder" {
		t.Errorf("ProjectReadFile(../x.md) = %v, want it outside the project's folder", err)
	}
	if _, err := svc.ProjectReadFile("gone", "in.md"); err == nil || err.Error() != "No project gone" {
		t.Errorf("ProjectReadFile of a project that is not configured = %v", err)
	}
}

func TestReadFileRefusesWhatIsNotAFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"folder.d/a.md": "# A", "full.md": "x"})
	symlink(t, "folder.d", filepath.Join(dir, "folder"))
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := readService(t, dir, dir)
	for path, want := range map[string]string{
		"folder.d":  "This is not a file",
		"folder":    "This is not a file",
		"pipe":      "This is not a file",
		"gone.md":   "This file does not exist",
		"full.md/x": "This file does not exist",
	} {
		if got, err := svc.ChatReadFile("main/abcdefgh", path); err == nil || err.Error() != want {
			t.Errorf("ChatReadFile(%q) = %+v, %v; want %q", path, got, err, want)
		}
	}
}

func TestReadFileShowsAnyTextFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"main.go": "package main\n", ".env": "TOKEN=x", "empty.txt": "", "ünï/cødé.ts": "const é = 1"})
	svc := readService(t, dir, dir)
	for path, want := range map[string]string{"main.go": "package main\n", ".env": "TOKEN=x", "empty.txt": "", "ünï/cødé.ts": "const é = 1"} {
		if got, err := svc.ChatReadFile("main/abcdefgh", path); err != nil || got.Kind != "text" || got.Text != want {
			t.Errorf("ChatReadFile(%q) = %+v, %v; want the text %q", path, got, err, want)
		}
	}
}

func TestReadFileCutsABigTextFileWhereACharacterEnds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// "é" is two bytes, so the cap falls inside the last one.
	big := strings.Repeat("a", maxShown-1) + "é" + "tail"
	writeFiles(t, dir, map[string]string{"big.txt": big, "full.txt": strings.Repeat("a", maxShown)})
	svc := readService(t, dir, dir)
	got, err := svc.ChatReadFile("main/abcdefgh", "big.txt")
	if err != nil || got.Kind != "text" || !got.Truncated || got.Size != int64(len(big)) || got.Text != strings.Repeat("a", maxShown-1) {
		t.Errorf("ChatReadFile(big.txt) = %d bytes of %q, truncated %v, size %d, %v; want the first %d bytes, cut before the character", len(got.Text), got.Kind, got.Truncated, got.Size, err, maxShown-1)
	}
	got, err = svc.ChatReadFile("main/abcdefgh", "full.txt")
	if err != nil || got.Truncated || len(got.Text) != maxShown {
		t.Errorf("ChatReadFile(full.txt) = %d bytes, truncated %v, %v; want the whole file", len(got.Text), got.Truncated, err)
	}
}

func TestReadFileSaysABinaryFileIsBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"nul.bin": "ab\x00cd", "latin1.txt": "caf\xe9", "later.bin": strings.Repeat("a", 5000) + "\x00"})
	svc := readService(t, dir, dir)
	for _, path := range []string{"nul.bin", "latin1.txt", "later.bin"} {
		if got, err := svc.ChatReadFile("main/abcdefgh", path); err != nil || got.Kind != "binary" || got.Text != "" || got.Size == 0 {
			t.Errorf("ChatReadFile(%q) = %+v, %v; want a binary file with no text", path, got, err)
		}
	}
}

func TestReadFileGivesAnImageAsBase64(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"logo.PNG": "\x89PNG-bytes", "icon.svg": "<svg/>", "huge.png": strings.Repeat("a", maxImage+1)})
	svc := readService(t, dir, dir)
	got, err := svc.ChatReadFile("main/abcdefgh", "logo.PNG")
	if want := (FileContent{Kind: "image", Size: 10, Mime: "image/png", Data: base64.StdEncoding.EncodeToString([]byte("\x89PNG-bytes"))}); err != nil || got != want {
		t.Errorf("ChatReadFile(logo.PNG) = %+v, %v; want %+v", got, err, want)
	}
	if got, err := svc.ChatReadFile("main/abcdefgh", "icon.svg"); err != nil || got.Mime != "image/svg+xml" {
		t.Errorf("ChatReadFile(icon.svg) = %+v, %v; want an svg image", got, err)
	}
	if got, err := svc.ChatReadFile("main/abcdefgh", "huge.png"); err == nil || err.Error() != "This file is too big to show" {
		t.Errorf("ChatReadFile(huge.png) = %d bytes, %v; want it too big", len(got.Data), err)
	}
}

func TestReadFileRefusesAnImageOutside(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	dir := filepath.Join(parent, "issue-7")
	writeFiles(t, parent, map[string]string{"secret.png": "png", "issue-7/in.png": "png"})
	symlink(t, filepath.Join(parent, "secret.png"), filepath.Join(dir, "out.png"))
	svc := readService(t, dir, dir)
	for _, path := range []string{"../secret.png", filepath.Join(parent, "secret.png"), "out.png"} {
		if got, err := svc.ChatReadFile("main/abcdefgh", path); err == nil || err.Error() != "This file is outside thread 7's folder" {
			t.Errorf("ChatReadFile(%q) = %+v, %v; want it outside the folder", path, got, err)
		}
	}
	if got, err := svc.ChatReadFile("main/abcdefgh", "in.png"); err != nil || got.Kind != "image" {
		t.Errorf("ChatReadFile(in.png) = %+v, %v; want the image", got, err)
	}
}
