// Package sessions tracks the agents in the private tmux server: their state, their pull requests and their clean-up.
package sessions

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/media"
	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/internal/scoped"
	"github.com/nednella/orchestrator/desktop/internal/warn"
	"github.com/nednella/orchestrator/internal/activity"
	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/folder"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/mcp"
	"github.com/nednella/orchestrator/internal/procs"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/session"
	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/usage"
	"github.com/nednella/orchestrator/internal/util"
)

const (
	pollEvery    = 2 * time.Second
	stopSettle   = time.Second // Claude Code may write a turn's last reply just after the turn's end
	tmuxTimeout  = 5 * time.Second
	defaultTitle = "agent"
	endedSettle  = 15 * time.Second // a state file this fresh may belong to a session tmux has not listed yet
	startCols    = 120
	startRows    = 32
	hostWait     = 10 * time.Second // for a new session's host to listen
	hostStarting = 3 * time.Second  // for a host that is still starting to listen, before a later message is dropped
)

var (
	errNotSession = errors.New("the shell is not a session")
)

// Session is one agent as the front end sees it.
type Session struct {
	ID          string           `json:"id"`
	Project     string           `json:"project"` // the key of the project the session belongs to
	N           int              `json:"n"`
	Title       string           `json:"title"`
	State       session.State    `json:"state"`
	Detail      string           `json:"detail"`
	LastEventAt int64            `json:"lastEventAt"`
	CreatedAt   int64            `json:"createdAt"` // when the thread first started
	WokeAt      int64            `json:"wokeAt"`    // when it last started: its first start, or its latest wake
	Issue       int              `json:"issue"`
	Access      session.Access   `json:"access"`
	Action      string           `json:"action"` // the queue action the session was started with, "" when none
	ForkOf      string           `json:"forkOf"` // the thread the session was forked from, "" when none
	ForkAt      string           `json:"forkAt"` // the uuid of the reply the fork's chat was cut at, "" when none
	Parent      string           `json:"parent"` // the thread that delegated to this one, "" for a thread the owner started
	Model       string           `json:"model"`  // the model the agent runs: the start choice until its first turn, then what it used last; "" when unknown
	Effort      string           `json:"effort"` // the effort the agent runs, kept like Model; "" when unknown
	History     []session.Change `json:"history"`
	Evidence    int              `json:"evidence"`
	Browser     bool             `json:"browser"`

	Branch        string        `json:"branch"`
	Worktree      string        `json:"worktree"`
	PR            *PR           `json:"pr"`
	Git           *BranchStatus `json:"git"` // nil when git cannot read the branch
	PRAttention   string        `json:"prAttention"`
	Cleanup       string        `json:"cleanup"`
	CleanupReason string        `json:"cleanupReason"`

	SettledAt int64 `json:"settledAt"` // unix ms; 0 while the session runs

	Tokens int64   `json:"tokens"` // what the agent used in this run of the app, 0 when not known
	Cost   float64 `json:"cost"`   // what those tokens cost at list prices, in US dollars

	Memory int64 `json:"memory"` // bytes held by the processes under the session's pane, rounded up; 0 when not known

	Subagents int `json:"subagents"` // the tasks other than shells that its Claude runs in the background, which a settle stops
}

// Project is a project as the front end sees it, with how its sessions stand.
type Project struct {
	Key      string `json:"key"` // the project's key, as in the project field of events and in session ids
	Name     string `json:"name"`
	Dir      string `json:"dir"`
	Repo     string `json:"repo"`
	NeedsYou int    `json:"needsYou"`
	Working  int    `json:"working"`
	Sessions int    `json:"sessions"`

	NeedsSetup     bool `json:"needsSetup"`     // the project has no queue sections, so the app offers to set it up
	SetupDismissed bool `json:"setupDismissed"` // the owner turned the offer down; it stays in the project's menu on the rail
}

// Sessions tracks the agents in the private tmux server and tells the front end when they change.
type Sessions struct {
	tmux     *term.Tmux
	agents   agent.Registry
	binary   string
	run      run.Runner
	stateDir string
	localDir string
	emit     func(event string, payload any)
	warn     *warn.Warnings

	createMu  sync.Mutex           // one creation, or one refresh of the list from tmux, at a time: a list read before a creation must not be applied after it
	stateRead map[string]fileStamp // each state file as the last refresh read it, under createMu
	shellMu   sync.Mutex           // one shell check-and-create at a time: two callers at once would both find no shell and start two
	issueMu   sync.Mutex           // one issue check-and-create at a time: two starts at once would both find no session and start two
	childMu   sync.Mutex           // one child check-and-create at a time: two delegations at once would both pass the limits

	settling map[string]chan struct{} // under mu, the threads a settle stops now, each closed once that settle ends: one settle of a thread at a time

	mu         sync.Mutex
	ctx        context.Context
	project    project.Project
	configured ProjectList
	tally      Tally
	activity   Activity
	closeTerm  func(id string)
	evidence   Evidence
	browsers   Browsers
	awake      Awake
	keepAwake  func(projectKey string) bool
	editorApp  func() string
	newPlace   func() string
	info       []term.Info              // the live sessions under a host
	shells     []term.Shell             // every project's shell sessions
	sentShells map[string][]Shell       // by project key, the shells last sent
	known      map[string]term.Info     // the last info of every session seen, for when it ends
	afterList  func()                   // see AfterList
	ended      map[string]*endedSession // settled sessions, whose tmux session is gone, until deleted
	nums       map[string]int           // the number each session shows: its state file's
	lastNum    map[string]int           // the highest number given or recorded in each project
	deleted    map[string]bool          // the threads the owner deleted in this run: nothing brings them back
	deleting   map[string]bool          // the threads a delete runs for: they are not woken, forked or given children
	waking     map[string]bool          // the threads a wake runs for: no clean-up starts on them
	records    map[string]session.Record
	cwds       map[string]string // the working directory each session's last event reported
	seen       map[string]session.State
	history    map[string][]session.Change
	loaded     bool                 // the first refresh is in, so later changes may raise attention
	synced     bool                 // the first refresh is synced, so a session seen from now on is a new one
	lastSent   map[string][]Session // by project key
	lastProjs  []Project
	lastIssues string
	life       *Lifecycle
	made       *madeTrees
	procs      *procs.Table
	repoOf     func(dir string) string
	onIssues   func()
	wakes      map[string][]heldPrompt // what waits for each thread to take it, oldest first: its PR's commands, each once, and the messages threads told it
	afterTurn  map[string][]func()     // what waits for a session's turn to end or the session to go, oldest first: the settle after its PR merged or closed
	slept      func(id string)         // hears of each thread that stopped running or was found asleep; OnSlept sets it before Run
	onDelete   func(id string)         // hears of each thread deleted; OnDelete sets it before Run
	found      []string                // the threads a Refresh took in as asleep, for slept to hear of once it ends
	readers    map[string]*reader      // each session's transcript reader
	spent      map[string]usage.Tokens // what each session's agent used
	usageReads atomic.Int64            // how many times countUsage ran, which a test counts
	held       map[string]int64        // the memory each live session held at the last sample
	delivering map[string]bool         // the threads whose held prompts, or a message told to them, are being sent now
	idling     map[string]bool         // the threads SettleIdle puts to sleep now: what they are told waits until they sleep
	tokens     map[string]string       // the SHA-256, in hex, of the token each thread's host was last started with
	refused    map[string]bool         // the flags Claude did not know in this run of the app, which it now starts without
	restarting map[string]bool         // the threads settled for a change of access until they wake again: they keep their servers
	background map[string]Background   // what each live thread's Claude does beside its turns, as its host's chat log says
	memory     Memory
	serverPID  int // the tmux server's, as tmux gave it; asked again once ps no longer lists it
}

// ProjectList is the configured projects.
type ProjectList interface {
	List() []project.Project
}

// Tally records when a session waits on the user.
type Tally interface {
	Opened(id, title string, issue int, model string, rec session.Record, at time.Time)
	Closed(id string, at time.Time) bool
}

// Activity records what the user, the agents and the app do.
type Activity interface {
	Record(projectKey string, e activity.Event)
	Since(projectKey, day string, each func(activity.Event))
}

// Evidence is what the sessions need of the evidence store.
type Evidence interface {
	Count(id string) int
	Purge(id string) int
}

// Browsers is what the sessions need of the browsers.
type Browsers interface {
	AgentReady() bool
	AgentEnv(id, tmpDir string) ([]string, error)
	Has(id string) bool
	Close(ctx context.Context, id string)
	Forget(ctx context.Context, id string)
	PIDs() []int
}

// Awake holds off idle sleep.
type Awake interface {
	Hold(on bool)
}

// Options is what a Sessions needs from outside.
type Options struct {
	Tmux          *term.Tmux
	Agents        agent.Registry
	Binary        string // this app's own binary, whose host command hosted agents run under
	StateDir      string
	LocalDir      string // pull-request tracking and the clean-up log: never synced
	Projects      ProjectList
	Current       project.Project
	Emit          func(event string, payload any)
	Run           run.Runner
	Stream        run.Streamer // long-running commands: gh webhook forward
	Tally         Tally
	Activity      Activity
	CloseTerminal func(id string)
	Evidence      Evidence
	Browsers      Browsers
	Awake         Awake
	KeepAwake     func(projectKey string) bool
	EditorApp     func() string // the macOS app that opens a thread's folder
	ThreadPlace   func() string // where a thread with no issue and no place picked runs: worktree or project, "" for a worktree
}

// New tracks the sessions of o.Tmux, and follows their pull requests.
func New(o Options) *Sessions {
	s := newSessions(o)
	s.life = newLifecycle(o.Run, o.Stream, o.LocalDir, o.StateDir, s, o.Activity, o.Emit)
	return s
}

// Stop ends the clean-ups that are running and waits for them.
func (s *Sessions) Stop() { s.life.stopCleanups() }

// Hook sets what the sessions ask of the issues: the repo of a folder, a nudge when the
// sessions tied to issues change, and what the pull request watch needs.
func (s *Sessions) Hook(repoOf func(dir string) string, onIssues func(), repos Repos) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.repoOf, s.onIssues = repoOf, onIssues
	s.life.repos = repos
}

// Touch tells the front end that something shown in the session list changed.
func (s *Sessions) Touch() { s.changed() }

func newSessions(o Options) *Sessions {
	return &Sessions{
		tmux: o.Tmux, agents: o.Agents, binary: o.Binary, run: o.Run, stateDir: o.StateDir, localDir: o.LocalDir, emit: o.Emit, warn: warn.New(o.Emit),
		project: o.Current, configured: o.Projects, tally: o.Tally, activity: o.Activity, closeTerm: o.CloseTerminal, evidence: o.Evidence, browsers: o.Browsers, awake: o.Awake, keepAwake: o.KeepAwake, editorApp: o.EditorApp, newPlace: o.ThreadPlace,
		repoOf: func(string) string { return "" }, onIssues: func() {},
		known: map[string]term.Info{}, ended: map[string]*endedSession{}, deleted: map[string]bool{}, deleting: map[string]bool{}, waking: map[string]bool{},
		nums: map[string]int{}, lastNum: map[string]int{}, restarting: map[string]bool{},
		records:    map[string]session.Record{},
		cwds:       map[string]string{},
		seen:       map[string]session.State{},
		history:    map[string][]session.Change{},
		wakes:      map[string][]heldPrompt{},
		delivering: map[string]bool{},
		idling:     map[string]bool{},
		settling:   map[string]chan struct{}{},
		afterTurn:  map[string][]func(){},
		slept:      func(string) {},
		onDelete:   func(string) {},
		readers:    map[string]*reader{},
		spent:      map[string]usage.Tokens{},
		held:       map[string]int64{},
		tokens:     map[string]string{},
		refused:    map[string]bool{},
		background: map[string]Background{},
		stateRead:  map[string]fileStamp{},
		sentShells: map[string][]Shell{},
		made:       newMadeTrees(o.LocalDir),
		procs:      procs.New(control.SocketPath(o.StateDir), anyDir(o.Run)),
		ctx:        context.Background(),
	}
}

// Start polls tmux until ctx ends.
func (s *Sessions) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	s.Refresh()
	go s.sweepServers()
	go func() {
		tick := time.NewTicker(pollEvery)
		defer tick.Stop()
		sample := time.NewTicker(sampleEvery)
		defer sample.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s.Refresh()
			case <-sample.C:
				s.SampleMemory()
			}
		}
	}()
	go s.SampleMemory()
	return nil
}

// Refresh reads tmux and the state files. A state file with no tmux session
// behind it is a session that ended. When tmux cannot be listed the state stays as it was:
// a failed listing says nothing about which sessions are gone.
func (s *Sessions) Refresh() {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	s.mu.Lock()
	parent := s.ctx
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, tmuxTimeout)
	defer cancel()
	listing := time.Now()
	infos, shells, err := s.tmux.ListAll(ctx)
	if s.afterList != nil {
		s.afterList()
	}
	if err != nil {
		if parent.Err() == nil {
			s.warn.Report("tmux", util.FirstLine(err.Error()))
		}
		return
	}
	s.warn.Clear("tmux")
	stamps := map[string]fileStamp{}
	records := bus.ReadAll(s.stateDir, func(name string, info fs.FileInfo) bool {
		// A shell is no thread.
		if n, err := session.ParseName(name); err == nil && n.IsShell() {
			return true
		}
		stamps[name] = stampOf(info)
		return s.stateRead[name] == stamps[name]
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	// A settle that ran while tmux was listed took its thread out of the list's world: the list still has it live.
	infos = slices.DeleteFunc(infos, func(in term.Info) bool {
		e := s.ended[in.Name.String()]
		return e != nil && e.at.After(listing)
	})
	s.info = infos
	s.shells = shells
	s.announceShells()
	for _, in := range s.info {
		if name := in.Name.String(); s.ended[name] != nil {
			records[name] = s.revive(name, in)
		}
	}
	for name, rec := range records {
		s.holdNumber(name, rec)
		switch {
		case slices.ContainsFunc(s.info, func(i term.Info) bool { return i.Name.String() == name }):
			s.take(rec)
		default:
			s.adoptEnded(name, rec)
		}
	}
	s.markRead(records, stamps, s.info)
	s.readTokens()
	s.number()
	s.loaded = true
	s.sync()
	s.synced = true
	if found := s.found; len(found) > 0 {
		s.found = nil
		slices.Sort(found)
		go s.heardAsleep(found)
	}
}

// fileStamp tells whether a file changed since it was read.
type fileStamp struct {
	mod  time.Time
	size int64
}

func stampOf(info fs.FileInfo) fileStamp { return fileStamp{info.ModTime(), info.Size()} }

// markRead keeps the stamps the state files had when they were read, so the next refresh reads a file again only once
// it changes. A file of a session that is neither live, ended nor deleted, such as one still settling, is read each
// time. It needs createMu and mu.
func (s *Sessions) markRead(records map[string]session.Record, stamps map[string]fileStamp, infos []term.Info) {
	for name := range records {
		if s.ended[name] != nil || s.deleted[name] || slices.ContainsFunc(infos, func(i term.Info) bool { return i.Name.String() == name }) {
			s.stateRead[name] = stamps[name]
		} else {
			delete(s.stateRead, name)
		}
	}
	for name := range s.stateRead {
		if _, ok := stamps[name]; !ok {
			delete(s.stateRead, name)
		}
	}
}

// revive takes back a session that tmux lists although it was marked settled, as a wake takes it back: its state file
// keeps its identity and history and says it runs from when tmux started it. It returns the record to take in. It
// needs mu.
func (s *Sessions) revive(name string, in term.Info) session.Record {
	rec := s.woken(name, in)
	delete(s.ended, name)
	if err := bus.WriteState(s.stateDir, rec); err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: saving a thread that runs again: %v\n", err)
	}
	return rec
}

// woken is the record of the settled thread id once it runs again as in, started at in.Created: idle, with what the
// thread was started as and its conversation, and its history, which gets the change. It needs mu.
func (s *Sessions) woken(id string, in term.Info) session.Record {
	e := s.ended[id]
	rec := e.record()
	at := in.Created
	rec.State, rec.Event, rec.At, rec.Detail, rec.Notify, rec.Tool, rec.Command, rec.SettledAt = session.Idle, "", at, "", "", "", "", 0
	rec.Identity = keptFrom(identityOf(in), rec)
	rec.Woke = at.UnixMilli()
	history := s.history[id]
	if len(history) == 0 {
		history = rec.Changes
	}
	rec.Changes = session.AddChange(history, session.Change{State: session.Idle, At: at.UnixMilli()})
	return rec
}

// endedSession is a session that is gone but whose row stays, settled, until deleted.
type endedSession struct {
	info term.Info
	rec  session.Record
	at   time.Time
}

// persist saves what the row needs to outlive the app. It needs mu.
func (e *endedSession) record() session.Record {
	rec := e.rec
	rec.State, rec.SettledAt = session.Settled, e.at.UnixMilli()
	rec.Identity = keptFrom(identityOf(e.info), e.rec)
	return rec
}

// identityOf is what the session was started as, with its tmux session's start as both its first and latest start.
func identityOf(in term.Info) session.Identity {
	id := session.Identity{Title: in.Title, Access: in.Access, Action: in.Action, Model: in.Model, Effort: in.Effort, Path: in.Path, Fork: in.Fork, Branch: in.Branch, Parent: in.Parent}
	id.Issue, _ = strconv.Atoi(in.Issue)
	if !in.Created.IsZero() {
		id.Created = in.Created.UnixMilli()
		id.Woke = id.Created
	}
	return id
}

// keptFrom is id with the number, the first and latest start and the result reported that rec holds: tmux knows no
// number and no result, a wake leaves the first start where it was and sets the latest itself, and tmux dates a
// session only to the second.
func keptFrom(id session.Identity, rec session.Record) session.Identity {
	id.N, id.ForkAt = cmp.Or(rec.N, id.N), rec.ForkAt
	id.Created, id.Woke = cmp.Or(rec.Created, id.Created), cmp.Or(rec.Woke, id.Woke)
	id.Reported = max(rec.Reported, id.Reported)
	return id
}

// forkAtOf is the reply the fork's chat was cut at, from the thread's record. It needs mu.
func (s *Sessions) forkAtOf(id string) string {
	if e, ok := s.ended[id]; ok {
		return e.rec.ForkAt
	}
	return s.records[id].ForkAt
}

// startsOf are when the thread first started and when it last started, its first start or its latest wake, as its
// state file holds them, else when its tmux session started. It needs mu.
func (s *Sessions) startsOf(id string, in term.Info) (created, woke time.Time) {
	rec := s.records[id]
	if e, ok := s.ended[id]; ok {
		rec = e.rec
	}
	created, woke = in.Created, in.Created
	if rec.Created > 0 {
		created = time.UnixMilli(rec.Created)
	}
	if rec.Woke > 0 {
		woke = time.UnixMilli(rec.Woke)
	}
	return created, woke
}

// saveIdentity writes what a live session was started as into its state file, which the events keep. Without it, a
// session that ends while the app is closed comes back with no title, access or issue.
func (s *Sessions) saveIdentity(in term.Info) {
	id := in.Name.String()
	rec, err := bus.ReadState(s.stateDir, id)
	if err != nil {
		return
	}
	want := keptFrom(identityOf(in), rec)
	want.N = cmp.Or(want.N, s.nums[id])
	if rec.Identity == want {
		return
	}
	if rec.Session == "" {
		// Dated when the session was created, so its first event is not taken for one that came too late.
		rec = session.Record{Session: id, State: session.Idle, At: in.Created}
	}
	rec.Identity = want
	if err := bus.WriteState(s.stateDir, rec); err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: saving a session's identity: %v\n", err)
	}
}

// adoptEnded takes in a state file whose tmux session is gone, as a settled row. A deleted thread has no row. It needs mu.
func (s *Sessions) adoptEnded(name string, rec session.Record) {
	n, err := session.ParseName(name)
	if err != nil || s.deleted[name] {
		return
	}
	if e, ok := s.ended[name]; ok {
		if rec.Title == "" && e.info.Title != "" { // a late event overwrote the file
			_ = bus.WriteState(s.stateDir, e.record())
		}
		return
	}
	at := time.UnixMilli(rec.SettledAt)
	if rec.SettledAt == 0 {
		at = rec.At
	}
	if time.Since(rec.At) < endedSettle && rec.SettledAt == 0 {
		return
	}
	stale := rec.SettledAt == 0 || rec.State != session.Settled
	info := term.Info{Name: n, Title: rec.Title, Path: rec.Path, Model: rec.Model, Effort: rec.Effort, Access: session.StoredAccess(string(rec.Access)), Action: rec.Action, Fork: rec.Fork, Branch: rec.Branch, Parent: rec.Parent}
	if rec.Issue > 0 {
		info.Issue = strconv.Itoa(rec.Issue)
	}
	if started := cmp.Or(rec.Woke, rec.Created); started > 0 {
		info.Created = time.UnixMilli(started)
	}
	e := &endedSession{info: info, rec: rec, at: at}
	s.ended[name] = e
	s.found = append(s.found, name)
	if stale {
		_ = bus.WriteState(s.stateDir, e.record())
	}
	if len(s.history[name]) == 0 {
		s.history[name] = slices.Clone(rec.Changes)
	}
	if h := s.history[name]; len(h) == 0 || h[len(h)-1].State != session.Settled {
		s.history[name] = append(h, session.Change{State: session.Settled, At: at.UnixMilli()})
	}
}

