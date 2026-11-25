package chat

import (
	"cmp"
	"context"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nednella/agentos/desktop/internal/run"
)

const (
	filesFor  = 30 * time.Second // how long a folder's list is used before git lists it again
	fileLimit = 30
)

// files lists the files of a folder that git tracks or does not ignore, and keeps each folder's list a while: a
// mention looks the list up at each key.
type files struct {
	run run.Runner

	mu    sync.Mutex
	byDir map[string]listed
}

type listed struct {
	paths []string
	at    time.Time
}

func newFiles(runner run.Runner) *files { return &files{run: runner, byDir: map[string]listed{}} }

// list is the folder's files, relative to it. A folder outside git has none.
func (f *files) list(ctx context.Context, dir string) ([]string, error) {
	f.mu.Lock()
	l, ok := f.byDir[dir]
	f.mu.Unlock()
	if ok && time.Since(l.at) < filesFor {
		return l.paths, nil
	}
	out, err := f.run(ctx, dir, "git", "ls-files", "-z", "-co", "--exclude-standard")
	if err != nil && !strings.Contains(err.Error(), "not a git repository") {
		return nil, err
	}
	paths := []string{}
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	paths = slices.Compact(paths) // git lists a file with a merge conflict once per side
	f.mu.Lock()
	f.byDir[dir] = listed{paths: paths, at: time.Now()}
	f.mu.Unlock()
	return paths, nil
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
