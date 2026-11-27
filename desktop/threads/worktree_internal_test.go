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

func systemPrompt(t *testing.T, access thread.Access) string {
	t.Helper()
	argv := agent.Claude{}.Command(launchFor(project.Model{}, "", "", "", access, false, forkStart{}, "", 0))
	i := slices.Index(argv, "--append-system-prompt")
	if i < 0 {
		t.Fatalf("argv = %q", argv)
	}
	return argv[i+1]
}

func TestOnlyAReadThreadGetsTheReadText(t *testing.T) {
	t.Parallel()
	if got := systemPrompt(t, thread.Read); got != prompts.Base()+"\n\n"+prompts.Read() {
		t.Errorf("a Read thread's prompt:\n%s", got)
	}
	for _, write := range []thread.Access{thread.Supervised, thread.Full} {
		if got := systemPrompt(t, write); got != prompts.Base() {
			t.Errorf("a %s thread's prompt:\n%s", write, got)
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