// endSession turns a session that is gone into a settled row that stays, and saves it. It needs mu.
func (s *Sessions) endSession(id string, at time.Time) {
	if _, ok := s.ended[id]; ok || s.deleted[id] {
		return
	}
	n, err := session.ParseName(id)
	if err != nil {
		return
	}
	info, ok := s.known[id]
	if !ok {
		info = term.Info{Name: n}
	}
	rec := s.records[id]
	if rec.Session == "" {
		rec = session.Record{Session: id, At: at}
	}
	rec.N = cmp.Or(rec.N, s.nums[id]) // no event may have come since the number was saved
	// Nor since SetReported wrote the file.
	if cur, err := bus.ReadState(s.stateDir, id); err == nil {
		rec.Reported = max(rec.Reported, cur.Reported)
	}
	if rec.State == session.Settled && !rec.At.IsZero() {
		at = rec.At
	}
	e := &endedSession{info: info, rec: rec, at: at}
	s.ended[id] = e
	if err := bus.WriteState(s.stateDir, e.record()); err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: saving a settled session: %v\n", err)
	}
	if h := s.history[id]; len(h) == 0 || h[len(h)-1].State != session.Settled {
		s.history[id] = append(h, session.Change{State: session.Settled, At: at.UnixMilli()})
	}
}

// number gives each session that has no number the next one of its project, oldest first, and saves it in its state
// file. It needs mu.
func (s *Sessions) number() {
	var fresh []term.Info
	for _, in := range s.info {
		if _, ok := s.nums[in.Name.String()]; !ok {
			fresh = append(fresh, in)
		}
	}
	for id, e := range s.ended {
		if _, ok := s.nums[id]; !ok {
			fresh = append(fresh, e.info)
		}
	}
	first := func(in term.Info) time.Time {
		created, _ := s.startsOf(in.Name.String(), in)
		return created
	}
	slices.SortFunc(fresh, func(a, b term.Info) int {
		return cmp.Or(first(a).Compare(first(b)), cmp.Compare(a.Name.String(), b.Name.String()))
	})
	for _, in := range fresh {
		id, key := in.Name.String(), in.Name.Project
		s.nums[id] = s.nextNumber(key)
		s.saveNumber(id, s.nums[id])
	}
}

// holdNumber takes the number a state file records as the thread's, and keeps the project from giving it again. It
// needs mu.
func (s *Sessions) holdNumber(id string, rec session.Record) {
	if rec.N == 0 {
		return
	}
	key := session.ProjectKey(id)
	s.lastNum[key] = max(s.lastNum[key], rec.N)
	if !s.deleted[id] && s.nums[id] == 0 {
		s.nums[id] = rec.N
	}
}

// nextNumber is the project's next thread number. It is saved as the project's last one before any thread holds it, so
// no restart gives it again, also after the thread is deleted. It needs mu.
func (s *Sessions) nextNumber(key string) int {
	n := max(s.lastNum[key], s.savedLastNumber(key)) + 1
	if err := atomicfile.Write(s.lastNumberPath(key), []byte(strconv.Itoa(n)+"\n"), 0o600); err != nil {
		log.Printf("orchestrator: saving the last thread number of %s: %v", key, err)
	}
	s.lastNum[key] = n
	return n
}

func (s *Sessions) lastNumberPath(key string) string {
	return filepath.Join(s.localDir, key, "last-thread-number")
}

// savedLastNumber is the project's last thread number as saved, which outlives the state file of a deleted thread; 0
// when none is saved.
func (s *Sessions) savedLastNumber(key string) int {
	data, _ := os.ReadFile(s.lastNumberPath(key))
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return n
}

// saveNumber writes the thread's number into its state file and the records the app holds of it. A new thread has no
// state file yet; saveIdentity writes it with the number. It needs mu.
func (s *Sessions) saveNumber(id string, n int) {
	if rec, ok := s.records[id]; ok {
		rec.N = n
		s.records[id] = rec
	}
	if e, ok := s.ended[id]; ok {
		e.rec.N = n
	}
	rec, err := bus.ReadState(s.stateDir, id)
	if err != nil || rec.Session == "" {
		return
	}
	rec.N = n
	if err := bus.WriteState(s.stateDir, rec); err != nil {
		log.Printf("orchestrator: saving the number of thread %s: %v", id, err)
	}
}

// tmpDir is the session's own TMPDIR, and Claude's scratchpad. It sits in a folder the agent may use
// without asking, so the screenshots and scratch files it makes there need no approval. It goes at the thread's
// clean-up or delete. A shell's name has no token, and would name the folder of every thread's.
func (s *Sessions) tmpDir(name session.Name) string {
	return filepath.Join(s.localDir, name.Project, "tmp", name.Token)
}

// chatLogPath is the chat log the session's host keeps.
func (s *Sessions) chatLogPath(name session.Name) string {
	return filepath.Join(s.localDir, name.Project, "chats", name.Token+".jsonl")
}

// chatMediaDir holds the images sent in the session's chat.
func (s *Sessions) chatMediaDir(name session.Name) string {
	return filepath.Join(s.localDir, name.Project, media.ChatFolder, name.Token)
}

// hostSocket is where the session's host listens. A unix socket's path is short, so it sits in the state folder, in a
// folder named by a hash of the project key: its length does not depend on the project's name.
func (s *Sessions) hostSocket(name session.Name) string {
	sum := sha256.Sum256([]byte(name.Project))
	return filepath.Join(s.stateDir, "hosts", hex.EncodeToString(sum[:4]), name.Token+".sock")
}

// handoverPath is where the handover a session started with is kept: a wake that resumes its conversation needs it
// again, since --resume does not bring back the system prompt.
func (s *Sessions) handoverPath(name session.Name) string {
	return filepath.Join(s.localDir, name.Project, "handovers", name.Token+".md")
}

// handoverOf is the handover the session started with, "" when none.
func (s *Sessions) handoverOf(id string) string {
	name, err := session.ParseName(id)
	if err != nil {
		return ""
	}
	data, _ := os.ReadFile(s.handoverPath(name))
	return string(data)
}

// Delete stops the thread when it runs, as a settle does, and removes it for good: its row, state file, chat log, chat
// images, evidence, handover, usage mark, temp folder, browser window, host socket and shells. Its clean-up log entries
// and its events stay, and so does its branch. With worktree, the worktree orchestrator made for it goes first, by a clean-
// up with every check, and only when no other thread or shell works in it; a worktree that stays refuses the Delete,
// and no thread goes. Without, the worktree stays on disk as it is, and the record that orchestrator made it goes with the
// thread, so no clean-up removes it later. Its asleep descendants go with it, each as it would alone, the deepest
// first, so none is left without the parent that reaches it. With worktree their worktrees go with the thread's, all or
// none. A descendant's worktree that a thread outside the family works in stays on disk, and goes over to that thread,
// so a Delete of it can take the worktree later. A thread with a running descendant is refused. While the delete runs,
// neither the thread nor a descendant can be woken or forked, and none starts a child.
func (s *Sessions) Delete(id string, worktree bool) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if name.IsShell() {
		return errNotSession
	}
	family, err := s.startDelete(id)
	if err != nil {
		return err
	}
	defer s.endDelete(family)
	trees := s.familyWorktrees(s.context(), family)
	// stopped is the thread this Delete has put to sleep, so an error after that says it is asleep now.
	stopped := ""
	if worktree {
		if stopped, err = s.removeWorktrees(family, trees); err != nil {
			return err
		}
	}
	for d, heir := range trees.heirs {
		s.handOverWorktree(d, heir)
	}
	for _, d := range family {
		settled, err := s.remove(d)
		if settled {
			stopped = d
		}
		if err != nil {
			return s.asleepNow(err, stopped)
		}
	}
	return nil
}

// remove stops the thread id of a Delete and its shells, and removes it with its files, and says whether it stopped the thread.
func (s *Sessions) remove(id string) (settled bool, err error) {
	name, err := session.ParseName(id)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	rec := s.records[id]
	if e, ok := s.ended[id]; ok {
		rec = e.rec
	}
	s.mu.Unlock()
	live := s.isLive(id)
	if err := s.settleToDelete(name); err != nil {
		return false, err
	}
	s.closeShellsOf(id, "")
	// Counted here: the count sync starts when the session goes, also at the settle of the clean-up, may run after the
	// delete and find no thread.
	if rec.Transcript != "" {
		s.countUsage(id, rec.Transcript, rec.AgentModel, true)
	}
	if err := s.dropThread(name); err != nil {
		return live, err
	}
	s.onDelete(id)

	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	s.browsers.Forget(ctx, id)
	s.evidence.Purge(id)
	if err := s.made.Forget(name.Project, id); err != nil {
		log.Printf("orchestrator: forgetting the worktree of deleted thread %s: %v", id, err)
	}
	s.awaitHostExit(name)
	s.stopServers(true, id) // before its host socket goes, which says whether it runs elsewhere
	for _, path := range []string{s.chatLogPath(name), s.chatMediaDir(name), s.handoverPath(name), s.usageMarkPath(name), s.tmpDir(name), s.hostSocket(name)} {
		if err := os.RemoveAll(path); err != nil {
			log.Printf("orchestrator: removing %s of deleted thread %s: %v", path, id, err)
		}
	}
	return live, nil
}

// startDelete marks the thread id and its descendants as being deleted and returns them, the deepest first and id
// last, or says why they cannot be: it was deleted, a delete of it or of a descendant runs, one of its
// descendants runs, or a clean-up of one of them runs. It takes childMu, so a child whose start passed its
// checks is running before the descendants are looked at, and createMu, under which a child's clean-up at its sleep
// decides too, so only one of the two goes on. Any other clean-up marks itself before it looks for a delete.
func (s *Sessions) startDelete(id string) ([]string, error) {
	s.childMu.Lock()
	defer s.childMu.Unlock()
	s.createMu.Lock()
	defer s.createMu.Unlock()
	family, err := s.markDeleting(id)
	if err != nil {
		return nil, err
	}
	for _, d := range family {
		if s.life.CleaningUp(d) {
			s.endDelete(family)
			return nil, fmt.Errorf("Thread %d is being cleaned up; delete it when that ends", s.Number(d))
		}
	}
	return family, nil
}

// endDelete takes off the marks startDelete put on the family. It takes mu.
func (s *Sessions) endDelete(family []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range family {
		delete(s.deleting, d)
	}
}

// markDeleting is startDelete's checks and marks, under mu.
func (s *Sessions) markDeleting(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.deleted[id]:
		return nil, fmt.Errorf("Thread %s was deleted", id)
	case s.deleting[id]:
		return nil, fmt.Errorf("Thread %d is being deleted", s.nums[id])
	}
	family := s.familyOf(id)
	for _, d := range family[:len(family)-1] {
		if in, live := s.liveInfo(d); live {
			return nil, fmt.Errorf("Thread %d has a running child, %s; stop it first", s.nums[id], cmp.Or(in.Title, defaultTitle))
		}
		if s.deleting[d] {
			return nil, fmt.Errorf("Thread %d is being deleted", s.nums[d])
		}
	}
	for _, d := range family {
		s.deleting[d] = true
	}
	return family, nil
}

// familyOf is the thread id and its descendants in the order a Delete takes them: the deepest first and id last. It
// needs mu.
func (s *Sessions) familyOf(id string) []string {
	family := s.descendants(id)
	slices.Reverse(family)
	return append(family, id)
}

// cleanupRefused says why a clean-up of the thread, marked pending already, must not go on, or nil when it may: a delete
// of the thread runs or has run, unless by is that delete, a wake of it runs, or, for the sweep, which only acts on an
// asleep thread, it runs again. It takes mu.
func (s *Sessions) cleanupRefused(id string, by cleanupBy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, live := s.liveInfo(id)
	switch {
	case by != byDelete && (s.deleting[id] || s.deleted[id]):
		return fmt.Errorf("Thread %d is being deleted", s.nums[id])
	case s.waking[id]:
		return fmt.Errorf("Thread %d is waking; clean it up once it runs", s.nums[id])
	case by == bySweep && live:
		return fmt.Errorf("Thread %d runs again", s.nums[id])
	}
	return nil
}

// dropThread removes the settled thread's state file and row, and marks it deleted, so nothing brings it back. It
// holds createMu, so a refresh that read the state file before it went does not apply it after.
func (s *Sessions) dropThread(name session.Name) error {
	id := name.String()
	s.createMu.Lock()
	defer s.createMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.ended[id]; !ok {
		return fmt.Errorf("No thread %s", id)
	}
	if err := bus.RemoveState(s.stateDir, id); err != nil {
		return fmt.Errorf("deleting the thread: %w", err)
	}
	s.deleted[id] = true
	delete(s.deleting, id)
	delete(s.ended, id)
	delete(s.history, id)
	delete(s.known, id)
	delete(s.nums, id)
	delete(s.readers, id)
	delete(s.spent, id)
	s.dropTold(id, ErrToldDeleted)
	s.dropResultsOf(id)
	delete(s.wakes, id)
	delete(s.afterTurn, id)
	delete(s.cwds, id)
	delete(s.stateRead, id)
	s.life.forget(id)
	s.forgetToken(name)
	s.sync()
	return nil
}

// settleToDelete settles the thread when it runs. It holds createMu, so a refresh with a list read before the settle
// does not take the thread back as live.
func (s *Sessions) settleToDelete(name session.Name) error {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	s.mu.Lock()
	live := slices.ContainsFunc(s.info, func(in term.Info) bool { return in.Name == name })
	s.mu.Unlock()
	if !live {
		return nil
	}
	return s.Settle(name.String())
}

// OnSlept has fn hear of each thread that stops running while the app runs, other than for a restart, once its host has
// logged Claude's exit or hostWait has passed, and of each one the app finds asleep, as it starts. fn runs on a
// goroutine of its own for a thread that stops, and on one for all the threads a Refresh found asleep, one after
// another. It comes before Run.
func (s *Sessions) OnSlept(fn func(id string)) { s.slept = fn }

// OnDelete has fn hear of each thread once it is deleted. It comes before Run.
func (s *Sessions) OnDelete(fn func(id string)) { s.onDelete = fn }

// heardAsleep tells the function OnSlept set of each of the threads ids found asleep, in turn.
func (s *Sessions) heardAsleep(ids []string) {
	for _, id := range ids {
		s.slept(id)
	}
}

// heardSleep tells the thread's stop to the function OnSlept set, once the thread's host is gone.
func (s *Sessions) heardSleep(id string) {
	name, err := session.ParseName(id)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(s.context(), hostWait)
	defer cancel()
	if err := host.Gone(ctx, s.hostSocket(name)); err != nil {
		log.Printf("orchestrator: the host of thread %s still runs after it stopped: %v", id, err)
	}
	s.slept(id)
}

// awaitHostExit waits, for at most hostWait, until the thread's host is gone: it logs Claude's exit as it stops, and
// a chat log removed before that would be made again.
func (s *Sessions) awaitHostExit(name session.Name) {
	ctx, cancel := context.WithTimeout(s.context(), hostWait)
	defer cancel()
	if err := host.Gone(ctx, s.hostSocket(name)); err != nil {
		log.Printf("orchestrator: deleting thread %s while its host runs: %v", name, err)
	}
}

// familyTrees are the worktrees orchestrator made for the threads of a Delete's family that are on disk, as a Delete with the
// box would take them. The box offers count of them, and a refusal counts the same.
type familyTrees struct {
	own     []string          // the threads whose worktrees only the family works in, the deepest first
	kept    []keptTree        // the descendants' worktrees that a shell or a thread outside the family works in
	topKept string            // why the thread's own worktree is kept, as kept's are; "" when it is not
	heirs   map[string]string // by descendant, the fork outside the family that takes its worktree over
}

// keptTree is a worktree of a descendant that a Delete with the box keeps, and why.
type keptTree struct{ id, reason string }

// count is how many worktrees the box offers: the thread's own worktree when only the family works in it, and every
// descendant's but one a fork takes over. A thread whose own worktree is shared gets no box, as without children, since
// a Delete with it would always be refused.
func (t familyTrees) count() int {
	if t.topKept != "" {
		return 0
	}
	return len(t.own) + len(t.kept)
}

// familyWorktrees sorts the worktrees of family, the deepest first and the thread last, by who else works in them: no
// one outside the family, a fork outside it that takes a descendant's over (heirOf), or a shell or another thread,
// which keeps it. A shell keeps a worktree a fork takes over too, but the fork still gets it when the Delete runs
// without the box.
func (s *Sessions) familyWorktrees(ctx context.Context, family []string) familyTrees {
	top := family[len(family)-1]
	trees := familyTrees{heirs: map[string]string{}}
	for _, id := range family {
		path := s.madeWorktree(id)
		if path == "" {
			continue
		}
		key := session.ProjectKey(id)
		heir := ""
		if id != top {
			heir = s.heirOf(key, path, family)
		}
		reason := s.life.shellIn(ctx, key, path, family...)
		if user := s.usersOf(path, true, family...); reason == "" && user != "" && heir == "" {
			reason = fmt.Sprintf("Thread %s also works in this worktree", user)
		}
		switch {
		case heir != "":
			trees.heirs[id] = heir
			if reason != "" {
				trees.kept = append(trees.kept, keptTree{id, reason})
			}
		case reason != "" && id == top:
			trees.topKept = reason
		case reason != "":
			trees.kept = append(trees.kept, keptTree{id, reason})
		default:
			trees.own = append(trees.own, id)
		}
	}
	return trees
}

// removeWorktrees removes the worktrees of a Delete's family, trees, once every one has passed every check of a
// clean-up, so a worktree kept refuses the Delete before any goes; each that goes takes the family's shells in it.
// A worktree kept because a shell or a thread outside the family works in it refuses the Delete with no check run; one
// a fork takes over stays for the fork. When the thread runs and works in a worktree that is to go, its own or a
// descendant's, the Delete puts it to sleep after the checks and runs them again, since its agent may have changed the
// worktree until it stopped; a refusal then says it is asleep. The family and its shells do not count as others in a
// worktree. asleep is the thread it put to sleep, "" when none.
func (s *Sessions) removeWorktrees(family []string, trees familyTrees) (asleep string, err error) {
	ctx := s.context()
	top := family[len(family)-1]
	switch {
	case trees.topKept != "":
		return "", fmt.Errorf("Thread %d keeps its worktree: %s. Delete it without the worktree", s.Number(top), trees.topKept)
	case len(trees.kept) > 0:
		return "", s.keptBy(trees.kept[0].id, trees.kept[0].reason, "", trees.count())
	}
	own := trees.own
	if err := s.checkWorktrees(ctx, family, own, "", trees.count()); err != nil {
		return "", err
	}
	if s.isLive(top) && slices.ContainsFunc(own, func(id string) bool { return id == top || s.worksIn(top, s.madeWorktree(id)) }) {
		name, _ := session.ParseName(top)
		if err := s.settleToDelete(name); err != nil {
			return "", err
		}
		asleep = top
		if err := s.checkWorktrees(ctx, family, own, asleep, trees.count()); err != nil {
			return asleep, err
		}
	}
	for _, id := range own {
		_, reason, err := s.life.cleanupNow(ctx, id, false, byDelete, family...)
		switch {
		case err != nil:
			return asleep, s.asleepNow(err, asleep)
		case reason != "":
			return asleep, s.keptBy(id, reason, asleep, trees.count())
		}
	}
	return asleep, nil
}

// checkWorktrees runs the checks of a clean-up for each thread of own, and refuses the Delete of family and its
// worktrees worktrees with the first worktree one keeps. asleep is the thread the Delete has put to sleep, "" before it
// has.
func (s *Sessions) checkWorktrees(ctx context.Context, family, own []string, asleep string, worktrees int) error {
	for _, id := range own {
		reason, err := s.life.checkCleanup(ctx, id, family...)
		switch {
		case err != nil:
			return s.asleepNow(err, asleep)
		case reason != "":
			return s.keptBy(id, reason, asleep, worktrees)
		}
	}
	return nil
}

