package chat_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/chat"
	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
)

// oneThread is a single live thread under a host at socket, with an empty chat log.
type oneThread struct{ chat sessions.Chat }

func (o oneThread) ChatOf(string) (sessions.Chat, error) { return o.chat, nil }
func (o oneThread) Chats() []sessions.Chat               { return []sessions.Chat{o.chat} }
func (o oneThread) TurnEnded(string, chatlog.Event)      {}
func (o oneThread) AccessChanged(string, string)         {}
func (o oneThread) Wake(string) (sessions.Chat, error)   { return o.chat, nil }

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
	chats := chat.New(thread, rec.Emit)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chats.Start(ctx)
	svc := chat.NewService(chats, nil, context.Background)
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
	chats := chat.New(thread, rec.Emit)
	svc := chat.NewService(chats, nil, context.Background)
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
	dir := t.TempDir()
	thread := oneThread{sessions.Chat{ID: "main/abcdefgh", Project: "main", Log: filepath.Join(dir, "chat.jsonl"), Socket: filepath.Join(dir, "gone.sock"), Live: true}}
	svc := chat.NewService(chat.New(thread, (&apptest.Recorder{}).Emit), nil, context.Background)
	if err, want := svc.ChatInterrupt(thread.chat.ID), "Thread main/abcdefgh did not take the interrupt"; err == nil || err.Error() != want {
		t.Errorf("ChatInterrupt = %v, want %q", err, want)
	}
}

func TestPendingIsRightBeforeTheFollowerReadsTheLog(t *testing.T) {
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
	svc := chat.NewService(chat.New(thread, (&apptest.Recorder{}).Emit), nil, context.Background)
	want := []chat.Pending{{ID: thread.chat.ID, Request: "req-2", Tool: "Bash", Summary: "make test", Kind: "tool"}}
	if got := svc.ChatPending(); !slices.Equal(got, want) {
		t.Errorf("ChatPending before any follower = %+v, want %+v", got, want)
	}
}

func TestAnOldHostSendsTheTextAndSaysSo(t *testing.T) {
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
	svc := chat.NewService(chat.New(thread, (&apptest.Recorder{}).Emit), nil, context.Background)
	sent, err := svc.ChatSend(thread.chat.ID, "see", []chat.Image{{Base64: dot, Mime: "image/png"}})
	if want := "Your text was sent, but this thread's host is too old for images. Restart the thread to send images."; err != nil || sent.Notice != want {
		t.Errorf("ChatSend = %+v, %v; want the notice %q", sent, err, want)
	}
}
