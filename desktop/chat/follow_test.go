package chat_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/chat"
	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/session"
)

// oneThread is a single live thread under a host at socket, with an empty chat log.
type oneThread struct{ chat sessions.Chat }

func (o oneThread) ChatOf(string) (sessions.Chat, error)          { return o.chat, nil }
func (o oneThread) Chats() []sessions.Chat                        { return []sessions.Chat{o.chat} }
func (o oneThread) Observe(string, ...session.Event)              {}
func (o oneThread) TranscriptPath(string, string) string          { return "" }
func (o oneThread) BackgroundChanged(string, sessions.Background) {}
func (o oneThread) AccessChanged(string, string, bool)            {}
func (o oneThread) FlagRefused(string)                            {}
func (o oneThread) NextTurn(string)                               {}
func (o oneThread) Wake(string) (sessions.Chat, error)            { return o.chat, nil }
func (o oneThread) FilesRewind(string) (string, bool, string)     { return "", true, "" }

// noProject knows no project.
func noProject(string) string { return "" }

// fakeHost answers subscribes on a unix socket: each one gets the next list of seqs, and the stream is closed after
// every list but the last.
type fakeHost struct {
	mu     sync.Mutex
	afters []int64
}

func (f *fakeHost) serve(t *testing.T, ln net.Listener, streams [][]int64) {
	for i := 0; ; i++ {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		var req struct {
			Op    string `json:"op"`
			After int64  `json:"after"`
		}
		line, err := bufio.NewReader(conn).ReadBytes('\n')
		if len(line) == 0 && errors.Is(err, io.EOF) {
			// The follower waiting for the host to listen.
			conn.Close()
			i--
			continue
		}
		if err != nil || json.Unmarshal(line, &req) != nil || req.Op != "subscribe" {
			t.Errorf("request %q: %v", line, err)
			conn.Close()
			continue
		}
		f.mu.Lock()
		f.afters = append(f.afters, req.After)
		f.mu.Unlock()
		conn.Write([]byte(`{"ok":true}` + "\n"))
		enc := json.NewEncoder(conn)
		for _, seq := range streams[min(i, len(streams)-1)] {
			enc.Encode(chatlog.Event{Seq: seq, Kind: chatlog.KindMessage, Text: "x"})
		}
		if i < len(streams)-1 {
			conn.Close()
		} else {
			t.Cleanup(func() { conn.Close() })
		}
	}
}

func (f *fakeHost) subscribedAfter() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.afters)
}

func TestFollowerResubscribesAfterTheStreamDrops(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "host.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	fake := &fakeHost{}
	go fake.serve(t, ln, [][]int64{{1, 2, 3}, {4, 5}})

	rec := &apptest.Recorder{}
	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: filepath.Join(dir, "chat.jsonl"), Socket: socket, Live: true}}
	chats := chat.New(thread, t.TempDir(), rec.Emit)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chats.Start(ctx)
	svc := chat.NewService(chats, nil, noProject, context.Background)
	if err := svc.ChatWatch(thread.chat.ID); err != nil {
		t.Fatal(err)
	}

	apptest.Eventually(t, "seqs 1 to 5", func() bool { return len(emittedSeqs(rec, thread.chat.ID)) >= 5 })
	time.Sleep(100 * time.Millisecond)
	if got := emittedSeqs(rec, thread.chat.ID); !slices.Equal(got, []int64{1, 2, 3, 4, 5}) {
		t.Errorf("emitted seqs = %v, want 1 to 5 each once", got)
	}
	if got := fake.subscribedAfter(); !slices.Equal(got, []int64{0, 3}) {
		t.Errorf("subscribed after %v, want [0 3]", got)
	}
}

// emittedSeqs are the seqs of the logged events in the thread's chat:<id> events, in the order they went out.
func emittedSeqs(rec *apptest.Recorder, id string) []int64 {
	var seqs []int64
	for _, p := range rec.All("chat:" + id) {
		for _, e := range p.(chat.Update).Events {
			if e.Persisted() {
				seqs = append(seqs, e.Seq)
			}
		}
	}
	return seqs
}