// handOverWorktree gives the worktree orchestrator made for the thread id of a Delete's family to heir, the fork outside the
// family that works in it, so the record does not go with id and the worktree is still orchestrator's to remove. The heir was
// found before the Delete's checks, so it gets the worktree only while it is still a fork, live or asleep, with no
// worktree orchestrator made for it.
func (s *Sessions) handOverWorktree(id, heir string) {
	key := session.ProjectKey(id)
	w, ok := s.made.Of(key, id)
	if !ok || w.Removed != 0 || gone(w.Path) {
		return
	}
	s.mu.Lock()
	in, live := s.liveInfo(heir)
	e, asleep := s.ended[heir]
	if asleep {
		in = e.info
	}
	going := s.deleting[heir] || s.deleted[heir]
	s.mu.Unlock()
	if _, made := s.made.Of(key, heir); !live && !asleep || going || in.Fork == "" || made {
		return
	}
	if err := s.life.handOver(s.context(), key, w.Path, id, heir); err != nil {
		log.Printf("orchestrator: giving the worktree %s of %s to %s: %v", w.Path, id, heir, err)
	}
}

// keptBy is the refusal of a Delete of worktrees worktrees whose clean-up of the thread id kept its worktree for reason.
// asleep is the thread the Delete has put to sleep, "" when none.
func (s *Sessions) keptBy(id, reason, asleep string, worktrees int) error {
	n := s.Number(id)
	keep := "the worktree"
	if worktrees > 1 {
		keep = "the worktrees"
	}
	untick := fmt.Sprintf("Untick the box to keep %s, or clean Thread %d up first", keep, n)
	switch asleep {
	case "":
		return fmt.Errorf("Thread %d keeps its worktree: %s. %s", n, reason, untick)
	case id:
		return fmt.Errorf("Thread %d keeps its worktree: %s. It is asleep now. %s", n, reason, untick)
	}
	return fmt.Errorf("Thread %d keeps its worktree: %s. Thread %d is asleep now. %s", n, reason, s.Number(asleep), untick)
}

// asleepNow adds to err, from a Delete that failed after it put the thread asleep to sleep, that the thread is asleep
// now. With asleep "" it is err.
func (s *Sessions) asleepNow(err error, asleep string) error {
	if asleep == "" {
		return err
	}
	return fmt.Errorf("%w. Thread %d is asleep now", err, s.Number(asleep))
}

// isLive says whether the thread id runs under a host. It takes mu.
func (s *Sessions) isLive(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, live := s.liveInfo(id)
	return live
}

// DeletePlan is what a Delete of a thread would take with it, for its confirm.
type DeletePlan struct {
	Worktrees int      `json:"worktrees"` // the worktrees orchestrator made for the thread and its descendants that are on disk and that the box would take, as a Delete counts them
	Children  []string `json:"children"`  // the titles of the asleep descendants that go with the thread, nearest first
}

// DeletePlan is what a Delete of the thread id would take with it.
func (s *Sessions) DeletePlan(id string) DeletePlan {
	s.mu.Lock()
	family := s.familyOf(id)
	s.mu.Unlock()
	p := DeletePlan{Children: []string{}, Worktrees: s.familyWorktrees(s.context(), family).count()}
	for _, d := range family[:len(family)-1] {
		if !s.isLive(d) {
			p.Children = append(p.Children, s.titleOf(d))
		}
	}
	slices.Reverse(p.Children)
	return p
}

// OwnWorktree is the worktree orchestrator made for the thread id that only it uses, which a Delete may take with it; ""
// when there is none.
func (s *Sessions) OwnWorktree(id string) string {
	path, shared := s.ownWorktree(s.context(), id)
	if shared != "" {
		return ""
	}
	return path
}

// ownWorktree is the worktree orchestrator made for the thread id that is on disk, "" when there is none or the thread's
// clean-up finished, and why it is not the thread's alone, "" when it is: another thread, running or asleep, works in
// it, as a fork does, or a shell is open in it. Its descendants and the shells of the thread and its descendants do not
// count, as a Delete takes them with it.
func (s *Sessions) ownWorktree(ctx context.Context, id string) (path, shared string) {
	path = s.madeWorktree(id)
	if path == "" {
		return "", ""
	}
	family := append(s.Descendants(id), id)
	if user := s.usersOf(path, true, family...); user != "" {
		return path, fmt.Sprintf("Thread %s also works in this worktree", user)
	}
	return path, s.life.shellIn(ctx, session.ProjectKey(id), path, family...)
}

// madeWorktree is the worktree orchestrator made for the thread id that is on disk, "" when there is none or the thread's
// clean-up finished.
func (s *Sessions) madeWorktree(id string) string {
	w, ok := s.made.Of(session.ProjectKey(id), id)
	if !ok || w.Removed != 0 || gone(w.Path) || s.life.cleanedUp(id) {
		return ""
	}
	return w.Path
}

// take folds a record read from the state file in, unless the app holds a later one. It needs mu.
func (s *Sessions) take(rec session.Record) {
	if _, ended := s.ended[rec.Session]; ended || s.deleted[rec.Session] {
		return
	}
	if prev, ok := s.records[rec.Session]; ok && !rec.At.After(prev.At) {
		return
	}
	s.records[rec.Session] = rec
	s.follow(rec, endsTurn(rec.Event), rec.Event == "PostToolUse")
}

// follow does what a record that is the thread's latest calls for: the branch, the PR and the usage. It needs mu.
func (s *Sessions) follow(rec session.Record, turnEnded, toolDone bool) {
	s.followBranch(rec, turnEnded, toolDone)
	// Each event moves the row's cost. The event log gets a turn's usage once the turn is over, and the
	// first refresh's usage at once: it catches up on turns while the app was closed.
	if rec.Transcript != "" {
		if turnEnded {
			time.AfterFunc(stopSettle, func() { s.countUsage(rec.Session, rec.Transcript, rec.AgentModel, true) })
		} else {
			go s.countUsage(rec.Session, rec.Transcript, rec.AgentModel, !s.loaded)
		}
	}
}

// endsTurn says whether a record's event ends the agent's turn: its Stop, or its TurnError.
func endsTurn(event string) bool { return event == "Stop" || event == session.TurnError }

// followBranch has the lifecycle look up the PR of the thread's branch when its turn ends, since the agent may just
// have opened it, and read the branch's git status again after each tool call. It needs mu.
func (s *Sessions) followBranch(rec session.Record, turnEnded, toolDone bool) {
	if rec.Cwd != "" {
		s.cwds[rec.Session] = rec.Cwd
	}
	for _, in := range s.info {
		if in.Name.String() != rec.Session || in.Fork != "" || in.Branch == "" {
			continue
		}
		proj, ok := s.projectOf(in)
		if !ok {
			continue
		}
		if turnEnded {
			go s.life.track(s.ctx, rec.Session, proj, in.Branch)
		}
		if toolDone {
			go s.life.restatus(s.ctx, rec.Session, proj)
		}
	}
}

// sync updates history and attention from what is known, and tells the front
// end if the list changed. It needs mu.
func (s *Sessions) sync() {
	var attention [][2]string // session id and the attention state
	waitsChanged := false
	live := map[string]bool{}
	for _, in := range s.info {
		id := in.Name.String()
		live[id] = true
		s.known[id] = in
		state, at := s.stateOf(id), time.Now()
		if rec, ok := s.records[id]; ok {
			at = rec.At
		}
		if b, ok := s.background[id]; ok && b.At.After(at) {
			at = b.At
		}
		prev, known := s.seen[id]
		if !known {
			if s.synced {
				s.recordStart(id, in)
			}
			created, _ := s.startsOf(id, in)
			s.adoptHistory(id, created)
			s.saveIdentity(in)
		}
		opened := s.loaded && state == session.Idle && s.life.opened(id)
		if opened {
			attention = append(attention, [2]string{id, "opened"})
		}
		if known && prev == state {
			continue
		}
		s.seen[id] = state
		if known && prev == session.Working {
			s.worked(id, at)
		}
		if h := s.history[id]; len(h) == 0 || h[len(h)-1].State != state {
			s.history[id] = session.AddChange(h, session.Change{State: state, At: at.UnixMilli()})
		}
		rec := s.records[id]
		// A child's reply goes to its parent, which reads it; the owner hears of the parent's.
		replied := known && prev == session.Working && state == session.Idle && rec.Event == "Stop" && in.Parent == ""
		if s.loaded {
			switch {
			case state == session.Waiting:
				attention = append(attention, [2]string{id, "waiting"})
			case replied && !opened:
				attention = append(attention, [2]string{id, "replied"})
			}
		}
		if s.loaded {
			ended := s.tally.Closed(id, at)
			opens := state == session.Waiting
			if opens {
				issue, _ := strconv.Atoi(in.Issue)
				s.tally.Opened(id, cmp.Or(in.Title, defaultTitle), issue, in.Model, rec, at)
			}
			if ended || opens {
				waitsChanged = true
			}
		}
	}
	for id := range s.seen {
		if !live[id] {
			if s.tally.Closed(id, time.Now()) {
				waitsChanged = true
			}
			if s.seen[id] == session.Working {
				s.worked(id, time.Now())
			}
			if in := s.known[id]; !in.Created.IsZero() {
				s.record(id, activity.Event{At: time.Now().UnixMilli(), Kind: activity.SessionEnd, Ms: time.Since(in.Created).Milliseconds()})
			}
			if rec := s.records[id]; rec.Transcript != "" {
				go s.countUsage(id, rec.Transcript, rec.AgentModel, true)
			}
			delete(s.background, id)
			go s.closeBrowser(id)
			if !s.restarting[id] {
				go s.stopServers(true, id)
				go s.heardSleep(id)
				if s.known[id].Parent != "" {
					s.life.inBackground(s.ctx, func(ctx context.Context) { s.life.childAsleep(ctx, id) })
				}
			}
			s.endSession(id, time.Now())
			delete(s.seen, id)
			delete(s.records, id)
		}
	}
	if waitsChanged {
		s.emit("stats", nil)
	}
	s.endTurns()
	s.deliverWakes()
	s.holdAwake()

	s.announceProjects()
	list := s.list()
	s.announceIssues(list)
	byProject := map[string][]Session{}
	for _, v := range list {
		byProject[v.Project] = append(byProject[v.Project], v)
	}
	changed := false
	for _, k := range slices.Sorted(maps.Keys(byProject)) {
		if !reflect.DeepEqual(byProject[k], s.lastSent[k]) {
			s.emit("sessions", scoped.Of(k, byProject[k]))
			changed = true
		}
	}
	for _, k := range slices.Sorted(maps.Keys(s.lastSent)) {
		if _, ok := byProject[k]; !ok {
			s.emit("sessions", scoped.Of(k, []Session{}))
			changed = true
		}
	}
	if !changed {
		return
	}
	s.lastSent = byProject
	for _, a := range attention {
		s.emit("attention", map[string]string{"id": a[0], "state": a[1]})
	}
}

// recordStart records that the thread started, labelled resumed when a wake started it again. It needs mu.
func (s *Sessions) recordStart(id string, in term.Info) {
	e := activity.Event{At: time.Now().UnixMilli(), Kind: activity.SessionStart, Model: in.Model, Effort: in.Effort}
	e.Issue, _ = strconv.Atoi(in.Issue)
	if rec := s.records[id]; rec.Created > 0 && rec.Woke > rec.Created {
		e.Label = activity.Resumed
	}
	s.record(id, e)
}

// adoptHistory takes the timeline the state file kept for a session first seen, and records the working
// stretches in it that ended while the app was closed. It needs mu.
func (s *Sessions) adoptHistory(id string, created time.Time) {
	changes := s.records[id].Changes
	if len(changes) == 0 || len(s.history[id]) > 0 {
		return
	}
	s.history[id] = slices.Clone(changes)
	if len(changes) > 1 {
		go s.catchUpWorked(id, created, changes)
	}
}

// catchUpWorked records the working stretches of changes that end after the latest one the event
// log holds for the session: the app records a stretch at the change that ends it, so the ones
// before that were counted by a run of the app that saw them.
func (s *Sessions) catchUpWorked(id string, created time.Time, changes []session.Change) {
	last := changes[len(changes)-1].At
	var counted int64
	s.activity.Since(session.ProjectKey(id), created.In(time.Local).Format(time.DateOnly), func(e activity.Event) {
		if e.Kind == activity.Worked && e.Session == id && e.At <= last { // a later one is this run's
			counted = max(counted, e.At)
		}
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range changes[:len(changes)-1] {
		if end := changes[i+1].At; c.State == session.Working && end > counted {
			s.record(id, activity.Event{At: end, Kind: activity.Worked, Ms: end - c.At})
		}
	}
}

// worked counts the time since the session last changed state as time it spent working. It needs mu.
func (s *Sessions) worked(id string, end time.Time) {
	if h := s.history[id]; len(h) > 0 {
		if ms := end.UnixMilli() - h[len(h)-1].At; ms > 0 {
			s.record(id, activity.Event{At: end.UnixMilli(), Kind: activity.Worked, Ms: ms})
		}
	}
}

// record adds an event of the session to the activity record.
func (s *Sessions) record(id string, e activity.Event) {
	e.Session = id
	s.activity.Record(session.ProjectKey(id), e)
}

// holdAwake keeps the Mac awake while a session of a project that keeps awake is working. It needs mu.
func (s *Sessions) holdAwake() {
	if s.awake == nil {
		return
	}
	working := slices.ContainsFunc(s.info, func(in term.Info) bool {
		return s.stateOf(in.Name.String()) == session.Working && s.keepAwake(in.Name.Project)
	})
	s.awake.Hold(working)
}

// announceProjects tells the front end when a project appears, goes, or its counts move. It needs mu.
func (s *Sessions) announceProjects() {
	views := s.projectViews()
	if reflect.DeepEqual(views, s.lastProjs) {
		return
	}
	s.lastProjs = views
	s.emit("projects", views)
}

// announceIssues asks for a fresh issues event when the sessions tied to issues change. It needs mu.
func (s *Sessions) announceIssues(list []Session) {
	var sig strings.Builder
	sig.WriteString(s.project.Key())
	for _, v := range list {
		if v.Project == s.project.Key() && v.Issue > 0 && v.State != session.Settled && v.ForkOf == "" {
			fmt.Fprintf(&sig, " %d=%s", v.Issue, v.ID)
		}
	}
	if sig.String() == s.lastIssues {
		return
	}
	first := s.lastIssues == ""
	s.lastIssues = sig.String()
	if !first {
		go s.onIssues()
	}
}

// projectViews lists every known project with its session counts. It needs mu.
func (s *Sessions) projectViews() []Project {
	var views []Project
	for _, p := range s.projects() {
		v := Project{Key: p.Key(), Name: p.Name, Dir: p.Dir, Repo: s.repoOf(p.Dir), NeedsSetup: p.NeedsSetup(), SetupDismissed: p.SetupDismissed}
		for _, in := range s.info {
			if in.Name.Project != p.Key() {
				continue
			}
			switch s.stateOf(in.Name.String()) {
			case session.Settled:
				continue
			case session.Waiting:
				v.NeedsYou++
			case session.Working:
				v.Working++
			}
			v.Sessions++
		}
		views = append(views, v)
	}
	return views
}

// list is every project's sessions, each project's in rail order. It needs mu.
func (s *Sessions) list() []Session {
	var rail []session.Session
	byName := map[session.Name]term.Info{}
	for _, in := range s.info {
		byName[in.Name] = in
		ss := session.Session{Name: in.Name, N: s.nums[in.Name.String()], State: s.stateOf(in.Name.String())}
		if rec, ok := s.records[in.Name.String()]; ok {
			ss.At, ss.Detail = rec.At, rec.Detail
		}
		rail = append(rail, ss)
	}
	for _, e := range s.ended {
		byName[e.info.Name] = e.info
		rail = append(rail, session.Session{Name: e.info.Name, N: s.nums[e.info.Name.String()], State: session.Settled, At: e.at})
	}
	session.Sort(rail)
	out := make([]Session, 0, len(rail))
	for _, ss := range rail {
		in := byName[ss.Name]
		id := ss.Name.String()
		issue, _ := strconv.Atoi(in.Issue)
		created, woke := s.startsOf(id, in)
		view := Session{
			ID: id, Project: ss.Name.Project, N: ss.N, Title: cmp.Or(in.Title, defaultTitle),
			State: ss.State, Detail: ss.Detail,
			CreatedAt: created.UnixMilli(), WokeAt: woke.UnixMilli(), Issue: issue, Access: in.Access, Action: in.Action, ForkOf: in.Fork, Parent: in.Parent, Model: in.Model, Effort: in.Effort, Evidence: s.evidence.Count(id), Browser: s.browsers.Has(id),
			ForkAt:  s.forkAtOf(id),
			History: s.history[id], // shared, not copied: a change in a history is never written over
		}
		if !ss.At.IsZero() {
			view.LastEventAt = ss.At.UnixMilli()
		}
		if ss.State == session.Settled {
			view.SettledAt = ss.At.UnixMilli()
		}
		f := s.life.fields(id)
		view.Branch, view.PR, view.Git = in.Branch, f.pr, f.git
		if w, ok := s.made.Of(ss.Name.Project, id); ok && w.Removed == 0 {
			view.Worktree = w.Path
		}
		view.PRAttention, view.Cleanup, view.CleanupReason = f.attention, f.cleanup, f.reason
		if spent, ok := s.spent[id]; ok {
			view.Tokens, view.Cost = spent.Total(), spent.Dollars()
		}
		view.Memory = s.held[id]
		view.Subagents = s.background[id].Subagents
		if view.History == nil {
			view.History = []session.Change{}
		}
		out = append(out, view)
	}
	return out
}

func (s *Sessions) List() []Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list()
}

// ListOf is the sessions of one project, in rail order.
func (s *Sessions) ListOf(key string) []Session {
	return slices.DeleteFunc(s.List(), func(v Session) bool { return v.Project != key })
}

// Title is the title of the session with the given id, live or asleep, or "" when there is none.
func (s *Sessions) Title(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range s.info {
		if in.Name.String() == id {
			return cmp.Or(in.Title, defaultTitle)
		}
	}
	if e := s.ended[id]; e != nil {
		return cmp.Or(e.info.Title, defaultTitle)
	}
	return ""
}

// Number is the number the app shows for the session with the given id, or 0 when there is none.
func (s *Sessions) Number(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nums[id]
}

// Current is the current project as the config holds it now, so a setting saved from the panel applies at once.
func (s *Sessions) Current() project.Project {
	s.mu.Lock()
	defer s.mu.Unlock()
	configured := s.configured.List()
	if i := slices.IndexFunc(configured, func(p project.Project) bool { return p.Key() == s.project.Key() }); i >= 0 {
		return configured[i]
	}
	return s.project
}

// Projects lists every known project: the configured ones, the current one, and any other with live sessions.
func (s *Sessions) Projects() []Project {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projectViews()
}

func (s *Sessions) projects() []project.Project {
	out := s.configured.List()
	if !slices.ContainsFunc(out, func(p project.Project) bool { return p.Key() == s.project.Key() }) {
		out = append(out, s.project)
	}
	for _, in := range s.info {
		if !slices.ContainsFunc(out, func(p project.Project) bool { return p.Key() == in.Name.Project }) {
			out = append(out, project.Project{Name: in.Name.Project, Dir: in.Path})
		}
	}
	return out
}

// SwitchProject makes the named project current and returns it.
func (s *Sessions) SwitchProject(name string) (project.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.projects() {
		if strings.EqualFold(p.Name, name) || p.Key() == name {
			s.project = p
			s.sync()
			return p, nil
		}
	}
	return project.Project{}, fmt.Errorf("no project %q", name)
}

var errAskerNotLive = errors.New("Only a running thread can start an issue's thread")

var errFullFromThread = errors.New("A thread that is not Full access cannot start one that is")

// liveAccessOf is the access of the running thread id, and whether it runs. It takes mu.
func (s *Sessions) liveAccessOf(id string) (session.Access, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range s.info {
		if in.Name.String() == id {
			return in.Access, true
		}
	}
	return "", false
}

// mayGive refuses a thread the asker starts or wakes Full access unless the asker is a live Full thread, so a thread
// cannot give itself every tool without the owner. An asker that names no live thread is not Full. It takes mu.
func (s *Sessions) mayGive(asker string, access session.Access) error {
	if asker == "" || access != session.Full {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range s.info {
		if in.Name.String() == asker && in.Access == session.Full {
			return nil
		}
	}
	return errFullFromThread
}

// liveIssueSession is the id of the running thread of the project for the issue, whatever its access, "" when none. A
// fork only shows its source's issue. It takes mu.
func (s *Sessions) liveIssueSession(key string, issue int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range s.info {
		if in.Name.Project == key && in.Issue == strconv.Itoa(issue) && in.Fork == "" {
			return in.Name.String()
		}
	}
	return ""
}

// threadOn is the number of a thread of the project other than except that works on branch, live or settled, and
// whether there is one. A deleted thread has let its branch go. It takes mu.
func (s *Sessions) threadOn(key, branch, except string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	infos := slices.Clone(s.info)
	for _, e := range s.ended {
		infos = append(infos, e.info)
	}
	for _, in := range infos {
		if id := in.Name.String(); in.Name.Project == key && in.Branch == branch && id != except {
			return s.nums[id], true
		}
	}
	return 0, false
}

// threadIn is the number of a thread of the project other than except whose folder, or the worktree orchestrator made for
// it, is dir, live or settled, and whether there is one. A fork and a child with no worktree run in a folder that is
// not theirs alone and are left out. It takes mu.
func (s *Sessions) threadIn(key, dir, except string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	infos := slices.Clone(s.info)
	for _, e := range s.ended {
		infos = append(infos, e.info)
	}
	for _, in := range infos {
		id := in.Name.String()
		if in.Name.Project != key || inOthersFolder(in) || id == except {
			continue
		}
		if in.Path != "" && samePath(in.Path, dir) {
			return s.nums[id], true
		}
		if w, ok := s.made.Of(key, id); ok && w.Removed == 0 && samePath(w.Path, dir) {
			return s.nums[id], true
		}
	}
	return 0, false
}

// IssueSessions maps issue numbers to the live thread started for each, whatever its access, in the current project.
func (s *Sessions) IssueSessions() map[int]string {
	out := map[int]string{}
	for _, v := range s.ListOf(s.Current().Key()) {
		if v.Issue > 0 && v.State != session.Settled && v.ForkOf == "" {
			out[v.Issue] = v.ID
		}
	}
	return out
}

// ProjectByKey is the project with that key among the known ones: the configured, the current and those with live threads.
func (s *Sessions) ProjectByKey(key string) (project.Project, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.projects(), func(p project.Project) bool { return p.Key() == key })
	if i < 0 {
		return project.Project{}, false
	}
	return s.projects()[i], true
}

// Create starts the agent in proj with the access given. A non-empty text is given to the agent through its host: sent
// when send is set, else left as a draft in its chat.
func (s *Sessions) Create(proj project.Project, title, text string, send bool, issue int, access session.Access) (Session, error) {
	return s.createIn(proj, title, text, send, issue, "", "", "", forkStart{}, StartOptions{Access: access, Place: placeFor(s.placeSetting(), issue, "")})
}

// placeSetting is the config's new_thread_place, "" when it is unset.
func (s *Sessions) placeSetting() Place {
	if s.newPlace == nil {
		return ""
	}
	return Place(s.newPlace())
}

// StartOptions is what the owner chose for a new thread: the first message, sent as it is, the model and effort,
// which win over every other choice, the access and where it runs. Empty fields leave the choice to the defaults; an
// empty access is Supervised, and an empty place is placeFor's. Asker is the thread that asks for this one, "" when the
// owner asks from the app, the shell or a terminal. Parent is the thread that delegated to this one, "" for a thread
// the owner started, and Base the commit a new branch in its worktree starts at, "" for the default branch on origin.
type StartOptions struct {
	Prompt string
	Model  project.Model
	Access session.Access
	Place  Place
	Asker  string
	Parent string
	Base   string

	dir string // the folder a thread in the project place runs in, "" for the project folder
}

const titleLength = 60

// titleFromPrompt is the first line of the prompt, cut to the length of a title.
func titleFromPrompt(prompt string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(prompt), "\n")
	title := []rune(strings.TrimSpace(first))
	if len(title) > titleLength {
		title = title[:titleLength]
	}
	return strings.TrimSpace(string(title))
}

