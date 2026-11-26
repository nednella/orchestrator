package app

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nednella/orchestrator/desktop/awake"
	"github.com/nednella/orchestrator/desktop/browser"
	"github.com/nednella/orchestrator/desktop/chat"
	"github.com/nednella/orchestrator/desktop/control"
	"github.com/nednella/orchestrator/desktop/digest"
	"github.com/nednella/orchestrator/desktop/evidence"
	"github.com/nednella/orchestrator/desktop/internal/media"
	"github.com/nednella/orchestrator/desktop/internal/network"
	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/internal/warn"
	"github.com/nednella/orchestrator/desktop/issues"
	"github.com/nednella/orchestrator/desktop/limits"
	"github.com/nednella/orchestrator/desktop/notes"
	"github.com/nednella/orchestrator/desktop/orchestration"
	"github.com/nednella/orchestrator/desktop/projects"
	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/desktop/settings"
	"github.com/nednella/orchestrator/desktop/stats"
	"github.com/nednella/orchestrator/desktop/terminal"
	"github.com/nednella/orchestrator/desktop/update"
	"github.com/nednella/orchestrator/internal/agent"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/session"
	upd "github.com/nednella/orchestrator/internal/update"
)

// Host is what the window provides. The services never touch Wails directly.
type Host struct {
	Emit      func(event string, payload any)
	Clipboard func(text string)
	PickDir   func() (string, error)          // "" when the user cancels
	Quit      func()                          // closes the app, as the user would
	Awake     func() (stop func(), err error) // holds off idle sleep until stop is called
	Machine   func() string                   // this Mac's name, read once; nil leaves it empty
}

// App is the services, wired together.
type App struct {
	ctx      atomic.Pointer[context.Context]
	cancel   atomic.Pointer[context.CancelFunc]
	sessions *sessions.Sessions
	chats    *chat.Chats
	terms    *terminal.Terms
	notes    *notes.Notes
	evidence *evidence.Store
	events   *stats.Log
	browsers *browser.Browsers
	awake    *awake.Awake
	digests  *digest.Manager
	updates  *update.Updater
	limits   *limits.Tracker
	router   *control.Router
	tools    *orchestration.Commands
	registry *projects.Registry
	issues   *issues.Issues
	warn     *warn.Warnings
	media    http.Handler
	assets   http.Handler
	stateDir string
	services []any
}

// New wires the services: who needs whom is decided here, and nowhere else.
func New(c Config, h Host, runner run.Runner, stream run.Streamer, claude run.EnvRunner) *App {
	a := &App{}
	ctx := func() context.Context {
		if c := a.ctx.Load(); c != nil {
			return *c
		}
		return context.Background()
	}
	watch := network.New(h.Emit)
	runner = watch.Runner(runner)
	events := stats.NewLog(c.DataDir, c.Machine)
	waits := stats.New(events)
	proofs := evidence.New(c.DataDir, events)
	browsers := browser.New(c.LocalDir, h.Emit)
	terms := terminal.New(c.Tmux, h.Emit, h.Clipboard)
	stayAwake := awake.New(h.Awake, h.Emit)
	sess := sessions.New(sessions.Options{
		Tmux: c.Tmux, Agents: c.Agents, Binary: c.Binary, StateDir: c.StateDir, LocalDir: c.LocalDir, Projects: c.Registry,
		Current: c.Project, Emit: h.Emit, Run: runner, Stream: stream, Tally: waits, Activity: events, CloseTerminal: terms.Close, Evidence: proofs, Browsers: browsers,
		Awake: stayAwake, KeepAwake: c.Registry.KeepAwake, EditorApp: c.Registry.EditorApp, ThreadPlace: c.Registry.ThreadPlace,
	})
	chats := chat.New(sess, c.LocalDir, h.Emit)
	a.limits = limits.New(sess, host.Usage, limits.Exec, filepath.Join(c.StateDir, "usage-probe"), h.Emit)
	chats.OnRateLimit(a.limits.Seen)
	iss := issues.New(runner, sess, h.Emit)
	sess.Hook(iss.CachedRepo, iss.Emit, iss)
	a.registry, a.issues, a.warn = c.Registry, iss, warn.New(h.Emit)
	store := notes.New(c.DataDir)
	a.sessions, a.chats, a.terms, a.notes, a.evidence, a.events, a.stateDir = sess, chats, terms, store, proofs, events, c.StateDir
	a.browsers, a.awake = browsers, stayAwake
	a.media = media.Handler(c.DataDir, c.LocalDir)
	assets := http.NewServeMux()
	assets.Handle(media.Prefix, a.media)
	assets.Handle(terminal.Prefix, terms.Handler())
	a.assets = assets
	browsers.Hook(func(id string) string {
		name, err := session.ParseName(id)
		if err != nil {
			return ""
		}
		for _, p := range c.Registry.List() {
			if p.Key() == name.Project {
				return p.URL
			}
		}
		return ""
	}, func(id string) string {
		name, err := session.ParseName(id)
		if err != nil {
			return id
		}
		project := name.Project
		for _, p := range c.Registry.List() {
			if p.Key() == name.Project {
				project = p.Name
			}
		}
		return fmt.Sprintf("%s · %d %s", project, sess.Number(id), sess.Title(id))
	}, sess.Touch)
	mgr := digest.NewManager(digest.New(c.DataDir), sess, store, claude, h.Emit, ctx)
	a.digests = mgr
	a.updates = update.New(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return runner(ctx, "", name, args...)
	}, c.StateDir, Bundle(), h.Emit, upd.Relaunch, h.Quit)
	machine := func() string { return "" }
	if h.Machine != nil {
		machine = sync.OnceValue(h.Machine)
	}
	proj := projects.NewService(c.Registry, sess, store, iss, c.StateDir, h.PickDir, machine, ctx)
	sessSvc := sessions.NewService(sess, ctx)
	a.router = control.New(sess)
	a.tools = orchestration.NewCommands(sess, chats, a.router, ctx)
	chats.OnTurnEnd(a.tools.ChildTurnEnded)
	sess.OnSlept(a.tools.ChildSlept)
	sess.OnDelete(a.tools.ThreadDeleted)
	changes := evidence.Changes{Emit: h.Emit, Touch: sess.Touch}
	sc, ic := sessions.NewCommands(sess), issues.NewCommands(iss, sessSvc)
	// Only a tool call reaches these, as the thread its token names (orchestration.agentRequest sets the session and
	// the project from it); the socket answers none of them.
	a.handle(map[string]control.Handler{
		"track":   sc.Track,
		"note":    notes.NewCommands(store, a.router, sess, h.Emit).Note,
		"show":    evidence.NewCommands(proofs, a.router, sess, changes).Show,
		"browser": browser.NewCommands(browsers, a.router, proofs, changes).Browser,
		"issue":   ic.Issue,
		"refresh": ic.Refresh,
		"new":     sc.New,
	})
	// The socket answers this one only, and it takes its caller from the token.
	a.router.HandleExternal("orchestrate", a.tools.Orchestrate)
	a.services = []any{
		proj,
		sessSvc,
		chat.NewService(chats, runner, c.Registry.Dir, ctx),
		terminal.NewService(terms),
		issues.NewService(iss, ctx),
		notes.NewService(store, sess, sess, iss, events, runner, h.Emit, ctx),
		evidence.NewService(proofs, changes),
		browser.NewService(browsers, proofs, changes, ctx),
		digest.NewService(mgr),
		awake.NewService(stayAwake),
		stats.NewService(waits, c.Registry, stats.NewBooks(events, c.Registry)),
		update.NewService(a.updates, ctx),
		limits.NewService(a.limits, ctx),
		settings.NewService(c.Registry, sess, c.DataDir, c.DataDirFixed, h.PickDir, a.updates.Relaunch),
	}
	return a
}

