// Package update keeps the app current: it checks the releases 15 seconds after the start and every 4 hours, and on
// the owner's word downloads a newer one and installs it, telling the front end each step.
package update

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nednella/agentos/internal/update"
	"github.com/nednella/agentos/internal/version"
)

const (
	firstCheck = 15 * time.Second
	checkEvery = 4 * time.Hour
	// checkWait is how long an install waits for a check that runs, since it checks again itself.
	checkWait = 90 * time.Second
	// quitDelay lets the answer to an update or relaunch reach its caller before the app goes.
	quitDelay = 300 * time.Millisecond
)

const (
	idle        = "idle"
	checking    = "checking"
	available   = "available"
	downloading = "downloading"
	downloaded  = "downloaded"
	failed      = "failed"
	// installed: the new bundle is in place, and only the restart into it failed.
	installed = "installed"
)

var errBusy = errors.New("An update action is running")

// State is where the update stands, the payload of the update event. Version is the newer release, "" when none is
// known. Percent is how far its download got, -1 while the size is unknown. FailedStep says which step a failed
// state failed at: "download" or "install". Error is why the last action failed.
type State struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	Percent    int    `json:"percent"`
	FailedStep string `json:"failedStep"`
	Error      string `json:"error"`
}

// Updater checks for releases, downloads one and installs it, one action at a time.
type Updater struct {
	run       update.Runner
	stateDir  string
	bundle    string // the .app the app runs from; "" when it runs from none
	emit      func(event string, payload any)
	relaunch  func(bundle string) error
	quit      func()
	current   string // the running version
	first     time.Duration
	every     time.Duration
	after     func(time.Duration) <-chan time.Time
	checkWait time.Duration

	mu      sync.Mutex
	state   State
	latest  update.Release // the newer release the last check found; empty while the app is current
	zip     string         // the downloaded bundle of latest, "" when there is none
	running string         // the action that runs: "check", "download" or "install"; "" when none
	ended   chan struct{}  // closed when the running action ends
}

// New makes an updater for the app at bundle. relaunch opens it again once the app has quit.
func New(run update.Runner, stateDir, bundle string, emit func(string, any), relaunch func(string) error, quit func()) *Updater {
	return &Updater{run: run, stateDir: stateDir, bundle: bundle, emit: emit, relaunch: relaunch, quit: quit, current: version.Version,
		first: firstCheck, every: checkEvery, after: time.After, checkWait: checkWait, state: State{Status: idle}}
}

// SetVersion replaces the running version, for tests: the test binary is always "dev".
func (u *Updater) SetVersion(v string) { u.current = v }

// SetLoop sets how long to wait before the first check, and between checks.
func (u *Updater) SetLoop(first, every time.Duration) { u.first, u.every = first, every }

// SetRelaunch replaces how the app is opened again, for tests.
func (u *Updater) SetRelaunch(relaunch func(string) error) { u.relaunch = relaunch }

// Loop checks for a newer release after the first wait and then after every wait, until ctx ends. A check that
// fails waits for the next one: there is no retry sooner. These checks run unseen: the state changes only when what
// they find does.
func (u *Updater) Loop(ctx context.Context) {
	wait := u.first
	for {
		select {
		case <-ctx.Done():
			return
		case <-u.after(wait):
		}
		if err := u.begin(ctx, "check"); err == nil {
			if err := u.check(ctx, false); err != nil {
				log.Printf("orchestrator: checking for a newer release: %v", err)
			}
			u.end()
		}
		wait = u.every
	}
}

// State is where the update stands now.
func (u *Updater) State() State {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.state
}

// Check asks GitHub for the latest release, showing that it checks. A check that fails keeps what the one before it
// found, and its download.
func (u *Updater) Check(ctx context.Context) error {
	if err := u.begin(ctx, "check"); err != nil {
		return err
	}
	defer u.end()
	return u.check(ctx, true)
}

// Download fetches the newer release the last check found into the state folder, where it waits for the install.
func (u *Updater) Download(ctx context.Context) error {
	if err := u.begin(ctx, "download"); err != nil {
		return err
	}
	defer u.end()
	return u.download(ctx)
}

// Install checks for a newer release, downloads it unless it waits already, installs it and relaunches the app;
// threads live in their hosts and keep going. When the check fails, a download that waits is installed. Once the
// new bundle is in place, Install only relaunches.
func (u *Updater) Install(ctx context.Context) error {
	if u.bundle == "" {
		return errors.New("orchestrator runs from no .app bundle, so it cannot replace itself")
	}
	if err := u.begin(ctx, "install"); err != nil {
		return err
	}
	defer u.end()
	if s := u.State(); s.Status == installed {
		return u.restart(s.Version)
	}
	err := u.check(ctx, true)
	u.mu.Lock()
	rel, waiting := u.latest, u.zip != ""
	u.mu.Unlock()
	if err != nil && !waiting {
		return err
	}
	if rel.Version == "" {
		return errors.New("orchestrator is up to date")
	}
	if err := u.download(ctx); err != nil {
		return err
	}
	u.mu.Lock()
	zip := u.zip
	u.mu.Unlock()
	err = update.InstallZip(ctx, u.run, rel, zip, u.bundle)
	// Installed, the zip is done with; failed, it is not trusted again.
	u.dropDownload()
	if err != nil {
		u.set(State{Status: failed, Version: rel.Version, FailedStep: "install", Error: err.Error()})
		return err
	}
	return u.restart(rel.Version)
}

