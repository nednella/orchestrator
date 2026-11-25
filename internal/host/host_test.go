package host

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/prompts"
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

type fakeLine struct {
	Type    string `json:"type"`
	UUID    string `json:"uuid"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	RequestID string `json:"request_id"`
	Request   struct {
		Subtype string `json:"subtype"`
		Mode    string `json:"mode"`
	} `json:"request"`
	Response struct {
		RequestID string `json:"request_id"`
		Response  struct {
			Behavior string `json:"behavior"`
		} `json:"response"`
	} `json:"response"`
}

func (l fakeLine) text() string {
	for _, c := range l.Message.Content {
		if c.Type == "text" {
			return c.Text
		}
	}
	return ""
}

// fakeClaude acts as claude -p in stream-json mode with --replay-user-messages. "slow" waits up to 2 s for an
// interrupt; a message read meanwhile joins it at its end, and one read before an interrupt runs as a turn of its own
// after it, except "lost", which the interrupt drops. "late" ends its turn 300 ms after it starts and, a second later, runs the messages read meanwhile as a turn
// of its own, as Claude does with a message written during a turn's last step.
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
	echo := func(l fakeLine) {
		if l.UUID != "" {
			print(map[string]any{"type": "user", "uuid": l.UUID, "isReplay": true, "message": map[string]any{"role": "user", "content": l.Message.Content}})
		}
	}
	turn := 0
	reply := func(text string) {
		turn++
		id := fmt.Sprintf("msg_%d", turn)
		print(map[string]any{"type": "stream_event", "api_message_id": id, "event": map[string]any{
			"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": text},
		}})
		print(map[string]any{"type": "assistant", "uuid": "u" + id, "message": map[string]any{
			"id": id, "role": "assistant", "content": []any{map[string]any{"type": "text", "text": text}},
		}})
	}
	// ownTurn runs messages Claude held past a turn's end as one turn of their own.
	ownTurn := func(held []fakeLine) {
		if len(held) == 0 {
			return
		}
		var texts []string
		for _, l := range held {
			echo(l)
			texts = append(texts, l.text())
		}
		reply(strings.Join(texts, " "))
		result(false)
	}
	// setMode answers a mode change as Claude does: a status line after the reply, and bypassPermissions refused, as
	// for a Claude not started with it allowed.
	setMode := func(l fakeLine) {
		if l.Request.Mode == "bypassPermissions" {
			print(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "error", "request_id": l.RequestID,
				"error": "Cannot set permission mode to bypassPermissions because the session was not launched with --dangerously-skip-permissions", "error_code": "bypass_not_launched"}})
			return
		}
		print(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": l.RequestID, "response": map[string]any{"mode": l.Request.Mode}}})
		print(map[string]any{"type": "system", "subtype": "status", "status": nil, "permissionMode": l.Request.Mode})
	}
	fmt.Fprintln(out, fakeInit)

	lines := make(chan fakeLine)
	go func() {
		defer close(lines)
		r := bufio.NewReader(in)
		for {
			raw, err := r.ReadBytes('\n')
			if len(raw) > 0 {
				logf.Write(raw)
			}
			if err != nil {
				return
			}
			var l fakeLine
			if json.Unmarshal(raw, &l) == nil {
				lines <- l
			}
		}
	}()
	// during reads lines until stop, holding user messages and answering control requests, and reports whether an
	// interrupt came.
	during := func(stop <-chan time.Time) (held []fakeLine, interrupted bool) {
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					return held, false
				}
				switch l.Type {
				case "user":
					held = append(held, l)
				case "control_request":
					still := []string{}
					held = slices.DeleteFunc(held, func(h fakeLine) bool { return h.text() == "lost" })
					for _, h := range held {
						still = append(still, h.UUID)
					}
					print(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": l.RequestID, "response": map[string]any{"still_queued": still}}})
					if l.Request.Subtype == "interrupt" {
						return held, true
					}
				}
			case <-stop:
				return held, false
			}
		}
	}

	for l := range lines {
		switch l.Type {
		case "control_request":
			if l.Request.Subtype == "set_permission_mode" {
				setMode(l)
				continue
			}
			if l.Request.Subtype != "interrupt" {
				print(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": l.RequestID}})
				continue
			}
			print(map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success", "request_id": l.RequestID, "response": map[string]any{"still_queued": []string{}}}})
			result(true)
		case "control_response":
			if l.Response.RequestID == "r1" {
				result(l.Response.Response.Behavior != "allow")
			}
		case "user":
			echo(l)
			text := l.text()
			switch text {
			case "ask":
				print(map[string]any{"type": "control_request", "request_id": "r1", "request": map[string]any{
					"subtype": "can_use_tool", "tool_name": "Bash", "input": map[string]any{"command": "ls"},
				}})
			case "slow":
				reply("slow")
				held, interrupted := during(time.After(2 * time.Second))
				if interrupted {
					result(true)
					ownTurn(held)
					continue
				}
				for _, h := range held {
					echo(h)
				}
				result(false)
			case "late":
				reply("late")
				held, _ := during(time.After(300 * time.Millisecond))
				result(false)
				more, _ := during(time.After(time.Second))
				ownTurn(append(held, more...))
			case "big":
				reply(strings.Repeat("x", 1<<20))
				result(false)
			case "quit":
				os.Stdout.Close()
				time.Sleep(time.Second)
				os.Exit(0)
			default:
				reply(text)
				result(false)
			}
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
	eventually(t, "50 user events in the log", func() bool { return len(logged(t, h.log, chat.KindUser)) == n })
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
	var inLog []string
	for _, e := range logged(t, h.log, chat.KindUser) {
		inLog = append(inLog, e.Text)
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
	// A bound socket that never listened refuses connections, even while a test running in parallel forks with its fd open.
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Bind(fd, &syscall.SockaddrUnix{Name: socket}); err != nil {
		t.Fatal(err)
	}
	syscall.Close(fd)

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
	eventually(t, "socket mode 600", func() bool {
		info, err := os.Stat(socket)
		return err == nil && info.Mode().Perm() == 0o600
	})
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

func TestSocketFolderWiderThan0700Fails(t *testing.T) {
	t.Parallel()
	for _, mode := range []os.FileMode{0o755, 0o750, 0o770} {
		dir := socketDir(t)
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		socket := filepath.Join(dir, "s")
		err := Run(context.Background(), Config{
			Log:    filepath.Join(t.TempDir(), "chat.jsonl"),
			Socket: socket,
			Argv:   []string{os.Args[0], "-test.run=^TestHelperFakeClaude$"},
		})
		if err == nil || !strings.Contains(err.Error(), dir) {
			t.Fatalf("mode %o: Run = %v, want an error naming %s", mode, err, dir)
		}
		if _, statErr := os.Lstat(socket); statErr == nil {
			t.Fatalf("mode %o: a socket was made in the folder", mode)
		}
	}
}

func TestEchoShowsEachTurnsOwnCostAndANewSessionOnce(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	h := &host{echo: &out}
	for _, e := range []chat.Event{
		{Kind: chat.KindInit, ID: "s1"},
		{Kind: chat.KindTurnEnd, Cost: 0.0061},
		{Kind: chat.KindInit, ID: "s1"},
		{Kind: chat.KindTurnEnd, Cost: 0.0091},
		{Kind: chat.KindInit, ID: "s2"},
		{Kind: chat.KindTurnEnd, Cost: 0.0104, Error: true},
		{Kind: chat.KindTurnEnd},
		{Kind: chat.KindTurnEnd, Cost: 0.0111},
		{Kind: chat.KindTurnEnd, Cost: 0.002},
	} {
		h.echoEvent(e)
	}
	want := "session s1\nturn ended, $0.0061\nturn ended, $0.0030\nsession s2\nturn ended, $0.0013, with an error\nturn ended\nturn ended, $0.0007\nturn ended, $0.0020\n"
	if got := out.String(); got != want {
		t.Errorf("echo =\n%s\nwant\n%s", got, want)
	}
}

// logged returns the logged events of the given kinds, all of them when none are given.
func logged(t *testing.T, path string, kinds ...chat.Kind) []chat.Event {
	t.Helper()
	all, err := chat.ReadLog(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []chat.Event
	for _, e := range all {
		if len(kinds) == 0 || slices.Contains(kinds, e.Kind) {
			out = append(out, e)
		}
	}
	return out
}

// userLines returns claude's stdin user lines.
func userLines(t *testing.T, path string) []fakeLine {
	t.Helper()
	var out []fakeLine
	for _, raw := range stdinLines(t, path) {
		var l fakeLine
		if json.Unmarshal([]byte(raw), &l) == nil && l.Type == "user" {
			out = append(out, l)
		}
	}
	return out
}

func sendMessage(t *testing.T, socket, text string, files ...string) string {
	t.Helper()
	id, err := SendMessage(ctxT(t), socket, Message{Text: text, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// index is the position of the first event with kind and id, -1 for none.
func index(events []chat.Event, kind chat.Kind, id string) int {
	return slices.IndexFunc(events, func(e chat.Event) bool { return e.Kind == kind && e.ID == id })
}

func TestSendReturnsTheMessageID(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	id := sendMessage(t, h.socket, "hi")
	if id == "" {
		t.Fatal("send replied with no id")
	}
	if e := waitKind(t, events, chat.KindUser); e.ID != id || e.Text != "hi" {
		t.Fatalf("user event = %+v, want id %s", e, id)
	}
	waitKind(t, events, chat.KindTurnEnd)
	lines := userLines(t, h.stdin)
	if len(lines) != 1 || lines[0].UUID != id {
		t.Fatalf("stdin user lines = %+v, want one with uuid %s", lines, id)
	}
	if got := len(logged(t, h.log, chat.KindUser, chat.KindQueued)); got != 1 {
		t.Fatalf("%d user and queued events, want 1: the echo is not logged again", got)
	}
}

func TestSendWhileBusyIsQueuedThenDelivered(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "slow")
	waitKind(t, events, chat.KindMessage)
	nextID := sendMessage(t, h.socket, "next")
	waitKind(t, events, chat.KindTurnEnd)

	log := logged(t, h.log)
	queued, end, user := index(log, chat.KindQueued, nextID), index(log, chat.KindTurnEnd, ""), index(log, chat.KindUser, nextID)
	if queued < 0 || queued > end || user < queued {
		t.Fatalf("log = %+v, want queued{next} before the first turn_end, then user{next}", log)
	}
	if log[user].Text != "next" {
		t.Fatalf("user event = %+v, want the queued text", log[user])
	}
	count := 0
	for _, l := range userLines(t, h.stdin) {
		if l.text() == "next" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("stdin holds next %d times, want once", count)
	}

	after := sendMessage(t, h.socket, "after")
	if e := waitKind(t, events, chat.KindUser); e.ID != after {
		t.Fatalf("event = %+v, want the message after the turn logged as the user's at once", e)
	}
}

func TestMessageAfterTheLastStepWaitsForItsOwnTurn(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "late")
	waitKind(t, events, chat.KindMessage)
	first := sendMessage(t, h.socket, "one")
	if e := waitKind(t, events, chat.KindQueued); e.ID != first {
		t.Fatalf("queued = %+v, want %s", e, first)
	}
	waitKind(t, events, chat.KindTurnEnd)
	second := sendMessage(t, h.socket, "two")
	if e := next(t, events); e.Kind != chat.KindQueued || e.ID != second {
		t.Fatalf("event = %+v, want %s queued: a written message keeps the host busy after the turn ends", e, second)
	}
	waitKind(t, events, chat.KindTurnEnd)
	log := logged(t, h.log)
	if index(log, chat.KindUser, first) < 0 || index(log, chat.KindUser, second) < 0 {
		t.Fatalf("log = %+v, want both messages logged as the user's at their echo", log)
	}
}

func TestSendNowInterruptsAndDeliversFirst(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "slow")
	waitKind(t, events, chat.KindMessage)
	first := sendMessage(t, h.socket, "first")
	second := sendMessage(t, h.socket, "second")
	if err := SendNow(ctxT(t), h.socket, "not-queued"); err == nil || !strings.Contains(err.Error(), "no queued message not-queued") {
		t.Fatalf("SendNow(unknown) = %v, want no queued message", err)
	}
	if err := SendNow(ctxT(t), h.socket, second); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindInterrupt); e.ID != second {
		t.Fatalf("interrupt = %+v, want it to name %s", e, second)
	}
	if e := waitKind(t, events, chat.KindTurnEnd); !e.Error {
		t.Fatal("interrupted turn ended without an error")
	}
	waitKind(t, events, chat.KindTurnEnd)

	log := logged(t, h.log)
	interrupt := index(log, chat.KindInterrupt, second)
	if index(log, chat.KindUser, first) < interrupt || index(log, chat.KindUser, second) < interrupt {
		t.Fatalf("log = %+v, want both messages logged as the user's after the interrupt", log)
	}
	var texts []string
	for _, l := range userLines(t, h.stdin) {
		texts = append(texts, l.text())
	}
	if fmt.Sprint(texts) != "[slow first second]" {
		t.Fatalf("stdin user lines = %v, want each message once: Claude runs the queued ones after the interrupt", texts)
	}
	interrupts := 0
	for _, l := range stdinLines(t, h.stdin) {
		if strings.Contains(l, `"subtype":"interrupt"`) {
			interrupts++
		}
	}
	if interrupts != 1 {
		t.Fatalf("%d interrupts on stdin, want 1", interrupts)
	}
	if err := SendNow(ctxT(t), h.socket, second); err == nil {
		t.Fatal("SendNow of a delivered message was accepted")
	}
}

func TestSendDuringApprovalDeniesWithTheText(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "ask")
	waitKind(t, events, chat.KindApproval)
	png := writePNG(t)
	if _, err := SendMessage(ctxT(t), h.socket, Message{Text: "look", Files: []string{png}}); err == nil || !strings.Contains(err.Error(), "Answer the approval before sending images") {
		t.Fatalf("send with an image = %v, want a refusal", err)
	}
	id := sendMessage(t, h.socket, "use ls instead")
	if e := waitKind(t, events, chat.KindAnswer); e.ID != "r1" || e.Text != "deny" {
		t.Fatalf("answer = %+v, want r1 deny", e)
	}
	if e := next(t, events); e.Kind != chat.KindUser || e.ID != id || e.Parent != "r1" || e.Text != "use ls instead" {
		t.Fatalf("event = %+v, want the user's message with parent r1", e)
	}
	waitKind(t, events, chat.KindTurnEnd)

	var deny string
	for _, l := range stdinLines(t, h.stdin) {
		if strings.Contains(l, `"control_response"`) {
			deny = l
		}
	}
	var answer struct {
		Response struct {
			Response struct {
				Behavior string `json:"behavior"`
				Message  string `json:"message"`
			} `json:"response"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(deny), &answer); err != nil {
		t.Fatal(err)
	}
	if r := answer.Response.Response; r.Behavior != "deny" || r.Message != prompts.Denied("use ls instead") {
		t.Fatalf("control_response = %s, want a deny with the text in its framing", deny)
	}
	if lines := userLines(t, h.stdin); len(lines) != 1 {
		t.Fatalf("%d user lines on stdin, want only the first", len(lines))
	}
}