// CreateThread starts a thread in the current project as Create does, with the owner's first message sent through the
// same path and the owner's model, effort and access. A blank title is the first line of the prompt.
func (s *Sessions) CreateThread(title string, o StartOptions) (Session, error) {
	p := s.Provider()
	if err := p.ValidModel(o.Model.Model, o.Model.Effort); err != nil {
		return Session{}, err
	}
	access, err := session.ParseAccess(string(o.Access))
	if err != nil {
		return Session{}, err
	}
	place, err := parsePlace(string(o.Place))
	if err != nil {
		return Session{}, err
	}
	title = cmp.Or(strings.TrimSpace(title), titleFromPrompt(o.Prompt))
	return s.createIn(s.Current(), title, strings.TrimSpace(o.Prompt), true, 0, "", "", "", forkStart{}, StartOptions{Model: o.Model, Access: access, Place: placeFor(s.placeSetting(), 0, place)})
}

// CreateBriefed is Create for a Supervised session with a job of its own, in the project folder: brief is added to the
// agent's system prompt, and the project says whether text is sent.
func (s *Sessions) CreateBriefed(title, text, brief string) (Session, error) {
	proj := s.Current()
	return s.createIn(proj, title, FromApp(text), proj.SendsPrompt(), 0, brief, "", "", forkStart{}, StartOptions{Access: session.Supervised, Place: InProject})
}

// IssueStart is what a start of an issue did.
type IssueStart int

const (
	Kept    IssueStart = iota // gave back the issue's live thread, with nothing given to it
	Woken                     // woke the issue's settled thread and gave it the command and the handover
	Created                   // started a new thread
)

// CreateIssue starts the agent for an issue, with the access the action says, and gives it text; the project says
// whether that is sent. The labels and the action pick the model it takes. A handover from the session that asked for
// this one is added to the agent's system prompt. An issue has one thread: one that has a live thread gets it back with
// nothing given to it; one whose thread has settled gets that thread woken, with the handover in place of the one it
// had when one is given, and given text. The owner's prompt follows the action's command in the same first message,
// and the owner's model and effort win over the labels' and the action's; a thread that is live or woken keeps its
// own, and its access.
func (s *Sessions) CreateIssue(proj project.Project, title, text string, issue int, action project.Action, labels []string, handover string, o StartOptions) (view Session, did IssueStart, err error) {
	p := s.Provider()
	if err := p.ValidModel(o.Model.Model, o.Model.Effort); err != nil {
		return Session{}, Kept, err
	}
	model, _ := project.Pick(action, labels, p.ValidModel)
	model = project.Model{Model: cmp.Or(o.Model.Model, model.Model), Effort: cmp.Or(o.Model.Effort, model.Effort)}
	text = FromApp(text)
	if extra := strings.TrimSpace(o.Prompt); extra != "" {
		text = strings.TrimSpace(text + " " + extra)
	}
	place, err := parsePlace(string(o.Place))
	if err != nil {
		return Session{}, Kept, err
	}
	access := action.ThreadAccess()
	if o.Asker != "" {
		asker, live := s.liveAccessOf(o.Asker)
		if !live {
			return Session{}, Kept, errAskerNotLive
		}
		if asker = cmp.Or(asker, session.Read); asker.Rank() < access.Rank() {
			access = asker
		}
	}
	view, did, err = s.createForIssue(proj, title, text, proj.SendsPrompt(), issue, model, handover, access, action.Name, o.Asker, place)
	if err != nil || did == Created {
		return view, did, err
	}
	view, err = s.row(view.ID)
	return view, did, err
}

// row is the thread's row.
func (s *Sessions) row(id string) (Session, error) {
	for _, v := range s.List() {
		if v.ID == id {
			return v, nil
		}
	}
	return Session{}, fmt.Errorf("Thread %s is gone", id)
}

// Wake starts the settled thread id again under its own id: in the folder it started in, where Claude finds its
// conversation, under a host that goes on with the same chat log at the same socket, and with Claude resuming the
// conversation when the thread has one. The thread keeps its row, number, state file, history, handover and usage, and
// starts with its model, effort, browser and fork tag. It returns the thread's chat once its host listens.
func (s *Sessions) Wake(id string) (Chat, error) { return s.wakeWith(id, "") }

// wakeWith is Wake with a handover that takes the place of the one the thread had, unless it is "".
func (s *Sessions) wakeWith(id, handover string) (Chat, error) {
	name, err := session.ParseName(id)
	if err != nil {
		return Chat{}, err
	}
	if err := s.startAgain(name, handover); err != nil {
		return Chat{}, err
	}
	ctx, cancel := context.WithTimeout(s.context(), hostWait)
	err = host.Ready(ctx, s.hostSocket(name))
	cancel()
	if err != nil {
		log.Printf("orchestrator: the host of thread %s did not listen: %v", id, err)
		return Chat{}, fmt.Errorf("Thread %s did not start", id)
	}
	return s.ChatOf(id)
}

// wakeOrJoin wakes the settled thread id as wakeWith does, or gives its chat when another wake started it first.
func (s *Sessions) wakeOrJoin(id, handover string) (Chat, error) {
	ch, err := s.wakeWith(id, handover)
	if err == nil {
		return ch, nil
	}
	if live, again := s.ChatOf(id); again == nil && live.Live {
		return live, nil
	}
	return Chat{}, err
}

// startAgain starts the tmux session of the settled thread again and takes the thread back as live. Its state file says
// it runs before Claude starts, so the first event builds on it; a launch that fails writes back the settled one.
func (s *Sessions) startAgain(name session.Name, handover string) error {
	id := name.String()
	s.createMu.Lock()
	defer s.createMu.Unlock()

	// The mark goes on before the look for a running clean-up, as a clean-up marks itself before it looks for a wake:
	// one of the two always sees the other. It stays until the thread runs, so no clean-up starts while the worktree is
	// restored and Claude starts in it.
	s.mu.Lock()
	s.waking[id] = true
	defer func() {
		s.mu.Lock()
		delete(s.waking, id)
		s.mu.Unlock()
	}()
	e, settled := s.ended[id]
	var in term.Info
	if settled {
		in = e.info
	}
	deleted, deleting := s.deleted[id], s.deleting[id]
	proj, known := s.projectOf(in)
	conversation := s.conversationOf(id)
	source, _ := s.infoOf(in.Fork)
	n := s.nums[id]
	ctx := s.ctx
	s.mu.Unlock()
	switch {
	case deleted:
		return fmt.Errorf("Thread %s was deleted", id)
	case deleting:
		return fmt.Errorf("Thread %d is being deleted", n)
	case !settled:
		return fmt.Errorf("Thread %s is not asleep", id)
	case !known:
		return fmt.Errorf("The project of thread %s is gone", id)
	case s.life.CleaningUp(id):
		return fmt.Errorf("Thread %d is being cleaned up; wake it when that ends", n)
	}

	p := s.Provider()
	if err := agent.CheckInstalled(p); err != nil {
		return err
	}
	if err := s.life.restore(ctx, proj, id); err != nil {
		return err
	}
	dir := cmp.Or(in.Path, proj.Dir)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("%s is gone, so this thread cannot start there", dir)
	}
	model := fill(p, proj.Dir, project.Model{Model: in.Model, Effort: in.Effort})
	in.Created, in.Model, in.Effort = time.Now(), model.Model, model.Effort
	s.mu.Lock()
	rec, before := s.woken(id, in), e.record()
	s.mu.Unlock()
	if err := bus.WriteState(s.stateDir, rec); err != nil {
		return fmt.Errorf("saving the thread's state: %w", err)
	}
	issue, _ := strconv.Atoi(in.Issue)
	tctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	fork := forkStart{of: in.Fork, title: cmp.Or(source.Title, defaultTitle)}
	if err := s.launch(tctx, p, name, cmp.Or(in.Title, defaultTitle), proj, dir, in.Branch, issue, model, conversation, "", cmp.Or(handover, s.handoverOf(id)), in.Access, in.Action, fork, in.Parent); err != nil {
		_ = bus.WriteState(s.stateDir, before)
		return err
	}
	s.life.unretire(id)
	s.mu.Lock()
	delete(s.ended, id)
	s.records[id] = rec
	s.info = append(s.info, in)
	s.sync()
	s.mu.Unlock()
	if in.Branch != "" {
		s.life.ensureWatchers(ctx)
	}
	return nil
}

// Delegation stops here, so it cannot run away: a thread three delegations from one the owner started, a grandchild's
// child, cannot delegate, and a thread has at most eight children running at once.
const (
	maxDepth        = 3
	maxLiveChildren = 8
)

// Where a child works. In another project than its parent's, the folder is that project's, and a new branch starts at
// that project's default branch on origin.
const (
	WhereFolder   = "folder"   // its parent's folder, on no branch of its own
	WhereWorktree = "worktree" // a new worktree, on a new branch that starts at the commit its parent's folder is at
	WhereIssue    = "issue"    // the worktree of an issue, on the issue's branch
)

// ChildOptions is what a thread asks for when it delegates a task: the child's title, "" for the task's first line,
// its access and model, its project, a configured project's name or key or "" for the parent's, and where it works in
// that project, with the issue's number for WhereIssue.
type ChildOptions struct {
	Parent, Task, Title string
	Access              session.Access
	Model               project.Model
	Project             string
	Where               string
	Issue               int
}

// CreateChild starts a thread for the task the live thread o.Parent delegates, and gives it the task as its first
// message before it returns, so the child's chat holds the task once the parent has its id. A child never gets more
// access than its parent has. A parent too many delegations deep, or with too many children running, starts none, and
// so does a parent that asks for an issue that has a thread, or for its own folder while a clean-up of it runs, which
// may be removing that folder.
func (s *Sessions) CreateChild(ctx context.Context, o ChildOptions) (Session, error) {
	task := strings.TrimSpace(o.Task)
	if task == "" {
		return Session{}, errors.New("A child needs a task")
	}
	view, parentTitle, err := s.startChild(ctx, o)
	if err != nil {
		return Session{}, err
	}
	name, _ := session.ParseName(view.ID)
	if err := toHost(s.context(), s.hostSocket(name), FromApp(prompts.Delegated(parentTitle, task)), true, hostWait); err != nil {
		log.Printf("orchestrator: child %s did not take its task: %v", view.ID, err)
		return Session{}, fmt.Errorf("Thread %s started but did not take its task: send it again", view.ID)
	}
	return view, nil
}

// startChild checks that the parent may start the child o asks for, then starts it unless ctx has ended, and returns it
// and its parent's title. It holds childMu, so two delegations at once cannot both pass the limits, and the parent's
// access cannot change between the check and the start.
func (s *Sessions) startChild(ctx context.Context, o ChildOptions) (Session, string, error) {
	p := s.Provider()
	var err error
	if err := p.ValidModel(o.Model.Model, o.Model.Effort); err != nil {
		return Session{}, "", err
	}
	if o.Access == "" {
		return Session{}, "", errors.New("A child needs an access")
	}
	if _, err := session.ParseAccess(string(o.Access)); err != nil {
		return Session{}, "", err
	}
	s.childMu.Lock()
	defer s.childMu.Unlock()
	s.mu.Lock()
	parent, live := s.liveInfo(o.Parent)
	live = live && !s.deleted[o.Parent] && !s.deleting[o.Parent]
	depth, running := s.depthOf(o.Parent), 0
	for _, in := range s.info {
		if in.Parent == o.Parent {
			running++
		}
	}
	proj, known := s.projectOf(parent)
	s.mu.Unlock()
	switch {
	case !live:
		return Session{}, "", fmt.Errorf("Thread %s is not running, so it cannot delegate", o.Parent)
	case o.Access.Rank() > parent.Access.Rank():
		return Session{}, "", fmt.Errorf("A thread with %s access cannot start a child with %s access", parent.Access, o.Access)
	case depth >= maxDepth:
		return Session{}, "", fmt.Errorf("This thread is %d delegations from one the owner started, and a thread that deep cannot delegate", depth)
	case running >= maxLiveChildren:
		return Session{}, "", fmt.Errorf("This thread has %d children running, the most it may have: wait for one to finish or stop one", running)
	case o.Where != WhereIssue && o.Issue != 0:
		return Session{}, "", fmt.Errorf("An issue number goes with where %s", WhereIssue)
	}
	if !known {
		return Session{}, "", fmt.Errorf("The project of thread %s is gone", o.Parent)
	}
	home := proj
	if o.Project != "" {
		if proj, err = s.configuredProject(o.Project, home); err != nil {
			return Session{}, "", err
		}
	}
	own := proj.Key() == home.Key()
	start := StartOptions{Model: o.Model, Access: o.Access, Parent: o.Parent, Place: InWorktree}
	switch o.Where {
	case WhereFolder:
		start.Place = InProject
		if own {
			if s.life.CleaningUp(o.Parent) {
				return Session{}, "", fmt.Errorf("Thread %d is being cleaned up; start a child in its folder when that ends", s.Number(o.Parent))
			}
			start.dir = parent.Path
		}
	case WhereWorktree:
		if own {
			if start.Base, err = s.headOf(proj, parent.Path); err != nil {
				return Session{}, "", err
			}
		}
	case WhereIssue:
		if o.Issue <= 0 {
			return Session{}, "", fmt.Errorf("Where %s needs the issue's number", WhereIssue)
		}
	default:
		return Session{}, "", fmt.Errorf("where must be %s, %s or %s, not %q", WhereFolder, WhereWorktree, WhereIssue, o.Where)
	}
	title := cmp.Or(strings.TrimSpace(o.Title), titleFromPrompt(o.Task))
	parentTitle := cmp.Or(parent.Title, defaultTitle)
	if err := ctx.Err(); err != nil {
		return Session{}, "", fmt.Errorf("The call ended, so no child started: %w", err)
	}
	if o.Where == WhereIssue {
		s.issueMu.Lock()
		defer s.issueMu.Unlock()
		if err := ctx.Err(); err != nil {
			return Session{}, "", fmt.Errorf("The call ended, so no child started: %w", err)
		}
		thread := s.liveIssueSession(proj.Key(), o.Issue)
		if thread == "" {
			thread, _ = s.settledIssueThread(proj.Key(), o.Issue)
		}
		if thread != "" {
			return Session{}, "", fmt.Errorf("Issue #%d has thread %s: send to it instead", o.Issue, thread)
		}
	}
	view, err := s.createIn(proj, title, "", false, o.Issue, "", "", "", forkStart{}, start)
	if err == nil && !own {
		// The owner may not have that project on screen, so the log says who started work there.
		log.Printf("orchestrator: thread %s %q in project %s started child %s %q in project %s", o.Parent, parentTitle, home.Name, view.ID, title, proj.Name)
	}
	return view, parentTitle, err
}

// configuredProject is the project that name, a project's name or key, names: home, the parent's, or a configured
// one. A name never becomes a folder, so a child runs only where the owner's config points.
func (s *Sessions) configuredProject(name string, home project.Project) (project.Project, error) {
	names := func(p project.Project) bool { return strings.EqualFold(p.Name, name) || p.Key() == name }
	if names(home) {
		return home, nil
	}
	list := s.configured.List()
	if i := slices.IndexFunc(list, names); i >= 0 {
		return list[i], nil
	}
	var known []string
	for _, p := range list {
		known = append(known, p.Name)
	}
	if !slices.ContainsFunc(list, func(p project.Project) bool { return p.Key() == home.Key() }) {
		known = append(known, home.Name)
	}
	return project.Project{}, fmt.Errorf("No project %q: the projects are %s", name, strings.Join(known, ", "))
}

// headOf is the commit dir, a folder of the project, stands at; "" in a project that is not a git repository, whose
// threads run in its folder.
func (s *Sessions) headOf(proj project.Project, dir string) (string, error) {
	ctx := s.context()
	if _, err := s.life.git(ctx, proj.Dir, "rev-parse", "--git-dir"); err != nil {
		return "", nil
	}
	head, err := s.life.head(ctx, proj.Dir, dir, "")
	if err != nil {
		return "", fmt.Errorf("reading the commit %s is at: %w", dir, err)
	}
	return head, nil
}

// inOthersFolder says whether the thread runs in a folder that is not its own, with no branch of its own: a fork, or a
// child that delegate started with no worktree, in its parent's folder or another project's.
func inOthersFolder(in term.Info) bool { return in.Fork != "" || in.Parent != "" && in.Branch == "" }

// liveInfo is the live thread id's info, and whether it runs under a host. It needs mu.
func (s *Sessions) liveInfo(id string) (term.Info, bool) {
	i := slices.IndexFunc(s.info, func(in term.Info) bool { return in.Name.String() == id })
	if i < 0 {
		return term.Info{}, false
	}
	return s.info[i], true
}

// depthOf is how many delegations the thread is from one the owner started, counted up to one past maxDepth. It needs
// mu.
func (s *Sessions) depthOf(id string) int {
	depth := 0
	for parent := s.parentOf(id); parent != "" && depth <= maxDepth; parent = s.parentOf(parent) {
		depth++
	}
	return depth
}

// parentOf is the thread that delegated to the thread id, "" for one the owner started or one that is deleted. It
// needs mu.
func (s *Sessions) parentOf(id string) string {
	in, _ := s.infoOf(id)
	return in.Parent
}

// ParentOf is the thread that delegated to the thread id, "" for one the owner started. It takes mu.
func (s *Sessions) ParentOf(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parentOf(id)
}

// ChildrenOf are the threads that the thread id delegated to, running or settled, by number. A deleted child is left
// out. It takes mu.
func (s *Sessions) ChildrenOf(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.childrenOf(id)
}

