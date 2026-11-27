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
	shell  thread.Name
	status SetupState
	code   int
	cancel context.CancelFunc
}

const (
	setupPoll  = 150 * time.Millisecond
	setupGrace = 5 * time.Second // before a terminal that is not listed is taken for one the owner closed
)

// startSetup runs the project's worktree_command, the set-up of a worktree, in a terminal in the thread's worktree.
// The agent does not wait for it. It runs once, for a worktree orchestrator made. A command that cannot start is
// logged and shown as failed.
func (s *Threads) startSetup(st *starting) {
	command := strings.TrimSpace(st.proj.WorktreeCommand)
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
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		log.Printf("orchestrator: set-up command of thread %s did not start: %v", id, err)
		return
	}
	exit := filepath.Join(tmp, "setup-exit")
	_ = os.Remove(exit)
	// A command that fails leaves its terminal open as a login shell, to look into.
	script := fmt.Sprintf(`sh -c %s; code=$?; printf %%s "$code" > %s; if [ "$code" -ne 0 ]; then printf '\n[set-up command exited with %%s]\n' "$code"; exec %s -l; fi`, util.ShellQuote(command), util.ShellQuote(exit), util.ShellQuote(s.loginShell()))
	s.shellMu.Lock()
	shell, err := s.startShellRunning(st.proj, st.work.dir, "set-up", []string{"sh", "-c", script})
	s.shellMu.Unlock()
	if err != nil {
		log.Printf("orchestrator: set-up command of thread %s did not start: %v", id, err)
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

// watchSetup tells the thread's row how its set-up command ends: the line goes when it exits with 0, and a failure keeps
// the code and the terminal.
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
		data, err := os.ReadFile(exit)
		if err != nil {
			if time.Since(began) > setupGrace && !s.shellListed(run.shell) {
				// The owner closed the terminal first.
				s.dropSetup(id, run)
				return
			}
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
			} else {
				run.status, run.code = SetupFailed, code
			}
			s.sync()
		}
		s.mu.Unlock()
		return
	}
}

func (s *Threads) shellListed(name thread.Name) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.shells, func(sh term.Shell) bool { return sh.Name == name })
}

func (s *Threads) dropSetup(id string, run *setupRun) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setups[id] == run {
		delete(s.setups, id)
		s.sync()
	}
}

// isSetupShell says whether the shell runs a thread's set-up command, which a clean-up or a delete may stop.
func (s *Threads) isSetupShell(name thread.Name) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, run := range s.setups {
		if run.shell == name {
			return true
		}
	}
	return false
}

// stopSetup stops the thread's set-up command, if one runs or failed, and closes its terminal.
func (s *Threads) stopSetup(id string) {
	s.mu.Lock()
	run := s.setups[id]
	delete(s.setups, id)
	s.mu.Unlock()
	if run == nil {
		return
	}
	run.cancel()
	if err := s.stopShell(run.shell); err != nil {
		log.Printf("orchestrator: closing the set-up terminal %s of thread %s: %v", run.shell, id, err)
	}
}
