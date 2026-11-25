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
	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/bus"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
)

func issueSession(h *apptest.Harness, t *testing.T, issue int) sessions.Session {
	t.Helper()
	s, err := h.Sessions().Create(fmt.Sprintf("#%d work", issue), "", false, issue, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	_, socket := hostFiles(h, s.ID)
	eventually(t, "the host's socket", func() bool { return exists(socket) })
	return s
}

func TestPRTracking(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueSession(h, t, 12)
	if s.Branch != "issue-12" || s.PR != nil || s.PRAttention != "" || s.Cleanup != "" {
		t.Errorf("fresh session = %+v", s)
	}
	prAlerts := func() int { return h.Rec.CountAttentionFor(s.ID, "pr") }

	h.GH.SetPR(prJSON("OPEN", true, `[{"status":"IN_PROGRESS"}]`, 1, 0))
	h.RefreshPRs()
	got, _ := h.Session(s.ID)
	if got.PR == nil || got.PR.State != "draft" || got.PR.Checks != "pending" || got.PR.Comments != 1 || got.PRAttention != "" {
		t.Fatalf("first sight = %+v", got)
	}

	h.GH.SetPR(prJSON("OPEN", false, "[]", 2, 1))
	h.RefreshPRs()
	h.RefreshPRs()
	if got, _ = h.Session(s.ID); got.PRAttention != "comments" || got.PR.State != "open" || got.PR.NewComments != 2 {
		t.Fatalf("new comments = %+v", got)
	}
	if prAlerts() != 1 {
		t.Errorf("pr attention events = %d, want 1", prAlerts())
	}
	if last := h.Rec.LastSessions(); len(last) == 0 || last[0].PRAttention != "comments" {
		t.Error("the sessions event did not carry the attention")
	}

	if err := h.AckPR(s.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = h.Session(s.ID); got.PRAttention != "" || got.PR.NewComments != 0 {
		t.Errorf("after ack = %+v", got)
	}

	h.GH.SetPR(prJSON("OPEN", false, `[{"status":"COMPLETED","conclusion":"FAILURE"}]`, 3, 0))
	h.RefreshPRs()
	if got, _ = h.Session(s.ID); got.PRAttention != "checks" || prAlerts() != 2 {
		t.Errorf("failing checks: attention %q, events %d", got.PRAttention, prAlerts())
	}
	if err := h.AckPR(s.ID); err != nil {
		t.Fatal(err)
	}

	t.Run("a restart does not alert again", func(t *testing.T) {
		fresh := h.Restart(t)
		fresh.RefreshPRs()
		var again sessions.Session
		for _, v := range fresh.Sessions().List() {
			if v.ID == s.ID {
				again = v
			}
		}
		if again.PR == nil || again.PRAttention != "" || fresh.Rec.CountAttentionFor(s.ID, "pr") != 0 {
			t.Errorf("after restart = %+v, events %d", again, fresh.Rec.CountAttentionFor(s.ID, "pr"))
		}
		h.GH.SetPR(prJSON("OPEN", false, `[{"status":"COMPLETED","conclusion":"FAILURE"}]`, 4, 0))
		fresh.RefreshPRs()
		for _, v := range fresh.Sessions().List() {
			if v.ID == s.ID {
				again = v
			}
		}
		if again.PRAttention != "comments" {
			t.Errorf("a new comment after restart: %q", again.PRAttention)
		}
	})
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// repoFixture makes h.Dir a git repo with a bare origin, and returns the origin's path.
func repoFixture(t *testing.T, h *apptest.Harness) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, filepath.Dir(origin), "init", "--bare", origin)
	gitIn(t, h.Dir, "init")
	if err := os.WriteFile(filepath.Join(h.Dir, "README"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, h.Dir, "add", "README")
	gitIn(t, h.Dir, "commit", "-m", "first")
	gitIn(t, h.Dir, "remote", "add", "origin", origin)
	gitIn(t, h.Dir, "push", "-u", "origin", "main")
	return origin
}

// issueThread starts the thread of issue n in the worktree agentos makes for it; repoFixture comes first.
func issueThread(h *apptest.Harness, t *testing.T, n int) sessions.Session {
	t.Helper()
	s := issueSession(h, t, n)
	if s.Worktree == "" {
		t.Fatalf("the thread of #%d has no worktree: %+v", n, s)
	}
	return s
}

// pushWork commits a file in the worktree and pushes its branch to origin.
func pushWork(t *testing.T, wt string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(wt, "work.txt"), []byte("work\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, wt, "add", "work.txt")
	gitIn(t, wt, "commit", "-m", "work")
	gitIn(t, wt, "push", "-u", "origin", "HEAD")
}

func branches(t *testing.T, dir string) string {
	return gitIn(t, dir, "branch", "--list", "issue-7")
}

func TestCleanupMerged(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
	if resp := h.Ask(t, ctl.Request{Cmd: "show", Session: s.ID, Opts: map[string]string{"text": "tested by hand"}}); !resp.OK {
		t.Fatalf("show = %+v", resp)
	}

	openThenMerge(h)
	eventually(t, "the session to be cleaned up", cleanedUp(h, s.ID))

	if exists(wt) || branches(t, h.Dir) == "" {
		t.Errorf("worktree exists: %v, branches: %q", exists(wt), branches(t, h.Dir))
	}
	log := h.Cleanups()
	if len(log) != 1 || log[0].Status != "done" || log[0].Issue != 7 || log[0].PR != 12 || !log[0].Merged || log[0].SessionTitle != "#7 work" {
		t.Fatalf("log = %+v", log)
	}
	if want := []string{"worktree " + wt, "settled thread", "temp folder"}; !slices.Equal(log[0].Removed, want) {
		t.Errorf("removed = %q, want %q", log[0].Removed, want)
	}
	if got := h.Events(t, activity.Cleanup); len(got) != 1 || got[0].Label != "done" || got[0].Issue != 7 || got[0].PR != 12 || got[0].Session != s.ID {
		t.Errorf("cleanup events = %+v", got)
	}
	if got := h.Rec.ProjectOf("cleanups"); got != "main" || len(h.Rec.LastCleanups()) != 1 {
		t.Errorf("cleanups event: project %q, %d entries", got, len(h.Rec.LastCleanups()))
	}
	if out, _ := exec.Command("tmux", "-L", h.Socket, "list-sessions").CombinedOutput(); strings.Contains(string(out), s.ID) {
		t.Errorf("tmux session still there: %s", out)
	}
	if exists(filepath.Join(h.Dir, "README")) == false {
		t.Error("the project folder itself was touched")
	}
}

func TestCleanupBlocked(t *testing.T) {
	tests := []struct {
		name   string
		spoil  func(t *testing.T, wt string)
		reason string
	}{
		{"uncommitted change", func(t *testing.T, wt string) {
			if err := os.WriteFile(filepath.Join(wt, "work.txt"), []byte("changed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "the worktree of issue-7 has uncommitted changes"},
		{"untracked file", func(t *testing.T, wt string) {
			if err := os.WriteFile(filepath.Join(wt, "new.txt"), []byte("x\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "the worktree of issue-7 has uncommitted changes"},
		{"unpushed commit", func(t *testing.T, wt string) {
			if err := os.WriteFile(filepath.Join(wt, "more.txt"), []byte("x\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			gitIn(t, wt, "add", "more.txt")
			gitIn(t, wt, "commit", "-m", "unpushed")
		}, "issue-7 has a commit that is not on origin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			repoFixture(t, h)
			s := issueThread(h, t, 7)
			wt := s.Worktree
			pushWork(t, wt)
			tt.spoil(t, wt)

			openThenMerge(h)
			eventually(t, "blocked", func() bool { got, _ := h.Session(s.ID); return got.Cleanup == "blocked" })

			got, _ := h.Session(s.ID)
			if got.State != "settled" || got.CleanupReason != tt.reason {
				t.Errorf("state %q, reason %q, want settled, %q", got.State, got.CleanupReason, tt.reason)
			}
			if !exists(wt) || branches(t, h.Dir) == "" {
				t.Error("something was removed although the clean-up was blocked")
			}
			log := h.Cleanups()
			if len(log) != 1 || log[0].Status != "blocked" || log[0].Reason != tt.reason || len(log[0].Removed) != 0 {
				t.Errorf("log = %+v", log)
			}
			h.RefreshPRs()
			h.RefreshPRs()
			if len(h.Cleanups()) != 1 {
				t.Error("a blocked clean-up was tried again by itself")
			}

			if err := h.Cleanup(s.ID, true); err != nil {
				t.Fatal(err)
			}
			if !cleanedUp(h, s.ID)() || exists(wt) || branches(t, h.Dir) == "" {
				t.Errorf("forced clean-up left things: wt %v branches %q", exists(wt), branches(t, h.Dir))
			}
			if log := h.Cleanups(); len(log) != 2 || log[0].Status != "done" {
				t.Errorf("log after force = %+v", log)
			}
			if ev := h.Events(t, activity.Cleanup); len(ev) != 2 || ev[0].Label != "blocked" || ev[1].Label != "done" {
				t.Errorf("cleanup events = %+v", ev)
			}
		})
	}
}

func TestCleanupClosedAsksFirst(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	h.GH.SetPR(prJSON("CLOSED", false, "[]", 0, 0))
	h.RefreshPRs()
	h.RefreshPRs()
	if got, _ := h.Session(s.ID); got.Cleanup != "ask" || !exists(wt) || len(h.Cleanups()) != 0 {
		t.Fatalf("closed PR: cleanup %q, worktree %v", got.Cleanup, exists(wt))
	}
	if err := h.Cleanup(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if !cleanedUp(h, s.ID)() || exists(wt) || branches(t, h.Dir) == "" {
		t.Errorf("the confirmed clean-up: worktree there %v, branches %q", exists(wt), branches(t, h.Dir))
	}
}

// closedAt is the pull request gh lists in pr, merged or closed at that time.
func closedAt(pr string, at time.Time) string {
	return strings.Replace(pr, `"updatedAt"`, fmt.Sprintf(`"closedAt":%q,"updatedAt"`, at.UTC().Format(time.RFC3339Nano)), 1)
}

func TestPRClosedBeforeTheSessionStartedIsNotItsPR(t *testing.T) {
	tests := []struct {
		name    string
		state   string
		at      time.Time
		cleanup string
	}{
		{"closed before", "CLOSED", time.Now().Add(-time.Hour), ""},
		{"merged before", "MERGED", time.Now().Add(-time.Hour), ""},
		{"closed after", "CLOSED", time.Now().Add(time.Hour), "ask"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			repoFixture(t, h)
			s := issueThread(h, t, 7)
			wt := s.Worktree
			pushWork(t, wt)

			h.GH.SetPR(closedAt(prJSON(tt.state, false, "[]", 0, 0), tt.at))
			h.RefreshPRs()
			h.RefreshPRs()
			got, _ := h.Session(s.ID)
			if got.Cleanup != tt.cleanup || (got.PR == nil) != (tt.cleanup == "") || !exists(wt) || len(h.Cleanups()) != 0 {
				t.Errorf("cleanup %q, PR %+v, worktree %v, log %+v", got.Cleanup, got.PR, exists(wt), h.Cleanups())
			}
		})
	}
}

func TestCleanupMergedWithRemoteBranchGone(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	gitIn(t, origin, "branch", "-D", "issue-7")
	gitIn(t, h.Dir, "fetch", "--prune")

	openThenMergeAt(h, gitIn(t, h.Dir, "rev-parse", "issue-7"))
	eventually(t, "clean-up of a merged branch GitHub deleted", cleanedUp(h, s.ID))
	if exists(wt) || branches(t, h.Dir) == "" {
		t.Errorf("worktree there %v, branches %q", exists(wt), branches(t, h.Dir))
	}
}

// onOrigin says whether a branch on origin holds commit.
func onOrigin(t *testing.T, origin, commit string) bool {
	t.Helper()
	return gitIn(t, origin, "branch", "--contains", commit) != ""
}

// keptFor waits for the session's clean-up to block, and checks the reason and that the worktree and branch are on disk.
func keptFor(t *testing.T, h *apptest.Harness, id, wt, reason string) {
	t.Helper()
	eventually(t, "blocked", func() bool { got, _ := h.Session(id); return got.Cleanup == "blocked" })
	if got, _ := h.Session(id); got.CleanupReason != reason {
		t.Errorf("reason = %q, want %q", got.CleanupReason, reason)
	}
	if !exists(wt) || branches(t, h.Dir) == "" {
		t.Error("the worktree or branch was removed although the clean-up was blocked")
	}
}

func TestRebaseMergeAtThePRsLastCommitIsMerged(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	head := apptest.MergeAs(t, origin, "issue-7", "rebase+delete")
	if head != gitIn(t, wt, "rev-parse", "HEAD") || onOrigin(t, origin, head) {
		t.Fatal("the rebase kept the branch's commit ids, so the test proves nothing")
	}

	openThenMergeAt(h, head)
	eventually(t, "the clean-up of a rebase merge", cleanedUp(h, s.ID))
	if exists(wt) {
		t.Error("the worktree of a merged branch was kept")
	}
}

func TestSquashMergeAtThePRsLastCommitIsMerged(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	commitIn(t, wt, "second.txt")
	gitIn(t, wt, "push", "origin", "issue-7")
	head := apptest.MergeAs(t, origin, "issue-7", "squash+delete")
	if onOrigin(t, origin, head) {
		t.Fatal("the squash kept the branch's commits on origin")
	}

	openThenMergeAt(h, head)
	eventually(t, "the clean-up of a squash merge", cleanedUp(h, s.ID))
	if exists(wt) {
		t.Error("the worktree of a merged branch was kept")
	}
}

func TestMergeCommitInTheDefaultBranchIsMerged(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	apptest.MergeAs(t, origin, "issue-7", "merge+delete")

	openThenMerge(h)
	eventually(t, "the clean-up of a merge commit", cleanedUp(h, s.ID))
	if exists(wt) {
		t.Error("the worktree of a merged branch was kept")
	}
}

func TestWorktreePastTheMergedHeadIsKept(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	head := apptest.MergeAs(t, origin, "issue-7", "rebase+delete")
	commitIn(t, wt, "after.txt")
	after := gitIn(t, wt, "rev-parse", "HEAD")

	openThenMergeAt(h, head)
	keptFor(t, h, s.ID, wt, "issue-7 has commits that are not in the merged pull request")
	if got := gitIn(t, h.Dir, "rev-parse", "issue-7"); got != after {
		t.Errorf("issue-7 is at %s, want the commit after the merge %s", got, after)
	}
}

func TestWorktreeThatMovedOnAfterTheMergeIsKept(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	head := apptest.MergeAs(t, origin, "issue-7", "rebase")
	commitIn(t, wt, "after.txt")
	gitIn(t, wt, "push", "origin", "issue-7")

	openThenMergeAt(h, head)
	keptFor(t, h, s.ID, wt, "issue-7 has commits that are not in the merged pull request")
}

func TestForcePushBeforeTheMergeKeepsTheOldWork(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	other := filepath.Join(t.TempDir(), "other")
	gitIn(t, filepath.Dir(other), "clone", "--branch", "issue-7", origin, other)
	gitIn(t, other, "reset", "--hard", "origin/main")
	commitIn(t, other, "instead.txt")
	gitIn(t, other, "push", "--force", "origin", "issue-7")
	head := apptest.MergeAs(t, origin, "issue-7", "rebase+delete")
	gitIn(t, h.Dir, "config", "fetch.prune", "false") // over a fetch.prune in the user's own config
	if gitIn(t, h.Dir, "rev-parse", "origin/issue-7") != gitIn(t, wt, "rev-parse", "HEAD") {
		t.Fatal("the project's clone has no stale copy of the branch, so the test proves nothing")
	}

	openThenMergeAt(h, head)
	keptFor(t, h, s.ID, wt, "issue-7 has a commit that is not on origin")
}

func TestWorktreeBehindThePRsHeadIsMerged(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	owners := filepath.Join(t.TempDir(), "owners")
	gitIn(t, filepath.Dir(owners), "clone", "--branch", "issue-7", origin, owners)
	commitIn(t, owners, "fixup.txt")
	gitIn(t, owners, "push", "origin", "issue-7")
	gitIn(t, h.Dir, "fetch") // the PR's last commit is known here, as after any fetch while the PR was open
	head := apptest.MergeAs(t, origin, "issue-7", "rebase+delete")

	openThenMergeAt(h, head)
	eventually(t, "the clean-up of a worktree behind the merged head", cleanedUp(h, s.ID))
	if exists(wt) {
		t.Error("the worktree of a merged branch was kept")
	}
}

func TestFailedFetchKeepsTheWorktree(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	gitIn(t, h.Dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))

	openThenMerge(h)
	keptFor(t, h, s.ID, wt, "Could not fetch origin, so agentos cannot tell whether every commit is pushed")
}

func TestUntrackedFileKeepsTheWorktreeWhenGitHidesThem(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	gitIn(t, h.Dir, "config", "status.showUntrackedFiles", "no")
	if err := os.WriteFile(filepath.Join(wt, "new.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if gitIn(t, wt, "status", "--porcelain") != "" {
		t.Fatal("git still shows untracked files, so the test proves nothing")
	}

	openThenMerge(h)
	keptFor(t, h, s.ID, wt, "the worktree of issue-7 has uncommitted changes")
}

func TestMergedPRWithoutAHeadAndNotInMainIsKept(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	apptest.MergeAs(t, origin, "issue-7", "rebase+delete")

	openThenMerge(h)
	keptFor(t, h, s.ID, wt, "issue-7 has a commit that is not on origin")
}

func TestMergedPRAtACommitGitDoesNotKnowIsKept(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	apptest.MergeAs(t, origin, "issue-7", "rebase+delete")

	openThenMergeAt(h, strings.Repeat("0", 40))
	keptFor(t, h, s.ID, wt, "issue-7 has a commit that is not on origin")
}

func TestClosedPRWithPushedWorkIsRemovedOnConfirm(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(prJSON("CLOSED", false, "[]", 0, 0))
	h.RefreshPRs()
	if got, _ := h.Session(s.ID); got.Cleanup != "ask" || !exists(wt) {
		t.Fatalf("closed PR: cleanup %q, worktree %v", got.Cleanup, exists(wt))
	}
	if err := h.Cleanup(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := h.Session(s.ID); !cleanedUp(h, s.ID)() || exists(wt) {
		t.Errorf("the confirmed clean-up of pushed work: session %+v, worktree %v", got, exists(wt))
	}
}

// commitIn commits a new file in dir.
func commitIn(t *testing.T, dir, file string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(file+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", file)
	gitIn(t, dir, "commit", "-m", file)
}

func gitStatus(t *testing.T, h *apptest.Harness, id string) sessions.BranchStatus {
	t.Helper()
	got, _ := h.Session(id)
	if got.Git == nil {
		t.Fatalf("no git status: %+v", got)
	}
	return *got.Git
}

func TestBranchStatus(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	worksIn(h, t, s, wt)
	h.RefreshPRs()
	if got := gitStatus(t, h, s.ID); got != (sessions.BranchStatus{Base: "main", Ahead: 1}) {
		t.Errorf("a pushed branch = %+v", got)
	}

	other := filepath.Join(t.TempDir(), "other")
	gitIn(t, filepath.Dir(other), "clone", origin, other)
	commitIn(t, other, "on-main.txt")
	gitIn(t, other, "push", "origin", "main")
	gitIn(t, h.Dir, "fetch")
	commitIn(t, wt, "later.txt")
	if err := os.WriteFile(filepath.Join(wt, "new.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h.RefreshPRs()
	if got := gitStatus(t, h, s.ID); got != (sessions.BranchStatus{Base: "main", Ahead: 2, Behind: 1, Unpushed: 1, Uncommitted: 1}) {
		t.Errorf("after main moved and the agent worked = %+v", got)
	}

	if err := os.WriteFile(filepath.Join(wt, "README"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cwd, _ := json.Marshal(map[string]string{"cwd": wt}) // the same folder, so only the status is read again
	h.Hook(t, s.ID, "PostToolUse", string(cwd))
	eventually(t, "the tool call's change in the row", func() bool { got, _ := h.Session(s.ID); return got.Git != nil && got.Git.Uncommitted == 2 })
}

func TestBranchStatusOfABranchOriginHasNot(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	commitIn(t, s.Worktree, "a.txt")
	commitIn(t, s.Worktree, "b.txt")
	h.RefreshPRs()
	if got := gitStatus(t, h, s.ID); got != (sessions.BranchStatus{Base: "main", Ahead: 2, Unpushed: 2}) {
		t.Errorf("a branch never pushed = %+v", got)
	}
}

func TestBranchStatusAfterAMergeThatLeftACommitOut(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	head := gitIn(t, wt, "rev-parse", "HEAD")
	commitIn(t, wt, "after.txt")
	gitIn(t, origin, "branch", "-D", "issue-7")
	gitIn(t, h.Dir, "fetch", "--prune")

	openThenMergeAt(h, head)
	eventually(t, "blocked", func() bool { got, _ := h.Session(s.ID); return got.Cleanup == "blocked" })
	if got := gitStatus(t, h, s.ID); got.NotInMain != 1 || got.Unpushed != 0 {
		t.Errorf("after the merge = %+v, want 1 commit not in main and none counted unpushed", got)
	}
}

func TestBranchStatusAfterARebaseMerge(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    session_cleanup_mode: {merge: manual}\n"})
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	head := apptest.MergeAs(t, origin, "issue-7", "rebase+delete")

	h.GH.SetPR(apptest.WithHead(prJSON("MERGED", false, "[]", 0, 0), head)) // the owner decides, so the row stays
	h.RefreshPRs()
	if got := gitStatus(t, h, s.ID); got.NotInMain != 0 || got.Unpushed != 0 {
		t.Errorf("at the merged head = %+v, want nothing left out", got)
	}
	commitIn(t, wt, "after.txt")
	h.RefreshPRs()
	if got := gitStatus(t, h, s.ID); got.NotInMain != 1 || got.Unpushed != 0 {
		t.Errorf("one commit after the merged head = %+v, want 1 not in main", got)
	}
}

func TestCleanupCustomCommand(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: "git worktree remove {force} {worktree} && touch {dir}/cleaned-{branch}"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	openThenMerge(h)
	eventually(t, "clean-up", cleanedUp(h, s.ID))
	if exists(wt) || !exists(filepath.Join(h.Dir, "cleaned-issue-7")) {
		t.Errorf("custom command: worktree exists %v, marker exists %v", exists(wt), exists(filepath.Join(h.Dir, "cleaned-issue-7")))
	}
	if branches(t, h.Dir) == "" {
		t.Error("the branch is gone")
	}
}

func TestCleanupCommandGetsForceOnlyWhenForced(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: "printf '[%s]' {force} > {dir}/force-{branch}; git worktree remove {force} {worktree}"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	marker := filepath.Join(h.Dir, "force-issue-7")

	openThenMerge(h)
	eventually(t, "clean-up", cleanedUp(h, s.ID))
	if got, _ := os.ReadFile(marker); string(got) != "[]" {
		t.Errorf("unforced {force} = %q, want empty", got)
	}

	s8 := issueThread(h, t, 8)
	dirty := s8.Worktree
	if err := os.WriteFile(filepath.Join(dirty, "new.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.Cleanup(s8.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(h.Dir, "force-issue-8")); string(got) != "[--force]" || exists(dirty) {
		t.Errorf("forced {force} = %q, worktree exists %v", got, exists(dirty))
	}
	if gitIn(t, h.Dir, "branch", "--list", "issue-8") == "" {
		t.Error("the forced clean-up deleted the branch")
	}
}

func TestSetupCleanupCommand(t *testing.T) {
	var set project.Project
	set.SetUp()
	if set.CleanupCommand != "" || set.RemoveWorktree() != project.DefaultCleanupCommand {
		t.Fatalf("a set-up project cleans up with %q, want the default %q", set.RemoveWorktree(), project.DefaultCleanupCommand)
	}
	h := apptest.New(t) // no session_cleanup_command, as SetUp writes none
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	head := gitIn(t, wt, "rev-parse", "HEAD")

	openThenMerge(h)
	eventually(t, "clean-up", cleanedUp(h, s.ID))
	if exists(wt) {
		t.Error("the default command left the worktree")
	}
	if got := gitIn(t, h.Dir, "rev-parse", "refs/heads/issue-7"); got != head {
		t.Errorf("issue-7 is at %q, want %s", got, head)
	}
}

func TestCleanupRemovesTheWorktreeAndKeepsTheBranch(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	head := gitIn(t, wt, "rev-parse", "HEAD")

	openThenMergeAt(h, head)
	eventually(t, "the clean-up", cleanedUp(h, s.ID))
	if exists(wt) || strings.Contains(gitIn(t, h.Dir, "worktree", "list", "--porcelain"), wt) {
		t.Errorf("the worktree is still there: on disk %v, git lists %s", exists(wt), gitIn(t, h.Dir, "worktree", "list", "--porcelain"))
	}
	if got := gitIn(t, h.Dir, "branch", "--list", "issue-7"); got == "" {
		t.Error("the branch is gone")
	}
	if got := gitIn(t, h.Dir, "rev-parse", "refs/heads/issue-7"); got != head {
		t.Errorf("issue-7 is at %s, want %s", got, head)
	}
	if got, _ := h.Session(s.ID); got.State != "settled" || got.Worktree != "" || got.Branch != "issue-7" {
		t.Errorf("row = %+v", got)
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Path != wt || records[0].Removed == 0 {
		t.Errorf("records = %+v", records)
	}
	log := h.Cleanups()
	if len(log) != 1 || log[0].Status != "done" || !log[0].Merged {
		t.Fatalf("log = %+v", log)
	}
	if want := []string{"worktree " + wt, "settled thread", "temp folder"}; !slices.Equal(log[0].Removed, want) {
		t.Errorf("removed = %q, want %q", log[0].Removed, want)
	}
}

func TestCleanupNeverRemovesAWorktreeAHumanMade(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(fmt.Sprintf("forced %v", forced), func(t *testing.T) {
			ran := filepath.Join(t.TempDir(), "ran")
			h := apptest.NewWith(t, apptest.Options{CleanupCommand: noteCleanups(ran)})
			repoFixture(t, h)
			human := filepath.Join(h.Dir, "trees", "issue-7")
			gitIn(t, h.Dir, "worktree", "add", "-b", "issue-7", human)
			commitIn(t, human, "human.txt")
			head := gitIn(t, human, "rev-parse", "HEAD")
			notes := filepath.Join(human, "notes.txt")
			if err := os.WriteFile(notes, []byte("mine\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			s := issueSession(h, t, 7)
			if s.Branch != "issue-7" || s.Worktree != "" {
				t.Fatalf("thread = %+v, want it in the human's worktree on issue-7", s)
			}

			if forced {
				if err := h.Cleanup(s.ID, true); err != nil {
					t.Fatal(err)
				}
			} else {
				openThenMerge(h)
			}
			eventually(t, "the clean-up", cleanedUp(h, s.ID))
			if !exists(notes) || !exists(filepath.Join(human, "human.txt")) || worktreesOf(t, h.Dir)["issue-7"] != realDir(t, human) {
				t.Errorf("the human's worktree: notes there %v, worktrees %v", exists(notes), worktreesOf(t, h.Dir))
			}
			if got := gitIn(t, h.Dir, "rev-parse", "refs/heads/issue-7"); got != head {
				t.Errorf("issue-7 is at %s, want %s", got, head)
			}
			if runs := cleanupRuns(ran); len(runs) != 0 {
				t.Errorf("the clean-up command ran for %q", runs)
			}
			if log := h.Cleanups(); len(log) != 1 || log[0].Status != "done" || !slices.Equal(log[0].Removed, []string{"settled thread", "temp folder"}) {
				t.Errorf("log = %+v", log)
			}
		})
	}
}

// projectFolderThread is a Write thread in the project folder that tracks my-fix, checked out and pushed there. Its
// clean-up command notes each folder it runs for in the file ran.
func projectFolderThread(t *testing.T) (h *apptest.Harness, s sessions.Session, ran string) {
	t.Helper()
	ran = filepath.Join(t.TempDir(), "ran")
	h = apptest.NewWith(t, apptest.Options{CleanupCommand: noteCleanups(ran)})
	repoFixture(t, h)
	gitIn(t, h.Dir, "checkout", "-b", "my-fix")
	gitIn(t, h.Dir, "push", "-u", "origin", "my-fix")
	s, err := h.NewThreadIn("my work", "", "supervised", "project")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Sessions().Track(s.ID, "my-fix"); err != nil {
		t.Fatal(err)
	}
	return h, s, ran
}

// projectFolderIntact checks that the project folder is the repo's checkout, on my-fix, with its README and file.
func projectFolderIntact(t *testing.T, h *apptest.Harness, file string) {
	t.Helper()
	if !exists(filepath.Join(h.Dir, "README")) || !exists(filepath.Join(h.Dir, file)) {
		t.Errorf("the project folder lost files: README there %v, %s there %v", exists(filepath.Join(h.Dir, "README")), file, exists(filepath.Join(h.Dir, file)))
	}
	if got := gitIn(t, h.Dir, "branch", "--show-current"); got != "my-fix" {
		t.Errorf("the project folder is on %q, want my-fix", got)
	}
	if realDir(t, gitIn(t, h.Dir, "rev-parse", "--show-toplevel")) != realDir(t, h.Dir) {
		t.Error("the project folder is no longer the repo's checkout")
	}
}

func TestThreadInTheProjectFolderLosesOnlyItsTempFolder(t *testing.T) {
	h, s, ran := projectFolderThread(t)
	openThenMerge(h)
	eventually(t, "the clean-up", cleanedUp(h, s.ID))
	projectFolderIntact(t, h, "README")
	if runs := cleanupRuns(ran); len(runs) != 0 {
		t.Errorf("the clean-up command ran for %q", runs)
	}
	if log := h.Cleanups(); len(log) != 1 || log[0].Status != "done" || !slices.Equal(log[0].Removed, []string{"settled thread", "temp folder"}) {
		t.Errorf("log = %+v", log)
	}
}

func TestForcedCleanupNeverRemovesTheProjectFolder(t *testing.T) {
	h, s, ran := projectFolderThread(t)
	if err := os.WriteFile(filepath.Join(h.Dir, "notes.txt"), []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.Cleanup(s.ID, true); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the clean-up", cleanedUp(h, s.ID))
	projectFolderIntact(t, h, "notes.txt")
	if runs := cleanupRuns(ran); len(runs) != 0 {
		t.Errorf("the clean-up command ran for %q", runs)
	}
	if log := h.Cleanups(); len(log) != 1 || slices.ContainsFunc(log[0].Removed, func(r string) bool { return strings.HasPrefix(r, "worktree") }) {
		t.Errorf("log = %+v", log)
	}
}

func TestHumanWorktreeAtARemovedPathIsNotRemoved(t *testing.T) {
	ran := filepath.Join(t.TempDir(), "ran")
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: noteCleanups(ran)})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	openThenMerge(h)
	eventually(t, "the clean-up", cleanedUp(h, s.ID))
	if exists(wt) {
		t.Fatal("the clean-up left the worktree")
	}

	gitIn(t, h.Dir, "worktree", "add", wt, "issue-7")
	mine := filepath.Join(wt, "mine.txt")
	if err := os.WriteFile(mine, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.Cleanup(s.ID, true); err == nil {
		t.Error("a forced clean-up after the worktree was removed ran")
	}
	fresh := h.Restart(t)
	fresh.RefreshPRs()
	fresh.RefreshPRs()
	if err := fresh.Cleanup(s.ID, true); err == nil {
		t.Error("a forced clean-up after a restart ran")
	}
	time.Sleep(300 * time.Millisecond)
	if !exists(mine) || worktreesOf(t, h.Dir)["issue-7"] != realDir(t, wt) {
		t.Errorf("the human's worktree: file there %v, worktrees %v", exists(mine), worktreesOf(t, h.Dir))
	}
	if runs := cleanupRuns(ran); !slices.Equal(runs, []string{wt}) {
		t.Errorf("the clean-up command ran for %q, want once for %s", runs, wt)
	}
	if log := fresh.Cleanups(); len(log) != 1 {
		t.Errorf("log = %+v", log)
	}
}

func TestCleanupCommandThatFailsWithTheWorktreeGoneCounts(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: "git worktree remove {force} {worktree}; exit 3"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	openThenMerge(h)
	eventually(t, "clean-up", cleanedUp(h, s.ID))
	if exists(wt) || branches(t, h.Dir) == "" {
		t.Errorf("worktree exists %v, branches %q", exists(wt), branches(t, h.Dir))
	}
	if log := h.Cleanups(); len(log) != 1 || log[0].Status != "done" || !slices.Equal(log[0].Removed, []string{"worktree " + wt, "settled thread", "temp folder"}) {
		t.Errorf("log = %+v", log)
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Removed == 0 {
		t.Errorf("records = %+v", records)
	}
}

func TestCleanupCommandThatFailsWithTheWorktreeLeftKeeps(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: "echo no such luck >&2; exit 3"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	openThenMerge(h)
	keptFor(t, h, s.ID, wt, "the clean-up command failed: sh -c: exit status 3: no such luck")
	if got, _ := h.Session(s.ID); got.State != "settled" || got.Worktree != wt {
		t.Errorf("row = %+v", got)
	}
	if log := h.Cleanups(); len(log) != 1 || log[0].Status != "blocked" || len(log[0].Removed) != 0 {
		t.Errorf("log = %+v", log)
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Removed != 0 {
		t.Errorf("records = %+v", records)
	}
}

func TestCleanupCommandThatLeavesTheWorktreeKeeps(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: "true {worktree}"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	openThenMerge(h)
	keptFor(t, h, s.ID, wt, "the clean-up command left "+wt)
}

func TestReadThreadHasNothingToCleanUp(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s, err := h.NewThread("ask", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	openThenMerge(h)
	time.Sleep(400 * time.Millisecond)
	if got, ok := h.Session(s.ID); !ok || got.State == "settled" || got.Branch != "" || got.PR != nil {
		t.Errorf("the Read thread = %+v, there %v", got, ok)
	}
	if err := h.Cleanup(s.ID, true); err == nil {
		t.Error("Cleanup accepted a Read thread with no branch")
	}
	if !exists(filepath.Join(h.Dir, "README")) || len(h.Cleanups()) != 0 {
		t.Errorf("README there %v, log %+v", exists(filepath.Join(h.Dir, "README")), h.Cleanups())
	}
}

// cleanedUp says whether the session's clean-up finished: its thread stays, settled, with no PR or clean-up left.
func cleanedUp(h *apptest.Harness, id string) func() bool {
	return func() bool {
		got, ok := h.Session(id)
		return ok && got.State == "settled" && got.PR == nil && got.Cleanup == ""
	}
}

func openThenMerge(h *apptest.Harness) {
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(prJSON("MERGED", false, "[]", 0, 0))
	h.RefreshPRs()
}

// openThenMergeAt is openThenMerge for a merged PR that says which commit its branch ended on.
func openThenMergeAt(h *apptest.Harness, head string) {
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(apptest.WithHead(prJSON("MERGED", false, "[]", 0, 0), head))
	h.RefreshPRs()
}

func TestPRMergedWhileTheAppWasClosedIsCleanedUp(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	h.RefreshPRs()
	time.Sleep(2 * time.Millisecond) // the merge must come after the start to the millisecond
	merged := time.Now()

	fresh := h.Restart(t)
	h.GH.SetPR(closedAt(prJSON("MERGED", false, "[]", 0, 0), merged))
	fresh.RefreshPRs()
	eventually(t, "the clean-up at the next start", cleanedUp(fresh, s.ID))
	if exists(wt) || branches(t, h.Dir) == "" {
		t.Errorf("worktree there %v, branches %q", exists(wt), branches(t, h.Dir))
	}
	if log := fresh.Cleanups(); len(log) != 1 || log[0].Status != "done" || !log[0].Merged {
		t.Errorf("log = %+v", log)
	}
}

func TestOldPRsFileLoads(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	path := filepath.Join(h.State, "data", "main", "prs.json")
	old := fmt.Sprintf(`{"acks":{"12":{"comments":1,"checks":false}},"nudged":{},"opened":{},"branches":{%[1]q:["side","issue-7"]},"reported":{%[1]q:["issue-7"]},"sawOpen":{%[1]q:["issue-7"]},"cleaned":[]}`, s.ID)
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	fresh := h.Restart(t)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 2, 0))
	fresh.RefreshPRs()
	if got, _ := fresh.Session(s.ID); got.PR == nil || got.PRAttention != "comments" || got.PR.NewComments != 1 {
		t.Fatalf("row = %+v, want the one comment the old file had not seen", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"branches", "reported", "sawOpen"} {
		if _, ok := saved[gone]; ok {
			t.Errorf("the saved file still has %q: %s", gone, data)
		}
	}
	if string(saved["acks"]) != `{"12":{"comments":1,"checks":false}}` {
		t.Errorf("acks = %s", saved["acks"])
	}
}

func TestCleanupAfterRestartBetweenOpenAndMerged(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()

	fresh := h.Restart(t)
	h.GH.SetPR(prJSON("MERGED", false, "[]", 0, 0))
	fresh.RefreshPRs()
	eventually(t, "the restarted app to clean up", cleanedUp(fresh, s.ID))
	if exists(wt) || branches(t, h.Dir) == "" {
		t.Errorf("worktree there %v, branches %q", exists(wt), branches(t, h.Dir))
	}
}

func TestPROpenedAttention(t *testing.T) {
	t.Run("a PR seen while the session is idle", func(t *testing.T) {
		h := newHarness(t)
		repoFixture(t, h)
		s := issueSession(h, t, 12)
		h.GH.SetPR(prJSON("OPEN", true, "[]", 0, 0))
		h.RefreshPRs()
		h.RefreshPRs()
		if got := h.Rec.CountAttentionFor(s.ID, "opened"); got != 1 {
			t.Errorf("opened attention events = %d, want 1", got)
		}
		fresh := h.Restart(t)
		fresh.RefreshPRs()
		if got := fresh.Rec.CountAttentionFor(s.ID, "opened"); got != 0 {
			t.Errorf("opened attention events after restart = %d, want 0", got)
		}
	})

	t.Run("a PR seen while the session works waits for its reply", func(t *testing.T) {
		h := newHarness(t)
		repoFixture(t, h)
		s := issueSession(h, t, 12)
		h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
		eventually(t, "working", func() bool {
			got, _ := h.Session(s.ID)
			return got.State == "working"
		})
		h.GH.SetPR(prJSON("OPEN", true, "[]", 0, 0))
		h.RefreshPRs()
		if got := h.Rec.CountAttentionFor(s.ID, "opened"); got != 0 {
			t.Errorf("opened attention events while working = %d, want 0", got)
		}
		h.Hook(t, s.ID, "Stop", `{}`)
		eventually(t, "idle", func() bool {
			got, _ := h.Session(s.ID)
			return got.State == "idle"
		})
		if opened, replied := h.Rec.CountAttentionFor(s.ID, "opened"), h.Rec.CountAttentionFor(s.ID, "replied"); opened != 1 || replied != 0 {
			t.Errorf("after the reply: opened %d, replied %d; want 1 and 0", opened, replied)
		}
		h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"more"}`)
		h.Hook(t, s.ID, "Stop", `{}`)
		eventually(t, "replied again", func() bool { return h.Rec.CountAttentionFor(s.ID, "replied") == 1 })
		if got := h.Rec.CountAttentionFor(s.ID, "opened"); got != 1 {
			t.Errorf("opened attention events after a second reply = %d, want 1", got)
		}
	})
}

func openThenClose(h *apptest.Harness) {
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(prJSON("CLOSED", false, "[]", 0, 0))
	h.RefreshPRs()
}

func TestCleanupMergeManualAsks(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    session_cleanup_mode: {merge: manual}\n"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	openThenMerge(h)
	h.RefreshPRs()
	time.Sleep(300 * time.Millisecond)
	if got, _ := h.Session(s.ID); got.Cleanup != "ask" || !exists(wt) || len(h.Cleanups()) != 0 {
		t.Fatalf("manual merge: cleanup %q, worktree %v", got.Cleanup, exists(wt))
	}
}

func TestCleanupCloseAuto(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    session_cleanup_mode: {close: auto}\n"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	openThenClose(h)
	eventually(t, "the session to be cleaned up", cleanedUp(h, s.ID))
	if exists(wt) || branches(t, h.Dir) == "" {
		t.Errorf("worktree exists: %v, branches: %q", exists(wt), branches(t, h.Dir))
	}
}

func TestCleanupCloseAutoKeepsSafetyChecks(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    session_cleanup_mode: {close: auto}\n"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	if err := os.WriteFile(filepath.Join(wt, "unsaved"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	openThenClose(h)
	eventually(t, "the clean-up to block", func() bool { got, _ := h.Session(s.ID); return got.Cleanup == "blocked" })
	if !exists(wt) {
		t.Error("an automatic clean-up removed a worktree with unsaved changes")
	}
}

func numberedPR(n int, state string) string {
	return strings.Replace(prJSON(state, false, "[]", 0, 0), `"number":12`, fmt.Sprintf(`"number":%d`, n), 1)
}

func onBranch(h *apptest.Harness, id, branch string) func() bool {
	return func() bool { got, ok := h.Session(id); return ok && got.Branch == branch }
}

func TestRebasingWorktreeBlocksTheCleanup(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	cmd := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@example.com", "rebase", "--exec", "false", "HEAD~1")
	cmd.Dir = wt
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("the rebase did not stop: %s", out)
	}

	openThenMerge(h)
	eventually(t, "blocked", func() bool { got, _ := h.Session(s.ID); return got.Cleanup == "blocked" })
	realWt, _ := filepath.EvalSymlinks(wt)
	if got, _ := h.Session(s.ID); got.CleanupReason != "the worktree of issue-7 is in the middle of a rebase" || got.Worktree != realWt {
		t.Errorf("reason %q, worktree %q", got.CleanupReason, got.Worktree)
	}
	if !exists(wt) || branches(t, h.Dir) == "" {
		t.Error("the clean-up removed a worktree in the middle of a rebase")
	}
}

func TestMergedPRSettlesTheThread(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    session_cleanup_mode: {merge: manual}\n"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	openThenMerge(h)
	got, _ := h.Session(s.ID)
	if got.State != "settled" || got.PR == nil || got.PR.State != "merged" || got.Cleanup != "ask" {
		t.Fatalf("after the merge = %+v", got)
	}
	if out, _ := exec.Command("tmux", "-L", h.Socket, "list-sessions").CombinedOutput(); strings.Contains(string(out), s.ID) {
		t.Errorf("tmux session still there: %s", out)
	}
	if !exists(wt) || len(h.Cleanups()) != 0 {
		t.Errorf("settling cleaned up: worktree there %v, log %+v", exists(wt), h.Cleanups())
	}
}

func TestClosedPRSettlesTheThread(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    session_cleanup_mode: {close: manual}\n"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)

	openThenClose(h)
	h.RefreshPRs()
	time.Sleep(300 * time.Millisecond)
	if got, _ := h.Session(s.ID); got.State != "settled" || got.PR == nil || got.PR.State != "closed" || got.Cleanup != "ask" {
		t.Fatalf("after the close = %+v", got)
	}
	if !exists(wt) || len(h.Cleanups()) != 0 {
		t.Errorf("a manual close cleaned up: worktree there %v, log %+v", exists(wt), h.Cleanups())
	}
}

func TestPRMergedBeforeTheLastWakeDoesNotSettle(t *testing.T) {
	h := newHarness(t)
	old := issueSession(h, t, 7)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	ended(t, h, old)
	merged := time.Now()
	time.Sleep(2 * time.Millisecond) // the wake must come after the merge to the millisecond
	woken, err := h.WakeThread(old.ID)
	if err != nil || woken.ID != old.ID {
		t.Fatalf("wake = %+v, %v", woken, err)
	}

	h.GH.SetPR(closedAt(prJSON("MERGED", false, "[]", 0, 0), merged))
	h.RefreshPRs()
	h.RefreshPRs()
	time.Sleep(300 * time.Millisecond)
	if got, _ := h.Session(woken.ID); got.State == "settled" || got.PR != nil || got.Cleanup != "" || len(h.Cleanups()) != 0 {
		t.Errorf("woken thread = %+v, log %+v", got, h.Cleanups())
	}
}

func TestCleanupKeepsTheThread(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	h.Hook(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
	if resp := h.Ask(t, ctl.Request{Cmd: "show", Session: s.ID, Opts: map[string]string{"text": "tested by hand"}}); !resp.OK {
		t.Fatalf("show = %+v", resp)
	}
	chatLog, _ := hostFiles(h, s.ID)
	name, _ := session.ParseName(s.ID)
	tmp := filepath.Join(h.State, "data", name.Project, "tmp", name.Token)
	if !exists(chatLog) || !exists(tmp) {
		t.Fatalf("before the merge: chat log there %v, temp folder there %v", exists(chatLog), exists(tmp))
	}

	openThenMerge(h)
	eventually(t, "the clean-up", cleanedUp(h, s.ID))
	if got, _ := h.Session(s.ID); got.State != "settled" || got.Evidence != 1 {
		t.Errorf("row = %+v", got)
	}
	if rec, err := bus.ReadState(h.State, s.ID); err != nil || rec.Session != s.ID || rec.State != "settled" {
		t.Errorf("state file = %+v, %v", rec, err)
	}
	if !exists(chatLog) || exists(tmp) || exists(wt) {
		t.Errorf("chat log there %v, temp folder there %v, worktree there %v", exists(chatLog), exists(tmp), exists(wt))
	}
	if log := h.Cleanups(); len(log) != 1 || log[0].Status != "done" || !slices.Contains(log[0].Removed, "settled thread") || !slices.Contains(log[0].Removed, "temp folder") {
		t.Errorf("log = %+v", log)
	}
}

func TestCleanupRunsOncePerWorktree(t *testing.T) {
	ran := filepath.Join(t.TempDir(), "ran")
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: noteCleanups(ran)})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	pushWork(t, s.Worktree)

	openThenMerge(h)
	eventually(t, "the clean-up", cleanedUp(h, s.ID))
	removedAt := madeRecords(t, h)[0].Removed
	h.RefreshPRs()
	if err := h.Cleanup(s.ID, true); err == nil {
		t.Error("a second clean-up of the thread was accepted")
	}

	fresh := h.Restart(t)
	fresh.RefreshPRs()
	fresh.RefreshPRs()
	if err := fresh.Cleanup(s.ID, true); err == nil {
		t.Error("a clean-up after a restart was accepted")
	}
	time.Sleep(300 * time.Millisecond)
	if log := fresh.Cleanups(); len(log) != 1 {
		t.Errorf("log = %+v", log)
	}
	if runs := cleanupRuns(ran); !slices.Equal(runs, []string{s.Worktree}) {
		t.Errorf("the clean-up command ran for %q", runs)
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Removed != removedAt {
		t.Errorf("records = %+v, want removed at %d", records, removedAt)
	}
	if got, ok := fresh.Session(s.ID); !ok || !cleanedUp(fresh, s.ID)() {
		t.Errorf("after a restart the row = %+v, there %v", got, ok)
	}
}

func TestWakeKeepsTheAskOfAPRThatMergedWhileSettled(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    session_cleanup_mode: {merge: manual}\n"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	ended(t, h, s)
	time.Sleep(2 * time.Millisecond) // the merge must come after the start, and the wake after the merge, to the millisecond
	h.GH.SetPR(closedAt(prJSON("MERGED", false, "[]", 0, 0), time.Now()))
	h.RefreshPRs()
	if got, _ := h.Session(s.ID); got.Cleanup != "ask" {
		t.Fatalf("after the merge = %+v", got)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := h.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}

	asks := func(h *apptest.Harness, when string) {
		t.Helper()
		got, _ := h.Session(s.ID)
		if got.State == "settled" || got.PR == nil || got.PR.State != "merged" || got.Cleanup != "ask" || !exists(wt) {
			t.Errorf("%s: row = %+v, worktree there %v", when, got, exists(wt))
		}
	}
	h.RefreshPRs()
	h.RefreshPRs()
	asks(h, "after the wake")
	fresh := h.Restart(t)
	fresh.RefreshPRs()
	asks(fresh, "after a restart")

	if err := fresh.Cleanup(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if !cleanedUp(fresh, s.ID)() || exists(wt) || branches(t, h.Dir) == "" {
		t.Errorf("the confirmed clean-up: row cleaned up %v, worktree there %v, branches %q", cleanedUp(fresh, s.ID)(), exists(wt), branches(t, h.Dir))
	}
}

func TestForkDoesNotSettleOnTheSourcesPR(t *testing.T) {
	h, _, _, src, fork := forkHarness(t)
	openThenMerge(h)
	eventually(t, "the source cleaned up", cleanedUp(h, src.ID))
	if got := stateOf(h, fork.ID); got == "settled" || got == "missing" {
		t.Errorf("the fork is %s", got)
	}
}

// slowOrigin makes each fetch from origin wait first, through an ssh command that sleeps and then runs git's own side
// of the fetch on the bare origin.
func slowOrigin(t *testing.T, h *apptest.Harness, origin string, wait time.Duration) {
	t.Helper()
	ssh := filepath.Join(t.TempDir(), "slow-ssh")
	script := fmt.Sprintf("#!/bin/sh\nsleep %g\nfor last; do :; done\nexec sh -c \"$last\"\n", wait.Seconds())
	if err := os.WriteFile(ssh, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	gitIn(t, h.Dir, "config", "core.sshCommand", ssh)
	gitIn(t, h.Dir, "config", "remote.origin.uploadpack", "git upload-pack")
	gitIn(t, h.Dir, "remote", "set-url", "origin", "ssh://localhost"+origin)
}

func TestWakeDuringTheCleanupKeepsTheWorktree(t *testing.T) {
	h := newHarness(t)
	origin := repoFixture(t, h)
	s := issueThread(h, t, 7)
	wt := s.Worktree
	pushWork(t, wt)
	withConversation(t, h, s.ID)
	slowOrigin(t, h, origin, 3*time.Second) // the clean-up fetches before the settle and again after it

	done := make(chan error, 1)
	go func() { done <- h.Cleanup(s.ID, false) }()
	eventually(t, "the clean-up to settle the thread", func() bool { return stateOf(h, s.ID) == "settled" })
	if _, err := h.WakeThread(s.ID); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	want := fmt.Sprintf("Thread %d woke during the clean-up", s.N)
	if got, _ := h.Session(s.ID); got.State == "settled" || got.Cleanup != "blocked" || got.CleanupReason != want || got.Worktree != wt {
		t.Errorf("row = %+v, want it live and kept with %q", got, want)
	}
	if !exists(filepath.Join(wt, "work.txt")) || branches(t, h.Dir) == "" {
		t.Errorf("worktree there %v, branches %q", exists(wt), branches(t, h.Dir))
	}
	if records := madeRecords(t, h); len(records) != 1 || records[0].Removed != 0 {
		t.Errorf("records = %+v", records)
	}
	if log := h.Cleanups(); len(log) != 1 || log[0].Status != "blocked" || log[0].Reason != want || len(log[0].Removed) != 0 {
		t.Errorf("log = %+v", log)
	}
}
