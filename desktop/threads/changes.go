package threads

import (
	"context"
	"strconv"
	"strings"
)

// Changes are the lines a thread's folder changes against origin's default branch: Base is that branch as
// "origin/<name>", "" when the folder has none to compare with.
type Changes struct {
	Base    string `json:"base"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// Changes counts the lines the thread's folder changes in tracked files, committed or not, since its branch left
// origin's default branch, as that is locally; it fetches nothing. A folder that is gone or no repository, and a
// failed git, give no Base.
func (s *Threads) Changes(id string) Changes {
	dir, err := s.threadFolder(id)
	if err != nil {
		return Changes{}
	}
	return s.life.changes(s.context(), dir)
}

func (l *Lifecycle) changes(ctx context.Context, dir string) Changes {
	base := "origin/" + l.mainBranch(ctx, dir)
	fork, err := l.git(ctx, dir, "merge-base", base, "HEAD")
	if err != nil {
		return Changes{}
	}
	out, err := l.git(ctx, dir, "diff", "--numstat", strings.TrimSpace(string(fork)))
	if err != nil {
		return Changes{}
	}
	got := Changes{Base: base}
	for line := range strings.SplitSeq(string(out), "\n") {
		added, rest, _ := strings.Cut(line, "\t")
		removed, _, _ := strings.Cut(rest, "\t")
		a, errA := strconv.Atoi(added)
		r, errR := strconv.Atoi(removed)
		if errA == nil && errR == nil {
			got.Added += a
			got.Removed += r
		}
	}
	return got
}
