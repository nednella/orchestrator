// Package host runs one thread's Claude headless, keeps its chat log and serves a unix socket the app and the agentos command talk to.
package host

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/prompts"
)

type Config struct {
	Log, Socket, Dir string
	Argv, Env        []string
	Echo             io.Writer
	// Interrupts interrupts Claude's turn at each signal, as the socket's interrupt op does.
	Interrupts <-chan os.Signal
}

const (
	stopGrace      = 5 * time.Second
	requestTimeout = 10 * time.Second
	shutdownGrace  = time.Second
	echoWidth      = 200
	// subscriberBuffer is how many events a subscriber may fall behind before it is dropped.
	subscriberBuffer = 1024
	// maxImage keeps an image's base64, which the API limits to 5 MB, under that limit.
	maxImage = 3932160
)

var imageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

type approval struct {
	id    string
	input json.RawMessage
}

type queuedMessage struct {
	text  string
	files []string
}

// interruptRequest is an interrupt written to Claude and the messages that were queued then.
type interruptRequest struct {
	id     string
	queued []string
}

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

	echoMu      sync.Mutex
	echoSession string  // the session id the echo showed last
	echoTotal   float64 // Claude's running cost at the last turn's end

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
	approvals []approval // in the order Claude asked
	// turnOpen is set from writing a user line, or from Claude taking in a queued one, until the turn ends.
	turnOpen bool
	// queued holds the messages written during a turn that Claude has not taken in yet. Claude runs one written during
	// a turn's last step as a turn of its own after that turn ends, so the host stays busy while any is left.
	queued map[string]queuedMessage
	// interrupted is the latest interrupt that Claude has not answered yet.
	interrupted interruptRequest
}

// Run starts Claude and serves the socket until Claude exits, then returns Claude's exit error. Cancelling ctx stops Claude and the processes it started: SIGTERM, then SIGKILL after 5 s.
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
		logPath: c.Log,
		log:     log,
		echo:    &lockedWriter{w: echo},
		subs:    map[*subscriber]struct{}{},
		conns:   map[net.Conn]struct{}{},
		queued:  map[string]queuedMessage{},
	}
	defer h.shutdown(ln)

	cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.Stderr = h.echo
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
	waited := make(chan struct{})
	go stopOnCancel(ctx, cmd.Process.Pid, waited)
	if err := h.write(chat.Initialize(h.nextID())); err != nil {
		fmt.Fprintf(h.echo, "host: initialize: %v\n", err)
	}
	h.handlers.Add(1)
	go h.serve(ln)
	go h.interruptOn(c.Interrupts, waited)

	h.read(stdout)
	h.mu.Lock()
	h.exited = true
	h.mu.Unlock()
	err = cmd.Wait()
	close(waited)
	h.publishExit(err)
	return err
}

// stopOnCancel stops Claude's whole process group when ctx ends: SIGTERM, then SIGKILL after stopGrace. The group takes in a process Claude started that still holds its stdout, which would keep Run reading.
func stopOnCancel(ctx context.Context, pid int, waited <-chan struct{}) {
	select {
	case <-ctx.Done():
	case <-waited:
		return
	}
	syscall.Kill(-pid, syscall.SIGTERM)
	select {
	case <-waited:
	case <-time.After(stopGrace):
		syscall.Kill(-pid, syscall.SIGKILL)
	}
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
		switch e.Kind {
		case chat.KindEcho:
			h.echoed(e.ID)
			continue
		case chat.KindStillQueued:
			h.stillQueued(e.ID, e.Input)
			continue
		}
		if _, err := h.publish(e); err != nil {
			fmt.Fprintf(h.echo, "host: %v\n", err)
		}
	}
}

// publish logs e and hands it to every subscriber.
func (h *host) publish(e chat.Event) (chat.Event, error) {
	h.mu.Lock()
	e, err := h.appendLocked(e)
	h.mu.Unlock()
	if err != nil {
		return chat.Event{}, err
	}
	h.echoEvent(e)
	return e, nil
}

