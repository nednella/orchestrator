package threads

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/folder"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/thread"
	"github.com/nednella/orchestrator/internal/util"
)

// Place is where a new thread runs.
type Place string

const (
	InWorktree Place = "worktree" // a worktree orchestrator makes for the thread, on a branch of its own
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

// placeFor is where a new thread runs: the place picked, else a worktree for an issue thread, since a running Claude
// cannot change folder and the issue's work goes on its branch, else the place the setting names, a worktree when it is
// unset.
func placeFor(setting Place, issue int, picked Place) Place {
	switch {
	case picked != "":
		return picked
	case issue > 0 || setting == "":
		return InWorktree
	}
	return setting
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
// A folder that is no repository has no branch to take, so it gets name; any other failure is an error, never a free name.
func (l *Lifecycle) freeBranch(ctx context.Context, dir, name string) (string, error) {
	taken := func(branch string) (bool, error) {
		out, err := l.git(ctx, dir, "for-each-ref", "--format=%(refname)", "refs/heads/"+branch, "refs/remotes/origin/"+branch)
		return err == nil && strings.TrimSpace(string(out)) != "", err
	}
	branch := name
	for n := 2; ; n++ {
		used, err := taken(branch)
		switch {
		case ctx.Err() != nil:
			return "", ctx.Err()
		case err != nil && !inRepository(dir):
			return name, nil
		case err != nil:
			return "", fmt.Errorf("looking for a free branch: %w", err)
		case !used:
			return branch, nil
		}
		branch = fmt.Sprintf("%s-%d", name, n)
	}
}

// inRepository says whether dir or a folder above it holds a .git, which is how git finds the repository. It is how a
// folder that is no repository is told from a repository git could not read, whatever language git speaks.
func inRepository(dir string) bool {
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return true
		}
		if filepath.Dir(d) == d {
			return false
		}
	}
}

// worktreeDir is the folder orchestrator makes the worktree of branch in, `<project>/trees/<branch>`; a `/` in the branch
// is a folder.
func (l *Lifecycle) worktreeDir(proj project.Project, branch string) string {
	return filepath.Join(proj.Dir, "trees", filepath.FromSlash(branch))
}

// workPlace is where a thread runs: its folder, and the branch it works on there, "" in the project folder.
type workPlace struct {
	dir, branch string
	made        bool   // orchestrator made the worktree for the thread and recorded it
	note        string // what the owner should know of how it was made, "" when nothing
}

