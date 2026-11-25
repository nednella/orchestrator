package sessions_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/prompts"
)

// panePath is the folder the thread's tmux session started in.
func panePath(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{session_path}").Output()
	if err != nil {
		t.Fatalf("reading the folder of %s: %v", id, err)
	}
	return strings.TrimSpace(string(out))
}

// realDir is path with its links resolved.
func realDir(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

// madeRecords are the main project's records of the worktrees agentos made.
func madeRecords(t *testing.T, h *apptest.Harness) []sessions.Made {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.State, "data", "main", "worktrees.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var list []sessions.Made
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatal(err)
	}
	return list
}

// worktreesOf are the worktrees git lists for the repo at dir, by branch, their folders with links resolved.
func worktreesOf(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	path := ""
	for line := range strings.Lines(gitIn(t, dir, "worktree", "list", "--porcelain")) {
		line = strings.TrimSpace(line)
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			path = realDir(t, p)
		}
		if b, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
			out[b] = path
		}
	}
	return out
}

// systemPrompt is the thread's --append-system-prompt and the arguments after it, as the stand-in claude got them.
func systemPrompt(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	args := claudeArgs(t, h, id)
	i := slices.Index(args, "--append-system-prompt")
	if i < 0 {
		t.Fatalf("args = %q", args)
	}
	return strings.Join(args[i+1:], "\n")
}

const worktreeText = "agentos made this thread's worktree"