// serveUpTo answers each subscribe as a host whose log ends at seq last: every seq after the one asked for, then an
// open stream.
func serveUpTo(t *testing.T, ln net.Listener, last int64) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		var req struct {
			After int64 `json:"after"`
		}
		line, _ := bufio.NewReader(conn).ReadBytes('\n')
		if len(line) == 0 {
			conn.Close()
			continue
		}
		if err := json.Unmarshal(line, &req); err != nil {
			t.Errorf("request %q: %v", line, err)
		}
		conn.Write([]byte(`{"ok":true}` + "\n"))
		enc := json.NewEncoder(conn)
		for seq := req.After + 1; seq <= last; seq++ {
			enc.Encode(chatlog.Event{Seq: seq, Kind: chatlog.KindMessage, Text: "x"})
		}
		t.Cleanup(func() { conn.Close() })
	}
}

func TestAWatchBeforeTheStartMissesNothing(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket, logPath := filepath.Join(dir, "host.sock"), filepath.Join(dir, "chat.jsonl")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go serveUpTo(t, ln, 5)
	chatLog, err := chatlog.OpenLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { chatLog.Close() })
	appendMessage := func() {
		if _, err := chatLog.Append(chatlog.Event{Kind: chatlog.KindMessage, Text: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	appendMessage()
	appendMessage()

	rec := &apptest.Recorder{}
	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: socket, Live: true}}
	chats := chat.New(thread, t.TempDir(), rec.Emit)
	svc := chat.NewService(chats, nil, noProject, context.Background)
	read := make(chan []chatlog.Event)
	go func() {
		if err := svc.ChatWatch(thread.chat.ID); err != nil {
			t.Error(err)
		}
		history, err := svc.ChatHistory(thread.chat.ID, 0)
		if err != nil {
			t.Error(err)
		}
		appendMessage() // logged after the history read: only the stream can bring it
		read <- history
	}()
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chats.Start(ctx)
	history := <-read

	apptest.Eventually(t, "seq 5 in a chat event", func() bool { got := emittedSeqs(rec, thread.chat.ID); return len(got) > 0 && got[len(got)-1] == 5 })
	var all []int64
	for _, e := range history {
		all = append(all, e.Seq)
	}
	all = append(all, emittedSeqs(rec, thread.chat.ID)...)
	slices.Sort(all)
	if all = slices.Compact(all); !slices.Equal(all, []int64{1, 2, 3, 4, 5}) {
		t.Errorf("history and chat events hold seqs %v, want 1 to 5 with no gap", all)
	}
}

func TestAHostThatCannotBeReachedIsNamedPlainly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: filepath.Join(dir, "chat.jsonl"), Socket: filepath.Join(dir, "gone.sock"), Live: true}}
	svc := chat.NewService(chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit), nil, noProject, context.Background)
	if err, want := svc.ChatInterrupt(thread.chat.ID), "Thread main/abcdefgh did not take the interrupt"; err == nil || err.Error() != want {
		t.Errorf("ChatInterrupt = %v, want %q", err, want)
	}
}

func TestAKilledRunsApprovalDoesNotWaitOnceTheNextRunStarts(t *testing.T) {
	t.Parallel()
	logPath := filepath.Join(t.TempDir(), "chat.jsonl")
	chatLog, err := chatlog.OpenLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []chatlog.Event{
		{Kind: chatlog.KindStart},
		{Kind: chatlog.KindApproval, ID: "req-1", Tool: "Bash", Input: json.RawMessage(`{"command":"make test"}`)},
		{Kind: chatlog.KindStart},
	} {
		if _, err := chatLog.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	chatLog.Close()
	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: filepath.Join(t.TempDir(), "gone.sock"), Live: true}}
	svc := chat.NewService(chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit), nil, noProject, context.Background)
	if got := svc.ChatPending(); len(got) != 0 {
		t.Errorf("ChatPending = %+v, want nothing: the run that asked was killed", got)
	}
}

