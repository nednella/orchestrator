package sessions

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nednella/agentos/desktop/internal/run"
	"github.com/nednella/agentos/desktop/internal/scoped"
	"github.com/nednella/agentos/desktop/internal/warn"
	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/atomicfile"
	"github.com/nednella/agentos/internal/folder"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
	"github.com/nednella/agentos/internal/term"
	"github.com/nednella/agentos/internal/util"
)

const (
	reconcileEvery = time.Minute // a full look at every pull request: for what a watcher missed, and for conflicts, which no watcher sees
	cleanupSweep   = time.Hour   // how often a clean-up the checks kept is tried again
	cleanupsKept   = 100
	gitTimeout     = 30 * time.Second

	closedReason = "app closed during clean-up"
)

// PR is a pull request as the front end sees it.
type PR struct {
	Number    int    `json:"number"`
	URL       string `json:"url"`
	State     string `json:"state"`
	Checks    string `json:"checks"`
	Comments  int    `json:"comments"` // the comments and reviews by others than the gh user that are not approvals
	UpdatedAt int64  `json:"updatedAt"`

	Conflicting bool `json:"conflicting"` // the branch does not merge cleanly into its base

	Title         string   `json:"title"`
	Review        string   `json:"review"`       // the reviewers' verdict: "changes", "approved", "commented" or ""
	FailedChecks  []string `json:"failedChecks"` // the names of the checks that fail
	RunningChecks int      `json:"runningChecks"`
	PassedChecks  int      `json:"passedChecks"`
	NewComments   int      `json:"newComments"` // the comments and reviews the user has not seen

	ClosedAt int64  `json:"-"` // when the PR merged or closed; 0 while it is live
	HeadOid  string `json:"-"` // the commit the PR's branch ended on
}

// BranchStatus is where a session's branch stands against origin, as git on this Mac sees it.
type BranchStatus struct {
	Base        string `json:"base"`        // the branch pull requests merge into
	Unpushed    int    `json:"unpushed"`    // commits origin's copy of the branch lacks, or the base when origin has none; after a merge NotInMain counts instead
	Uncommitted int    `json:"uncommitted"` // changed and new files in the worktree
	Ahead       int    `json:"ahead"`       // commits the base on origin lacks
	Behind      int    `json:"behind"`      // commits on the base on origin that the branch lacks
	NotInMain   int    `json:"notInMain"`   // after a merge, the commits the merged pull request did not take
}

// Cleanup is one entry of the clean-up log.
type Cleanup struct {
	At           int64    `json:"at"`
	SessionTitle string   `json:"sessionTitle"`
	Issue        int      `json:"issue"`
	PR           int      `json:"pr"`
	Merged       bool     `json:"merged"`
	Status       string   `json:"status"`
	Removed      []string `json:"removed"`
	Reason       string   `json:"reason"`
}

// target is a thread that works on a branch, with the project it belongs to. Its branch is the one agentos made its
// worktree on or the one it reported with agentos track, and no other thread has it.
type target struct {
	id, title string
	issue     int
	branch    string
	dir       string // the folder the thread runs in
	made      bool   // agentos made dir for the thread, and has not removed it
	proj      project.Project
	model     project.Model
	woke      time.Time // the thread's latest start: its first one, or its latest wake
	ended     bool
}

// predates says whether the PR merged or closed before the thread last started: it belongs to an
// earlier thread on the same branch name, or to the thread before its latest wake.
func (t target) predates(pr *PR) bool {
	return pr.ClosedAt > 0 && !t.woke.IsZero() && pr.ClosedAt < t.woke.UnixMilli()
}

// track is what is known about the pull request and clean-up of one session.
type track struct {
	branch     string // the branch the PR was looked up on
	worktree   string
	pr         *PR
	git        *BranchStatus
	attention  string
	cleanup    string
	reason     string
	autoTried  bool
	keptAsleep string // why the latest clean-up at the child's sleep kept its worktree, so the same reason is not logged again
	opened     bool   // the PR is new to the app; the user hears of it once the session is idle
	reading    bool   // a read of the git status runs
	again      bool   // another read was asked for during that one
}

// ack is how much of a PR the user has seen.
type ack struct {
	Comments int  `json:"comments"`
	Checks   bool `json:"checks"` // failing checks were seen
}

// nudge is how much of a PR its session has been woken for.
type nudge struct {
	Comments int    `json:"comments"`
	Head     string `json:"head"`     // the commit whose failing checks the session was told about
	Conflict string `json:"conflict"` // the commit whose conflict with the base the session was told about
}

// Repos is what the lifecycle needs of the issues: a project's repo, and a fresh queue when a PR merges or closes.
type Repos interface {
	Repo(ctx context.Context, dir string) string
	Reload(ctx context.Context, proj project.Project)
}

type noRepos struct{}

func (noRepos) Repo(context.Context, string) string     { return "" }
func (noRepos) Reload(context.Context, project.Project) {}

// Lifecycle follows each issue session's branch, worktree and pull request, wakes the
// session when its PR needs work, and cleans up after it when the PR is merged or closed.
type Lifecycle struct {
	run      run.Runner
	stream   run.Streamer
	dataDir  string
	stateDir string
	emit     func(event string, payload any)
	warn     *warn.Warnings
	sessions *Sessions
	activity Activity
	repos    Repos

	pollMu sync.Mutex // one reconcile at a time
	logMu  sync.Mutex // the clean-up log files

	mu       sync.Mutex
	tracks   map[string]*track
	gone     map[string]bool      // forgotten ids: a poll that was waiting on gh must not write for them
	files    map[string]*prsFile  // by project key
	etags    map[string]string    // by project key: the ETag of the last pull request list
	watchers map[string]*watcher  // by project key
	lists    map[string]openPulls // by project key: the open pull requests of the last list
	polling  map[string]bool      // by project key: a poll watcher reads the list with the ETag
	now      func() time.Time
	listMu   sync.Mutex        // one PullRequests read at a time
	logins   map[string]string // by project folder: the gh user's login
	// polled closes once Run's first poll has read every thread's PR: until then no track says a clean-up is due.
	polled chan struct{}

	sweepEvery time.Duration

	cleanMu   sync.Mutex // guards stopping against cleanups starting
	stopping  bool
	cleanups  sync.WaitGroup // the clean-ups a poll started on its own
	stopped   context.Context
	stopClean context.CancelFunc
}

func newLifecycle(runner run.Runner, stream run.Streamer, dataDir, stateDir string, s *Sessions, activity Activity, emit func(string, any)) *Lifecycle {
	stopped, stop := context.WithCancel(context.Background())
	return &Lifecycle{
		run: runner, stream: stream, dataDir: dataDir, stateDir: stateDir, sessions: s, activity: activity, emit: emit, warn: warn.New(emit), repos: noRepos{},
		tracks: map[string]*track{}, gone: map[string]bool{}, files: map[string]*prsFile{}, etags: map[string]string{}, watchers: map[string]*watcher{}, lists: map[string]openPulls{}, polling: map[string]bool{}, now: time.Now, logins: map[string]string{},
		polled: make(chan struct{}), sweepEvery: cleanupSweep, stopped: stopped, stopClean: stop,
	}
}

// autoCleanup cleans up after a session in the background.
func (l *Lifecycle) autoCleanup(ctx context.Context, id string) {
	l.inBackground(ctx, func(ctx context.Context) { _ = l.Cleanup(ctx, id, false) })
}

// inBackground runs a clean-up until ctx or stopCleanups ends it.
func (l *Lifecycle) inBackground(ctx context.Context, cleanup func(context.Context)) {
	l.cleanMu.Lock()
	defer l.cleanMu.Unlock()
	if l.stopping {
		return
	}
	l.cleanups.Go(func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer context.AfterFunc(l.stopped, cancel)()
		cleanup(ctx)
	})
}

// stopCleanups cancels the clean-ups a poll started and waits until each has logged where it stopped.
func (l *Lifecycle) stopCleanups() {
	if l == nil {
		return
	}
	l.cleanMu.Lock()
	l.stopping = true
	l.cleanMu.Unlock()
	l.stopClean()
	l.cleanups.Wait()
}

// sessionFields is what a session row shows of its track.
type sessionFields struct {
	worktree, attention, cleanup, reason string
	pr                                   *PR
	git                                  *BranchStatus
}