func TestWriteThreadStartsInAWorktreeAgentosMade(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s, err := h.NewThreadIn("Fix the login button", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	branch, dir := "fix-the-login-button", realDir(t, filepath.Join(h.Dir, "trees", "fix-the-login-button"))
	if got := realDir(t, panePath(t, h, s.ID)); got != dir {
		t.Errorf("the thread runs in %s, want %s", got, dir)
	}
	if got := worktreesOf(t, h.Dir)[branch]; got != dir {
		t.Errorf("git lists the worktree of %s at %q, want %s", branch, got, dir)
	}
	base := gitIn(t, h.Dir, "rev-parse", "origin/main")
	if head := gitIn(t, dir, "rev-parse", "HEAD"); head != base {
		t.Errorf("the worktree is at %s, want origin/main at %s", head, base)
	}
	records := madeRecords(t, h)
	if len(records) != 1 || records[0].Path != dir || records[0].Branch != branch || records[0].Base != "origin/main" || records[0].BaseOid != base || records[0].Thread != s.ID || records[0].Created == 0 || records[0].Removed != 0 {
		t.Errorf("records = %+v", records)
	}
	gitDir := gitIn(t, dir, "rev-parse", "--path-format=absolute", "--git-dir")
	if marker, err := os.ReadFile(filepath.Join(gitDir, "agentos-thread")); err != nil || strings.TrimSpace(string(marker)) != s.ID {
		t.Errorf("marker = %q, %v; want %s", marker, err, s.ID)
	}
	if s.Branch != branch || s.Worktree != dir {
		t.Errorf("row branch %q, worktree %q", s.Branch, s.Worktree)
	}
	if got := systemPrompt(t, h, s.ID); !strings.Contains(got, prompts.Worktree(branch, dir)) {
		t.Errorf("the system prompt does not name the worktree and branch:\n%s", got)
	}
}

func TestReadThreadStartsInTheProjectFolder(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s, err := h.NewThreadIn("Look around", "", "read", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := realDir(t, panePath(t, h, s.ID)); got != realDir(t, h.Dir) {
		t.Errorf("the thread runs in %s, want the project folder", got)
	}
	if trees := worktreesOf(t, h.Dir); len(trees) != 1 || exists(filepath.Join(h.Dir, "trees")) {
		t.Errorf("worktrees = %v", trees)
	}
	if records := madeRecords(t, h); len(records) != 0 {
		t.Errorf("records = %+v", records)
	}
	if s.Branch != "" || s.Worktree != "" {
		t.Errorf("row branch %q, worktree %q", s.Branch, s.Worktree)
	}
	if got := systemPrompt(t, h, s.ID); strings.Contains(got, worktreeText) {
		t.Errorf("the system prompt names a worktree:\n%s", got)
	}
}

func TestIssueThreadGetsTheIssueBranch(t *testing.T) {
	for _, action := range []string{"Work", "Plan"} {
		t.Run(action, func(t *testing.T) {
			h := newHarness(t)
			repoFixture(t, h)
			s, err := h.StartIssueWith(7, action)
			if err != nil {
				t.Fatal(err)
			}
			dir := realDir(t, filepath.Join(h.Dir, "trees", "issue-7"))
			if s.Branch != "issue-7" || s.Worktree != dir {
				t.Errorf("%s thread (%s): branch %q, worktree %q", action, s.Access, s.Branch, s.Worktree)
			}
			if got := realDir(t, panePath(t, h, s.ID)); got != dir {
				t.Errorf("the thread runs in %s, want %s", got, dir)
			}
			if got := worktreesOf(t, h.Dir)["issue-7"]; got != dir {
				t.Errorf("git lists issue-7 at %q", got)
			}
			if got := systemPrompt(t, h, s.ID); !strings.Contains(got, prompts.Worktree("issue-7", dir)) {
				t.Errorf("the system prompt does not name the worktree and branch:\n%s", got)
			}
		})
	}
}

func TestThreadBranchIsNamedFromItsTitle(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	first, err := h.NewThreadIn("Fix the login button!", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.NewThreadIn("Fix the login button!", "", "full", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.Branch != "fix-the-login-button" || second.Branch != "fix-the-login-button-2" {
		t.Errorf("branches %q and %q", first.Branch, second.Branch)
	}
	if want := realDir(t, filepath.Join(h.Dir, "trees", "fix-the-login-button-2")); second.Worktree != want {
		t.Errorf("second worktree %q, want %s", second.Worktree, want)
	}
}

func TestWorktreeCommandGetsItsPlaceholders(t *testing.T) {
	got := filepath.Join(t.TempDir(), "placeholders")
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "printf '%s\\n' {branch} {worktree} {dir} {base} > " + got + " && git worktree add -b {branch} {worktree} {base}"})
	repoFixture(t, h)
	s, err := h.NewThreadIn("Tidy up", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(got)
	if want := strings.Join([]string{"tidy-up", filepath.Join(h.Dir, "trees", "tidy-up"), h.Dir, "origin/main"}, "\n") + "\n"; err != nil || string(data) != want {
		t.Errorf("the command got %q, %v; want %q", data, err, want)
	}
	if want := realDir(t, filepath.Join(h.Dir, "trees", "tidy-up")); s.Worktree != want {
		t.Errorf("worktree %q, want %s", s.Worktree, want)
	}
}

func TestWorktreeMadeElsewhereByTheCommandIsRecorded(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{WorktreeCommand: "git worktree add -b {branch} {dir}/elsewhere {base}"})
	repoFixture(t, h)
	s, err := h.NewThreadIn("Tidy up", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := realDir(t, filepath.Join(h.Dir, "elsewhere"))
	if got := realDir(t, panePath(t, h, s.ID)); got != dir {
		t.Errorf("the thread runs in %s, want %s", got, dir)
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Path != dir || records[0].Branch != "tidy-up" {
		t.Errorf("records = %+v", records)
	}
	if s.Worktree != dir || exists(filepath.Join(h.Dir, "trees", "tidy-up")) {
		t.Errorf("worktree %q; trees/tidy-up exists: %v", s.Worktree, exists(filepath.Join(h.Dir, "trees", "tidy-up")))
	}
}

func TestFailedWorktreeCommandStartsNoThread(t *testing.T) {
	for _, tt := range []struct{ command, says string }{
		{"echo no room on the disk >&2; exit 3", "session_worktree_command failed for tidy-up: sh -c: exit status 3: no room on the disk"},
		{"echo all done", "session_worktree_command made no worktree for tidy-up: all done"},
	} {
		t.Run(tt.command, func(t *testing.T) {
			h := apptest.NewWith(t, apptest.Options{WorktreeCommand: tt.command})
			repoFixture(t, h)
			if _, err := h.NewThreadIn("Tidy up", "", "supervised", ""); err == nil || err.Error() != tt.says {
				t.Errorf("start = %v, want %q", err, tt.says)
			}
			if out, _ := exec.Command("tmux", "-L", h.Socket, "list-sessions").CombinedOutput(); strings.Contains(string(out), "main/") {
				t.Errorf("a thread runs:\n%s", out)
			}
			if records := madeRecords(t, h); len(records) != 0 {
				t.Errorf("records = %+v", records)
			}
			if rows := h.Sessions().List(); len(rows) != 0 {
				t.Errorf("rows = %+v", rows)
			}
		})
	}
}

func TestFailedStartRemovesOnlyTheWorktreeItMade(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	human := filepath.Join(h.Dir, "trees", "human")
	gitIn(t, h.Dir, "worktree", "add", "-b", "human", human)
	if err := os.WriteFile(filepath.Join(human, "notes.txt"), []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	newFakeTmux(t).fail(t, "new", true)

	if _, err := h.NewThreadIn("Doomed", "", "supervised", ""); err == nil || !strings.Contains(err.Error(), "no room for a session") {
		t.Fatalf("start = %v", err)
	}
	if exists(filepath.Join(h.Dir, "trees", "doomed")) {
		t.Error("the worktree of the thread that did not start is still there")
	}
	if _, listed := worktreesOf(t, h.Dir)["doomed"]; listed {
		t.Error("git still lists the worktree of the thread that did not start")
	}
	if data, err := os.ReadFile(filepath.Join(human, "notes.txt")); err != nil || string(data) != "mine\n" {
		t.Errorf("the human's worktree lost its file: %q, %v", data, err)
	}
	if got := worktreesOf(t, h.Dir)["human"]; got != realDir(t, human) {
		t.Errorf("git lists the human's worktree at %q", got)
	}
	if !exists(filepath.Join(h.Dir, "README")) || !exists(filepath.Join(h.Dir, ".git")) {
		t.Error("the project folder lost its files")
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Branch != "doomed" || records[0].Removed == 0 {
		t.Errorf("records = %+v", records)
	}
}

func TestExistingBranchIsCheckedOutNotReset(t *testing.T) {
	for _, where := range []string{"local", "only on origin"} {
		t.Run(where, func(t *testing.T) {
			h := newHarness(t)
			repoFixture(t, h)
			elsewhere := filepath.Join(t.TempDir(), "issue-7")
			gitIn(t, h.Dir, "worktree", "add", "-b", "issue-7", elsewhere)
			pushWork(t, elsewhere)
			gitIn(t, h.Dir, "worktree", "remove", elsewhere)
			head := gitIn(t, h.Dir, "rev-parse", "issue-7")
			if where == "only on origin" {
				gitIn(t, h.Dir, "branch", "-D", "issue-7")
			}

			s, err := h.StartIssue(7)
			if err != nil {
				t.Fatal(err)
			}
			dir := realDir(t, filepath.Join(h.Dir, "trees", "issue-7"))
			if got := gitIn(t, dir, "rev-parse", "HEAD"); got != head || got == gitIn(t, h.Dir, "rev-parse", "origin/main") {
				t.Errorf("the worktree is at %s, want the branch's own commit %s", got, head)
			}
			if got := gitIn(t, dir, "branch", "--show-current"); got != "issue-7" || !exists(filepath.Join(dir, "work.txt")) {
				t.Errorf("the worktree is on %q; work.txt there: %v", got, exists(filepath.Join(dir, "work.txt")))
			}
			if records := madeRecords(t, h); len(records) != 1 || records[0].Path != dir || s.Worktree != dir {
				t.Errorf("records = %+v, row worktree %q", records, s.Worktree)
			}
		})
	}
}

func TestBranchOfAnotherThreadIsRefused(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	other, err := h.NewThreadIn("Issue 7", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	if other.Branch != "issue-7" {
		t.Fatalf("the first thread is on %q", other.Branch)
	}
	if err := h.SettleThread(other.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.StartIssue(7); err == nil || err.Error() != fmt.Sprintf("Thread %d works on issue-7", other.N) {
		t.Errorf("start = %v", err)
	}
	if records := madeRecords(t, h); len(records) != 1 {
		t.Errorf("records = %+v", records)
	}
}

func TestBranchCheckedOutByAHumanRunsThereUnrecorded(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	human := filepath.Join(h.Dir, "trees", "issue-7")
	gitIn(t, h.Dir, "worktree", "add", "-b", "issue-7", human)

	s, err := h.StartIssue(7)
	if err != nil {
		t.Fatal(err)
	}
	if got := realDir(t, panePath(t, h, s.ID)); got != realDir(t, human) {
		t.Errorf("the thread runs in %s, want the human's worktree %s", got, human)
	}
	if records := madeRecords(t, h); len(records) != 0 {
		t.Errorf("records = %+v", records)
	}
	if s.Branch != "issue-7" || s.Worktree != "" {
		t.Errorf("row branch %q, worktree %q", s.Branch, s.Worktree)
	}
}

func TestNotAGitRepoRunsInTheProjectFolder(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewThreadIn("Write it", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := realDir(t, panePath(t, h, s.ID)); got != realDir(t, h.Dir) {
		t.Errorf("the thread runs in %s, want the project folder", got)
	}
	if s.Branch != "" || s.Worktree != "" || exists(filepath.Join(h.Dir, "trees")) || len(madeRecords(t, h)) != 0 {
		t.Errorf("row branch %q, worktree %q; records %+v", s.Branch, s.Worktree, madeRecords(t, h))
	}
}

func TestForkStartsInTheSourcesWorktree(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	src, err := h.NewThreadIn("Source", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)

	fork, err := h.ForkSession(src.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	if got := realDir(t, panePath(t, h, fork.ID)); got != src.Worktree {
		t.Errorf("the fork runs in %s, want the source's worktree %s", got, src.Worktree)
	}
	if fork.Branch != "" || fork.Worktree != "" {
		t.Errorf("fork branch %q, worktree %q", fork.Branch, fork.Worktree)
	}
	if trees, records := worktreesOf(t, h.Dir), madeRecords(t, h); len(trees) != 2 || len(records) != 1 {
		t.Errorf("worktrees %v, records %+v", trees, records)
	}
}

func TestWorktreeKeepsAfterARestart(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	live := issueThread(h, t, 7)
	settled, err := h.NewThreadIn("Settle me", "", "supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(settled.ID); err != nil {
		t.Fatal(err)
	}

	fresh := h.Restart(t)
	for _, want := range []sessions.Session{live, settled} {
		got, ok := fresh.Session(want.ID)
		if !ok || got.Branch != want.Branch || got.Worktree != want.Worktree || got.Branch == "" || got.Worktree == "" {
			t.Errorf("after a restart %s has branch %q, worktree %q; want %q, %q", want.ID, got.Branch, got.Worktree, want.Branch, want.Worktree)
		}
	}
	if rec, err := bus.ReadState(h.State, settled.ID); err != nil || rec.Branch != "settle-me" {
		t.Errorf("the settled thread's state file has branch %q, %v", rec.Branch, err)
	}
}
