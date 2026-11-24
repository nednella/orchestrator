// Package host runs one thread's Claude headless, keeps its chat log and serves a unix socket the app and the agentos command talk to.
package host

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nednella/agentos/internal/chat"
)

type Config struct {
	Log, Socket, Dir string
	Argv, Env        []string
	Echo             io.Writer
}

const (
	stopGrace      = 5 * time.Second
	requestTimeout = 10 * time.Second
	shutdownGrace  = time.Second
	echoWidth      = 200
	// subscriberBuffer is how many events a subscriber may fall behind before it is dropped.
	subscriberBuffer = 1024
)

type subscriber struct {
	ch   chan chat.Event
	conn net.Conn
	// mark is the newest Seq in the log when the subscriber was added: the replay sends up to it, ch carries the rest.
	mark int64
}

type host struct {
	logPath string
	log     *chat.Log
	echo    io.Writer

	stdinMu  sync.Mutex
	stdin    io.Writer
	stdinErr error
	reqID    atomic.Int64

	handlers sync.WaitGroup

	mu        sync.Mutex
	closed    bool
	exited    bool
	subs      map[*subscriber]struct{}
	conns     map[net.Conn]struct{}
	approvals map[string]json.RawMessage
}

// Run starts Claude and serves the socket until Claude exits, then returns Claude's exit error. Cancelling ctx stops Claude: SIGTERM, then SIGKILL after 5 s.
func Run(ctx context.Context, c Config) error {
	if len(c.Argv) == 0 {
		return errors.New("host: no command to run")
	}
	ln, err := listen(c.Socket)
	if err != nil {
		return err
	}
	log, err := chat.OpenLog(c.Log)
	if err != nil {
		ln.Close()
		return err
	}
	echo := c.Echo
	if echo == nil {
		echo = io.Discard
	}
	h := &host{
		logPath:   c.Log,
		log:       log,
		echo:      &lockedWriter{w: echo},
		subs:      map[*subscriber]struct{}{},
		conns:     map[net.Conn]struct{}{},
		approvals: map[string]json.RawMessage{},
	}
	defer h.shutdown(ln)

	cmd := exec.CommandContext(ctx, c.Argv[0], c.Argv[1:]...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.Stderr = h.echo
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = stopGrace
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		h.publishExit(err)
		return err
	}
	h.stdin = stdin
	if err := h.write(chat.Initialize(h.nextID())); err != nil {
		fmt.Fprintf(h.echo, "host: initialize: %v\n", err)
	}
	h.handlers.Add(1)
	go h.serve(ln)

	h.read(stdout)
	h.mu.Lock()
	h.exited = true
	h.mu.Unlock()
	err = cmd.Wait()
	h.publishExit(err)
	return err
}

var errExited = errors.New("claude has exited")

func (h *host) publishExit(err error) {
	e := chat.Event{Kind: chat.KindExit}
	if err != nil {
		e.Text = err.Error()
		e.Error = true
	}
	if _, err := h.publish(e); err != nil {
		fmt.Fprintf(h.echo, "host: %v\n", err)
	}
}

// shutdown stops taking connections, ends every stream once it has sent what it holds, and closes the log.
func (h *host) shutdown(ln net.Listener) {
	ln.Close()
	h.mu.Lock()
	h.closed = true
	for s := range h.subs {
		delete(h.subs, s)
		close(s.ch)
	}
	h.mu.Unlock()

	done := make(chan struct{})
	go func() {
		h.handlers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownGrace):
		h.mu.Lock()
		for conn := range h.conns {
			conn.Close()
		}
		h.mu.Unlock()
		<-done
	}
	h.log.Close()
}

func (h *host) read(stdout io.Reader) {
	r := bufio.NewReader(stdout)
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			h.handleLine(line)
		}
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Fprintf(h.echo, "host: read claude: %v\n", err)
			return
		}
	}
}

func (h *host) handleLine(line []byte) {
	events, err := chat.Parse(line)
	if err != nil {
		fmt.Fprintf(h.echo, "host: %v\n", err)
		return
	}
	for _, e := range events {
		if _, err := h.publish(e); err != nil {
			fmt.Fprintf(h.echo, "host: %v\n", err)
		}
	}
}

