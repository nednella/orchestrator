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
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/nednella/agentos/internal/chat"
)

const fakeInit = `{"type":"system","subtype":"init","cwd":"/tmp/rec","session_id":"fake-session","model":"claude-haiku-5-5","permissionMode":"default"}`

// TestHelperFakeClaude is not a test: the host tests run this binary as Claude.
func TestHelperFakeClaude(t *testing.T) {
	if os.Getenv("AOS_FAKE_CLAUDE") != "1" {
		return
	}
	if os.Getenv("AOS_FAKE_IGNORE_TERM") == "1" {
		signal.Ignore(syscall.SIGTERM)
	}
	if pidFile := os.Getenv("AOS_FAKE_GRANDCHILD"); pidFile != "" {
		holder := exec.Command("sleep", "30")
		holder.Stdout = os.Stdout
		if err := holder.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.WriteFile(pidFile, []byte(fmt.Sprint(holder.Process.Pid)), 0o600)
	}
	fakeClaude(os.Stdin, os.Stdout, os.Getenv("AOS_FAKE_STDIN"))
	os.Exit(0)
}

func fakeClaude(in io.Reader, out io.Writer, stdinLog string) {
	logf, err := os.OpenFile(stdinLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer logf.Close()
	print := func(v any) {
		b, _ := json.Marshal(v)
		out.Write(append(b, '\n'))
	}
	result := func(isError bool) {
		print(map[string]any{"type": "result", "subtype": "success", "is_error": isError, "total_cost_usd": 0.01})
	}
	fmt.Fprintln(out, fakeInit)

	r := bufio.NewReader(in)
	turn := 0
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			logf.Write(line)
		}
		if err != nil {
			return
		}
		var msg struct {
			Type    string `json:"type"`
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
			RequestID string `json:"request_id"`
			Request   struct {
				Subtype string `json:"subtype"`
			} `json:"request"`
			Response struct {
				RequestID string `json:"request_id"`
				Response  struct {
					Behavior string `json:"behavior"`
				} `json:"response"`
			} `json:"response"`
		}
		if json.Unmarshal(line, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "control_request":
			print(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": msg.RequestID}})
			if msg.Request.Subtype == "interrupt" {
				result(true)
			}
		case "control_response":
			if msg.Response.RequestID == "r1" {
				result(msg.Response.Response.Behavior != "allow")
			}
		case "user":
			turn++
			text := ""
			if len(msg.Message.Content) > 0 {
				text = msg.Message.Content[0].Text
			}
			id := fmt.Sprintf("msg_%d", turn)
			switch text {
			case "ask":
				print(map[string]any{"type": "control_request", "request_id": "r1", "request": map[string]any{
					"subtype": "can_use_tool", "tool_name": "Bash", "input": map[string]any{"command": "ls"},
				}})
				continue
			case "big":
				text = strings.Repeat("x", 1<<20)
			case "quit":
				os.Stdout.Close()
				time.Sleep(time.Second)
				os.Exit(0)
			default:
				print(map[string]any{"type": "stream_event", "api_message_id": id, "event": map[string]any{
					"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": text},
				}})
			}
			print(map[string]any{"type": "assistant", "uuid": "u" + id, "message": map[string]any{
				"id": id, "role": "assistant", "content": []any{map[string]any{"type": "text", "text": text}},
			}})
			result(false)
		}
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type testHost struct {
	socket, log, stdin string
	echo               *syncBuffer
	cancel             context.CancelFunc
	done               chan error
}

// socketDir is under /tmp because t.TempDir() on macOS can push a socket path past its 104-byte limit.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func startHost(t *testing.T, env ...string) *testHost {
	t.Helper()
	dir := t.TempDir()
	h := &testHost{
		socket: filepath.Join(socketDir(t), "h.sock"),
		log:    filepath.Join(dir, "chat.jsonl"),
		stdin:  filepath.Join(dir, "stdin.jsonl"),
		echo:   &syncBuffer{},
		done:   make(chan error, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	c := Config{
		Log:    h.log,
		Socket: h.socket,
		Dir:    dir,
		Argv:   []string{os.Args[0], "-test.run=^TestHelperFakeClaude$"},
		Env:    append(os.Environ(), append([]string{"AOS_FAKE_CLAUDE=1", "AOS_FAKE_STDIN=" + h.stdin}, env...)...),
		Echo:   h.echo,
	}
	go func() { h.done <- Run(ctx, c) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-h.done:
		case <-time.After(10 * time.Second):
			t.Error("host did not stop")
		}
	})
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer readyCancel()
	if err := Ready(readyCtx, h.socket); err != nil {
		t.Fatal(err)
	}
	return h
}