// childrenOf needs mu.
func (s *Sessions) childrenOf(id string) []string {
	var out []string
	for _, in := range s.info {
		if in.Parent == id {
			out = append(out, in.Name.String())
		}
	}
	for child, e := range s.ended {
		if e.info.Parent == id {
			out = append(out, child)
		}
	}
	slices.SortFunc(out, func(a, b string) int { return cmp.Or(cmp.Compare(s.nums[a], s.nums[b]), cmp.Compare(a, b)) })
	return out
}

// Descendants are the thread's children, their children and so on, each once, nearest first. It takes mu.
func (s *Sessions) Descendants(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.descendants(id)
}

// descendants needs mu.
func (s *Sessions) descendants(id string) []string {
	seen := map[string]bool{id: true}
	var out []string
	for next := []string{id}; len(next) > 0; {
		var children []string
		for _, n := range next {
			for _, child := range s.childrenOf(n) {
				if !seen[child] {
					seen[child] = true
					children = append(children, child)
				}
			}
		}
		out = append(out, children...)
		next = children
	}
	return out
}

// AccessOf is the access of the thread id, running or settled; "" for none, which ranks as Read. It takes mu.
func (s *Sessions) AccessOf(id string) session.Access {
	s.mu.Lock()
	defer s.mu.Unlock()
	in, _ := s.infoOf(id)
	return in.Access
}

// titleOf is the title of the thread id, also of one whose row went, from its state file while it has one. It takes mu.
func (s *Sessions) titleOf(id string) string {
	s.mu.Lock()
	in, ok := s.infoOf(id)
	s.mu.Unlock()
	if ok {
		return cmp.Or(in.Title, defaultTitle)
	}
	rec, _ := bus.ReadState(s.stateDir, id)
	return cmp.Or(rec.Title, defaultTitle)
}

// Provider is the provider every session starts with.
func (s *Sessions) Provider() agent.Provider { return s.agents.Provider() }

// createForIssue starts a thread for the issue, in place or where placeFor says, unless the project has a live one for
// it, which it names, or a settled one, which it wakes with the handover and gives text to, and names.
func (s *Sessions) createForIssue(proj project.Project, title, text string, send bool, issue int, model project.Model, handover string, access session.Access, action, asker string, place Place) (Session, IssueStart, error) {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()
	if live := s.liveIssueSession(proj.Key(), issue); live != "" {
		return Session{ID: live}, Kept, nil
	}
	if settled, was := s.settledIssueThread(proj.Key(), issue); settled != "" {
		if err := s.mayGive(asker, was); err != nil {
			return Session{}, Kept, err
		}
		if asker != "" {
			from, _ := s.liveAccessOf(asker)
			if from = cmp.Or(from, session.Read); was.Rank() > from.Rank() {
				return Session{}, Kept, fmt.Errorf("A thread with %s access cannot wake a thread with %s access", from, was)
			}
		}
		ch, err := s.wakeOrJoin(settled, handover)
		if err != nil {
			return Session{}, Kept, err
		}
		if text != "" {
			if err := toHost(s.context(), ch.Socket, text, send, hostWait); err != nil {
				log.Printf("orchestrator: thread %s did not get the issue's command: %v", settled, err)
				return Session{}, Woken, fmt.Errorf("Thread %s woke but did not take the issue's command", settled)
			}
		}
		return Session{ID: settled}, Woken, nil
	}
	if err := s.mayGive(asker, access); err != nil {
		return Session{}, Kept, err
	}
	view, err := s.createIn(proj, title, text, send, issue, "", handover, action, forkStart{}, StartOptions{Model: model, Access: access, Place: placeFor(s.placeSetting(), issue, place)})
	return view, Created, err
}

// settledIssueThread is the id and access of the settled thread of the project for the issue,
// the one that settled last, "" when none. A fork only shows its source's issue. It takes mu.
func (s *Sessions) settledIssueThread(key string, issue int) (string, session.Access) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, access, at := "", session.Access(""), time.Time{}
	for name, e := range s.ended {
		in := e.info
		if in.Name.Project == key && in.Issue == strconv.Itoa(issue) && in.Fork == "" && (id == "" || e.at.After(at)) {
			id, access, at = name, in.Access, e.at
		}
	}
	return id, access
}

// AppPrompt starts each text the app writes to an agent but a slash command.
const AppPrompt = "APPLICATION_PROMPT: "

// FromApp marks text the app writes to the agent, so the agent does not take it for the owner's. A slash command
// stays as it is: Claude Code runs one only when it starts the prompt.
func FromApp(text string) string {
	if text == "" || strings.HasPrefix(text, "/") {
		return text
	}
	return AppPrompt + text
}

// createIn is Create in the project given, which need not be the current one, with the model, access, place and parent
// of o. A brief or a handover is added to the agent's system prompt. In a worktree the thread works on its issue's
// branch, or on a new one named from its title that starts at o.Base; else it runs in the project folder, or in the
// folder o names, on no branch.
func (s *Sessions) createIn(proj project.Project, title, text string, send bool, issue int, brief, handover, action string, fork forkStart, o StartOptions) (Session, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()

	p := s.Provider()
	var err error
	if err := agent.CheckInstalled(p); err != nil {
		return Session{}, err
	}
	model := fill(p, proj.Dir, o.Model)
	name, ctx := s.reserve(proj)
	work := workPlace{dir: cmp.Or(o.dir, proj.Dir)}
	if o.Place == InWorktree {
		branch, base := proj.BranchFor(issue), ""
		if issue == 0 {
			branch, base = s.life.freeBranch(ctx, proj.Dir, branchFromTitle(title)), o.Base
		}
		if work, err = s.life.makeWorktree(ctx, proj, name, branch, base); err != nil {
			return Session{}, err
		}
	}
	restore := func() {}
	if fork.at != "" {
		if restore, err = s.copyChat(fork, name); err != nil {
			s.life.undoWorktree(ctx, proj, name, work)
			return Session{}, err
		}
	}
	created := time.Now()
	title = cmp.Or(strings.TrimSpace(title), defaultTitle)
	tctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	if err := s.launch(tctx, p, name, title, proj, work.dir, work.branch, issue, model, fork.conversation, brief, handover, o.Access, action, fork, o.Parent); err != nil {
		restore()
		s.life.undoWorktree(ctx, proj, name, work)
		return Session{}, err
	}
	view := s.register(name, title, work, issue, model, created, o.Access, action, fork.of, fork.at, o.Parent)
	if text != "" {
		go s.giveHost(ctx, name, text, send)
	}
	if work.branch != "" {
		s.life.ensureWatchers(ctx)
	}
	return view, nil
}

// DefaultModel is the model and effort a new session in the current project gets from the agent's own settings
// when nothing else picks them; with model set, the effort is the one for that model.
func (s *Sessions) DefaultModel(model string) project.Model {
	proj := s.Current()
	if proj.Dir == "" {
		return project.Model{}
	}
	p := s.Provider()
	return fill(p, proj.Dir, project.Model{Model: model})
}

// forkStart is what a fork starts from: the thread it forks and that thread's title, which a woken fork keeps, and for a
// new fork that thread's conversation, the message it forks at and its chat up to that message.
type forkStart struct {
	of, title, conversation, at string
	events                      []chat.Event
}

// Fork starts a thread that continues the conversation of the thread id, live or ended, as a new conversation that
// ends at the message at: the uuid of a reply's last block. The fork has the same access, issue, model, effort and
// handover, starts in the folder the thread runs in, its worktree too, since Claude finds a conversation by its folder,
// and its chat starts as the source's does up to that reply. The issue is only shown: the fork has no branch and hears
// no pull request. It is refused while a clean-up of the thread runs, which may be removing that worktree.
func (s *Sessions) Fork(id, at string) (Session, error) {
	s.mu.Lock()
	in, found := s.infoOf(id)
	conversation := s.conversationOf(id)
	proj, known := s.projectOf(in)
	deleting := s.deleting[id]
	s.mu.Unlock()
	switch {
	case !found:
		return Session{}, fmt.Errorf("No thread %s", id)
	case deleting:
		return Session{}, fmt.Errorf("Thread %d is being deleted", s.Number(id))
	case conversation == "":
		return Session{}, errors.New("This thread has no conversation to fork yet")
	case !known:
		return Session{}, fmt.Errorf("The project of thread %s is gone", id)
	case s.life.CleaningUp(id):
		return Session{}, fmt.Errorf("Thread %d is being cleaned up; fork it when that ends", s.Number(id))
	}
	events, err := chat.ReadLog(s.chatLogPath(in.Name), 0)
	if err != nil {
		log.Printf("orchestrator: reading the chat log of %s: %v", id, err)
		return Session{}, errors.New("Could not read the thread's chat")
	}
	cut, compacted := -1, -1
	for i, e := range events {
		if e.UUID == at {
			cut = i
		}
		if e.Kind == chat.KindCompact {
			compacted = i
		}
	}
	if at == "" || cut < 0 {
		return Session{}, errors.New("That message is not in the thread's chat")
	}
	// Claude resumes a compacted conversation from its last compaction, so --resume-session-at cannot find an earlier message.
	if cut < compacted {
		return Session{}, errors.New("This message is from before the chat was compacted, so a fork cannot start there")
	}
	if _, err := os.Stat(in.Path); err != nil {
		return Session{}, fmt.Errorf("%s is gone, so this thread cannot be forked there", in.Path)
	}
	issue, _ := strconv.Atoi(in.Issue)
	title, model := cmp.Or(in.Title, defaultTitle)+" (fork)", project.Model{Model: in.Model, Effort: in.Effort}
	fork := forkStart{of: id, title: cmp.Or(in.Title, defaultTitle), conversation: conversation, at: at, events: events[:cut+1]}
	return s.createIn(proj, title, "", false, issue, "", s.handoverOf(id), "", fork, StartOptions{Model: model, Access: in.Access, Place: InProject, dir: in.Path})
}

// copyChat writes the fork's chat log before the fork's host opens it: the source's events, with the turn they stop in
// ended at the last of them, and copies the images they show into the fork's folder. Drafts are left out, and so are
// queued messages and those not sent, which only the source's host held: a message Claude took in is logged again as
// a user event. The source's run starts are left out too: the fork's host logs its own. restore removes what it wrote, for a fork that did not start.
func (s *Sessions) copyChat(fork forkStart, to session.Name) (restore func(), err error) {
	from, err := session.ParseName(fork.of)
	if err != nil {
		return nil, err
	}
	restore = func() {
		_ = os.Remove(s.chatLogPath(to))
		_ = os.RemoveAll(s.chatMediaDir(to))
	}
	chatLog, err := chat.OpenLog(s.chatLogPath(to))
	if err != nil {
		return nil, fmt.Errorf("copying the chat: %w", err)
	}
	err = s.copyEvents(chatLog, fork.events, from, to)
	if closeErr := chatLog.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		restore()
		return nil, fmt.Errorf("copying the chat: %w", err)
	}
	return restore, nil
}

// copyEvents appends the events and a turn's end to the fork's log, and copies each image they show from the source's
// folder, by its name.
func (s *Sessions) copyEvents(chatLog *chat.Log, events []chat.Event, from, to session.Name) error {
	for _, e := range events {
		switch e.Kind {
		case chat.KindDraft, chat.KindQueued, chat.KindUnsent, chat.KindStart:
			continue
		}
		for _, file := range e.Files {
			data, err := os.ReadFile(filepath.Join(s.chatMediaDir(from), filepath.Base(file)))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if err := atomicfile.Write(filepath.Join(s.chatMediaDir(to), filepath.Base(file)), data, 0o600); err != nil {
				return err
			}
		}
		if _, err := chatLog.Append(e); err != nil {
			return err
		}
	}
	_, err := chatLog.Append(chat.Event{Kind: chat.KindTurnEnd, At: events[len(events)-1].At})
	return err
}

// fill sets what model leaves unset from the agent's own settings, skipping a value the agent would not accept.
func fill(p agent.Provider, dir string, model project.Model) project.Model {
	l := p.Fill(dir, agent.Launch{Model: model.Model, Effort: model.Effort})
	if p.ValidModel(l.Model, "") == nil {
		model.Model = l.Model
	}
	if p.ValidModel("", l.Effort) == nil {
		model.Effort = l.Effort
	}
	return model
}

// reserve mints the name of a new session. It takes mu.
func (s *Sessions) reserve(proj project.Project) (session.Name, context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return session.Name{Project: proj.Key(), Token: session.NewToken()}, s.ctx
}

// launch starts the agent of the session under its host in tmux, in dir, a worktree on branch when branch is set. The
// host gets a new token, which the token it had before no longer matches. A child, whose parent is set, learns which
// thread it works for.
func (s *Sessions) launch(ctx context.Context, p agent.Provider, name session.Name, title string, proj project.Project, dir, branch string, issue int, model project.Model, conversation, brief, handover string, access session.Access, action string, fork forkStart, parent string) (err error) {
	env := []string{
		"ORCHESTRATOR_SESSION=" + name.String(),
		mcp.ControlEnv + "=" + control.SocketPath(s.stateDir),
	}
	tmp := s.tmpDir(name)
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		return fmt.Errorf("making the session's temp folder: %w", err)
	}
	env = append(env, "TMPDIR="+tmp)
	if handover != "" {
		if err := atomicfile.Write(s.handoverPath(name), []byte(handover), 0o600); err != nil {
			return fmt.Errorf("saving the handover: %w", err)
		}
	}
	browser := s.hasBrowser(proj, brief)
	if browser {
		agentEnv, err := s.browsers.AgentEnv(name.String(), tmp)
		if err != nil {
			log.Printf("orchestrator: session %s starts without its browser: %v", name, err)
			browser = false
		}
		env = append(env, agentEnv...)
	}
	tree := workTree{}
	if branch != "" {
		tree = workTree{branch: branch, dir: dir}
	}
	parentTitle := ""
	if parent != "" {
		parentTitle = s.titleOf(parent)
	}
	l := launchFor(model, conversation, brief, handover, access, browser, fork, tree, parentTitle, issue)
	l.TmpDir, l.ForkAt, l.MCPConfig = tmp, fork.at, s.mcpConfigPath(name)
	if err := p.Prepare(l); err != nil {
		return err
	}
	env = append(env, p.Env(l)...)
	argv, err := s.hosted(name, s.withoutRefused(p.Command(l)))
	if err != nil {
		return err
	}
	if err := s.mintToken(name); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			s.mu.Lock()
			s.forgetToken(name)
			s.mu.Unlock()
		}
	}()
	return s.start(ctx, name, title, dir, branch, env, argv, issue, model, access, action, fork.of, parent)
}

// hosted is argv run under this app's own host command, with the session's chat log and socket. It makes the socket's
// folder private, as the host asks; a folder an older run made may be wider, and it is the app's own.
func (s *Sessions) hosted(name session.Name, argv []string) ([]string, error) {
	socket := s.hostSocket(name)
	if len(socket) > host.MaxSocketPath {
		return nil, fmt.Errorf("the session socket path is too long: %d bytes, over the %d a unix socket takes", len(socket), host.MaxSocketPath)
	}
	dir := filepath.Dir(socket)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("making the session's socket folder: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("making the session's socket folder private: %w", err)
	}
	return append([]string{s.binary, "host", "--log", s.chatLogPath(name), "--socket", socket, "--"}, argv...), nil
}

// mcpConfig is the MCP config a thread's Claude starts the orchestrator server with.
type mcpConfig struct {
	Servers map[string]mcpServer `json:"mcpServers"`
}

type mcpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// mcpConfigPath is the MCP config of the session's host, beside its socket in the private folder.
func (s *Sessions) mcpConfigPath(name session.Name) string {
	return filepath.Join(filepath.Dir(s.hostSocket(name)), name.Token+".mcp.json")
}

// mintToken makes a new token for the session's host and writes the MCP config that hands it to the orchestrator server,
// in the host's private folder, so it is in no command line and in no environment but the server's. It takes mu.
func (s *Sessions) mintToken(name session.Name) error {
	raw := make([]byte, 32)
	rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	data, err := json.Marshal(mcpConfig{Servers: map[string]mcpServer{
		mcp.Server: {Command: s.binary, Args: []string{"mcp"}, Env: map[string]string{mcp.TokenEnv: token, mcp.ControlEnv: control.SocketPath(s.stateDir)}},
	}})
	if err != nil {
		return err
	}
	if err := atomicfile.Write(s.mcpConfigPath(name), data, 0o600); err != nil {
		return fmt.Errorf("saving the thread's MCP config: %w", err)
	}
	s.mu.Lock()
	s.tokens[name.String()] = tokenSum(token)
	s.mu.Unlock()
	return nil
}