// makeWorktree gives the thread its worktree on branch. A project that is not a git repository runs the thread in its
// folder. A branch checked out in a worktree already runs the thread there, unrecorded, so orchestrator never removes it;
// one that another thread has is refused. An existing branch is checked out as it is, never reset; a new one is cut by
// git from base, a commit, or with no base from the local copy of the default branch on origin, with no fetch first. A
// fresh branch is one the thread's title named, which origin is not asked for.
//
// A start that is cut short while git makes its worktree has said so before it began (try.begin, saved with the start), so
// its retry finds the worktree at that folder, with no mark of another thread, and takes it as its own (try.prior).
func (l *Lifecycle) makeWorktree(ctx context.Context, proj project.Project, name thread.Name, branch, base string, fresh bool, try worktreeTry) (workPlace, error) {
	top, err := l.git(ctx, proj.Dir, "rev-parse", "--show-toplevel")
	switch {
	case ctx.Err() != nil:
		return workPlace{}, ctx.Err()
	case err != nil && inRepository(proj.Dir):
		return workPlace{}, fmt.Errorf("reading the repository: %w", err)
	case err != nil:
		return workPlace{dir: proj.Dir}, nil
	}
	if _, err := l.git(ctx, proj.Dir, "check-ref-format", "--branch", branch); err != nil {
		return workPlace{}, fmt.Errorf("%q is not a branch name git takes", branch)
	}
	if n, ok := l.threads.threadOn(proj.Key(), branch, name.String()); ok {
		return workPlace{}, fmt.Errorf("Thread %d works on %s", n, branch)
	}
	trees, err := l.worktrees(ctx, proj.Dir)
	if err != nil {
		return workPlace{}, fmt.Errorf("listing the worktrees: %w", err)
	}
	if dir, ok := trees.byBranch[branch]; ok {
		if _, err := os.Stat(dir); err != nil {
			return workPlace{}, staleCheckout(branch, dir)
		}
		// The worktree an earlier try of this thread's start made is the thread's own: a retry never makes a second.
		own := folder.Canonical(dir)
		switch {
		case l.ours(ctx, proj.Key(), own, name.String()):
			return workPlace{dir: own, branch: branch, made: true}, nil
		case try.prior != "" && own == try.prior && l.unclaimed(ctx, proj.Key(), own, name.String()):
			state, reason := l.madeState(ctx, proj, own)
			switch state {
			case madeWhole:
				return l.adoptWorktree(ctx, proj, name, own, branch, base)
			case madeInterrupted:
				// Git's own lock proves it never finished, so nothing in it is anyone's work: it is made again.
				if err := l.removeInterrupted(ctx, proj, own); err != nil {
					return workPlace{}, err
				}
				try.prior = ""
				return l.makeWorktree(ctx, proj, name, branch, base, fresh, try)
			}
			return workPlace{}, fmt.Errorf("the worktree %s, which an earlier try began to make, is not whole: %s. Put it right or remove it, then retry", own, reason)
		}
		return workPlace{dir: dir, branch: branch}, nil
	}
	base, baseOid, err := l.startOf(ctx, proj.Dir, base)
	if err != nil {
		return workPlace{}, err
	}
	dir := l.worktreeDir(proj, branch)
	if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
		return workPlace{}, fmt.Errorf("%s is already there, so orchestrator cannot make the worktree of %s in it", dir, branch)
	}
	if err := l.excludeTrees(ctx, proj, strings.TrimSpace(string(top))); err != nil {
		return workPlace{}, err
	}
	// A branch picked from the title is new, so origin is not asked for it: that would cost a round trip that always fails.
	branchFound, unreachable, err := l.addWorktree(ctx, proj, dir, branch, !fresh, func() { try.began(dir) }, func() { try.abandoned(dir) })
	if err != nil {
		return workPlace{}, err
	}
	note := ""
	if unreachable {
		note = fmt.Sprintf("Could not reach origin; started from %s as last fetched", l.mainBranch(ctx, proj.Dir))
	}
	if !branchFound {
		try.began(dir)
		if _, err := l.git(context.WithoutCancel(ctx), proj.Dir, "worktree", "add", "--quiet", "-b", branch, dir, base); err != nil {
			try.abandoned(dir)
			return workPlace{}, fmt.Errorf("making the worktree of %s: %w", branch, err)
		}
	}
	ctx = context.WithoutCancel(ctx)
	// As git lists it, with every link resolved: the folder a thread is known by, wherever the project sits.
	dir = folder.Canonical(dir)
	w := Made{Path: dir, Branch: branch, Base: base, BaseOid: baseOid, Thread: name.String(), Created: time.Now().UnixMilli()}
	if err := l.threads.made.Add(proj.Key(), w); err != nil {
		l.removeUnstarted(ctx, proj, dir)
		return workPlace{}, err
	}
	if err := l.mark(ctx, dir, name.String()); err != nil {
		l.removeUnstarted(ctx, proj, dir)
		return workPlace{}, err
	}
	return workPlace{dir: dir, branch: branch, made: true, note: note}, nil
}

// worktreeTry is what a start tells makeWorktree of its earlier tries: prior is the folder one began to make a worktree
// in, "", begin hears the folder before git starts to make one, and clear hears that git failed and left no folder, so a
// worktree made there by hand later is never taken for the start's.
type worktreeTry struct {
	prior string
	begin func(dir string)
	clear func()
}

func (t worktreeTry) abandoned(dir string) {
	if t.clear != nil && gone(dir) {
		t.clear()
	}
}

// madeState is how far git got with a worktree a start began to make.
type madeState int

const (
	madeWhole       madeState = iota // git finished, and nothing in it is changed
	madeInterrupted                  // git still has it locked as initializing: a quit or a timeout cut it off
	madeOther                        // anything else: locked for another reason, changed, or unreadable
)

