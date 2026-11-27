package threads

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/thread"
	"github.com/nednella/orchestrator/internal/util"
)

// SetupState is how the project's set-up command of a new worktree went, for the thread's row.
type SetupState string

const (
	SetupRunning SetupState = "running"
	SetupFailed  SetupState = "failed" // it exited with a code other than 0; its terminal stays open
)

// setupRun is the set-up command of a thread, which runs in a terminal of its own while the agent works.
type setupRun struct {
	shell   thread.Name
	status  SetupState
	code    int
	problem string // why a command that could not start did not
	cancel  context.CancelFunc
}

const (
	setupPoll  = 150 * time.Millisecond
	setupGrace = 5 * time.Second // before a terminal that is not listed is taken for one the owner closed
)

// startSetup runs the project's setup_command, the set-up of a worktree, in a terminal in the thread's worktree. The
// agent does not wait for it. It runs once, for a worktree orchestrator made. A command that cannot start shows as a
// failed set-up.
func (s *Threads) startSetup(st *starting) {
	command := strings.TrimSpace(st.proj.SetupCommand)
	if command == "" || !st.work.made || st.setupStarted {
		return
	}
	st.setupStarted = true
	id := st.name.String()
	base := ""
	if w, ok := s.made.Of(st.proj.Key(), id); ok {
		base = w.Base
	}
	command = strings.NewReplacer("{dir}", util.ShellQuote(st.proj.Dir), "{branch}", util.ShellQuote(st.work.branch), "{worktree}", util.ShellQuote(st.work.dir), "{base}", util.ShellQuote(base)).Replace(command)
	tmp := s.tmpDir(st.name)
	exit := filepath.Join(tmp, "setup-exit")
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		s.setupDidNotStart(id, err)
		return
	}
	_ = os.Remove(exit)
	// A command that fails leaves its terminal open as a login shell, to look into.
	script := fmt.Sprintf(`sh -c %s; code=$?; printf %%s "$code" > %s; if [ "$code" -ne 0 ]; then printf '\n[set-up command exited with %%s]\n' "$code"; exec %s -l; fi`, util.ShellQuote(command), util.ShellQuote(exit), util.ShellQuote(s.loginShell()))
	s.shellMu.Lock()
	shell, err := s.startShellRunning(st.proj, st.work.dir, "set-up", []string{"sh", "-c", script}, id)
	s.shellMu.Unlock()
	if err != nil {
		s.setupDidNotStart(id, err)
		return
	}
	name, _ := thread.ParseName(shell)
	ctx, cancel := context.WithCancel(st.app)
	run := &setupRun{shell: name, status: SetupRunning, cancel: cancel}
	s.mu.Lock()
	s.setups[id] = run
	s.sync()
	s.mu.Unlock()
	go s.watchSetup(ctx, id, run, exit)
}

// setupDidNotStart shows a set-up command that could not start as a failed one, with why.
func (s *Threads) setupDidNotStart(id string, err error) {
	log.Printf("orchestrator: set-up command of thread %s did not start: %v", id, err)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setups[id] = &setupRun{status: SetupFailed, problem: util.FirstLine(err.Error()), cancel: func() {}}
	s.sync()
}

// watchSetup tells the thread's row how its set-up command ends: the line goes when it exits with 0, and a failure keeps
// the code and the terminal until the terminal goes. The terminal is the session tmux tagged with the thread, so a
// shell that took its name after it went is never taken for it.
func (s *Threads) watchSetup(ctx context.Context, id string, run *setupRun, exit string) {
	began := time.Now()
	tick := time.NewTicker(setupPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if time.Since(began) > setupGrace && !s.setupListed(id, run) {
			s.dropSetup(id, run)
			return
		}
		s.mu.Lock()
		failed := run.status == SetupFailed
		s.mu.Unlock()
		if failed {
			continue
		}
		data, err := os.ReadFile(exit)
		if err != nil {
			continue
		}
		code, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if convErr != nil {
			continue
		}
		s.mu.Lock()
		if s.setups[id] == run {
			if code == 0 {
				delete(s.setups, id)
				s.sync()
				s.mu.Unlock()
				return
			}
			run.status, run.code = SetupFailed, code
			s.sync()
		}
		s.mu.Unlock()
	}
}

// setupListed says whether the run's terminal is still there: a shell of that name that tmux tagged with the thread.
func (s *Threads) setupListed(id string, run *setupRun) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.shells, func(sh term.Shell) bool { return sh.Name == run.shell && sh.Setup == id })
}

func (s *Threads) dropSetup(id string, run *setupRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setups[id] == run {
		run.cancel()
		delete(s.setups, id)
		s.sync()
	}
}

// runsSetup says whether the shell is the terminal of a set-up command that still runs, which a clean-up or a delete may
// stop. A failed one is an ordinary shell.
func (s *Threads) runsSetup(name thread.Name) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, run := range s.setups {
		if run.shell == name && run.status == SetupRunning && slices.ContainsFunc(s.shells, func(sh term.Shell) bool { return sh.Name == name && sh.Setup == id }) {
			return true
		}
	}
	return false
}

// forgetSetupShell drops the record of a set-up command whose terminal is closed, so the name a later shell takes is
// never taken for it.
func (s *Threads) forgetSetupShell(name thread.Name) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, run := range s.setups {
		if run.shell == name && run.shell != (thread.Name{}) {
			run.cancel()
			delete(s.setups, id)
			s.sync()
		}
	}
}

// stopSetup stops the thread's set-up command when it runs and closes its terminal. A failed one is left to its
// terminal, an ordinary shell, and only the record goes.
func (s *Threads) stopSetup(id string) {
	s.mu.Lock()
	run := s.setups[id]
	delete(s.setups, id)
	s.mu.Unlock()
	if run == nil {
		return
	}
	run.cancel()
	if run.status != SetupRunning {
		return
	}
	if err := s.stopShell(run.shell); err != nil {
		log.Printf("orchestrator: closing the set-up terminal %s of thread %s: %v", run.shell, id, err)
	}
}