// forgetToken makes the token of the session's host match no thread, and removes the config that holds it. It needs mu.
func (s *Sessions) forgetToken(name session.Name) {
	delete(s.tokens, name.String())
	if err := os.Remove(s.mcpConfigPath(name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("orchestrator: removing the MCP config of thread %s: %v", name, err)
	}
}

// readTokens takes the token of each live thread whose token it does not hold from the thread's MCP config: a thread
// the app started before it restarted keeps its token. It needs mu.
func (s *Sessions) readTokens() {
	for _, in := range s.info {
		if _, ok := s.tokens[in.Name.String()]; ok {
			continue
		}
		data, err := os.ReadFile(s.mcpConfigPath(in.Name))
		if err != nil {
			continue
		}
		var c mcpConfig
		if json.Unmarshal(data, &c) != nil {
			continue
		}
		if token := c.Servers[mcp.Server].Env[mcp.TokenEnv]; token != "" {
			s.tokens[in.Name.String()] = tokenSum(token)
		}
	}
}

// tokenSum is what the app keeps of a token: the token itself is only in the host's MCP config.
func tokenSum(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ThreadOfToken is the live thread, under a host and not deleted, whose host the token was made for. A token from
// before the thread's latest start, or of a thread that settled, names none.
func (s *Sessions) ThreadOfToken(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	sum := tokenSum(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, held := range s.tokens {
		if subtle.ConstantTimeCompare([]byte(held), []byte(sum)) == 1 && !s.deleted[id] && slices.ContainsFunc(s.info, func(in term.Info) bool { return in.Name.String() == id }) {
			return id, true
		}
	}
	return "", false
}

// register adds the new session, which runs in work, to the list and returns its row. forkOf is the thread it was
// forked from and parent the thread that delegated to it, if any. It takes mu.
func (s *Sessions) register(name session.Name, title string, work workPlace, issue int, model project.Model, created time.Time, access session.Access, action, forkOf, forkAt, parent string) Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if forkAt != "" {
		rec := session.Record{Session: name.String(), State: session.Idle, At: created}
		// A later read of the state file has no newer At, so it never adds the start to this record, and tmux dates a
		// session only to the second: a delete's heir is the fork that started last.
		rec.ForkAt, rec.Created, rec.Woke = forkAt, created.UnixMilli(), created.UnixMilli()
		if err := bus.WriteState(s.stateDir, rec); err != nil {
			log.Printf("orchestrator: saving where %s was forked: %v", name, err)
		}
		s.records[rec.Session] = rec
	}
	in := term.Info{Name: name, Title: title, Path: work.dir, Branch: work.branch, Model: model.Model, Effort: model.Effort, Access: access, Action: action, Fork: forkOf, Parent: parent, Created: created}
	if issue > 0 {
		in.Issue = strconv.Itoa(issue)
	}
	if !slices.ContainsFunc(s.info, func(i term.Info) bool { return i.Name == name }) {
		s.info = append(s.info, in)
	}
	s.number()
	s.sync()
	for _, v := range s.list() {
		if v.ID == name.String() {
			return v
		}
	}
	return Session{}
}

func (s *Sessions) start(ctx context.Context, name session.Name, title, dir, branch string, env, argv []string, issue int, model project.Model, access session.Access, action, forkOf, parent string) error {
	if err := s.tmux.NewSession(ctx, name, title, dir, env, argv, startCols, startRows); err != nil {
		return err
	}
	tag := func(err error) error {
		if err == nil {
			return nil
		}
		// Without its tags the session would be a stray: untracked, and tmux would still run it.
		killCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tmuxTimeout)
		defer cancel()
		_ = s.tmux.Kill(killCtx, name)
		return err
	}
	if issue > 0 {
		if err := tag(s.tmux.SetIssue(ctx, name, issue)); err != nil {
			return err
		}
	}
	if err := tag(s.tmux.SetAccess(ctx, name, access)); err != nil {
		return err
	}
	if action != "" {
		if err := tag(s.tmux.SetAction(ctx, name, action)); err != nil {
			return err
		}
	}
	if forkOf != "" {
		if err := tag(s.tmux.SetFork(ctx, name, forkOf)); err != nil {
			return err
		}
	}
	if parent != "" {
		if err := tag(s.tmux.SetParent(ctx, name, parent)); err != nil {
			return err
		}
	}
	if branch != "" {
		if err := tag(s.tmux.SetBranch(ctx, name, branch)); err != nil {
			return err
		}
	}
	if model != (project.Model{}) {
		return tag(s.tmux.SetModel(ctx, name, model.Model, model.Effort))
	}
	return nil
}

// giveHost gives text to the agent through the session's host once its socket takes connections: sent when send is set,
// else left as a draft in the chat.
func (s *Sessions) giveHost(ctx context.Context, name session.Name, text string, send bool) {
	if err := toHost(ctx, s.hostSocket(name), text, send, hostWait); err != nil {
		log.Printf("orchestrator: session %s did not get its first prompt: %v", name, err)
	}
}

// toHost gives text to the agent through the host at socket once its socket takes connections, waiting at most wait.
// It is sent when submit is set, else left as a draft in the chat.
func toHost(ctx context.Context, socket, text string, submit bool, wait time.Duration) error {
	ready, cancel := context.WithTimeout(ctx, wait)
	err := host.Ready(ready, socket)
	cancel()
	if err != nil {
		return err
	}
	if !submit {
		return host.Draft(ctx, socket, text)
	}
	return host.Send(ctx, socket, text)
}

// send gives text to the session through its host: sent when submit is set, else left as a draft in its chat.
func (s *Sessions) send(ctx context.Context, name session.Name, text string, submit bool) {
	if err := toHost(ctx, s.hostSocket(name), text, submit, hostStarting); err != nil {
		log.Printf("orchestrator: session %s did not get a prompt: %v", name, err)
	}
}

// wake sends the prompt to the session once its turn is over and it is not waiting on the user:
// sent while an approval waits, the text would deny it. A prompt already held for the session
// is not held again; a session of a project that sends by hand gets it as a draft at once, with no holding. A settled
// thread is woken for it first.
func (s *Sessions) wake(ctx context.Context, t target, prompt string) {
	if t.ended && !s.wakeFor(t) {
		return
	}
	name, err := session.ParseName(t.id)
	if err != nil {
		return
	}
	if !t.proj.SendsPrompt() {
		s.send(ctx, name, prompt, false)
		return
	}
	s.mu.Lock()
	if holdsPrompts(s.stateOf(t.id)) {
		if h := (heldPrompt{Told: Told{Text: prompt}, command: true}); !slices.Contains(s.wakes[t.id], h) {
			s.wakes[t.id] = append(s.wakes[t.id], h)
		}
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.send(ctx, name, prompt, true)
}

// wakeFor wakes the settled thread of t for a pull request's command and says whether it runs. A thread with an issue
// stays settled when the issue has another live thread, which keeps the issue's branch.
func (s *Sessions) wakeFor(t target) bool {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()
	if t.issue != 0 {
		if live := s.liveIssueSession(t.proj.Key(), t.issue); live != "" {
			return live == t.id
		}
	}
	if _, err := s.wakeOrJoin(t.id, ""); err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: waking thread %s for the PR on its branch: %v\n", t.id, err)
		return false
	}
	return true
}

// whenTurnEnds runs then once the session is neither working nor waiting on the owner, or is gone: at once when it is
// so now. A PR that merged or closed mid-turn settles the thread this way, so its turn is not cut off.
func (s *Sessions) whenTurnEnds(id string, then func()) {
	s.mu.Lock()
	if s.turnRuns(id) {
		s.afterTurn[id] = append(s.afterTurn[id], then)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	then()
}

// endTurns runs what waited for a turn that ended or a session that went. The commands held for such a session are
// dropped: they came for the PR that has merged or closed since, and sent now they would start a turn the settle cuts
// off, or wake a thread that went. It needs mu.
func (s *Sessions) endTurns() {
	for id, then := range s.afterTurn {
		if s.turnRuns(id) {
			continue
		}
		delete(s.afterTurn, id)
		s.dropCommands(id)
		go func() {
			for _, f := range then {
				f()
			}
		}()
	}
}

// turnRuns says whether the live session's turn runs: it works, or it waits on the owner in the middle of the turn, for
// a permission or a question tool. A reply that ends in a question waits too, but its turn is over. It needs mu.
func (s *Sessions) turnRuns(id string) bool {
	if _, live := s.seen[id]; !live {
		return false
	}
	rec := s.records[id]
	return s.stateOf(id) == session.Working || awaitsOwner(rec)
}

// awaitsOwner says whether the thread waits on the owner in the middle of its turn, for a permission or a question tool.
func awaitsOwner(rec session.Record) bool {
	return rec.State == session.Waiting && !endsTurn(rec.Event)
}

// asks says whether an approval of the thread waits on the owner's answer, which a message from a thread would deny:
// in the middle of its turn, as its state says, or a subagent's after its turn ended, as its chat log says. It needs mu.
func (s *Sessions) asks(id string) bool {
	return awaitsOwner(s.records[id]) || s.background[id].Asks
}

func holdsPrompts(state session.State) bool {
	return state == session.Working || state == session.Waiting
}

// heldPrompt waits for its thread to take it. A command, for the thread's PR, waits until the thread neither works nor
// waits on the owner; a message a thread told it waits only while the thread waits on the owner's answer to an
// approval, which the message would deny. nextTurn holds a child's result a stop kept until the thread's next turn or
// run starts (NextTurn): it starts none itself, and wakes no thread.
type heldPrompt struct {
	Told
	command, nextTurn bool
}

// toldHeld says whether a message a thread told the thread id is held for it, or being sent, that goes before a new
// one: a result kept for the next turn does not. It needs mu.
func (s *Sessions) toldHeld(id string) bool {
	return s.delivering[id] || slices.ContainsFunc(s.wakes[id], func(h heldPrompt) bool { return !h.command && !h.nextTurn })
}

// dropCommands drops the commands held for the thread and keeps the messages threads told it. It needs mu.
func (s *Sessions) dropCommands(id string) {
	if kept := slices.DeleteFunc(s.wakes[id], func(h heldPrompt) bool { return h.command }); len(kept) > 0 {
		s.wakes[id] = kept
	} else {
		delete(s.wakes, id)
	}
}

// deliverWakes sends what was held for each thread once it may take it, in order. A thread that settled meanwhile, by
// itself or at the owner's ask, is woken for the messages threads told it, and one that stopped by itself is woken for
// its commands too; Settle drops the commands of a thread the owner or a clean-up settled. A thread whose held prompts
// are being sent is passed over until they are, so nothing held after them overtakes them. It needs mu.
func (s *Sessions) deliverWakes() {
	for id, all := range s.wakes {
		if s.delivering[id] {
			continue
		}
		_, live := s.seen[id]
		var now, later []heldPrompt
		for _, h := range all {
			if h.nextTurn || live && s.holds(id, h) {
				later = append(later, h)
			} else {
				now = append(now, h)
			}
		}
		if len(now) == 0 {
			continue
		}
		if len(later) == 0 {
			delete(s.wakes, id)
		} else {
			s.wakes[id] = later
		}
		s.delivering[id] = true
		go s.deliverHeld(s.ctx, id, live, now)
	}
}

// deliverHeld sends the prompts deliverWakes took for the thread, in order, waking it first when it is not live. A
// message its host refuses while an approval waits, or while it is held for a restart, goes back to be held, with every
// prompt after it.
func (s *Sessions) deliverHeld(ctx context.Context, id string, live bool, now []heldPrompt) {
	refused := false
	defer func() { s.doneDelivering(id, refused) }()
	name, err := session.ParseName(id)
	if err != nil {
		return
	}
	if !live {
		t, err := s.target(id)
		var told []heldPrompt
		for _, h := range now {
			switch {
			case !h.command:
				told = append(told, h)
			case err == nil:
				s.wake(ctx, t, h.Text)
			}
		}
		if len(told) == 0 {
			return
		}
		if err := s.awaken(id); err != nil {
			log.Printf("orchestrator: thread %s did not get %d messages held for it: %v", id, len(told), err)
			for _, h := range told {
				h.gone(err)
			}
			return
		}
		now = told
	}
	for i, h := range now {
		if h.command {
			s.send(ctx, name, h.Text, true)
			continue
		}
		_, err := s.deliverRanked(name, h.Told)
		var ranked *outranked
		switch {
		case errors.Is(err, host.ErrApprovalWaits), errors.Is(err, host.ErrRestarting):
			s.hold(id, now[i:]...)
			refused = true
			return
		case errors.As(err, &ranked):
			log.Printf("orchestrator: dropped a message held for thread %s from %s: %v", id, h.From, err)
		case err != nil:
			log.Printf("orchestrator: thread %s did not get a message from %s: %v", id, cmp.Or(h.From, "orchestrator"), err)
		}
		h.gone(err)
	}
}

// holds says whether the live thread id keeps the prompt h held: a command while the thread works or waits on the owner,
// a message a thread told it while an approval of the thread waits on the owner, whose answer the message would be
// taken for, and either while SettleIdle puts it to sleep or a change of access restarts it. It needs mu.
func (s *Sessions) holds(id string, h heldPrompt) bool {
	if s.idling[id] || s.restarting[id] {
		return true
	}
	if h.command {
		return holdsPrompts(s.stateOf(id))
	}
	return s.asks(id)
}

// doneDelivering ends the sending of the thread's held prompts. What was held meanwhile goes now, unless the host
// refused a message, which then waits for the next change of the thread's state. It takes mu.
func (s *Sessions) doneDelivering(id string, refused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.delivering, id)
	if !refused {
		s.deliverWakes()
	}
}

// Told is a message for a thread: its text and the thread it is from, "" for the app. Body is the sender's bare words,
// which Text frames for Claude, and Result marks a child's result; the log keeps both for the chat to show. Steer stops
// the turn that runs so the thread takes it in now. Ranked holds a message a thread sends of its own accord to the rank
// rule: it may reach no thread whose access is above its own, when it is told and again when it goes, also after being
// held. Done, when set, gets how a message Tell held ended: nil once its thread took it, else why it was dropped. It
// gets one value and must have room for it.
type Told struct {
	From, Text, Body      string
	Result, Steer, Ranked bool
	Done                  chan<- error
}

// gone tells m's sender how the held message ended, when it asked to know.
func (m Told) gone(err error) {
	if m.Done == nil {
		return
	}
	select {
	case m.Done <- err:
	default:
	}
}

// Tell gives m to the thread id and returns the id its host logged it under, "" while it is held. A settled thread is
// woken for it, but for a child's result while a clean-up of it is due or runs: that waits for its next wake. A thread that waits on the owner's answer to an approval holds it until the answer, so it never denies
// the approval, and one whose host is held for a restart until it runs again; one with messages held, or being sent,
// holds it after them. A working thread's host queues it, for the
// turn's next step or, with Steer, at once. A thread that is deleted or being deleted takes none.
func (s *Sessions) Tell(id string, m Told) (string, error) {
	name, err := session.ParseName(id)
	if err != nil {
		return "", err
	}
	if m.Ranked {
		s.childMu.Lock()
		err := s.outranks(id, m.From)
		s.childMu.Unlock()
		if err != nil {
			return "", err
		}
	}
	if m.Result && s.asleepForCleanup(id) {
		s.mu.Lock()
		// Asked again under the lock: a thread deleted meanwhile has had its messages dropped, and one woken has had its
		// next turn start, so a result held for either would wait forever. A child deleted meanwhile has had its results
		// dropped, so its result is too.
		if s.deleted[m.From] || s.deleting[m.From] {
			s.mu.Unlock()
			return "", ErrToldDeleted
		}
		if _, settled := s.ended[id]; settled {
			s.wakes[id] = append(s.wakes[id], heldPrompt{Told: m, nextTurn: true})
			s.mu.Unlock()
			return "", nil
		}
		s.mu.Unlock()
	}
	if err := s.awaken(id); err != nil {
		return "", err
	}
	s.mu.Lock()
	if s.asks(id) || s.idling[id] || s.toldHeld(id) {
		s.wakes[id] = append(s.wakes[id], heldPrompt{Told: m})
		s.mu.Unlock()
		return "", nil
	}
	s.delivering[id] = true
	s.mu.Unlock()
	msg, err := s.deliverRanked(name, m)
	refused := errors.Is(err, host.ErrApprovalWaits) || errors.Is(err, host.ErrRestarting)
	if refused {
		s.hold(id, heldPrompt{Told: m})
		msg, err = "", nil
	}
	s.doneDelivering(id, refused)
	return msg, err
}

// asleepForCleanup says whether the thread sleeps with a clean-up due or running: a child's result does not wake it,
// and waits for its next wake. It waits for the first look at the PRs after a start. It takes mu.
func (s *Sessions) asleepForCleanup(id string) bool {
	s.mu.Lock()
	_, settled := s.ended[id]
	s.mu.Unlock()
	return settled && s.life.cleanupDue(s.context(), id)
}

// outranked is the rank rule's refusal: a thread with access may not message a thread with more.
type outranked struct{ from, to session.Access }

func (e *outranked) Error() string {
	return fmt.Sprintf("A thread with %s access cannot message a thread with %s access", e.from, e.to)
}

// outranks is an *outranked when the thread id's access ranks above that of the thread from, which may then not
// message it. A thread that is gone ranks as Read. It needs childMu, under which an access changes.
func (s *Sessions) outranks(id, from string) error {
	a, b := cmp.Or(s.AccessOf(from), session.Read), cmp.Or(s.AccessOf(id), session.Read)
	if b.Rank() > a.Rank() {
		return &outranked{from: a, to: b}
	}
	return nil
}

// deliverRanked is deliver for a message that passes the rank rule now, checked under childMu, as an access changes.
// The lock is not held for the send itself: a host that is slow to answer would hold up every delegation and change of
// access.
func (s *Sessions) deliverRanked(name session.Name, m Told) (string, error) {
	if m.Ranked {
		s.childMu.Lock()
		err := s.outranks(name.String(), m.From)
		s.childMu.Unlock()
		if err != nil {
			return "", err
		}
	}
	return s.deliver(name, m)
}

// awaken makes sure the thread id runs, waking it when it has settled, once a clean-up of it that runs now has ended. A
// thread that is deleted or being deleted or is unknown is an error.
func (s *Sessions) awaken(id string) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if name.IsShell() {
		return errNotSession
	}
	s.mu.Lock()
	_, settled := s.ended[id]
	_, live := s.liveInfo(id)
	deleted, deleting := s.deleted[id], s.deleting[id]
	n := s.nums[id]
	s.mu.Unlock()
	switch {
	case deleted:
		return fmt.Errorf("Thread %s was deleted", id)
	case deleting:
		return fmt.Errorf("Thread %d is being deleted", n)
	case settled:
		s.awaitCleanup(id)
		_, err := s.wakeOrJoin(id, "")
		return err
	case !live:
		return fmt.Errorf("No thread %s", id)
	}
	return nil
}

// awaitCleanup waits, at most gitTimeout, for a clean-up of the thread that runs now to end: a wake is refused while
// one runs, and a message for the thread wakes it once the clean-up is over instead of failing.
func (s *Sessions) awaitCleanup(id string) {
	ctx, cancel := context.WithTimeout(s.context(), gitTimeout)
	defer cancel()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for s.life.CleaningUp(id) {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// HoldsTold says whether a message a thread told the thread id is held for it, or held prompts are being given to it
// now, a PR command among them: each starts a turn once it goes. It takes mu.
func (s *Sessions) HoldsTold(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.toldHeld(id)
}

// Reported is the seq of the child's latest turn_end whose result went to its parent, 0 when none did, as its state
// file says: a thread no event has reported on yet has no record in the app.
func (s *Sessions) Reported(id string) int64 {
	rec, err := bus.ReadState(s.stateDir, id)
	if err != nil {
		log.Printf("orchestrator: reading the state of thread %s: %v", id, err)
	}
	return rec.Reported
}

// SetReported records that the result of the child's turn that ended at the seq went to its parent, in the records the
// app holds and in the child's state file, so the app never sends it again, also after a restart. It takes mu.
func (s *Sessions) SetReported(id string, seq int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.records[id]; ok && rec.Reported < seq {
		rec.Reported = seq
		s.records[id] = rec
	}
	if e, ok := s.ended[id]; ok && e.rec.Reported < seq {
		e.rec.Reported = seq
	}
	rec, err := bus.ReadState(s.stateDir, id)
	if err != nil {
		return fmt.Errorf("reading the state of thread %s: %w", id, err)
	}
	if rec.Session == "" || rec.Reported >= seq {
		return nil
	}
	rec.Reported = seq
	if err := bus.WriteState(s.stateDir, rec); err != nil {
		return fmt.Errorf("saving the state of thread %s: %w", id, err)
	}
	return nil
}

// NextTurn hears that the thread id took a message or started a run: the results a stop kept for its next turn go now.
// It takes mu.
func (s *Sessions) NextTurn(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, asleep := s.ended[id]; asleep {
		return // a follower that started as the thread went to sleep
	}
	kept := false
	for i, h := range s.wakes[id] {
		if h.nextTurn {
			s.wakes[id][i].nextTurn, kept = false, true
		}
	}
	if kept {
		s.deliverWakes()
	}
}

// DropTold drops the messages threads told the thread id that are held for it, and says how many: a thread stopped
// at another thread's ask must not start a new turn for them once its approval ends. A child's result is kept for the
// thread's next turn instead, since nothing would send it again. It takes mu.
func (s *Sessions) DropTold(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropTold(id, errToldStopped)
}

var (
	errToldStopped = errors.New("it was stopped")
	// ErrToldDeleted is what Tell, or a held message's Done, gives when the thread or the child whose result it is was
	// deleted.
	ErrToldDeleted = errors.New("it was deleted")
)

// dropTold drops the messages threads told the thread that are held for it, logs it with why, which each one's Done
// gets, and says how many. A stop keeps each child's result, held for the thread's next turn. It needs mu.
func (s *Sessions) dropTold(id string, why error) int {
	all := s.wakes[id]
	var kept []heldPrompt
	for _, h := range all {
		switch {
		case h.command:
		case h.Result && errors.Is(why, errToldStopped):
			h.nextTurn = true
		default:
			h.gone(why)
			continue
		}
		kept = append(kept, h)
	}
	dropped := len(all) - len(kept)
	if dropped == 0 {
		if len(kept) > 0 {
			s.wakes[id] = kept
		}
		return 0
	}
	if len(kept) > 0 {
		s.wakes[id] = kept
	} else {
		delete(s.wakes, id)
	}
	log.Printf("orchestrator: dropped %d messages held for thread %s: %v", dropped, id, why)
	return dropped
}

// dropResultsOf drops each result of the child, deleted, that is kept for its parent's next turn, and tells its
// sender so. It needs mu.
func (s *Sessions) dropResultsOf(child string) {
	for id, all := range s.wakes {
		var kept []heldPrompt
		for _, h := range all {
			if h.nextTurn && h.From == child {
				h.gone(ErrToldDeleted)
				continue
			}
			kept = append(kept, h)
		}
		switch {
		case len(kept) == len(all):
		case len(kept) == 0:
			delete(s.wakes, id)
		default:
			s.wakes[id] = kept
		}
	}
}

// hold puts what a host refused while an approval waited back before what is held for the thread. It takes mu.
func (s *Sessions) hold(id string, hs ...heldPrompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wakes[id] = append(slices.Clone(hs), s.wakes[id]...)
}

// deliver gives m to the live thread's host and returns the id it logged it under. With Steer, the turn that runs
// takes it in now; a message that started a turn of its own was not queued, and needs no steer. It is
// host.ErrApprovalWaits when an approval waits that the recorded state did not show yet, and host.ErrRestarting while the
// host is held for a restart.
func (s *Sessions) deliver(name session.Name, m Told) (string, error) {
	socket, err := s.readyHost(name)
	var msg string
	if err == nil {
		msg, err = host.SendMessage(s.context(), socket, host.Message{Text: m.Text, From: m.From, Body: m.Body, Result: m.Result})
	}
	if errors.Is(err, host.ErrApprovalWaits) || errors.Is(err, host.ErrRestarting) {
		return "", err
	}
	if err != nil {
		log.Printf("orchestrator: thread %s did not take a message from %s: %v", name, cmp.Or(m.From, "orchestrator"), err)
		if refused := (*host.Refused)(nil); errors.As(err, &refused) {
			return "", errors.New(refused.Reason)
		}
		return "", fmt.Errorf("Thread %s did not take the message", name)
	}
	if m.Steer {
		if err := host.SendNowFrom(s.context(), socket, msg, m.From); err != nil && !errors.As(err, new(*host.Refused)) {
			log.Printf("orchestrator: steering thread %s: %v", name, err)
		}
	}
	return msg, nil
}

// Settle stops the session's tmux session, whose host stops Claude, and then the servers it started; its row stays,
// settled.
func (s *Sessions) Settle(id string) error { return s.settleBy(id, "") }

// SettleAsOwner is Settle at the owner's word, which the exit in the thread's log records: a parent hears that the
// owner put its child to sleep.
func (s *Sessions) SettleAsOwner(id string) error { return s.settleBy(id, chat.ByOwner) }

// settleBy settles the thread, with by as its exit's text: chat.ByOwner or none.
func (s *Sessions) settleBy(id, by string) error {
	if err := s.settle(id, chat.StopSettle, by); err != nil {
		return err
	}
	s.stopServers(false, id)
	return nil
}

// SettleIdle settles the live thread if its host is idle: no turn open or message queued, no approval waiting and no
// subagent at work. It says whether the thread sleeps now: a thread another settle, as the owner's Sleep, put to sleep
// first gives true and no error, since it is asleep either way. A message a thread tells it meanwhile is held, and
// wakes it once it sleeps; the host takes no other message from its check until it stops, so none is cut off. It takes
// mu.
func (s *Sessions) SettleIdle(id string) (bool, error) {
	name, err := session.ParseName(id)
	if err != nil {
		return false, fmt.Errorf("putting thread %s to sleep: %w", id, err)
	}
	s.mu.Lock()
	if s.idling[id] || s.toldHeld(id) {
		s.mu.Unlock()
		return false, nil
	}
	s.idling[id] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.idling, id)
		s.sync()
		s.mu.Unlock()
	}()
	done, err := s.startSettle(id)
	if errors.As(err, new(errAsleep)) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("putting thread %s to sleep: %w", id, err)
	}
	defer done()
	socket, err := s.readyHost(name)
	idle := false
	if err == nil {
		idle, err = host.Hold(s.context(), socket)
	}
	if err != nil {
		return false, fmt.Errorf("holding the host of thread %s: %w", id, err)
	}
	if !idle {
		return false, nil
	}
	if err := s.stop(name, chat.StopSettle, ""); err != nil {
		s.release(name)
		return false, fmt.Errorf("putting thread %s to sleep: %w", id, err)
	}
	s.stopServers(false, id)
	return true, nil
}