// madeState says how far git got with the worktree at dir, and for madeOther why it is not taken: a worktree that git
// lists as locked, or whose status is not clean, is no whole one, and only git's own initializing lock says it never was.
func (l *Lifecycle) madeState(ctx context.Context, proj project.Project, dir string) (madeState, string) {
	out, err := l.git(ctx, proj.Dir, "worktree", "list", "--porcelain")
	if err != nil {
		return madeOther, fmt.Sprintf("git could not list the worktrees: %v", util.FirstLine(err.Error()))
	}
	for block := range strings.SplitSeq(string(out), "\n\n") {
		var path, lock string
		var locked bool
		for line := range strings.Lines(block) {
			line = strings.TrimRight(line, "\n")
			switch rest, ok := strings.CutPrefix(line, "worktree "); {
			case ok:
				path = rest
			case line == "locked" || strings.HasPrefix(line, "locked "):
				locked, lock = true, strings.TrimSpace(strings.TrimPrefix(line, "locked"))
			}
		}
		if path == "" || folder.Canonical(path) != dir {
			continue
		}
		switch {
		case locked && lock == "initializing":
			return madeInterrupted, ""
		case locked:
			return madeOther, "git has it locked: " + lock
		}
		status, err := l.git(ctx, dir, "status", "--porcelain")
		switch {
		case err != nil:
			return madeOther, "git cannot read it: " + util.FirstLine(err.Error())
		case strings.TrimSpace(string(status)) != "":
			return madeOther, "it has changes: " + util.FirstLine(strings.TrimSpace(string(status)))
		}
		return madeWhole, ""
	}
	return madeOther, "git does not list it"
}

// removeInterrupted removes a worktree git began to make and did not finish, which its lock says.
func (l *Lifecycle) removeInterrupted(ctx context.Context, proj project.Project, dir string) error {
	if _, err := l.git(context.WithoutCancel(ctx), proj.Dir, "worktree", "remove", "--force", "--force", dir); err != nil {
		return fmt.Errorf("removing the worktree %s that git did not finish: %w", dir, err)
	}
	return nil
}

func (t worktreeTry) began(dir string) {
	if t.begin != nil {
		t.begin(folder.Canonical(dir))
	}
}

// unclaimed says whether nothing says the worktree at dir is another thread's: no live record at its path for another
// thread, and no mark of another thread in its git folder.
func (l *Lifecycle) unclaimed(ctx context.Context, key, dir, thread string) bool {
	if w, ok := l.threads.made.At(key, dir); ok && w.Removed == 0 && w.Thread != thread {
		return false
	}
	gitDir, err := l.linkedGitDir(ctx, dir)
	if err != nil {
		return false
	}
	marker, err := os.ReadFile(filepath.Join(gitDir, markerFile))
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	return err == nil && strings.TrimSpace(string(marker)) == thread
}

// adoptWorktree records the worktree at dir, on branch, as the one orchestrator made for the thread, which git made
// before the record was written.
func (l *Lifecycle) adoptWorktree(ctx context.Context, proj project.Project, name thread.Name, dir, branch, base string) (workPlace, error) {
	base, baseOid, err := l.startOf(ctx, proj.Dir, base)
	if err != nil {
		return workPlace{}, err
	}
	w := Made{Path: dir, Branch: branch, Base: base, BaseOid: baseOid, Thread: name.String(), Created: time.Now().UnixMilli()}
	if err := l.threads.made.Add(proj.Key(), w); err != nil {
		return workPlace{}, err
	}
	if err := l.mark(ctx, dir, name.String()); err != nil {
		return workPlace{}, err
	}
	return workPlace{dir: dir, branch: branch, made: true}, nil
}

// staleCheckout is the error for a branch that git lists as checked out in dir, a folder that is gone.
func staleCheckout(branch, dir string) error {
	return fmt.Errorf("%s is checked out in %s, which is gone: run git worktree prune", branch, dir)
}