func TestPendingIsRightBeforeTheFollowerReadsTheLog(t *testing.T) {
	t.Parallel()
	logPath := filepath.Join(t.TempDir(), "chat.jsonl")
	chatLog, err := chatlog.OpenLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []chatlog.Event{
		{Kind: chatlog.KindApproval, ID: "req-1", Tool: "Edit", Input: json.RawMessage(`{"file_path":"a.go"}`)},
		{Kind: chatlog.KindAnswer, ID: "req-1", Text: "allow"},
		{Kind: chatlog.KindApproval, ID: "req-2", Tool: "Bash", Input: json.RawMessage(`{"command":"make test"}`)},
	} {
		if _, err := chatLog.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	chatLog.Close()
	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: filepath.Join(t.TempDir(), "gone.sock"), Live: true}}
	svc := chat.NewService(chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit), nil, noProject, context.Background)
	want := []chat.Pending{{ID: thread.chat.ID, Request: "req-2", Tool: "Bash", Summary: "make test", Kind: "tool", Input: json.RawMessage(`{"command":"make test"}`)}}
	if got := svc.ChatPending(); !slices.EqualFunc(got, want, samePending) {
		t.Errorf("ChatPending before any follower = %+v, want %+v", got, want)
	}
}

func TestASubagentClaudesListLeavesOutDoesNotWaitBeforeTheFollowerReadsTheLog(t *testing.T) {
	t.Parallel()
	logPath := filepath.Join(t.TempDir(), "chat.jsonl")
	chatLog, err := chatlog.OpenLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []chatlog.Event{
		{Kind: chatlog.KindTaskStart, ID: "a3", Parent: "toolu_lost", Tool: "local_agent"},
		{Kind: chatlog.KindApproval, ID: "lost-1", Tool: "Bash", Parent: "toolu_ls", Agent: "toolu_lost", Input: json.RawMessage(`{"command":"ls"}`)},
		{Kind: chatlog.KindTasks, Input: json.RawMessage(`[]`)},
	} {
		if _, err := chatLog.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	chatLog.Close()
	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: filepath.Join(t.TempDir(), "gone.sock"), Live: true}}
	svc := chat.NewService(chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit), nil, noProject, context.Background)
	if got := svc.ChatPending(); len(got) != 0 {
		t.Errorf("ChatPending = %+v, want nothing: Claude's list left the subagent that asked out", got)
	}
}

func TestAnOldHostSendsTheTextAndSaysSo(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "host.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	// A host from before message ids answers a send with no id.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			if _, err := bufio.NewReader(conn).ReadBytes('\n'); err == nil {
				conn.Write([]byte(`{"ok":true}` + "\n"))
			}
			conn.Close()
		}
	}()

	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: filepath.Join(dir, "chat.jsonl"), Socket: socket, Media: filepath.Join(dir, "media"), Live: true}}
	svc := chat.NewService(chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit), nil, noProject, context.Background)
	sent, err := svc.ChatSend(thread.chat.ID, "see", []chat.Image{{Base64: dot, Mime: "image/png"}})
	if want := "Your text was sent, but this thread's host is too old for images. Restart the thread to send images."; err != nil || sent.Notice != want {
		t.Errorf("ChatSend = %+v, %v; want the notice %q", sent, err, want)
	}
}

// serveEvents answers each subscribe as a host whose log holds events: those after the seq asked for, then an open
// stream.
func serveEvents(t *testing.T, ln net.Listener, events []chatlog.Event) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		var req struct {
			After int64 `json:"after"`
		}
		line, _ := bufio.NewReader(conn).ReadBytes('\n')
		if len(line) == 0 {
			conn.Close()
			continue
		}
		if err := json.Unmarshal(line, &req); err != nil {
			t.Errorf("request %q: %v", line, err)
		}
		conn.Write([]byte(`{"ok":true}` + "\n"))
		enc := json.NewEncoder(conn)
		for _, e := range events {
			if e.Seq > req.After {
				enc.Encode(e)
			}
		}
		t.Cleanup(func() { conn.Close() })
	}
}

