package threads

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/thread"
)

// SetFast turns fast mode on or off for the thread id. A settled thread starts with it at its next wake; a live one
// changes as it runs, from its next turn, and a refusal of Claude's leaves it as it was.
func (s *Threads) SetFast(id string, on bool) error {
	return s.setFlags(id, "fast mode", agent.FastSettings(on), func(in *term.Info) { in.Fast = on })
}

// SetEffort gives the thread id the effort, ultracode among them, as SetFast does for fast mode.
func (s *Threads) SetEffort(id, effort string) error {
	if effort == "" {
		return errors.New("Pick an effort")
	}
	if err := s.Provider().ValidModel("", effort); err != nil {
		return err
	}
	return s.setFlags(id, "effort", agent.EffortSettings(effort), func(in *term.Info) { in.Effort = effort })
}

// setFlags changes what the thread runs with: change is made to its record and settings go to its Claude. The record
// is changed after Claude takes the settings, so a refusal leaves it as it was.
func (s *Threads) setFlags(id, what string, settings map[string]any, change func(*term.Info)) error {
	name, err := thread.ParseName(id)
	if err != nil {
		return err
	}
	if err := s.startingError(id); err != nil {
		return err
	}
	if name.IsShell() {
		return errNotThread
	}
	s.mu.Lock()
	deleted := s.deleted[id]
	_, settled := s.ended[id]
	live := slices.ContainsFunc(s.info, func(i term.Info) bool { return i.Name == name })
	s.mu.Unlock()
	switch {
	case deleted:
		return fmt.Errorf("Thread %s was deleted", id)
	case settled:
		return s.settledFlags(id, what, change)
	case !live:
		return fmt.Errorf("No thread %s", id)
	}
	socket, err := s.readyHost(name)
	if err == nil {
		err = host.ApplyFlags(s.context(), socket, settings)
	}
	if err != nil {
		return flagsFailed(name, what, err)
	}
	return s.liveFlags(name, what, change)
}

// flagsFailed is the host's own reason for not changing the thread's setting, or a plain sentence when the host could
// not be reached, whose cause goes to the log.
func flagsFailed(name thread.Name, what string, err error) error {
	log.Printf("orchestrator: thread %s did not change its %s: %v", name, what, err)
	if refused := (*host.Refused)(nil); errors.As(err, &refused) {
		return errors.New(refused.Reason)
	}
	return fmt.Errorf("Thread %s did not change its %s", name, what)
}

// liveFlags records the change in the live thread's tmux session and state file, where a restart of the app finds it.
func (s *Threads) liveFlags(name thread.Name, what string, change func(*term.Info)) error {
	s.mu.Lock()
	i := slices.IndexFunc(s.info, func(i term.Info) bool { return i.Name == name })
	if i < 0 {
		s.mu.Unlock()
		return fmt.Errorf("No thread %s", name)
	}
	next := s.info[i]
	s.mu.Unlock()
	change(&next)
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	if err := s.tmux.SetModel(ctx, name, next.Model, next.Effort, next.Fast); err != nil {
		return fmt.Errorf("saving the thread's %s: %w", what, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.info {
		if s.info[i].Name == name {
			change(&s.info[i])
			s.saveIdentity(s.info[i])
			s.sync()
		}
	}
	return nil
}

// settledFlags records the change in the settled thread's state file, where its next wake reads it. It takes mu.
func (s *Threads) settledFlags(id, what string, change func(*term.Info)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.ended[id]
	if !ok {
		return fmt.Errorf("Thread %s did not go to sleep", id)
	}
	before := e.info
	change(&e.info)
	if e.info == before {
		return nil
	}
	if err := bus.WriteState(s.stateDir, e.record()); err != nil {
		e.info = before
		return fmt.Errorf("saving the thread's %s: %w", what, err)
	}
	s.sync()
	return nil
}