func subscribe(t *testing.T, socket string, after int64) <-chan chat.Event {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan chat.Event, 4096)
	done := make(chan struct{})
	go func() {
		defer close(done)
		Subscribe(ctx, socket, after, func(e chat.Event) { ch <- e })
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return ch
}

func next(t *testing.T, ch <-chan chat.Event) chat.Event {
	t.Helper()
	select {
	case e := <-ch:
		return e
	case <-time.After(10 * time.Second):
		t.Fatal("no event within 10s")
		return chat.Event{}
	}
}

func waitKind(t *testing.T, ch <-chan chat.Event, kind chat.Kind) chat.Event {
	t.Helper()
	for {
		if e := next(t, ch); e.Kind == kind {
			return e
		}
	}
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func stdinLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func ctxT(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestSendReachesClaudeAndStreamsBack(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "hi"); err != nil {
		t.Fatal(err)
	}
	var kinds []chat.Kind
	var message chat.Event
	for len(kinds) < 4 {
		e := next(t, events)
		kinds = append(kinds, e.Kind)
		if e.Kind == chat.KindMessage {
			message = e
		}
	}
	want := []chat.Kind{chat.KindUser, chat.KindDelta, chat.KindMessage, chat.KindTurnEnd}
	if fmt.Sprint(kinds) != fmt.Sprint(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	if message.Text != "hi" {
		t.Fatalf("message = %q, want hi", message.Text)
	}
	eventually(t, "the echo of the reply", func() bool {
		echo := h.echo.String()
		return strings.Contains(echo, "you: hi") && strings.Contains(echo, "claude: hi")
	})
}

func TestApprovalRoundTrip(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "ask"); err != nil {
		t.Fatal(err)
	}
	approval := waitKind(t, events, chat.KindApproval)
	if approval.ID != "r1" || approval.Tool != "Bash" {
		t.Fatalf("approval = %+v, want id r1 for Bash", approval)
	}
	if err := Answer(ctxT(t), h.socket, "r1", true, ""); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindAnswer); e.ID != "r1" || e.Text != "allow" {
		t.Fatalf("answer = %+v, want r1 allow", e)
	}
	if e := waitKind(t, events, chat.KindTurnEnd); e.Error {
		t.Fatal("turn ended with an error after allow")
	}
	var allow string
	for _, l := range stdinLines(t, h.stdin) {
		if strings.Contains(l, `"control_response"`) {
			allow = l
		}
	}
	if !strings.Contains(allow, `"behavior":"allow"`) || !strings.Contains(allow, `"updatedInput":{"command":"ls"}`) {
		t.Fatalf("control_response = %s, want allow with the tool's input", allow)
	}
	if err := Answer(ctxT(t), h.socket, "r1", true, ""); err == nil {
		t.Fatal("a second answer to r1 was accepted")
	}
}

func TestDenyEndsTheTurnWithAnError(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "ask"); err != nil {
		t.Fatal(err)
	}
	waitKind(t, events, chat.KindApproval)
	if err := Answer(ctxT(t), h.socket, "r1", false, "not now"); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindAnswer); e.Text != "deny" {
		t.Fatalf("answer = %q, want deny", e.Text)
	}
	if e := waitKind(t, events, chat.KindTurnEnd); !e.Error {
		t.Fatal("turn ended without an error after deny")
	}
	lines := stdinLines(t, h.stdin)
	if last := lines[len(lines)-1]; !strings.Contains(last, `"behavior":"deny"`) || !strings.Contains(last, `"message":"not now"`) {
		t.Fatalf("last stdin line = %s, want a deny with the message", last)
	}
}

func TestInterruptStopsTheTurn(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "ask"); err != nil {
		t.Fatal(err)
	}
	waitKind(t, events, chat.KindApproval)
	if err := Interrupt(ctxT(t), h.socket); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindTurnEnd); !e.Error {
		t.Fatal("interrupted turn ended without an error")
	}
	var interrupts int
	for _, l := range stdinLines(t, h.stdin) {
		if strings.Contains(l, `"subtype":"interrupt"`) {
			interrupts++
		}
	}
	if interrupts != 1 {
		t.Fatalf("%d interrupt requests on stdin, want 1", interrupts)
	}
}

