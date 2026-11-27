// Package apptest runs the whole app on a private tmux socket, state dir and config, for the services' tests.
// A test must never use the default socket: it holds live threads.
package apptest

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/browser"
	"github.com/nednella/orchestrator/desktop/chat"
	"github.com/nednella/orchestrator/desktop/digest"
	"github.com/nednella/orchestrator/desktop/evidence"
	"github.com/nednella/orchestrator/desktop/internal/app"
	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/internal/scoped"
	"github.com/nednella/orchestrator/desktop/internal/warn"
	"github.com/nednella/orchestrator/desktop/issues"
	"github.com/nednella/orchestrator/desktop/limits"
	"github.com/nednella/orchestrator/desktop/notes"
	"github.com/nednella/orchestrator/desktop/projects"
	"github.com/nednella/orchestrator/desktop/settings"
	"github.com/nednella/orchestrator/desktop/stats"
	"github.com/nednella/orchestrator/desktop/terminal"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/activity"
	"github.com/nednella/orchestrator/internal/bus"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/environ"
	"github.com/nednella/orchestrator/internal/gittest"
	"github.com/nednella/orchestrator/internal/thread"
	"github.com/nednella/orchestrator/internal/tmuxtest"
)

type Recorder struct {
	mu     sync.Mutex
	events []recorded
	clips  []string
	pick   string // what the folder picker returns
	quits  int
	opened []string // the bundles a relaunch asked to open
}

type recorded struct {
	name    string
	payload any
}

func (r *Recorder) Emit(name string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recorded{name, payload})
}

func (r *Recorder) Picker() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pick, nil
}

// Quit is the host's Quit: it counts.
func (r *Recorder) Quit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.quits++
}

// Quits is how many times the app asked to quit.
func (r *Recorder) Quits() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.quits
}

// Relaunch stands in for opening the bundle again: it only records the path.
func (r *Recorder) Relaunch(bundle string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opened = append(r.opened, bundle)
	return nil
}

// Relaunched is the bundles a relaunch asked to open.
func (r *Recorder) Relaunched() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.opened)
}

// Awake takes no assertion: a test must not keep the machine awake.
func (r *Recorder) Awake() (func(), error) { return func() {}, nil }

func (r *Recorder) Clip(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clips = append(r.clips, text)
}

// SetPick sets what the folder picker returns.
func (r *Recorder) SetPick(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pick = dir
}

// HasClip says whether the clipboard was set to that text.
func (r *Recorder) HasClip(text string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.clips, text)
}

func (r *Recorder) Count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.name == name {
			n++
		}
	}
	return n
}

// countAttention counts the attention events of one state.
func (r *Recorder) CountAttention(state string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if m, ok := e.payload.(map[string]string); ok && e.name == "attention" && m["state"] == state {
			n++
		}
	}
	return n
}

// last is the payload of the latest event of that name, or nil.
// CountAttentionFor counts the attention events of one state for one thread.
func (r *Recorder) CountAttentionFor(id, state string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if m, ok := e.payload.(map[string]string); ok && e.name == "attention" && m["state"] == state && m["id"] == id {
			n++
		}
	}
	return n
}

// All are the payloads of the events of that name, oldest first.
func (r *Recorder) All(name string) []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []any
	for _, e := range r.events {
		if e.name == name {
			out = append(out, e.payload)
		}
	}
	return out
}

// LastEvent is the name and payload of the latest event.
func (r *Recorder) LastEvent() (string, any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		return "", nil
	}
	e := r.events[len(r.events)-1]
	return e.name, e.payload
}

// Last is the payload of the latest event of that name, or nil.
func (r *Recorder) Last(name string) any {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range slices.Backward(r.events) {
		if e.name == name {
			return e.payload
		}
	}
	return nil
}

// lastList is the payload of the latest event of that name, which carries one project's list.
func lastList[T any](r *Recorder, name string) scoped.List[T] {
	list, _ := r.Last(name).(scoped.List[T])
	return list
}

// LastThreads is the list of the latest threads event.
func (r *Recorder) LastThreads() []threads.Thread {
	return lastList[threads.Thread](r, "threads").Items
}

// LastThreadsOf is the list of the latest threads event of one project, nil when it has had none.
func (r *Recorder) LastThreadsOf(project string) []threads.Thread {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range slices.Backward(r.events) {
		if list, ok := e.payload.(scoped.List[threads.Thread]); ok && e.name == "threads" && list.Project == project {
			return list.Items
		}
	}
	return nil
}

// LastNotes is the list of the latest notes event.
func (r *Recorder) LastNotes() []notes.Note { return lastList[notes.Note](r, "notes").Items }

// LastIssues is the list of the latest issues event.
func (r *Recorder) LastIssues() []issues.Issue { return lastList[issues.Issue](r, "issues").Items }

// LastCleanups is the list of the latest cleanups event.
func (r *Recorder) LastCleanups() []threads.Cleanup {
	return lastList[threads.Cleanup](r, "cleanups").Items
}

// Warnings are the warnings events so far, in order.
func (r *Recorder) Warnings() []warn.Warning {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []warn.Warning
	for _, e := range r.events {
		if w, ok := e.payload.(warn.Warning); ok && e.name == "warnings" {
			out = append(out, w)
		}
	}
	return out
}

// Settled are the ids of the threads that any threads event showed as settled.
func (r *Recorder) Settled() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		if list, ok := e.payload.(scoped.List[threads.Thread]); ok && e.name == "threads" {
			for _, s := range list.Items {
				if s.State == "settled" && !slices.Contains(out, s.ID) {
					out = append(out, s.ID)
				}
			}
		}
	}
	return out
}

// ProjectOf is the project key of the latest event of that name (threads, notes, issues or cleanups), or "".
func (r *Recorder) ProjectOf(name string) string {
	switch name {
	case "threads":
		return lastList[threads.Thread](r, name).Project
	case "notes":
		return lastList[notes.Note](r, name).Project
	case "issues":
		return lastList[issues.Issue](r, name).Project
	case "cleanups":
		return lastList[threads.Cleanup](r, name).Project
	}
	return ""
}

// Eventually waits until cond holds. The time it gives is long, since tests run at the same time on a busy machine; a
// test that fails does not take it, only one that waits for what does not come.
func Eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// FakeGH answers the gh calls with canned output; any other command runs for real.

type FakeGH struct {
	mu            sync.Mutex
	calls         []string
	Repo          error
	UserErr       error             // what gh api user fails with, if set: gh is not signed in
	CreateRepoErr error             // what gh repo create fails with, if set
	Create        string            // what gh issue create prints
	CreateErr     error             // what gh issue create fails with, if set; it still prints Create
	Delay         time.Duration     // how long gh issue create takes
	Releases      string            // the latest release's tag; "" fails the check
	Notes         map[string]string // the body of each release, by version without the v; a version missing has no release
	NotesErr      error             // what reading a release's notes fails with, if set: GitHub is not reachable
	pr            string            // what gh pr list prints
	prByHead      map[string]string // what gh pr list --head <branch> prints instead, by branch
	PRErr         error             // what gh pr list fails with, if set
	IssuesErr     error             // what gh issue list fails with, if set
	issues        string            // what gh issue list prints instead of the usual six issues, if set
	pulls         string            // the body gh api prints for the repo's pull request list
	etag          string            // its ETag: a request carrying it gets a 304
	hooks         []*io.PipeWriter
	hooked        bool                                  // gh webhook forward works
	before        func(dir, name string, args []string) // runs before each command, in the command's goroutine
}

