package threads

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/thread"
	"github.com/nednella/orchestrator/internal/util"
)

func systemPrompt(t *testing.T, access thread.Access, tree workTree) string {
	t.Helper()
	argv := agent.Claude{}.Command(launchFor(project.Model{}, "", "", "", access, false, forkStart{}, tree, "", 0))
	i := slices.Index(argv, "--append-system-prompt")
	if i < 0 {
		t.Fatalf("argv = %q", argv)
	}
	return argv[i+1]
}

func TestWorktreeThreadIsToldItsBranchAndFolder(t *testing.T) {
	t.Parallel()
	got := systemPrompt(t, thread.Supervised, workTree{branch: "issue-7", dir: "/p/trees/issue-7"})
	if !strings.Contains(got, "`/p/trees/issue-7`") || !strings.Contains(got, "`issue-7`") {
		t.Errorf("system prompt lacks the worktree and branch:\n%s", got)
	}
	if want := prompts.Base() + "\n\n" + prompts.Worktree("issue-7", "/p/trees/issue-7"); !strings.Contains(got, want) {
		t.Errorf("the worktree text does not follow the thread text:\n%s", got)
	}
}

func TestProjectFolderThreadGetsNoWorktreeText(t *testing.T) {
	t.Parallel()
	got := systemPrompt(t, thread.Supervised, workTree{})
	if strings.Contains(got, "This thread runs in the worktree") {
		t.Errorf("system prompt holds the worktree text:\n%s", got)
	}
}

func TestOnlyAReadThreadGetsTheReadText(t *testing.T) {
	t.Parallel()
	tree := workTree{branch: "issue-7", dir: "/p/trees/issue-7"}
	if got := systemPrompt(t, thread.Read, tree); !strings.Contains(got, prompts.Base()+"\n\n"+prompts.Read()) {
		t.Errorf("a Read thread lacks the Read text:\n%s", got)
	}
	for _, write := range []thread.Access{thread.Supervised, thread.Full} {
		if got := systemPrompt(t, write, tree); strings.Contains(got, prompts.Read()) {
			t.Errorf("a %s thread gets the Read text:\n%s", write, got)
		}
	}
}

func TestBranchFromTitle(t *testing.T) {
	t.Parallel()
	for title, want := range map[string]string{
		"Fix the login button!":        "fix-the-login-button",
		"  --Wrap: (it) up, please.. ": "wrap-it-up-please",
		"Café ünïcode ✓ résumé":        "caf-n-code-r-sum",
		"日本語":                          "thread",
		strings.Repeat("abcde ", 10):   "abcde-abcde-abcde-abcde-abcde-abcde",
		strings.Repeat("x", 60):        strings.Repeat("x", 40),
		"":                             "thread",
	} {
		if got := branchFromTitle(title); got != want {
			t.Errorf("branchFromTitle(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestPlaceFor(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		setting Place
		issue   int
		picked  Place
		want    Place
	}{
		{"", 0, "", InWorktree},
		{InWorktree, 0, "", InWorktree},
		{InProject, 0, "", InProject},
		{InProject, 7, "", InWorktree},
		{"", 0, InProject, InProject},
		{InProject, 0, InWorktree, InWorktree},
	} {
		if got := placeFor(tt.setting, tt.issue, tt.picked); got != tt.want {
			t.Errorf("placeFor(%q, %d, %q) = %q, want %q", tt.setting, tt.issue, tt.picked, got, tt.want)
		}
	}
}

func TestProjectCommandQuotesItsPlaceholders(t *testing.T) {
	t.Parallel()
	l, _, _ := madeRepo(t)
	dir := filepath.Join(t.TempDir(), "my 'project'")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := l.projectCommand(t.Context(), project.Project{Dir: dir}, `printf '%s\n' {dir} {branch}`, "{branch}", util.ShellQuote("a $b"))
	if want := dir + "\na $b\n"; err != nil || string(out) != want {
		t.Errorf("output = %q, %v; want %q", out, err, want)
	}
}