// keptCommands follows a thread of the project main whose log holds a commands list reported at 1000 and whose host
// then streams more, and says what ProjectCommands gives for main.
func keptCommands(t *testing.T, more ...chatlog.Event) func() []chat.Command {
	dir, err := os.MkdirTemp("", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket, logPath := filepath.Join(dir, "host.sock"), filepath.Join(dir, "chat.jsonl")
	chatLog, err := chatlog.OpenLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logged, err := chatLog.Append(chatlog.Event{At: 1000, Kind: chatlog.KindCommands, Input: json.RawMessage(`[{"name":"review","description":"Review","argumentHint":"<pr>","builtin":true,"aliases":["pr"]}]`)})
	chatLog.Close()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	stream := []chatlog.Event{logged}
	for i, e := range more {
		e.Seq = logged.Seq + int64(i) + 1
		stream = append(stream, e)
	}
	go serveEvents(t, ln, stream)

	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: socket, Live: true}}
	chats := chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit)
	svc := chat.NewService(chats, nil, func(key string) string {
		if key == "main" {
			return dir
		}
		return ""
	}, context.Background)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chats.Start(ctx)
	return func() []chat.Command {
		got, err := svc.ProjectCommands("main")
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
}

func TestALoggedCommandListIsKeptForItsProject(t *testing.T) {
	t.Parallel()
	commands := keptCommands(t)
	want := []chat.Command{{Name: "review", Description: "Review", ArgumentHint: "<pr>", Builtin: true, Aliases: []string{"pr"}}}
	apptest.Eventually(t, "the logged list kept", func() bool { return reflect.DeepEqual(commands(), want) })
}

func TestALiveCommandListReplacesTheKeptOne(t *testing.T) {
	t.Parallel()
	commands := keptCommands(t, chatlog.Event{At: 2000, Kind: chatlog.KindCommands, Input: json.RawMessage(`[{"name":"compact","description":"Compact"}]`)})
	want := []chat.Command{{Name: "compact", Description: "Compact"}}
	apptest.Eventually(t, "the live list kept", func() bool { return reflect.DeepEqual(commands(), want) })
}

// pushed answers the follower's subscribe as a host that streams each event sent on events.
func pushed(t *testing.T, ln net.Listener, events <-chan chatlog.Event) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		line, _ := bufio.NewReader(conn).ReadBytes('\n')
		if len(line) == 0 {
			conn.Close()
			continue
		}
		t.Cleanup(func() { conn.Close() })
		conn.Write([]byte(`{"ok":true}` + "\n"))
		enc := json.NewEncoder(conn)
		for e := range events {
			if enc.Encode(e) != nil {
				return
			}
		}
	}
}

// switchedThread is oneThread that runs until live is false.
type switchedThread struct {
	oneThread
	live *atomic.Bool
}

func (s switchedThread) ChatOf(string) (sessions.Chat, error) {
	c := s.chat
	c.Live = s.live.Load()
	return c, nil
}

func (s switchedThread) Chats() []sessions.Chat {
	if !s.live.Load() {
		return nil
	}
	return []sessions.Chat{s.chat}
}

