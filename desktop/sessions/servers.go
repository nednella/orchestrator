package sessions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nednella/agentos/desktop/internal/run"
	"github.com/nednella/agentos/internal/procs"
	"github.com/nednella/agentos/internal/session"
)

// hostDial is how long the sweep waits for a thread's host to answer before it counts the host as gone.
const hostDial = 200 * time.Millisecond

// Server is one port a thread's server listens on, as the front end sees it.
type Server struct {
	Port    int    `json:"port"`
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Started int64  `json:"started"` // when its process started, in milliseconds since the epoch
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
			rows = append(rows, Server{Port: port, PID: srv.PID, Command: srv.Command, Started: srv.Started().UnixMilli()})
		}
	}
	slices.SortFunc(rows, func(a, b Server) int { return a.Port - b.Port })
	return rows, nil
}

// StopServer stops the server pid of thread id, once it finds pid among the thread's servers again, and returns once it
// has exited.
func (s *Sessions) StopServer(id string, pid int) error {
	ctx := s.context()
	list, err := s.serversOf(ctx, id)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(srv procs.Server) bool { return srv.PID == pid })
	if i < 0 {
		return errors.New("That server is not running")
	}
	<-s.procs.Stop(ctx, list[i:i+1]) // so the list read after it no longer has the server
	return nil
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

// stopServers stops the servers of the threads, which no longer run. Once Settle has stopped a thread its host is
// gone; elsewhere guarded leaves out a thread whose host answers, which runs: it may run under another app on the same
// state folder, which knows it as live.
func (s *Sessions) stopServers(guarded bool, ids ...string) {
	threads := map[string]int{}
	for _, id := range ids {
		threads[id] = 0
	}
	ctx := s.context()
	list, err := s.procs.Servers(ctx, threads)
	if err != nil {
		log.Printf("agentos: finding the servers of %s: %v", strings.Join(ids, ", "), err)
		return
	}
	if guarded {
		list = slices.DeleteFunc(list, func(srv procs.Server) bool {
			name, err := session.ParseName(srv.Thread)
			return err != nil || hostAnswers(s.hostSocket(name))
		})
	}
	s.procs.Stop(ctx, list)
}

// sweepServers stops at the app's start the servers of the threads this app knows have settled or were deleted,
// which a host that was killed left running.
func (s *Sessions) sweepServers() {
	s.mu.Lock()
	var ids []string
	for id, e := range s.ended {
		if e.info.Hosted {
			ids = append(ids, id)
		}
	}
	for id := range s.deleted {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	if len(ids) > 0 {
		s.stopServers(true, ids...)
	}
}

// anyDir runs commands through r in no folder of their own.
func anyDir(r run.Runner) procs.Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return r(ctx, "", name, args...)
	}
}

// hostAnswers says whether a host listens on socket.
func hostAnswers(socket string) bool {
	conn, err := net.DialTimeout("unix", socket, hostDial)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