// publish logs e and hands it to every subscriber. Both happen under h.mu, so a subscriber added in between sees e in the log or on its channel, never both and never neither. Once Claude's stdout has ended, only the exit event is taken.
func (h *host) publish(e chat.Event) (chat.Event, error) {
	h.mu.Lock()
	if h.exited && e.Kind != chat.KindExit {
		h.mu.Unlock()
		return chat.Event{}, errExited
	}
	e, err := h.log.Append(e)
	if err != nil {
		h.mu.Unlock()
		return chat.Event{}, err
	}
	switch e.Kind {
	case chat.KindApproval:
		h.approvals[e.ID] = e.Input
	case chat.KindExit:
		h.exited = true
	}
	for s := range h.subs {
		select {
		case s.ch <- e:
		default:
			delete(h.subs, s)
			close(s.ch)
			s.conn.Close()
		}
	}
	h.mu.Unlock()
	if e.Persisted() {
		fmt.Fprintln(h.echo, echoLine(e))
	}
	return e, nil
}

func (h *host) write(line []byte) error {
	h.stdinMu.Lock()
	defer h.stdinMu.Unlock()
	return h.writeLocked(line)
}

// writeLocked must be called with stdinMu held. After one failed write every later one fails without trying, so a send can refuse before it logs anything.
func (h *host) writeLocked(line []byte) error {
	if h.stdinErr != nil {
		return h.stdinErr
	}
	if _, err := h.stdin.Write(line); err != nil {
		h.stdinErr = fmt.Errorf("claude's stdin is closed: %w", err)
		return h.stdinErr
	}
	return nil
}

func (h *host) running() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.exited {
		return errExited
	}
	return nil
}

func (h *host) nextID() string {
	return fmt.Sprintf("aos-%d", h.reqID.Add(1))
}

func (h *host) serve(ln net.Listener) {
	defer h.handlers.Done()
	for {
		conn, err := ln.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			fmt.Fprintf(h.echo, "host: accept: %v\n", err)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if !h.track(conn) {
			conn.Close()
			continue
		}
		go h.handle(conn)
	}
}

func (h *host) track(conn net.Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	h.conns[conn] = struct{}{}
	h.handlers.Add(1)
	return true
}

func (h *host) handle(conn net.Conn) {
	defer h.handlers.Done()
	defer func() {
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
		conn.Close()
	}()
	conn.SetReadDeadline(time.Now().Add(requestTimeout))
	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return
	}
	conn.SetReadDeadline(time.Time{})
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		writeReply(conn, fmt.Errorf("bad request: %w", err))
		return
	}
	if req.Op == opSubscribe {
		h.subscribe(conn, r, req.After)
		return
	}
	writeReply(conn, h.do(req))
}

func (h *host) do(req request) error {
	switch req.Op {
	case opSend:
		h.stdinMu.Lock()
		failed := h.stdinErr
		h.stdinMu.Unlock()
		if failed != nil {
			return failed
		}
		if _, err := h.publish(chat.Event{Kind: chat.KindUser, Text: req.Text}); err != nil {
			return err
		}
		return h.write(chat.UserMessage(req.Text))
	case opDraft:
		_, err := h.publish(chat.Event{Kind: chat.KindDraft, Text: req.Text})
		return err
	case opInterrupt:
		if err := h.running(); err != nil {
			return err
		}
		return h.write(chat.Interrupt(h.nextID()))
	case opAnswer:
		return h.answer(req.ID, req.Allow, req.Message)
	}
	return fmt.Errorf("unknown op %q", req.Op)
}