// SetBefore has before run ahead of each command the app runs, gh or not, so a test can hold one at a point it picks.
func (f *FakeGH) SetBefore(before func(dir, name string, args []string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.before = before
}

func (f *FakeGH) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	before := f.before
	f.mu.Unlock()
	if before != nil {
		before(dir, name, args)
	}
	if name == "curl" {
		return f.curl(args)
	}
	if name == "open" {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.calls = append(f.calls, "open "+strings.Join(args, " "))
		return nil, nil
	}
	if name != "gh" {
		return run.Exec(ctx, dir, name, args...)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(args, " "))
	switch {
	case args[0] == "api" && args[1] == "-i":
		if f.pulls == "" {
			return nil, fmt.Errorf("gh api: exit status 1: gh: HTTP 404: Not Found")
		}
		if i := slices.Index(args, "-H"); i >= 0 && args[i+1] == "If-None-Match: "+f.etag {
			return nil, fmt.Errorf("gh api: exit status 1: gh: HTTP 304")
		}
		return []byte("HTTP/2.0 200 OK\r\nEtag: " + f.etag + "\r\nX-Ratelimit-Remaining: 4999\r\n\r\n" + f.pulls), nil
	case args[0] == "pr":
		if f.PRErr != nil {
			return nil, f.PRErr
		}
		if i := slices.Index(args, "--head"); i >= 0 {
			if out, ok := f.prByHead[args[i+1]]; ok {
				return []byte(out), nil
			}
		}
		if f.pr == "" {
			return []byte("[]"), nil
		}
		return []byte(f.pr), nil
	case args[0] == "repo" && args[1] == "create":
		return nil, f.CreateRepoErr
	case args[0] == "repo":
		if f.Repo != nil {
			return nil, f.Repo
		}
		return []byte(`{"nameWithOwner":"acme/widgets"}`), nil
	case args[0] == "api" && args[1] == "user":
		if f.UserErr != nil {
			return nil, f.UserErr
		}
		return []byte(GHUser + "\n"), nil
	case args[0] == "api" && strings.HasSuffix(args[1], "/comments?per_page=100"):
		if !strings.Contains(args[1], "/issues/7/") {
			return []byte("[]"), nil
		}
		return []byte(`[{"user":{"login":"amy"},"created_at":"2026-09-03T10:00:00Z","body_html":"<p>Agreed.</p>"}]`), nil
	case args[0] == "api":
		if strings.HasSuffix(args[1], "/issues/7") {
			return []byte(`{"number":7,"body_html":"<h2>Description</h2>\n<p>Fix it.</p>","comments":1}`), nil
		}
		if strings.HasSuffix(args[1], "/issues/11") {
			return []byte(`{"number":11,"body_html":"","comments":0}`), nil
		}
		return nil, fmt.Errorf("gh api: HTTP 404: Not Found")
	case args[0] == "issue" && args[1] == "create":
		f.mu.Unlock()
		time.Sleep(f.Delay)
		f.mu.Lock()
		return []byte(f.Create), f.CreateErr
	case args[0] == "issue":
		if f.IssuesErr != nil {
			return nil, f.IssuesErr
		}
		if f.issues != "" {
			return []byte(f.issues), nil
		}
		return []byte(`[
			{"number":7,"title":"Fix the thing that breaks when the window is narrow","url":"https://x/7","author":{"login":"sam"},"assignees":[{"login":"sam"},{"login":"amy"}],"createdAt":"2026-09-01T10:00:00Z","updatedAt":"2026-09-02T10:00:00Z","labels":[{"name":"ready"},{"name":"type:bug"}]},
			{"number":8,"title":"Plan it","url":"https://x/8","labels":[{"name":"needs-plan"},{"name":"type:feature"}]},
			{"number":9,"title":"Human","url":"https://x/9","labels":[{"name":"needs-human"}]},
			{"number":10,"title":"Maybe","url":"https://x/10","labels":[{"name":"idea"},{"name":"type:chore"}]},
			{"number":11,"title":"New","url":"https://x/11","labels":[]},
			{"number":12,"title":"Other","url":"https://x/12","labels":[{"name":"roadmap"},{"name":"model:haiku"},{"name":"effort:high"}]}
		]`), nil
	}
	return nil, fmt.Errorf("unexpected %s %v", name, args)
}

// curl answers the release check as GitHub would for the release Releases, and refuses anything else.
func (f *FakeGH) curl(args []string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "curl "+strings.Join(args, " "))
	url := args[len(args)-1]
	if _, tag, ok := strings.Cut(url, "/releases/tags/v"); ok {
		return f.notes(tag)
	}
	switch {
	case f.Releases == "":
	case strings.HasSuffix(url, "/releases/latest"):
		return []byte("https://github.com/nednella/orchestrator/releases/tag/" + f.Releases), nil
	case strings.HasSuffix(url, "/releases/download/"+f.Releases+"/orchestrator-darwin-arm64.zip.sha256"):
		return nil, nil
	}
	return nil, fmt.Errorf("curl: no network in tests: %v", args)
}

// notes answers the API's release for version as curl prints it with -w "\n%{http_code}".
func (f *FakeGH) notes(version string) ([]byte, error) {
	if f.NotesErr != nil {
		return nil, f.NotesErr
	}
	body, ok := f.Notes[version]
	if !ok {
		return []byte("{\"message\":\"Not Found\"}\n404"), nil
	}
	out, err := json.Marshal(map[string]string{"body": body})
	return append(out, "\n200"...), err
}

func (f *FakeGH) IssueCalls() int { return f.Calls("issue list") }

// Calls counts the gh calls that start with prefix, and the open calls, as "open <args>".
func (f *FakeGH) Calls(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// SetIssues sets what gh issue list prints.
func (f *FakeGH) SetIssues(list string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issues = list
}

// SetPulls sets what the repo's pull request list answers: its ETag and the body.
func (f *FakeGH) SetPulls(etag, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.etag, f.pulls = etag, body
}

// Stream is the run.Streamer: gh webhook forward streams the events the test sends, when the test
// said it works; any other command fails.
func (f *FakeGH) Stream(_ context.Context, _, name string, args ...string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(args, " "))
	if name != "gh" || args[0] != "webhook" || !f.hooked {
		return nil, fmt.Errorf("gh %s: exit status 1: unknown command %q for \"gh\"", args[0], args[0])
	}
	r, w := io.Pipe()
	f.hooks = append(f.hooks, w)
	return r, nil
}

// WebhookWorks makes gh webhook forward run, as it does where the user is admin.
func (f *FakeGH) WebhookWorks() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hooked = true
}

// Event sends a webhook payload down every forward stream.
func (f *FakeGH) Event(payload string) {
	f.mu.Lock()
	hooks := slices.Clone(f.hooks)
	f.mu.Unlock()
	for _, w := range hooks {
		_, _ = io.WriteString(w, payload+"\n")
	}
}

// DropWebhooks ends every forward stream, as a lost connection would.
func (f *FakeGH) DropWebhooks() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.hooks {
		_ = w.Close()
	}
	f.hooks = nil
}

// Forwards counts the gh webhook forward runs.
func (f *FakeGH) Forwards() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, "webhook forward") {
			n++
		}
	}
	return n
}

// SetPR sets what gh pr list prints.
func (f *FakeGH) SetPR(out string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pr = out
}

// SetPRFor sets what gh pr list prints for the PR of one branch.
func (f *FakeGH) SetPRFor(branch, out string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.prByHead == nil {
		f.prByHead = map[string]string{}
	}
	f.prByHead[branch] = out
}

func (f *FakeGH) CallLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// NoGH is a runner for tests that must not reach GitHub.
func NoGH(context.Context, string, string, ...string) ([]byte, error) {
	return nil, errors.New("gh is not available in this test")
}

// NoStream is a streamer for tests that must not start anything.
func NoStream(context.Context, string, string, ...string) (io.ReadCloser, error) {
	return nil, errors.New("streams are not available in this test")
}

// NoClaude is an env runner for tests that must not run claude.
func NoClaude(context.Context, string, []string, string, ...string) ([]byte, error) {
	return nil, errors.New("claude is not available in this test")
}

// queueSections is the queue of the project "main": the issues of FakeGH fall in Ready (7), Plan (8), Inbox (11) and Other (9, 10, 12).
const queueSections = `    queue_sections:
      - name: Ready
        labels: [ready]
        actions:
          - {name: Work, command: "/ship {n}"}
          - {name: Plan, command: "/plan {n}", model: opus, effort: high, access: read}
      - name: Plan
        labels: [needs-plan]
        actions:
          - {name: Investigate, command: "/investigate {n}", model: opus}
      - name: Inbox
        actions:
          - {name: Investigate, command: "/investigate {n}"}
`

// Options changes the harness's config.
type Options struct {
	ProjectExtra    string        // yaml lines added under the project "main"
	Queue           string        // the project's queue_sections block, in place of the usual one
	NoBranch        bool          // leave out the project's `issue_branch`, so the default `issue-{n}` applies
	CleanupCommand  string        // the project's cleanup_command; none by default
	WorktreeCommand string        // the project's worktree_command; the default by default
	TopExtra        string        // yaml lines added at the top of the config
	DigestFirst     time.Duration // wait before the first automatic digest check; an hour by default
	DigestTick      time.Duration
	UpdateTick      time.Duration     // how often to check the releases; an hour by default, and an hour before the first check
	Releases        string            // the latest release's tag; "" fails the check
	Notes           map[string]string // the body of each release's notes, by version; see FakeGH.Notes
	NotesErr        error             // what reading the notes fails with, if set
	Version         string            // the version the app believes it runs; "dev" by default
	DataDirConfig   bool              // put the data folder in the config's data_dir, not ORCHESTRATOR_DEV_DATA_DIR, and home in the state dir
	NoProject       bool              // leave the project out of the config, so the app starts in a folder it does not know
	AbsentProject   string            // with NoProject: the folder is named so, and the config holds a project of that name whose folder is missing here
	ClaudeRejects   string            // a flag the stand-in claude does not know: started with it, it fails at once as Claude does
	CleanupSweep    time.Duration     // how often kept clean-ups are tried again; 0 keeps the app's hour
	PollEvery       time.Duration     // how often tmux is listed; 0 keeps the app's 2 seconds
	StopSettle      time.Duration     // how long after a turn's end its usage is counted; 0 keeps the app's second
	RetryFirst      time.Duration     // the first pause before a child's result the parent could not take goes again; 0 keeps the app's second
	ConfigTick      time.Duration     // how often the config file is read again; 0 keeps the app's 2 seconds
	Env             []string          // entries of the environment the app and its threads run in; the harness's own isolation wins over them
	PathWithout     []string          // commands whose folders the environment's PATH leaves out; the stand-in claude is added to it
}

