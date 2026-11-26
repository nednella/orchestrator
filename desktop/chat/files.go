package chat

import (
	"cmp"
	"context"
	"log"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/run"
)

const (
	filesFor   = 30 * time.Second // how long a folder's list is used before git lists it again
	gitTimeout = 10 * time.Second
	fileLimit  = 30
)

// files lists the files of a folder that git tracks or does not ignore, and keeps each folder's list a while: a
// mention looks the list up at each key. Calls for a folder that is being listed wait for that listing.
type files struct {
	run run.Runner

	mu    sync.Mutex
	byDir map[string]*listing
}

// listing is one git listing of a folder; paths, err and at are set before done is closed.
type listing struct {
	done  chan struct{}
	paths []string
	err   error
	at    time.Time
}

// stale says whether the listing is done and too old to use.
func (l *listing) stale() bool {
	select {
	case <-l.done:
		return time.Since(l.at) >= filesFor
	default:
		return false
	}
}

func newFiles(runner run.Runner) *files { return &files{run: runner, byDir: map[string]*listing{}} }

// list is the folder's files, relative to it. A folder outside git has none.
func (f *files) list(ctx context.Context, dir string) ([]string, error) {
	f.mu.Lock()
	l := f.byDir[dir]
	if l != nil && !l.stale() {
		f.mu.Unlock()
		select {
		case <-l.done:
			return l.paths, l.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	for d, old := range f.byDir {
		if old.stale() {
			delete(f.byDir, d)
		}
	}
	l = &listing{done: make(chan struct{})}
	f.byDir[dir] = l
	f.mu.Unlock()

	l.paths, l.err = f.read(ctx, dir)
	l.at = time.Now()
	close(l.done)
	if l.err != nil {
		f.mu.Lock()
		if f.byDir[dir] == l {
			delete(f.byDir, dir)
		}
		f.mu.Unlock()
	}
	return l.paths, l.err
}

// read runs git for the folder's files.
func (f *files) read(ctx context.Context, dir string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	out, err := f.run(ctx, dir, "git", "ls-files", "-z", "-co", "--exclude-standard")
	if err != nil && !strings.Contains(err.Error(), "not a git repository") {
		return nil, err
	}
	text := string(out)
	if !strings.HasSuffix(text, "\x00") && text != "" {
		// The runner keeps only the first part of a long output, so the last path may be cut.
		log.Printf("orchestrator: git listed more files in %s than the app reads; the list is cut short", dir)
		text = text[:strings.LastIndex(text, "\x00")+1]
	}
	paths := []string{}
	for p := range strings.SplitSeq(text, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths), nil // git lists a file with a merge conflict once per side
}

// rankFiles is at most limit of the paths that match query, ignoring case, best first: those whose file name starts
// with it, then those that contain it, then those that have its letters in order. A shorter path goes first among
// equals.
func rankFiles(paths []string, query string, limit int) []string {
	q := strings.ToLower(query)
	type ranked struct {
		path string
		rank int
	}
	var found []ranked
	for _, p := range paths {
		lower := strings.ToLower(p)
		switch {
		case strings.HasPrefix(path.Base(lower), q):
			found = append(found, ranked{p, 0})
		case strings.Contains(lower, q):
			found = append(found, ranked{p, 1})
		case subsequence(lower, q):
			found = append(found, ranked{p, 2})
		}
	}
	slices.SortStableFunc(found, func(a, b ranked) int { return cmp.Or(a.rank-b.rank, len(a.path)-len(b.path)) })
	out := []string{}
	for _, r := range found[:min(limit, len(found))] {
		out = append(out, r.path)
	}
	return out
}

// subsequence says whether s has the letters of sub in order, not always side by side.
func subsequence(s, sub string) bool {
	for _, r := range sub {
		i := strings.IndexRune(s, r)
		if i < 0 {
			return false
		}
		s = s[i+len(string(r)):]
	}
	return true
}