func (l *Lifecycle) fields(id string) sessionFields {
	if l == nil {
		return sessionFields{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	t := l.tracks[id]
	if t == nil {
		return sessionFields{}
	}
	f := sessionFields{worktree: t.worktree, attention: t.attention, cleanup: t.cleanup, reason: t.reason}
	if t.pr != nil {
		pr := *t.pr
		if a, seen := l.prs(session.ProjectKey(id)).Acks[strconv.Itoa(pr.Number)]; seen {
			pr.NewComments = max(0, pr.Comments-a.Comments)
		}
		f.pr = &pr
	}
	if t.git != nil {
		st := *t.git
		f.git = &st
	}
	return f
}

// opened says, once, that the session's PR has appeared since the user last heard of it.
func (l *Lifecycle) opened(id string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	t := l.tracks[id]
	if t == nil || !t.opened {
		return false
	}
	t.opened = false
	return true
}

func (l *Lifecycle) forget(id string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.tracks, id)
	l.gone[id] = true
	key := session.ProjectKey(id)
	l.setAsked(id, 0)
	if f := l.prs(key); slices.Contains(f.Cleaned, id) {
		f.Cleaned = slices.DeleteFunc(f.Cleaned, func(c string) bool { return c == id })
		l.savePRs(key)
	}
}

// setAsked records that the thread's clean-up after PR n waits on the owner, or with 0 that it does not. It needs mu.
func (l *Lifecycle) setAsked(id string, n int) {
	key := session.ProjectKey(id)
	f := l.prs(key)
	if f.Asked[id] == n {
		return
	}
	if n == 0 {
		delete(f.Asked, id)
	} else {
		f.Asked[id] = n
	}
	l.savePRs(key)
}

// retire forgets a session whose clean-up finished, also after a restart, so no second one runs on its settled thread.
func (l *Lifecycle) retire(id string) {
	l.forget(id)
	l.mu.Lock()
	defer l.mu.Unlock()
	key := session.ProjectKey(id)
	f := l.prs(key)
	f.Cleaned = append(f.Cleaned, id)
	l.savePRs(key)
}

// unretire follows a woken thread again, so its next PR settles it and is cleaned up.
func (l *Lifecycle) unretire(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.gone, id)
	key := session.ProjectKey(id)
	f := l.prs(key)
	if slices.Contains(f.Cleaned, id) {
		f.Cleaned = slices.DeleteFunc(f.Cleaned, func(c string) bool { return c == id })
		l.savePRs(key)
	}
}

// retired says whether the lifecycle is done with the session: its row went or its clean-up finished. It needs mu.
func (l *Lifecycle) retired(id string) bool {
	return l.gone[id] || slices.Contains(l.prs(session.ProjectKey(id)).Cleaned, id)
}

// cleanedUp says whether the thread's clean-up finished, so no other one runs. It takes mu.
func (l *Lifecycle) cleanedUp(id string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.retired(id)
}

// workTargets are the sessions on a branch that the lifecycle still follows.
func (l *Lifecycle) workTargets() []target {
	targets := l.sessions.workTargets()
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.DeleteFunc(targets, func(t target) bool { return l.retired(t.id) })
}

// Run reconciles every pull request now and then, keeps a watcher per project in between, and tries the kept
// clean-ups again every sweepEvery. The first reconcile is the start's try: what a clean-up tried is not kept over a
// restart, so it runs the clean-up of each merged or closed PR again as the mode says.
func (l *Lifecycle) Run(ctx context.Context) {
	tick := time.NewTicker(reconcileEvery)
	defer tick.Stop()
	var sweep <-chan time.Time // nil, so never ready, while the sweep is off
	if l.sweepEvery > 0 {
		t := time.NewTicker(l.sweepEvery)
		defer t.Stop()
		sweep = t.C
	}
	l.Poll(ctx)
	close(l.polled)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			l.Poll(ctx)
		case <-sweep:
			l.sweep(ctx)
		}
	}
}

// sweep tries again, with every check, each clean-up the checks kept after the thread's PR merged or closed, since what
// kept it, as a turn, a shell or work since pushed, may be gone. An ask waits on the owner, a running clean-up is left
// to end, and a thread that runs again, as after a wake, is left alone. A try that keeps it for the same reason logs
// nothing, so the log does not fill with one line an hour.
func (l *Lifecycle) sweep(ctx context.Context) {
	for _, t := range l.workTargets() {
		if l.keptAfterPR(t) {
			l.inBackground(ctx, func(ctx context.Context) { _, _, _ = l.cleanupNow(ctx, t.id, false, bySweep) })
		}
	}
}

// keptAfterPR says whether the checks kept the settled thread's clean-up after the PR of its branch merged or closed
// since its latest start. It takes mu.
func (l *Lifecycle) keptAfterPR(t target) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	tr := l.tracks[t.id]
	if !t.ended || tr == nil || tr.cleanup != "blocked" || tr.pr == nil || tr.branch != t.branch {
		return false
	}
	return (tr.pr.State == "merged" || tr.pr.State == "closed") && !t.predates(tr.pr)
}

// group is the sessions of one project that work on a branch.
type group struct {
	proj    project.Project
	targets []target
}

// groups splits the sessions by project.
func groups(targets []target) []group {
	var out []group
	for _, t := range targets {
		i := slices.IndexFunc(out, func(g group) bool { return g.proj.Key() == t.proj.Key() })
		if i < 0 {
			out = append(out, group{proj: t.proj})
			i = len(out) - 1
		}
		out[i].targets = append(out[i].targets, t)
	}
	return out
}

// Poll looks up the worktree and pull request of every session on a branch, live or ended.
func (l *Lifecycle) Poll(ctx context.Context) {
	l.pollMu.Lock()
	defer l.pollMu.Unlock()
	var treeFailure, prFailure string
	for _, g := range groups(l.workTargets()) {
		treeErr, prErr := l.refresh(ctx, g.proj, g.targets, nil)
		if treeErr != nil && treeFailure == "" {
			treeFailure = util.FirstLine(treeErr.Error())
		}
		if prErr != nil && prFailure == "" {
			prFailure = util.FirstLine(prErr.Error())
		}
	}
	if ctx.Err() == nil {
		l.report("worktrees", treeFailure)
		l.report("pull requests", prFailure)
	}
	l.ensureWatchers(ctx)
}

// refresh fetches the pull requests of the project's sessions, all of them or only those on
// the branches named, and acts on what changed: clean-ups, a fresh queue, and wakes.
func (l *Lifecycle) refresh(ctx context.Context, proj project.Project, targets []target, only []string) (treeErr, prErr error) {
	trees, treeErr := l.worktrees(ctx, proj.Dir)
	prs := map[string]*PR{} // by branch
	var fetched []fetch
	var wakes []wake
	reload := false
	for _, t := range sharing(targets, only) {
		pr, err := l.prOf(ctx, t, prs)
		if err != nil {
			prErr = cmp.Or(prErr, err)
			continue
		}
		fetched = append(fetched, fetch{t, pr})
	}
	var statuses map[string]*BranchStatus
	if treeErr == nil {
		statuses = l.statuses(ctx, proj.Dir, trees, fetched)
	}
	var ends []fetch
	var outcomes []outcome
	for _, f := range fetched {
		o := l.update(f.target, trees.byBranch[f.branch], f.pr)
		l.setStatus(f.id, f.branch, statuses[f.branch])
		if o.settle || o.auto {
			ends, outcomes = append(ends, f), append(outcomes, o)
		}
		if o.wake != "" {
			wakes = append(wakes, wake{f.target, o.wake})
		}
		reload = reload || o.queue
	}
	for i, f := range ends {
		l.sessions.whenTurnEnds(f.id, func() { l.end(f.id, outcomes[i]) })
	}
	for _, w := range wakes {
		go l.sessions.wake(ctx, w.target, FromApp(w.prompt))
	}
	l.sessions.changed()
	if reload && ctx.Err() == nil {
		l.repos.Reload(ctx, proj)
	}
	return treeErr, prErr
}

// end settles a thread whose PR merged or closed, unless it went meanwhile, and cleans up after it when o says so. It
// runs under the app's context, not the refresh's: a watcher that stops after the refresh must not stop the clean-up.
func (l *Lifecycle) end(id string, o outcome) {
	if o.settle && l.sessions.Has(id) {
		if err := l.sessions.Settle(id); err != nil {
			fmt.Fprintf(os.Stderr, "agentos: settling thread %s after its PR: %v\n", id, err)
		}
	}
	if o.auto {
		l.autoCleanup(l.sessions.context(), id)
	}
}