// appendLocked must be called with h.mu held. Logging and fan-out both happen under h.mu, so a subscriber added in between sees e in the log or on its channel, never both and never neither. Once Claude's stdout has ended, only the exit event is taken.
func (h *host) appendLocked(e chat.Event) (chat.Event, error) {
	if h.exited && e.Kind != chat.KindExit {
		return chat.Event{}, errExited
	}
	e, err := h.log.Append(e)
	if err != nil {
		return chat.Event{}, err
	}
	switch e.Kind {
	case chat.KindApproval:
		h.approvals = append(h.approvals, approval{id: e.ID, input: e.Input})
	case chat.KindTurnEnd:
		h.approvals = nil
		h.turnOpen = false
	case chat.KindExit:
		h.exited = true
		h.approvals = nil
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
	return e, nil
}

// echoEvent shows e in the pane. Claude sends its session id at every turn and its running cost at every turn's end,
// so the echo shows the id only when it changes and the cost as the turn's own; a total below the last starts a new
// count, and a result with no cost shows none and keeps the count.
func (h *host) echoEvent(e chat.Event) {
	if !e.Persisted() || e.Kind == chat.KindCommands {
		return
	}
	h.echoMu.Lock()
	defer h.echoMu.Unlock()
	switch e.Kind {
	case chat.KindInit:
		if e.ID == h.echoSession {
			return
		}
		h.echoSession = e.ID
	case chat.KindTurnEnd:
		if total := e.Cost; total > 0 {
			if total >= h.echoTotal {
				e.Cost -= h.echoTotal
			}
			h.echoTotal = total
		}
	}
	fmt.Fprintln(h.echo, echoLine(e))
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

// interruptOn interrupts Claude's turn at each signal until done closes.
func (h *host) interruptOn(signals <-chan os.Signal, done <-chan struct{}) {
	for {
		select {
		case <-signals:
			if err := h.interrupt(""); err != nil {
				fmt.Fprintf(h.echo, "host: interrupt: %v\n", err)
			}
		case <-done:
			return
		}
	}
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
	line, err := bufio.NewReader(io.LimitReader(conn, maxRequest+1)).ReadBytes('\n')
	if err != nil {
		if len(line) > maxRequest {
			writeReply(conn, "", errors.New("request is over 1 MB"))
		}
		return
	}
	conn.SetReadDeadline(time.Time{})
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		writeReply(conn, "", fmt.Errorf("bad request: %w", err))
		return
	}
	if req.Op == opSubscribe {
		h.subscribe(conn, req.After)
		return
	}
	id, err := h.do(req)
	writeReply(conn, id, err)
}

// do runs one request and returns the id a send logged its message under.
func (h *host) do(req request) (string, error) {
	switch req.Op {
	case opSend:
		return h.send(req.Text, req.Files)
	case opSendNow:
		if req.ID == "" {
			return "", errors.New("send_now needs the id of a queued message")
		}
		return "", h.interrupt(req.ID)
	case opDraft:
		_, err := h.publish(chat.Event{Kind: chat.KindDraft, Text: req.Text})
		return "", err
	case opInterrupt:
		return "", h.interrupt("")
	case opAnswer:
		return "", h.answer(req.ID, req.Allow, req.Message, req.Input)
	}
	return "", fmt.Errorf("unknown op %q", req.Op)
}

// send gives the user's message to Claude and returns its id. While an approval waits, the message denies each waiting
// tool with the text as the reason and is not written as a message. While a turn runs, it is logged as queued and
// written at once: Claude takes it in at the turn's next step, or runs it as its own turn after this one, and its echo
// then logs it as the user's. Logging and writing happen under stdinMu, so the log and stdin hold messages in the same
// order. The lock order is stdinMu, then h.mu.
func (h *host) send(text string, files []string) (string, error) {
	images, err := readImages(files)
	if err != nil {
		return "", err
	}
	id := newMessageID()
	h.stdinMu.Lock()
	defer h.stdinMu.Unlock()
	if h.stdinErr != nil {
		return "", h.stdinErr
	}
	h.mu.Lock()
	lines, logged, err := h.sendLocked(id, text, files, images)
	h.mu.Unlock()
	for _, e := range logged {
		h.echoEvent(e)
	}
	for _, line := range lines {
		if werr := h.writeLocked(line); werr != nil {
			return "", werr
		}
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// sendLocked must be called with stdinMu and h.mu held. It logs the message and returns the lines to write for what it
// logged, also when it fails partway.
func (h *host) sendLocked(id, text string, files []string, images []chat.Image) (lines [][]byte, logged []chat.Event, err error) {
	log := func(e chat.Event) error {
		e, err := h.appendLocked(e)
		if err == nil {
			logged = append(logged, e)
		}
		return err
	}
	if len(h.approvals) > 0 {
		if len(files) > 0 {
			return nil, nil, errors.New("Answer the approval before sending images")
		}
		parent := h.approvals[0].id
		for len(h.approvals) > 0 {
			a := h.approvals[0]
			if err := log(chat.Event{Kind: chat.KindAnswer, ID: a.id, Text: "deny"}); err != nil {
				return lines, logged, err
			}
			h.approvals = h.approvals[1:]
			lines = append(lines, chat.Deny(a.id, prompts.Denied(text)))
		}
		return lines, logged, log(chat.Event{Kind: chat.KindUser, ID: id, Text: text, Parent: parent})
	}
	line := chat.UserMessageWith(id, text, images)
	if h.turnOpen || len(h.queued) > 0 {
		if err := log(chat.Event{Kind: chat.KindQueued, ID: id, Text: text, Files: files}); err != nil {
			return nil, logged, err
		}
		h.queued[id] = queuedMessage{text: text, files: files}
		return [][]byte{line}, logged, nil
	}
	if err := log(chat.Event{Kind: chat.KindUser, ID: id, Text: text, Files: files}); err != nil {
		return nil, logged, err
	}
	h.turnOpen = true
	return [][]byte{line}, logged, nil
}

// echoed handles Claude taking in the user message id. A queued one is logged as the user's now, and its turn is open:
// Claude may run it as a turn of its own after the turn it was written during.
func (h *host) echoed(id string) {
	h.mu.Lock()
	m, ok := h.queued[id]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.queued, id)
	h.turnOpen = true
	e, err := h.appendLocked(chat.Event{Kind: chat.KindUser, ID: id, Text: m.text, Files: m.files})
	h.mu.Unlock()
	if err != nil {
		fmt.Fprintf(h.echo, "host: %v\n", err)
		return
	}
	h.echoEvent(e)
}

// interrupt logs an interrupt and writes it. With queued set, it is Send now for that queued message: Claude answers the
// interrupt with the messages it still holds and runs them next, so the message is not written again.
func (h *host) interrupt(queued string) error {
	h.stdinMu.Lock()
	defer h.stdinMu.Unlock()
	if h.stdinErr != nil {
		return h.stdinErr
	}
	h.mu.Lock()
	if _, ok := h.queued[queued]; queued != "" && !ok {
		h.mu.Unlock()
		return fmt.Errorf("no queued message %s", queued)
	}
	e, err := h.appendLocked(chat.Event{Kind: chat.KindInterrupt, ID: queued})
	if err != nil {
		h.mu.Unlock()
		return err
	}
	req := h.nextID()
	h.interrupted = interruptRequest{id: req, queued: slices.Collect(maps.Keys(h.queued))}
	h.mu.Unlock()
	h.echoEvent(e)
	return h.writeLocked(chat.Interrupt(req))
}

// stillQueued drops each message that was queued when the interrupt request was written and that Claude no longer
// holds, and logs it unsent: an interrupted turn can end without taking one in, and then no echo comes to end its wait.
func (h *host) stillQueued(request string, held json.RawMessage) {
	var ids []string
	if err := json.Unmarshal(held, &ids); err != nil {
		fmt.Fprintf(h.echo, "host: interrupt reply: %v\n", err)
		return
	}
	h.mu.Lock()
	if h.interrupted.id != request {
		h.mu.Unlock()
		return
	}
	var unsent []chat.Event
	for _, id := range h.interrupted.queued {
		m, ok := h.queued[id]
		if !ok || slices.Contains(ids, id) {
			continue
		}
		delete(h.queued, id)
		e, err := h.appendLocked(chat.Event{Kind: chat.KindUnsent, ID: id, Text: m.text})
		if err != nil {
			fmt.Fprintf(h.echo, "host: %v\n", err)
			continue
		}
		unsent = append(unsent, e)
	}
	h.interrupted = interruptRequest{}
	h.mu.Unlock()
	for _, e := range unsent {
		h.echoEvent(e)
	}
}

// answer takes the approval and logs the answer in one step under h.mu, so a turn that ends meanwhile cannot leave a stale answer to write, and a failed log keeps the approval open. An allow with input sends it in place of the tool's own.
func (h *host) answer(id string, allow bool, message string, given json.RawMessage) error {
	if len(given) > 0 && !isObject(given) {
		return errors.New("an answer's input must be a JSON object")
	}
	h.stdinMu.Lock()
	defer h.stdinMu.Unlock()
	h.mu.Lock()
	if h.exited {
		h.mu.Unlock()
		return errExited
	}
	i := h.approvalIndex(id)
	if i < 0 {
		h.mu.Unlock()
		return fmt.Errorf("no approval %q is waiting", id)
	}
	input := h.approvals[i].input
	if len(given) > 0 {
		input = given
	}
	// chat.Allow panics on input that is not JSON, and a panic here would end the thread's Claude.
	if allow && len(input) > 0 && !json.Valid(input) {
		h.mu.Unlock()
		return fmt.Errorf("approval %q has input that is not JSON", id)
	}
	line, text := chat.Deny(id, message), "deny"
	if allow {
		line, text = chat.Allow(id, input), "allow"
	}
	e, err := h.appendLocked(chat.Event{Kind: chat.KindAnswer, ID: id, Text: text})
	if err != nil {
		h.mu.Unlock()
		return err
	}
	h.approvals = append(h.approvals[:i], h.approvals[i+1:]...)
	h.mu.Unlock()
	h.echoEvent(e)
	return h.writeLocked(line)
}

// approvalIndex must be called with h.mu held; it is -1 when no approval id waits.
func (h *host) approvalIndex(id string) int {
	for i, a := range h.approvals {
		if a.id == id {
			return i
		}
	}
	return -1
}

func isObject(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	return json.Unmarshal(raw, &m) == nil && m != nil
}

// readImages reads each file as an image block's data, refusing one that is too big or not a picture Claude takes.
func readImages(files []string) ([]chat.Image, error) {
	images := make([]chat.Image, 0, len(files))
	for _, path := range files {
		data, err := readLimited(path, maxImage)
		if err != nil {
			return nil, err
		}
		mime := http.DetectContentType(data)
		if !imageTypes[mime] {
			return nil, fmt.Errorf("%s is not a PNG, JPEG, GIF or WebP image", path)
		}
		images = append(images, chat.Image{Mime: mime, Data: base64.StdEncoding.EncodeToString(data)})
	}
	return images, nil
}

func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is over %.2f MB", path, float64(limit)/(1<<20))
	}
	return data, nil
}

