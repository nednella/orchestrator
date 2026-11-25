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
	go chats.Run(ctx)
	svc := chat.NewService(chats, context.Background)
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