// sharing are the targets on a branch named in only. A nil only is all of them.
func sharing(targets []target, only []string) []target {
	if only == nil {
		return targets
	}
	return slices.DeleteFunc(slices.Clone(targets), func(t target) bool { return !slices.Contains(only, t.branch) })
}

// prOf is the PR of the thread's branch, or nil when the branch has none or its PR merged or closed before the thread
// last started, unless the owner was asked to clean up after it and has not answered: a wake does not hide the ask.
// found caches the lookups by branch.
func (l *Lifecycle) prOf(ctx context.Context, t target, found map[string]*PR) (*PR, error) {
	pr, err := l.cachedPR(ctx, t.proj.Dir, t.branch, found)
	if err != nil || pr == nil || t.predates(pr) && !l.asked(t.id, pr.Number) {
		return nil, err
	}
	return pr, nil
}

// asked says whether the thread's clean-up after PR n waits on the owner.
func (l *Lifecycle) asked(id string, n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.prs(session.ProjectKey(id)).Asked[id] == n
}

func (l *Lifecycle) cachedPR(ctx context.Context, dir, branch string, found map[string]*PR) (*PR, error) {
	if pr, ok := found[branch]; ok {
		return pr, nil
	}
	pr, err := l.findPR(ctx, dir, branch)
	if err == nil {
		found[branch] = pr
	}
	return pr, err
}

// fetch is a thread with the PR a fetch found on its branch.
type fetch struct {
	target
	pr *PR
}

// wake is a prompt to send a session.
type wake struct {
	target target
	prompt string
}

// report warns about a failure of the source, or says it works again when failure is "".
func (l *Lifecycle) report(source, failure string) {
	if failure == "" {
		l.warn.Clear(source)
		return
	}
	l.warn.Report(source, failure)
}

// outcome is what a fetched pull request calls for.
type outcome struct {
	auto   bool   // clean up now
	queue  bool   // the PR merged or closed, so the issue leaves the queue
	wake   string // a prompt to send the session, or ""
	settle bool   // the PR merged or closed while the thread runs
}

// update stores what a fetch found and says what it calls for.
func (l *Lifecycle) update(t target, worktree string, pr *PR) outcome {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.retired(t.id) {
		return outcome{}
	}
	key := t.proj.Key()
	tr := l.tracks[t.id]
	if tr == nil {
		tr = &track{}
		l.tracks[t.id] = tr
	}
	prev := tr.pr
	tr.branch, tr.worktree, tr.pr = t.branch, worktree, pr
	var o outcome
	if pr == nil {
		if tr.cleanup == "ask" {
			tr.cleanup = ""
		}
		l.setAsked(t.id, 0)
		return o
	}
	live := pr.State == "draft" || pr.State == "open"
	if _, known := l.prs(key).Acks[strconv.Itoa(pr.Number)]; live && !known {
		tr.opened = true
	}
	l.recordState(t, pr)
	attention := l.attentionFor(key, pr)
	if attention != "" && attention != tr.attention {
		l.emit("attention", map[string]string{"id": t.id, "state": "pr"})
	}
	tr.attention = attention
	if live {
		var kind string
		if o.wake, kind = l.nudgeFor(t.proj, pr); o.wake != "" {
			l.record(t, pr, activity.Event{Kind: kind})
		}
	}
	o.queue = !live && (prev == nil || prev.State != pr.State)
	switch {
	case live:
		if tr.cleanup == "ask" {
			tr.cleanup = ""
		}
		l.setAsked(t.id, 0)
	case tr.cleanup != "":
	case l.prs(key).Asked[t.id] == pr.Number, !wantsAuto(t.proj.CleanupMode, pr.State):
		tr.cleanup = "ask"
		l.setAsked(t.id, pr.Number)
	case !tr.autoTried:
		tr.autoTried = true
		o.auto = true
	}
	changed := prev == nil || prev.State != pr.State
	o.settle = !live && changed && !t.ended && !t.predates(pr)
	return o
}

// recordState records a PR first seen open, and one seen open before that merged or closed, once
// each however many sessions work on it. It needs mu.
func (l *Lifecycle) recordState(t target, pr *PR) {
	key := t.proj.Key()
	opened := l.prs(key).Opened
	n := strconv.Itoa(pr.Number)
	at, known := opened[n]
	now := time.Now().UnixMilli()
	live := pr.State == "draft" || pr.State == "open"
	switch {
	case live && !known:
		opened[n] = now
		l.record(t, pr, activity.Event{Kind: activity.PROpened})
	case !live && known:
		delete(opened, n)
		kind := activity.PRClosed
		if pr.State == "merged" {
			kind = activity.PRMerged
		}
		l.record(t, pr, activity.Event{Kind: kind, Ms: now - at})
	default:
		return
	}
	l.savePRs(key)
}

// record adds an event about the session's PR to the activity record.
func (l *Lifecycle) record(t target, pr *PR, e activity.Event) {
	e.At = time.Now().UnixMilli()
	e.Session, e.Issue, e.PR, e.Model, e.Effort = t.id, t.issue, pr.Number, t.model.Model, t.model.Effort
	l.activity.Record(t.proj.Key(), e)
}

func wantsAuto(c project.Cleanup, state string) bool {
	if state == "merged" {
		return c.OnMerge() == "auto"
	}
	return c.OnClose() == "auto"
}

func (l *Lifecycle) prsPath(key string) string { return filepath.Join(l.dataDir, key, "prs.json") }

// prsFile is what is remembered of a project's PRs: what the user has seen of each, what each PR's
// thread was woken for, and which threads were cleaned up.
type prsFile struct {
	Acks    map[string]ack   `json:"acks"`    // PR number -> ack
	Nudged  map[string]nudge `json:"nudged"`  // PR number -> nudge
	Opened  map[string]int64 `json:"opened"`  // PR number -> when it was first seen open, until it merges or closes
	Cleaned []string         `json:"cleaned"` // the sessions whose clean-up finished; their threads stay, settled
	Asked   map[string]int   `json:"asked"`   // thread id -> the merged or closed PR whose clean-up waits on the owner
}

// prs needs mu.
func (l *Lifecycle) prs(key string) *prsFile {
	if f, ok := l.files[key]; ok {
		return f
	}
	f := &prsFile{}
	if data, err := os.ReadFile(l.prsPath(key)); err == nil {
		_ = json.Unmarshal(data, f)
	}
	if f.Acks == nil {
		f.Acks = map[string]ack{}
	}
	if f.Nudged == nil {
		f.Nudged = map[string]nudge{}
	}
	if f.Opened == nil {
		f.Opened = map[string]int64{}
	}
	if f.Asked == nil {
		f.Asked = map[string]int{}
	}
	l.files[key] = f
	return f
}

// savePRs needs mu.
func (l *Lifecycle) savePRs(key string) {
	data, err := json.Marshal(l.prs(key))
	if err == nil {
		err = atomicfile.Write(l.prsPath(key), data, 0o600)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "agentos: saving PR state: %v\n", err)
	}
}

// attentionFor says what about the PR needs the user: failing checks, or more
// comments than they last saw. It needs mu.
func (l *Lifecycle) attentionFor(key string, pr *PR) string {
	acks := l.prs(key).Acks
	n := strconv.Itoa(pr.Number)
	a, seen := acks[n]
	if !seen {
		a = ack{Comments: pr.Comments}
	}
	if !seen {
		acks[n] = a
		l.savePRs(key)
	}
	if (pr.Checks != "failing" && a.Checks) || pr.Comments < a.Comments {
		a.Checks = a.Checks && pr.Checks == "failing"
		a.Comments = min(a.Comments, pr.Comments)
		acks[n] = a
		l.savePRs(key)
	}
	switch {
	case pr.Checks == "failing" && !a.Checks:
		return "checks"
	case pr.Comments > a.Comments:
		return "comments"
	}
	return ""
}

