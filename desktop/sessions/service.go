package sessions

import (
	"context"

	"github.com/nednella/agentos/internal/project"
)

// Service is bound to the front end.
type Service struct {
	s   *Sessions
	ctx func() context.Context
}

// NewService binds the sessions; ctx gives the app's context at call time.
func NewService(s *Sessions, ctx func() context.Context) *Service { return &Service{s: s, ctx: ctx} }

// NewSession starts the agent in the current project and sends it the prompt once it is ready. A blank title is the
// first line of the prompt. A model or effort that is not empty is passed to the agent.
func (v *Service) NewSession(title, prompt, model, effort string) (Session, error) {
	return v.s.CreateWith(title, false, StartOptions{Prompt: prompt, Model: project.Model{Model: model, Effort: effort}})
}

// NewChat starts a chat in the current project: a read-only session for questions about it.
func (v *Service) NewChat(title, prompt, model, effort string) (Session, error) {
	return v.s.CreateWith(title, true, StartOptions{Prompt: prompt, Model: project.Model{Model: model, Effort: effort}})
}

// ShellInfo names a shell session.
type ShellInfo struct {
	ID string `json:"id"`
}

// ShellOpen makes sure the current project has a shell session and returns the first one's id. A new shell starts
// in the folder of the session sessionID, or in the project folder when that is empty or unknown.
// Attach to it with TermOpen like any session; it is not a session row.
func (v *Service) ShellOpen(sessionID string) (ShellInfo, error) {
	id, err := v.s.OpenShell(sessionID)
	return ShellInfo{ID: id}, err
}

// ShellNew starts another shell in the current project and returns its id; see ShellOpen for its folder.
func (v *Service) ShellNew(sessionID string) (ShellInfo, error) {
	id, err := v.s.NewShell(sessionID)
	return ShellInfo{ID: id}, err
}

// ShellClose stops one shell of the current project.
func (v *Service) ShellClose(id string) error { return v.s.CloseShell(id) }

// KillSession stops the agent; its row stays as ended.
func (v *Service) KillSession(id string) error { return v.s.Kill(id) }

// DismissSession removes the row of an ended session.
func (v *Service) DismissSession(id string) error { return v.s.Dismiss(id) }

// ResumeSession starts a session that continues an ended one's conversation, and removes the ended row.
func (v *Service) ResumeSession(id string) (Session, error) { return v.s.Resume(id) }

func (v *Service) RenameSession(id, title string) error { return v.s.Rename(id, title) }

// TypeInto types text into the session's prompt, not sent.
func (v *Service) TypeInto(id, text string) error { return v.s.TypeInto(id, text) }

// RefreshPRs polls the pull requests of the issue sessions now.
func (v *Service) RefreshPRs() { v.s.life.Poll(v.ctx()) }

// AckPR says the user has seen the session's pull request comments and failing checks.
func (v *Service) AckPR(id string) error { return v.s.life.Ack(id) }

// Cleanup cleans up after a session now; without force the safety checks apply.
func (v *Service) Cleanup(id string, force bool) error { return v.s.life.Cleanup(v.ctx(), id, force) }

// Cleanups is the current project's clean-up log, newest first.
func (v *Service) Cleanups() []Cleanup { return v.s.life.Cleanups(v.s.Current().Key()) }

// Memory is what agentos and its agents held at the last sample.
func (v *Service) Memory() Memory { return v.s.Memory() }