// restart relaunches into the bundle that is in place; a relaunch that fails leaves it there, to restart later.
func (u *Updater) restart(version string) error {
	if err := u.Relaunch(); err != nil {
		u.set(State{Status: installed, Version: version, Error: err.Error()})
		return err
	}
	return nil
}

// begin takes the one action slot. An install waits for a check that runs, as it checks again itself; any other
// action that runs refuses the new one.
func (u *Updater) begin(ctx context.Context, action string) error {
	var deadline <-chan time.Time
	for {
		u.mu.Lock()
		running, ended := u.running, u.ended
		if running == "" {
			u.running, u.ended = action, make(chan struct{})
			u.mu.Unlock()
			return nil
		}
		u.mu.Unlock()
		if action != "install" || running != "check" {
			return errBusy
		}
		if deadline == nil {
			timer := time.NewTimer(u.checkWait)
			defer timer.Stop()
			deadline = timer.C
		}
		select {
		case <-ended:
		case <-deadline:
			return errors.New("The update check did not finish")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (u *Updater) end() {
	u.mu.Lock()
	defer u.mu.Unlock()
	close(u.ended)
	u.running = ""
}

// check asks for the latest release. shown says the owner asked: the state says it checks, and a failure says why.
// Unseen, a check moves the state on only when it finds another release than the last one.
func (u *Updater) check(ctx context.Context, shown bool) error {
	before := u.State()
	if shown {
		u.set(State{Status: checking, Version: before.Version, Percent: before.Percent})
	}
	rel, err := update.Check(ctx, u.run, u.stateDir, 0, time.Now())
	if err != nil {
		if shown {
			before.Error = err.Error()
			u.set(before)
		}
		return err
	}
	if !update.Newer(u.current, rel.Version) {
		rel = update.Release{}
	}
	u.mu.Lock()
	changed := u.latest != rel
	u.latest = rel
	if changed {
		u.zip = ""
	}
	zip := u.zip
	u.mu.Unlock()
	if before.Status == installed && !changed {
		u.set(before)
		return nil
	}
	adopted := false
	if zip == "" {
		zip = u.kept(ctx, rel)
		adopted = zip != ""
	}
	if !shown && !changed && !adopted {
		return nil
	}
	switch {
	case rel.Version == "":
		u.set(State{Status: idle})
	case zip != "":
		u.set(State{Status: downloaded, Version: rel.Version, Percent: 100})
	default:
		u.set(State{Status: available, Version: rel.Version})
	}
	return nil
}

// kept takes up a download of rel an earlier run of the app left, when its sha256 matches; anything else in the
// download folder goes.
func (u *Updater) kept(ctx context.Context, rel update.Release) string {
	zip := filepath.Join(u.downloads(), update.Asset)
	if _, err := os.Stat(zip); err != nil {
		_ = os.RemoveAll(u.downloads())
		return ""
	}
	if rel.Version == "" || update.Verify(ctx, u.run, rel, zip) != nil {
		_ = os.RemoveAll(u.downloads())
		return ""
	}
	u.mu.Lock()
	u.zip = zip
	u.mu.Unlock()
	return zip
}

func (u *Updater) download(ctx context.Context) error {
	u.mu.Lock()
	rel, zip := u.latest, u.zip
	u.mu.Unlock()
	if rel.Version == "" {
		return errors.New("There is no update to download")
	}
	if zip != "" {
		return nil
	}
	u.set(State{Status: downloading, Version: rel.Version})
	zip, err := update.Download(ctx, u.run, rel, u.downloads(), func(percent int) {
		u.set(State{Status: downloading, Version: rel.Version, Percent: percent})
	})
	if err != nil {
		_ = os.RemoveAll(u.downloads())
		u.set(State{Status: failed, Version: rel.Version, FailedStep: "download", Error: err.Error()})
		return err
	}
	u.mu.Lock()
	u.zip = zip
	u.mu.Unlock()
	u.set(State{Status: downloaded, Version: rel.Version, Percent: 100})
	return nil
}

// downloads is where a downloaded release waits for its install, also over a restart of the app.
func (u *Updater) downloads() string { return filepath.Join(u.stateDir, "update") }

func (u *Updater) dropDownload() {
	u.mu.Lock()
	u.zip = ""
	u.mu.Unlock()
	_ = os.RemoveAll(u.downloads())
}

// set moves the state on and tells the front end, when it changed.
func (u *Updater) set(s State) {
	u.mu.Lock()
	same := u.state == s
	u.state = s
	u.mu.Unlock()
	if !same {
		u.emit("update", s)
	}
}

// Relaunch arranges for the app to open again, then quits it shortly after.
func (u *Updater) Relaunch() error {
	if u.bundle == "" {
		return errors.New("orchestrator runs from no .app bundle, so it cannot relaunch")
	}
	if err := u.relaunch(u.bundle); err != nil {
		return err
	}
	time.AfterFunc(quitDelay, u.quit)
	return nil
}
