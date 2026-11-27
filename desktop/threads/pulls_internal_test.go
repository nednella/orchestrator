package threads

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nednella/orchestrator/internal/project"
)

type listGH struct {
	mu       sync.Mutex
	calls    int
	sentETag []string
	fail     bool
}

func (g *listGH) run(_ context.Context, _, _ string, args ...string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	etag := ""
	if i := slices.Index(args, "-H"); i >= 0 {
		etag = strings.TrimPrefix(args[i+1], "If-None-Match: ")
	}
	g.sentETag = append(g.sentETag, etag)
	switch {
	case g.fail:
		return nil, errors.New("gh api: exit status 1: HTTP 502 secret detail")
	case etag == `W/"one"`:
		return nil, errors.New("gh api: exit status 1: gh: HTTP 304")
	}
	return []byte("HTTP/2.0 200 OK\r\nEtag: W/\"one\"\r\n\r\n" + `[{"number":5,"title":"T","state":"open","html_url":"u","user":{"login":"amy"},"head":{"ref":"b"},"created_at":"2026-09-30T12:00:00Z","updated_at":"2026-10-01T12:00:00Z","labels":[{"name":"bug"}],"requested_reviewers":[{"login":"bo"}]}]`), nil
}

func (g *listGH) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

type noProjects struct{}

func (noProjects) List() []project.Project     { return nil }
func (noProjects) Remove(string) (bool, error) { return false, nil }

type fixedRepo struct{}

func (fixedRepo) Repo(context.Context, string) string     { return "acme/widgets" }
func (fixedRepo) Reload(context.Context, project.Project) {}

type listFixture struct {
	l    *Lifecycle
	gh   *listGH
	proj project.Project
	at   time.Time
}

func newListFixture(t *testing.T) *listFixture {
	t.Helper()
	f := &listFixture{gh: &listGH{}, proj: project.Project{Name: "p", Dir: t.TempDir()}, at: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	f.l = newLifecycle(f.gh.run, nil, t.TempDir(), t.TempDir(), newThreads(Options{LocalDir: t.TempDir(), Projects: noProjects{}}), noActivity{}, func(string, any) {})
	f.l.repos = fixedRepo{}
	f.l.now = func() time.Time { return f.at }
	return f
}

func (f *listFixture) ask() RepoPulls { return f.l.openPulls(context.Background(), f.proj) }

func TestOpenPullsComeFromTheWatchersListWithoutACall(t *testing.T) {
	t.Parallel()
	f := newListFixture(t)
	if _, _, err := f.l.listPulls(context.Background(), f.proj, "acme/widgets", true); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if got := f.ask(); len(got.Pulls) != 1 || got.Error != "" {
			t.Fatalf("PullRequests = %+v", got)
		}
	}
	if f.gh.count() != 1 {
		t.Errorf("gh calls = %d, want only the watcher's", f.gh.count())
	}
}

func TestTheListExpiresAfterAMinute(t *testing.T) {
	t.Parallel()
	f := newListFixture(t)
	f.ask()
	f.at = f.at.Add(59 * time.Second)
	f.ask()
	if f.gh.count() != 1 {
		t.Fatalf("a list 59 s old read again: %d calls", f.gh.count())
	}
	f.at = f.at.Add(2 * time.Second)
	f.ask()
	if f.gh.count() != 2 {
		t.Errorf("a list 61 s old did not read again: %d calls", f.gh.count())
	}
}

func TestAFailedReadKeepsTheListAndBacksOff(t *testing.T) {
	t.Parallel()
	f := newListFixture(t)
	f.ask()
	f.at = f.at.Add(61 * time.Second)
	f.gh.fail = true
	got := f.ask()
	if len(got.Pulls) != 1 || got.Error != pullsFailed || strings.Contains(got.Error, "secret") {
		t.Fatalf("after a failure: %+v", got)
	}
	calls := f.gh.count()
	f.at = f.at.Add(29 * time.Second)
	if again := f.ask(); len(again.Pulls) != 1 || again.Error != pullsFailed || f.gh.count() != calls {
		t.Fatalf("inside the back-off: %+v, %d calls, want %d", again, f.gh.count(), calls)
	}
	f.at = f.at.Add(2 * time.Second)
	f.gh.fail = false
	if ok := f.ask(); ok.Error != "" || len(ok.Pulls) != 1 || f.gh.count() != calls+1 {
		t.Errorf("after the back-off: %+v, %d calls", ok, f.gh.count())
	}
}

func TestWithoutAPollTheOnDemandReadKeepsTheETag(t *testing.T) {
	t.Parallel()
	f := newListFixture(t)
	f.ask()
	f.at = f.at.Add(61 * time.Second)
	if got := f.ask(); len(got.Pulls) != 1 || got.Error != "" {
		t.Fatalf("a 304 lost the list: %+v", got)
	}
	if want := []string{"", `W/"one"`}; !slices.Equal(f.gh.sentETag, want) {
		t.Errorf("ETags sent = %q, want %q", f.gh.sentETag, want)
	}
}

func TestAPollWatcherKeepsItsETagToItself(t *testing.T) {
	t.Parallel()
	f := newListFixture(t)
	f.l.polling[f.proj.Key()] = true
	f.ask()
	if f.l.etags[f.proj.Key()] != "" {
		t.Errorf("an on-demand read took the ETag the poll needs: %q", f.l.etags[f.proj.Key()])
	}
}

func TestOpenPullsCarryTheirLabelsCreationAndAskedPeople(t *testing.T) {
	t.Parallel()
	f := newListFixture(t)
	got := f.ask().Pulls
	if len(got) != 1 || !slices.Equal(got[0].Labels, []string{"bug"}) || !slices.Equal(got[0].Asked, []string{"bo"}) || got[0].CreatedAt == 0 || got[0].CreatedAt >= got[0].UpdatedAt {
		t.Errorf("pull = %+v", got)
	}
}