func TestAnswerWithInputAllowsWithTheGivenInput(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "ask")
	waitKind(t, events, chat.KindApproval)
	if err := AnswerWith(ctxT(t), h.socket, "r1", json.RawMessage(`["not an object"]`)); err == nil {
		t.Fatal("an input that is not an object was accepted")
	}
	given := `{"command":"ls -la","answers":{"Which?":"Red"}}`
	if err := AnswerWith(ctxT(t), h.socket, "r1", json.RawMessage(given)); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindAnswer); string(e.Input) != given {
		t.Fatalf("logged answer input = %s, want %s", e.Input, given)
	}
	if e := waitKind(t, events, chat.KindTurnEnd); e.Error {
		t.Fatal("turn ended with an error after allow")
	}
	for _, l := range stdinLines(t, h.stdin) {
		if !strings.Contains(l, `"control_response"`) {
			continue
		}
		var answer struct {
			Response struct {
				Response struct {
					Behavior     string          `json:"behavior"`
					UpdatedInput json.RawMessage `json:"updatedInput"`
				} `json:"response"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(l), &answer); err != nil {
			t.Fatal(err)
		}
		if r := answer.Response.Response; r.Behavior != "allow" || string(r.UpdatedInput) != given {
			t.Fatalf("answer = %s, want allow with %s", l, given)
		}
		return
	}
	t.Fatal("no control_response on stdin")
}

func TestDenyLogsNoInput(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "ask")
	waitKind(t, events, chat.KindApproval)
	if _, err := call(ctxT(t), h.socket, request{Op: opAnswer, ID: "r1", Input: json.RawMessage(`{"answers":{"Which?":"Red"}}`)}); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindAnswer); e.Text != "deny" || e.Input != nil {
		t.Fatalf("logged answer = %+v, want a deny with no input", e)
	}
}

func TestInterruptIsLogged(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "slow")
	waitKind(t, events, chat.KindMessage)
	if err := Interrupt(ctxT(t), h.socket); err != nil {
		t.Fatal(err)
	}
	if e := next(t, events); e.Kind != chat.KindInterrupt || e.ID != "" {
		t.Fatalf("event = %+v, want an interrupt", e)
	}
	waitKind(t, events, chat.KindTurnEnd)
	eventually(t, "the interrupt in the pane", func() bool { return strings.Contains(h.echo.String(), "interrupted") })
}

// writePNG writes a 1x1 png and returns its path.
func writePNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dot.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSendWithImagesBuildsImageBlocks(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	path := writePNG(t)
	id := sendMessage(t, h.socket, "what is this", path)
	if e := waitKind(t, events, chat.KindUser); e.ID != id || len(e.Files) != 1 || e.Files[0] != path {
		t.Fatalf("user event = %+v, want files [%s]", e, path)
	}
	waitKind(t, events, chat.KindTurnEnd)
	var line struct {
		Message struct {
			Content []struct {
				Type   string `json:"type"`
				Text   string `json:"text"`
				Source struct {
					Type      string `json:"type"`
					MediaType string `json:"media_type"`
					Data      string `json:"data"`
				} `json:"source"`
			} `json:"content"`
		} `json:"message"`
	}
	for _, l := range stdinLines(t, h.stdin) {
		if strings.Contains(l, `"type":"user"`) {
			if err := json.Unmarshal([]byte(l), &line); err != nil {
				t.Fatal(err)
			}
		}
	}
	content := line.Message.Content
	if len(content) != 2 || content[0].Type != "image" || content[0].Source.MediaType != "image/png" || content[1].Text != "what is this" {
		t.Fatalf("content = %+v, want a png image block, then the text", content)
	}
	if content[0].Source.Type != "base64" || content[0].Source.Data != base64.StdEncoding.EncodeToString(readFile(t, path)) {
		t.Fatalf("image source = %+v, want the file in base64", content[0].Source)
	}
}

func TestImageOverTheLimitIsRefused(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	dir := t.TempDir()
	dot := readFile(t, writePNG(t))
	big := filepath.Join(dir, "big.png")
	if err := os.WriteFile(big, append(dot, make([]byte, MaxImage+1-len(dot))...), 0o600); err != nil {
		t.Fatal(err)
	}
	most := filepath.Join(dir, "most.png")
	if err := os.WriteFile(most, append(dot, make([]byte, MaxImage-len(dot))...), 0o600); err != nil {
		t.Fatal(err)
	}
	text := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(text, []byte("plain words"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := logged(t, h.log)
	for _, path := range []string{big, text, filepath.Join(dir, "missing.png")} {
		_, err := SendMessage(ctxT(t), h.socket, Message{Text: "see", Files: []string{path}})
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("send of %s = %v, want an error naming it", path, err)
		}
	}
	if _, err := SendMessage(ctxT(t), h.socket, Message{Text: "see", Files: []string{big}}); err == nil || !strings.Contains(err.Error(), "is over 3.75 MB") {
		t.Fatalf("send of a file over the limit = %v, want it named over 3.75 MB", err)
	}
	if after := logged(t, h.log); len(after) != len(before) {
		t.Fatalf("log went from %d to %d events, want it unchanged", len(before), len(after))
	}
	if lines := userLines(t, h.stdin); len(lines) != 0 {
		t.Fatalf("%d user lines reached claude, want none", len(lines))
	}
	if _, err := SendMessage(ctxT(t), h.socket, Message{Text: "see", Files: []string{most}}); err != nil {
		t.Fatalf("send of a file at the limit = %v, want it sent", err)
	}
	if encoded := base64.StdEncoding.EncodedLen(MaxImage); encoded > 5<<20 {
		t.Fatalf("an image at the limit is %d bytes in base64, over the API's 5 MB", encoded)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestImagesToAnOldHostAreAnError(t *testing.T) {
	t.Parallel()
	socket := filepath.Join(socketDir(t), "old.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			bufio.NewReader(conn).ReadBytes('\n')
			fmt.Fprintln(conn, `{"ok":true,"error":""}`)
			conn.Close()
		}
	}()
	if _, err := SendMessage(ctxT(t), socket, Message{Text: "see", Files: []string{"dot.png"}}); err == nil || err.Error() != "this session's host is too old for images" {
		t.Fatalf("send of an image = %v, want the host named too old", err)
	}
	if err := Send(ctxT(t), socket, "hi"); err != nil {
		t.Fatalf("send of text = %v, want it taken", err)
	}
}

func TestInterruptDropsWhatClaudeNoLongerHolds(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "slow")
	waitKind(t, events, chat.KindMessage)
	lost := sendMessage(t, h.socket, "lost")
	if err := Interrupt(ctxT(t), h.socket); err != nil {
		t.Fatal(err)
	}
	if e := waitKind(t, events, chat.KindUnsent); e.ID != lost || e.Text != "lost" {
		t.Fatalf("event = %+v, want the dropped message logged unsent", e)
	}
	waitKind(t, events, chat.KindTurnEnd)
	if err := SendNow(ctxT(t), h.socket, lost); err == nil {
		t.Fatal("the message the interrupt dropped is still queued")
	}
	after := sendMessage(t, h.socket, "after")
	if e := waitKind(t, events, chat.KindUser); e.ID != after {
		t.Fatalf("event = %+v, want the next message logged as the user's at once", e)
	}
}

// modes are the permission modes the host's log holds, in order.
func modes(t *testing.T, log string) []string {
	t.Helper()
	events, err := chat.ReadLog(log, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range events {
		if e.Kind == chat.KindMode {
			out = append(out, e.Text)
		}
	}
	return out
}

func modeRequests(t *testing.T, stdin string) []string {
	t.Helper()
	var out []string
	for _, l := range stdinLines(t, stdin) {
		if strings.Contains(l, `"subtype":"set_permission_mode"`) {
			out = append(out, l)
		}
	}
	return out
}

func TestModeSetsPermissionMode(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	if err := SetMode(ctxT(t), h.socket, "plan"); err != nil {
		t.Fatal(err)
	}
	if got := modeRequests(t, h.stdin); len(got) != 1 || !strings.Contains(got[0], `"mode":"plan"`) {
		t.Fatalf("stdin = %q, want one request for plan", got)
	}
	if e := waitKind(t, events, chat.KindMode); e.Text != "plan" {
		t.Fatalf("mode = %+v, want plan", e)
	}
	// After a turn, Claude's own report of the change has been read too.
	sendMessage(t, h.socket, "hi")
	waitKind(t, events, chat.KindTurnEnd)
	if got := modes(t, h.log); !slices.Equal(got, []string{"plan"}) {
		t.Errorf("logged modes = %q, want plan once", got)
	}
	eventually(t, "the mode in the pane", func() bool { return strings.Contains(h.echo.String(), "permission mode: plan") })
}

func TestModeIsRefusedDuringATurn(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	sendMessage(t, h.socket, "slow")
	waitKind(t, events, chat.KindMessage)
	err := SetMode(ctxT(t), h.socket, "default")
	if refused := (*Refused)(nil); !errors.As(err, &refused) || refused.Reason != "Change access when the turn ends" {
		t.Fatalf("err = %v, want the host's refusal", err)
	}
	waitKind(t, events, chat.KindTurnEnd)
	if got := modeRequests(t, h.stdin); len(got) != 0 {
		t.Errorf("stdin = %q, want no mode request", got)
	}
	if got := modes(t, h.log); len(got) != 0 {
		t.Errorf("logged modes = %q, want none", got)
	}
}

func TestModeReportsClaudesError(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)

	err := SetMode(ctxT(t), h.socket, "bypassPermissions")
	if refused := (*Refused)(nil); !errors.As(err, &refused) || !strings.Contains(refused.Reason, "was not launched with --dangerously-skip-permissions") {
		t.Fatalf("err = %v, want Claude's reason", err)
	}
	sendMessage(t, h.socket, "hi")
	waitKind(t, events, chat.KindTurnEnd)
	if got := modes(t, h.log); len(got) != 0 {
		t.Errorf("logged modes = %q, want none", got)
	}
}

func TestBusyWhileATurnRunsOrAMessageWaits(t *testing.T) {
	t.Parallel()
	h := startHost(t)
	events := subscribe(t, h.socket, 0)
	waitKind(t, events, chat.KindInit)
	busy := func() bool {
		t.Helper()
		b, err := Busy(ctxT(t), h.socket)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	if busy() {
		t.Fatal("busy before any message")
	}
	sendMessage(t, h.socket, "slow")
	if !busy() {
		t.Error("not busy once a message is written, before Claude answers")
	}
	waitKind(t, events, chat.KindTurnEnd)
	if busy() {
		t.Error("busy after the turn ended")
	}
	sendMessage(t, h.socket, "ask")
	waitKind(t, events, chat.KindApproval)
	if !busy() {
		t.Error("not busy while an approval waits")
	}
}
