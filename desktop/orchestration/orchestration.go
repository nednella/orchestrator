// Package orchestration answers the agentos tools a thread's Claude calls. Each call acts as the thread its token
// names.
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/nednella/agentos/desktop/chat"
	"github.com/nednella/agentos/desktop/control"
	"github.com/nednella/agentos/desktop/sessions"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/session"
)

// Threads is what the tools use of the threads.
type Threads interface {
	CreateChild(ctx context.Context, o sessions.ChildOptions) (sessions.Session, error)
	AccessOf(id string) session.Access
	ChatOf(id string) (sessions.Chat, error)
	List() []sessions.Session
	Projects() []sessions.Project
	Title(id string) string
	Tell(id string, m sessions.Told) (string, error)
	Descendants(id string) []string
	ParentOf(id string) string
	DropTold(id string) int
	HoldsTold(id string) bool
	Reported(id string) int64
	SetReported(id string, seq int64) error
	SettleIdle(id string) (bool, error)
}

// tool runs one agentos tool as the caller, the thread the call's token names.
type tool func(ctx context.Context, caller string, req ctl.Request) (string, error)

// Commands answers the orchestrate command, which carries every agentos tool call.
type Commands struct {
	threads Threads
	chats   *chat.Chats
	router  *control.Router
	tools   map[string]tool

	waitMu  sync.Mutex
	waiting map[waitOn]held // each wait, and how long it lasts
	// given is, for a parent and its child, the latest result of the child's that went to the parent or is going, and
	// how: by a wait or as a message. Whichever takes a result first gives it, so the parent never gets it twice.
	given map[waitOn]givenBy

	ctx        func() context.Context // the app's, for the reports to stop with it
	reportMu   sync.Mutex
	reporting  map[string]bool   // the children whose results are being given to their parents now
	reportNext map[string]report // the next result to give for each of them, once the one in hand is given
}

// NewCommands answers the agentos tools as the threads s, and gives each child's result to its parent until the context
// ctx gives ends.
func NewCommands(s *sessions.Sessions, chats *chat.Chats, r *control.Router, ctx func() context.Context) *Commands {
	c := &Commands{threads: s, chats: chats, router: r, waiting: map[waitOn]held{}, given: map[waitOn]givenBy{}, ctx: ctx, reporting: map[string]bool{}, reportNext: map[string]report{}}
	c.tools = map[string]tool{"delegate": c.delegate, "threads": c.list, "read": c.read, "send": c.send, "wait": c.wait, "stop": c.stop}
	return c
}

// Orchestrate runs the tool req.Args[0] as the thread the request's token names.
func (c *Commands) Orchestrate(ctx context.Context, req ctl.Request) (string, error) {
	caller, err := c.router.Caller(req)
	if err != nil {
		return "", err
	}
	if len(req.Args) == 0 {
		return "", errors.New("This call names no agentos tool")
	}
	run, ok := c.tools[req.Args[0]]
	if !ok {
		return "", fmt.Errorf("agentos has no tool %q", req.Args[0])
	}
	return run(ctx, caller, req)
}
