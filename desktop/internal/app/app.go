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
	"github.com/nednella/orchestrator/desktop/settings"
	"github.com/nednella/orchestrator/desktop/stats"
	"github.com/nednella/orchestrator/desktop/terminal"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/desktop/update"
	"github.com/nednella/orchestrator/internal/agent"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/thread"
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
	DockIcon  func(theme string)              // shows the theme's icon in the dock; nil leaves the bundle's
}

const defaultConfigTick = 2 * time.Second

// App is the services, wired together.
type App struct {
	ctx      atomic.Pointer[context.Context]
	cancel   atomic.Pointer[context.CancelFunc]
	threads  *threads.Threads
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
	// configTick is how often the config file is read again; a field so a test can shorten it.
	configTick time.Duration
	registry   *projects.Registry
	agents     agent.Registry
	issues     *issues.Issues
	warn       *warn.Warnings
	media      http.Handler
	assets     http.Handler
	stateDir   string
	services   []any
}

// New wires the services: who needs whom is decided here, and nowhere else.
func New(c Config, h Host, runner run.Runner, stream run.Streamer, claude run.EnvRunner) *App {
	a := &App{configTick: defaultConfigTick}
	ctx := func() context.Context {
		if c := a.ctx.Load(); c != nil {
			return *c
		}
		return context.Background()
	}
	watch := network.New(h.Emit)
	runner = watch.Runner(runner)
	events := stats.NewLog(c.DataDir, c.Machine)
	proofs := evidence.New(c.DataDir, events)
	browsers := browser.New(c.LocalDir, h.Emit, c.Env)
	terms := terminal.New(c.Tmux, h.Emit, h.Clipboard)
	stayAwake := awake.New(h.Awake, h.Emit)
	sess := threads.New(threads.Options{
		Tmux: c.Tmux, Agents: c.Agents, Binary: c.Binary, StateDir: c.StateDir, LocalDir: c.LocalDir, Projects: c.Registry,
		Current: c.Project, Emit: h.Emit, Run: runner, Stream: stream, Activity: events, CloseTerminal: terms.Close, Evidence: proofs, Browsers: browsers,
		Awake: stayAwake, KeepAwake: c.Registry.KeepAwake, EditorApp: c.Registry.EditorApp, ThreadPlace: c.Registry.ThreadPlace,
	})
	chats := chat.New(sess, c.LocalDir, h.Emit)
	a.limits = limits.New(sess, host.Usage, limits.Exec(c.Env), filepath.Join(c.StateDir, "usage-probe"), h.Emit)
	chats.OnRateLimit(a.limits.Seen)
	iss := issues.New(runner, sess, h.Emit)
	sess.Hook(iss.CachedRepo, iss.Emit, iss)
	a.registry, a.issues, a.warn, a.agents = c.Registry, iss, warn.New(h.Emit), c.Agents
	store := notes.New(c.DataDir)
	a.threads, a.chats, a.terms, a.notes, a.evidence, a.events, a.stateDir = sess, chats, terms, store, proofs, events, c.StateDir
	a.browsers, a.awake = browsers, stayAwake
	a.media = media.Handler(c.DataDir, c.LocalDir)
	assets := http.NewServeMux()
	assets.Handle(media.Prefix, a.media)
	assets.Handle(terminal.Prefix, terms.Handler())
	a.assets = assets
	browsers.Hook(func(id string) string {
		name, err := thread.ParseName(id)
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
		name, err := thread.ParseName(id)
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
	mgr := digest.NewManager(digest.New(c.DataDir), sess, store, claude, c.Env, h.Emit, ctx)
	a.digests = mgr
	a.updates = update.New(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return runner(ctx, "", name, args...)
	}, c.StateDir, Bundle(), h.Emit, upd.Relaunch, h.Quit)
	machine := func() string { return "" }
	if h.Machine != nil {
		machine = sync.OnceValue(h.Machine)
	}
	proj := projects.NewService(c.Registry, sess, store, iss, runner, c.StateDir, h.PickDir, machine, ctx)
	sessSvc := threads.NewService(sess, ctx)
	a.router = control.New(sess)
	a.tools = orchestration.NewCommands(sess, chats, a.router, ctx)
	chats.OnTurnEnd(a.tools.ChildTurnEnded)
	sess.OnSlept(a.tools.ChildSlept)
	sess.OnDelete(a.tools.ThreadDeleted)
	changes := evidence.Changes{Emit: h.Emit, Touch: sess.Touch}
	sc, ic := threads.NewCommands(sess), issues.NewCommands(iss, sessSvc)
	// Only a tool call reaches these, as the thread its token names (orchestration.agentRequest sets the thread and
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
		stats.NewService(stats.NewBooks(events, c.Registry)),
		update.NewService(a.updates, ctx),
		limits.NewService(a.limits, ctx),
		settings.NewService(c.Registry, sess, c.DataDir, c.DataDirFixed, h.PickDir, a.updates.Relaunch, h.DockIcon),
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

// Start begins following the threads; it runs until ctx ends or Stop.
func (a *App) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.ctx.Store(&ctx)
	a.cancel.Store(&cancel)
	a.browsers.StopStrays()
	go a.events.Load()
	if err := a.threads.Run(ctx); err != nil {
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
	tick := time.NewTicker(a.configTick)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if err := a.agents.CheckInstalled(a.threads.Provider()); err != nil {
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
			a.issues.Reload(ctx, a.threads.Current())
		}
	}
}

// Stop closes the terminal streams, stops answering the command line and ends what Start began: Wails never
// cancels the context it starts the app with.
func (a *App) Stop() {
	if cancel := a.cancel.Load(); cancel != nil {
		(*cancel)()
	}
	a.chats.Stop()
	a.terms.CloseAll()
	a.browsers.CloseAll()
	a.awake.Hold(false)
	a.threads.Stop()
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

// Threads is the thread list, for tests and wiring.
func (a *App) Threads() *threads.Threads { return a.threads }

// Notes is the notes store, for tests.
func (a *App) Notes() *notes.Notes { return a.notes }

// Browsers are the hidden browsers, for tests.
func (a *App) Browsers() *browser.Browsers { return a.browsers }

// SetConfigTick sets how often the config file is read again, 2 seconds by default. It comes before Start.
func (a *App) SetConfigTick(every time.Duration) { a.configTick = every }

// Digests is the digest manager, for tests.
func (a *App) Digests() *digest.Manager { return a.digests }

// Updates is the updater, for tests.
func (a *App) Updates() *update.Updater { return a.updates }