func TestConcurrentAnswersSendOne(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "ask"); err != nil {
		t.Fatal(err)
	}
	waitKind(t, events, chat.KindApproval)
	const n = 10
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { errs <- Answer(ctxT(t), h.socket, "r1", true, "") })
	}
	wg.Wait()
	close(errs)
	accepted := 0
	for err := range errs {
		if err == nil {
			accepted++
		}
	}
	if accepted != 1 {
		t.Fatalf("%d answers accepted, want 1", accepted)
	}
	waitKind(t, events, chat.KindTurnEnd)
	logged, err := chat.ReadLog(h.log, 0)
	if err != nil {
		t.Fatal(err)
	}
	answers := 0
	for _, e := range logged {
		if e.Kind == chat.KindAnswer {
			answers++
		}
	}
	responses := 0
	for _, l := range stdinLines(t, h.stdin) {
		if strings.Contains(l, `"control_response"`) {
			responses++
		}
	}
	if answers != 1 || responses != 1 {
		t.Fatalf("%d answers logged and %d sent, want 1 each", answers, responses)
	}
}

func TestAnswerAfterTheTurnEndedIsRefused(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "ask"); err != nil {
		t.Fatal(err)
	}
	waitKind(t, events, chat.KindApproval)
	if err := Interrupt(ctxT(t), h.socket); err != nil {
		t.Fatal(err)
	}
	waitKind(t, events, chat.KindTurnEnd)
	err := Answer(ctxT(t), h.socket, "r1", true, "")
	if err == nil || !strings.Contains(err.Error(), `no approval "r1" is waiting`) {
		t.Fatalf("late answer = %v, want no approval waiting", err)
	}
	logged, err := chat.ReadLog(h.log, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range logged {
		if e.Kind == chat.KindAnswer {
			t.Fatalf("a late answer was logged: %+v", e)
		}
	}
	for _, l := range stdinLines(t, h.stdin) {
		if strings.Contains(l, `"control_response"`) {
			t.Fatalf("a late answer reached claude: %s", l)
		}
	}
}

func TestDraftIsLoggedNotSent(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Draft(ctxT(t), h.socket, "later"); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindDraft); e.Text != "later" {
		t.Fatalf("draft = %q, want later", e.Text)
	}
	for _, l := range stdinLines(t, h.stdin) {
		if strings.Contains(l, "later") {
			t.Fatalf("draft reached claude: %s", l)
		}
	}
}

// seqs returns the Seqs of the persisted events among events.
func seqs(events []chat.Event) []int64 {
	var out []int64
	for _, e := range events {
		if e.Persisted() {
			out = append(out, e.Seq)
		}
	}
	return out
}

func checkRun(t *testing.T, got []int64, from int64) {
	t.Helper()
	for i, s := range got {
		if s != from+int64(i)+1 {
			t.Fatalf("seqs %v: want each seq once, in order, from %d", got, from+1)
		}
	}
}