// awaitable starts the chats of a live thread whose log holds logged and whose host streams what is sent on the
// channel it returns. Cancelling the context it returns stops the chats, as the app's quit does.
func awaitable(t *testing.T, logged ...chatlog.Event) (*chat.Chats, switchedThread, chan<- chatlog.Event, context.CancelFunc) {
	t.Helper()
	dir, err := os.MkdirTemp("", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket, logPath := filepath.Join(dir, "host.sock"), filepath.Join(dir, "chat.jsonl")
	chatLog, err := chatlog.OpenLog(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range logged {
		if _, err := chatLog.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	chatLog.Close()
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	events := make(chan chatlog.Event)
	t.Cleanup(func() { close(events) })
	go pushed(t, ln, events)

	live := &atomic.Bool{}
	live.Store(true)
	thread := switchedThread{oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: socket, Live: true}}, live}
	chats := chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chats.Start(ctx)
	return chats, thread, events, cancel
}

type awaited struct {
	e   chatlog.Event
	err error
}

// awaitIn runs AwaitTurn in the background and gives its answer on the channel it returns.
func awaitIn(ctx context.Context, chats *chat.Chats, id string, after int64) <-chan awaited {
	got := make(chan awaited, 1)
	go func() {
		e, err := chats.AwaitTurn(ctx, id, after)
		got <- awaited{e, err}
	}()
	return got
}

func answered(t *testing.T, got <-chan awaited) awaited {
	t.Helper()
	select {
	case a := <-got:
		return a
	case <-time.After(5 * time.Second):
		t.Fatal("AwaitTurn did not return within 5 s")
		return awaited{}
	}
}

func TestAwaitTurnGetsTheNextTurnEnd(t *testing.T) {
	t.Parallel()
	chats, thread, events, _ := awaitable(t,
		chatlog.Event{Kind: chatlog.KindUser, Text: "first"},
		chatlog.Event{Kind: chatlog.KindTurnEnd},
		chatlog.Event{Kind: chatlog.KindUser, Text: "second"},
	)
	id := thread.chat.ID
	short, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := chats.AwaitTurn(short, id, 3); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("AwaitTurn after 3 with no turn_end since = %v, want the deadline: the turn_end at 2 is before it", err)
	}

	gone, cancelGone := context.WithCancel(context.Background())
	dropped := awaitIn(gone, chats, id, 3)
	cancelGone()
	if a := answered(t, dropped); !errors.Is(a.err, context.Canceled) {
		t.Fatalf("a cancelled AwaitTurn = %+v, want context.Canceled", a)
	}

	next := awaitIn(context.Background(), chats, id, 3)
	events <- chatlog.Event{Seq: 4, Kind: chatlog.KindMessage, Text: "done"}
	events <- chatlog.Event{Seq: 5, Kind: chatlog.KindTurnEnd, Cost: 0.01}
	if a := answered(t, next); a.err != nil || a.e.Seq != 5 || a.e.Kind != chatlog.KindTurnEnd {
		t.Fatalf("AwaitTurn after 3 = %+v, want the turn_end at 5", a)
	}
	if a := answered(t, awaitIn(context.Background(), chats, id, 3)); a.err != nil || a.e.Seq != 5 {
		t.Errorf("AwaitTurn after 3 once the follower took 5 = %+v, want 5 at once", a)
	}
	short, cancel = context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := chats.AwaitTurn(short, id, 5); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("AwaitTurn after 5 = %v, want the deadline", err)
	}
	// The cancelled await held nothing up: the follower takes the next turn too.
	later := awaitIn(context.Background(), chats, id, 5)
	events <- chatlog.Event{Seq: 6, Kind: chatlog.KindUser, Text: "third"}
	events <- chatlog.Event{Seq: 7, Kind: chatlog.KindTurnEnd}
	if a := answered(t, later); a.err != nil || a.e.Seq != 7 {
		t.Errorf("AwaitTurn after 5 = %+v, want the turn_end at 7", a)
	}
}

func TestAwaitTurnEndsWhenTheHostExits(t *testing.T) {
	t.Parallel()
	user := chatlog.Event{Kind: chatlog.KindUser, Text: "work"}
	t.Run("claude exits", func(t *testing.T) {
		chats, thread, events, _ := awaitable(t, user)
		got := awaitIn(context.Background(), chats, thread.chat.ID, 1)
		events <- chatlog.Event{Seq: 2, Kind: chatlog.KindExit, Error: true, Text: "signal: killed"}
		if a := answered(t, got); !errors.Is(a.err, chat.ErrThreadGone) {
			t.Fatalf("AwaitTurn = %+v, want ErrThreadGone", a)
		}
		if a := answered(t, awaitIn(context.Background(), chats, thread.chat.ID, 1)); !errors.Is(a.err, chat.ErrThreadGone) {
			t.Errorf("AwaitTurn from before the exit = %+v, want ErrThreadGone at once", a)
		}
	})
	t.Run("the thread no longer runs", func(t *testing.T) {
		chats, thread, _, _ := awaitable(t, user)
		got := awaitIn(context.Background(), chats, thread.chat.ID, 1)
		thread.live.Store(false)
		if a := answered(t, got); !errors.Is(a.err, chat.ErrThreadGone) {
			t.Fatalf("AwaitTurn = %+v, want ErrThreadGone", a)
		}
		if a := answered(t, awaitIn(context.Background(), chats, thread.chat.ID, 1)); !errors.Is(a.err, chat.ErrThreadGone) {
			t.Errorf("AwaitTurn on a thread that does not run = %+v, want ErrThreadGone at once", a)
		}
	})
	t.Run("the app stops", func(t *testing.T) {
		chats, thread, _, stop := awaitable(t, user)
		got := awaitIn(context.Background(), chats, thread.chat.ID, 1)
		time.Sleep(100 * time.Millisecond)
		stop()
		if a := answered(t, got); !errors.Is(a.err, chat.ErrThreadGone) {
			t.Fatalf("AwaitTurn = %+v, want ErrThreadGone", a)
		}
	})
}

