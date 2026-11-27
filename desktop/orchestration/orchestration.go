// Package orchestration answers the orchestrator tools a thread's Claude calls: the orchestration tools, and the agent tools
// that do what the orchestrator commands did. Each call acts as the thread its token names.
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nednella/orchestrator/desktop/chat"
	"github.com/nednella/orchestrator/desktop/control"
	"github.com/nednella/orchestrator/desktop/threads"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/thread"
)

// Threads is what the tools use of the threads.
type Threads interface {
	CreateChild(ctx context.Context, o threads.ChildOptions) (threads.Thread, error)
	AccessOf(id string) thread.Access
	ChatOf(id string) (threads.Chat, error)
	List() []threads.Thread
	Projects() []threads.Project
	Title(id string) string
	Tell(id string, m threads.Told) (string, error)
	Descendants(id string) []string
	ParentOf(id string) string
	SideChat(id string) bool
	DropTold(id string) int
	HoldsTold(id string) bool
	Reported(id string) int64
	SetReported(id string, seq int64) error
	SettleIdle(id string) (bool, error)
}

// tool runs one orchestrator tool as the caller, the thread the call's token names.
type tool func(ctx context.Context, caller string, req ctl.Request) (string, error)

// Commands answers the orchestrate command, which carries every orchestrator tool call.
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
	retryFirst time.Duration          // the first pause before a result the parent could not take goes again
	reportMu   sync.Mutex
	reporting  map[string]bool   // the children whose results are being given to their parents now
	reportNext map[string]report // the next result to give for each of them, once the one in hand is given
}

// NewCommands answers the orchestrator tools as the threads s, and gives each child's result to its parent until the context
// ctx gives ends.
func NewCommands(s *threads.Threads, chats *chat.Chats, r *control.Router, ctx func() context.Context) *Commands {
	c := &Commands{threads: s, chats: chats, router: r, waiting: map[waitOn]held{}, given: map[waitOn]givenBy{}, ctx: ctx, retryFirst: defaultRetryFirst, reporting: map[string]bool{}, reportNext: map[string]report{}}
	c.tools = map[string]tool{"delegate": c.delegate, "threads": c.list, "read": c.read, "send": c.send, "wait": c.wait, "stop": c.stop}
	for name := range agentTools() {
		c.tools[name] = c.agentTool(name)
	}
	return c
}

// reachable errs for a side chat, which is the owner's alone, as in the thread list: to the tools it is no thread.
func (c *Commands) reachable(id string) error {
	if c.threads.SideChat(id) {
		return fmt.Errorf("No thread %s", id)
	}
	return nil
}

// SetRetryFirst sets the first pause before a result the parent could not take goes again, a second by default; the
// pause doubles each time. It comes before the first result.
func (c *Commands) SetRetryFirst(pause time.Duration) { c.retryFirst = pause }

// Orchestrate runs the tool req.Args[0] as the thread the request's token names.
func (c *Commands) Orchestrate(ctx context.Context, req ctl.Request) (string, error) {
	caller, err := c.router.Caller(req)
	if err != nil {
		return "", err
	}
	if len(req.Args) == 0 {
		return "", errors.New("This call names no orchestrator tool")
	}
	run, ok := c.tools[req.Args[0]]
	if !ok {
		return "", fmt.Errorf("orchestrator has no tool %q", req.Args[0])
	}
	return run(ctx, caller, req)
}
