package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	chatlog "github.com/nednella/agentos/internal/chat"
)

func commandsAt(at int64, names ...string) chatlog.Event {
	var list []Command
	for _, n := range names {
		list = append(list, Command{Name: n, Description: n + " it"})
	}
	input, _ := json.Marshal(list)
	return chatlog.Event{At: at, Kind: chatlog.KindCommands, Input: input}
}

func names(t *testing.T, c *commandLists, project string) []string {
	t.Helper()
	list, err := c.list(project)
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Fatal("list is nil, want a list")
	}
	var out []string
	for _, cmd := range list {
		out = append(out, cmd.Name)
	}
	return out
}

func TestCommandListsKeepTheLatestPerProject(t *testing.T) {
	dir := t.TempDir()
	c := newCommandLists(dir)
	if got := names(t, c, "main"); len(got) != 0 {
		t.Errorf("before any thread = %q, want none", got)
	}

	c.keep("main", commandsAt(2000, "review", "init"))
	c.keep("main", commandsAt(1000, "old"))
	c.keep("other", commandsAt(500, "deploy"))
	if got := names(t, c, "main"); !slices.Equal(got, []string{"review", "init"}) {
		t.Errorf("main = %q, want the list reported last, not the older one read after it", got)
	}
	if got := names(t, c, "other"); !slices.Equal(got, []string{"deploy"}) {
		t.Errorf("other = %q, want its own list", got)
	}

	reopened := newCommandLists(dir)
	reopened.keep("main", commandsAt(1500, "older"))
	if got := names(t, reopened, "main"); !slices.Equal(got, []string{"review", "init"}) {
		t.Errorf("after a restart main = %q, want the saved later list kept", got)
	}
	reopened.keep("main", commandsAt(3000, "compact"))
	if got := names(t, reopened, "main"); !slices.Equal(got, []string{"compact"}) {
		t.Errorf("main = %q, want a newer list to replace it", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "main", "commands.json")); err != nil {
		t.Errorf("the list is not in the project's folder: %v", err)
	}
}

func TestProjectCommandsNeedsAKnownProject(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(New(folderThread(dir), dir, func(string, any) {}), nil, func(key string) string {
		if key == "main" {
			return dir
		}
		return ""
	}, nil)
	got, err := svc.ProjectCommands("main")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("ProjectCommands(main) = %v, %v; want an empty list", got, err)
	}
	if _, err := svc.ProjectCommands("../main"); err == nil {
		t.Error("ProjectCommands of a project that is not configured gave no error")
	}
}

func TestTheSameCommandListIsNotWrittenAgain(t *testing.T) {
	dir := t.TempDir()
	c := newCommandLists(dir)
	c.keep("main", commandsAt(1000, "review"))
	path := filepath.Join(dir, "main", "commands.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c.keep("main", commandsAt(2000, "review"))
	c.keep("main", commandsAt(1500, "older"))
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("the file = %s, want it untouched: %s", after, before)
	}
	if got := names(t, c, "main"); !slices.Equal(got, []string{"review"}) {
		t.Errorf("main = %q, want the list reported at 2000 to outrank the one at 1500", got)
	}
}
