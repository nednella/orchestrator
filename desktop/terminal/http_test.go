package terminal_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/terminal"
)

// reading is one GET /term/<id> on the app's assets, read as the window reads it.
type reading struct {
	header http.Header
	done   chan struct{}

	mu      sync.Mutex
	status  int
	body    bytes.Buffer
	writes  int
	flushes int
	gate    chan struct{} // while set, a write waits for it to close, as a slow browser holds the reader up
}

func (r *reading) Header() http.Header { return r.header }

func (r *reading) WriteHeader(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == 0 {
		r.status = code
	}
}

func (r *reading) Write(b []byte) (int, error) {
	r.mu.Lock()
	gate := r.gate
	r.mu.Unlock()
	if gate != nil {
		<-gate
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.writes++
	return r.body.Write(b)
}

// hold makes the next writes wait until release.
func (r *reading) hold() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gate = make(chan struct{})
}

func (r *reading) release() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gate != nil {
		close(r.gate)
		r.gate = nil
	}
}

func (r *reading) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushes++
}

func (r *reading) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return bytes.Clone(r.body.Bytes())
}

func (r *reading) String() string { return string(r.Bytes()) }

func (r *reading) Writes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writes
}

func (r *reading) Status() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *reading) ended() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

// read starts a GET of the terminal's output, as the front end does once TermOpen has returned.
func read(t *testing.T, h *apptest.Harness, id string) *reading {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &reading{header: http.Header{}, done: make(chan struct{})}
	req := httptest.NewRequestWithContext(ctx, "GET", terminal.Prefix+url.PathEscape(id), nil)
	req.Header.Set("X-Orchestrator", "1")
	go func() {
		defer close(r.done)
		h.App.Assets().ServeHTTP(r, req)
	}()
	t.Cleanup(func() {
		cancel()
		<-r.done
	})
	return r
}

// openRead opens the shell's terminal and reads its output.
func openRead(t *testing.T, h *apptest.Harness, id string, cols, rows int) *reading {
	t.Helper()
	if err := h.TermOpen(id, cols, rows); err != nil {
		t.Fatal(err)
	}
	return read(t, h, id)
}

// passthrough wraps b so tmux hands it to the terminal unchanged: the body of a passthrough sequence, each ESC doubled.
func passthrough(b []byte) []byte {
	out := []byte("\x1bPtmux;")
	out = append(out, bytes.ReplaceAll(b, []byte{0x1b}, []byte{0x1b, 0x1b})...)
	return append(out, "\x1b\\"...)
}

func TestStreamCarriesRawBytesInOrder(t *testing.T) {
	h, id := newShell(t)
	out := openRead(t, h, id, 100, 30)
	eventually(t, "the prompt", func() bool { return out.Status() == http.StatusOK && len(out.Bytes()) > 0 })
	if got := out.header.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := out.header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q", got)
	}

	var raw []byte
	for range 16 {
		for b := range 256 {
			raw = append(raw, byte(b))
		}
		raw = append(raw, 0xff, 0xfe, 0xc3, 0x28, 0xe2, 0x82) // not UTF-8
	}
	file := filepath.Join(t.TempDir(), "raw")
	if err := os.WriteFile(file, passthrough(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.TermWrite(id, "stty -opost; cat '"+file+"'; stty opost\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the bytes, unchanged", func() bool { return bytes.Contains(out.Bytes(), raw) })
}

func TestStreamRefusesAThreadID(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("a thread", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = h.TermOpen(s.ID, 80, 24)
	for _, id := range []string{s.ID, "main/shell"} {
		out := read(t, h, id)
		eventually(t, "the answer for "+id, out.ended)
		if out.Status() != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", id, out.Status())
		}
	}
}

func TestStreamNeedsTheAppHeader(t *testing.T) {
	h, id := newShell(t)
	out := openRead(t, h, id, 80, 24)
	eventually(t, "the first draw", func() bool { return len(out.Bytes()) > 0 })
	// A same-origin <img> or <script> sends no header of the app's own.
	for _, header := range []string{"", "0"} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) // a GET it takes would stream on
		req := httptest.NewRequestWithContext(ctx, "GET", terminal.Prefix+url.PathEscape(id), nil)
		if header != "" {
			req.Header.Set("X-Orchestrator", header)
		}
		rec := httptest.NewRecorder()
		h.App.Assets().ServeHTTP(rec, req)
		cancel()
		if rec.Code != http.StatusForbidden {
			t.Errorf("X-Orchestrator %q: status %d, want 403", header, rec.Code)
		}
	}
	if out.ended() {
		t.Fatal("a refused GET ended the reader")
	}
	if err := h.TermWrite(id, "echo $((40+2))\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the echo on the reader", func() bool { return strings.Contains(out.String(), "\r\n42\r\n") })
}

