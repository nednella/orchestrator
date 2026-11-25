// Package sessions tracks the agents in the private tmux server: their state, their pull requests and their clean-up.
package sessions

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"time"

	"github.com/nednella/agentos/desktop/internal/media"
	"github.com/nednella/agentos/desktop/internal/run"
	"github.com/nednella/agentos/desktop/internal/scoped"
	"github.com/nednella/agentos/desktop/internal/warn"
	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/agent"
	"github.com/nednella/agentos/internal/atomicfile"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/host"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/prompts"
	"github.com/nednella/agentos/internal/session"
	"github.com/nednella/agentos/internal/term"
	"github.com/nednella/agentos/internal/usage"
	"github.com/nednella/agentos/internal/util"
)

const (
	pollEvery    = 2 * time.Second
	stopSettle   = time.Second // Claude Code writes a turn's last reply just after the Stop hook
	tmuxTimeout  = 5 * time.Second
	defaultTitle = "agent"
	chatTitle    = "chat"
	endedSettle  = 15 * time.Second // a state file this fresh may belong to a session tmux has not listed yet
	startCols    = 120
	startRows    = 32
	hostWait     = 10 * time.Second // for a new session's host to listen
	hostStarting = 3 * time.Second  // for a host that is still starting to listen, before a later message is dropped
	oldDetail    = "Ran in a terminal before chats"
)

var (
	errNotSession = errors.New("the shell is not a session")
	errOldChat    = errors.New("This thread ran in a terminal before agentos showed chats")
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
	Chat        bool             `json:"chat"`
	Hosted      bool             `json:"hosted"` // the agent runs, or ran, under a host, whose chat log the chat service serves
	Action      string           `json:"action"` // the queue action the session was started with, "" when none
	ForkOf      string           `json:"forkOf"` // the thread the session was forked from, "" when none
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
	info       []term.Info              // the live sessions under a host
	old        []term.Info              // the live sessions that ran in a terminal before the host: shown settled and never touched
	shells     []session.Name           // every project's shell sessions
	known      map[string]term.Info     // the last info of every session seen, for when it ends
	ended      map[string]*endedSession // settled sessions, whose tmux session is gone, until dismissed
	nums       map[string]int           // the number each session shows, for this run of the app
	lastNum    map[string]int           // the last number given in each project
	dismissed  map[string]bool          // ended sessions the user dismissed: the poll must not bring them back
	records    map[string]session.Record
	cwds       map[string]string // the working directory each session's last hook reported
	heads      map[string]string // the HEAD of the project folder at each session's latest PreToolUse
	seen       map[string]session.State
	history    map[string][]session.Change
	loaded     bool                 // the first refresh is in, so later changes may raise attention
	synced     bool                 // the first refresh is synced, so a session seen from now on is a new one
	lastSent   map[string][]Session // by project key
	lastProjs  []Project
	lastIssues string
	life       *Lifecycle
	made       *madeTrees
	repoOf     func(dir string) string
	onIssues   func()
	wakes      map[string][]string     // prompts for sessions that were working or waiting on the user when their PR needed them, oldest first, each once
	readers    map[string]*reader      // each session's transcript reader
	spent      map[string]usage.Tokens // what each session's agent used
	held       map[string]int64        // the memory each live session held at the last sample
	memory     Memory
	serverPID  int // the tmux server's, as tmux gave it; asked again once ps no longer lists it
}

// ProjectList is the configured projects.
type ProjectList interface {
	List() []project.Project
	Remove(name string) (bool, error)
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
		project: o.Current, configured: o.Projects, tally: o.Tally, activity: o.Activity, closeTerm: o.CloseTerminal, evidence: o.Evidence, browsers: o.Browsers, awake: o.Awake, keepAwake: o.KeepAwake,
		repoOf: func(string) string { return "" }, onIssues: func() {},
		known: map[string]term.Info{}, ended: map[string]*endedSession{}, dismissed: map[string]bool{},
		nums: map[string]int{}, lastNum: map[string]int{},
		records:   map[string]session.Record{},
		cwds:      map[string]string{},
		heads:     map[string]string{},
		seen:      map[string]session.State{},
		history:   map[string][]session.Change{},
		wakes:     map[string][]string{},
		readers:   map[string]*reader{},
		spent:     map[string]usage.Tokens{},
		held:      map[string]int64{},
		stateRead: map[string]fileStamp{},
		made:      newMadeTrees(o.LocalDir),
		ctx:       context.Background(),
	}
}

