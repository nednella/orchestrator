package sessions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/util"
)

const (
	webhookEvents = "pull_request,pull_request_review,issue_comment,check_suite"
	webhookSettle = 10 * time.Second // a stream that lasts this long worked; one that ends sooner never did
	webhookRetry  = 30 * time.Second
	repoRetry     = 30 * time.Second
	payloadCap    = 4 << 20
)

// watcher follows one project's pull requests between reconciles: a webhook stream where
// gh webhook forward works, else a conditional poll of the REST list.
type watcher struct {
	cancel context.CancelFunc
	config string // the pr_watch and pr_poll it was started with
}

// ensureWatchers keeps a watcher running for every project with a session on a branch, and stops the rest.
func (l *Lifecycle) ensureWatchers(ctx context.Context) {
	want := map[string]project.Project{}
	for _, g := range groups(l.workTargets()) {
		want[g.proj.Key()] = g.proj
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, w := range l.watchers {
		if proj, ok := want[key]; !ok || w.config != watchConfig(proj) {
			w.cancel()
			delete(l.watchers, key)
		}
	}
	for key, proj := range want {
		if _, ok := l.watchers[key]; ok || ctx.Err() != nil {
			continue
		}
		wctx, cancel := context.WithCancel(ctx)
		l.watchers[key] = &watcher{cancel: cancel, config: watchConfig(proj)}
		go l.watch(wctx, proj)
	}
}

func watchConfig(proj project.Project) string { return proj.PRWatch + " " + proj.PRPoll }

// watch runs until ctx ends. Unless the project says poll, the webhook stream is tried first;
// a stream that never got going leaves the poll.
func (l *Lifecycle) watch(ctx context.Context, proj project.Project) {
	for ctx.Err() == nil {
		repo := l.repos.Repo(ctx, proj.Dir)
		if repo == "" {
			if !sleep(ctx, repoRetry) {
				return
			}
			continue
		}
		if proj.PRWatch != "poll" && l.forward(ctx, proj, repo) {
			continue
		}
		l.pollPRs(ctx, proj, repo)
	}
}

// sleep waits for d, and says whether ctx is still live.
func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// forward streams the repo's events through gh webhook forward and refreshes the pull requests they name.
// It returns true when the stream worked and dropped, so it is worth another try after a pause.
func (l *Lifecycle) forward(ctx context.Context, proj project.Project, repo string) bool {
	started := time.Now()
	out, err := l.stream(ctx, proj.Dir, "gh", "webhook", "forward", "--repo", repo, "--events", webhookEvents)
	if err == nil {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 64<<10), payloadCap)
		for sc.Scan() {
			l.onEvent(ctx, proj, sc.Bytes())
		}
		err = out.Close()
	}
	if ctx.Err() != nil {
		return false
	}
	ran := time.Since(started) >= webhookSettle
	switch {
	case proj.PRWatch == "webhook":
		l.report("pull requests", fmt.Sprintf("gh webhook forward for %s stopped: %s", repo, util.FirstLine(endedWith(err))))
	case !ran:
		fmt.Fprintf(os.Stderr, "orchestrator: gh webhook forward does not work for %s, polling its pull requests instead: %v\n", repo, err)
		return false
	}
	return sleep(ctx, webhookRetry)
}

func endedWith(err error) string {
	if err == nil {
		return "it ended"
	}
	return err.Error()
}

