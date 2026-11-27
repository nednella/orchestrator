package threads_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/internal/warn"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/thread"
)

// fakeTmux puts a tmux on PATH that fails the calls a test asks it to and passes the rest to the real one,
// on whatever private socket the call names.
type fakeTmux struct{ dir string }

func newFakeTmux(t *testing.T) fakeTmux {
	t.Helper()
	real, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is not installed")
	}
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case " $* " in
  *" new-session "*)
    if [ -e %[1]s/fail-new ]; then echo "no room for a session" >&2; exit 1; fi
    if [ -e %[1]s/fail-slow-new ]; then case "$*" in *ORCHESTRATOR_THREAD=*) sleep 1 ;; esac; fi ;;
  *" list-sessions "*)
    if [ -e %[1]s/fail-list ]; then echo "error connecting to /tmp/x (Permission denied)" >&2; exit 1; fi
    if [ -e %[1]s/fail-slow-list ]; then out=$(%[2]s "$@"); rc=$?; sleep 0.4; printf '%%s\n' "$out"; exit $rc; fi
    if [ -e %[1]s/fail-cut-list ]; then out=$(%[2]s "$@"); rc=$?; printf '%%s\n' "$out" | awk -F'\t' -v id="$(cat %[1]s/fail-cut-list)" 'BEGIN{OFS="\t"} $1==id {print $1,$2,$3,$4,$5,$6,""; next} {print}'; exit $rc; fi ;;
  *"@orchestrator-issue"*) if [ -e %[1]s/fail-issue ]; then echo "no space for the option" >&2; exit 1; fi ;;
  *"@orchestrator-access"*) if [ -e %[1]s/fail-access ]; then echo "no space for the option" >&2; exit 1; fi ;;
  *" kill-session "*) if [ -e %[1]s/fail-kill ]; then echo "no server running" >&2; exit 1; fi ;;
