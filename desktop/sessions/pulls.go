package sessions

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/util"
)

// pullsFresh is how long the last list of a project's pull requests answers PullRequests without a new read.
const pullsFresh = 60 * time.Second

// RepoPull is an open pull request of the repository.
type RepoPull struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Draft     bool   `json:"draft"`
	Branch    string `json:"branch"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	UpdatedAt int64  `json:"updatedAt"`
	Thread    string `json:"thread"` // the thread that tracks the branch, "" when none
}

// openPulls is the open pull requests of a project's last list and when that list was current.
type openPulls struct {
	at    time.Time
	pulls []RepoPull
}

func openOf(list []listedPull) []RepoPull {
	out := []RepoPull{}
	for _, pr := range list {
		if pr.State != "open" {
			continue
		}
		out = append(out, RepoPull{Number: pr.Number, Title: pr.Title, Draft: pr.Draft, Branch: pr.Head.Ref, URL: pr.HTMLURL, Author: pr.User.Login, UpdatedAt: util.Millis(pr.UpdatedAt)})
	}
	slices.SortStableFunc(out, func(a, b RepoPull) int { return int(b.UpdatedAt - a.UpdatedAt) })
	return out
}

// PullRequests is the current project's open pull requests, newest update first. It answers from the list the
// watcher last read; with none, or one older than a minute, it reads the repo's list once, the same call the watcher makes.
func (s *Sessions) PullRequests() ([]RepoPull, error) {
	return s.life.openPulls(s.context(), s.Current())
}

func (l *Lifecycle) openPulls(ctx context.Context, proj project.Project) ([]RepoPull, error) {
	l.listMu.Lock()
	defer l.listMu.Unlock()
	key := proj.Key()
	l.mu.Lock()
	kept, ok := l.lists[key]
	l.mu.Unlock()
	if !ok || time.Since(kept.at) > pullsFresh {
		repo := l.repos.Repo(ctx, proj.Dir)
		if repo == "" {
			return nil, errors.New("This project has no GitHub repository.")
		}
		l.mu.Lock()
		_, watched := l.watchers[key]
		l.mu.Unlock()
		if _, _, err := l.listPulls(ctx, proj, repo, !watched); err != nil {
			return nil, err
		}
		l.mu.Lock()
		kept = l.lists[key]
		l.mu.Unlock()
	}
	out := slices.Clone(kept.pulls)
	targets := l.targetsOf(proj)
	for i := range out {
		if j := slices.IndexFunc(targets, func(t target) bool { return t.branch == out[i].Branch }); j >= 0 {
			out[i].Thread = targets[j].id
		}
	}
	return out, nil
}
