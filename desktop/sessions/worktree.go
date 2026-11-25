package sessions

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nednella/agentos/internal/atomicfile"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
	"github.com/nednella/agentos/internal/util"
)

// Place is where a new thread runs.
type Place string

const (
	InWorktree Place = "worktree" // a worktree agentos makes for the thread, on a branch of its own
	InProject  Place = "project"  // the project folder, on the branch checked out there
)

// parsePlace reads a place; "" leaves it to placeFor.
func parsePlace(s string) (Place, error) {
	switch p := Place(s); p {
	case "", InWorktree, InProject:
		return p, nil
	}
	return "", fmt.Errorf("place must be %s or %s, not %q", InWorktree, InProject, s)
}

// placeFor is where a new thread runs: the place picked, else a worktree for an issue thread at every access, since a
// running Claude cannot change folder and a Read issue thread is often switched to Write next, else a worktree for a
// thread that writes and the project folder for a Read one.
func placeFor(access session.Access, issue int, picked Place) Place {
	switch {
	case picked != "":
		return picked
	case issue > 0 || access != session.Read:
		return InWorktree
	}
	return InProject
}

const branchLength = 40

// branchFromTitle is the branch a thread without an issue is named by: its title in lower case, each run of
// characters other than a-z and 0-9 a `-`, at most 40 characters cut at a `-`; `thread` when nothing is left.
func branchFromTitle(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	name := strings.TrimSuffix(b.String(), "-")
	if len(name) > branchLength {
		cut := name[:branchLength]
		if i := strings.LastIndexByte(name[:branchLength+1], '-'); i > 0 {
			cut = name[:i]
		}
		name = cut
	}
	return cmp.Or(name, "thread")
}

// freeBranch is name, or name with -2, -3… after it, the first that no local or origin branch has. A ref under it,
// such as `<name>/x`, takes the name too: git could not make a branch beside it.
func (l *Lifecycle) freeBranch(ctx context.Context, dir, name string) string {
	taken := func(branch string) bool {
		out, err := l.git(ctx, dir, "for-each-ref", "--format=%(refname)", "refs/heads/"+branch, "refs/remotes/origin/"+branch)
		return err == nil && strings.TrimSpace(string(out)) != ""
	}
	branch := name
	for n := 2; taken(branch); n++ {
		branch = fmt.Sprintf("%s-%d", name, n)
	}
	return branch
}

// worktreeDir is the folder agentos makes the worktree of branch in, `<project>/trees/<branch>`; a `/` in the branch
// is a folder.
func (l *Lifecycle) worktreeDir(proj project.Project, branch string) string {
	return filepath.Join(proj.Dir, "trees", filepath.FromSlash(branch))
}

// workPlace is where a thread runs: its folder, and the branch it works on there, "" in the project folder.
type workPlace struct {
	dir, branch string
	made        bool // agentos made the worktree for the thread and recorded it
}

// makeWorktree gives the thread its worktree on branch. A project that is not a git repository runs the thread in its
// folder. A branch checked out in a worktree already runs the thread there, unrecorded, so agentos never removes it;
// one that another thread has is refused. An existing branch is checked out as it is, never reset; a new one is made by
// the project's session_worktree_command from the default branch on origin, fetched first, and the worktree is the one
// git then lists on the branch, wherever the command put it.
func (l *Lifecycle) makeWorktree(ctx context.Context, proj project.Project, thread session.Name, branch string) (workPlace, error) {
	top, err := l.git(ctx, proj.Dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return workPlace{dir: proj.Dir}, nil
	}
	if _, err := l.git(ctx, proj.Dir, "check-ref-format", "--branch", branch); err != nil {
		return workPlace{}, fmt.Errorf("%q is not a branch name git takes", branch)
	}
	if n, ok := l.sessions.threadOn(proj.Key(), branch, thread.String()); ok {
		return workPlace{}, fmt.Errorf("Thread %d works on %s", n, branch)
	}
	trees, err := l.worktrees(ctx, proj.Dir)
	if err != nil {
		return workPlace{}, fmt.Errorf("listing the worktrees: %w", err)
	}
	if dir, ok := trees.byBranch[branch]; ok {
		if _, err := os.Stat(dir); err != nil {
			return workPlace{}, fmt.Errorf("%s is checked out in %s, which is gone: run git worktree prune", branch, dir)
		}
		return workPlace{dir: dir, branch: branch}, nil
	}
	base, baseOid, err := l.base(ctx, proj.Dir)
	if err != nil {
		return workPlace{}, err
	}
	dir := l.worktreeDir(proj, branch)
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		return workPlace{}, fmt.Errorf("%s is already there, so agentos cannot make the worktree of %s in it", dir, branch)
	}
	if err := l.excludeTrees(ctx, proj, strings.TrimSpace(string(top))); err != nil {
		return workPlace{}, err
	}
	switch {
	case l.hasRef(ctx, proj.Dir, "refs/heads/"+branch):
		if _, err := l.git(ctx, proj.Dir, "worktree", "add", "--quiet", dir, branch); err != nil {
			return workPlace{}, fmt.Errorf("checking out %s in a worktree: %w", branch, err)
		}
	case l.onOrigin(ctx, proj.Dir, branch):
		if _, err := l.git(ctx, proj.Dir, "worktree", "add", "--quiet", "--track", "-b", branch, dir, "origin/"+branch); err != nil {
			return workPlace{}, fmt.Errorf("checking out %s in a worktree: %w", branch, err)
		}
	default:
		out, err := l.projectCommand(ctx, proj, proj.MakeWorktree(), "{branch}", util.ShellQuote(branch), "{worktree}", util.ShellQuote(dir), "{base}", util.ShellQuote(base))
		if err != nil {
			return workPlace{}, fmt.Errorf("session_worktree_command failed for %s: %w", branch, err)
		}
		if trees, err = l.worktrees(ctx, proj.Dir); err != nil {
			return workPlace{}, fmt.Errorf("listing the worktrees: %w", err)
		}
		made, ok := trees.byBranch[branch]
		if !ok {
			return workPlace{}, fmt.Errorf("session_worktree_command made no worktree for %s: %s", branch, cmp.Or(util.FirstLine(strings.TrimSpace(string(out))), "it printed nothing"))
		}
		dir = made
	}
	w := Made{Path: dir, Branch: branch, Base: base, BaseOid: baseOid, Thread: thread.String(), Created: time.Now().UnixMilli()}
	if err := l.sessions.made.Add(proj.Key(), w); err != nil {
		l.removeWorktree(ctx, proj, dir)
		return workPlace{}, err
	}
	if err := l.mark(ctx, dir, thread.String()); err != nil {
		l.removeWorktree(ctx, proj, dir)
		return workPlace{}, err
	}
	return workPlace{dir: dir, branch: branch, made: true}, nil
}