func TestALoggedSettleEndsTheTurnOnlyForItsOwnRun(t *testing.T) {
	t.Parallel()
	user := chatlog.Event{Kind: chatlog.KindUser, Text: "work"}
	settle := chatlog.Event{Kind: chatlog.KindExit, Stop: chatlog.StopSettle}
	for _, tt := range []struct {
		name   string
		logged []chatlog.Event
		want   int // how many exits OnTurnEnd hears
	}{
		{"the run's own settle", []chatlog.Event{{Kind: chatlog.KindStart}, user, settle}, 1},
		{"an earlier run's settle", []chatlog.Event{{Kind: chatlog.KindStart}, user, settle, {Kind: chatlog.KindStart}}, 0},
		{"an exit that is no settle", []chatlog.Event{{Kind: chatlog.KindStart}, user, {Kind: chatlog.KindExit, Error: true}}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "aos")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })
			socket, logPath := filepath.Join(dir, "host.sock"), filepath.Join(dir, "chat.jsonl")
			chatLog, err := chatlog.OpenLog(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range tt.logged {
				if _, err := chatLog.Append(e); err != nil {
					t.Fatal(err)
				}
			}
			chatLog.Close()
			ln, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { ln.Close() })
			events := make(chan chatlog.Event)
			t.Cleanup(func() { close(events) })
			go pushed(t, ln, events)

			thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: socket, Live: true}}
			chats := chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit)
			heard := make(chan chatlog.Event, 4)
			chats.OnTurnEnd(func(_ string, e chatlog.Event) {
				if e.Kind == chatlog.KindExit {
					heard <- e
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			chats.Start(ctx)
			apptest.Eventually(t, "the log read", func() bool { _, known := chats.TurnOver(thread.chat.ID, 0); return known })
			time.Sleep(100 * time.Millisecond) // the follower tells what it read just after it is read
			if got := len(heard); got != tt.want {
				t.Errorf("OnTurnEnd heard %d exits, want %d", got, tt.want)
			}
		})
	}
}

func TestRateLimitEventsAreHandedOn(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket, logPath := filepath.Join(dir, "host.sock"), filepath.Join(dir, "chat.jsonl")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	events := make(chan chatlog.Event)
	t.Cleanup(func() { close(events) })
	go pushed(t, ln, events)

	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: logPath, Socket: socket, Live: true}}
	chats := chat.New(thread, t.TempDir(), (&apptest.Recorder{}).Emit)
	type heard struct {
		id string
		e  chatlog.Event
	}
	limits := make(chan heard, 4)
	chats.OnRateLimit(func(id string, e chatlog.Event) { limits <- heard{id, e} })
	turns := make(chan chatlog.Event, 4)
	chats.OnTurnEnd(func(_ string, e chatlog.Event) { turns <- e })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chats.Start(ctx)

	info := json.RawMessage(`{"rateLimitType":"five_hour","utilization":0.4,"resetsAt":1796079600}`)
	events <- chatlog.Event{Kind: chatlog.KindRateLimit, Input: info}
	events <- chatlog.Event{Seq: 1, Kind: chatlog.KindTurnEnd}
	select {
	case got := <-limits:
		if got.id != thread.chat.ID || string(got.e.Input) != string(info) {
			t.Errorf("OnRateLimit heard %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnRateLimit heard nothing")
	}
	select {
	case <-turns:
	case <-time.After(5 * time.Second):
		t.Fatal("the follower took nothing after the rate_limit event")
	}
	if len(limits) != 0 {
		t.Errorf("OnRateLimit heard %d more", len(limits))
	}
}
