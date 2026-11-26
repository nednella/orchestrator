package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/nednella/agentos/desktop/internal/run"
	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
)

// fileThread is one ended thread, number 7, that works in its folder and keeps its chat log at log.
type fileThread struct {
	folderThread
	log string
}

func (f fileThread) ChatOf(id string) (sessions.Chat, error) {
	return sessions.Chat{ID: id, Project: "main", Dir: string(f.folderThread), Log: f.log, N: 7}, nil
}

// readService is a service for one thread in dir and the project demo in projectDir.
func readService(t *testing.T, dir, projectDir, log string) *Service {
	t.Helper()
	known := func(key string) string {
		if key == "demo" {
			return projectDir
		}
		return ""
	}
	return NewService(New(fileThread{folderThread(dir), log}, t.TempDir(), func(string, any) {}), run.Exec, known, context.Background)
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
	dir, projectDir := t.TempDir(), t.TempDir()
	writeFiles(t, dir, map[string]string{"docs/plan.md": "# Plan", "README.markdown": "# Readme"})
	writeFiles(t, projectDir, map[string]string{"AGENTS.md": "# Agents"})
	symlink(t, filepath.Join("docs", "plan.md"), filepath.Join(dir, "linked.md"))
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := readService(t, dir, projectDir, "")
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
		if got, err := svc.ChatReadFile("main/abcdefgh", path); err != nil || got != want {
			t.Errorf("ChatReadFile(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
	if got, err := svc.ProjectReadFile("demo", "AGENTS.md"); err != nil || got != "# Agents" {
		t.Errorf("ProjectReadFile = %q, %v; want the project's AGENTS.md", got, err)
	}
}

func TestReadFileRefusesOutside(t *testing.T) {
	parent, elsewhere := t.TempDir(), t.TempDir()
	dir := filepath.Join(parent, "issue-7")
	writeFiles(t, parent, map[string]string{"x.md": "# Secret", "issue-7/in.md": "# In"})
	writeFiles(t, elsewhere, map[string]string{"y.md": "# Elsewhere"})
	symlink(t, filepath.Join(parent, "x.md"), filepath.Join(dir, "out.md"))
	symlink(t, elsewhere, filepath.Join(dir, "docs"))
	svc := readService(t, dir, dir, "")
	for _, path := range []string{"../x.md", "../issue-7/../x.md", filepath.Join(parent, "x.md"), filepath.Join(elsewhere, "y.md"), "out.md", "docs/y.md", "../gone.md"} {
		got, err := svc.ChatReadFile("main/abcdefgh", path)
		if want := "This file is outside thread 7's folder"; err == nil || err.Error() != want {
			t.Errorf("ChatReadFile(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
	if _, err := svc.ProjectReadFile("demo", "../x.md"); err == nil || err.Error() != "This file is outside project demo's folder" {
		t.Errorf("ProjectReadFile(../x.md) = %v, want it outside the project's folder", err)
	}
	if _, err := svc.ProjectReadFile("gone", "in.md"); err == nil || err.Error() != "No project gone" {
		t.Errorf("ProjectReadFile of a project that is not configured = %v", err)
	}
}

func TestReadFileRefusesBigAndOtherFiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"big.md":         strings.Repeat("a", maxShown+1),
		"full.md":        strings.Repeat("a", maxShown),
		"notes.txt":      "notes",
		".env":           "TOKEN=x",
		"folder.md/a.md": "# A",
	})
	symlink(t, ".env", filepath.Join(dir, "env.md"))
	symlink(t, "full.md", filepath.Join(dir, "full"))
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe.md"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := readService(t, dir, dir, "")
	for path, want := range map[string]string{
		"big.md":    "This file is too big to show",
		"notes.txt": "This is not a markdown file",
		"env.md":    "This is not a markdown file",
		"full":      "This is not a markdown file",
		"folder.md": "This is not a markdown file",
		"pipe.md":   "This is not a markdown file",
		"gone.md":   "This file does not exist",
	} {
		if got, err := svc.ChatReadFile("main/abcdefgh", path); err == nil || err.Error() != want {
			t.Errorf("ChatReadFile(%q) = %.20q, %v; want %q", path, got, err, want)
		}
	}
	if got, err := svc.ChatReadFile("main/abcdefgh", "full.md"); err != nil || len(got) != maxShown {
		t.Errorf("ChatReadFile(full.md) = %d bytes, %v; want the whole file", len(got), err)
	}
}

// logOf writes the chat log of the stream in testdata name, then the events more, and gives its path.
func logOf(t *testing.T, name string, more ...chatlog.Event) string {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "internal", "chat", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	path := filepath.Join(t.TempDir(), "chat.jsonl")
	l, err := chatlog.OpenLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	var events []chatlog.Event
	for sc.Scan() {
		evs, err := chatlog.Parse(sc.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, evs...)
	}
	for _, e := range append(events, more...) {
		if _, err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func planInput(t *testing.T, plan string) json.RawMessage {
	t.Helper()
	input, err := json.Marshal(map[string]string{"plan": plan})
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func TestLatestPlanIsTheLastExitPlanMode(t *testing.T) {
	svc := readService(t, t.TempDir(), "", logOf(t, "planexit.jsonl"))
	got, err := svc.ChatLatestPlan("main/abcdefgh")
	if err != nil || !strings.HasPrefix(got, "# Plan: add made.txt\n") {
		t.Errorf("ChatLatestPlan = %.40q, %v; want the recorded plan", got, err)
	}

	later := []chatlog.Event{
		{Kind: chatlog.KindApproval, ID: "plan-2", Tool: "ExitPlanMode", Input: planInput(t, "# Second plan")},
		{Kind: chatlog.KindApproval, ID: "bash-1", Tool: "Bash", Input: json.RawMessage(`{"command":"ls","plan":"no"}`)},
	}
	svc = readService(t, t.TempDir(), "", logOf(t, "planexit.jsonl", later...))
	if got, err := svc.ChatLatestPlan("main/abcdefgh"); err != nil || got != "# Second plan" {
		t.Errorf("ChatLatestPlan = %q, %v; want the later plan", got, err)
	}

	svc = readService(t, t.TempDir(), "", filepath.Join(t.TempDir(), "none.jsonl"))
	if got, err := svc.ChatLatestPlan("main/abcdefgh"); err != nil || got != "" {
		t.Errorf("ChatLatestPlan of a chat with no plan = %q, %v; want none", got, err)
	}
}

func TestMarkdownFilesListsOnlyMarkdown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	git(t, dir, "init", "-q")
	writeFiles(t, dir, map[string]string{"README.md": "", "docs/config.markdown": "", "NOTES.MD": "", "main.go": "", "notes.txt": "", "build/out.md": ""})
	writeFiles(t, dir, map[string]string{".git/info/exclude": "build/\n"})
	git(t, dir, "add", "README.md")
	svc := readService(t, dir, dir, "")
	want := []string{"NOTES.MD", "README.md", "docs/config.markdown"}
	if got, err := svc.ChatMarkdownFiles("main/abcdefgh"); err != nil || !slices.Equal(got, want) {
		t.Errorf("ChatMarkdownFiles = %q, %v; want %q", got, err, want)
	}
	if got, err := svc.ProjectMarkdownFiles("demo"); err != nil || !slices.Equal(got, want) {
		t.Errorf("ProjectMarkdownFiles = %q, %v; want %q", got, err, want)
	}

	var listed strings.Builder
	for i := range maxMarkdownFiles + 10 {
		fmt.Fprintf(&listed, "doc-%03d.md\x00", i)
	}
	many := func(context.Context, string, string, ...string) ([]byte, error) { return []byte(listed.String()), nil }
	svc = NewService(New(folderThread(dir), t.TempDir(), func(string, any) {}), many, noProject, context.Background)
	if got, err := svc.ChatMarkdownFiles("main/abcdefgh"); err != nil || len(got) != maxMarkdownFiles {
		t.Errorf("ChatMarkdownFiles of a folder with %d = %d files, %v; want %d", maxMarkdownFiles+10, len(got), err, maxMarkdownFiles)
	}
}