// addWorktree checks out branch as it is in a new worktree at dir: the local branch, else origin's, fetched when this
// clone has no copy. When neither has the branch it adds nothing and says branchFound false, and unreachable when that
// is because origin could not be asked.
func (l *Lifecycle) addWorktree(ctx context.Context, proj project.Project, dir, branch string, askOrigin bool, begin, abandoned func()) (branchFound, unreachable bool, err error) {
	args := []string{"worktree", "add", "--quiet", dir, branch}
	switch {
	case l.hasRef(ctx, proj.Dir, "refs/heads/"+branch):
	case !askOrigin && l.hasRef(ctx, proj.Dir, "refs/remotes/origin/"+branch):
		args = []string{"worktree", "add", "--quiet", "--track", "-b", branch, dir, "origin/" + branch}
	case askOrigin:
		onOrigin, unreachable, err := l.onOrigin(ctx, proj.Dir, branch)
		if err != nil {
			return false, false, err
		}
		if !onOrigin {
			return false, unreachable, nil
		}
		args = []string{"worktree", "add", "--quiet", "--track", "-b", branch, dir, "origin/" + branch}
	default:
		return false, false, nil
	}
	// Once git has begun to make the worktree it ends, so a stop of the start finds it whole, with its record.
	if begin != nil {
		begin()
	}
	if _, err := l.git(context.WithoutCancel(ctx), proj.Dir, args...); err != nil {
		if abandoned != nil {
			abandoned()
		}
		return true, false, fmt.Errorf("checking out %s in a worktree: %w", branch, err)
	}
	return true, false, nil
}

// restore puts back the worktree orchestrator made for the thread, at the same path on the same branch, when it is gone, so
// a woken thread never moves folder. A thread with no record has nothing to restore. A folder at the path is left as it
// is: it is the thread's own worktree only when its marker holds the thread's id, and anything else is an error. The
// branch is checked out as it is, the local one else origin's, with plain git: a start makes a new
// branch, and this one exists. A restore that fails leaves the record as it was.
func (l *Lifecycle) restore(ctx context.Context, proj project.Project, thread string) error {
	key := proj.Key()
	w, ok := l.threads.made.Of(key, thread)
	if !ok {
		return nil
	}
	if _, err := os.Lstat(w.Path); !errors.Is(err, fs.ErrNotExist) {
		if !l.marked(ctx, w.Path, thread) {
			return fmt.Errorf("%s exists and is not this thread's worktree", w.Path)
		}
		if w.Removed == 0 {
			return nil
		}
		return l.threads.made.Restored(key, w.Path)
	}
	trees, err := l.worktrees(ctx, proj.Dir)
	if err != nil {
		return fmt.Errorf("listing the worktrees: %w", err)
	}
	if dir, ok := trees.byBranch[w.Branch]; ok && folder.Canonical(dir) != w.Path {
		if _, err := os.Stat(dir); err != nil {
			return staleCheckout(w.Branch, dir)
		}
		return fmt.Errorf("%s is checked out in %s, so thread %d cannot get its worktree back", w.Branch, dir, l.threads.Number(thread))
	}
	if err := l.dropEntry(ctx, proj.Dir, trees, w.Path); err != nil {
		return err
	}
	branchFound, _, err := l.addWorktree(ctx, proj, w.Path, w.Branch, true, nil, nil)
	if err != nil {
		return err
	}
	if !branchFound {
		return fmt.Errorf("%s is gone, so thread %d cannot get its worktree back", w.Branch, l.threads.Number(thread))
	}
	if err := l.mark(ctx, w.Path, thread); err != nil {
		if _, rerr := l.git(ctx, proj.Dir, "worktree", "remove", w.Path); rerr != nil {
			log.Printf("orchestrator: removing the worktree %s, which orchestrator could not mark: %v", w.Path, rerr)
		}
		return err
	}
	return l.threads.made.Restored(key, w.Path)
}

// dropEntry removes git's entry for a worktree at path, whose folder is gone, and no other: a repository-wide prune
// would also drop the entries of the owner's worktrees on a disk that is not mounted, and with them their hold on their
// branches. git refuses to drop a locked entry.
func (l *Lifecycle) dropEntry(ctx context.Context, dir string, trees worktrees, path string) error {
	for _, listed := range append(slices.Collect(maps.Values(trees.byBranch)), trees.detached...) {
		if folder.Canonical(listed) != path {
			continue
		}
		if _, err := l.git(ctx, dir, "worktree", "remove", listed); err != nil {
			return fmt.Errorf("dropping git's entry for the missing worktree %s: %w", listed, err)
		}
		return nil
	}
	return nil
}

