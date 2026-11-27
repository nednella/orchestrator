package threads

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/util"
)

// pullsFresh is how long the last list of a project's pull requests answers PullRequests without a new read, and
// pullsBackoff how long a failed read holds every new one back.
const (
	pullsFresh   = 60 * time.Second
	pullsBackoff = 30 * time.Second
)

// RepoPull is an open pull request of the repository.
type RepoPull struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Draft     bool     `json:"draft"`
	Branch    string   `json:"branch"`
	URL       string   `json:"url"`
	Author    string   `json:"author"`
	CreatedAt int64    `json:"createdAt"`
	UpdatedAt int64    `json:"updatedAt"`
	Labels    []string `json:"labels"`
	Asked     []string `json:"asked"`  // the people whose review is asked for
	Thread    string   `json:"thread"` // the thread that tracks the branch, "" when none
}

// RepoPulls is what PullRequests answers: the last list it knows, and why the latest read failed, "" when it did not.
type RepoPulls struct {
	Pulls []RepoPull `json:"pulls"`
	Error string     `json:"error"`
}

// openPulls is the open pull requests of a project's last list and when that list was current, and when the latest
// read failed (zero when it did not).
type openPulls struct {
	at       time.Time
	pulls    []RepoPull
	failedAt time.Time
}

func openOf(list []listedPull) []RepoPull {
	out := []RepoPull{}
	for _, pr := range list {
		if pr.State != "open" {
			continue
		}
		pull := RepoPull{Number: pr.Number, Title: pr.Title, Draft: pr.Draft, Branch: pr.Head.Ref, URL: pr.HTMLURL, Author: pr.User.Login, CreatedAt: util.Millis(pr.CreatedAt), UpdatedAt: util.Millis(pr.UpdatedAt), Labels: []string{}, Asked: []string{}}
		for _, l := range pr.Labels {
			pull.Labels = append(pull.Labels, l.Name)
		}
		for _, a := range pr.Asked {
			pull.Asked = append(pull.Asked, a.Login)
		}
		out = append(out, pull)
	}
	slices.SortStableFunc(out, func(a, b RepoPull) int { return cmp.Compare(b.UpdatedAt, a.UpdatedAt) })
	return out
}

// PullRequests is the current project's open pull requests, newest update first. It answers from the list the
// watcher last read; with none, or one older than a minute, it reads the repo's list once, the same call the watcher
// makes. A failed read leaves the last list and says so, and no new read starts for 30 seconds.
func (s *Threads) PullRequests() RepoPulls {
	return s.life.openPulls(s.context(), s.Current())
}

const pullsFailed = "Could not read the pull requests from GitHub."

func (l *Lifecycle) openPulls(ctx context.Context, proj project.Project) RepoPulls {
	l.listMu.Lock()
	defer l.listMu.Unlock()
	key := proj.Key()
	l.mu.Lock()
	kept, ok := l.lists[key]
	l.mu.Unlock()
	failed := ""
	switch {
	case !kept.failedAt.IsZero() && l.now().Sub(kept.failedAt) < pullsBackoff:
		failed = pullsFailed
	case !ok || l.now().Sub(kept.at) > pullsFresh:
		repo := l.repos.Repo(ctx, proj.Dir)
		if repo == "" {
			return RepoPulls{Pulls: []RepoPull{}, Error: "This project has no GitHub repository."}
		}
		l.mu.Lock()
		keepETag := !l.polling[key]
		l.mu.Unlock()
		if _, _, err := l.listPulls(ctx, proj, repo, keepETag); err != nil {
			fmt.Fprintf(os.Stderr, "orchestrator: %v\n", err)
			failed = pullsFailed
			l.mu.Lock()
			kept = l.lists[key]
			kept.failedAt = l.now()
			l.lists[key] = kept
			l.mu.Unlock()
		}
		l.mu.Lock()
		kept = l.lists[key]
		l.mu.Unlock()
	}
	out := slices.Clone(kept.pulls)
	if out == nil {
		out = []RepoPull{}
	}
	targets := l.targetsOf(proj)
	for i := range out {
		if j := slices.IndexFunc(targets, func(t target) bool { return t.branch == out[i].Branch }); j >= 0 {
			out[i].Thread = targets[j].id
		}
	}
	return RepoPulls{Pulls: out, Error: failed}
}
