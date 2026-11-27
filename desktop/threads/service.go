package threads

import (
	"context"
	"errors"

	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/thread"
)

// Service is bound to the front end.
type Service struct {
	s   *Threads
	ctx func() context.Context
}

// NewService binds the threads; ctx gives the app's context at call time.
func NewService(s *Threads, ctx func() context.Context) *Service { return &Service{s: s, ctx: ctx} }

// NewThread starts a thread in the current project with the access given, read, supervised or full, and sends it the
// prompt once it is ready. A blank title is the first line of the prompt. A model or effort that is not empty is
// passed to the agent, and fast runs it in its faster mode; a blank access is supervised. place is worktree, a worktree orchestrator makes on a branch named
// from the title, or project, the project folder; blank is a worktree for a thread that writes and the project folder
// for a read one.
func (v *Service) NewThread(title, prompt, model, effort string, fast bool, access, place string) (Thread, error) {
	return v.s.CreateThread(title, StartOptions{Prompt: prompt, Model: project.Model{Model: model, Effort: effort, Fast: fast}, Access: thread.Access(access), Place: Place(place)})
}

// ModelChoice is a model and an effort; "" where nothing sets one.
type ModelChoice struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

// DefaultModel is what the agent's own settings give a new thread in the current project when no label, action or
// pick sets the model and effort. With model set, it is the effort the settings give that model.
func (v *Service) DefaultModel(model string) ModelChoice {
	m := v.s.DefaultModel(model)
	return ModelChoice{Model: m.Model, Effort: m.Effort}
}

// ShellInfo names a shell.
type ShellInfo struct {
	ID string `json:"id"`
}

// ShellNew starts a shell in the current project and returns its id: a shell of the thread in its worktree, or with
// thread "" a shell of the project in the project folder. Attach to it with TermOpen; it is not a thread row.
func (v *Service) ShellNew(thread string) (ShellInfo, error) {
	id, err := v.s.NewShell(thread)
	return ShellInfo{ID: id}, err
}

// ShellClose stops one shell of the current project.
func (v *Service) ShellClose(id string) error { return v.s.CloseShell(id) }

// SettleThread stops the agent and the servers it started at the owner's word; its row stays, settled.
func (v *Service) SettleThread(id string) error { return v.s.SettleAsOwner(id) }

// DeleteThread stops the thread when it runs and removes it with its chat. With worktree, the worktree orchestrator made
// for it goes too, unless a safety check keeps it, which refuses the delete; the branch always stays.
func (v *Service) DeleteThread(id string, worktree bool) error { return v.s.Delete(id, worktree) }

// DeletePlan is what DeleteThread of the thread would take with it, for its confirm: how many worktrees orchestrator made for
// it and its descendants may go, and whether they must.
func (v *Service) DeletePlan(id string) DeletePlan { return v.s.DeletePlan(id) }

// Servers are the ports the servers thread id started listen on, one row per port: the processes that carry the
// thread's marker, without its host and Claude. [] when it has none.
func (v *Service) Servers(id string) ([]Server, error) { return v.s.Servers(id) }

// StopServer stops the server pid of thread id. pid is looked up again among the thread's servers first, so no other
// process is stopped.
func (v *Service) StopServer(id string, pid int) error { return v.s.StopServer(id, pid) }

// OpenServer opens http://localhost:<port> in the default browser.
func (v *Service) OpenServer(port int) error { return v.s.OpenServer(port) }

// WakeThread starts a settled thread again under its own id, with nothing sent to it.
func (v *Service) WakeThread(id string) (Thread, error) {
	if _, err := v.s.Wake(id); err != nil {
		return Thread{}, err
	}
	return v.s.row(id)
}

// SetAccess gives the thread the access, read, supervised or full, between turns: a settled thread wakes with it, and a
// change to or from full starts a live thread again under its id.
func (v *Service) SetAccess(id, access string) error {
	if access == "" {
		return errors.New("Pick an access level")
	}
	a, err := thread.ParseAccess(access)
	if err != nil {
		return err
	}
	return v.s.SetAccess(id, a)
}

// SetFast turns fast mode on or off for the thread: a settled thread wakes with it, and a live one runs its next turn
// with it.
func (v *Service) SetFast(id string, on bool) error { return v.s.SetFast(id, on) }

// SetEffort gives the thread the effort, ultracode among them: a settled thread wakes with it, and a live one runs its
// next turn with it.
func (v *Service) SetEffort(id, effort string) error { return v.s.SetEffort(id, effort) }

// ForkThread starts a thread that continues the conversation of the thread id up to its reply at, the uuid of the
// reply's last block, as a new conversation.
func (v *Service) ForkThread(id, at string) (Thread, error) { return v.s.Fork(id, at) }

func (v *Service) RenameThread(id, title string) error { return v.s.Rename(id, title) }

// SideChatDelete deletes the side chat id; with unused, only when nobody sent it a message, as when its tab closes.
func (v *Service) SideChatDelete(id string, unused bool) error { return v.s.DeleteSideChat(id, unused) }

// TypeInto types text into the thread's prompt, not sent; an agent under a host gets it as a draft in its chat.
func (v *Service) TypeInto(id, text string) error { return v.s.TypeInto(id, text) }

// RefreshPRs polls the pull requests of the issue threads now.
func (v *Service) RefreshPRs() { v.s.life.Poll(v.ctx()) }

// AckPR says the user has seen the thread's pull request comments and failing checks.
func (v *Service) AckPR(id string) error { return v.s.life.Ack(id) }

// Cleanup cleans up after a thread now; without force the safety checks apply.
func (v *Service) Cleanup(id string, force bool) error { return v.s.life.Cleanup(v.ctx(), id, force) }

// Cleanups is the current project's clean-up log, newest first.
func (v *Service) Cleanups() []Cleanup { return v.s.life.Cleanups(v.s.Current().Key()) }

// PullRequests is the current project's open pull requests, newest update first.
func (v *Service) PullRequests() RepoPulls { return v.s.PullRequests() }

// Viewer is the login of the GitHub user the lists mark as the owner's own.
func (v *Service) Viewer() (string, error) {
	me, err := v.s.life.login(v.ctx(), v.s.Current().Dir)
	if err != nil {
		return "", errors.New("Could not read your GitHub login.")
	}
	return me, nil
}

// Memory is what orchestrator and its agents held at the last sample.
func (v *Service) Memory() Memory { return v.s.Memory() }

// OpenInEditor opens the folder the thread works in, its worktree or else the project folder, in the editor app; a path
// opens that file in the folder instead.
func (v *Service) OpenInEditor(id, path string) error { return v.s.OpenInEditor(id, path) }

// OpenProjectInEditor is OpenInEditor for the project's folder, where no thread is open.
func (v *Service) OpenProjectInEditor(project, path string) error {
	return v.s.OpenProjectInEditor(project, path)
}
