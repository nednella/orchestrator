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
	last, end, err := lastSeq(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Truncate(end); err != nil {
		f.Close()
		return nil, err
	}
	return &Log{f: f, last: last, size: end}, nil
}

// lastSeq returns the Seq of the log's last event that parses, and the offset where its last whole line ends. It
// decodes only the last line, unless that line does not parse.
func lastSeq(f *os.File) (int64, int64, error) {
	var tail []byte
	end, err := eachLine(f, func(line []byte) { tail = line })
	if err != nil {
		return 0, 0, err
	}
	var e Event
	if tail == nil || json.Unmarshal(tail, &e) == nil {
		return e.Seq, end, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, 0, err
	}
	var last int64
	_, err = eachLine(f, func(line []byte) {
		var e Event
		if json.Unmarshal(line, &e) == nil {
			last = e.Seq
		}
	})
	return last, end, err
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
	out := []Event{}
	if _, err := eachLine(f, func(line []byte) {
		var e Event
		if json.Unmarshal(line, &e) == nil && e.Seq > after {
			out = append(out, e)
		}
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// eachLine calls fn with each whole line of r, which fn may keep, and returns the offset where the last whole line ends.
func eachLine(r io.Reader, fn func(line []byte)) (int64, error) {
	br := bufio.NewReaderSize(r, 1<<16)
	var end int64
	for {
		line, err := br.ReadBytes('\n')
		if err == io.EOF {
			return end, nil
		}
		if err != nil {
			return 0, err
		}
		end += int64(len(line))
		fn(line)
	}
}
