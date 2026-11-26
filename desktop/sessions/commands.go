package sessions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/session"
)

// Commands answers the tools that work on threads.
type Commands struct{ s *Sessions }

func NewCommands(s *Sessions) *Commands { return &Commands{s: s} }

func started(s Session) string { return fmt.Sprintf("started session %d: %s", s.N, s.Title) }

// New starts a thread with the access option, Supervised unless set, titled with the words given, and gives it the
// prompt, sent unless the project says manual. A prompt from a thread is framed as that thread's handover, so the new
// one does not take it for the owner's. A thread that is not Full access cannot start one that is.
func (c *Commands) New(_ context.Context, req ctl.Request) (string, error) {
	access, err := session.ParseAccess(req.Opts["access"])
	if err != nil {
		return "", err
	}
	if _, err := session.ParseName(req.Session); err != nil {
		return "", errors.New("no calling thread")
	}
	if err := c.s.mayGive(req.Session, access); err != nil {
		return "", err
	}
	prompt := req.Opts["prompt"]
	if prompt != "" {
		prompt = FromApp(prompts.Handover(prompt))
	}
	s, err := c.s.Create(strings.Join(req.Args, " "), prompt, c.s.Current().SendsPrompt(), 0, access)
	if err != nil {
		return "", err
	}
	return started(s), nil
}

// Track tells the app which branch the asking session works on.
func (c *Commands) Track(_ context.Context, req ctl.Request) (string, error) {
	branch := strings.TrimSpace(req.Opts["branch"])
	if branch == "" {
		return "", errors.New("track needs a branch")
	}
	if _, err := session.ParseName(req.Session); err != nil {
		return "", errors.New("no calling thread")
	}
	if err := c.s.Track(req.Session, branch); err != nil {
		return "", err
	}
	return "tracking " + branch, nil
}