// Harness is an app whose Claude threads run the stand-in claude under the built command's host.
type Harness struct {
	App    *app.App
	Rec    *Recorder
	GH     *FakeGH
	Claude *FakeClaude
	Socket string
	Dir    string
	State  string
	Conf   string
	Env    []string // the environment the app and its threads run in: the process's, with the harness's own entries

	opts      Options // what the harness was made with
	claudeDir string
	cancel    context.CancelFunc // ends the context the app started with

	projects *projects.Service
	threads  *threads.Service
	chat     *chat.Service
	terminal *terminal.Service
	issues   *issues.Service
	notes    *notes.Service
	stats    *stats.Service
	evidence *evidence.Service
	browser  *browser.Service
	digest   *digest.Service
	settings *settings.Service
	limits   *limits.Service
}

// New starts an app for the test; the cleanup stops it and its tmux server.
func New(t *testing.T) *Harness { return NewWith(t, Options{}) }

func NewWith(t *testing.T, o Options) *Harness {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	takeSlot(t)
	state, err := os.MkdirTemp("", "orc")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if o.AbsentProject != "" {
		dir = filepath.Join(dir, o.AbsentProject)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	confPath := filepath.Join(state, "config.yaml")
	branch := "    issue_branch: \"issue-{n}\"\n"
	if o.NoBranch {
		branch = ""
	}
	cleanup := ""
	if o.CleanupCommand != "" {
		cleanup = fmt.Sprintf("    cleanup_command: %q\n", o.CleanupCommand)
	}
	if o.WorktreeCommand != "" {
		cleanup += fmt.Sprintf("    worktree_command: %q\n", o.WorktreeCommand)
	}
	top := o.TopExtra
	if o.DataDirConfig {
		top += "data_dir: " + filepath.Join(state, "data") + "\n"
	}
	conf := fmt.Sprintf("%sprojects:\n  - name: main\n    directory: %s\n%s%s%s%s", top, dir, branch, cleanup, cmp.Or(o.Queue, queueSections), o.ProjectExtra)
	if o.NoProject {
		conf = fmt.Sprintf("%sprojects: []\n", top)
		if o.AbsentProject != "" {
			conf = fmt.Sprintf("%sprojects:\n  - name: %s\n    directory: /nonexistent/%[2]s\n", top, o.AbsentProject)
		}
	}
	if err := os.WriteFile(confPath, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	socket := fmt.Sprintf("orctest-%d-%d", os.Getpid(), harnesses.Add(1))
	h := &Harness{Rec: &Recorder{}, GH: &FakeGH{Releases: o.Releases, Notes: o.Notes, NotesErr: o.NotesErr}, Claude: &FakeClaude{}, Socket: socket, Dir: dir, State: state, Conf: confPath}
	start := standInEnv
	if o.ClaudeRejects != "" {
		start += rejecting(o.ClaudeRejects)
	}
	h.claudeDir = agentOnPath(t, "claude", start, fakeClaudeTurns)
	env := environ.With(gittest.Env(), o.Env...)
	path := environ.Get(env, "PATH")
	if len(o.PathWithout) > 0 {
		path = environ.PathWithout(env, o.PathWithout...)
	}
	set := []string{
		"ORCHESTRATOR_TMUX_SOCKET=" + socket,
		"ORCHESTRATOR_STATE_DIR=" + state,
		"ORCHESTRATOR_CONFIG=" + confPath,
		"ORCHESTRATOR_DIR=" + dir,
		"ORCHESTRATOR_BROWSER_HEADLESS=1",                     // tests never open windows on the owner's screen
		"CLAUDE_CONFIG_DIR=" + filepath.Join(state, "claude"), // never the user's own settings
		"PATH=" + h.claudeDir + string(os.PathListSeparator) + path,
	}
	if o.DataDirConfig {
		set = append(set, "ORCHESTRATOR_DEV_DATA_DIR=", "HOME="+filepath.Join(state, "home")) // the local folder sits under home
	} else {
		set = append(set, "ORCHESTRATOR_DEV_DATA_DIR="+filepath.Join(state, "data"))
	}
	h.Env = environ.With(env, set...)
	h.App = h.build(t)
	first, tick := o.DigestFirst, o.DigestTick
	if first == 0 {
		first, tick = time.Hour, time.Hour
	}
	h.App.Digests().SetLoop(first, tick) // a test must never start the real claude
	if o.UpdateTick == 0 {
		h.App.Updates().SetLoop(time.Hour, time.Hour)
	} else {
		h.App.Updates().SetLoop(0, o.UpdateTick)
	}
	if o.Version != "" {
		h.App.Updates().SetVersion(o.Version)
	}
	h.App.Updates().SetRelaunch(h.Rec.Relaunch)
	if o.CleanupSweep != 0 {
		h.App.Threads().SetCleanupSweep(o.CleanupSweep)
	}
	h.opts = o
	h.shortenWaits()
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	if err := h.App.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		h.App.Stop()
		tmuxtest.Stop(socket)
		os.RemoveAll(state)
	})
	return h
}

// shortenWaits applies the waits the options shorten to the app.
func (h *Harness) shortenWaits() {
	if h.opts.PollEvery != 0 {
		h.App.Threads().SetPollEvery(h.opts.PollEvery)
	}
	if h.opts.StopSettle != 0 {
		h.App.Threads().SetStopSettle(h.opts.StopSettle)
	}
	if h.opts.RetryFirst != 0 {
		h.App.Tools().SetRetryFirst(h.opts.RetryFirst)
	}
	if h.opts.ConfigTick != 0 {
		h.App.SetConfigTick(h.opts.ConfigTick)
	}
}

// build wires an app from the environment the harness set up.
func (h *Harness) build(t *testing.T) *app.App {
	t.Helper()
	cfg, err := app.LoadEnv(h.Env)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Binary = CLIPath(t) // the test binary runs the app but has no host command
	a := app.New(cfg, app.Host{Emit: h.Rec.Emit, Clipboard: h.Rec.Clip, PickDir: h.Rec.Picker, Quit: h.Rec.Quit, Awake: h.Rec.Awake}, h.GH.Run, h.GH.Stream, h.Claude.RunEnv)
	for _, svc := range a.Services() {
		switch s := svc.(type) {
		case *projects.Service:
			h.projects = s
		case *threads.Service:
			h.threads = s
		case *chat.Service:
			h.chat = s
		case *terminal.Service:
			h.terminal = s
		case *issues.Service:
			h.issues = s
		case *notes.Service:
			h.notes = s
		case *stats.Service:
			h.stats = s
		case *evidence.Service:
			h.evidence = s
		case *browser.Service:
			h.browser = s
		case *digest.Service:
			h.digest = s
		case *settings.Service:
			h.settings = s
		case *limits.Service:
			h.limits = s
		}
	}
	return a
}

// Restart starts a second app on the same dirs, as if the first was quit and opened again. env are entries of its
// environment that replace the first's, except the harness's isolation: its state, config, folder and data entries, and
// its tmux socket unless env names another private one, never an empty or a default one.
func (h *Harness) Restart(t *testing.T, env ...string) *Harness {
	t.Helper()
	isolation := []string{"ORCHESTRATOR_STATE_DIR", "ORCHESTRATOR_CONFIG", "ORCHESTRATOR_DIR", "ORCHESTRATOR_DEV_DATA_DIR"}
	if sock := environ.Get(env, "ORCHESTRATOR_TMUX_SOCKET"); sock == "" || sock == "orchestrator" || sock == "agentos" {
		isolation = append(isolation, "ORCHESTRATOR_TMUX_SOCKET")
	}
	next := environ.With(h.Env, env...)
	for _, k := range isolation {
		next = environ.With(next, k+"="+environ.Get(h.Env, k))
	}
	second := &Harness{Rec: &Recorder{}, GH: h.GH, Claude: h.Claude, Socket: h.Socket, Dir: h.Dir, State: h.State, Conf: h.Conf, Env: next, claudeDir: h.claudeDir, opts: h.opts}
	second.App = second.build(t)
	second.shortenWaits()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(second.App.Stop)
	if err := second.App.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return second
}