// nudgeFor is the prompt the PR's session should get, and its kind of event: once per new comment
// or review, once per commit whose checks fail, and once per commit that conflicts with the base.
// Comments a PR had when first seen are not news, nor are those on a draft unless the project sets
// pr_review_drafts. A project that sets no command for an event gets no prompt, but the event still
// counts as seen.
// It needs mu.
func (l *Lifecycle) nudgeFor(proj project.Project, pr *PR) (prompt, kind string) {
	key := proj.Key()
	nudged := l.prs(key).Nudged
	n := strconv.Itoa(pr.Number)
	seen, known := nudged[n]
	if !known {
		seen = nudge{Comments: pr.Comments}
	}
	draftComments := pr.State == "draft" && !proj.ReviewDrafts && pr.Comments > seen.Comments
	fewer := pr.Comments < seen.Comments // a deleted comment must not hide the next one
	if draftComments || fewer {
		seen.Comments = pr.Comments
	}
	head := cmp.Or(pr.HeadOid, "unknown")
	switch {
	case pr.Comments > seen.Comments:
		seen.Comments = pr.Comments
		prompt, kind = proj.ReviewCommand(pr.Number), activity.Review
	case pr.Checks == "failing" && seen.Head != head:
		seen.Head = head
		prompt, kind = proj.ChecksCommand(pr.Number), activity.Checks
	case pr.Conflicting && seen.Conflict != head:
		seen.Conflict = head
		prompt, kind = proj.ConflictCommand(pr.Number), activity.Conflict
	}
	if !known || draftComments || fewer || prompt != "" {
		nudged[n] = seen
		l.savePRs(key)
	}
	return prompt, kind
}

// Ack marks the session's PR comments and failing checks as seen.
func (l *Lifecycle) Ack(id string) error {
	t, err := l.sessions.target(id)
	if err != nil {
		return err
	}
	l.mu.Lock()
	tr := l.tracks[id]
	if tr == nil || tr.pr == nil {
		l.mu.Unlock()
		return nil
	}
	key := t.proj.Key()
	l.prs(key).Acks[strconv.Itoa(tr.pr.Number)] = ack{Comments: tr.pr.Comments, Checks: tr.pr.Checks == "failing"}
	l.savePRs(key)
	tr.attention = ""
	l.mu.Unlock()
	l.sessions.changed()
	return nil
}

// worktrees are a project's git worktrees by branch; main is the project folder's own. A worktree
// in the middle of a rebase is detached, so its branch comes from the rebase.
type worktrees struct {
	main     string
	paths    []string // every worktree's folder, main's first
	byBranch map[string]string
	detached []string
	rebasing map[string]bool
}

func (l *Lifecycle) git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	return l.run(ctx, dir, "git", args...)
}

func (l *Lifecycle) worktrees(ctx context.Context, dir string) (worktrees, error) {
	out, err := l.git(ctx, dir, "worktree", "list", "--porcelain")
	if err != nil {
		return worktrees{}, err
	}
	w := parseWorktrees(string(out))
	for _, path := range w.detached {
		if branch := l.rebasingBranch(ctx, path); branch != "" {
			w.byBranch[branch] = path
			w.rebasing[branch] = true
		}
	}
	return w, nil
}

// rebasingBranch is the branch a rebase in the worktree is on, or "" when no rebase runs there or
// git cannot read the worktree, as when its folder is gone.
func (l *Lifecycle) rebasingBranch(ctx context.Context, worktree string) string {
	gitDir, _, err := l.gitDirs(ctx, worktree)
	if err != nil {
		return ""
	}
	for _, state := range []string{"rebase-merge", "rebase-apply"} {
		if head, err := os.ReadFile(filepath.Join(gitDir, state, "head-name")); err == nil {
			return strings.TrimPrefix(strings.TrimSpace(string(head)), "refs/heads/")
		}
	}
	return ""
}

func parseWorktrees(out string) worktrees {
	w := worktrees{byBranch: map[string]string{}, rebasing: map[string]bool{}}
	path := ""
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
			w.paths = append(w.paths, path)
			if w.main == "" {
				w.main = path
			}
		case strings.HasPrefix(line, "branch refs/heads/"):
			w.byBranch[strings.TrimPrefix(line, "branch refs/heads/")] = path
		case line == "detached":
			w.detached = append(w.detached, path)
		}
	}
	return w
}

