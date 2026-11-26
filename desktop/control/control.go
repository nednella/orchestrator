// Package control answers the commands of the orchestrator tools: each command is a handler that a service registers.
package control

import (
	"context"
	"errors"
	"fmt"
	"log"

	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/session"
)

// Handler runs one command and returns what the command prints.
type Handler func(ctx context.Context, req ctl.Request) (string, error)

// Sessions tells who may ask.
type Sessions interface {
	Has(id string) bool
	Current() project.Project
	ThreadOfToken(token string) (string, bool)
}

// Router sends each command to its handler.
type Router struct {
	sessions Sessions
	switchTo func(name string) error
	handlers map[string]Handler
	external map[string]bool
	server   *ctl.Server
}

// New routes commands. switchTo makes a project current.
func New(s Sessions, switchTo func(name string) error) *Router {
	return &Router{sessions: s, switchTo: switchTo, handlers: map[string]Handler{}, external: map[string]bool{}}
}

// Handle registers the handler of a command that only a tool call reaches, through Answer. Such a handler trusts the
// request's session and project, which the tool call sets from the thread's token, so the socket never answers it:
// a client there could name any thread.
func (r *Router) Handle(cmd string, h Handler) { r.handlers[cmd] = h }

// HandleExternal registers the handler of a command the control socket answers. It must take its caller from the
// request's token and nothing else.
func (r *Router) HandleExternal(cmd string, h Handler) {
	r.handlers[cmd] = h
	r.external[cmd] = true
}

// Listen starts answering at the control socket in stateDir. Without it agents
// cannot use the app, which is logged and not fatal.
func (r *Router) Listen(stateDir string) {
	srv, err := ctl.Listen(ctl.SocketPath(stateDir), r.answerExternal)
	if err != nil {
		log.Printf("orchestrator: agents cannot reach the app: %v", err)
		return
	}
	r.server = srv
}

// Close stops answering.
func (r *Router) Close() {
	if r.server != nil {
		r.server.Close()
	}
}

// answerExternal is Answer for a client on the control socket, which may run only what HandleExternal registered.
func (r *Router) answerExternal(ctx context.Context, req ctl.Request) ctl.Response {
	if !r.external[req.Cmd] {
		return ctl.Response{Error: fmt.Sprintf("unknown command %q", req.Cmd)}
	}
	return r.Answer(ctx, req)
}

// Answer runs the handler of the request's command and returns what it printed, or why it failed.
func (r *Router) Answer(ctx context.Context, req ctl.Request) ctl.Response {
	h, ok := r.handlers[req.Cmd]
	if !ok {
		return ctl.Response{Error: fmt.Sprintf("unknown command %q", req.Cmd)}
	}
	out, err := h(ctx, req)
	if err != nil {
		return ctl.Response{Error: err.Error()}
	}
	return ctl.Response{OK: true, Out: out}
}

// AskerSession checks that the command comes from a session that runs.
func (r *Router) AskerSession(req ctl.Request) (string, error) {
	if _, err := session.ParseName(req.Session); err != nil {
		return "", errors.New("no calling thread")
	}
	if !r.sessions.Has(req.Session) {
		return "", fmt.Errorf("session %s is not running", req.Session)
	}
	return req.Session, nil
}

// Caller is the thread whose host the request's token was made for. It reads nothing else of the request: an orchestrator
// tool acts as its own thread, whatever the call says.
func (r *Router) Caller(req ctl.Request) (string, error) {
	if req.Token == "" {
		return "", errors.New("This call carries no orchestrator thread token")
	}
	id, ok := r.sessions.ThreadOfToken(req.Token)
	if !ok {
		return "", errors.New("This thread token is not valid: the thread settled or was woken again")
	}
	return id, nil
}

// AskerProject is the project key of the asking session, else the current project.
func (r *Router) AskerProject(req ctl.Request) string {
	if req.Project != "" {
		return req.Project
	}
	if name, err := session.ParseName(req.Session); err == nil {
		return name.Project
	}
	return r.sessions.Current().Key()
}

// Enter makes the project the command comes from the current one: the caller's
// own project, else its session's. The screen follows.
func (r *Router) Enter(req ctl.Request) {
	key := req.Project
	if name, err := session.ParseName(req.Session); key == "" && err == nil {
		key = name.Project
	}
	if key == "" || key == r.sessions.Current().Key() {
		return
	}
	_ = r.switchTo(key) // an unknown project leaves the current one
}

// Scope wraps a handler so it works in the project the command comes from.
func (r *Router) Scope(h Handler) Handler {
	return func(ctx context.Context, req ctl.Request) (string, error) {
		r.Enter(req)
		return h(ctx, req)
	}
}
