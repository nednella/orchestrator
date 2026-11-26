package sessions_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/internal/activity"
)

func TestPROpenedThenMergedAcrossARestartCountsOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	s := issueSession(h, t, 7)
	h.GH.SetPR(prJSON("OPEN", true, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	if got := h.Events(t, activity.PROpened); len(got) != 1 || got[0].Session != s.ID || got[0].Issue != 7 || got[0].PR != 12 {
		t.Fatalf("pr_opened = %+v", got)
	}

	fresh := h.Restart(t)
	fresh.RefreshPRs()
	h.GH.SetPR(prJSON("MERGED", false, "[]", 0, 0))
	fresh.RefreshPRs()
	fresh.RefreshPRs()
	if got := h.Events(t, activity.PROpened); len(got) != 1 {
		t.Errorf("pr_opened after a restart = %+v", got)
	}
	merged := h.Events(t, activity.PRMerged)
	if len(merged) != 1 || merged[0].Issue != 7 || merged[0].PR != 12 || merged[0].Ms <= 0 {
		t.Fatalf("pr_merged = %+v", merged)
	}
	led, err := fresh.Ledger(30, "")
	if err != nil {
		t.Fatal(err)
	}
	if led.Totals.PRsOpened != 1 || led.Totals.PRsMerged != 1 || led.Totals.IssuesClosed != 1 {
		t.Errorf("ledger totals = %+v", led.Totals)
	}
}

func TestPRFirstSeenMergedCountsNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	issueSession(h, t, 7)
	h.GH.SetPR(prJSON("MERGED", false, "[]", 0, 0))
	h.RefreshPRs()
	h.RefreshPRs()
	if n := len(h.Events(t, activity.PROpened)) + len(h.Events(t, activity.PRMerged)); n != 0 {
		t.Errorf("%d events for a PR never seen open", n)
	}
}

func TestPRClosedUnmerged(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	issueSession(h, t, 7)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(prJSON("CLOSED", false, "[]", 0, 0))
	h.RefreshPRs()
	h.RefreshPRs()
	if got := h.Events(t, activity.PRClosed); len(got) != 1 || got[0].PR != 12 {
		t.Errorf("pr_closed = %+v", got)
	}
	if got := h.Events(t, activity.PRMerged); len(got) != 0 {
		t.Errorf("pr_merged = %+v", got)
	}
}

func TestReviewChecksAndConflictAreRecordedWhenSent(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n    pr_checks_command: \"/fix-checks {n}\"\n    pr_conflict_command: \"/rebase {n}\"\n"})
	repoFixture(t, h)
	s := issueSession(h, t, 12)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(prJSON("OPEN", false, "[]", 1, 0))
	h.RefreshPRs()
	h.RefreshPRs()
	h.GH.SetPR(apptest.WithHead(prJSON("OPEN", false, `[{"status":"COMPLETED","conclusion":"FAILURE"}]`, 1, 0), "aaa"))
	h.RefreshPRs()
	h.RefreshPRs()
	h.GH.SetPR(apptest.WithMergeable(apptest.WithHead(prJSON("OPEN", false, "[]", 1, 0), "bbb"), "CONFLICTING"))
	h.RefreshPRs()
	h.RefreshPRs()
	eventually(t, "all three prompts", func() bool {
		pane := sentTo(t, h, s.ID)
		return strings.Contains(pane, "/address-review 12") && strings.Contains(pane, "/fix-checks 12") && strings.Contains(pane, "/rebase 12")
	})
	r, c, x := h.Events(t, activity.Review), h.Events(t, activity.Checks), h.Events(t, activity.Conflict)
	if len(r) != 1 || len(c) != 1 || len(x) != 1 || r[0].PR != 12 || c[0].Session != s.ID || x[0].Issue != 12 {
		t.Errorf("review %+v, checks %+v, conflict %+v", r, c, x)
	}
}

func TestReviewWithoutACommandIsNotRecorded(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	issueSession(h, t, 12)
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	h.GH.SetPR(prJSON("OPEN", false, "[]", 1, 0))
	h.RefreshPRs()
	if got := h.Events(t, activity.Review); len(got) != 0 {
		t.Errorf("review = %+v", got)
	}
}

func TestWokenThreadIsLabelled(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n"})
	repoFixture(t, h)
	s := issueSession(h, t, 12)
	h.Observe(t, s.ID, "Stop", fmt.Sprintf(`{"session_id":"abc","transcript_path":%q}`, writtenTranscript(t)))
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 0))
	h.RefreshPRs()
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	h.GH.SetPR(prJSON("OPEN", false, "[]", 0, 1))
	h.RefreshPRs()
	eventually(t, "the woken thread's start", func() bool { return len(h.Events(t, activity.SessionStart)) == 2 })
	got := h.Events(t, activity.SessionStart)
	if got[0].Label != "" || got[1].Label != activity.Resumed || got[1].Session != s.ID || got[1].Issue != 12 {
		t.Errorf("session_start = %+v, want the wake labelled resumed under the thread's id", got)
	}
}