// settle stops the thread for reason, with text as its exit's: chat.StopSettle with chat.ByOwner or none, or
// chat.StopRestart with the access it starts again with. Its host is told first, so it logs Claude's exit as that stop
// and not as a crash; a kill that fails takes that back.
func (s *Sessions) settle(id, reason, text string) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if name.IsShell() {
		return errNotSession
	}
	done, err := s.startSettle(id)
	if err != nil {
		return err
	}
	defer done()
	return s.stop(name, reason, text)
}

// errAsleep refuses a settle of a thread that sleeps already.
type errAsleep struct{ id string }

func (e errAsleep) Error() string { return fmt.Sprintf("Thread %s is asleep", e.id) }

// startSettle marks a settle of the thread as running, once any other settle of it has ended, and gives the func that
// ends the mark. Two at once, as the owner's Sleep and the one after a child's result, would both kill the session, and
// the one that lost would fail and take back the planned stop of the one that won. It refuses a thread that is deleted
// or asleep. It takes mu.
func (s *Sessions) startSettle(id string) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for running := s.settling[id]; running != nil; running = s.settling[id] {
		s.mu.Unlock()
		<-running
		s.mu.Lock()
	}
	_, ended := s.ended[id]
	switch {
	case s.deleted[id]:
		return nil, fmt.Errorf("Thread %s was deleted", id)
	case ended:
		return nil, errAsleep{id}
	}
	running := make(chan struct{})
	s.settling[id] = running
	return func() {
		s.mu.Lock()
		delete(s.settling, id)
		s.mu.Unlock()
		close(running)
	}, nil
}

// stop stops the thread a settle marked as running, as settle says. It takes mu.
func (s *Sessions) stop(name session.Name, reason, text string) error {
	id := name.String()
	// Dropped before the kill, so a Refresh that sees the session gone first does not wake it for them. The messages
	// threads told it are kept, and wake it once it has settled.
	s.mu.Lock()
	s.dropCommands(id)
	s.mu.Unlock()
	planned := s.planStop(name, reason, text)
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	if err := s.tmux.Kill(ctx, name); err != nil {
		if planned {
			s.release(name)
		}
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// The session is gone: sync turns it into a settled row.
	s.info = slices.DeleteFunc(s.info, func(i term.Info) bool { return i.Name == name })
	s.forgetToken(name)
	s.sync()
	return nil
}

// planStop tells the thread's host that the app stops it for reason, with text, and says whether the host heard. A
// host that did not logs the exit as it comes.
func (s *Sessions) planStop(name session.Name, reason, text string) bool {
	socket, err := s.readyHost(name)
	if err == nil {
		err = host.PlanStop(s.context(), socket, reason, text)
	}
	if err != nil {
		log.Printf("orchestrator: thread %s stops without telling its host: %v", name, err)
		return false
	}
	return true
}

// release takes back the hold and the planned stop of the thread's host, for a restart or a settle that did not happen.
func (s *Sessions) release(name session.Name) {
	if err := host.Release(s.context(), s.hostSocket(name)); err != nil {
		log.Printf("orchestrator: releasing the host of thread %s: %v", name, err)
	}
}

var (
	errAccessDuringTurn     = errors.New("Change access when the turn ends")
	errAccessWhileSubagents = errors.New("Change access when the subagents end")
)

// accessChange is what is left to do once the thread's new access is checked.
type accessChange int

const (
	accessDone    accessChange = iota // the access is recorded, or was the thread's already
	accessLive                        // Claude changes its permission mode as it runs
	accessRestart                     // the thread settled with the new access and wakes with it
)

// SetAccess gives the thread id the access between turns. A settled thread starts with it at its next wake. A live one
// changes as it runs between Read and Supervised; a change to or from Full starts it again under the same id, resuming
// its conversation, since Claude takes bypassPermissions only at its start.
func (s *Sessions) SetAccess(id string, access session.Access) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	// A delegation checks the parent's access under childMu and starts the child under it.
	s.childMu.Lock()
	defer s.childMu.Unlock()
	if name.IsShell() {
		return errNotSession
	}
	change, err := s.checkAccess(name, access)
	switch {
	case err != nil:
		return err
	case change == accessRestart:
		defer s.restarted(id)
		_, err := s.wakeOrJoin(id, "")
		return err
	case change == accessLive:
		return s.changeMode(name, access)
	}
	return nil
}

// checkAccess records the access of a settled thread, and of a live one that must start again for it, which it then
// settles. Its host says whether a turn is open: the recorded state lags a message just written to Claude, and a reply that
// ends in a question waits on the owner with no turn open. A live change is checked by the host as it makes it. It holds
// createMu, so neither a refresh nor a wake comes between the check and the settle. A record or a settle that fails
// after the hold releases the host, and a failed settle puts the access back.
func (s *Sessions) checkAccess(name session.Name, access session.Access) (accessChange, error) {
	id := name.String()
	s.createMu.Lock()
	defer s.createMu.Unlock()
	s.mu.Lock()
	in, live := term.Info{}, false
	if i := slices.IndexFunc(s.info, func(i term.Info) bool { return i.Name == name }); i >= 0 {
		in, live = s.info[i], true
	}
	_, settled := s.ended[id]
	deleted := s.deleted[id]
	// The events of a subagent's tool calls say the thread works, so they cannot tell its own turn from its subagents'.
	subagents := s.background[id].Subagents > 0
	s.mu.Unlock()
	switch {
	case deleted:
		return accessDone, fmt.Errorf("Thread %s was deleted", id)
	case settled:
		return accessDone, s.settledAccess(id, access)
	case !live:
		return accessDone, fmt.Errorf("No thread %s", id)
	case in.Access == access:
		return accessDone, nil
	case in.Access != session.Full && access != session.Full:
		return accessLive, nil
	}
	// A host that answers idle is held: it takes no message from here until Settle stops it.
	socket, err := s.readyHost(name)
	idle := false
	if err == nil {
		idle, err = host.Hold(s.context(), socket)
	}
	switch {
	case err != nil:
		return accessDone, accessFailed(name, access, err)
	case !idle && subagents:
		return accessDone, errAccessWhileSubagents
	case !idle:
		return accessDone, errAccessDuringTurn
	}
	if err := s.liveAccess(name, access); err != nil {
		s.release(name)
		return accessDone, fmt.Errorf("saving the thread's access: %w", err)
	}
	s.mu.Lock()
	s.restarting[id] = true
	s.mu.Unlock()
	if err := s.settle(id, chat.StopRestart, string(access)); err != nil {
		s.restarted(id)
		s.release(name)
		if undo := s.liveAccess(name, in.Access); undo != nil {
			log.Printf("orchestrator: putting back the access of thread %s: %v", id, undo)
		}
		return accessDone, err
	}
	return accessRestart, nil
}

// restarted ends the mark a restart for a change of access set, which kept the thread's servers.
func (s *Sessions) restarted(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.restarting, id)
}

// readyHost is the socket of the live thread's host once it listens, waiting for one that is still starting.
func (s *Sessions) readyHost(name session.Name) (string, error) {
	socket := s.hostSocket(name)
	ready, cancel := context.WithTimeout(s.context(), hostStarting)
	defer cancel()
	return socket, host.Ready(ready, socket)
}

// accessFailed is the host's own reason for not changing the thread's access, or a plain sentence when the host could
// not be reached, whose cause goes to the log.
func accessFailed(name session.Name, access session.Access, err error) error {
	log.Printf("orchestrator: thread %s did not change its access to %s: %v", name, access, err)
	if refused := (*host.Refused)(nil); errors.As(err, &refused) {
		return errors.New(refused.Reason)
	}
	return fmt.Errorf("Thread %s did not change its access", name)
}

// settledAccess records the settled thread's access in its state file, where its next wake reads it. It takes mu.
func (s *Sessions) settledAccess(id string, access session.Access) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.ended[id]
	if !ok {
		return fmt.Errorf("Thread %s did not go to sleep", id)
	}
	if e.info.Access == access {
		return nil
	}
	before := e.info.Access
	e.info.Access = access
	if err := bus.WriteState(s.stateDir, e.record()); err != nil {
		e.info.Access = before
		return fmt.Errorf("saving the thread's access: %w", err)
	}
	s.sync()
	return nil
}

// changeMode changes the live thread's access as Claude runs, through its host, which refuses it while busy. The access
// is recorded before the host is asked, so the mode Claude then reports meets the new access and not the old one, which
// it would give more than; a refusal puts the old access back.
func (s *Sessions) changeMode(name session.Name, access session.Access) error {
	s.mu.Lock()
	var before session.Access
	if i := slices.IndexFunc(s.info, func(i term.Info) bool { return i.Name == name }); i >= 0 {
		before = s.info[i].Access
	}
	s.mu.Unlock()
	if err := s.liveAccess(name, access); err != nil {
		return fmt.Errorf("saving the thread's access: %w", err)
	}
	socket, err := s.readyHost(name)
	if err == nil {
		err = host.SetMode(s.context(), socket, agent.PermissionMode(access))
	}
	if err != nil {
		if undo := s.liveAccess(name, before); undo != nil {
			log.Printf("orchestrator: putting back the access of thread %s: %v", name, undo)
		}
		return accessFailed(name, access, err)
	}
	return nil
}

// FlagRefused leaves flag out of every Claude this run of the app starts from now on: a thread's Claude did not know it.
func (s *Sessions) FlagRefused(flag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refused[flag] {
		return
	}
	s.refused[flag] = true
	log.Printf("orchestrator: claude does not know %s, so threads start without it", flag)
}

// withoutRefused is argv without the flags Claude did not know.
func (s *Sessions) withoutRefused(argv []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for flag := range s.refused {
		argv = agent.WithoutFlag(argv, flag)
	}
	return argv
}

// AccessChanged follows a permission mode the live thread's Claude reports. The access a thread has changes only when
// the owner sets it, which SetAccess records. The one mode Claude's report adds is a Read thread's Supervised right
// after the owner approved its plan (planApproved), as Claude leaves plan mode to carry the plan out; any other mode,
// plan mode in a Full thread among them, leaves the access as it is.
func (s *Sessions) AccessChanged(id, mode string, planApproved bool) {
	access, ok := agent.AccessOfMode(mode)
	name, err := session.ParseName(id)
	if !ok || err != nil {
		log.Printf("orchestrator: thread %s reports a permission mode orchestrator does not know: %q", id, mode)
		return
	}
	// A delegation checks the parent's access under childMu and starts the child under it.
	s.childMu.Lock()
	defer s.childMu.Unlock()
	s.mu.Lock()
	i := slices.IndexFunc(s.info, func(i term.Info) bool { return i.Name == name })
	var have session.Access
	if i >= 0 {
		have = s.info[i].Access
	}
	s.mu.Unlock()
	if i < 0 {
		return
	}
	if !planApproved || have != session.Read || access != session.Supervised {
		return
	}
	if err := s.liveAccess(name, access); err != nil {
		log.Printf("orchestrator: recording the access of thread %s: %v", id, err)
	}
}

// liveAccess keeps the live thread's access where its start choice is kept, its tmux session and its state file.
func (s *Sessions) liveAccess(name session.Name, access session.Access) error {
	s.mu.Lock()
	i := slices.IndexFunc(s.info, func(i term.Info) bool { return i.Name == name })
	same := i >= 0 && s.info[i].Access == access
	s.mu.Unlock()
	if i < 0 || same {
		return nil
	}
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	if err := s.tmux.SetAccess(ctx, name, access); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.info {
		if s.info[i].Name == name {
			s.info[i].Access = access
			s.saveIdentity(s.info[i])
			s.sync()
		}
	}
	return nil
}

func (s *Sessions) Rename(id, title string) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if name.IsShell() {
		return errNotSession
	}
	title = cmp.Or(strings.TrimSpace(title), defaultTitle)
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	if err := s.tmux.Rename(ctx, name, title); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.info {
		if s.info[i].Name == name {
			s.info[i].Title = title
			s.saveIdentity(s.info[i])
		}
	}
	s.sync()
	return nil
}

func (s *Sessions) context() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctx
}

// changed re-checks what the front end shows after something outside the session list moved.
func (s *Sessions) changed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sync()
}

// workTargets are the sessions that have a branch to follow, live or ended, in any project we know.
func (s *Sessions) workTargets() []target {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []target
	projs := s.projects()
	for _, in := range s.info {
		if t, ok := s.targetOf(in, projs); ok {
			out = append(out, t)
		}
	}
	for _, e := range s.ended {
		if t, ok := s.targetOf(e.info, projs); ok {
			t.ended = true
			out = append(out, t)
		}
	}
	return out
}

func (s *Sessions) target(id string) (target, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range s.info {
		if in.Name.String() == id {
			return s.targetOrErr(in, false)
		}
	}
	if e, ok := s.ended[id]; ok {
		return s.targetOrErr(e.info, true)
	}
	return target{}, fmt.Errorf("no session %s", id)
}

// targetOrErr needs mu. A fork that a Delete gave a worktree to has a target too, so a Delete of the fork can take it;
// workTargets leaves it out, so it hears no pull request.
func (s *Sessions) targetOrErr(in term.Info, ended bool) (target, error) {
	t, ok := s.targetOf(in, s.projects())
	if !ok {
		t, ok = s.handedTarget(in)
	}
	if !ok {
		return target{}, fmt.Errorf("session %s has no issue or branch to clean up after", in.Name)
	}
	t.ended = ended
	return t, nil
}

// handedTarget is the target of a fork that holds a worktree orchestrator made, which a Delete gave it: the worktree and
// the branch of its record. It needs mu.
func (s *Sessions) handedTarget(in term.Info) (target, bool) {
	proj, ok := projectIn(s.projects(), in)
	if !ok || in.Fork == "" {
		return target{}, false
	}
	id := in.Name.String()
	w, ok := s.made.Of(proj.Key(), id)
	if !ok || w.Removed != 0 || w.Branch == "" {
		return target{}, false
	}
	_, woke := s.startsOf(id, in)
	return target{id: id, title: cmp.Or(in.Title, defaultTitle), branch: w.Branch, dir: w.Path, proj: proj, model: project.Model{Model: in.Model, Effort: in.Effort}, woke: woke, made: true}, true
}

// targetOf finds the session's project in projs. Only a thread with a branch has a target: a fork and a thread started
// before orchestrator recorded branches have none. It needs mu.
func (s *Sessions) targetOf(in term.Info, projs []project.Project) (target, bool) {
	proj, ok := projectIn(projs, in)
	if !ok || in.Fork != "" || in.Branch == "" {
		return target{}, false
	}
	issue, _ := strconv.Atoi(in.Issue)
	id := in.Name.String()
	_, woke := s.startsOf(id, in)
	t := target{id: id, title: cmp.Or(in.Title, defaultTitle), issue: issue, branch: in.Branch, dir: cmp.Or(in.Path, proj.Dir), proj: proj, model: project.Model{Model: in.Model, Effort: in.Effort}, woke: woke}
	if w, ok := s.made.Of(proj.Key(), id); ok && w.Removed == 0 && w.Branch == in.Branch {
		t.dir, t.made = w.Path, true
	}
	return t, true
}

// conversationOf is the agent's conversation id of a session, running or ended, "" until the agent has written its
// transcript: Claude names the conversation at start but writes it only at the first prompt, and --resume finds none
// before that. It needs mu.
func (s *Sessions) conversationOf(id string) string {
	rec := s.records[id]
	if e, ok := s.ended[id]; ok {
		rec = e.rec
	}
	if _, err := os.Stat(rec.Transcript); err != nil {
		return ""
	}
	return rec.Conversation
}

// projectOf needs mu.
func (s *Sessions) projectOf(in term.Info) (project.Project, bool) {
	return projectIn(s.projects(), in)
}

func projectIn(projs []project.Project, in term.Info) (project.Project, bool) {
	for _, p := range projs {
		if p.Key() == in.Name.Project {
			return p, true
		}
	}
	return project.Project{}, false
}

// TypeInto leaves text as a draft in the session's chat.
func (s *Sessions) TypeInto(id, text string) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if err := s.refuseSettled(id); err != nil {
		return err
	}
	if err := toHost(s.context(), s.hostSocket(name), text, false, hostStarting); err != nil {
		log.Printf("orchestrator: session %s did not take a message: %v", id, err)
		return fmt.Errorf("session %s did not take the text", id)
	}
	return nil
}

// refuseSettled is an error for a session that has settled or been deleted. It takes mu.
func (s *Sessions) refuseSettled(id string) error {
	s.mu.Lock()
	_, ended := s.ended[id]
	deleted := s.deleted[id]
	s.mu.Unlock()
	switch {
	case deleted:
		return fmt.Errorf("Thread %s was deleted", id)
	case ended:
		return fmt.Errorf("Thread %s is asleep", id)
	}
	return nil
}

// Observe takes in the changes of the thread's state that its host's stream shows, in the names of Claude's hooks, and
// saves the record after each. The record holds the latest seq of the chat log it folds in, so an event taken in before
// a restart is not taken in twice. The row and what follows from it change once, for the last of them: the events a
// follower catches up on may end with the thread asleep, or answered. A thread that is asleep or deleted takes none. It
// takes mu.
func (s *Sessions) Observe(id string, evs ...session.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed, turnEnded, toolDone := false, false, false
	for _, ev := range evs {
		if s.observe(id, ev) {
			changed = true
			turnEnded = turnEnded || endsTurn(ev.Name)
			toolDone = toolDone || ev.Name == "PostToolUse"
		}
	}
	if changed {
		// A catch-up on many events does what they call for once: the PR lookup if one ended a turn, the branch read if
		// one was a tool's result, and one read of the usage, which records the turns' if one ended a turn.
		s.follow(s.records[id], turnEnded, toolDone)
		s.sync()
	}
}

// observe takes in one change of the thread's state, apart from what follows from the record. It says whether the
// thread's record changed. It needs mu.
func (s *Sessions) observe(id string, ev session.Event) bool {
	if _, ended := s.ended[id]; ended || s.deleted[id] {
		return false
	}
	held := s.records[id]
	if ev.Seq > 0 && ev.Seq <= held.Observed {
		return false
	}
	prev, _ := bus.ReadState(s.stateDir, id)
	if prev.Session == "" {
		prev = held
	}
	if ev.Seq > 0 && ev.Seq <= prev.Observed {
		return false
	}
	latest := prev.At
	if held.At.After(latest) {
		latest = held.At
	}
	at := ev.At
	if at.IsZero() {
		at = time.Now()
	}
	// An event older than the record is an earlier one's, which another change of the state has passed. The log's times
	// have only milliseconds.
	if at.UnixMilli() < latest.UnixMilli() {
		return false
	}
	if ev.Name == "SessionEnd" {
		// A thread is asleep once tmux has lost its session, which a wake and a delete go by, so its row stays as it
		// was until then. Only the seq is kept.
		prev.Observed = max(prev.Observed, ev.Seq)
		if prev.Session != "" {
			if err := bus.WriteState(s.stateDir, prev); err != nil {
				log.Printf("orchestrator: saving the state of thread %s: %v", id, err)
			}
		}
		if held.Session != "" {
			held.Observed = prev.Observed
			s.records[id] = held
		}
		return false
	}
	rec := session.Apply(id, prev, ev, at)
	if err := bus.WriteState(s.stateDir, rec); err != nil {
		log.Printf("orchestrator: saving the state of thread %s: %v", id, err)
		return false
	}
	if ev.Name == "UserPromptSubmit" {
		s.record(id, activity.Event{At: rec.At.UnixMilli(), Kind: activity.Prompt})
	}
	s.records[id] = rec
	return true
}

