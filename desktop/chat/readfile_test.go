package chat

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/sessions"
	chatlog "github.com/nednella/orchestrator/internal/chat"
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
	t.Parallel()
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
	svc := readService(t, dir, dir, "")
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
	svc := readService(t, dir, dir, "")
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
	svc := readService(t, dir, dir, "")
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
	svc := readService(t, dir, dir, "")
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
	svc := readService(t, dir, dir, "")
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
	svc := readService(t, dir, dir, "")
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
	t.Parallel()
	recorded := chatlog.Event{Kind: chatlog.KindAnswer, ID: "72988c6f-7876-4e3e-9fdd-442e209bc19e", Text: "allow"}
	plan := func(id, text string) chatlog.Event {
		return chatlog.Event{Kind: chatlog.KindApproval, ID: id, Tool: "ExitPlanMode", Input: planInput(t, text)}
	}
	answer := func(id, text string) chatlog.Event {
		return chatlog.Event{Kind: chatlog.KindAnswer, ID: id, Text: text}
	}
	subagents := plan("plan-s", "# A subagent's plan")
	subagents.Agent = "toolu_agent"
	cases := []struct {
		name string
		more []chatlog.Event
		want func(Plan) bool
	}{
		{"the recorded plan, approved", []chatlog.Event{recorded}, func(p Plan) bool { return strings.HasPrefix(p.Text, "# Plan: add made.txt\n") && !p.Waiting }},
		{"a later plan", []chatlog.Event{recorded, plan("plan-2", "# Second"), answer("plan-2", "allow")}, func(p Plan) bool { return p == Plan{Text: "# Second"} }},
		{"a subagent's plan is left out", []chatlog.Event{recorded, plan("plan-2", "# Second"), answer("plan-2", "allow"), subagents, answer("plan-s", "allow")}, func(p Plan) bool { return p == Plan{Text: "# Second"} }},
		{"a denied plan is left out", []chatlog.Event{recorded, plan("plan-2", "# Second"), answer("plan-2", "allow"), plan("plan-3", "# Denied"), answer("plan-3", "deny")}, func(p Plan) bool { return p == Plan{Text: "# Second"} }},
		{"an input that does not parse is left out", []chatlog.Event{recorded, plan("plan-2", "# Second"), answer("plan-2", "allow"), {Kind: chatlog.KindApproval, ID: "plan-4", Tool: "ExitPlanMode", Input: json.RawMessage(`{"plan":7}`)}}, func(p Plan) bool { return p.Text == "# Second" }},
		{"a plan that waits says so", []chatlog.Event{recorded, plan("plan-5", "# Asked")}, func(p Plan) bool { return p == Plan{Text: "# Asked", Waiting: true} }},
		{"a denied request id asked again", []chatlog.Event{recorded, plan("plan-6", "# No"), answer("plan-6", "deny"), plan("plan-6", "# Yes"), answer("plan-6", "allow")}, func(p Plan) bool { return p == Plan{Text: "# Yes"} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := readService(t, t.TempDir(), "", logOf(t, "planexit.jsonl", c.more...))
			if got, err := svc.ChatLatestPlan("main/abcdefgh"); err != nil || !c.want(got) {
				t.Errorf("ChatLatestPlan = %+.60v, %v", got, err)
			}
		})
	}
	svc := readService(t, t.TempDir(), "", filepath.Join(t.TempDir(), "none.jsonl"))
	if got, err := svc.ChatLatestPlan("main/abcdefgh"); err != nil || got != (Plan{}) {
		t.Errorf("ChatLatestPlan of a chat with no log = %+v, %v; want none", got, err)
	}
}

func TestLatestPlanIsReadAgainOnlyOnceTheLogChanges(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "chat.jsonl")
	l, err := chatlog.OpenLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if _, err := l.Append(chatlog.Event{Kind: chatlog.KindApproval, ID: "p1", Tool: "ExitPlanMode", Input: planInput(t, "# One")}); err != nil {
		t.Fatal(err)
	}
	svc := readService(t, t.TempDir(), "", path)
	if got, _ := svc.ChatLatestPlan("main/abcdefgh"); got.Text != "# One" {
		t.Fatalf("ChatLatestPlan = %+v, want # One", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "# One", "# Two", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.ChatLatestPlan("main/abcdefgh"); got.Text != "# One" {
		t.Errorf("ChatLatestPlan of a log of the same size and time = %+v, want the plan read before", got)
	}

	if _, err := l.Append(chatlog.Event{Kind: chatlog.KindApproval, ID: "p3", Tool: "ExitPlanMode", Input: planInput(t, "# Three")}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.ChatLatestPlan("main/abcdefgh"); got.Text != "# Three" {
		t.Errorf("ChatLatestPlan of a grown log = %+v, want # Three", got)
	}
}