// BeginQuit ends the context the app started with, as the app's quit does before it stops answering commands.
func (h *Harness) BeginQuit() { h.cancel() }

// Threads is the thread list behind the bound service.
func (h *Harness) Threads() *threads.Threads { return h.App.Threads() }

// Notes is the notes store behind the bound service.
func (h *Harness) Notes() *notes.Notes { return h.App.Notes() }

// Observe gives the thread a change of state in the names and the payload fields of Claude's hooks, as its follower
// gives the threads one it read off the host's stream. A live thread has had its SessionStart, so one is given first
// when the thread has no state file. A test of how the stream makes these changes drives its host instead.
func (h *Harness) Observe(t *testing.T, id, event, payload string) {
	t.Helper()
	live := exec.Command("tmux", "-L", h.Socket, "has-session", "-t", "="+id).Run() == nil
	if live {
		h.followsHost(t, id)
	}
	if rec, _ := bus.ReadState(h.State, id); live && event != "SessionStart" && rec.Thread == "" {
		h.Threads().Observe(id, thread.Event{Name: "SessionStart"})
	}
	var p struct {
		NotificationType     string         `json:"notification_type"`
		Message              string         `json:"message"`
		ToolName             string         `json:"tool_name"`
		ToolInput            map[string]any `json:"tool_input"`
		Prompt               string         `json:"prompt"`
		LastAssistantMessage string         `json:"last_assistant_message"`
		Cwd                  string         `json:"cwd"`
		SessionID            string         `json:"session_id"`
		TranscriptPath       string         `json:"transcript_path"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		t.Fatalf("the payload of %s is not JSON: %v", event, err)
	}
	if p.Cwd != "" {
		// The thread's Claude says so from its next turn on, as after a cd, not the folder it started in.
		if err := os.WriteFile(h.standIn(id, ".cwd"), []byte(p.Cwd), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ev := thread.Event{Name: event, NotificationType: p.NotificationType, Message: p.Message, Cwd: p.Cwd, Conversation: p.SessionID, Transcript: p.TranscriptPath}
	switch event {
	case "Notification":
		ev.Detail = p.Message
	case "PreToolUse", "PostToolUse":
		ev.Tool, ev.Detail = p.ToolName, p.ToolName
		ev.Command, _ = p.ToolInput["command"].(string)
		for _, key := range []string{"command", "file_path", "pattern", "url", "description"} {
			if v, _ := p.ToolInput[key].(string); v != "" {
				ev.Detail = p.ToolName + " " + v
				break
			}
		}
	case "UserPromptSubmit":
		ev.Detail = p.Prompt
	case "Stop":
		ev.Detail = p.LastAssistantMessage
		ev.Asks = strings.HasSuffix(strings.TrimRight(p.LastAssistantMessage, " \t\n*_"), "?")
	}
	h.Threads().Observe(id, ev)
}

// followsHost waits until the app has taken in the start of the thread's host, which its follower does once the host
// listens. A change of state the test makes before is overtaken by that start when the host is slow to come up, as on a
// busy machine: the start is then newer than the change. A thread that runs no host is not waited for.
func (h *Harness) followsHost(t *testing.T, id string) {
	t.Helper()
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{pane_start_command}").Output()
	if err != nil || !strings.Contains(string(out), " host ") {
		return
	}
	Eventually(t, "the app to take in the start of the host of "+id, func() bool {
		rec, _ := bus.ReadState(h.State, id)
		return rec.Observed > 0
	})
}

// Thread is the thread with that id in the current project's list.
func (h *Harness) Thread(id string) (threads.Thread, bool) {
	for _, s := range h.Threads().List() {
		if s.ID == id {
			return s, true
		}
	}
	return threads.Thread{}, false
}

// harnesses numbers the harnesses of this process, so each has a tmux socket of its own.
var harnesses atomic.Int64

// startEnv is the environment the tests started in, without what a parent git set. The command is built in it, not in
// one a test changed: under a throwaway HOME, go would fetch every module again.
var startEnv = gittest.Env()

var (
	cliOnce sync.Once
	cliBin  string
	cliDir  string
	cliErr  error
)

// CLIPath builds the orchestrator command from this checkout, so the host and the tools' server run the code under test.
// It is the app's binary with a stub page in place of the front end.
func CLIPath(t *testing.T) string {
	t.Helper()
	cliOnce.Do(func() {
		if cliDir, cliErr = os.MkdirTemp("", "orccli"); cliErr != nil {
			return
		}
		cliBin = filepath.Join(cliDir, "orchestrator")
		_, file, _, _ := runtime.Caller(0)
		root := filepath.Join(filepath.Dir(file), "..", "..", "..")
		cmd := exec.Command("go", "build", "-tags", "stub", "-o", cliBin, "./desktop")
		cmd.Dir = root
		cmd.Env = startEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			cliErr = fmt.Errorf("building the orchestrator command: %w: %s", err, out)
		}
	})
	if cliErr != nil {
		t.Fatal(cliErr)
	}
	return cliBin
}

// ClaudeDir is the folder of the stand-in claude, which NewWith puts first on PATH beside the built orchestrator command,
// so the app runs its Claude provider and each thread's host starts the stand-in. The stand-in writes the
// environment it was started with to <dir>/<thread id, "/" as "_">.env and its arguments
// to <dir>/<thread id, "/" as "_">.args, then appends each line it reads to <dir>/<thread id, "/" as "_">.stdin
// until its stdin ends. It answers each user message with an init line, a reply and a result, as Claude does each
// turn, an interrupt with a result that has an error, a mode change with Claude's reply and its report of the new
// mode, and any other control request with a success. Each init line names the folder the stand-in runs in, as Claude
// does, or the one a payload of Observe names. The message "ask", or a task that ends in "needs approval", asks to run a Bash command instead and replies
// once answered, "tool" runs one and replies, "tool wait" holds its tool call until Proceed and its reply until Finish, "question" replies with a
// question, one that ends in "slow", such as a task delegated as "slow", replies a second later, one that ends in
// "fails" replies "half done" and ends its turn with the error "API Error: 500", one that ends in "background shell"
// starts a shell in the background and replies a second later, and a second after that tells of the shell's end in a
// turn of its own, which replies "shell done", "stream" streams 200 deltas, "0 " to
// "199 ", before its reply, "plan"
// asks to exit plan mode and, once allowed, reports the mode default before its reply, as Claude does, "leave plan"
// reports the mode default before its reply with no plan approved, "work", or a text that ends in "keep working", such
// as a task delegated so, keeps its turn open until an interrupt: it
// holds each message read meanwhile, and after the interrupt takes them in and replies to them in a turn of their own,
// as Claude does, and "serve <port>" starts a listener on 127.0.0.1:<port> in a session of its own, which writes its
// pid to <dir>/<thread id, "/" as "_">.serve<port>, and "exit" ends it, as Claude ending by itself. It answers a rewind
// of the files with the files RewindAs names, into each of which a rewind that is not a dry run writes "before", and a
// rewind of the conversation with the text of the message rewound to, as Claude does; RewindAs can make it answer as
// another Claude would.
func (h *Harness) ClaudeDir() string { return h.claudeDir }

// RewindAs makes the thread's stand-in claude answer its rewinds so: "" as Claude Code does, "stale" holding a later
// message than any named, "nocheckpoint" as a Claude started without file checkpoints and "unsupported" as one that
// knows neither request. A rewind of the files changes files.
func (h *Harness) RewindAs(t *testing.T, id, mode string, files ...string) {
	t.Helper()
	list := ""
	for _, f := range files {
		list += f + "\n"
	}
	if err := os.WriteFile(h.standIn(id, ".rewind"), []byte(mode), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.standIn(id, ".rewind-files"), []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}
}

// standIn is the path of the stand-in claude's file of the thread with that extension.
// RefuseFlags makes the thread's stand-in claude refuse a change of its settings with reason, as Claude does for
// ultracode on a model that has none.
func (h *Harness) RefuseFlags(t *testing.T, id, reason string) {
	t.Helper()
	if err := os.WriteFile(h.standIn(id, ".refuse-flags"), []byte(reason), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (h *Harness) standIn(id, ext string) string {
	return filepath.Join(h.claudeDir, strings.ReplaceAll(id, "/", "_")+ext)
}

// Proceed lets the thread's stand-in claude go on from a "tool wait" message: it holds its tool call until then, and
// its turn after the tool's result until Finish.
func (h *Harness) Proceed(t *testing.T, id string) {
	t.Helper()
	if err := os.WriteFile(h.standIn(id, ".proceed"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Finish lets the stand-in claude end the turn of a "tool wait" message.
func (h *Harness) Finish(t *testing.T, id string) {
	t.Helper()
	if err := os.WriteFile(h.standIn(id, ".finish"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Stdin are the lines the thread's stand-in claude has read so far, none before it starts.
func (h *Harness) Stdin(t *testing.T, id string) []string {
	t.Helper()
	return readLines(t, h.standIn(id, ".stdin"))
}

// Refused is how many times the thread's stand-in claude failed at its start on the flag it does not know.
func (h *Harness) Refused(t *testing.T, id string) int {
	t.Helper()
	return len(readLines(t, h.standIn(id, ".refused")))
}

// Args are the arguments the thread's stand-in claude was started with, none before it starts.
func (h *Harness) Args(t *testing.T, id string) []string {
	t.Helper()
	return readLines(t, h.standIn(id, ".args"))
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// standInEnv writes the environment the stand-in claude was started with, before its arguments.
const standInEnv = `env > "$f.env"
`

// rejecting makes the stand-in claude fail as Claude does when it is started with flag and does not know it. Each
// start that fails adds a line to <dir>/<thread id, "/" as "_">.refused.
func rejecting(flag string) string {
	return `for a in "$@"; do
	if [ "$a" = '` + flag + `' ]; then
		echo refused >> "$f.refused"
		echo "error: unknown option '` + flag + `'" >&2
		exit 1
	fi
done
`
}

// fakeClaudeTurns is the stand-in claude's stream-json side. serve is a listener on the port given that leaves the
// stand-in's process and thread, as a server a Bash call starts with & does, writes its pid to the file given and
// lives five minutes. "background" starts a subagent in the background, which ends at the next "finish", or is gone
// with no end when "list none" lists no task; "subagent asks" starts one that asks for an approval, which waits past
// the turn's end, and ends once it is answered; "subagent lost" starts one that asks and is gone with no end a moment
// after the turn's end. "limit" streams a rate_limit event before its reply, and a get_usage request is answered with a
// thread and a weekly window.
const fakeClaudeTurns = `here=$(pwd -P)
serve='import os, socket, sys, time
if os.fork():
    sys.exit(0)
os.setsid()
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", int(sys.argv[1])))
s.listen(5)
open(sys.argv[2] + ".tmp", "w").write(str(os.getpid()))
os.rename(sys.argv[2] + ".tmp", sys.argv[2])
time.sleep(300)'
reply() {
	printf '{"type":"assistant","message":{"id":"msg_fake","content":[{"type":"text","text":"%s"}]}}\n' "${1:-done}"
	printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.01}'
}
while IFS= read -r line; do
	printf '%s\n' "$line" >> "$f.stdin"
	cwd=$(cat "$f.cwd" 2>/dev/null || echo "$here")
	init='{"type":"system","subtype":"init","session_id":"fake-session","cwd":"'"$cwd"'","original_cwd":"'"$here"'"}'
	case "$line" in
	*'"type":"user"'*'"text":"tool"'*|*'"type":"user"'*'"text":"tool wait"'*)
		printf '%s\n' "$init" '{"type":"assistant","message":{"id":"msg_tool","content":[{"type":"tool_use","id":"toolu_tool","name":"Bash","input":{"command":"git status"}}]}}'
		case "$line" in
		*'"text":"tool wait"'*) until [ -e "$f.proceed" ]; do sleep 0.05; done ;;
		esac
		printf '%s\n' '{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_tool","content":"clean"}]}}'
		case "$line" in
		*'"text":"tool wait"'*) until [ -e "$f.finish" ]; do sleep 0.05; done ;;
		esac
		reply ;;
	*'"type":"user"'*'"text":"question"'*)
		printf '%s\n' "$init"
		reply 'Which branch should I use?' ;;
	*'"type":"user"'*'"text":"ask"'*|*'"type":"user"'*'needs approval"}]}'*)
		printf '%s\n' "$init" '{"type":"control_request","request_id":"ask-1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"touch made.txt"}}}' ;;
	*'"type":"user"'*'slow"}]}'*)
		printf '%s\n' "$init"
		sleep 1
		reply ;;
	*'"type":"user"'*'background shell"}]}'*)
		printf '%s\n' "$init" '{"type":"assistant","message":{"id":"msg_sh","content":[{"type":"tool_use","id":"toolu_sh","name":"Bash","input":{"command":"sleep 2","run_in_background":true}}]}}' \
			'{"type":"system","subtype":"task_started","task_id":"b1","tool_use_id":"toolu_sh","description":"sleep 2","is_backgrounded":true,"task_type":"local_bash"}' \
			'{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_sh","content":"Running in the background"}]}}'
		sleep 1
		reply
		(
			sleep 1
			printf '%s\n' '{"type":"system","subtype":"task_notification","task_id":"b1","tool_use_id":"toolu_sh","status":"completed","summary":"slept"}' \
				'{"type":"assistant","message":{"id":"msg_shell","content":[{"type":"text","text":"shell done"}]}}' '{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.01}'
		) & ;;
	*'"type":"user"'*'fails"}]}'*)
		printf '%s\n' "$init" '{"type":"assistant","message":{"id":"msg_fake","content":[{"type":"text","text":"half done"}]}}' \
			'{"type":"result","subtype":"error_during_execution","is_error":true,"errors":["API Error: 500"],"total_cost_usd":0.01}' ;;
	*'"type":"user"'*'"text":"stream"'*)
		printf '%s\n' "$init"
		i=0
		while [ $i -lt 200 ]; do
			printf '{"type":"stream_event","api_message_id":"msg_fake","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"%d "}}}\n' $i
			i=$((i + 1))
		done
		reply ;;
	*'"type":"user"'*'"text":"serve '[0-9]*)
		port=$(printf '%s' "$line" | sed -n 's/.*"text":"serve \([0-9]*\)".*/\1/p')
		python3 -c "$serve" "$port" "$f.serve$port" </dev/null >/dev/null 2>&1
		printf '%s\n' "$init"
		reply ;;
	*'"type":"user"'*'"text":"exit"'*)
		exit 0 ;;
	*'"type":"user"'*'"text":"background"'*)
		printf '%s\n' "$init" '{"type":"assistant","message":{"id":"msg_call","content":[{"type":"tool_use","id":"toolu_bg","name":"Agent","input":{"description":"List files"}}]}}' \
			'{"type":"system","subtype":"task_started","task_id":"a1","tool_use_id":"toolu_bg","description":"List files","is_backgrounded":true,"task_type":"local_agent"}' \
			'{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_bg","content":"Async agent launched"}]}}' \
			'{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.01}' ;;
	*'"type":"user"'*'"text":"subagent asks"'*)
		printf '%s\n' "$init" '{"type":"assistant","message":{"id":"msg_call","content":[{"type":"tool_use","id":"toolu_asker","name":"Agent","input":{"description":"Write sub.txt"}}]}}' \
			'{"type":"system","subtype":"task_started","task_id":"a2","tool_use_id":"toolu_asker","description":"Write sub.txt","is_backgrounded":true,"task_type":"local_agent"}' \
			'{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_asker","content":"Async agent launched"}]}}' \
			'{"type":"assistant","parent_tool_use_id":"toolu_asker","message":{"id":"msg_sub","content":[{"type":"tool_use","id":"toolu_write","name":"Write","input":{"file_path":"sub.txt"}}]}}' \
			'{"type":"control_request","request_id":"sub-1","request":{"subtype":"can_use_tool","tool_name":"Write","input":{"file_path":"sub.txt"},"tool_use_id":"toolu_write","agent_id":"a2"}}' \
			'{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.01}' ;;
	*'"type":"user"'*'"text":"subagent lost"'*)
		printf '%s\n' "$init" '{"type":"assistant","message":{"id":"msg_call","content":[{"type":"tool_use","id":"toolu_lost","name":"Agent","input":{"description":"Lost"}}]}}' \
			'{"type":"system","subtype":"task_started","task_id":"a3","tool_use_id":"toolu_lost","description":"Lost","is_backgrounded":true,"task_type":"local_agent"}' \
			'{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_lost","content":"Async agent launched"}]}}' \
			'{"type":"control_request","request_id":"lost-1","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"ls"},"tool_use_id":"toolu_ls","agent_id":"a3"}}' \
			'{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.01}'
		sleep 1
		printf '%s\n' '{"type":"system","subtype":"background_tasks_changed","tasks":[]}' ;;
	*'"type":"control_response"'*'"request_id":"sub-1"'*)
		printf '%s\n' '{"type":"user","parent_tool_use_id":"toolu_asker","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_write","content":"written"}]}}' \
			'{"type":"system","subtype":"task_notification","task_id":"a2","tool_use_id":"toolu_asker","status":"completed","summary":"written"}'
		reply ;;
	*'"type":"user"'*'"text":"limit"'*)
		printf '%s\n' "$init" '{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","resetsAt":1795705200,"rateLimitType":"five_hour","utilization":0.5}}'
		reply ;;
	*'"type":"user"'*'"text":"list none"'*)
		printf '%s\n' "$init" '{"type":"system","subtype":"background_tasks_changed","tasks":[]}'
		reply ;;
	*'"type":"user"'*'"text":"finish"'*)
		printf '%s\n' "$init" '{"type":"system","subtype":"task_notification","task_id":"a1","tool_use_id":"toolu_bg","status":"completed","summary":"a.go"}'
		reply ;;
	*'"type":"user"'*'"text":"plan"'*)
		printf '%s\n' "$init" '{"type":"control_request","request_id":"plan-1","request":{"subtype":"can_use_tool","tool_name":"ExitPlanMode","input":{"plan":"Add made.txt"}}}' ;;
	*'"type":"user"'*'"text":"leave plan"'*)
		printf '%s\n' "$init" '{"type":"system","subtype":"status","status":null,"permissionMode":"default"}'
		reply ;;
	*'"type":"user"'*'"text":"work"'*|*'"type":"user"'*'keep working"}]}'*)
		printf '%s\n' "$init"
		working=1 held= ;;
	*'"type":"user"'*)
		uuid=$(printf '%s' "$line" | sed -n 's/.*"uuid":"\([^"]*\)".*/\1/p')
		if [ -n "$working" ]; then
			held="$held $uuid"
			continue
		fi
		printf '{"type":"user","uuid":"%s","isReplay":true,"message":{"role":"user","content":[]}}\n' "$uuid"
		printf '%s\n' "$init"
		reply ;;
	*'"type":"control_response"'*'"request_id":"plan-1"'*'"behavior":"allow"'*)
		printf '%s\n' '{"type":"system","subtype":"status","status":null,"permissionMode":"default"}'
		reply ;;
	*'"type":"control_response"'*)
		reply ;;
	*'"subtype":"rewind_files"'*|*'"subtype":"rewind_conversation"'*)
		id=$(printf '%s' "$line" | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
		answer() { printf '{"type":"control_response","response":{"subtype":"success","request_id":"%s","response":%s}}\n' "$id" "$1"; }
		fail() { printf '{"type":"control_response","response":{"subtype":"error","request_id":"%s","error":"%s"}}\n' "$id" "$1"; }
		case "$(cat "$f.rewind" 2>/dev/null):$line" in
		unsupported:*) fail "Unsupported control request subtype: rewind" ;;
		nocheckpoint:*'"dry_run":true'*) answer '{"canRewind":false,"error":"File rewinding is not enabled."}' ;;
		nocheckpoint:*'"subtype":"rewind_files"'*) fail "File rewinding is not enabled." ;;
		*'"subtype":"rewind_files"'*'"dry_run":true'*)
			files=$(sed 's/.*/"&"/' "$f.rewind-files" 2>/dev/null | paste -sd , -)
			answer "{\"canRewind\":true,\"filesChanged\":[$files],\"insertions\":2,\"deletions\":1}" ;;
		*'"subtype":"rewind_files"'*)
			cat "$f.rewind-files" 2>/dev/null | while IFS= read -r file; do printf before > "$file"; done
			answer '{"canRewind":true,"skippedLinks":0}' ;;
		stale:*) answer '{"rewound":false,"error":"stale target","reason":"stale_target"}' ;;
		*)
			target=$(printf '%s' "$line" | sed -n 's/.*"target_message_uuid":"\([^"]*\)".*/\1/p')
			text=$(grep -F "\"uuid\":\"$target\"" "$f.stdin" | head -n 1 | sed -n 's/.*"text":"\([^"]*\)".*/\1/p')
			answer "{\"rewound\":true,\"targetMessageUuid\":\"$target\",\"prefillText\":\"$text\",\"precedingAssistantUuid\":\"fake-reply\"}" ;;
		esac ;;
	*'"subtype":"get_usage"'*)
		id=$(printf '%s' "$line" | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
		printf '{"type":"control_response","response":{"subtype":"success","request_id":"%s","response":{"rate_limits":{"five_hour":{"utilization":25,"resets_at":"2026-11-26T15:00:00+00:00"},"seven_day":{"utilization":96,"resets_at":"2026-11-30T23:00:00+00:00"}}}}}\n' "$id" ;;
	*'"subtype":"set_permission_mode"'*)
		id=$(printf '%s' "$line" | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
		mode=$(printf '%s' "$line" | sed -n 's/.*"mode":"\([^"]*\)".*/\1/p')
		printf '{"type":"control_response","response":{"subtype":"success","request_id":"%s","response":{"mode":"%s"}}}\n' "$id" "$mode"
		printf '{"type":"system","subtype":"status","status":null,"permissionMode":"%s"}\n' "$mode" ;;
	*'"subtype":"apply_flag_settings"'*)
		id=$(printf '%s' "$line" | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
		if [ -f "$f.refuse-flags" ]; then
			printf '{"type":"control_response","response":{"subtype":"error","request_id":"%s","error":"apply_flag_settings: %s"}}\n' "$id" "$(cat "$f.refuse-flags")"
		else
			printf '{"type":"control_response","response":{"subtype":"success","request_id":"%s"}}\n' "$id"
		fi ;;
	*'"subtype":"interrupt"'*)
		printf '%s\n' '{"type":"control_cancel_request","request_id":"ask-1"}' '{"type":"control_cancel_request","request_id":"plan-1"}' \
			'{"type":"result","subtype":"error_during_execution","is_error":true,"total_cost_usd":0.01}'
		working=
		if [ -n "$held" ]; then
			for u in $held; do
				printf '{"type":"user","uuid":"%s","isReplay":true,"message":{"role":"user","content":[]}}\n' "$u"
			done
			held=
			reply
		fi ;;
	*'"type":"control_request"'*)
		id=$(printf '%s' "$line" | sed -n 's/.*"request_id":"\([^"]*\)".*/\1/p')
		printf '{"type":"control_response","response":{"subtype":"success","request_id":"%s"}}\n' "$id" ;;
	esac