func TestSecondReaderReplacesTheFirst(t *testing.T) {
	h, id := newShell(t)
	first := openRead(t, h, id, 80, 24)
	eventually(t, "the first draw", func() bool { return len(first.Bytes()) > 0 })
	second := read(t, h, id)
	eventually(t, "the first reader to end", first.ended)
	if err := h.TermWrite(id, "echo $((40+2))\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the echo on the second reader", func() bool { return strings.Contains(second.String(), "\r\n42\r\n") })
	if strings.Contains(first.String(), "42") {
		t.Error("the replaced reader still got output")
	}
}

func TestStreamEndsWhenTheShellExits(t *testing.T) {
	t.Run("the shell exits", func(t *testing.T) {
		h, id := newShell(t)
		out := openRead(t, h, id, 80, 24)
		eventually(t, "the first draw", func() bool { return len(out.Bytes()) > 0 })
		if err := exec.Command("tmux", "-L", h.Socket, "kill-session", "-t", "="+id).Run(); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the end of the body", out.ended)
	})
	t.Run("the terminal closes", func(t *testing.T) {
		h, id := newShell(t)
		out := openRead(t, h, id, 80, 24)
		eventually(t, "the first draw", func() bool { return len(out.Bytes()) > 0 })
		h.TermClose(id)
		eventually(t, "the end of the body", out.ended)
	})
}

// flood writes a file that, printed, sends n × 512 KB through tmux to the terminal unchanged.
func flood(t *testing.T, n int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capped-output-file")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	chunk := passthrough(bytes.Repeat([]byte("x"), 512<<10)) // tmux keeps at most 1 MB of one sequence
	for range n {
		if _, err := f.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestALossWhileReadingOnlyRepaints(t *testing.T) {
	h, id := newShell(t)
	out := openRead(t, h, id, 100, 30)
	t.Cleanup(out.release)
	eventually(t, "the prompt", func() bool { return len(out.Bytes()) > 0 })
	big, done := flood(t, 20), filepath.Join(t.TempDir(), "done")
	out.hold()
	cmd := `printf '\033[H\033[2J'; echo after-the-clear; cat '` + big + `'; touch '` + done + "'\n"
	if err := h.TermWrite(id, cmd); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the 10 MB printed", func() bool { _, err := os.Stat(done); return err == nil })
	time.Sleep(time.Second) // for the last of it to leave tmux
	before := len(out.Bytes())
	out.release()
	eventually(t, "the repaint", func() bool { return strings.Contains(out.String()[before:], "after-the-clear") })
	// The write held up is at most a few reads; what was left after the drop is about 2 MB.
	if n := len(out.Bytes()) - before; n > 512<<10 {
		t.Errorf("the reader got %d bytes after the loss, want only the held write and the repaint", n)
	}
}

func TestQueuedOutputIsCapped(t *testing.T) {
	h, id := newShell(t)
	if err := h.TermOpen(id, 100, 30); err != nil {
		t.Fatal(err)
	}
	big, done := flood(t, 18), filepath.Join(t.TempDir(), "done")
	// The screen is cleared and then shows one line before the 9 MB, so only a redraw brings that line back.
	cmd := `echo before-the-flood; printf '\033[H\033[2J'; echo after-the-clear; cat '` + big + `'; touch '` + done + "'\n"
	if err := h.TermWrite(id, cmd); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the 9 MB printed", func() bool { _, err := os.Stat(done); return err == nil })
	time.Sleep(time.Second) // for the last of it to leave tmux

	out := read(t, h, id)
	eventually(t, "the redraw", func() bool { return strings.Contains(out.String(), "after-the-clear") })
	if strings.Contains(out.String(), "before-the-flood") {
		t.Error("the reader got output from before the queue overflowed")
	}
	if n := len(out.Bytes()); n > 8<<20 {
		t.Errorf("the reader got %d bytes, want at most 8 MB", n)
	}
}