func (h *host) answer(id string, allow bool, message string) error {
	h.mu.Lock()
	if h.exited {
		h.mu.Unlock()
		return errExited
	}
	input, ok := h.approvals[id]
	if !ok {
		h.mu.Unlock()
		return fmt.Errorf("no approval %q is waiting", id)
	}
	// chat.Allow panics on input that is not JSON, and a panic here would end the thread's Claude.
	if allow && len(input) > 0 && !json.Valid(input) {
		h.mu.Unlock()
		return fmt.Errorf("approval %q has input that is not JSON", id)
	}
	delete(h.approvals, id)
	h.mu.Unlock()

	line, text := chat.Deny(id, message), "deny"
	if allow {
		line, text = chat.Allow(id, input), "allow"
	}
	if _, err := h.publish(chat.Event{Kind: chat.KindAnswer, ID: id, Text: text}); err != nil {
		return err
	}
	return h.write(line)
}

// subscribe sends the logged events after `after`, then live events, until the client hangs up, falls too far behind or the host stops.
func (h *host) subscribe(conn net.Conn, r *bufio.Reader, after int64) {
	s, err := h.addSubscriber(conn)
	if err != nil {
		writeReply(conn, err)
		return
	}
	defer h.removeSubscriber(s)
	logged, err := chat.ReadLog(h.logPath, after)
	if err != nil {
		writeReply(conn, err)
		return
	}
	if writeReply(conn, nil) != nil {
		return
	}
	enc := json.NewEncoder(conn)
	for _, e := range logged {
		if e.Seq > s.mark {
			break
		}
		if enc.Encode(e) != nil {
			return
		}
	}
	gone := make(chan struct{})
	go func() {
		io.Copy(io.Discard, r)
		close(gone)
	}()
	for {
		select {
		case e, ok := <-s.ch:
			if !ok {
				return
			}
			if e.Persisted() && e.Seq <= after {
				continue
			}
			if enc.Encode(e) != nil {
				return
			}
		case <-gone:
			return
		}
	}
}

func (h *host) addSubscriber(conn net.Conn) (*subscriber, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, errors.New("host is stopping")
	}
	s := &subscriber{ch: make(chan chat.Event, subscriberBuffer), conn: conn, mark: h.log.Last()}
	h.subs[s] = struct{}{}
	return s, nil
}

func (h *host) removeSubscriber(s *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, s)
}

func writeReply(conn net.Conn, err error) error {
	r := reply{OK: err == nil}
	if err != nil {
		r.Error = err.Error()
	}
	b, _ := json.Marshal(r)
	_, werr := conn.Write(append(b, '\n'))
	return werr
}

func listen(path string) (net.Listener, error) {
	if len(path) > maxSocketPath {
		return nil, fmt.Errorf("socket path %s is %d bytes, over the %d a unix socket allows", path, len(path), maxSocketPath)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := removeStale(path); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// removeStale removes a socket file left by a host that died, and refuses one a live host still listens on.
func removeStale(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s exists and is not a socket", path)
	}
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err == nil {
		conn.Close()
		return fmt.Errorf("a host already listens on %s", path)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("check socket %s: %w", path, err)
	}
	return os.Remove(path)
}

func echoLine(e chat.Event) string {
	var s string
	switch e.Kind {
	case chat.KindInit:
		s = "session " + e.ID
	case chat.KindUser:
		s = "you: " + e.Text
	case chat.KindMessage:
		s = "claude: " + e.Text
	case chat.KindToolUse:
		s = "tool: " + e.Tool + " " + string(e.Input)
	case chat.KindToolResult:
		s = "result: " + e.Text
		if e.Error {
			s = "tool error: " + e.Text
		}
	case chat.KindApproval:
		s = "asks to use " + e.Tool + " " + string(e.Input)
	case chat.KindAnswer:
		s = "answer: " + e.Text
	case chat.KindTurnEnd:
		s = fmt.Sprintf("turn ended, $%.4f", e.Cost)
		if e.Error {
			s += ", with an error"
		}
	case chat.KindDraft:
		s = "draft: " + e.Text
	case chat.KindExit:
		s = "claude exited"
		if e.Text != "" {
			s += ": " + e.Text
		}
	default:
		s = string(e.Kind)
	}
	return clip(strings.Join(strings.Fields(s), " "), echoWidth)
}

func clip(s string, runes int) string {
	n := 0
	for i := range s {
		if n == runes {
			return s[:i] + "…"
		}
		n++
	}
	return s
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
