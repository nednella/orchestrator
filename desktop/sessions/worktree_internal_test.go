package sessions

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/internal/agent"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/prompts"
	"github.com/nednella/agentos/internal/session"
	"github.com/nednella/agentos/internal/util"
)

func systemPrompt(t *testing.T, access session.Access, tree workTree) string {
	t.Helper()
	argv := agent.Claude{Exe: "/x"}.Command(launchFor(project.Model{}, "", "", "", access, false, forkStart{}, tree, ""))
	i := slices.Index(argv, "--append-system-prompt")
	if i < 0 {
		t.Fatalf("argv = %q", argv)
	}
	return argv[i+1]
}

func TestWorktreeThreadIsToldItsBranchAndFolder(t *testing.T) {
	got := systemPrompt(t, session.Supervised, workTree{branch: "issue-7", dir: "/p/trees/issue-7"})
	if !strings.Contains(got, "`/p/trees/issue-7`") || !strings.Contains(got, "`issue-7`") {
		t.Errorf("system prompt lacks the worktree and branch:\n%s", got)
	}
	if want := prompts.Thread() + "\n\n" + prompts.Worktree("issue-7", "/p/trees/issue-7"); !strings.Contains(got, want) {
		t.Errorf("the worktree text does not follow the thread text:\n%s", got)
	}
}

func TestProjectFolderThreadGetsNoWorktreeText(t *testing.T) {
	got := systemPrompt(t, session.Supervised, workTree{})
	if strings.Contains(got, "This thread runs in the worktree") {
		t.Errorf("system prompt holds the worktree text:\n%s", got)
	}
}

func TestEveryAccessGetsTheSamePromptInAWorktree(t *testing.T) {
	tree := workTree{branch: "issue-7", dir: "/p/trees/issue-7"}
	read := systemPrompt(t, session.Read, tree)
	for _, write := range []session.Access{session.Supervised, session.Full} {
		if got := systemPrompt(t, write, tree); got != read {
			t.Errorf("a Read and a %s thread get different prompts:\n%s\n---\n%s", write, read, got)
		}
	}
}

func TestBranchFromTitle(t *testing.T) {
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
	for _, tt := range []struct {
		access session.Access
		issue  int
		picked Place
		want   Place
	}{
		{session.Read, 0, "", InProject},
		{session.Supervised, 0, "", InWorktree},
		{session.Full, 0, "", InWorktree},
		{session.Read, 7, "", InWorktree},
		{session.Supervised, 0, InProject, InProject},
		{session.Read, 0, InWorktree, InWorktree},
	} {
		if got := placeFor(tt.access, tt.issue, tt.picked); got != tt.want {
			t.Errorf("placeFor(%s, %d, %q) = %q, want %q", tt.access, tt.issue, tt.picked, got, tt.want)
		}
	}
}

func TestProjectCommandQuotesItsPlaceholders(t *testing.T) {
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