// base is what a new branch starts from, the default branch on origin, fetched first, else the local one when origin
// has no copy, and the commit it is at.
func (l *Lifecycle) base(ctx context.Context, dir string) (ref, oid string, err error) {
	branch := l.mainBranch(ctx, dir)
	for _, ref := range []string{"origin/" + branch, branch} {
		if out, err := l.git(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			return ref, strings.TrimSpace(string(out)), nil
		}
	}
	return "", "", fmt.Errorf("%s has no commit to start a worktree from", branch)
}

// fetchBase brings origin's copy of the default branch up to date, which the next new branch starts from. A fetch that
// fails is only logged.
func (l *Lifecycle) fetchBase(ctx context.Context, dir string) {
	if branch, err := l.updateBase(ctx, dir); err != nil {
		log.Printf("orchestrator: fetching %s after a new worktree was made from the copy of it there was: %v", branch, err)
	}
}

func (l *Lifecycle) updateBase(ctx context.Context, dir string) (branch string, err error) {
	branch = l.mainBranch(ctx, dir)
	return branch, l.fetch(ctx, dir, "--quiet", "origin", branch)
}

// startOf is what a new branch starts from, and the commit it is at: base when it is set, else what l.base gives.
func (l *Lifecycle) startOf(ctx context.Context, dir, base string) (ref, oid string, err error) {
	if base == "" {
		return l.base(ctx, dir)
	}
	out, err := l.git(ctx, dir, "rev-parse", "--verify", "--quiet", base+"^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("%s is no commit to start a worktree from", base)
	}
	return base, strings.TrimSpace(string(out)), nil
}

// onOrigin says whether origin has branch, fetching it when this clone has no copy yet: it may have been pushed from
// another clone. Origin saying it has no such branch, or a project with no origin, means it has none. A stop is an
// error, and so is an origin that did not answer in time, so neither starts a new branch of that name. Any other failure
// to ask, as offline, DNS or a refused login, is origin not reached, and unreachable says so. It goes by git's exit code
// for a ref that is not there, not by what git says, which depends on its language.
func (l *Lifecycle) onOrigin(ctx context.Context, dir, branch string) (found, unreachable bool, err error) {
	ref := "refs/remotes/origin/" + branch
	if l.hasRef(ctx, dir, ref) {
		return true, false, nil
	}
	if ctx.Err() != nil {
		return false, false, ctx.Err()
	}
	if !l.hasOrigin(ctx, dir) {
		return false, false, nil
	}
	// Each question to origin has the time to itself.
	timedOut := false
	ask := func(args ...string) error {
		asked, cancel := context.WithTimeout(ctx, cmp.Or(l.originWait, gitTimeout))
		defer cancel()
		var err error
		if args[0] == "fetch" {
			err = l.fetch(asked, dir, args[1:]...)
		} else {
			_, err = l.git(asked, dir, args...)
		}
		if err != nil && asked.Err() != nil && ctx.Err() == nil {
			timedOut = true
		}
		return err
	}
	err = ask("ls-remote", "--exit-code", "origin", "refs/heads/"+branch)
	if err == nil {
		err = ask("fetch", "--quiet", "origin", branch)
	}
	switch exit := (*exec.ExitError)(nil); {
	case err == nil:
		return l.hasRef(ctx, dir, ref), false, nil
	case ctx.Err() != nil:
		return false, false, ctx.Err()
	case timedOut:
		return false, false, errors.New("origin did not answer in time")
	case errors.As(err, &exit) && exit.ExitCode() == 2:
		return false, false, nil
	}
	return false, true, nil
}

// hasOrigin says whether the repository has a remote called origin.
func (l *Lifecycle) hasOrigin(ctx context.Context, dir string) bool {
	_, err := l.git(ctx, dir, "remote", "get-url", "origin")
	return err == nil
}

// excludeTrees keeps the folder of the worktrees orchestrator makes out of the owner's git status, so `git add -A` never
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

