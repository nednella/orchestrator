package sessions

import (
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/internal/agent"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/prompts"
)

func systemPrompt(t *testing.T, chat bool, tree workTree) string {
	t.Helper()
	argv := agent.Claude{Exe: "/x"}.Command(launchFor(project.Model{}, "", "", "", chat, false, forkStart{}, tree))
	i := slices.Index(argv, "--append-system-prompt")
	if i < 0 {
		t.Fatalf("argv = %q", argv)
	}
	return argv[i+1]
}

func TestWorktreeThreadIsToldItsBranchAndFolder(t *testing.T) {
	got := systemPrompt(t, false, workTree{branch: "issue-7", dir: "/p/trees/issue-7"})
	if !strings.Contains(got, "`/p/trees/issue-7`") || !strings.Contains(got, "`issue-7`") {
		t.Errorf("system prompt lacks the worktree and branch:\n%s", got)
	}
	if want := prompts.Thread() + "\n\n" + prompts.Worktree("issue-7", "/p/trees/issue-7"); !strings.Contains(got, want) {
		t.Errorf("the worktree text does not follow the thread text:\n%s", got)
	}
}

func TestProjectFolderThreadGetsNoWorktreeText(t *testing.T) {
	got := systemPrompt(t, false, workTree{})
	if strings.Contains(got, "agentos made this thread's worktree") {
		t.Errorf("system prompt holds the worktree text:\n%s", got)
	}
}

func TestEveryAccessGetsTheSamePromptInAWorktree(t *testing.T) {
	tree := workTree{branch: "issue-7", dir: "/p/trees/issue-7"}
	if read, write := systemPrompt(t, true, tree), systemPrompt(t, false, tree); read != write {
		t.Errorf("a Read and a Write thread get different prompts:\n%s\n---\n%s", read, write)
	}
}
