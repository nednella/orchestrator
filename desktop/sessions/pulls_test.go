package sessions_test

import (
	"fmt"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
)

const pollEvery2s = "    pr_watch_method: poll\n    pr_poll_interval: 2s\n"

func repoPulls(entries ...string) string {
	out := "["
	for i, e := range entries {
		if i > 0 {
			out += ","
		}
		out += e
	}
	return out + "]"
}

func repoPull(number int, state, branch, updatedAt string, draft bool) string {
	return fmt.Sprintf(`{"number":%d,"title":"PR %d","draft":%v,"state":%q,"html_url":"https://github.com/acme/widgets/pull/%d","user":{"login":"amy"},"head":{"ref":%q},"updated_at":%q}`,
		number, number, draft, state, number, branch, updatedAt)
}

func TestOpenPullsComeFromTheWatchersList(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: pollEvery2s})
	repoFixture(t, h)
	h.GH.SetPulls(`W/"one"`, repoPulls(repoPull(12, "open", "issue-12", "2026-10-01T12:00:00Z", false)))
	issueSession(h, t, 12)
	eventually(t, "the watcher's first read", func() bool { return h.GH.Calls("api -i") >= 1 })
	before := h.GH.Calls("api -i")
	for range 3 {
		got, err := h.Sessions().PullRequests()
		if err != nil || len(got) != 1 || got[0].Number != 12 {
			t.Fatalf("PullRequests = %+v, %v", got, err)
		}
	}
	if after := h.GH.Calls("api -i"); after != before {
		t.Errorf("PullRequests made %d more gh api calls beside the watcher's", after-before)
	}
}

func TestPullRequestsFetchOnceWithoutAWatcher(t *testing.T) {
	h := newHarness(t)
	repoFixture(t, h)
	h.GH.SetPulls(`W/"one"`, repoPulls(repoPull(5, "open", "feature", "2026-10-01T12:00:00Z", true)))
	for range 2 {
		got, err := h.Sessions().PullRequests()
		if err != nil || len(got) != 1 || !got[0].Draft || got[0].Author != "amy" || got[0].Thread != "" {
			t.Fatalf("PullRequests = %+v, %v", got, err)
		}
	}
	if n := h.GH.Calls("api -i"); n != 1 {
		t.Errorf("two asks made %d gh api calls, want 1", n)
	}
}

func TestPullRequestsNameTheThreadOfTheirBranch(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: pollEvery2s})
	repoFixture(t, h)
	s := issueSession(h, t, 12)
	h.GH.SetPulls(`W/"one"`, repoPulls(
		repoPull(13, "open", "other", "2026-10-01T12:00:00Z", false),
		repoPull(12, "open", "issue-12", "2026-10-03T12:00:00Z", false),
		repoPull(14, "closed", "old", "2026-10-04T12:00:00Z", false),
	))
	got, err := h.Sessions().PullRequests()
	if err != nil || len(got) != 2 {
		t.Fatalf("PullRequests = %+v, %v", got, err)
	}
	if got[0].Number != 12 || got[0].Thread != s.ID || got[0].Branch != "issue-12" || got[0].URL == "" {
		t.Errorf("newest = %+v, want #12 on thread %s", got[0], s.ID)
	}
	if got[1].Number != 13 || got[1].Thread != "" {
		t.Errorf("second = %+v, want #13 with no thread", got[1])
	}
}

func TestA304KeepsTheLastList(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_watch_method: poll\n    pr_poll_interval: 1s\n"})
	repoFixture(t, h)
	h.GH.SetPulls(`W/"one"`, pulls("issue-12", "2026-10-01T12:00:00Z"))
	s := issueSession(h, t, 12)
	eventually(t, "two answers of 304", func() bool { return h.GH.Calls("api -i") >= 3 })
	got, err := h.Sessions().PullRequests()
	if err != nil || len(got) != 0 {
		t.Fatalf("PullRequests = %+v, %v; the fixture list has no state, so no open PR", got, err)
	}
	h.GH.SetPulls(`W/"two"`, repoPulls(repoPull(12, "open", "issue-12", "2026-10-01T12:00:00Z", false)))
	eventually(t, "the changed list", func() bool {
		got, _ := h.Sessions().PullRequests()
		return len(got) == 1 && got[0].Thread == s.ID
	})
	settled := h.GH.Calls("api -i")
	eventually(t, "a 304 of the new list", func() bool { return h.GH.Calls("api -i") >= settled+2 })
	if got, _ := h.Sessions().PullRequests(); len(got) != 1 {
		t.Errorf("a 304 dropped the list: %+v", got)
	}
}