// onEvent refreshes the pull requests a webhook payload names. A number no session tracks yet,
// as when a PR is opened, refreshes them all.
func (l *Lifecycle) onEvent(ctx context.Context, proj project.Project, payload []byte) {
	var ev struct {
		PullRequest *struct{ Number int } `json:"pull_request"`
		Issue       *struct {
			Number      int             `json:"number"`
			PullRequest json.RawMessage `json:"pull_request"`
		} `json:"issue"`
		CheckSuite *struct {
			PullRequests []struct{ Number int } `json:"pull_requests"`
		} `json:"check_suite"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		return
	}
	var numbers []int
	switch {
	case ev.PullRequest != nil:
		numbers = append(numbers, ev.PullRequest.Number)
	case ev.Issue != nil && ev.Issue.PullRequest != nil:
		numbers = append(numbers, ev.Issue.Number)
	case ev.CheckSuite != nil:
		for _, pr := range ev.CheckSuite.PullRequests {
			numbers = append(numbers, pr.Number)
		}
	default:
		return
	}
	targets := l.targetsOf(proj)
	var only []string
	for _, n := range numbers {
		branch, ok := l.branchOfPR(targets, n)
		if !ok {
			only = nil
			break
		}
		only = append(only, branch)
	}
	l.refreshOnly(ctx, proj, targets, only)
}

// branchOfPR is the branch of the session that tracks the pull request.
func (l *Lifecycle) branchOfPR(targets []target, number int) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, t := range targets {
		if tr := l.tracks[t.id]; tr != nil && tr.pr != nil && tr.pr.Number == number {
			return tr.branch, true
		}
	}
	return "", false
}

// targetsOf are the project's sessions on a branch.
func (l *Lifecycle) targetsOf(proj project.Project) []target {
	var out []target
	for _, t := range l.workTargets() {
		if t.proj.Key() == proj.Key() {
			out = append(out, t)
		}
	}
	return out
}

// refreshOnly refreshes the branches named, or all of them when only is nil, and warns when gh fails.
func (l *Lifecycle) refreshOnly(ctx context.Context, proj project.Project, targets []target, only []string) {
	if only != nil && len(only) == 0 {
		return
	}
	_, err := l.refresh(ctx, proj, targets, only)
	if err != nil && ctx.Err() == nil {
		l.report("pull requests", util.FirstLine(err.Error()))
	}
}

// pollPRs asks GitHub for the repo's pull request list at the project's interval, with the ETag of
// the last answer so an unchanged list costs nothing, and refreshes the branches whose PR moved.
func (l *Lifecycle) pollPRs(ctx context.Context, proj project.Project, repo string) {
	every, err := proj.PRPollEvery()
	if err != nil {
		every = project.DefaultPRPoll
	}
	tick := time.NewTicker(every)
	defer tick.Stop()
	key := proj.Key()
	l.mu.Lock()
	l.polling[key] = true
	delete(l.etags, key) // an on-demand read may have kept one: the poll must see the list whole
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		delete(l.polling, key)
		l.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		targets := l.targetsOf(proj)
		changed, err := l.changedBranches(ctx, proj, repo, targets)
		if err != nil {
			if ctx.Err() == nil {
				l.report("pull requests", util.FirstLine(err.Error()))
			}
			continue
		}
		// Checks do not touch a PR's updated_at, so pending ones are looked at every time.
		l.refreshOnly(ctx, proj, targets, append(changed, l.pendingBranches(targets)...))
	}
}

// changedBranches are the tracked branches whose pull request was updated since the last fetch,
// as the repo's PR list says. A list GitHub reports unchanged (304) names none.
func (l *Lifecycle) changedBranches(ctx context.Context, proj project.Project, repo string, targets []target) ([]string, error) {
	list, unchanged, err := l.listPulls(ctx, proj, repo, true)
	if err != nil {
		return nil, err
	}
	changed := []string{}
	if unchanged {
		return changed, nil
	}
	updated := map[string]int64{}
	for _, pr := range slices.Backward(list) { // the newest PR of a branch wins
		updated[pr.Head.Ref] = util.Millis(pr.UpdatedAt)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, t := range targets {
		at, listed := updated[t.branch]
		if !listed {
			continue
		}
		tr := l.tracks[t.id]
		known := tr != nil && tr.pr != nil && tr.branch == t.branch
		if (!known || tr.pr.UpdatedAt != at) && !slices.Contains(changed, t.branch) {
			changed = append(changed, t.branch)
		}
	}
	return changed, nil
}

// listedPull is one entry of the repo's REST pull request list.
type listedPull struct {
	Number    int                    `json:"number"`
	Title     string                 `json:"title"`
	Draft     bool                   `json:"draft"`
	State     string                 `json:"state"`
	HTMLURL   string                 `json:"html_url"`
	User      struct{ Login string } `json:"user"`
	Head      struct{ Ref string }   `json:"head"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// listPulls reads the repo's pull request list with the ETag of the last answer and keeps the open pull requests
// of a 200 for PullRequests; a 304 keeps the last list and says unchanged. The ETag is kept only when keepETag:
// a caller that does not act on what moved must not hide it from the poll.
func (l *Lifecycle) listPulls(ctx context.Context, proj project.Project, repo string, keepETag bool) (list []listedPull, unchanged bool, err error) {
	key := proj.Key()
	l.mu.Lock()
	etag := l.etags[key]
	l.mu.Unlock()
	args := []string{"api", "-i", fmt.Sprintf("repos/%s/pulls?state=all&sort=updated&direction=desc&per_page=100", repo)}
	if etag != "" {
		args = append(args, "-H", "If-None-Match: "+etag)
	}
	ctx, cancel := context.WithTimeout(ctx, run.GHTimeout)
	defer cancel()
	out, err := l.run(ctx, proj.Dir, "gh", args...)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 304") {
			l.mu.Lock()
			defer l.mu.Unlock()
			if kept, ok := l.lists[key]; ok {
				kept.at, kept.failedAt = l.now(), time.Time{}
				l.lists[key] = kept
			}
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("listing the pull requests of %s: %w", repo, err)
	}
	etag, body := splitResponse(out)
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, false, fmt.Errorf("reading the pull request list of %s: %w", repo, err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if keepETag {
		l.etags[key] = etag
	}
	l.lists[key] = openPulls{at: l.now(), pulls: openOf(list)}
	return list, false, nil
}

// splitResponse takes the ETag and the body out of what gh api -i printed: the headers, a blank line, the body.
func splitResponse(out []byte) (etag string, body []byte) {
	text := string(out)
	i := strings.Index(text, "\n\n")
	if j := strings.Index(text, "\r\n\r\n"); j >= 0 && (i < 0 || j < i) {
		i = j
	}
	if i < 0 {
		return "", out
	}
	head, rest := text[:i], strings.TrimLeft(text[i:], "\r\n")
	for line := range strings.SplitSeq(head, "\n") {
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(name, "etag") {
			etag = strings.TrimSpace(value)
		}
	}
	return etag, []byte(rest)
}

// pendingBranches are the tracked branches whose PR has checks still running.
func (l *Lifecycle) pendingBranches(targets []target) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, t := range targets {
		if tr := l.tracks[t.id]; tr != nil && tr.pr != nil && tr.pr.Checks == "pending" {
			out = append(out, tr.branch)
		}
	}
	return out
}
