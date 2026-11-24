package chat

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Log is a thread's append-only chat log: one JSON event per line.
type Log struct {
	mu   sync.Mutex
	f    *os.File
	last int64
	size int64
}

// OpenLog opens the log at path, creating it and its folders. A partial last line, left by a crash, is cut off.
func OpenLog(path string) (*Log, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, err
	}
	events, end, err := readEvents(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Truncate(end); err != nil {
		f.Close()
		return nil, err
	}
	l := &Log{f: f, size: end}
	if n := len(events); n > 0 {
		l.last = events[n-1].Seq
	}
	return l, nil
}

// Append adds e to the log. A persisted event gets the next Seq and is written; a delta keeps Seq 0 and is not.
func (l *Log) Append(e Event) (Event, error) {
	if e.At == 0 {
		e.At = time.Now().UnixMilli()
	}
	if !e.Persisted() {
		return e, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	e.Seq = l.last + 1
	line, err := json.Marshal(e)
	if err != nil {
		return Event{}, err
	}
	line = append(line, '\n')
	if _, err := l.f.Write(line); err != nil {
		_ = l.f.Truncate(l.size)
		return Event{}, err
	}
	l.last = e.Seq
	l.size += int64(len(line))
	return e, nil
}

// Last returns the Seq of the newest event, 0 for an empty log.
func (l *Log) Last() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}

// ReadLog returns the events with Seq above after. A line that does not parse is skipped, and so is a last line with no newline.
func ReadLog(path string, after int64) ([]Event, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	events, _, err := readEvents(f)
	if err != nil {
		return nil, err
	}
	out := []Event{}
	for _, e := range events {
		if e.Seq > after {
			out = append(out, e)
		}
	}
	return out, nil
}

// readEvents parses every whole line and returns the offset where the last whole line ends.
func readEvents(r io.Reader) ([]Event, int64, error) {
	br := bufio.NewReader(r)
	var events []Event
	var end int64
	for {
		line, err := br.ReadBytes('\n')
		if err == io.EOF {
			return events, end, nil
		}
		if err != nil {
			return nil, 0, err
		}
		end += int64(len(line))
		var e Event
		if json.Unmarshal(line, &e) == nil {
			events = append(events, e)
		}
	}
}
