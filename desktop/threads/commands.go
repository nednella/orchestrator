package threads

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/thread"
)

// Commands answers the tools that work on threads.
type Commands struct{ s *Threads }

func NewCommands(s *Threads) *Commands { return &Commands{s: s} }

func started(s Thread) string { return fmt.Sprintf("started thread %d: %s", s.N, s.Title) }

// New starts a thread with the access option, Supervised unless set, titled with the words given, and gives it the
// prompt, sent unless the project says manual. A prompt from a thread is framed as that thread's handover, so the new
// one does not take it for the owner's. A thread that is not Full access cannot start one that is.
func (c *Commands) New(_ context.Context, req ctl.Request) (string, error) {
	access, err := thread.ParseAccess(req.Opts["access"])
	if err != nil {
		return "", err
	}
	if _, err := thread.ParseName(req.Thread); err != nil {
		return "", errors.New("no calling thread")
	}
	if err := c.s.mayGive(req.Thread, access); err != nil {
		return "", err
	}
	prompt := req.Opts["prompt"]
	if prompt != "" {
		prompt = FromApp(prompts.Handover(prompt))
	}
	proj, ok := c.s.ProjectByKey(req.Project)
	if !ok {
		return "", fmt.Errorf("no project %q", req.Project)
	}
	s, err := c.s.Create(proj, strings.Join(req.Args, " "), prompt, proj.SendsPrompt(), 0, access)
	if err != nil {
		return "", err
	}
	return started(s), nil
}

// Track tells the app which branch the asking thread works on.
func (c *Commands) Track(_ context.Context, req ctl.Request) (string, error) {
	branch := strings.TrimSpace(req.Opts["branch"])
	if branch == "" {
		return "", errors.New("track needs a branch")
	}
	if _, err := thread.ParseName(req.Thread); err != nil {
		return "", errors.New("no calling thread")
	}
	if err := c.s.Track(req.Thread, branch); err != nil {
		return "", err
	}
	return "tracking " + branch, nil
}
