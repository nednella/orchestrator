package sessions_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/bus"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/mcp"
)

func sessionWithoutIssue(h *apptest.Harness, t *testing.T) sessions.Session {
	t.Helper()
	s, err := h.NewThreadIn("my work", "", "supervised", "project")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// worksIn tells the app, as the agent's hook does, that the session is working in dir.
func worksIn(h *apptest.Harness, t *testing.T, s sessions.Session, dir string) {
	t.Helper()
	cwd, _ := json.Marshal(map[string]string{"cwd": dir})
	h.Observe(t, s.ID, "Stop", string(cwd))
}

// inToolCall runs do between the PreToolUse and PostToolUse of one of the session's tool calls in dir.
func inToolCall(h *apptest.Harness, t *testing.T, s sessions.Session, dir string, do func()) {
	t.Helper()
	cwd, _ := json.Marshal(map[string]string{"cwd": dir})
	h.Observe(t, s.ID, "PreToolUse", string(cwd))
	eventually(t, "the tool call", func() bool { got, _ := h.Session(s.ID); return got.State == "working" })
	do()
	h.Observe(t, s.ID, "PostToolUse", string(cwd))
}

// worktreeThread starts a Supervised thread with no issue in the worktree orchestrator makes for it, on my-work.
func worktreeThread(h *apptest.Harness, t *testing.T) sessions.Session {
	t.Helper()
	s, err := h.NewThreadIn("my work", "", "supervised", "worktree")
	if err != nil {
		t.Fatal(err)
	}
	if s.Branch != "my-work" || s.Worktree == "" {
		t.Fatalf("thread = %+v, want it in its worktree on my-work", s)
	}
	return s
}

// ownBranch checks out my-fix in dir with one pushed commit.
func ownBranch(t *testing.T, dir string) {
	t.Helper()
	gitIn(t, dir, "checkout", "-b", "my-fix")
	if err := os.WriteFile(filepath.Join(dir, "fix.txt"), []byte("fix\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "fix.txt")
	gitIn(t, dir, "commit", "-m", "fix")
	gitIn(t, dir, "push", "-u", "origin", "my-fix")
}

func hasPR(h *apptest.Harness, id string) func() bool {
	return func() bool { got, _ := h.Session(id); return got.PR != nil }
}

func TestPRIsFoundWhenTheTurnEnds(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := worktreeThread(h, t)
	worksIn(h, t, s, s.Worktree)
	time.Sleep(300 * time.Millisecond)
	if got, _ := h.Session(s.ID); got.PR != nil {
		t.Fatalf("a PR before there is one: %+v", got.PR)
	}

	h.GH.SetPR(prJSON("OPEN", true, "[]", 0, 0))
	worksIn(h, t, s, s.Worktree)
	eventually(t, "the PR the agent opened during the turn", hasPR(h, s.ID))
}

func TestIdleSessionLooksUpItsPROncePerTurn(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := worktreeThread(h, t)
	worksIn(h, t, s, s.Worktree)
	eventually(t, "the lookup of the turn", func() bool { return h.GH.Calls("pr list") > 0 })
	time.Sleep(300 * time.Millisecond)

	before := h.GH.Calls("pr list")
	for range 3 {
		h.Sessions().Refresh()
	}
	time.Sleep(300 * time.Millisecond)
	if got := h.GH.Calls("pr list"); got != before {
		t.Errorf("pr lookups after refreshes of an idle session = %d, want %d", got, before)
	}
}

func TestSessionReportsItsBranch(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	ownBranch(t, h.Dir)
	gitIn(t, h.Dir, "checkout", "main")
	s := sessionWithoutIssue(h, t)
	h.GH.SetPR(prJSON("OPEN", true, "[]", 0, 0))

	if resp := h.AskTool(t, ctl.Request{Cmd: "track", Session: s.ID}); resp.OK {
		t.Error("track without a branch was accepted")
	}
	if resp := h.AskTool(t, ctl.Request{Cmd: "track", Opts: map[string]string{"branch": "my-fix"}}); resp.OK {
		t.Error("track outside a session was accepted")
	}
	if resp := h.AskTool(t, ctl.Request{Cmd: "track", Session: s.ID, Opts: map[string]string{"branch": "main"}}); resp.OK {
		t.Error("track of the default branch was accepted")
	}
	resp := h.AskTool(t, ctl.Request{Cmd: "track", Session: s.ID, Opts: map[string]string{"branch": "my-fix"}})
	if !resp.OK {
		t.Fatalf("track: %s", resp.Error)
	}
	eventually(t, "the PR of the reported branch", hasPR(h, s.ID))
	if got, _ := h.Session(s.ID); got.Branch != "my-fix" {
		t.Errorf("session = %+v", got)
	}
}

func TestNoIssueInTheEnvironment(t *testing.T) {
	h := newHarness(t)
	s := issueSession(h, t, 9)
	env := claudeEnv(t, h, s.ID)
	// Another app's thread that started this one would have given it its own, which the app drops.
	if v, ok := env["ORCHESTRATOR_ISSUE"]; ok {
		t.Errorf("claude was started with ORCHESTRATOR_ISSUE=%s", v)
	}
	if want := filepath.Join(h.State, "control.sock"); env["ORCHESTRATOR_SESSION"] != s.ID || env[mcp.ControlEnv] != want {
		t.Errorf("claude was started with ORCHESTRATOR_SESSION=%s and %s=%s, want %s and %s", env["ORCHESTRATOR_SESSION"], mcp.ControlEnv, env[mcp.ControlEnv], s.ID, want)
	}
}

// promptOf is the system prompt claude was started with.
func promptOf(t *testing.T, args []string) string {
	t.Helper()
	i := slices.Index(args, "--append-system-prompt")
	if i < 0 {
		t.Fatalf("claude has no system prompt: %q", args)
	}
	return strings.Join(args[i+1:], "\n")
}

func TestIssueThreadIsToldItsIssue(t *testing.T) {
	h := newHarness(t)
	s := issueSession(h, t, 42)
	const told = "This thread works on GitHub issue #42."
	if prompt := promptOf(t, claudeArgs(t, h, s.ID)); !strings.Contains(prompt, told) || strings.Index(prompt, told) < strings.Index(prompt, "Stay on this thread's topic.") {
		t.Errorf("the prompt does not tell the issue after the thread's text: %q", prompt)
	}

	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to sleep", func() bool {
		ch, err := h.Sessions().ChatOf(s.ID)
		return err == nil && !ch.Live
	})
	if err := os.Remove(filepath.Join(h.ClaudeDir(), strings.ReplaceAll(s.ID, "/", "_")+".args")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}
	if prompt := promptOf(t, claudeArgs(t, h, s.ID)); !strings.Contains(prompt, told) {
		t.Errorf("after a wake the prompt does not tell the issue: %q", prompt)
	}

	plain, err := h.NewSession("no issue", "")
	if err != nil {
		t.Fatal(err)
	}
	if prompt := promptOf(t, claudeArgs(t, h, plain.ID)); strings.Contains(prompt, "works on GitHub issue") {
		t.Errorf("a thread with no issue is told of one: %q", prompt)
	}
}

func TestThreadBranchComesFromItsRecord(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	other := filepath.Join(t.TempDir(), "other")
	gitIn(t, filepath.Dir(other), "init", other)
	commitIn(t, other, "a.txt")
	gitIn(t, other, "checkout", "-b", "theirs")
	h.GH.SetPRFor("issue-7", "[]")
	h.GH.SetPRFor("theirs", numberedPR(20, "OPEN"))
	h.GH.SetPRFor("my-fix", numberedPR(21, "OPEN"))
	s := issueThread(h, t, 7)

	worksIn(h, t, s, other)
	inToolCall(h, t, s, h.Dir, func() { ownBranch(t, h.Dir) })
	worksIn(h, t, s, h.Dir)
	h.RefreshPRs()
	time.Sleep(300 * time.Millisecond)
	if got, _ := h.Session(s.ID); got.Branch != "issue-7" || got.PR != nil {
		t.Errorf("row = %+v, want issue-7 and no PR", got)
	}
	if rec, err := bus.ReadState(h.State, s.ID); err != nil || rec.Branch != "issue-7" {
		t.Errorf("state file branch = %q, %v", rec.Branch, err)
	}
	if n := h.GH.Calls("pr list --head theirs") + h.GH.Calls("pr list --head my-fix"); n != 0 {
		t.Errorf("the PRs of branches the thread only ran on were looked up %d times", n)
	}
}

// branchTag is the branch tmux keeps for the thread.
func branchTag(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{@orchestrator-branch}").Output()
	if err != nil {
		t.Fatalf("reading the branch tag of %s: %v", id, err)
	}
	return strings.TrimSpace(string(out))
}

func TestTrackMovesTheThreadsBranch(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	gitIn(t, s.Worktree, "checkout", "-b", "issue-7-fix")
	h.GH.SetPRFor("issue-7", "[]")
	h.GH.SetPRFor("issue-7-fix", numberedPR(20, "OPEN"))

	if resp := h.AskTool(t, ctl.Request{Cmd: "track", Session: s.ID, Opts: map[string]string{"branch": "issue-7-fix"}}); !resp.OK {
		t.Fatalf("track: %s", resp.Error)
	}
	eventually(t, "the PR of the tracked branch", func() bool { got, _ := h.Session(s.ID); return got.PR != nil && got.PR.Number == 20 })
	if got, _ := h.Session(s.ID); got.Branch != "issue-7-fix" || got.Worktree != s.Worktree {
		t.Errorf("row = %+v", got)
	}
	if got := branchTag(t, h, s.ID); got != "issue-7-fix" {
		t.Errorf("tag = %q", got)
	}
	if rec, err := bus.ReadState(h.State, s.ID); err != nil || rec.Branch != "issue-7-fix" {
		t.Errorf("state file branch = %q, %v", rec.Branch, err)
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Branch != "issue-7-fix" || records[0].Thread != s.ID {
		t.Errorf("records = %+v", records)
	}
}

func TestTrackRefusesAnotherThreadsBranch(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	first := issueThread(h, t, 7)
	second := issueThread(h, t, 8)
	want := fmt.Sprintf("Thread %d works on issue-7", first.N)
	if err := h.Sessions().Track(second.ID, "issue-7"); err == nil || err.Error() != want {
		t.Errorf("track of a live thread's branch = %v, want %q", err, want)
	}
	if err := h.SettleThread(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.Sessions().Track(second.ID, "issue-7"); err == nil || err.Error() != want {
		t.Errorf("track of a settled thread's branch = %v, want %q", err, want)
	}
	if got, _ := h.Session(second.ID); got.Branch != "issue-8" || branchTag(t, h, second.ID) != "issue-8" {
		t.Errorf("the refused thread = %+v, tag %q", got, branchTag(t, h, second.ID))
	}
}

func TestTrackRefusesAReadThread(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	read, err := h.NewThreadIn("ask", "", "read", "project")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Sessions().Track(read.ID, "my-fix"); err == nil || err.Error() != "A Read thread works on no branch" {
		t.Errorf("track = %v", err)
	}
	if got, _ := h.Session(read.ID); got.Branch != "" || branchTag(t, h, read.ID) != "" {
		t.Errorf("the Read thread = %+v, tag %q", got, branchTag(t, h, read.ID))
	}
}

func TestTrackRefusesABranchCheckedOutInAnotherThreadsWorktree(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	first := issueThread(h, t, 7)
	if err := h.Sessions().Track(first.ID, "issue-7-fix"); err != nil {
		t.Fatal(err)
	}
	second := issueThread(h, t, 8)
	want := fmt.Sprintf("Thread %d works on issue-7", first.N)
	if err := h.Sessions().Track(second.ID, "issue-7"); err == nil || err.Error() != want {
		t.Errorf("track of the branch in the first thread's worktree = %v, want %q", err, want)
	}
	if got, _ := h.Session(second.ID); got.Branch != "issue-8" {
		t.Errorf("the refused thread is on %q", got.Branch)
	}
}

func TestCleanupOfABranchTrackedElsewhereLeavesTheMadeWorktree(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	gitIn(t, h.Dir, "branch", "side")
	gitIn(t, h.Dir, "push", "origin", "side")
	if err := h.Sessions().Track(s.ID, "side"); err != nil {
		t.Fatal(err)
	}
	h.GH.SetPRFor("side", numberedPR(20, "OPEN"))
	h.RefreshPRs()
	h.GH.SetPRFor("side", numberedPR(20, "MERGED"))
	h.RefreshPRs()
	eventually(t, "the clean-up of side to keep the worktree", func() bool { got, _ := h.Session(s.ID); return got.Cleanup == "blocked" })
	if got, _ := h.Session(s.ID); got.CleanupReason != "the worktree "+s.Worktree+" is not on side" {
		t.Errorf("reason = %q", got.CleanupReason)
	}
	if !exists(s.Worktree) || gitIn(t, s.Worktree, "branch", "--show-current") != "issue-7" {
		t.Error("the clean-up of a branch the worktree does not have removed the worktree")
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Branch != "issue-7" || records[0].Removed != 0 {
		t.Errorf("records = %+v", records)
	}

	if err := h.Cleanup(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if !cleanedUp(h, s.ID)() || exists(s.Worktree) || gitIn(t, h.Dir, "branch", "--list", "issue-7", "side") == "" {
		t.Errorf("the forced clean-up: worktree there %v, branches %q", exists(s.Worktree), gitIn(t, h.Dir, "branch", "--list", "issue-7", "side"))
	}
}
