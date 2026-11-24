package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func logPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "threads", "t1", "chat.jsonl")
}

func openLog(t *testing.T, p string) *Log {
	t.Helper()
	l, err := OpenLog(p)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func mustAppend(t *testing.T, l *Log, e Event) Event {
	t.Helper()
	got, err := l.Append(e)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestAppendNumbersEventsAndSkipsDeltas(t *testing.T) {
	p := logPath(t)
	l := openLog(t, p)
	defer l.Close()

	u := mustAppend(t, l, Event{Kind: KindUser, Text: "hi"})
	d := mustAppend(t, l, Event{Kind: KindDelta, Text: "h"})
	m := mustAppend(t, l, Event{Kind: KindMessage, Text: "hello"})
	if u.Seq != 1 || d.Seq != 0 || m.Seq != 2 {
		t.Fatalf("seqs = %d, %d, %d; want 1, 0, 2", u.Seq, d.Seq, m.Seq)
	}
	if u.At == 0 {
		t.Fatal("At was not set")
	}
	got, err := ReadLog(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ReadLog returned %d events, want 2", len(got))
	}
}

func TestReopenContinuesTheSequence(t *testing.T) {
	p := logPath(t)
	l := openLog(t, p)
	mustAppend(t, l, Event{Kind: KindUser})
	mustAppend(t, l, Event{Kind: KindMessage})
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	l = openLog(t, p)
	defer l.Close()
	if got := mustAppend(t, l, Event{Kind: KindUser}); got.Seq != 3 {
		t.Fatalf("seq = %d, want 3", got.Seq)
	}
}

func TestReadLogIgnoresAPartialLastLine(t *testing.T) {
	p := logPath(t)
	l := openLog(t, p)
	mustAppend(t, l, Event{Kind: KindUser})
	mustAppend(t, l, Event{Kind: KindMessage})
	l.Close()
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"seq":3,"ki`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := ReadLog(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ReadLog returned %d events, want 2", len(got))
	}
	l = openLog(t, p)
	defer l.Close()
	if e := mustAppend(t, l, Event{Kind: KindUser}); e.Seq != 3 {
		t.Fatalf("seq = %d, want 3", e.Seq)
	}
	got, err = ReadLog(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("ReadLog returned %d events, want 3", len(got))
	}
}

func TestReadLogAfter(t *testing.T) {
	p := logPath(t)
	l := openLog(t, p)
	defer l.Close()
	mustAppend(t, l, Event{Kind: KindUser})
	mustAppend(t, l, Event{Kind: KindMessage})
	got, err := ReadLog(p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Seq != 2 {
		t.Fatalf("got %+v, want only seq 2", got)
	}
}

func TestLargeEventRoundTrips(t *testing.T) {
	p := logPath(t)
	l := openLog(t, p)
	defer l.Close()
	text := strings.Repeat("x", 1<<20+10)
	mustAppend(t, l, Event{Kind: KindToolResult, Text: text})
	mustAppend(t, l, Event{Kind: KindMessage, Text: "after"})
	got, err := ReadLog(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != text {
		t.Fatalf("large event did not round-trip (%d events)", len(got))
	}
}

func TestConcurrentAppendsKeepTheSequence(t *testing.T) {
	p := logPath(t)
	l := openLog(t, p)
	defer l.Close()
	const workers, each = 8, 50
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				if _, err := l.Append(Event{Kind: KindMessage, Text: "x"}); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()

	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != workers*each {
		t.Fatalf("%d lines, want %d", len(lines), workers*each)
	}
	for i, line := range lines {
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		if e.Seq != int64(i+1) {
			t.Fatalf("line %d has seq %d", i+1, e.Seq)
		}
	}
}