done
`

// agentOnPath writes the stand-in named so, with first run before it records its arguments and then run after it,
// in a folder of its own, which it returns. Both may use f, the path of the args file without its extension.
func agentOnPath(t *testing.T, name, first, then string) string {
	t.Helper()
	dir := t.TempDir()
	// The rename makes the args file appear whole, so a test never reads it half written.
	script := "#!/bin/sh\nf=\"" + dir + "/$(echo \"$ORCHESTRATOR_THREAD\" | tr / _)\"\n" + first +
		"printf '%s\\n' \"$@\" > \"$f.args.tmp\" && mv \"$f.args.tmp\" \"$f.args\"\n" + then
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ClaudeConfigDir is the folder the app's Claude threads keep their settings and transcripts in.
func (h *Harness) ClaudeConfigDir() string { return environ.Get(h.Env, "CLAUDE_CONFIG_DIR") }

// RemoveClaude takes the stand-in claude off the harness's PATH, as if claude were not installed.
func (h *Harness) RemoveClaude(t *testing.T) {
	t.Helper()
	if err := os.Rename(filepath.Join(h.claudeDir, "claude"), filepath.Join(h.claudeDir, "claude.removed")); err != nil {
		t.Fatal(err)
	}
}

// RestoreClaude puts the stand-in claude back after RemoveClaude.
func (h *Harness) RestoreClaude(t *testing.T) {
	t.Helper()
	if err := os.Rename(filepath.Join(h.claudeDir, "claude.removed"), filepath.Join(h.claudeDir, "claude")); err != nil {
		t.Fatal(err)
	}
}

// Main runs a package's tests and removes the command they built.
func Main(m *testing.M) int {
	code := m.Run()
	if cliDir != "" {
		os.RemoveAll(cliDir)
	}
	return code
}

// LockedBuffer is a log the app's goroutines write to while the test reads it.
type LockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *LockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *LockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Exists says whether a path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// The methods below call the bound services the way the front end does.

func (h *Harness) Snapshot() projects.Snapshot { return h.projects.Snapshot() }
func (h *Harness) SwitchProject(name string) (projects.Snapshot, error) {
	return h.projects.SwitchProject(name)
}
func (h *Harness) PickFolder() (string, error) { return h.projects.PickFolder() }
func (h *Harness) AddProjectDir(dir string) (projects.Snapshot, error) {
	return h.projects.AddProjectDir(dir, false)
}
func (h *Harness) AddProjectDirCreating(dir string) (projects.Snapshot, error) {
	return h.projects.AddProjectDir(dir, true)
}
func (h *Harness) NewProjectOptions() projects.NewProjectOptions {
	return h.projects.NewProjectOptions()
}
func (h *Harness) GitHubOwner() string { return h.projects.GitHubOwner() }
func (h *Harness) ListFolders(path string) (projects.FolderListing, error) {
	return h.projects.ListFolders(path)
}
func (h *Harness) CreateProject(name string) (projects.NewProject, error) {
	return h.projects.CreateProject(name)
}
func (h *Harness) PublishProject(key string) (string, error) { return h.projects.PublishProject(key) }
func (h *Harness) SetUpProject() (threads.Thread, error)     { return h.projects.SetUpProject() }

func (h *Harness) NewThread(title, prompt, model, effort, access string) (threads.Thread, error) {
	return h.threads.NewThread(title, prompt, model, effort, false, access, "")
}

// NewThreadFast starts a Supervised thread with the model and effort given and fast mode as given.
func (h *Harness) NewThreadFast(title, model, effort string, fast bool) (threads.Thread, error) {
	return h.threads.NewThread(title, "", model, effort, fast, "supervised", "")
}

func (h *Harness) SetFast(id string, on bool) error  { return h.threads.SetFast(id, on) }
func (h *Harness) SetEffort(id, effort string) error { return h.threads.SetEffort(id, effort) }

// NewThreadIn starts a thread with the access given in the place given: worktree, project, or "" for the default.
func (h *Harness) NewThreadIn(title, prompt, access, place string) (threads.Thread, error) {
	return h.threads.NewThread(title, prompt, "", "", false, access, place)
}

func (h *Harness) DefaultModel(model string) threads.ModelChoice {
	return h.threads.DefaultModel(model)
}

// StartThread starts a Supervised thread.
func (h *Harness) StartThread(title, prompt string) (threads.Thread, error) {
	return h.threads.NewThread(title, prompt, "", "", false, "supervised", "")
}

// StartThreadWith starts a Supervised thread with the model and effort given.
func (h *Harness) StartThreadWith(title, prompt, model, effort string) (threads.Thread, error) {
	return h.threads.NewThread(title, prompt, model, effort, false, "supervised", "")
}
func (h *Harness) ShellNew(id string) (threads.ShellInfo, error) { return h.threads.ShellNew(id) }
func (h *Harness) ShellClose(id string) error                    { return h.threads.ShellClose(id) }
func (h *Harness) SettleThread(id string) error                  { return h.threads.SettleThread(id) }
func (h *Harness) OwnWorktree(id string) string                  { return h.Threads().OwnWorktree(id) }
func (h *Harness) DeletePlan(id string) threads.DeletePlan       { return h.threads.DeletePlan(id) }
func (h *Harness) Servers(id string) ([]threads.Server, error)   { return h.threads.Servers(id) }
func (h *Harness) StopServer(id string, pid int) error           { return h.threads.StopServer(id, pid) }
func (h *Harness) OpenServer(port int) error                     { return h.threads.OpenServer(port) }
func (h *Harness) RenameThread(id, title string) error           { return h.threads.RenameThread(id, title) }
func (h *Harness) SetAccess(id, access string) error             { return h.threads.SetAccess(id, access) }
func (h *Harness) TypeInto(id, text string) error                { return h.threads.TypeInto(id, text) }
func (h *Harness) RefreshPRs()                                   { h.threads.RefreshPRs() }
func (h *Harness) AckPR(id string) error                         { return h.threads.AckPR(id) }
func (h *Harness) Cleanup(id string, force bool) error           { return h.threads.Cleanup(id, force) }
func (h *Harness) Cleanups() []threads.Cleanup                   { return h.threads.Cleanups() }

func (h *Harness) DeleteThread(id string, worktree bool) error {
	return h.threads.DeleteThread(id, worktree)
}

func (h *Harness) WakeThread(id string) (threads.Thread, error) {
	return h.threads.WakeThread(id)
}

func (h *Harness) ForkThread(id, at string) (threads.Thread, error) {
	return h.threads.ForkThread(id, at)
}

func (h *Harness) SideChatDelete(id string, unused bool) error {
	return h.threads.SideChatDelete(id, unused)
}

func (h *Harness) ChatHistory(id string, after int64) ([]chatlog.Event, error) {
	return h.chat.ChatHistory(id, after)
}
func (h *Harness) ChatWatch(id string) error { return h.chat.ChatWatch(id) }
func (h *Harness) ChatUnwatch(id string)     { h.chat.ChatUnwatch(id) }
func (h *Harness) RewindFilesOffer(id string) chat.FilesRewind {
	return h.chat.RewindFilesOffer(id)
}
func (h *Harness) RewindPreview(id, message string) (chat.FilesRewind, error) {
	return h.chat.RewindPreview(id, message)
}
func (h *Harness) RewindTo(id, message string, files bool) (string, error) {
	return h.chat.RewindTo(id, message, files)
}
func (h *Harness) RewindOff() string { return h.chat.RewindOff() }

func (h *Harness) Limits() limits.Limits { return h.limits.Limits()[0] }
func (h *Harness) RefreshLimits() (limits.Limits, error) {
	all, err := h.limits.RefreshLimits()
	return all[0], err
}

func (h *Harness) ChatSend(id, text string, images ...chat.Image) (chat.Sent, error) {
	return h.chat.ChatSend(id, text, images, nil)
}

// ChatSendWith is ChatSend with side chats of the thread attached.
func (h *Harness) ChatSendWith(id, text string, sideChats ...string) (chat.Sent, error) {
	return h.chat.ChatSend(id, text, nil, sideChats)
}

func (h *Harness) SideChatStart(parent, text string, images ...chat.Image) (threads.Thread, error) {
	return h.chat.SideChatStart(parent, text, images)
}
func (h *Harness) ChatInterrupt(id string) error        { return h.chat.ChatInterrupt(id) }
func (h *Harness) ChatSendNow(id, message string) error { return h.chat.ChatSendNow(id, message) }
func (h *Harness) ChatPending() []chat.Pending          { return h.chat.ChatPending() }
func (h *Harness) ChatFiles(id, query string) ([]string, error) {
	return h.chat.ChatFiles(id, query)
}
func (h *Harness) ChatAnswerInput(id, request, input string) error {
	return h.chat.ChatAnswerInput(id, request, input)
}
func (h *Harness) ChatAnswer(id, request string, allow bool, message string) error {
	return h.chat.ChatAnswer(id, request, allow, message)
}

// Interrupt stops the thread's turn, as the front end does.
func (h *Harness) Interrupt(id string) error { return h.ChatInterrupt(id) }

func (h *Harness) TermOpen(id string, cols, rows int) error {
	return h.terminal.TermOpen(id, cols, rows)
}
func (h *Harness) TermWrite(id, data string) error { return h.terminal.TermWrite(id, data) }
func (h *Harness) TermResize(id string, cols, rows int) error {
	return h.terminal.TermResize(id, cols, rows)
}
func (h *Harness) TermClose(id string) { h.terminal.TermClose(id) }

func (h *Harness) Issues(refresh bool) ([]issues.Issue, error) { return h.issues.Issues(refresh) }
func (h *Harness) StartIssue(number int) (threads.Thread, error) {
	return h.issues.StartIssue(number, "", "", "", "", false)
}
func (h *Harness) StartIssueWith(number int, action string) (threads.Thread, error) {
	return h.issues.StartIssue(number, action, "", "", "", false)
}
func (h *Harness) StartIssueOwn(number int, action, prompt, model, effort string) (threads.Thread, error) {
	return h.issues.StartIssue(number, action, prompt, model, effort, false)
}
func (h *Harness) MoveIssue(number int, section string) error {
	return h.issues.MoveIssue(number, section)
}
func (h *Harness) UndoMove(number int) error { return h.issues.UndoMove(number) }
func (h *Harness) IssueDetail(number int) (issues.IssueDetail, error) {
	return h.issues.IssueDetail(number)
}

func (h *Harness) AddNote(text string) (notes.Note, error) { return h.notes.AddNote(text) }
func (h *Harness) UpdateNote(id, text string) (notes.Note, error) {
	return h.notes.UpdateNote(id, text)
}
func (h *Harness) SetNotePinned(id string, pinned bool) (notes.Note, error) {
	return h.notes.SetNotePinned(id, pinned)
}
func (h *Harness) SetNoteArchived(id string, archived bool) (notes.Note, error) {
	return h.notes.SetNoteArchived(id, archived)
}
func (h *Harness) DeleteNote(id string) error { return h.notes.DeleteNote(id) }
func (h *Harness) AddNoteImage(id, data, mime string) (notes.Note, error) {
	return h.notes.AddNoteImage(id, data, mime)
}
func (h *Harness) RemoveNoteImage(id, url string) (notes.Note, error) {
	return h.notes.RemoveNoteImage(id, url)
}
func (h *Harness) NoteToIssue(id string) error { return h.notes.NoteToIssue(id) }
func (h *Harness) NoteToThread(id string) (threads.Thread, error) {
	return h.notes.NoteToThread(id)
}

func (h *Harness) Settings() settings.Settings { return h.settings.Settings() }
func (h *Harness) SetProjectsDir(dir string) (settings.Settings, error) {
	return h.settings.SetProjectsDir(dir)
}
func (h *Harness) SetTheme(theme string) (settings.Settings, error) {
	return h.settings.SetTheme(theme)
}

func (h *Harness) SetThemes(dark, light string) (settings.Settings, error) {
	return h.settings.SetThemes(dark, light)
}

func (h *Harness) SetAccents(dark, light string) (settings.Settings, error) {
	return h.settings.SetAccents(dark, light)
}

func (h *Harness) SetCleanup(event, mode string) (settings.Settings, error) {
	return h.settings.SetCleanup(event, mode)
}

func (h *Harness) SetTextScale(scale float64) (settings.Settings, error) {
	return h.settings.SetTextScale(scale)
}

func (h *Harness) SetKeepAwake(on bool) (settings.Settings, error) {
	return h.settings.SetKeepAwake(on)
}

func (h *Harness) SetPromptSend(mode string) (settings.Settings, error) {
	return h.settings.SetPromptSend(mode)
}

func (h *Harness) SetBrowserEnabled(on bool) (settings.Settings, error) {
	return h.settings.SetBrowserEnabled(on)
}

func (h *Harness) SetDigestSchedule(schedule string) (settings.Settings, error) {
	return h.settings.SetDigestSchedule(schedule)
}

func (h *Harness) SetThreadPlace(place string) (settings.Settings, error) {
	return h.settings.SetThreadPlace(place)
}

func (h *Harness) SetThreadAccess(access string) (settings.Settings, error) {
	return h.settings.SetThreadAccess(access)
}

func (h *Harness) PickDataDir() (settings.DataDirChoice, error) { return h.settings.PickDataDir() }

func (h *Harness) SetDataDir(dir string, withData bool) error {
	return h.settings.SetDataDir(dir, withData)
}

func (h *Harness) Ledger(days int, key string) (stats.Ledger, error) {
	return h.stats.Ledger(days, key)
}

// Events are the main project's recorded events of the kind given, oldest first.
func (h *Harness) Events(t *testing.T, kind string) []activity.Event {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(h.State, "data", "main", "events", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []activity.Event
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(string(data)) {
			var e activity.Event
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			if e.Kind == kind {
				out = append(out, e)
			}
		}
	}
	return out
}

// GHUser is the login gh api user prints.
const GHUser = "me"

// PRJSON is what gh pr list prints for one pull request.
func PRJSON(state string, draft bool, rollup string, comments, reviews int) string {
	repeat := func(n int) string {
		return "[" + strings.TrimSuffix(strings.Repeat(`{"id":"x"},`, n), ",") + "]"
	}
	return fmt.Sprintf(`[{"number":12,"url":"https://github.com/acme/widgets/pull/12","state":%q,"isDraft":%v,"statusCheckRollup":%s,"comments":%s,"reviews":%s,"updatedAt":"2026-10-01T12:00:00Z"}]`,
		state, draft, rollup, repeat(comments), repeat(reviews))
}

// WithHead adds the commit the PR's branch ended on to what PRJSON printed.
func WithHead(prJSON, oid string) string {
	return strings.Replace(prJSON, `"isDraft"`, fmt.Sprintf(`"headRefOid":%q,"isDraft"`, oid), 1)
}

// MergeAs merges branch into main on the bare repo origin as GitHub's merge button does, and returns
// the commit the pull request ended on. how is "merge" (a merge commit), "squash" or "rebase", each
// with "+delete" after it to delete origin's branch too. GitHub commits a squash or rebase under its
// own name, so the commits on main get new ids even when the branch already sits on main.
func MergeAs(t *testing.T, origin, branch, how string) string {
	t.Helper()
	how, drop := strings.CutSuffix(how, "+delete")
	clone := filepath.Join(t.TempDir(), "merge")
	gitIn(t, filepath.Dir(clone), "clone", "--quiet", origin, clone)
	head := gitIn(t, clone, "rev-parse", "origin/"+branch)
	switch how {
	case "merge":
		gitIn(t, clone, "merge", "--quiet", "--no-ff", "-m", "Merge "+branch, "origin/"+branch)
	case "squash":
		gitIn(t, clone, "merge", "--quiet", "--squash", "origin/"+branch)
		gitIn(t, clone, "commit", "--quiet", "-m", "Squash "+branch)
	case "rebase":
		gitIn(t, clone, "checkout", "--quiet", "-B", "main", "origin/"+branch)
		gitIn(t, clone, "rebase", "--quiet", "--force-rebase", "origin/main")
	default:
		t.Fatalf("MergeAs: no merge %q", how)
	}
	gitIn(t, clone, "push", "--quiet", "origin", "main")
	if drop {
		gitIn(t, clone, "push", "--quiet", "origin", "--delete", branch)
	}
	return head
}

// gitIn runs git in dir as GitHub's merge button, with no user or system config.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gittest.Command(dir, append([]string{"-c", "user.name=GitHub", "-c", "user.email=noreply@github.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// WithMergeable adds whether the PR merges into its base, as GitHub says it, to what PRJSON printed.
func WithMergeable(prJSON, mergeable string) string {
	return strings.Replace(prJSON, `"isDraft"`, fmt.Sprintf(`"mergeable":%q,"isDraft"`, mergeable), 1)
}

// Ask sends a command to the app's control socket, as the orchestrator mcp command does. The socket answers only what a
// client may run with a token.
func (h *Harness) Ask(t *testing.T, req ctl.Request) ctl.Response {
	t.Helper()
	resp, err := ctl.Call(context.Background(), ctl.SocketPath(h.State), req)
	if err != nil {
		t.Fatalf("asking %s: %v", req.Cmd, err)
	}
	return resp
}

// AskTool runs a command as a tool call does: the router's handler, with the thread and project the request names. A
// tool call sets both from the thread's token, and the control socket never answers such a command. A request with no
// project gets the asking thread's, else the current one, as a tool call from a thread of that project would.
func (h *Harness) AskTool(t *testing.T, req ctl.Request) ctl.Response {
	t.Helper()
	if req.Project == "" {
		req.Project = h.Threads().Current().Key()
		if name, err := thread.ParseName(req.Thread); err == nil {
			req.Project = name.Project
		}
	}
	return h.App.Answer(context.Background(), req)
}

func (h *Harness) Evidence(id string) []evidence.Evidence { return h.evidence.Evidence(id) }
func (h *Harness) DeleteEvidence(id, evidenceID string) error {
	return h.evidence.DeleteEvidence(id, evidenceID)
}

func (h *Harness) BrowserOpen(id, url string) (browser.BrowserState, error) {
	return h.browser.BrowserOpen(id, url)
}
func (h *Harness) BrowserState(id string) browser.BrowserState { return h.browser.BrowserState(id) }
func (h *Harness) BrowserScreenshot(id, caption string) (evidence.Evidence, error) {
	return h.browser.BrowserScreenshot(id, caption)
}
func (h *Harness) BrowserClose(id string) { h.browser.BrowserClose(id) }

// DigestCall is one run of claude as the digest makes it.
type DigestCall struct {
	Dir  string
	Env  []string
	Args []string
}

// FakeClaude stands in for claude: it records each digest run, runs Do, if set, as claude would act, and prints
// Reply, "[]" when it is empty. The sign-in check is not a run; AuthErr is what it fails with.
type FakeClaude struct {
	mu      sync.Mutex
	calls   []DigestCall
	Do      func(call DigestCall)
	Reply   string
	Err     error
	AuthErr error
}

// RunEnv is the run.EnvRunner of the digest.
func (f *FakeClaude) RunEnv(_ context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	call := DigestCall{Dir: dir, Env: env, Args: append([]string{name}, args...)}
	if slices.Equal(args, []string{"auth", "status"}) {
		f.mu.Lock()
		defer f.mu.Unlock()
		return nil, f.AuthErr
	}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	do, reply, err := f.Do, cmp.Or(f.Reply, "[]"), f.Err
	f.mu.Unlock()
	if do != nil {
		do(call)
	}
	if err != nil {
		return nil, err
	}
	return []byte(reply), nil
}

// SetReply sets what claude prints as its reply to the digest.
func (f *FakeClaude) SetReply(reply string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Reply = reply
}

// Calls are the runs so far.
func (f *FakeClaude) Calls() []DigestCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// SetDo sets what claude does when called.
func (f *FakeClaude) SetDo(do func(DigestCall)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Do = do
}

func (h *Harness) Digest() digest.Digest                      { return h.digest.Digest() }
func (h *Harness) RunDigest() error                           { return h.digest.RunDigest() }
func (h *Harness) DismissDigestItem(id string) error          { return h.digest.DismissDigestItem(id) }
func (h *Harness) DigestToNote(id string) (notes.Note, error) { return h.digest.DigestToNote(id) }