// undoWorktree removes the worktree makeWorktree made for a thread that did not start, when it is still the one orchestrator
// made. git refuses to remove one with changes, and it stays.
func (l *Lifecycle) undoWorktree(ctx context.Context, proj project.Project, name thread.Name, w workPlace) {
	if !w.made || !l.ours(ctx, proj.Key(), w.dir, name.String()) {
		return
	}
	l.removeUnstarted(ctx, proj, w.dir)
}

// removeUnstarted removes a worktree orchestrator just made for a thread that did not start, and records it gone. git
// refuses one with changes, and it stays.
func (l *Lifecycle) removeUnstarted(ctx context.Context, proj project.Project, dir string) error {
	// A start stopped halfway has a cancelled ctx, and its worktree must go all the same.
	ctx = context.WithoutCancel(ctx)
	if _, err := l.git(ctx, proj.Dir, "worktree", "remove", dir); err != nil {
		log.Printf("orchestrator: removing the worktree %s of a thread that did not start: %v", dir, err)
		return err
	}
	if _, ok := l.threads.made.At(proj.Key(), dir); !ok {
		return nil
	}
	if err := l.threads.made.Removed(proj.Key(), dir, time.Now()); err != nil {
		log.Printf("orchestrator: recording the worktree %s removed: %v", dir, err)
	}
	return nil
}

// dropStartWorktree removes the worktree the start of the thread name made, when it is the one orchestrator made for the
// thread, not the project's own checkout, the checks of a clean-up pass and no other thread or running terminal is in it,
// and says why it stays, "" when it went or there was none. A worktree that a quit cut short before it was recorded, at
// making, is recorded first. The branch stays.
func (l *Lifecycle) dropStartWorktree(ctx context.Context, proj project.Project, name thread.Name, title, branch, making string) string {
	id, key := name.String(), proj.Key()
	if _, ok := l.threads.made.Of(key, id); !ok && making != "" && !gone(making) && branch != "" {
		if trees, err := l.worktrees(ctx, proj.Dir); err == nil && folder.Canonical(trees.byBranch[branch]) == making && l.unclaimed(ctx, key, making, id) {
			switch state, reason := l.madeState(ctx, proj, making); state {
			case madeWhole:
				if _, err := l.adoptWorktree(ctx, proj, name, making, branch, ""); err != nil {
					return err.Error()
				}
			case madeInterrupted:
				if err := l.removeInterrupted(ctx, proj, making); err != nil {
					return err.Error()
				}
				return ""
			default:
				return fmt.Sprintf("the worktree %s, which the start began to make, is not whole: %s", making, reason)
			}
		}
	}
	w, ok := l.threads.made.Of(key, id)
	if !ok || w.Removed != 0 {
		return ""
	}
	t := target{id: id, title: title, branch: w.Branch, dir: w.Path, proj: proj, made: true}
	trees, err := l.worktrees(ctx, proj.Dir)
	if err != nil {
		return fmt.Sprintf("git could not list the worktrees: %v", err)
	}
	if folder.Inside(proj.Dir, w.Path) || trees.main != "" && folder.Inside(trees.main, w.Path) {
		return fmt.Sprintf("%s holds the project's own checkout, which orchestrator never removes", w.Path)
	}
	if !gone(w.Path) && !l.ours(ctx, key, w.Path, id) {
		return fmt.Sprintf("%s is not the worktree orchestrator made for this thread", w.Path)
	}
	plan, reason := l.planFor(ctx, t, nil, false, l.unchanged(ctx, t))
	if reason != "" || !plan.made {
		return reason
	}
	if gone(plan.worktree) {
		if err := l.dropEntry(ctx, proj.Dir, trees, plan.worktree); err != nil {
			return err.Error()
		}
		if err := l.threads.made.Removed(key, plan.worktree, time.Now()); err != nil {
			log.Printf("orchestrator: recording the worktree %s removed: %v", plan.worktree, err)
		}
		return ""
	}
	if reason := l.threads.closeShellsIn(ctx, key, plan.worktree, false); reason != "" {
		return reason
	}
	if err := l.removeUnstarted(ctx, proj, plan.worktree); err != nil {
		return "git could not remove it: " + util.FirstLine(err.Error())
	}
	return ""
}