esac
exec %[2]s "$@"
`, dir, real)
	if err := os.WriteFile(filepath.Join(dir, "tmux"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return fakeTmux{dir}
}

// fail makes the named call ("new", "list", "issue", "access" or "kill") fail; "slow-new" makes a thread's new-session wait a second, or work again. "slow-list" makes list-sessions
// answer late with what tmux held when it was asked.
func (f fakeTmux) fail(t *testing.T, call string, on bool) {
	t.Helper()
	path := filepath.Join(f.dir, "fail-"+call)
	if on {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// cut makes list-sessions print the line of the session id cut short after its first fields, as tmux has under load,
// or whole again when id is "".
func (f fakeTmux) cut(t *testing.T, id string) {
	t.Helper()
	path := filepath.Join(f.dir, "fail-cut-list")
	if id == "" {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.WriteFile(path, []byte(id), 0o600); err != nil {
		t.Fatal(err)
	}
}

func stateOf(h *apptest.Harness, id string) string {
	got, ok := h.Thread(id)
	if !ok {
		return "missing"
	}
	return string(got.State)
}

func tmuxRun(t *testing.T, h *apptest.Harness, args ...string) {
	t.Helper()
	if out, err := exec.Command("tmux", append([]string{"-L", h.Socket}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("tmux %v: %v: %s", args, err, out)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestAFailedTmuxListingEndsNothing(t *testing.T) {
	h := newHarness(t)
	fake := newFakeTmux(t)
	s, err := h.StartThread("keep running", "")
	if err != nil {
		t.Fatal(err)
	}
	h.Observe(t, s.ID, "PreToolUse", `{"tool_name":"Edit"}`)
	eventually(t, "working", func() bool { return stateOf(h, s.ID) == "working" })

	fake.fail(t, "list", true)
	h.Threads().Refresh()
	h.Threads().Refresh()
	if got := stateOf(h, s.ID); got != "working" {
		t.Errorf("a failed listing changed the thread to %s", got)
	}
	warnings := h.Rec.Warnings()
	if len(warnings) != 1 || warnings[0].Source != "tmux" || !strings.Contains(warnings[0].Message, "Permission denied") {
		t.Fatalf("warnings after two failures = %+v", warnings)
	}

	fake.fail(t, "list", false)
	h.Threads().Refresh()
	if warnings = h.Rec.Warnings(); len(warnings) != 2 || warnings[1] != (warn.Warning{Source: "tmux"}) {
		t.Errorf("recovery was not announced: %+v", warnings)
	}
	if got := stateOf(h, s.ID); got != "working" {
		t.Errorf("after recovery the thread is %s", got)
	}
}

func TestRefreshRevivesAThreadTmuxStillLists(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.StartThread("comes back", "")
	if err != nil {
		t.Fatal(err)
	}
	tmuxRun(t, h, "kill-session", "-t", s.ID)
	h.Threads().Refresh()
	if got := stateOf(h, s.ID); got != "settled" {
		t.Fatalf("a killed thread is %s", got)
	}
	tmuxRun(t, h, "new-session", "-d", "-s", s.ID, "sleep", "60", ";", "set-option", "-t", s.ID, "@orchestrator-access", "supervised")
	h.Threads().Refresh()
	if got := stateOf(h, s.ID); got != "idle" {
		t.Errorf("a thread tmux lists again is %s, want idle", got)
	}
	if rec, _ := bus.ReadState(h.State, s.ID); rec.State == "settled" {
		t.Error("the saved ended record outlived the revival")
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestACutShortSessionLineEndsNothing(t *testing.T) {
	h := newHarness(t)
	fake := newFakeTmux(t)
	s, err := h.StartThread("runs on", "")
	if err != nil {
		t.Fatal(err)
	}
	other, err := h.StartThread("whole", "")
	if err != nil {
		t.Fatal(err)
	}
	fake.cut(t, s.ID)
	for range 3 {
		h.Threads().Refresh()
		for _, id := range []string{s.ID, other.ID} {
			if got := stateOf(h, id); got != "idle" {
				t.Fatalf("a list with a line cut short made thread %s %s", id, got)
			}
		}
	}
	if settled := h.Rec.Settled(); len(settled) != 0 {
		t.Errorf("threads settled although every session runs: %v", settled)
	}

	fake.cut(t, "")
	tmuxRun(t, h, "kill-session", "-t", "="+s.ID+":")
	h.Threads().Refresh()
	if got := stateOf(h, s.ID); got != "settled" {
		t.Errorf("a thread whose session is gone is %s", got)
	}
	if got := stateOf(h, other.ID); got != "idle" {
		t.Errorf("the thread whose session runs is %s", got)
	}
}

func TestRefreshDoesNotReviveAThreadSettledWhileTmuxWasListed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.StartThread("settles", "")
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	h.Threads().AfterList(func() {
		once.Do(func() {
			if err := h.SettleThread(s.ID); err != nil {
				t.Error(err)
			}
		})
	})
	h.Threads().Refresh()
	if got := stateOf(h, s.ID); got != "settled" {
		t.Errorf("the thread is %s after a refresh whose list was read before its settle, want settled", got)
	}
	h.Threads().Refresh()
	if got := stateOf(h, s.ID); got != "settled" {
		t.Errorf("the thread is %s at the next refresh, want settled", got)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestRefreshDoesNotEndAThreadBeingCreated(t *testing.T) {
	h := newHarness(t)
	fake := newFakeTmux(t)
	fake.fail(t, "slow-list", true)
	var wg sync.WaitGroup
	wg.Go(h.Threads().Refresh) // reads the list now, applies it 400 ms later
	time.Sleep(150 * time.Millisecond)
	s, err := h.StartThread("created meanwhile", "")
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if settled := h.Rec.Settled(); len(settled) != 0 {
		t.Errorf("a list read before the creation settled %v", settled)
	}
	if got := stateOf(h, s.ID); got != "idle" {
		t.Errorf("the new thread is %s", got)
	}
}

func TestDeletedThreadStaysGoneAfterALateHook(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.StartThread("done", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.DeleteThread(s.ID, false); err != nil {
		t.Fatal(err)
	}
	h.Observe(t, s.ID, "SessionEnd", `{}`) // Claude's hook, running after the delete
	if rec, _ := bus.ReadState(h.State, s.ID); rec.Thread != "" {
		t.Errorf("the late hook wrote a state file for a deleted thread: %+v", rec)
	}
	if got := h.Restart(t).Threads().List(); len(got) != 0 {
		t.Errorf("a deleted thread came back after a restart: %+v", got)
	}
}

// Not parallel: it puts a fake tmux on the process PATH.
func TestThreadIsKilledWhenItsIssueCannotBeRecorded(t *testing.T) {
	h := newHarness(t)
	fake := newFakeTmux(t)
	fake.fail(t, "issue", true)
	if _, err := h.Threads().Create(h.Threads().Current(), "#7 work", "", false, 7, thread.Supervised); err == nil {
		t.Fatal("Create succeeded although the issue could not be recorded")
	}
	if got := h.Threads().List(); len(got) != 0 {
		t.Errorf("the failed thread is listed: %+v", got)
	}
	out, _ := exec.Command("tmux", "-L", h.Socket, "list-sessions").CombinedOutput()
	if strings.Contains(string(out), "main/") {
		t.Errorf("the failed thread still runs in tmux:\n%s", out)
	}
}

func TestEventsNameTheirProject(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if _, err := h.StartThread("x", ""); err != nil {
		t.Fatal(err)
	}
	if got := h.Rec.ProjectOf("threads"); got != "main" {
		t.Errorf("threads event project = %q", got)
	}
	if got := h.Snapshot().Project.Key; got != "main" {
		t.Errorf("snapshot project key = %q", got)
	}
}

func TestCleanupStopsWhenTheAppCloses(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{CleanupCommand: "sleep 30; : {worktree} {branch}"})
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	pushWork(t, s.Worktree)
	openThenMerge(h)
	eventually(t, "the clean-up to start", func() bool { got, _ := h.Thread(s.ID); return got.Cleanup == "pending" })

	start := time.Now()
	h.App.Stop()
	if took := time.Since(start); took > 8*time.Second {
		t.Errorf("Stop took %v: it waited for the clean-up's command", took)
	}
	log := h.Cleanups()
	if len(log) != 1 || log[0].Status != "blocked" || log[0].Reason != "app closed during clean-up" {
		t.Fatalf("log = %+v", log)
	}
}

func TestPollFailuresAreWarnedAboutOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	issueThread(h, t, 7)
	if err := os.Rename(filepath.Join(h.Dir, ".git"), filepath.Join(h.Dir, "git-gone")); err != nil { // listing the worktrees fails too
		t.Fatal(err)
	}
	h.GH.PRErr = errors.New("gh pr list: exit status 1: HTTP 502 from GitHub\nmore detail")
	h.RefreshPRs()
	h.RefreshPRs()

	bySource := map[string][]string{}
	for _, w := range h.Rec.Warnings() {
		bySource[w.Source] = append(bySource[w.Source], w.Message)
	}
	// The message leaves out the branch: lookups of several branches that fail alike make one warning.
	if got := bySource["pull requests"]; len(got) != 1 || !strings.Contains(got[0], "HTTP 502") || strings.Contains(got[0], "more detail") || strings.Contains(got[0], "issue-7") {
		t.Errorf("pull request warnings = %q", got)
	}
	if got := bySource["worktrees"]; len(got) != 1 || !strings.Contains(got[0], "not a git repository") {
		t.Errorf("worktree warnings = %q", got)
	}

	h.GH.PRErr = nil
	h.RefreshPRs()
	got := bySource["pull requests"]
	for _, w := range h.Rec.Warnings() {
		if w.Source == "pull requests" {
			got = append(got[:0:0], w.Message)
		}
	}
	if len(got) != 1 || got[0] != "" {
		t.Errorf("the last pull request warning = %q, want the recovery", got)
	}
}