func (a *App) handle(handlers map[string]control.Handler) {
	for cmd, h := range handlers {
		a.router.Handle(cmd, h)
	}
}

// Services are the structs to bind to the front end.
func (a *App) Services() []any { return a.services }

// Start begins following the sessions; it runs until ctx ends or Stop.
func (a *App) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.ctx.Store(&ctx)
	a.cancel.Store(&cancel)
	a.browsers.StopStrays()
	go a.events.Load()
	if err := a.sessions.Run(ctx); err != nil {
		return err
	}
	a.chats.Start(ctx)
	a.router.Listen(a.stateDir)
	go a.digests.Loop(ctx)
	go a.updates.Loop(ctx)
	go a.limits.Run(ctx)
	go a.followConfig(ctx)
	return nil
}

// followConfig picks up hand edits to the config file while the app runs, and warns while no thread can start: claude
// is not installed.
func (a *App) followConfig(ctx context.Context) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if err := agent.CheckInstalled(a.sessions.Provider()); err != nil {
			a.warn.Report("agent", err.Error())
		} else {
			a.warn.Clear("agent")
		}
		changed, err := a.registry.Reload()
		if err != nil {
			a.warn.Report("config", err.Error())
			continue
		}
		a.warn.Clear("config")
		if changed {
			a.issues.Reload(ctx, a.sessions.Current())
		}
	}
}

// Stop closes the terminal streams, stops answering the command line and ends what Start began: Wails never
// cancels the context it starts the app with.
func (a *App) Stop() {
	if cancel := a.cancel.Load(); cancel != nil {
		(*cancel)()
	}
	a.terms.CloseAll()
	a.browsers.CloseAll()
	a.awake.Hold(false)
	a.sessions.Stop()
	a.router.Close()
}

// Media serves the pictures of notes, evidence and chats under /media/<project key>/<folder>/.
func (a *App) Media() http.Handler { return a.media }

// Assets serves what the front end fetches besides its own files: the pictures under /media/ and the shells' output
// under /term/.
func (a *App) Assets() http.Handler { return a.assets }

// Answer runs a command as a tool call does, which the control socket never answers for a client: for tests.
func (a *App) Answer(ctx context.Context, req ctl.Request) ctl.Response {
	return a.router.Answer(ctx, req)
}

// Tools returns the commands that answer the orchestrator tools threads call, for tests and wiring.
func (a *App) Tools() *orchestration.Commands { return a.tools }

// Sessions is the session list, for tests and wiring.
func (a *App) Sessions() *sessions.Sessions { return a.sessions }

// Notes is the notes store, for tests.
func (a *App) Notes() *notes.Notes { return a.notes }

// Browsers are the hidden browsers, for tests.
func (a *App) Browsers() *browser.Browsers { return a.browsers }

// Digests is the digest manager, for tests.
func (a *App) Digests() *digest.Manager { return a.digests }

// Updates is the updater, for tests.
func (a *App) Updates() *update.Updater { return a.updates }