// Start listens for hooks and polls tmux until ctx ends. A hook socket held by
// another agentos leaves only the poll.
func (s *Sessions) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	ln, err := bus.Listen(bus.SocketPath(s.stateDir), s.observe)
	if err != nil && !errors.Is(err, bus.ErrInUse) {
		return fmt.Errorf("listening for hooks: %w", err)
	}
	s.Refresh()
	go func() {
		if ln != nil {
			defer ln.Close()
		}
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
	infos, shells, err := s.tmux.ListAll(ctx)
	if err != nil {
		if parent.Err() == nil {
			s.warn.Report("tmux", util.FirstLine(err.Error()))
		}
		return
	}
	s.warn.Clear("tmux")
	stamps := map[string]fileStamp{}
	records := bus.ReadAll(s.stateDir, func(name string, info fs.FileInfo) bool {
		stamps[name] = stampOf(info)
		return s.stateRead[name] == stamps[name]
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.info, s.old = nil, nil
	for _, in := range infos {
		if in.Hosted {
			s.info = append(s.info, in)
		} else {
			s.old = append(s.old, in)
		}
	}
	s.shells = shells
	for _, in := range s.info {
		if name := in.Name.String(); s.ended[name] != nil {
			records[name] = s.revive(name, in)
		}
	}
	for name, rec := range records {
		switch {
		case s.isOld(name): // its hooks are dropped; once it is gone, its file makes its settled row
		case slices.ContainsFunc(s.info, func(i term.Info) bool { return i.Name.String() == name }):
			s.take(rec)
		default:
			s.adoptEnded(name, rec)
		}
	}
	s.markRead(records, stamps, s.info)
	s.number()
	s.loaded = true
	s.sync()
	s.synced = true
}

// fileStamp tells whether a file changed since it was read.
type fileStamp struct {
	mod  time.Time
	size int64
}

func stampOf(info fs.FileInfo) fileStamp { return fileStamp{info.ModTime(), info.Size()} }

// markRead keeps the stamps the state files had when they were read, so the next refresh reads a file again only once
// it changes. A file of a session that is neither live under a host nor ended yet, such as one still settling or an
// old terminal session's, is read each time. It needs createMu and mu.
func (s *Sessions) markRead(records map[string]session.Record, stamps map[string]fileStamp, infos []term.Info) {
	for name := range records {
		if s.ended[name] != nil || slices.ContainsFunc(infos, func(i term.Info) bool { return i.Name.String() == name }) {
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
	if err := s.writeState(rec); err != nil {
		fmt.Fprintf(os.Stderr, "agentos: saving a thread that runs again: %v\n", err)
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
	rec.Identity = keptStarts(identityOf(in), rec)
	rec.Woke = at.UnixMilli()
	history := s.history[id]
	if len(history) == 0 {
		history = rec.Changes
	}
	rec.Changes = session.AddChange(history, session.Change{State: session.Idle, At: at.UnixMilli()})
	return rec
}

// writeState writes the thread's record under the lock its hooks take.
func (s *Sessions) writeState(rec session.Record) error {
	unlock, err := bus.LockState(s.stateDir, rec.Session)
	if err != nil {
		return err
	}
	defer unlock()
	return bus.WriteState(s.stateDir, rec)
}

// endedSession is a session that is gone but whose row stays, settled, until dismissed.
type endedSession struct {
	info term.Info
	rec  session.Record
	at   time.Time
}

// persist saves what the row needs to outlive the app. It needs mu.
func (e *endedSession) record() session.Record {
	rec := e.rec
	rec.State, rec.SettledAt = session.Settled, e.at.UnixMilli()
	rec.Identity = keptStarts(identityOf(e.info), e.rec)
	return rec
}

// identityOf is what the session was started as, with its tmux session's start as both its first and latest start.
func identityOf(in term.Info) session.Identity {
	id := session.Identity{Title: in.Title, Chat: in.Chat, Hosted: in.Hosted, Action: in.Action, Model: in.Model, Effort: in.Effort, Path: in.Path, Fork: in.Fork}
	id.Issue, _ = strconv.Atoi(in.Issue)
	if !in.Created.IsZero() {
		id.Created = in.Created.UnixMilli()
		id.Woke = id.Created
	}
	return id
}

// keptStarts is id with the first and latest start rec holds: a wake leaves the first start where it was and sets the
// latest itself, and tmux dates a session only to the second.
func keptStarts(id session.Identity, rec session.Record) session.Identity {
	id.Created, id.Woke = cmp.Or(rec.Created, id.Created), cmp.Or(rec.Woke, id.Woke)
	return id
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

// saveIdentity writes what a live session was started as into its state file, which hooks keep. Without it, a
// session that ends while the app is closed comes back with no title, kind or issue.
func (s *Sessions) saveIdentity(in term.Info) {
	id := in.Name.String()
	unlock, err := bus.LockState(s.stateDir, id)
	if err != nil {
		return
	}
	defer unlock()
	rec, err := bus.ReadState(s.stateDir, id)
	if err != nil {
		return
	}
	want := keptStarts(identityOf(in), rec)
	if rec.Identity == want {
		return
	}
	if rec.Session == "" {
		// Dated when the session was created, so its first hook is not taken for one that came too late.
		rec = session.Record{Session: id, State: session.Idle, At: in.Created}
	}
	rec.Identity = want
	if err := bus.WriteState(s.stateDir, rec); err != nil {
		fmt.Fprintf(os.Stderr, "agentos: saving a session's identity: %v\n", err)
	}
}

// adoptEnded takes in a state file whose tmux session is gone, as a settled row. One that says ended, an older
// build's, is written back as settled. It needs mu.
func (s *Sessions) adoptEnded(name string, rec session.Record) {
	n, err := session.ParseName(name)
	if err != nil {
		return
	}
	if s.dismissed[name] {
		_ = bus.RemoveState(s.stateDir, name) // a hook that ran late wrote it again
		return
	}
	if e, ok := s.ended[name]; ok {
		if rec.Title == "" && e.info.Title != "" { // a late hook overwrote the file
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
	rec.Changes = session.SettledChanges(rec.Changes)
	info := term.Info{Name: n, Title: rec.Title, Path: rec.Path, Model: rec.Model, Effort: rec.Effort, Chat: rec.Chat, Hosted: rec.Hosted, Action: rec.Action, Fork: rec.Fork}
	if rec.Issue > 0 {
		info.Issue = strconv.Itoa(rec.Issue)
	}
	if started := cmp.Or(rec.Woke, rec.Created); started > 0 {
		info.Created = time.UnixMilli(started)
	}
	e := &endedSession{info: info, rec: rec, at: at}
	s.ended[name] = e
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
	if _, ok := s.ended[id]; ok || s.dismissed[id] {
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
	if rec.State == session.Settled && !rec.At.IsZero() {
		at = rec.At
	}
	e := &endedSession{info: info, rec: rec, at: at}
	s.ended[id] = e
	if err := bus.WriteState(s.stateDir, e.record()); err != nil {
		fmt.Fprintf(os.Stderr, "agentos: saving a settled session: %v\n", err)
	}
	if h := s.history[id]; len(h) == 0 || h[len(h)-1].State != session.Settled {
		s.history[id] = append(h, session.Change{State: session.Settled, At: at.UnixMilli()})
	}
}

// number gives each session that has no number the next one of its project, oldest first.
// It needs mu.
func (s *Sessions) number() {
	var fresh []term.Info
	for _, in := range slices.Concat(s.info, s.old) {
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
		s.lastNum[in.Name.Project]++
		s.nums[in.Name.String()] = s.lastNum[in.Name.Project]
	}
}

// drop forgets a settled session for good. It needs mu.
func (s *Sessions) drop(id string) {
	delete(s.ended, id)
	delete(s.history, id)
	delete(s.known, id)
	delete(s.nums, id)
	delete(s.readers, id)
	delete(s.spent, id)
	s.dismissed[id] = true
	s.life.forget(id)
	_ = bus.RemoveState(s.stateDir, id)
	go s.evidence.Purge(id)
	go s.browsers.Forget(s.ctx, id)
	if name, err := session.ParseName(id); err == nil {
		go os.RemoveAll(s.tmpDir(name))
		_ = os.Remove(s.handoverPath(name))
		_ = os.Remove(s.usageMarkPath(name))
		_ = os.Remove(s.chatLogPath(name))
		// A shell's name has no token, and would name the folder of every thread's images.
		if name.Token != "" {
			go os.RemoveAll(s.chatMediaDir(name))
		}
		_ = os.Remove(s.hostSocket(name))
	}
}

// tmpDir is the session's own TMPDIR, and Claude's scratchpad. It sits in a folder the agent may use
// without asking, so the screenshots and scratch files it makes there need no approval, and it goes with the session.
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

// Dismiss removes a settled session's row. An old terminal session that still runs only leaves the list: what it
// keeps is its own until it ends.
func (s *Sessions) Dismiss(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isOld(id) {
		s.dismissed[id] = true
		s.sync()
		return nil
	}
	if _, ok := s.ended[id]; !ok {
		return fmt.Errorf("Thread %s has not settled", id)
	}
	s.drop(id)
	s.sync()
	return nil
}

// observe folds in a hook's record. The hooks of an old terminal session are dropped before anything is recorded, so
// it raises nothing.
func (s *Sessions) observe(rec session.Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isOld(rec.Session) {
		return
	}
	if rec.Event == "UserPromptSubmit" {
		s.record(rec.Session, activity.Event{At: rec.At.UnixMilli(), Kind: activity.Prompt})
	}
	s.take(rec)
	s.sync()
}

// take folds a record in. It needs mu.
func (s *Sessions) take(rec session.Record) {
	if _, ended := s.ended[rec.Session]; ended || s.dismissed[rec.Session] {
		return
	}
	if prev, ok := s.records[rec.Session]; ok && !rec.At.After(prev.At) {
		return
	}
	s.records[rec.Session] = rec
	s.followBranch(rec)
	// Each hook moves the row's cost. The event log gets a turn's usage once the turn is over, and the
	// first refresh's usage at once: it catches up on turns while the app was closed.
	if rec.Transcript != "" {
		if endsTurn(rec.Event) {
			time.AfterFunc(stopSettle, func() { s.countUsage(rec.Session, rec.Transcript, rec.AgentModel, true) })
		} else {
			go s.countUsage(rec.Session, rec.Transcript, rec.AgentModel, rec.Event == "SessionEnd" || !s.loaded)
		}
	}
}

// endsTurn says whether a record's event ends the agent's turn: its Stop hook, or the app's TurnError.
func endsTurn(event string) bool { return event == "Stop" || event == session.TurnError }

// followBranch has the lifecycle look at the branch of a session, when the agent moved,
// finished a turn or switched the branch of the project folder, and read its git status again after
// each tool call. It needs mu.
func (s *Sessions) followBranch(rec session.Record) {
	if s.isChat(rec.Session) || rec.Cwd == "" {
		return
	}
	moved := rec.Cwd != s.cwds[rec.Session] || endsTurn(rec.Event)
	s.cwds[rec.Session] = rec.Cwd
	for _, in := range s.info {
		if in.Name.String() != rec.Session || in.Fork != "" {
			continue
		}
		proj, ok := s.projectOf(in)
		if !ok {
			continue
		}
		if switched := s.switched(rec, proj.Dir); moved || switched {
			go s.life.learn(s.ctx, rec.Session, proj, rec.Cwd, endsTurn(rec.Event), switched)
		}
		if rec.Event == "PostToolUse" {
			go s.life.restatus(s.ctx, rec.Session, proj)
		}
	}
}

// switched says whether the session's tool call changed the branch of the project folder. The owner
// works in that folder too, so a branch they check out there must not become the session's. The HEAD
// file is read, not git run: a PreToolUse must be read before the tool starts. It needs mu.
func (s *Sessions) switched(rec session.Record, dir string) bool {
	if rec.Event != "PreToolUse" && rec.Event != "PostToolUse" {
		return false
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	head := string(data)
	if rec.Event == "PreToolUse" {
		s.heads[rec.Session] = head
		return false
	}
	before, ok := s.heads[rec.Session]
	return ok && head != before
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
		state, at := session.Idle, time.Now()
		if rec, ok := s.records[id]; ok {
			state, at = rec.State, rec.At
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
		replied := known && prev == session.Working && state == session.Idle && rec.Event == "Stop"
		if s.loaded {
			switch {
			case state == session.Waiting:
				attention = append(attention, [2]string{id, "waiting"})
			case replied && !opened:
				attention = append(attention, [2]string{id, "replied"})
			}
		}
		if s.loaded {
			ended := !in.Chat && s.tally.Closed(id, at)
			opens := !in.Chat && state == session.Waiting
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
			go s.closeBrowser(id)
			s.endSession(id, time.Now())
			delete(s.seen, id)
			delete(s.records, id)
		}
	}
	if waitsChanged {
		s.emit("stats", nil)
	}
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
	e := activity.Event{At: time.Now().UnixMilli(), Kind: activity.ChatStart}
	if !in.Chat {
		e.Kind, e.Model, e.Effort = activity.SessionStart, in.Model, in.Effort
		e.Issue, _ = strconv.Atoi(in.Issue)
	}
	if rec := s.records[id]; rec.Created > 0 && rec.Woke > rec.Created {
		e.Label = activity.Resumed
	}
	s.record(id, e)
}

// adoptHistory takes the timeline the hooks kept for a session first seen, and records the working
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

// record adds an event of the session to the activity record. Of a chat, only its start and usage are recorded.
func (s *Sessions) record(id string, e activity.Event) {
	if e.Kind != activity.ChatStart && e.Kind != activity.ChatUsage && s.isChat(id) {
		return
	}
	e.Session = id
	s.activity.Record(session.ProjectKey(id), e)
}

// isChat says whether the session, live or ended, is a chat, which the stats only count and cost. It needs mu.
func (s *Sessions) isChat(id string) bool {
	if e, ok := s.ended[id]; ok {
		return e.info.Chat
	}
	return s.known[id].Chat || slices.ContainsFunc(s.info, func(i term.Info) bool { return i.Name.String() == id && i.Chat })
}

// holdAwake keeps the Mac awake while a session of a project that keeps awake is working. It needs mu.
func (s *Sessions) holdAwake() {
	if s.awake == nil {
		return
	}
	working := slices.ContainsFunc(s.info, func(in term.Info) bool {
		return s.records[in.Name.String()].State == session.Working && s.keepAwake(in.Name.Project)
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
		if v.Project == s.project.Key() && v.Issue > 0 && v.State != session.Settled && !v.Chat && v.ForkOf == "" {
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
			state := session.Idle
			if rec, ok := s.records[in.Name.String()]; ok {
				state = rec.State
			}
			switch state {
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
		ss := session.Session{Name: in.Name, N: s.nums[in.Name.String()], State: session.Idle}
		if rec, ok := s.records[in.Name.String()]; ok {
			ss.State, ss.At, ss.Detail = rec.State, rec.At, rec.Detail
		}
		rail = append(rail, ss)
	}
	for _, e := range s.ended {
		byName[e.info.Name] = e.info
		rail = append(rail, session.Session{Name: e.info.Name, N: s.nums[e.info.Name.String()], State: session.Settled, At: e.at})
	}
	for _, in := range s.old {
		if s.dismissed[in.Name.String()] {
			continue
		}
		byName[in.Name] = in
		rail = append(rail, session.Session{Name: in.Name, N: s.nums[in.Name.String()], State: session.Settled, At: in.Created})
	}
	session.Sort(rail)
	projs := s.projects()
	out := make([]Session, 0, len(rail))
	for _, ss := range rail {
		in := byName[ss.Name]
		id := ss.Name.String()
		issue, _ := strconv.Atoi(in.Issue)
		created, woke := s.startsOf(id, in)
		view := Session{
			ID: id, Project: ss.Name.Project, N: ss.N, Title: cmp.Or(in.Title, defaultTitle),
			State: ss.State, Detail: ss.Detail,
			CreatedAt: created.UnixMilli(), WokeAt: woke.UnixMilli(), Issue: issue, Chat: in.Chat, Hosted: in.Hosted, Action: in.Action, ForkOf: in.Fork, Model: in.Model, Effort: in.Effort, Evidence: s.evidence.Count(id), Browser: s.browsers.Has(id),
			History: s.history[id], // shared, not copied: a change in a history is never written over
		}
		if !ss.At.IsZero() {
			view.LastEventAt = ss.At.UnixMilli()
		}
		if ss.State == session.Settled {
			view.SettledAt = ss.At.UnixMilli()
		}
		if in.Hosted {
			f := s.life.fields(id)
			view.Branch, view.Worktree, view.PR, view.Git = f.branch, f.worktree, f.pr, f.git
			view.PRAttention, view.Cleanup, view.CleanupReason = f.attention, f.cleanup, f.reason
			if view.Branch == "" && issue > 0 {
				if p, ok := projectIn(projs, in); ok {
					view.Branch = p.BranchFor(issue)
				}
			}
		} else {
			view.Detail = oldDetail
		}
		if spent, ok := s.spent[id]; ok {
			view.Tokens, view.Cost = spent.Total(), spent.Dollars()
		}
		view.Memory = s.held[id]
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

// Title is the title of the live session with the given id, or "" when there is none.
func (s *Sessions) Title(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range slices.Concat(s.info, s.old) {
		if in.Name.String() == id {
			return cmp.Or(in.Title, defaultTitle)
		}
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
	for _, in := range slices.Concat(s.info, s.old) {
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

// Forget removes a configured project and ends its sessions, moving to another one if it was current.
func (s *Sessions) Forget(name string) (project.Project, error) {
	if live := s.liveIssueSessions(project.Project{Name: name}.Key()); len(live) > 0 {
		return project.Project{}, fmt.Errorf("cannot remove %q: it has live issue sessions (%s), which need its branch pattern and clean-up command; end them first", name, strings.Join(live, ", "))
	}
	removed, err := s.configured.Remove(name)
	if err != nil {
		return project.Project{}, err
	}
	s.mu.Lock()
	current := s.project.Name == name
	s.mu.Unlock()
	// The current project can be missing from the config (the folder the app started in); removing
	// it still means moving away.
	if !removed && !current {
		return project.Project{}, fmt.Errorf("%q is not in the config", name)
	}
	if err := s.settleProject(project.Project{Name: name}.Key()); err != nil {
		return project.Project{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.project.Name == name {
		for _, p := range s.projects() {
			if p.Key() != s.project.Key() {
				s.project = p
				break
			}
		}
	}
	s.sync()
	return s.project, nil
}

// settleProject settles every running session of the project. It takes mu.
func (s *Sessions) settleProject(key string) error {
	s.mu.Lock()
	var ids []string
	for _, in := range s.info {
		if in.Name.Project == key {
			ids = append(ids, in.Name.String())
		}
	}
	s.mu.Unlock()
	var errs []error
	for _, id := range ids {
		errs = append(errs, s.Settle(id))
	}
	return errors.Join(errs...)
}

// liveIssueSessions names the running sessions of the project that work on an issue. It takes mu.
func (s *Sessions) liveIssueSessions(key string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, in := range s.info {
		if issue, _ := strconv.Atoi(in.Issue); issue > 0 && in.Name.Project == key && !in.Chat {
			out = append(out, fmt.Sprintf("session %d for #%d", s.nums[in.Name.String()], issue))
		}
	}
	return out
}

// liveIssueSession is the id of the running task, or chat, of the project for the issue, "" when none. A fork only
// shows its source's issue. An old terminal session counts: a second session would claim the branch it works on, and
// clean it up under it. It takes mu.
func (s *Sessions) liveIssueSession(key string, issue int, chat bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range slices.Concat(s.info, s.old) {
		if in.Name.Project == key && in.Issue == strconv.Itoa(issue) && in.Chat == chat && in.Fork == "" {
			return in.Name.String()
		}
	}
	return ""
}

// IssueSessions maps issue numbers to the live task started for each, in the current project.
func (s *Sessions) IssueSessions() map[int]string {
	out := map[int]string{}
	for _, v := range s.ListOf(s.Current().Key()) {
		if v.Issue > 0 && v.State != session.Settled && !v.Chat && v.ForkOf == "" {
			out[v.Issue] = v.ID
		}
	}
	return out
}

// Create starts the agent in the current project. A non-empty text is given to the agent through its host: sent when
// send is set, else left as a draft in its chat.
func (s *Sessions) Create(title, text string, send bool, issue int) (Session, error) {
	return s.createIn(s.Current(), title, text, send, issue, project.Model{}, "", "", false, "", forkStart{})
}

// CreateChat starts a chat in the current project: an agent that only reads, for questions about the project. A
// non-empty text is given to it as Create gives it.
func (s *Sessions) CreateChat(title, text string, send bool) (Session, error) {
	return s.createIn(s.Current(), cmp.Or(strings.TrimSpace(title), chatTitle), text, send, 0, project.Model{}, "", "", true, "", forkStart{})
}

// StartOptions is what the owner chose for a new thread: the first message, sent as it is, and the model and effort,
// which win over every other choice. Empty fields leave the choice to the defaults.
type StartOptions struct {
	Prompt string
	Model  project.Model
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

// CreateWith starts a thread in the current project as Create or CreateChat does, with the owner's first message
// sent through the same path and the owner's model and effort. A blank title is the first line of the prompt.
func (s *Sessions) CreateWith(title string, chat bool, o StartOptions) (Session, error) {
	p, err := s.Provider()
	if err != nil {
		return Session{}, err
	}
	if err := p.ValidModel(o.Model.Model, o.Model.Effort); err != nil {
		return Session{}, err
	}
	title = cmp.Or(strings.TrimSpace(title), titleFromPrompt(o.Prompt))
	if chat {
		title = cmp.Or(title, chatTitle)
	}
	return s.createIn(s.Current(), title, strings.TrimSpace(o.Prompt), true, 0, o.Model, "", "", chat, "", forkStart{})
}

// CreateBriefed is Create for a session with a job of its own: brief is added to the agent's system prompt, and the
// project says whether text is sent.
func (s *Sessions) CreateBriefed(title, text, brief string) (Session, error) {
	proj := s.Current()
	return s.createIn(proj, title, fromApp(text), proj.SendsPrompt(), 0, project.Model{}, brief, "", false, "", forkStart{})
}

// IssueStart is what a start of an issue did.
type IssueStart int

const (
	Kept    IssueStart = iota // gave back the issue's live thread, with nothing given to it
	Woken                     // woke the issue's settled thread and gave it the command and the handover
	Created                   // started a new thread
)

// CreateIssue starts the agent for an issue, in a task or a chat as the action says, and gives it text; the project
// says whether that is sent. The labels and the action pick the model it takes. A handover from the session that
// asked for this one is added to the agent's system prompt. An issue that has a live session of that kind gets that
// session back with nothing given to it; one whose thread of that kind has settled gets that thread woken, with the
// handover in place of the one it had when one is given, and given text. The owner's prompt follows the action's
// command in the same first message, and the owner's model and effort win over the labels' and the action's; a
// thread that is live or woken keeps its own.
func (s *Sessions) CreateIssue(title, text string, issue int, action project.Action, labels []string, handover string, o StartOptions) (view Session, did IssueStart, err error) {
	proj := s.Current()
	p, err := s.Provider()
	if err != nil {
		return Session{}, Kept, err
	}
	if err := p.ValidModel(o.Model.Model, o.Model.Effort); err != nil {
		return Session{}, Kept, err
	}
	model, _ := project.Pick(action, labels, p.ValidModel)
	model = project.Model{Model: cmp.Or(o.Model.Model, model.Model), Effort: cmp.Or(o.Model.Effort, model.Effort)}
	text = fromApp(text)
	if extra := strings.TrimSpace(o.Prompt); extra != "" {
		text = strings.TrimSpace(text + " " + extra)
	}
	view, did, err = s.createForIssue(proj, title, text, proj.SendsPrompt(), issue, model, handover, action.Chat(), action.Name)
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
		log.Printf("agentos: the host of thread %s did not listen: %v", id, err)
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
// it runs before Claude starts, so the first hook builds on it; a launch that fails writes back the settled one.
func (s *Sessions) startAgain(name session.Name, handover string) error {
	id := name.String()
	s.createMu.Lock()
	defer s.createMu.Unlock()

	s.mu.Lock()
	e, settled := s.ended[id]
	var in term.Info
	if settled {
		in = e.info
	}
	old := s.isOld(id) || settled && !in.Hosted
	proj, known := s.projectOf(in)
	conversation := s.conversationOf(id)
	source, _ := s.infoOf(in.Fork)
	ctx := s.ctx
	s.mu.Unlock()
	switch {
	case old:
		return fmt.Errorf("Thread %s ran in a terminal and cannot be woken", id)
	case !settled:
		return fmt.Errorf("Thread %s is not settled", id)
	case !known:
		return fmt.Errorf("The project of thread %s is gone", id)
	}

	p, err := s.Provider()
	if err != nil {
		return err
	}
	if err := agent.CheckInstalled(p); err != nil {
		return err
	}
	model := fill(p, proj.Dir, project.Model{Model: in.Model, Effort: in.Effort})
	in.Created, in.Model, in.Effort = time.Now(), model.Model, model.Effort
	s.mu.Lock()
	rec, before := s.woken(id, in), e.record()
	s.mu.Unlock()
	if err := s.writeState(rec); err != nil {
		return fmt.Errorf("saving the thread's state: %w", err)
	}
	issue, _ := strconv.Atoi(in.Issue)
	tctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	fork := forkStart{of: in.Fork, title: cmp.Or(source.Title, defaultTitle)}
	if err := s.launch(tctx, p, name, cmp.Or(in.Title, defaultTitle), proj, cmp.Or(in.Path, proj.Dir), issue, model, conversation, "", cmp.Or(handover, s.handoverOf(id)), in.Chat, in.Action, fork); err != nil {
		_ = s.writeState(before)
		return err
	}
	s.life.unretire(id)
	if branch := proj.BranchFor(issue); issue > 0 && branch != "" && !in.Chat && in.Fork == "" {
		s.life.claim(proj, id, branch)
	}
	s.mu.Lock()
	delete(s.ended, id)
	s.records[id] = rec
	s.info = append(s.info, in)
	s.sync()
	s.mu.Unlock()
	if issue > 0 {
		s.life.ensureWatchers(ctx)
	}
	return nil
}

// Provider is the provider every session starts with.
func (s *Sessions) Provider() (agent.Provider, error) { return s.agents.Provider() }

// createForIssue starts a task or a chat for the issue, unless the project has a live one of that kind for it, which
// it names, or a settled one, which it wakes with the handover and gives text to, and names.
func (s *Sessions) createForIssue(proj project.Project, title, text string, send bool, issue int, model project.Model, handover string, chat bool, action string) (Session, IssueStart, error) {
	s.issueMu.Lock()
	defer s.issueMu.Unlock()
	if live := s.liveIssueSession(proj.Key(), issue, chat); live != "" {
		return Session{ID: live}, Kept, nil
	}
	if settled := s.settledIssueThread(proj.Key(), issue, chat); settled != "" {
		ch, err := s.wakeOrJoin(settled, handover)
		if err != nil {
			return Session{}, Kept, err
		}
		if text != "" {
			if err := toHost(s.context(), ch.Socket, text, send, hostWait); err != nil {
				log.Printf("agentos: thread %s did not get the issue's command: %v", settled, err)
				return Session{}, Woken, fmt.Errorf("Thread %s woke but did not take the issue's command", settled)
			}
		}
		return Session{ID: settled}, Woken, nil
	}
	view, err := s.createIn(proj, title, text, send, issue, model, "", handover, chat, action, forkStart{})
	return view, Created, err
}

// settledIssueThread is the id of the settled task, or chat, of the project for the issue that ran under a host, the
// one that settled last, "" when none. A fork only shows its source's issue. It takes mu.
func (s *Sessions) settledIssueThread(key string, issue int, chat bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, at := "", time.Time{}
	for name, e := range s.ended {
		in := e.info
		if in.Name.Project == key && in.Issue == strconv.Itoa(issue) && in.Chat == chat && in.Fork == "" && in.Hosted && (id == "" || e.at.After(at)) {
			id, at = name, e.at
		}
	}
	return id
}

// fromApp marks text the app writes to the agent, so the agent does not take it for the owner's. A slash command
// stays as it is: Claude Code runs one only when it starts the prompt.
func fromApp(text string) string {
	if text == "" || strings.HasPrefix(text, "/") {
		return text
	}
	return "APPLICATION_PROMPT: " + text
}

// createIn is Create in the project given, which need not be the current one, with the model given. A brief or a
// handover is added to the agent's system prompt. A fork claims no branch.
func (s *Sessions) createIn(proj project.Project, title, text string, send bool, issue int, model project.Model, brief, handover string, chat bool, action string, fork forkStart) (Session, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()

	p, err := s.Provider()
	if err != nil {
		return Session{}, err
	}
	if err := agent.CheckInstalled(p); err != nil {
		return Session{}, err
	}
	model = fill(p, proj.Dir, model)
	name, ctx := s.reserve(proj)
	restore := func() {}
	if fork.at != "" {
		if restore, err = s.copyChat(fork, name); err != nil {
			return Session{}, err
		}
	}
	created := time.Now()
	title = cmp.Or(strings.TrimSpace(title), defaultTitle)
	tctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	if err := s.launch(tctx, p, name, title, proj, proj.Dir, issue, model, fork.conversation, brief, handover, chat, action, fork); err != nil {
		restore()
		return Session{}, err
	}
	view := s.register(name, title, proj, issue, model, created, chat, action, fork.of)
	if branch := proj.BranchFor(issue); issue > 0 && branch != "" && !chat && fork.of == "" {
		s.life.claim(proj, name.String(), branch)
	}
	if text != "" {
		go s.giveHost(ctx, name, text, send)
	}
	if issue > 0 {
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
	p, err := s.Provider()
	if err != nil {
		return project.Model{}
	}
	return fill(p, proj.Dir, project.Model{Model: model})
}

// forkStart is what a fork starts from: the thread it forks and that thread's title, which a woken fork keeps, and for a
// new fork that thread's conversation, the message it forks at and its chat up to that message.
type forkStart struct {
	of, title, conversation, at string
	events                      []chat.Event
}

// Fork starts a thread that continues the conversation of the thread id, live or ended, as a new conversation that
// ends at the message at: the uuid of a reply's last block. The fork has the same kind, issue, model, effort and
// handover, starts in the same folder, and its chat starts as the source's does up to that reply. The issue is
// only shown: the fork claims no branch and hears no pull request.
func (s *Sessions) Fork(id, at string) (Session, error) {
	s.mu.Lock()
	in, found := s.infoOf(id)
	conversation := s.conversationOf(id)
	proj, known := s.projectOf(in)
	s.mu.Unlock()
	switch {
	case !found:
		return Session{}, fmt.Errorf("No thread %s", id)
	case !in.Hosted:
		return Session{}, errors.New("Only a thread with a chat can be forked")
	case conversation == "":
		return Session{}, errors.New("This thread has no conversation to fork yet")
	case !known:
		return Session{}, fmt.Errorf("The project of thread %s is gone", id)
	}
	events, err := chat.ReadLog(s.chatLogPath(in.Name), 0)
	if err != nil {
		log.Printf("agentos: reading the chat log of %s: %v", id, err)
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
	issue, _ := strconv.Atoi(in.Issue)
	title, model := cmp.Or(in.Title, defaultTitle)+" (fork)", project.Model{Model: in.Model, Effort: in.Effort}
	return s.createIn(proj, title, "", false, issue, model, "", s.handoverOf(id), in.Chat, "", forkStart{of: id, title: cmp.Or(in.Title, defaultTitle), conversation: conversation, at: at, events: events[:cut+1]})
}

// copyChat writes the fork's chat log before the fork's host opens it: the source's events, with the turn they stop in
// ended at the last of them, and copies the images they show into the fork's folder. Drafts are left out, and so are
// queued messages and those not sent, which only the source's host held: a message Claude took in is logged again as
// a user event. restore removes what it wrote, for a fork that did not start.
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
// folder, by its name: a log an older build carried over on a resume names the folder of the thread that sent it.
func (s *Sessions) copyEvents(chatLog *chat.Log, events []chat.Event, from, to session.Name) error {
	for _, e := range events {
		switch e.Kind {
		case chat.KindDraft, chat.KindQueued, chat.KindUnsent:
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

// launch starts the agent of the session under its host in tmux, in dir.
func (s *Sessions) launch(ctx context.Context, p agent.Provider, name session.Name, title string, proj project.Project, dir string, issue int, model project.Model, conversation, brief, handover string, chat bool, action string, fork forkStart) error {
	env := []string{
		"AGENTOS_SESSION=" + name.String(),
		"AGENTOS_SOCKET=" + bus.SocketPath(s.stateDir),
	}
	if issue > 0 {
		env = append(env, "AGENTOS_ISSUE="+strconv.Itoa(issue))
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
			log.Printf("agentos: session %s starts without its browser: %v", name, err)
			browser = false
		}
		env = append(env, agentEnv...)
	}
	l := launchFor(model, conversation, brief, handover, chat, browser, fork)
	l.TmpDir, l.ForkAt = tmp, fork.at
	if err := p.Prepare(l); err != nil {
		return err
	}
	env = append(env, p.Env(l)...)
	argv, err := s.hosted(name, p.Command(l))
	if err != nil {
		return err
	}
	return s.start(ctx, name, title, dir, env, argv, issue, model, chat, action, fork.of)
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

// register adds the new session to the list and returns its row. forkOf is the thread it was forked from, if any. It
// takes mu.
func (s *Sessions) register(name session.Name, title string, proj project.Project, issue int, model project.Model, created time.Time, chat bool, action, forkOf string) Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	in := term.Info{Name: name, Title: title, Path: proj.Dir, Model: model.Model, Effort: model.Effort, Chat: chat, Hosted: true, Action: action, Fork: forkOf, Created: created}
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

func (s *Sessions) start(ctx context.Context, name session.Name, title, dir string, env, argv []string, issue int, model project.Model, chat bool, action, forkOf string) error {
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
	if chat {
		if err := tag(s.tmux.SetChat(ctx, name)); err != nil {
			return err
		}
	}
	if err := tag(s.tmux.SetHosted(ctx, name)); err != nil {
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
	if model != (project.Model{}) {
		return tag(s.tmux.SetModel(ctx, name, model.Model, model.Effort))
	}
	return nil
}

// giveHost gives text to the agent through the session's host once its socket takes connections: sent when send is set,
// else left as a draft in the chat.
func (s *Sessions) giveHost(ctx context.Context, name session.Name, text string, send bool) {
	if err := toHost(ctx, s.hostSocket(name), text, send, hostWait); err != nil {
		log.Printf("agentos: session %s did not get its first prompt: %v", name, err)
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
		log.Printf("agentos: session %s did not get a prompt: %v", name, err)
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
	if holdsPrompts(s.records[t.id].State) {
		if !slices.Contains(s.wakes[t.id], prompt) {
			s.wakes[t.id] = append(s.wakes[t.id], prompt)
		}
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.send(ctx, name, prompt, true)
}

// wakeFor wakes the settled thread of t for a pull request's command and says whether it runs: a thread with no issue
// stays settled, and so does one whose issue has another live thread, which keeps the issue's branch.
func (s *Sessions) wakeFor(t target) bool {
	if t.issue == 0 {
		return false
	}
	s.issueMu.Lock()
	defer s.issueMu.Unlock()
	if live := s.liveIssueSession(t.proj.Key(), t.issue, false); live != "" {
		return live == t.id
	}
	if _, err := s.wakeOrJoin(t.id, ""); err != nil {
		fmt.Fprintf(os.Stderr, "agentos: waking thread %s for #%d: %v\n", t.id, t.issue, err)
		return false
	}
	return true
}

func holdsPrompts(state session.State) bool {
	return state == session.Working || state == session.Waiting
}

// deliverWakes sends the prompts held back while their sessions worked or waited on the user, in order. A thread that
// stopped by itself meanwhile is woken for them; Settle drops those of a thread the owner or a clean-up settled. It
// needs mu.
func (s *Sessions) deliverWakes() {
	for id, prompts := range s.wakes {
		_, live := s.seen[id]
		switch {
		case !live:
			delete(s.wakes, id)
			ctx := s.ctx
			go func() {
				t, err := s.target(id)
				if err != nil {
					return
				}
				for _, prompt := range prompts {
					s.wake(ctx, t, prompt)
				}
			}()
		case !holdsPrompts(s.records[id].State):
			delete(s.wakes, id)
			ctx, name := s.ctx, s.known[id].Name
			go func() {
				for _, prompt := range prompts {
					s.send(ctx, name, prompt, true)
				}
			}()
		}
	}
}

// Settle stops the session's tmux session, whose host stops Claude; its row stays, settled.
func (s *Sessions) Settle(id string) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if name.IsShell() {
		return errNotSession
	}
	s.mu.Lock()
	_, ended := s.ended[id]
	old := s.isOld(id)
	s.mu.Unlock()
	switch {
	case ended:
		return fmt.Errorf("Thread %s has settled", id)
	case old:
		return fmt.Errorf("Thread %s runs in an old terminal; end it there", id)
	}
	s.closeTerm(id)
	// Dropped before the kill, so a Refresh that sees the session gone first does not wake it for them.
	s.mu.Lock()
	delete(s.wakes, id)
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	if err := s.tmux.Kill(ctx, name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// The session is gone: sync turns it into a settled row.
	s.info = slices.DeleteFunc(s.info, func(i term.Info) bool { return i.Name == name })
	s.sync()
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
	s.mu.Lock()
	old := s.isOld(id)
	s.mu.Unlock()
	if old {
		return fmt.Errorf("Thread %s runs in an old terminal; it keeps its title", id)
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

// targetOrErr needs mu.
func (s *Sessions) targetOrErr(in term.Info, ended bool) (target, error) {
	t, ok := s.targetOf(in, s.projects())
	if !ok {
		return target{}, fmt.Errorf("session %s has no issue or branch to clean up after", in.Name)
	}
	t.ended = ended
	return t, nil
}

// targetOf finds the session's project in projs. A chat, a fork or an old terminal session has no branch or PR. It
// needs mu.
func (s *Sessions) targetOf(in term.Info, projs []project.Project) (target, bool) {
	proj, ok := projectIn(projs, in)
	if !ok || in.Chat || in.Fork != "" || !in.Hosted {
		return target{}, false
	}
	issue, _ := strconv.Atoi(in.Issue)
	id := in.Name.String()
	branches, owned := s.life.branchesOf(proj, id)
	if fallback := proj.BranchFor(issue); issue > 0 && fallback != "" {
		owned = append(owned, fallback)
		if len(branches) == 0 {
			branches = []string{fallback}
		}
	}
	if len(branches) == 0 {
		return target{}, false
	}
	_, woke := s.startsOf(id, in)
	return target{id: id, title: cmp.Or(in.Title, defaultTitle), issue: issue, branches: branches, owned: owned, proj: proj, model: project.Model{Model: in.Model, Effort: in.Effort}, woke: woke}, true
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
		log.Printf("agentos: session %s did not take a message: %v", id, err)
		return fmt.Errorf("session %s did not take the text", id)
	}
	return nil
}

// refuseSettled is an error for a session that has settled, or that runs in an old terminal. It takes mu.
func (s *Sessions) refuseSettled(id string) error {
	s.mu.Lock()
	_, ended := s.ended[id]
	old := s.isOld(id)
	s.mu.Unlock()
	switch {
	case ended:
		return fmt.Errorf("Thread %s has settled", id)
	case old:
		return fmt.Errorf("Thread %s runs in an old terminal", id)
	}
	return nil
}

// TurnEnded folds the host's turn_end e into the session's state as a Stop hook, when no Stop hook ended that turn:
// Claude Code runs none after an interrupt. A turn that ended in an error, an interrupt among them, is a TurnError,
// which ends the turn as a Stop does but is no reply to alert the owner to. A turn_end older than the latest hook is an earlier turn's. The Stop hook
// of a normal turn runs just before the host reports its end, so the state file is read under its lock, as the hook
// writes it. At each start of the app the chat service passes in the latest turn_end of the log, so one older than
// the record the app holds is passed over before the lock is taken.
func (s *Sessions) TurnEnded(id string, e chat.Event) {
	s.mu.Lock()
	rec, known := s.records[id]
	s.mu.Unlock()
	if known && e.At < rec.At.UnixMilli() {
		return
	}
	unlock, err := bus.LockState(s.stateDir, id)
	if err != nil {
		return
	}
	cur, err := bus.ReadState(s.stateDir, id)
	if err != nil || !holdsPrompts(cur.State) || cur.Event == "Stop" || e.At < cur.At.UnixMilli() {
		unlock()
		return
	}
	at := time.UnixMilli(e.At)
	if !at.After(cur.At) { // e.At has only milliseconds, and take drops a record no later than the one before
		at = cur.At.Add(time.Millisecond)
	}
	ev := session.Event{Name: "Stop"}
	if e.Error {
		ev.Name = session.TurnError
	}
	next := session.Apply(id, cur, ev, at)
	err = bus.WriteState(s.stateDir, next)
	unlock()
	if err == nil {
		s.observe(next)
	}
}

// Chat is a session whose agent runs, or ran, under a host: the chat log the host keeps, the socket it listens on, the
// folder that holds the images sent in it and the folder it works in.
type Chat struct {
	ID, Project, Log, Socket, Media, Dir string
	Live                                 bool // the session runs; an ended one has only its log
}

// ChatOf is the chat of the session id, live or ended. An old terminal session has none. It takes mu.
func (s *Sessions) ChatOf(id string) (Chat, error) {
	name, err := session.ParseName(id)
	if err != nil {
		return Chat{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	in, live := term.Info{}, false
	if e, ok := s.ended[id]; ok {
		in = e.info
	} else if i := slices.IndexFunc(s.info, func(i term.Info) bool { return i.Name == name }); i >= 0 {
		in, live = s.info[i], true
	} else if !s.isOld(id) {
		return Chat{}, fmt.Errorf("No thread %s", id)
	}
	if !in.Hosted {
		return Chat{}, errOldChat
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
	return Chat{ID: name.String(), Project: name.Project, Log: s.chatLogPath(name), Socket: s.hostSocket(name), Media: s.chatMediaDir(name), Dir: dir, Live: live}
}

// isOld says whether the session runs in a terminal it was started in before sessions ran under a host. The app
// shows it settled and leaves it alone: it takes no message, hook or PR, and the owner ends it there. It needs mu.
func (s *Sessions) isOld(id string) bool {
	return slices.ContainsFunc(s.old, func(in term.Info) bool { return in.Name.String() == id })
}

// Run follows tmux, the hook bus and the pull requests until ctx ends.
func (s *Sessions) Run(ctx context.Context) error {
	if err := s.Start(ctx); err != nil {
		return err
	}
	go s.life.Run(ctx)
	return nil
}

// Has says whether a session of that id is running, in any project.
// Track records branch as the one the session works on, and watches its worktree and pull request.
func (s *Sessions) Track(id, branch string) error {
	s.mu.Lock()
	var proj project.Project
	var found, fork bool
	for _, in := range s.info {
		if in.Name.String() == id {
			proj, found = s.projectOf(in)
			fork = in.Fork != ""
			break
		}
	}
	s.mu.Unlock()
	if !found {
		return fmt.Errorf("session %q is not running", id)
	}
	if fork {
		return fmt.Errorf("session %s is a fork, which works on no branch of its own", id)
	}
	if branch == s.life.mainBranch(s.ctx, proj.Dir) {
		return fmt.Errorf("%s is the project's default branch; track the branch your work is on", branch)
	}
	s.life.track(s.ctx, id, proj, branch, false, true)
	return nil
}

// forkIn is the title of a live fork whose working folder is dir or a folder inside it, "" when none. It takes mu.
func (s *Sessions) forkIn(dir string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, in := range s.info {
		if cwd := s.cwds[in.Name.String()]; in.Fork != "" && cwd != "" && inside(cwd, dir) {
			return cmp.Or(in.Title, defaultTitle)
		}
	}
	return ""
}

func (s *Sessions) Has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isOld(id) || slices.ContainsFunc(s.info, func(in term.Info) bool { return in.Name.String() == id })
}

func (s *Sessions) closeBrowser(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), tmuxTimeout)
	defer cancel()
	s.browsers.Close(ctx, id)
}

// ShellIDs are the current project's shell sessions in tab order, empty until one is opened.
func (s *Sessions) ShellIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.project.Key()
	var mine []session.Name
	for _, n := range s.shells {
		if n.Project == key {
			mine = append(mine, n)
		}
	}
	slices.SortFunc(mine, func(a, b session.Name) int { return a.Shell - b.Shell })
	ids := make([]string, len(mine))
	for i, n := range mine {
		ids[i] = n.String()
	}
	return ids
}

// OpenShell makes sure the current project has a shell session and returns the
// first one's id. A shell is a login shell in the folder its session works in, or
// the project folder when sessionID is empty or its folder is gone; it reports
// nothing: it is not an agent, so it gets no hooks and no session number.
func (s *Sessions) OpenShell(sessionID string) (string, error) {
	s.shellMu.Lock()
	defer s.shellMu.Unlock()
	if ids := s.ShellIDs(); len(ids) > 0 {
		return ids[0], nil
	}
	return s.startShell(sessionID)
}

// NewShell starts another shell in the current project, numbered after the shells it has.
func (s *Sessions) NewShell(sessionID string) (string, error) {
	s.shellMu.Lock()
	defer s.shellMu.Unlock()
	return s.startShell(sessionID)
}

func (s *Sessions) startShell(sessionID string) (string, error) {
	s.mu.Lock()
	ctx, proj := s.ctx, s.project
	dir := proj.Dir
	if cwd := s.cwds[sessionID]; cwd != "" {
		if info, err := os.Stat(cwd); err == nil && info.IsDir() {
			dir = cwd
		}
	}
	var used []int
	for _, n := range s.shells {
		if n.Project == proj.Key() {
			used = append(used, n.Shell)
		}
	}
	s.mu.Unlock()
	name := session.Name{Project: proj.Key(), Shell: session.NextN(used)}
	tctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	login := cmp.Or(os.Getenv("SHELL"), "/bin/zsh")
	env := []string{
		"AGENTOS_PROJECT=" + proj.Key(),
		"AGENTOS_SOCKET=" + bus.SocketPath(s.stateDir),
	}
	if err := s.tmux.NewSession(tctx, name, "shell", dir, env, []string{login, "-l"}, startCols, startRows); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.shells = append(s.shells, name)
	s.mu.Unlock()
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
	s.closeTerm(id)
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	if err := s.tmux.Kill(ctx, name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shells = slices.DeleteFunc(s.shells, func(n session.Name) bool { return n == name })
	return nil
}

// hasBrowser says whether the session's agent gets a tab of the project's browser: when the project allows it, the
// browser and agent-browser are there, and the session has no brief.
func (s *Sessions) hasBrowser(proj project.Project, brief string) bool {
	return brief == "" && proj.BrowserOn() && s.browsers.AgentReady()
}

// launchFor is what the session's agent starts with. The agent learns where it runs and the commands that reach the
// app, and about the browser when it has one; a session with a brief learns only the brief. A task started by another
// session gets that session's handover last.
func launchFor(model project.Model, conversation, brief, handover string, chat, browser bool, fork forkStart) agent.Launch {
	l := agent.Launch{Model: model.Model, Effort: model.Effort, Resume: conversation, Chat: chat}
	if brief != "" {
		l.Prompt = brief
		return l
	}
	l.Prompt = prompts.Session() + "\n\n" + prompts.Thread()
	if fork.of != "" {
		l.Prompt += "\n\n" + prompts.Fork(fork.title)
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
