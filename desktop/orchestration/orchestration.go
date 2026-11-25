// Package orchestration answers the agentos tools a thread's Claude calls. Each call acts as the thread its token
// names.
package orchestration

import (
	"context"
	"errors"
	"fmt"

	"github.com/nednella/agentos/desktop/chat"
	"github.com/nednella/agentos/desktop/control"
	"github.com/nednella/agentos/desktop/sessions"
	ctl "github.com/nednella/agentos/internal/control"
)

// Threads is what the tools use of the threads.
type Threads interface{}

// tool runs one agentos tool as the caller, the thread the call's token names.
type tool func(ctx context.Context, caller string, req ctl.Request) (string, error)

// Commands answers the orchestrate command, which carries every agentos tool call.
type Commands struct {
	threads Threads
	chats   *chat.Chats
	router  *control.Router
	tools   map[string]tool
}

func NewCommands(s *sessions.Sessions, c *chat.Chats, r *control.Router) *Commands {
	return &Commands{threads: s, chats: c, router: r, tools: map[string]tool{}}
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