// newMessageID is a random UUID v4, the uuid Claude echoes back when it takes the message in.
func newMessageID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// subscribe sends the logged events after `after`, then live events, until the client hangs up, falls too far behind or the host stops.
func (h *host) subscribe(conn net.Conn, after int64) {
	s, err := h.addSubscriber(conn)
	if err != nil {
		writeReply(conn, "", err)
		return
	}
	defer h.removeSubscriber(s)
	logged, err := chat.ReadLog(h.logPath, after)
	if err != nil {
		writeReply(conn, "", err)
		return
	}
	if writeReply(conn, "", nil) != nil {
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
		io.Copy(io.Discard, conn)
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

func writeReply(conn net.Conn, id string, err error) error {
	r := reply{OK: err == nil, ID: id}
	if err != nil {
		r.Error = err.Error()
	}
	b, _ := json.Marshal(r)
	_, werr := conn.Write(append(b, '\n'))
	return werr
}

func listen(path string) (net.Listener, error) {
	if len(path) > MaxSocketPath {
		return nil, fmt.Errorf("socket path %s is %d bytes, over the %d a unix socket allows", path, len(path), MaxSocketPath)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := checkPrivateDir(dir); err != nil {
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

// checkPrivateDir refuses a socket folder another user could reach: the socket is open to anyone in the folder until it is made 0600.
func checkPrivateDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o700 {
		return fmt.Errorf("socket folder %s has mode %o, want 700", dir, info.Mode().Perm())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("socket folder %s is owned by another user", dir)
	}
	return nil
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
		s = "turn ended"
		if e.Cost > 0 {
			s += fmt.Sprintf(", $%.4f", e.Cost)
		}
		if e.Error {
			s += ", with an error"
		}
	case chat.KindDraft:
		s = "draft: " + e.Text
	case chat.KindQueued:
		s = "queued: " + e.Text
	case chat.KindUnsent:
		s = "not sent: " + e.Text
	case chat.KindInterrupt:
		s = "interrupted"
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