func samePath(a, b string) bool { return realPath(a) == realPath(b) }

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func (l *Lifecycle) findPR(ctx context.Context, dir, branch string) (*PR, error) {
	me, err := l.login(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("finding the gh user: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, run.GHTimeout)
	defer cancel()
	out, err := l.run(ctx, dir, "gh", "pr", "list", "--head", branch, "--state", "all", "--json",
		"number,title,url,state,isDraft,headRefOid,mergeable,statusCheckRollup,comments,reviews,updatedAt,closedAt", "--limit", "1")
	if err != nil {
		return nil, fmt.Errorf("finding a pull request: %w", err)
	}
	return parsePR(out, me)
}

// login is the gh user's login, as the session's own comments and replies carry it.
func (l *Lifecycle) login(ctx context.Context, dir string) (string, error) {
	l.mu.Lock()
	me, ok := l.logins[dir]
	l.mu.Unlock()
	if ok {
		return me, nil
	}
	ctx, cancel := context.WithTimeout(ctx, run.GHTimeout)
	defer cancel()
	out, err := l.run(ctx, dir, "gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	me = strings.TrimSpace(string(out))
	l.mu.Lock()
	l.logins[dir] = me
	l.mu.Unlock()
	return me, nil
}

type author struct {
	Login string `json:"login"`
}

// review is one review of a pull request, as gh lists them, oldest first. Each reply in a review
// thread is a review of its own.
type review struct {
	State  string `json:"state"`
	Author author `json:"author"`
}

// parsePR reads what gh pr list printed. me is the gh user, whose own comments and reviews ask
// nothing of the session.
func parsePR(data []byte, me string) (*PR, error) {
	var raw []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		URL       string `json:"url"`
		State     string `json:"state"`
		IsDraft   bool   `json:"isDraft"`
		HeadOid   string `json:"headRefOid"`
		Mergeable string `json:"mergeable"`
		Rollup    []struct {
			Name       string `json:"name"`    // a check run's
			Context    string `json:"context"` // a commit status's
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			State      string `json:"state"`
		} `json:"statusCheckRollup"`
		Comments []struct {
			Author author `json:"author"`
		} `json:"comments"`
		Reviews   []review  `json:"reviews"`
		UpdatedAt time.Time `json:"updatedAt"`
		ClosedAt  time.Time `json:"closedAt"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("reading gh pr list: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	r := raw[0]
	pr := &PR{Number: r.Number, Title: r.Title, URL: r.URL, UpdatedAt: util.Millis(r.UpdatedAt), ClosedAt: util.Millis(r.ClosedAt), HeadOid: r.HeadOid, Conflicting: r.Mergeable == "CONFLICTING", FailedChecks: []string{}}
	for _, c := range r.Comments {
		if c.Author.Login != me {
			pr.Comments++
		}
	}
	for _, v := range r.Reviews {
		if v.Author.Login != me && v.State != "APPROVED" { // an approval asks nothing of the session
			pr.Comments++
		}
	}
	pr.Review = verdict(r.Reviews)
	switch {
	case r.State == "MERGED":
		pr.State = "merged"
	case r.State == "CLOSED":
		pr.State = "closed"
	case r.IsDraft:
		pr.State = "draft"
	default:
		pr.State = "open"
	}
	for _, c := range r.Rollup {
		switch {
		case slices.Contains([]string{"FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE", "ERROR"}, c.Conclusion),
			c.State == "FAILURE" || c.State == "ERROR":
			pr.FailedChecks = append(pr.FailedChecks, cmp.Or(c.Name, c.Context))
		case c.State == "PENDING" || c.State == "EXPECTED" || (c.State == "" && c.Status != "" && c.Status != "COMPLETED"):
			pr.RunningChecks++
		default:
			pr.PassedChecks++
		}
	}
	switch {
	case len(pr.FailedChecks) > 0:
		pr.Checks = "failing"
	case pr.RunningChecks > 0:
		pr.Checks = "pending"
	case pr.PassedChecks > 0:
		pr.Checks = "passing"
	default:
		pr.Checks = "none"
	}
	return pr, nil
}

// verdict sums up the latest verdict of each reviewer: changes requested by anyone, else an approval,
// else a comment. As on GitHub, a comment does not undo a verdict, and a dismissal does.
func verdict(reviews []review) string {
	latest := map[string]string{}
	commented := false
	for _, v := range reviews {
		switch v.State {
		case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
			latest[v.Author.Login] = v.State
		case "COMMENTED":
			commented = true
		}
	}
	verdicts := slices.Collect(maps.Values(latest))
	switch {
	case slices.Contains(verdicts, "CHANGES_REQUESTED"):
		return "changes"
	case slices.Contains(verdicts, "APPROVED"):
		return "approved"
	case commented:
		return "commented"
	}
	return ""
}

// Cleanup removes the worktree agentos made for a thread and its temp folder, and settles the thread, which stays with
// its branch. Without force it first checks that nothing would be lost.
func (l *Lifecycle) Cleanup(ctx context.Context, id string, force bool) error {
	_, _, err := l.cleanupNow(ctx, id, force, byOwner)
	return err
}

// cleanupBy is what started a clean-up.
type cleanupBy int

const (
	byOwner  cleanupBy = iota // the owner, or a PR that merged or closed
	byDelete                  // a delete that takes the worktree with the thread
	bySweep                   // the sweep, which logs a clean-up kept again only when its reason changed
)

// cleanupNow is Cleanup, which it runs to the end before it returns what it removed and the reason the clean-up
// stopped, "" when it went through. A clean-up that cannot start is an error, as one of a thread being deleted, unless
// the delete runs it. The threads of family, the Delete's that runs it, do not count as others in the worktree.
func (l *Lifecycle) cleanupNow(ctx context.Context, id string, force bool, by cleanupBy, family ...string) (removed []string, reason string, err error) {
	t, err := l.sessions.target(id)
	if err != nil {
		return nil, "", err
	}
	l.mu.Lock()
	if l.retired(id) {
		l.mu.Unlock()
		return nil, "", errors.New("this thread was cleaned up already")
	}
	tr := l.tracks[id]
	if tr == nil {
		tr = &track{}
		l.tracks[id] = tr
	}
	if tr.cleanup == "pending" {
		l.mu.Unlock()
		return nil, "", errors.New("clean-up is already running")
	}
	if tr.pr != nil && tr.branch != t.branch {
		l.mu.Unlock()
		return nil, "", fmt.Errorf("%s is no longer the branch of this thread", tr.branch)
	}
	was, wasReason := tr.cleanup, tr.reason
	tr.cleanup, tr.reason = "pending", ""
	var pr *PR
	if tr.pr != nil {
		c := *tr.pr
		pr = &c
	}
	l.mu.Unlock()
	// The mark goes on before the look at a delete's or a wake's, as each marks itself before it looks for a running
	// clean-up: one of the two always sees the other.
	if err := l.sessions.cleanupRefused(id, by); err != nil {
		l.mu.Lock()
		tr.cleanup, tr.reason = was, wasReason
		l.mu.Unlock()
		return nil, "", err
	}
	l.sessions.changed()

	removed, reason = l.removeAll(ctx, t, pr, force, family...)
	again := by == bySweep && was == "blocked" && reason == wasReason && len(removed) == 0 && ctx.Err() == nil
	if !again {
		reason = l.logCleanup(ctx, t, pr, removed, reason)
	}
	if reason != "" {
		l.mu.Lock()
		tr.cleanup, tr.reason = "blocked", reason
		l.mu.Unlock()
		l.sessions.changed()
		return removed, reason, nil
	}
	l.retire(id)
	l.sessions.changed()
	return removed, "", nil
}

// checkCleanup runs the checks a clean-up of the thread id starts with, and removes nothing. It says why one keeps the
// worktree, "" when none does, and logs a kept worktree and shows it on the row, as a clean-up that stopped there does.
// The threads of family, the Delete's that asks, do not count as others in the worktree.
func (l *Lifecycle) checkCleanup(ctx context.Context, id string, family ...string) (string, error) {
	t, err := l.sessions.target(id)
	if err != nil {
		return "", err
	}
	l.mu.Lock()
	tr := l.tracks[id]
	if tr == nil {
		tr = &track{}
		l.tracks[id] = tr
	}
	if tr.cleanup == "pending" {
		l.mu.Unlock()
		return "", errors.New("clean-up is already running")
	}
	var pr *PR
	if tr.pr != nil {
		c := *tr.pr
		pr = &c
	}
	l.mu.Unlock()
	_, reason := l.planFor(ctx, t, pr, false, l.unchanged(ctx, t), family...)
	if reason == "" {
		return "", nil
	}
	reason = l.logCleanup(ctx, t, pr, []string{}, reason)
	l.mu.Lock()
	tr.cleanup, tr.reason = "blocked", reason
	l.mu.Unlock()
	l.sessions.changed()
	return reason, nil
}

// logCleanup adds a clean-up to the log and returns its reason, which says when the app closed during it.
func (l *Lifecycle) logCleanup(ctx context.Context, t target, pr *PR, removed []string, reason string) string {
	if reason != "" && ctx.Err() != nil {
		reason = closedReason
	}
	entry := Cleanup{At: time.Now().UnixMilli(), SessionTitle: t.title, Issue: t.issue, Status: "done", Removed: removed, Reason: reason}
	if pr != nil {
		entry.PR, entry.Merged = pr.Number, pr.State == "merged"
	}
	if reason != "" {
		entry.Status = "blocked"
	}
	key := t.proj.Key()
	if err := l.appendCleanup(key, entry); err != nil {
		fmt.Fprintf(os.Stderr, "agentos: logging a clean-up: %v\n", err)
	}
	l.activity.Record(key, activity.Event{At: entry.At, Kind: activity.Cleanup, Session: t.id, Issue: t.issue, PR: entry.PR, Label: entry.Status})
	l.sessions.changed()
	l.emit("cleanups", scoped.Of(key, l.Cleanups(key)))
	return reason
}

// childAsleep removes the worktree agentos made for the child thread id, which went to sleep, when that loses nothing:
// planFor's checks pass, with a thread asleep in it and any shell of the child, idle or busy, counting too. The owner
// may have opened that shell after the child's work ended, so this clean-up never closes one, and as it runs only at
// the sleep, a worktree a shell kept stays until the child sleeps again, the clean-up after its PR or Clean up now
// removes it, or the child is deleted; those close the child's shells. A worktree with no commit since the one it
// started at has only its changes to lose, so origin is not asked. Its branch stays, and so does the thread, whose wake
// puts the worktree back. Anything else keeps the worktree for the clean-up after its pull request, and the log says
// why, unless it said so for the child's last sleep. A child in its parent's folder, or in one agentos did not make for
// it, has nothing to remove. A Delete of the child takes its worktree only when asked.
func (l *Lifecycle) childAsleep(ctx context.Context, id string) {
	t, err := l.sessions.target(id)
	if err != nil || !t.ended || !t.made {
		return
	}
	if !l.ours(ctx, t.proj.Key(), t.dir, id) {
		return
	}
	tr, before, ok := l.startChildCleanup(id)
	if !ok {
		return
	}
	defer func() {
		l.mu.Lock()
		if tr.cleanup == "pending" {
			tr.cleanup = before
		}
		l.mu.Unlock()
		l.sessions.changed()
	}()

	plan, reason := l.planFor(ctx, t, nil, false, l.unchanged(ctx, t))
	if user := l.sessions.usersOf(t.dir, true, id); reason == "" && user != "" {
		reason = fmt.Sprintf("Thread %s also works in this worktree", user)
	}
	if reason == "" {
		reason = l.shellOf(ctx, id)
	}
	var removed []string
	if reason == "" {
		removed, reason = l.removeWorktree(ctx, t, &plan)
	}
	l.mu.Lock()
	again := reason != "" && reason == tr.keptAsleep && len(removed) == 0 && ctx.Err() == nil
	tr.keptAsleep = reason
	l.mu.Unlock()
	if !again {
		l.logCleanup(ctx, t, nil, removed, reason)
	}
}

// shellOf says why a shell of the thread keeps its worktree: one is open, or tmux could not say. It holds shellMu, so a
// shell that started before the clean-up's mark is listed, and NewShell refuses one after it.
func (l *Lifecycle) shellOf(ctx context.Context, id string) string {
	s := l.sessions
	s.shellMu.Lock()
	defer s.shellMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	shells, err := s.tmux.ShellPaths(ctx)
	if err != nil {
		return shellsUnlisted(err)
	}
	if slices.ContainsFunc(shells, func(sh term.Shell) bool { return sh.Thread == id }) {
		return "a shell of this thread is open"
	}
	return ""
}

// startChildCleanup marks the clean-up of the asleep child id as running and returns its track and the clean-up state
// it had, unless the child runs again, a delete of it runs or it went, or another clean-up of it runs or finished. It
// holds createMu, which a wake holds from its check that no clean-up runs until the thread is live, so a wake either
// comes first and the child is found running, or waits for the clean-up; a delete holds it as it marks itself and looks
// for a clean-up, so either the delete or the clean-up goes on, never both.
func (l *Lifecycle) startChildCleanup(id string) (tr *track, before string, ok bool) {
	s := l.sessions
	s.createMu.Lock()
	defer s.createMu.Unlock()
	s.mu.Lock()
	_, live := s.liveInfo(id)
	gone := live || s.deleting[id] || s.deleted[id]
	s.mu.Unlock()
	if gone {
		return nil, "", false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tr = l.tracks[id]
	if tr == nil {
		tr = &track{}
		l.tracks[id] = tr
	}
	if l.retired(id) || tr.cleanup == "pending" {
		return nil, "", false
	}
	before = tr.cleanup
	tr.cleanup = "pending"
	return tr, before, true
}

// unchanged says whether the worktree agentos made for t holds no commit since the one it started at.
func (l *Lifecycle) unchanged(ctx context.Context, t target) bool {
	w, ok := l.sessions.made.Of(t.proj.Key(), t.id)
	if !ok || w.BaseOid == "" {
		return false
	}
	head, err := l.head(ctx, t.proj.Dir, t.dir, t.branch)
	if err != nil {
		return false
	}
	n, err := l.commitsNotIn(ctx, t.proj.Dir, w.BaseOid, head)
	return err == nil && n == 0
}

// removeAll runs the clean-up steps in order: the checks, the settle of a live thread, the checks again, since its agent
// may have changed the worktree until it stopped, each measuring the worktree from the commit it started at as a
// child's sleep does, the worktree's removal, and the rest of the settle. A non-empty reason means it stopped before the
// end. A step that finds the context cancelled stops with closedReason, and one that finds the thread woken stops and
// keeps what is left, as does the end. The threads of family, a Delete's, do not count as others in the worktree.
func (l *Lifecycle) removeAll(ctx context.Context, t target, pr *PR, force bool, family ...string) (removed []string, reason string) {
	removed = []string{}
	if _, reason := l.planFor(ctx, t, pr, force, l.unchanged(ctx, t), family...); reason != "" {
		return removed, reason
	}
	if ctx.Err() != nil {
		return removed, closedReason
	}
	if !t.ended {
		if err := l.sessions.Settle(t.id); err != nil {
			return removed, fmt.Sprintf("putting the thread to sleep failed: %v", err)
		}
		t.ended = true
	}
	plan, reason := l.planFor(ctx, t, pr, force, l.unchanged(ctx, t), family...)
	if reason != "" {
		return removed, reason
	}
	for _, step := range []func(context.Context, target, *cleanupPlan) ([]string, string){l.removeWorktree, l.settleSession} {
		if ctx.Err() != nil {
			return removed, closedReason
		}
		if reason := l.woke(t.id); reason != "" {
			return removed, reason
		}
		done, reason := step(ctx, t, &plan)
		removed = append(removed, done...)
		if reason != "" {
			return removed, reason
		}
	}
	return removed, l.woke(t.id)
}

// cleanupPlan is what a clean-up found out before it removes anything.
type cleanupPlan struct {
	branchPlan
	force  bool
	family []string // the threads of the Delete that runs the clean-up, whose shells in the worktree go with it
}

// branchPlan is the thread's branch and the worktree agentos made for it.
type branchPlan struct {
	branch   string
	worktree string // the recorded folder of the worktree, or ""
	made     bool   // agentos made the worktree for the thread and has not removed it, so the clean-up removes it
}

// planFor finds the worktree agentos made for the thread, and unless forced, says why removing it could lose work or
// pull it from under something that uses it: a rebase, changes or commits only it holds, a running turn or held
// prompts, another thread or a shell in it. Each check reads git, tmux and the threads afresh, so a second call sees
// what changed since the first. A thread with no such worktree, or one removed already, has nothing on disk to lose:
// the branch always stays. The worktree goes only while it is on the thread's branch, the one whose PR merged or
// closed. With unchanged, the worktree holds no commit since the one it started at, so only its changes count and
// origin is not fetched. The threads of family, a Delete's, do not count as others in the worktree, and neither do
// their shells or the thread's own, which the worktree's removal closes.
func (l *Lifecycle) planFor(ctx context.Context, t target, pr *PR, force, unchanged bool, family ...string) (cleanupPlan, string) {
	p := cleanupPlan{branchPlan: branchPlan{branch: t.branch}, force: force, family: family}
	w, ok := l.sessions.made.Of(t.proj.Key(), t.id)
	if !ok || w.Removed != 0 {
		return p, ""
	}
	p.worktree, p.made = w.Path, true
	if gone(w.Path) {
		return p, ""
	}
	dir := t.proj.Dir
	trees, err := l.worktrees(ctx, dir)
	switch {
	case err != nil:
		return p, fmt.Sprintf("git could not list the worktrees: %v", err)
	case force:
		return p, ""
	case !t.made || !samePath(trees.byBranch[t.branch], w.Path):
		return p, fmt.Sprintf("the worktree %s is not on %s", w.Path, t.branch)
	case trees.rebasing[t.branch]:
		return p, fmt.Sprintf("the worktree of %s is in the middle of a rebase", t.branch)
	}
	switch {
	case unchanged:
		if reason := l.uncommitted(ctx, dir, w.Path, t.branch); reason != "" {
			return p, reason
		}
	default:
		if err := l.fetchOrigin(ctx, dir); err != nil {
			fmt.Fprintf(os.Stderr, "agentos: fetching origin before a clean-up: %v\n", err)
			return p, "Could not fetch origin, so agentos cannot tell whether every commit is pushed"
		}
		if reason := l.unsafe(ctx, dir, w.Path, t.branch, pr, l.mainBranch(ctx, dir)); reason != "" {
			return p, reason
		}
	}
	if reason := l.sessions.pending(t.id); reason != "" {
		return p, reason
	}
	ours := slices.Concat(family, []string{t.id})
	if user := l.sessions.usersOf(w.Path, false, ours...); user != "" {
		return p, fmt.Sprintf("Thread %s also works in this worktree", user)
	}
	return p, l.shellIn(ctx, t.proj.Key(), w.Path, ours...)
}

// shellIn says why a shell keeps the worktree: one is open in it, or tmux could not say, or a shell of the project is
// open in a folder tmux does not know. It is "" when none is. The shells of threads do not count: what asks closes
// them when the worktree goes.
func (l *Lifecycle) shellIn(ctx context.Context, key, worktree string, threads ...string) string {
	ctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	shells, err := l.sessions.tmux.ShellPaths(ctx)
	if err != nil {
		return shellsUnlisted(err)
	}
	unknown := false
	for _, sh := range shells {
		switch {
		case slices.Contains(threads, sh.Thread):
		case sh.Path == "":
			unknown = unknown || sh.Name.Project == key
		case folder.Inside(sh.Path, worktree):
			return "a shell is open in this worktree"
		}
	}
	if unknown {
		return "a shell whose folder is unknown is open"
	}
	return ""
}

// shellsUnlisted is why a shell may keep a worktree or its files when tmux could not list the open shells.
func shellsUnlisted(err error) string {
	return fmt.Sprintf("tmux could not list the open shells: %v", err)
}

// removeWorktree removes the worktree agentos made for the thread with the project's session_cleanup_command, and
// records it removed. Forced or not, it never runs the command on the project folder, git's main worktree, a folder
// that holds either, or a worktree that is not the one agentos made for the thread, as one a human made again at a path
// agentos used, and it keeps everything when the thread woke since the clean-up settled it. Once nothing keeps the
// worktree, it closes the thread's shells and those of the plan's family in the worktree, so none is left in a folder
// that is gone. A worktree whose folder is gone already counts as removed.
func (l *Lifecycle) removeWorktree(ctx context.Context, t target, p *cleanupPlan) (removed []string, reason string) {
	if !p.made {
		return nil, ""
	}
	dir, path := t.proj.Dir, p.worktree
	if path == "" {
		return nil, "agentos has no folder recorded for the worktree of this thread"
	}
	trees, err := l.worktrees(ctx, dir)
	if err != nil {
		return nil, fmt.Sprintf("git could not list the worktrees: %v", err)
	}
	if folder.Inside(dir, path) || trees.main != "" && folder.Inside(trees.main, path) {
		return nil, fmt.Sprintf("%s holds the project's own checkout, which agentos never removes", path)
	}
	if reason := l.woke(t.id); reason != "" {
		return nil, reason
	}
	if !gone(path) && !l.ours(ctx, t.proj.Key(), path, t.id) {
		return nil, fmt.Sprintf("%s is not the worktree agentos made for this thread", path)
	}
	l.sessions.closeShellsOf(t.id, path, p.family...)
	if !gone(path) {
		force := ""
		if p.force {
			force = "--force"
		}
		_, err := l.projectCommand(ctx, t.proj, t.proj.RemoveWorktree(), "{branch}", util.ShellQuote(p.branch), "{worktree}", util.ShellQuote(path), "{force}", force)
		left := l.worktreeLeft(ctx, dir, path)
		switch {
		case left && err != nil:
			return nil, "the clean-up command failed: " + util.FirstLine(err.Error())
		case left:
			return nil, fmt.Sprintf("the clean-up command left %s", path)
		case err != nil:
			fmt.Fprintf(os.Stderr, "agentos: the clean-up command failed, but %s is gone: %v\n", path, err)
		}
	}
	if err := l.sessions.made.Removed(t.proj.Key(), path, time.Now()); err != nil {
		return []string{"worktree " + path}, fmt.Sprintf("recording the worktree removed failed: %v", err)
	}
	return []string{"worktree " + path}, ""
}

// woke is why a clean-up stops once the thread it settled runs again, as after a wake, or "" while it does not.
func (l *Lifecycle) woke(id string) string {
	l.sessions.mu.Lock()
	live := slices.ContainsFunc(l.sessions.info, func(in term.Info) bool { return in.Name.String() == id })
	l.sessions.mu.Unlock()
	if !live {
		return ""
	}
	return fmt.Sprintf("Thread %d woke during the clean-up", l.sessions.Number(id))
}

// CleaningUp says whether a clean-up of the thread runs now, so a wake can wait until it ends.
func (l *Lifecycle) CleaningUp(id string) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tr := l.tracks[id]
	return tr != nil && tr.cleanup == "pending"
}

// cleanupDue says whether the thread's PR merged or closed and its clean-up has not finished: it waits to run or for
// the owner's word, runs or was blocked. It waits for Run's first poll, as the tracks are empty before it, or for ctx
// to end. It takes mu.
func (l *Lifecycle) cleanupDue(ctx context.Context, id string) bool {
	if l == nil {
		return false
	}
	select {
	case <-l.polled:
	case <-ctx.Done():
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tr := l.tracks[id]
	return tr != nil && tr.pr != nil && (tr.pr.State == "merged" || tr.pr.State == "closed") && !l.retired(id)
}

// gone says whether nothing is at path.
func gone(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, fs.ErrNotExist)
}

// worktreeLeft says whether the worktree at path is still on disk or in git's list of the repo's worktrees. A list git
// cannot give counts as left.
func (l *Lifecycle) worktreeLeft(ctx context.Context, dir, path string) bool {
	if !gone(path) {
		return true
	}
	trees, err := l.worktrees(ctx, dir)
	return err != nil || slices.ContainsFunc(trees.paths, func(p string) bool { return folder.Canonical(p) == folder.Canonical(path) })
}

// settleSession settles the session's thread, closes its browser window and removes its temp folder. The thread
// keeps its row, state file, chat log, images and evidence.
func (l *Lifecycle) settleSession(ctx context.Context, t target, _ *cleanupPlan) (removed []string, reason string) {
	if !t.ended {
		if err := l.sessions.Settle(t.id); err != nil {
			return removed, fmt.Sprintf("putting the thread to sleep failed: %v", err)
		}
	}
	removed = append(removed, "thread put to sleep")
	if l.sessions.browsers.Has(t.id) {
		l.sessions.browsers.Close(ctx, t.id)
		removed = append(removed, "browser window")
	}
	name, err := session.ParseName(t.id)
	if err != nil {
		return removed, err.Error()
	}
	tmp := l.sessions.tmpDir(name)
	if _, err := os.Stat(tmp); err == nil {
		if err := os.RemoveAll(tmp); err != nil {
			return removed, fmt.Sprintf("removing the temp folder failed: %v", err)
		}
		removed = append(removed, "temp folder")
	}
	return removed, ""
}

// unsafe says why removing the worktree, or the branch when no worktree has it checked out, could
// lose work, or "".
func (l *Lifecycle) unsafe(ctx context.Context, dir, worktree, branch string, pr *PR, base string) string {
	if worktree != "" {
		if reason := l.uncommitted(ctx, dir, worktree, branch); reason != "" {
			return reason
		}
	}
	head, err := l.head(ctx, dir, worktree, branch)
	if err != nil {
		return fmt.Sprintf("git could not read where %s stands: %v", branch, err)
	}
	if l.merged(ctx, dir, head, base, pr) {
		return ""
	}
	if pr != nil && pr.State == "merged" && l.past(ctx, dir, pr.HeadOid, head) {
		return branch + " has commits that are not in the merged pull request"
	}
	n, err := l.notOnOrigin(ctx, dir, head)
	switch {
	case err != nil:
		return fmt.Sprintf("git could not compare %s with origin: %v", branch, err)
	case n == 1:
		return branch + " has a commit that is not on origin"
	case n > 1:
		return fmt.Sprintf("%s has %d commits that are not on origin", branch, n)
	}
	return ""
}

// uncommitted says why the worktree of branch holds changes no commit has, or "".
func (l *Lifecycle) uncommitted(ctx context.Context, dir, worktree, branch string) string {
	out, err := l.worktreeStatus(ctx, dir, worktree)
	switch {
	case err != nil:
		return fmt.Sprintf("git could not read the worktree of %s: %v", branch, err)
	case strings.TrimSpace(string(out)) != "":
		return fmt.Sprintf("the worktree of %s has uncommitted changes", branch)
	}
	return ""
}

// merged says whether pr merged the work at head. A squash or rebase merge copies the commits onto the
// base with new ids, so the work is in only while head is the PR's last commit or one before it; a
// merge commit keeps them, so head is in the base as origin was last fetched.
func (l *Lifecycle) merged(ctx context.Context, dir, head, base string, pr *PR) bool {
	if pr == nil || pr.State != "merged" {
		return false
	}
	if head == pr.HeadOid {
		return true
	}
	if pr.HeadOid != "" && l.ancestor(ctx, dir, head, pr.HeadOid) {
		return true
	}
	return l.ancestor(ctx, dir, head, "origin/"+base)
}

// fetchOrigin brings every branch on origin up to date and drops the ones origin deleted, so a
// commit that only a deleted or force-pushed branch held no longer counts as pushed.
func (l *Lifecycle) fetchOrigin(ctx context.Context, dir string) error {
	_, err := l.git(ctx, dir, "fetch", "--quiet", "--prune", "origin")
	return err
}

// ancestor says whether commit is rev or one of its ancestors.
func (l *Lifecycle) ancestor(ctx context.Context, dir, commit, rev string) bool {
	_, err := l.git(ctx, dir, "merge-base", "--is-ancestor", commit, rev)
	return err == nil
}

// past says whether head holds commits after prHead, the commit a merged pull request ended on.
func (l *Lifecycle) past(ctx context.Context, dir, prHead, head string) bool {
	return prHead != "" && prHead != head && l.ancestor(ctx, dir, prHead, head)
}

// head is the commit the worktree stands at, or the branch's when no worktree has it checked out.
func (l *Lifecycle) head(ctx context.Context, dir, worktree, branch string) (string, error) {
	args := []string{"rev-parse", "--verify", "refs/heads/" + branch + "^{commit}"}
	if worktree != "" {
		args = []string{"-C", worktree, "rev-parse", "--verify", "HEAD^{commit}"}
	}
	out, err := l.git(ctx, dir, args...)
	return strings.TrimSpace(string(out)), err
}

// notOnOrigin counts the commits at head that no ref on origin holds.
func (l *Lifecycle) notOnOrigin(ctx context.Context, dir, head string) (int, error) {
	out, err := l.git(ctx, dir, "rev-list", "--count", head, "--not", "--remotes=origin")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// worktreeStatus lists the worktree's changes and new files. It takes no index lock, so the agent's own
// git commands in the worktree never find one held by this read.
func (l *Lifecycle) worktreeStatus(ctx context.Context, dir, worktree string) ([]byte, error) {
	return l.git(ctx, dir, "--no-optional-locks", "-C", worktree, "status", "--porcelain", "--untracked-files=normal")
}

// commitsNotIn counts the commits at rev that ref lacks.
func (l *Lifecycle) commitsNotIn(ctx context.Context, dir, ref, rev string) (int, error) {
	out, err := l.git(ctx, dir, "rev-list", "--count", ref+".."+rev)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// statuses reads the git status of each fetched branch once.
func (l *Lifecycle) statuses(ctx context.Context, dir string, trees worktrees, fetched []fetch) map[string]*BranchStatus {
	out := map[string]*BranchStatus{}
	if len(fetched) == 0 {
		return out
	}
	base := l.mainBranch(ctx, dir)
	// merged reads origin's branches as last fetched here, so restatus, run at each tool call, never fetches.
	if slices.ContainsFunc(fetched, func(f fetch) bool { return f.pr != nil && f.pr.State == "merged" }) {
		if err := l.fetchOrigin(ctx, dir); err != nil {
			fmt.Fprintf(os.Stderr, "agentos: fetching origin for the git status: %v\n", err)
		}
	}
	for _, f := range fetched {
		if _, ok := out[f.branch]; !ok {
			out[f.branch] = l.branchStatus(ctx, dir, base, f.branch, trees.byBranch[f.branch], f.pr)
		}
	}
	return out
}

// branchStatus reads where branch stands against origin, or nil when git cannot compare it with the base.
func (l *Lifecycle) branchStatus(ctx context.Context, dir, base, branch, worktree string, pr *PR) *BranchStatus {
	out, err := l.git(ctx, dir, "rev-list", "--left-right", "--count", "origin/"+base+"..."+branch)
	if err != nil {
		return nil
	}
	st := &BranchStatus{Base: base}
	if _, err := fmt.Sscan(string(out), &st.Behind, &st.Ahead); err != nil {
		return nil
	}
	merged := pr != nil && pr.State == "merged"
	if st.Unpushed, err = l.commitsNotIn(ctx, dir, "origin/"+branch, branch); err != nil {
		st.Unpushed = 0
		if !merged { // a merge copies the commits onto the base, so NotInMain says what is lost
			st.Unpushed = st.Ahead
		}
	}
	if merged {
		st.NotInMain = l.notInMerged(ctx, dir, base, worktree, branch, pr)
	}
	if worktree != "" {
		if out, err := l.worktreeStatus(ctx, dir, worktree); err == nil {
			st.Uncommitted = strings.Count(string(out), "\n")
		}
	}
	return st
}

// notInMerged counts the commits the merged pull request did not take, by the rules unsafe keeps a worktree by.
func (l *Lifecycle) notInMerged(ctx context.Context, dir, base, worktree, branch string, pr *PR) int {
	head, err := l.head(ctx, dir, worktree, branch)
	if err != nil || l.merged(ctx, dir, head, base, pr) {
		return 0
	}
	var n int
	if l.past(ctx, dir, pr.HeadOid, head) {
		n, _ = l.commitsNotIn(ctx, dir, pr.HeadOid, head)
	} else {
		n, _ = l.notOnOrigin(ctx, dir, head)
	}
	return n
}

// setStatus stores the git status read for the session's branch, unless the session moved on since.
func (l *Lifecycle) setStatus(id, branch string, st *BranchStatus) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if tr := l.tracks[id]; tr != nil && tr.branch == branch {
		tr.git = st
	}
}

// restatus reads the git status of the session's branch again, so the row follows the agent's commits
// and edits between fetches. Reads for one session do not overlap: one asked for during a read runs after it.
func (l *Lifecycle) restatus(ctx context.Context, id string, proj project.Project) {
	l.mu.Lock()
	tr := l.tracks[id]
	if tr == nil || tr.branch == "" {
		l.mu.Unlock()
		return
	}
	if tr.reading {
		tr.again = true
		l.mu.Unlock()
		return
	}
	tr.reading = true
	for {
		tr.again = false
		branch, worktree := tr.branch, tr.worktree
		var pr *PR
		if tr.pr != nil {
			c := *tr.pr
			pr = &c
		}
		l.mu.Unlock()
		st := l.branchStatus(ctx, proj.Dir, l.mainBranch(ctx, proj.Dir), branch, worktree, pr)
		l.mu.Lock()
		if tr.branch == branch {
			tr.git = st
		}
		if !tr.again || l.gone[id] {
			break
		}
	}
	tr.reading = false
	l.mu.Unlock()
	l.sessions.changed()
}

// projectCommand runs one of the project's commands through sh -c in the project folder, with {dir} filled in and
// each placeholder of fill, given as pairs of a placeholder and the text that takes its place. It returns what the
// command printed.
func (l *Lifecycle) projectCommand(ctx context.Context, proj project.Project, command string, fill ...string) ([]byte, error) {
	cmd := strings.NewReplacer(append([]string{"{dir}", util.ShellQuote(proj.Dir)}, fill...)...).Replace(command)
	ctx, cancel := context.WithTimeout(ctx, 2*gitTimeout)
	defer cancel()
	return l.run(ctx, proj.Dir, "sh", "-c", cmd)
}

func (l *Lifecycle) cleanupPath(key string) string {
	return filepath.Join(l.dataDir, key, "cleanups.json")
}

// readCleanups is the project's log, oldest first.
func (l *Lifecycle) readCleanups(key string) []Cleanup {
	var log []Cleanup
	if data, err := os.ReadFile(l.cleanupPath(key)); err == nil {
		_ = json.Unmarshal(data, &log)
	}
	return log
}

func (l *Lifecycle) appendCleanup(key string, c Cleanup) error {
	l.logMu.Lock()
	defer l.logMu.Unlock()
	data, err := json.Marshal(append(l.readCleanups(key), c))
	if err != nil {
		return fmt.Errorf("encoding clean-up log: %w", err)
	}
	return atomicfile.Write(l.cleanupPath(key), data, 0o600)
}

// Cleanups is the project's clean-up log, newest first.
func (l *Lifecycle) Cleanups(key string) []Cleanup {
	l.logMu.Lock()
	out := l.readCleanups(key)
	l.logMu.Unlock()
	if out == nil {
		out = []Cleanup{}
	}
	for i := range out {
		if out[i].Removed == nil {
			out[i].Removed = []string{}
		}
	}
	slices.Reverse(out)
	return out[:min(len(out), cleanupsKept)]
}

// track follows the thread on branch, afresh when the branch is new to it, and looks up the branch's PR. A call for a
// branch the thread has left since, as a turn's end that comes after a track, does nothing.
func (l *Lifecycle) track(ctx context.Context, id string, proj project.Project, branch string) {
	if t, err := l.sessions.target(id); err != nil || t.branch != branch {
		return
	}
	l.mu.Lock()
	if l.gone[id] {
		l.mu.Unlock()
		return
	}
	if tr := l.tracks[id]; tr != nil && tr.branch != branch && tr.cleanup != "pending" {
		*tr = track{}
	}
	l.mu.Unlock()
	l.ensureWatchers(ctx)
	l.refreshOnly(ctx, proj, l.targetsOf(proj), []string{branch})
}

// gitDirs are the git folder of the checkout at dir, `<repo>/.git/worktrees/<name>` for a linked
// worktree, and the repo's common one, `<repo>/.git`.
func (l *Lifecycle) gitDirs(ctx context.Context, dir string) (gitDir, common string, err error) {
	out, err := l.git(ctx, dir, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	if err != nil {
		return "", "", fmt.Errorf("finding the git folders of %s: %w", dir, err)
	}
	gitDir, common, _ = strings.Cut(strings.TrimSpace(string(out)), "\n")
	return gitDir, common, nil
}

// mainBranch is the branch origin calls its default, or main when origin does not say.
func (l *Lifecycle) mainBranch(ctx context.Context, dir string) string {
	out, err := l.git(ctx, dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return "main"
	}
	return cmp.Or(strings.TrimPrefix(strings.TrimSpace(string(out)), "origin/"), "main")
}