func TestSubscribeReplaysThenGoesLive(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	first := subscribe(t, h.socket, 0)
	waitKind(t, first, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "one"); err != nil {
		t.Fatal(err)
	}
	firstEnd := waitKind(t, first, chat.KindTurnEnd)
	if err := Send(ctxT(t), h.socket, "two"); err != nil {
		t.Fatal(err)
	}
	secondEnd := waitKind(t, first, chat.KindTurnEnd)

	later := subscribe(t, h.socket, firstEnd.Seq)
	var got []chat.Event
	for {
		e := next(t, later)
		got = append(got, e)
		if e.Seq == secondEnd.Seq {
			break
		}
	}
	want, err := chat.ReadLog(h.log, firstEnd.Seq)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("replay = %d events, want the second turn's %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Seq != want[i].Seq || got[i].Kind != want[i].Kind || got[i].Text != want[i].Text {
			t.Fatalf("replay[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	if err := Send(ctxT(t), h.socket, "three"); err != nil {
		t.Fatal(err)
	}
	var live []chat.Event
	for {
		e := next(t, later)
		live = append(live, e)
		if e.Kind == chat.KindTurnEnd {
			break
		}
	}
	if len(live) != 4 || live[1].Kind != chat.KindDelta || live[2].Text != "three" {
		t.Fatalf("live = %+v, want user, delta, message three, turn_end", live)
	}
	checkRun(t, seqs(append(got, live...)), firstEnd.Seq)
}

func TestSubscribeDuringTurnsHasNoGapsOrDoubles(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	const turns = 30
	sent := make(chan error, 1)
	go func() {
		for i := range turns {
			if err := Send(context.Background(), h.socket, fmt.Sprint("m", i)); err != nil {
				sent <- err
				return
			}
		}
		sent <- nil
	}()

	var subs []<-chan chat.Event
	for range 10 {
		subs = append(subs, subscribe(t, h.socket, 0))
		time.Sleep(5 * time.Millisecond)
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	eventually(t, "all turns in the log", func() bool {
		events, _ := chat.ReadLog(h.log, 0)
		ends := 0
		for _, e := range events {
			if e.Kind == chat.KindTurnEnd {
				ends++
			}
		}
		return ends == turns
	})
	all, err := chat.ReadLog(h.log, 0)
	if err != nil {
		t.Fatal(err)
	}
	last := all[len(all)-1].Seq
	for i, ch := range subs {
		var got []chat.Event
		for {
			e := next(t, ch)
			got = append(got, e)
			if e.Seq == last {
				break
			}
		}
		t.Logf("subscriber %d: %d events", i, len(got))
		checkRun(t, seqs(got), 0)
	}
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	fast := subscribe(t, h.socket, 0)
	waitKind(t, fast, chat.KindInit)

	slow, err := net.Dial("unix", h.socket)
	if err != nil {
		t.Fatal(err)
	}
	defer slow.Close()
	fmt.Fprintln(slow, `{"op":"subscribe","after":0}`)

	for range subscriberBuffer / 2 {
		if err := Send(ctxT(t), h.socket, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if err := Send(ctxT(t), h.socket, "done"); err != nil {
		t.Fatal(err)
	}
	for {
		if e := waitKind(t, fast, chat.KindMessage); e.Text == "done" {
			break
		}
	}

	slow.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.Copy(io.Discard, slow); err != nil {
		t.Fatalf("slow subscriber was not dropped: %v", err)
	}
}

func TestSendAfterClaudeExitsFails(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "quit"); err != nil {
		t.Fatal(err)
	}
	var refused string
	tries := 0
	eventually(t, "a send to be refused", func() bool {
		tries++
		text := fmt.Sprint("late ", tries)
		err := Send(ctxT(t), h.socket, text)
		if err != nil && strings.Contains(err.Error(), "claude has exited") {
			refused = text
			return true
		}
		return false
	})
	select {
	case <-h.done:
		h.done <- nil
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after claude exited")
	}
	logged, err := chat.ReadLog(h.log, 0)
	if err != nil {
		t.Fatal(err)
	}
	if last := logged[len(logged)-1]; last.Kind != chat.KindExit {
		t.Fatalf("last event = %s, want exit", last.Kind)
	}
	for _, e := range logged {
		if e.Text == refused {
			t.Fatalf("a refused send was logged: %+v", e)
		}
	}
}

func TestBigLineIsNotCut(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := Send(ctxT(t), h.socket, "big"); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindMessage); len(e.Text) != 1<<20 {
		t.Fatalf("message is %d bytes, want %d", len(e.Text), 1<<20)
	}
	logged, err := chat.ReadLog(h.log, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range logged {
		if e.Kind == chat.KindMessage && len(e.Text) != 1<<20 {
			t.Fatalf("logged message is %d bytes, want %d", len(e.Text), 1<<20)
		}
	}
}

func TestConcurrentSendsStayWholeLines(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Go(func() {
			errs <- Send(ctxT(t), h.socket, fmt.Sprintf("message %d %s", i, strings.Repeat("y", 64<<10)))
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	users := func() int {
		count := 0
		for _, l := range stdinLines(t, h.stdin) {
			if strings.Contains(l, `"type":"user"`) {
				count++
			}
		}
		return count
	}
	eventually(t, "50 user lines on stdin", func() bool { return users() == n })
	var sent []string
	for i, l := range stdinLines(t, h.stdin) {
		var line struct {
			Type    string `json:"type"`
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(l), &line); err != nil {
			t.Fatalf("stdin line %d is not valid JSON: %.80s", i, l)
		}
		if line.Type == "user" {
			sent = append(sent, line.Message.Content[0].Text)
		}
	}
	logged, err := chat.ReadLog(h.log, 0)
	if err != nil {
		t.Fatal(err)
	}
	var inLog []string
	for _, e := range logged {
		if e.Kind == chat.KindUser {
			inLog = append(inLog, e.Text)
		}
	}
	if len(inLog) != len(sent) {
		t.Fatalf("log has %d user events, stdin %d", len(inLog), len(sent))
	}
	for i := range sent {
		if inLog[i] != sent[i] {
			t.Fatalf("user message %d: log has %.12q, stdin %.12q; want the same order", i, inLog[i], sent[i])
		}
	}
}

func TestOversizedRequestIsRefused(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	conn, err := net.Dial("unix", h.socket)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	go fmt.Fprintf(conn, `{"op":"draft","text":"%s"}`+"\n", strings.Repeat("x", 2<<20))

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		t.Fatalf("no reply to an oversized request: %v", err)
	}
	var rep reply
	if err := json.Unmarshal(line, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.OK || !strings.Contains(rep.Error, "over 1 MB") {
		t.Fatalf("reply = %+v, want an error about the size", rep)
	}
	logged, err := chat.ReadLog(h.log, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range logged {
		if e.Kind == chat.KindDraft {
			t.Fatal("an oversized draft was logged")
		}
	}
}

func TestLongSocketPathFails(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	socket := filepath.Join(dir, strings.Repeat("s", 120-len(dir)-1))
	if len(socket) != 120 {
		t.Fatalf("socket path is %d bytes, want 120", len(socket))
	}
	err := Run(context.Background(), Config{
		Log:    filepath.Join(t.TempDir(), "chat.jsonl"),
		Socket: socket,
		Argv:   []string{os.Args[0], "-test.run=^TestHelperFakeClaude$"},
	})
	if err == nil || !strings.Contains(err.Error(), socket) {
		t.Fatalf("Run = %v, want an error naming %s", err, socket)
	}
}

func TestSecondHostOnASocketFails(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	err := Run(context.Background(), Config{
		Log:    filepath.Join(t.TempDir(), "chat.jsonl"),
		Socket: h.socket,
		Argv:   []string{os.Args[0], "-test.run=^TestHelperFakeClaude$"},
	})
	if err == nil {
		t.Fatal("a second host started on a live socket")
	}
	if err := Send(ctxT(t), h.socket, "still there"); err != nil {
		t.Fatalf("first host stopped answering: %v", err)
	}
}

func TestStaleSocketIsReplaced(t *testing.T) {
	t.Parallel()
	socket := filepath.Join(socketDir(t), "h.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	dir := t.TempDir()
	go func() {
		done <- Run(ctx, Config{
			Log:    filepath.Join(dir, "chat.jsonl"),
			Socket: socket,
			Argv:   []string{os.Args[0], "-test.run=^TestHelperFakeClaude$"},
			Env:    append(os.Environ(), "AOS_FAKE_CLAUDE=1", "AOS_FAKE_STDIN="+filepath.Join(dir, "stdin.jsonl")),
		})
	}()
	defer func() {
		cancel()
		<-done
	}()
	if err := Ready(ctxT(t), socket); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("socket mode = %o, want 600", perm)
	}
}

func TestCancelStopsClaude(t *testing.T) {
	t.Parallel()
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	h := startHost(t, "AOS_FAKE_IGNORE_TERM=1", "AOS_FAKE_GRANDCHILD="+pidFile)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)
	var grandchild int
	eventually(t, "the grandchild's pid", func() bool {
		raw, _ := os.ReadFile(pidFile)
		_, err := fmt.Sscan(string(raw), &grandchild)
		return err == nil
	})
	t.Cleanup(func() { syscall.Kill(grandchild, syscall.SIGKILL) })

	start := time.Now()
	h.cancel()
	select {
	case <-h.done:
		h.done <- nil
	case <-time.After(8 * time.Second):
		t.Fatal("Run did not return within 8s of cancel")
	}
	t.Logf("Run returned %s after cancel", time.Since(start).Round(time.Millisecond))
	logged, err := chat.ReadLog(h.log, 0)
	if err != nil {
		t.Fatal(err)
	}
	if last := logged[len(logged)-1]; last.Kind != chat.KindExit {
		t.Fatalf("last event = %s, want exit", last.Kind)
	}
	if _, err := os.Stat(h.socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket left behind: %v", err)
	}
	eventually(t, "claude's grandchild to be killed", func() bool {
		return errors.Is(syscall.Kill(grandchild, 0), syscall.ESRCH)
	})
}
