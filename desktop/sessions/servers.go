package sessions

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/nednella/agentos/desktop/internal/run"
	"github.com/nednella/agentos/internal/procs"
	"github.com/nednella/agentos/internal/session"
)

// Server is one port a thread's server listens on, as the front end sees it.
type Server struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Command string `json:"command"`
}

// Servers are the ports the servers thread id started listen on, one row per port, by port: the processes that carry
// its marker and listen on TCP, without its host and Claude. An old terminal session has none: the app leaves it alone.
func (s *Sessions) Servers(id string) ([]Server, error) {
	list, err := s.serversOf(s.context(), id)
	if err != nil {
		return nil, err
	}
	rows := []Server{}
	for _, srv := range list {
		for _, port := range srv.Ports {
			rows = append(rows, Server{Port: port, PID: srv.PID, Command: srv.Command})
		}
	}
	slices.SortFunc(rows, func(a, b Server) int { return a.Port - b.Port })
	return rows, nil
}

// OpenServer opens http://localhost:<port> in the owner's default browser.
func (s *Sessions) OpenServer(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%d is not a port", port)
	}
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	if _, err := s.run(ctx, "", "open", "http://localhost:"+strconv.Itoa(port)); err != nil {
		return fmt.Errorf("opening localhost:%d: %w", port, err)
	}
	return nil
}

// serversOf are the servers of a thread this app knows, live, settled or deleted.
func (s *Sessions) serversOf(ctx context.Context, id string) ([]procs.Server, error) {
	name, err := session.ParseName(id)
	if err != nil {
		return nil, err
	}
	if name.IsShell() {
		return nil, errNotSession
	}
	s.mu.Lock()
	in, live := s.liveInfo(id)
	_, ended := s.ended[id]
	known, old := live || ended || s.deleted[id], s.isOld(id)
	s.mu.Unlock()
	switch {
	case old:
		return nil, nil
	case !known:
		return nil, fmt.Errorf("No thread %s", id)
	}
	host := 0
	if live {
		host = in.PID
	}
	return s.procs.Servers(ctx, map[string]int{id: host})
}

// anyDir runs commands through r in no folder of their own.
func anyDir(r run.Runner) procs.Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return r(ctx, "", name, args...)
	}
}