// base is what a new branch starts from, the default branch on origin, fetched first, else the local one when origin
// has no copy, and the commit it is at.
func (l *Lifecycle) base(ctx context.Context, dir string) (ref, oid string, err error) {
	branch := l.mainBranch(ctx, dir)
	if _, err := l.git(ctx, dir, "fetch", "--quiet", "origin", branch); err != nil {
		log.Printf("agentos: fetching %s for a new worktree, which starts from the copy of it there is: %v", branch, err)
	}
	for _, ref := range []string{"origin/" + branch, branch} {
		if out, err := l.git(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			return ref, strings.TrimSpace(string(out)), nil
		}
	}
	return "", "", fmt.Errorf("%s has no commit to start a worktree from", branch)
}

// onOrigin says whether origin has branch, fetching it when this clone has no copy yet: it may have been pushed from
// another clone. A fetch that fails means origin has no such branch.
func (l *Lifecycle) onOrigin(ctx context.Context, dir, branch string) bool {
	ref := "refs/remotes/origin/" + branch
	if l.hasRef(ctx, dir, ref) {
		return true
	}
	if _, err := l.git(ctx, dir, "fetch", "--quiet", "origin", branch); err != nil {
		return false
	}
	return l.hasRef(ctx, dir, ref)
}

// excludeTrees keeps the folder of the worktrees agentos makes out of the owner's git status, so `git add -A` never
// stages one as an embedded repository: the repository's own exclude file, which is never committed, gets the line once.
// A tracked .gitignore is the owner's and is left alone.
func (l *Lifecycle) excludeTrees(ctx context.Context, proj project.Project, top string) error {
	_, common, err := l.gitDirs(ctx, proj.Dir)
	if err != nil {
		return err
	}
	line := "/trees/"
	if rel, err := filepath.Rel(realPath(top), realPath(filepath.Join(proj.Dir, "trees"))); err == nil && !strings.HasPrefix(rel, "..") {
		line = "/" + filepath.ToSlash(rel) + "/"
	}
	path := filepath.Join(common, "info", "exclude")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	for have := range strings.Lines(string(data)) {
		if strings.TrimSpace(have) == line {
			return nil
		}
	}
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		data = append(data, '\n')
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("making %s: %w", filepath.Dir(path), err)
	}
	if err := atomicfile.Write(path, append(data, line+"\n"...), 0o644); err != nil {
		return fmt.Errorf("adding %s to %s: %w", line, path, err)
	}
	return nil
}

func (l *Lifecycle) hasRef(ctx context.Context, dir, ref string) bool {
	_, err := l.git(ctx, dir, "show-ref", "--verify", "--quiet", ref)
	return err == nil
}

// undoWorktree removes the worktree makeWorktree made for a thread that did not start, when it is still the one agentos
// made. git refuses to remove one with changes, and it stays.
func (l *Lifecycle) undoWorktree(ctx context.Context, proj project.Project, thread session.Name, w workPlace) {
	if !w.made || !l.ours(ctx, proj.Key(), w.dir, thread.String()) {
		return
	}
	l.removeWorktree(ctx, proj, w.dir)
}

// removeWorktree removes a worktree agentos just made, and records it gone.
func (l *Lifecycle) removeWorktree(ctx context.Context, proj project.Project, dir string) {
	if _, err := l.git(ctx, proj.Dir, "worktree", "remove", dir); err != nil {
		log.Printf("agentos: removing the worktree %s of a thread that did not start: %v", dir, err)
		return
	}
	if _, ok := l.sessions.made.At(proj.Key(), dir); !ok {
		return
	}
	if err := l.sessions.made.Removed(proj.Key(), dir, time.Now()); err != nil {
		log.Printf("agentos: recording the worktree %s removed: %v", dir, err)
	}
}