// TranscriptPath is the file the agent writes the conversation to, for an agent started in the folder origin.
func (s *Sessions) TranscriptPath(origin, conversation string) string {
	p := s.Provider()
	return p.Transcript(origin, conversation)
}

// Background is what a thread's Claude does beside its turns: subagents in the background, which go on after the turn
// that started them, and the turn Claude then starts itself to tell of one's end. Asks is an approval that waits on the
// owner as the chat log shows it: a subagent's waits past the end of the turn that started it, when the recorded state
// says the thread is idle.
type Background struct {
	Subagents int  // the tasks other than shells it runs in the background
	Reporting bool // a task of its own ended, and Claude tells of it in a turn of its own that has not ended yet
	Asks      bool
	At        time.Time
}

func (b Background) working() bool { return b.Subagents > 0 || b.Reporting }

// BackgroundChanged records what the thread's Claude does beside its turns, as the chat service reads it from the
// host. The stream says only that a turn ended, so the thread shows idle at the turn that started a subagent. It takes
// mu.
func (s *Sessions) BackgroundChanged(id string, b Background) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.ContainsFunc(s.info, func(in term.Info) bool { return in.Name.String() == id }) {
		return
	}
	if b.At.IsZero() {
		b.At = time.Now()
	}
	s.background[id] = b
	s.sync()
}

// stateOf is the live session's state: its events', but working while it is idle and its Claude works beside its turns.
// It needs mu.
func (s *Sessions) stateOf(id string) session.State {
	state := session.Idle
	if rec, ok := s.records[id]; ok {
		state = rec.State
	}
	if state == session.Idle && s.background[id].working() {
		return session.Working
	}
	return state
}

// Chat is a session whose agent runs, or ran, under a host: the chat log the host keeps, the socket it listens on, the
// folder that holds the images sent in it and the folder it works in.
type Chat struct {
	ID, Project, Log, Socket, Media, Dir string
	N                                    int  // the number the thread shows
	Live                                 bool // the session runs; an ended one has only its log
	Deleting                             bool // a delete of the thread runs
}

// ChatOf is the chat of the session id, live or ended. A deleted thread gives none. It takes mu.
func (s *Sessions) ChatOf(id string) (Chat, error) {
	name, err := session.ParseName(id)
	if err != nil {
		return Chat{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	in, live := term.Info{}, false
	if s.deleted[id] {
		return Chat{}, fmt.Errorf("Thread %s was deleted", id)
	}
	if e, ok := s.ended[id]; ok {
		in = e.info
	} else if i := slices.IndexFunc(s.info, func(i term.Info) bool { return i.Name == name }); i >= 0 {
		in, live = s.info[i], true
	} else {
		return Chat{}, fmt.Errorf("No thread %s", id)
	}
	return s.chatOf(in, live), nil
}

// Chats are the chats of the live sessions. It takes mu.
func (s *Sessions) Chats() []Chat {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Chat
	for _, in := range s.info {
		out = append(out, s.chatOf(in, true))
	}
	return out
}

// chatOf is the session's chat; it works in its worktree once the lifecycle knows it, else where it started. It needs mu.
func (s *Sessions) chatOf(in term.Info, live bool) Chat {
	name := in.Name
	dir := cmp.Or(s.life.fields(name.String()).worktree, in.Path)
	return Chat{ID: name.String(), Project: name.Project, Log: s.chatLogPath(name), Socket: s.hostSocket(name), Media: s.chatMediaDir(name), Dir: dir, N: s.nums[name.String()], Live: live, Deleting: s.deleting[name.String()]}
}

// AfterList sets what Refresh runs once it has listed tmux and before it applies the list; a test settles a thread there.
func (s *Sessions) AfterList(f func()) { s.afterList = f }

// SetCleanupSweep sets how often a clean-up the checks kept is tried again, an hour by default; 0 or less turns the
// sweep off. It comes before Run.
func (s *Sessions) SetCleanupSweep(every time.Duration) { s.life.sweepEvery = every }

// Run follows tmux and the pull requests until ctx ends.
func (s *Sessions) Run(ctx context.Context) error {
	if err := s.Start(ctx); err != nil {
		return err
	}
	go s.life.Run(ctx)
	return nil
}

// Track makes branch the one the thread works on, and looks up its pull request.
func (s *Sessions) Track(id, branch string) error {
	proj, err := s.setBranch(id, branch)
	if err != nil {
		return err
	}
	s.life.track(s.context(), id, proj, branch)
	return nil
}

// setBranch records branch as the live thread's, where its start choice is kept, unless another thread has it. When the
// thread's own worktree has the branch checked out, the record of the worktree follows it.
func (s *Sessions) setBranch(id, branch string) (project.Project, error) {
	// createMu keeps a start from taking the branch between the check and the record.
	s.createMu.Lock()
	defer s.createMu.Unlock()
	s.mu.Lock()
	var in term.Info
	var proj project.Project
	found := false
	if i := slices.IndexFunc(s.info, func(in term.Info) bool { return in.Name.String() == id }); i >= 0 {
		in = s.info[i]
		proj, found = s.projectOf(in)
	}
	s.mu.Unlock()
	switch {
	case !found:
		return proj, fmt.Errorf("session %q is not running", id)
	case in.Fork != "":
		return proj, fmt.Errorf("session %s is a fork, which works on no branch of its own", id)
	case inOthersFolder(in):
		return proj, fmt.Errorf("session %s is a child with no worktree, which works on no branch of its own", id)
	case in.Access == session.Read:
		return proj, errors.New("A Read thread works on no branch")
	}
	ctx := s.context()
	if out, err := s.life.git(ctx, proj.Dir, "check-ref-format", "--branch", branch); err != nil || strings.TrimSpace(string(out)) != branch {
		return proj, fmt.Errorf("%q is not a branch name git takes", branch)
	}
	if branch == s.life.mainBranch(ctx, proj.Dir) {
		return proj, fmt.Errorf("%s is the project's default branch; track the branch your work is on", branch)
	}
	if n, ok := s.threadOn(proj.Key(), branch, id); ok {
		return proj, fmt.Errorf("Thread %d works on %s", n, branch)
	}
	trees, err := s.life.worktrees(ctx, proj.Dir)
	if err != nil {
		if _, gitErr := s.life.git(ctx, proj.Dir, "rev-parse", "--git-dir"); gitErr == nil { // a project that is no git repo has no worktree to share
			return proj, fmt.Errorf("listing the worktrees: %w", err)
		}
	}
	if dir, ok := trees.byBranch[branch]; ok {
		if n, ok := s.threadIn(proj.Key(), dir, id); ok {
			return proj, fmt.Errorf("Thread %d works on %s", n, branch)
		}
	}
	tctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	if err := s.tmux.SetBranch(tctx, in.Name, branch); err != nil {
		return proj, err
	}
	if w, ok := s.made.Of(proj.Key(), id); ok && w.Removed == 0 {
		if out, err := s.life.git(ctx, w.Path, "branch", "--show-current"); err == nil && strings.TrimSpace(string(out)) == branch {
			if err := s.made.SetBranch(proj.Key(), w.Path, branch); err != nil {
				return proj, err
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.info {
		if s.info[i].Name == in.Name {
			s.info[i].Branch = branch
			s.saveIdentity(s.info[i])
		}
	}
	s.sync()
	return proj, nil
}

// usersOf is the title of a live thread not in except that works in dir or a folder inside it, "" when none: one that
// runs there, as a fork of a thread in its worktree does, or whose last reported folder is there. With asleep, a settled
// thread that would wake there counts too. It takes mu.
func (s *Sessions) usersOf(dir string, asleep bool, except ...string) string {
	if users := s.usersIn(dir, asleep, except...); len(users) > 0 {
		return cmp.Or(users[0].Title, defaultTitle)
	}
	return ""
}

// usersIn are the threads that usersOf looks for, the live ones first. It takes mu.
func (s *Sessions) usersIn(dir string, asleep bool, except ...string) []term.Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	var users []term.Info
	for _, in := range s.info {
		id := in.Name.String()
		if slices.Contains(except, id) {
			continue
		}
		if cwd := s.cwds[id]; in.Path != "" && folder.Inside(in.Path, dir) || cwd != "" && folder.Inside(cwd, dir) {
			users = append(users, in)
		}
	}
	if !asleep {
		return users
	}
	for id, e := range s.ended {
		if !slices.Contains(except, id) && e.info.Path != "" && folder.Inside(e.info.Path, dir) {
			users = append(users, e.info)
		}
	}
	return users
}

// worksIn says whether the live thread id works in dir, as usersOf finds it.
func (s *Sessions) worksIn(id, dir string) bool {
	return dir != "" && slices.ContainsFunc(s.usersIn(dir, false), func(in term.Info) bool { return in.Name.String() == id })
}

// heirOf is the fork outside family that works in the worktree dir, live or asleep, and has no worktree orchestrator made
// for it, so a Delete of family may give it the worktree; "" when there is none. A thread with a worktree of its own
// never gets a second one, and a thread that is not a fork works on a branch of its own. Of several, it is the one that
// started last, then the one made last, whose number is the higher, then the first by id, so a refusal and the Delete
// after it pick the same.
func (s *Sessions) heirOf(key, dir string, family []string) string {
	type fork struct {
		id      string
		created time.Time
		n       int
	}
	var forks []fork
	for _, in := range s.usersIn(dir, true, family...) {
		id := in.Name.String()
		if _, made := s.made.Of(key, id); in.Fork != "" && !made {
			s.mu.Lock()
			created, _ := s.startsOf(id, in)
			n := s.nums[id]
			s.mu.Unlock()
			forks = append(forks, fork{id, created, n})
		}
	}
	if len(forks) == 0 {
		return ""
	}
	return slices.MinFunc(forks, func(a, b fork) int {
		return cmp.Or(b.created.Compare(a.created), cmp.Compare(b.n, a.n), strings.Compare(a.id, b.id))
	}).id
}

// FilesRewind says whether a rewind may put back the files of the thread id, and where: only in the worktree orchestrator
// made for the thread, while no other live thread and no shell but the thread's own works there, and none of the
// thread's own shells runs a command. inProject is set for a thread that runs in the project folder, whose files a
// rewind never touches. reason says why the files cannot be rewound, "" when dir is where they may be.
func (s *Sessions) FilesRewind(id string) (dir string, inProject bool, reason string) {
	s.mu.Lock()
	in, found := s.infoOf(id)
	proj, known := s.projectOf(in)
	s.mu.Unlock()
	if !found || !known {
		return "", false, fmt.Sprintf("No thread %s", id)
	}
	if samePath(in.Path, proj.Dir) {
		return "", true, ""
	}
	ctx := s.context()
	w, made := s.made.Of(proj.Key(), id)
	if !made || w.Removed != 0 || !folder.Inside(in.Path, w.Path) || !s.life.ours(ctx, proj.Key(), w.Path, id) {
		return "", false, "Files are rewound only in a worktree orchestrator made for this thread"
	}
	if user := s.usersOf(w.Path, true, id); user != "" {
		return "", false, fmt.Sprintf("Thread %s also works in this worktree", user)
	}
	if shell := s.life.shellIn(ctx, proj.Key(), w.Path, id); shell != "" {
		return "", false, shell
	}
	if shell := s.busyShellOf(ctx, id); shell != "" {
		return "", false, shell
	}
	return w.Path, false, ""
}

// busyShellOf says why a shell of the thread keeps a rewind from its files: one runs a command, which may write them.
// It is "" when none does.
func (s *Sessions) busyShellOf(ctx context.Context, id string) string {
	ctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	_, shells, err := s.tmux.ListAll(ctx)
	if err != nil {
		return shellsUnlisted(err)
	}
	if slices.ContainsFunc(shells, func(sh term.Shell) bool { return sh.Thread == id && shellBusy(sh) }) {
		return "a shell of this thread runs a command"
	}
	return ""
}

// pending says why the thread is busy, or "" when it is not. Prompts are held while a turn runs, so they are named
// first, or their reason would rarely show. It takes mu.
func (s *Sessions) pending(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	// A child's result kept for the next wake does not: it waits out the clean-up.
	case slices.ContainsFunc(s.wakes[id], func(h heldPrompt) bool { return !h.nextTurn }):
		return "messages wait for this thread"
	case s.turnRuns(id):
		return "a turn is running"
	}
	return ""
}

// Has says whether a session of that id is running, in any project.
func (s *Sessions) Has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.info, func(in term.Info) bool { return in.Name.String() == id })
}

func (s *Sessions) closeBrowser(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxTimeout)
	defer cancel()
	s.browsers.Close(ctx, id)
}

// Shell is one shell of the current project.
type Shell struct {
	ID      string `json:"id"`
	Thread  string `json:"thread"`  // the thread the shell belongs to, "" for a shell of the project
	Command string `json:"command"` // the name of the process in front in the shell, such as zsh, vim or node, or ""
	Busy    bool   `json:"busy"`    // Command is not the login shell, so closing the shell stops what runs in it
}

// Shells are the current project's shells, by shell number.
func (s *Sessions) Shells() []Shell {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shellsIn(s.project.Key())
}

// shellsIn are the shells of the project key, by shell number. It needs mu.
func (s *Sessions) shellsIn(key string) []Shell {
	var mine []term.Shell
	for _, sh := range s.shells {
		if sh.Name.Project == key {
			mine = append(mine, sh)
		}
	}
	slices.SortFunc(mine, func(a, b term.Shell) int { return a.Name.Shell - b.Name.Shell })
	list := make([]Shell, len(mine))
	for i, sh := range mine {
		list[i] = Shell{ID: sh.Name.String(), Thread: sh.Thread, Command: sh.Command, Busy: shellBusy(sh)}
	}
	return list
}

// shellBusy says whether a command runs in front in the shell, as its terminal shows: a shell that waits for input is
// in front itself.
func shellBusy(sh term.Shell) bool {
	busy, err := procs.Busy(sh.PID)
	return err == nil && busy
}

// announceShells sends each project's shells when they changed since they were last sent. It needs mu.
func (s *Sessions) announceShells() {
	keys := map[string]bool{}
	for _, sh := range s.shells {
		keys[sh.Name.Project] = true
	}
	for k := range s.sentShells {
		keys[k] = true
	}
	for _, k := range slices.Sorted(maps.Keys(keys)) {
		list := s.shellsIn(k)
		if slices.Equal(list, s.sentShells[k]) {
			continue
		}
		s.emit("shells", scoped.Of(k, list))
		if len(list) == 0 {
			delete(s.sentShells, k)
		} else {
			s.sentShells[k] = list
		}
	}
}

// loginShell is the shell a new shell session runs, as a login shell.
func loginShell() string { return cmp.Or(os.Getenv("SHELL"), "/bin/zsh") }

// NewShell starts a shell in the current project, numbered after the shells it has, and returns its id. A shell is a
// login shell; it reports nothing: it is not an agent, so it gets no events and no session number. With thread "" it
// is a shell of the project, in the project folder. Else it belongs to the thread and starts in the thread's
// worktree, or in the project folder once that is gone; it is refused while a clean-up or a delete of the thread
// runs, which closes the thread's shells.
func (s *Sessions) NewShell(thread string) (string, error) {
	s.shellMu.Lock()
	defer s.shellMu.Unlock()
	s.mu.Lock()
	proj := s.project
	deleting := s.deleting[thread]
	s.mu.Unlock()
	dir := proj.Dir
	if thread != "" {
		switch {
		case s.life.CleaningUp(thread):
			return "", fmt.Errorf("Thread %d is being cleaned up; open a shell when that ends", s.Number(thread))
		case deleting:
			return "", fmt.Errorf("Thread %d is being deleted; open a shell when that ends", s.Number(thread))
		}
		folder, err := s.threadFolder(thread)
		switch {
		case err == nil:
			dir = folder
		case !errors.Is(err, errFolderGone):
			return "", err
		}
	}
	return s.startShell(proj, thread, dir)
}

// startShell starts a shell of the project proj that belongs to thread in dir. It needs shellMu.
func (s *Sessions) startShell(proj project.Project, thread, dir string) (string, error) {
	s.mu.Lock()
	ctx := s.ctx
	var used []int
	for _, sh := range s.shells {
		if sh.Name.Project == proj.Key() {
			used = append(used, sh.Name.Shell)
		}
	}
	s.mu.Unlock()
	name := session.Name{Project: proj.Key(), Shell: session.NextN(used)}
	tctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	env := []string{"ORCHESTRATOR_PROJECT=" + proj.Key()}
	if err := s.tmux.NewSession(tctx, name, "shell", dir, env, []string{loginShell(), "-l"}, startCols, startRows); err != nil {
		return "", err
	}
	if thread != "" {
		if err := s.tmux.SetThread(tctx, name, thread); err != nil {
			if err := s.tmux.Kill(tctx, name); err != nil {
				log.Printf("orchestrator: stopping the untagged shell %s: %v", name, err)
			}
			return "", err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A refresh may have listed the shell before its tag was on.
	s.shells = slices.DeleteFunc(s.shells, func(sh term.Shell) bool { return sh.Name == name })
	s.shells = append(s.shells, term.Shell{Name: name, Thread: thread})
	s.announceShells()
	return name.String(), nil
}

// CloseShell stops one shell of the current project.
func (s *Sessions) CloseShell(id string) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if !name.IsShell() {
		return fmt.Errorf("%s is not a shell", id)
	}
	return s.stopShell(name)
}

// closeShellsOf stops every shell of the thread, and those of the threads in others that are in the worktree or in a
// folder tmux cannot read, as tmux lists them now. A shell it cannot stop is logged and left running.
func (s *Sessions) closeShellsOf(thread, worktree string, others ...string) {
	if thread == "" {
		return
	}
	s.shellMu.Lock()
	defer s.shellMu.Unlock()
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	shells, err := s.tmux.ShellPaths(ctx)
	if err != nil {
		log.Printf("orchestrator: listing the shells of thread %s: %v", thread, err)
		return
	}
	for _, sh := range shells {
		inWorktree := worktree != "" && (sh.Path == "" || folder.Inside(sh.Path, worktree))
		if sh.Thread != thread && (!inWorktree || !slices.Contains(others, sh.Thread)) {
			continue
		}
		if err := s.stopShell(sh.Name); err != nil {
			log.Printf("orchestrator: closing shell %s of thread %s: %v", sh.Name, thread, err)
		}
	}
}

// stopShell stops the shell name and its terminal.
func (s *Sessions) stopShell(name session.Name) error {
	s.closeTerm(name.String())
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	if err := s.tmux.Kill(ctx, name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shells = slices.DeleteFunc(s.shells, func(sh term.Shell) bool { return sh.Name == name })
	s.announceShells()
	return nil
}

// hasBrowser says whether the session's agent gets a tab of the project's browser: when the project allows it, the
// browser and agent-browser are there, and the session has no brief.
func (s *Sessions) hasBrowser(proj project.Project, brief string) bool {
	return brief == "" && proj.BrowserOn() && s.browsers.AgentReady()
}

// workTree is the worktree orchestrator made for a thread, and the branch it holds. The zero value is a thread in the
// project folder.
type workTree struct{ branch, dir string }

// launchFor is what the session's agent starts with. The agent learns where it runs and the commands that reach the
// app, and about the browser when it has one; a session with a brief learns only the brief. A child learns the title
// of its parent, the thread it works for. A thread for an issue learns the issue's number. A task started by another
// session gets that session's handover last.
func launchFor(model project.Model, conversation, brief, handover string, access session.Access, browser bool, fork forkStart, tree workTree, parent string, issue int) agent.Launch {
	l := agent.Launch{Model: model.Model, Effort: model.Effort, Resume: conversation, Access: access}
	if brief != "" {
		l.Prompt = brief
		return l
	}
	l.Prompt = prompts.Session() + "\n\n" + prompts.Thread()
	if issue > 0 {
		l.Prompt += "\n\n" + prompts.Issue(issue)
	}
	if tree.branch != "" {
		l.Prompt += "\n\n" + prompts.Worktree(tree.branch, tree.dir)
	}
	if fork.of != "" {
		l.Prompt += "\n\n" + prompts.Fork(fork.title)
	}
	if parent != "" {
		l.Prompt += "\n\n" + prompts.Child(parent)
	}
	if browser {
		l.Browser = true
		l.Prompt += "\n\n" + prompts.Browser()
	}
	if handover != "" {
		l.Prompt += "\n\n" + prompts.Handover(handover)
	}
	return l
}
