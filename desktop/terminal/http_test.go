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
	defer r.mu.Unlock()
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.writes++
	return r.body.Write(b)
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

func TestQueuedOutputIsCapped(t *testing.T) {
	h, id := newShell(t)
	if err := h.TermOpen(id, 100, 30); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	big, done := filepath.Join(dir, "capped-output-file"), filepath.Join(dir, "done")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	chunk := passthrough(bytes.Repeat([]byte("x"), 512<<10)) // tmux keeps at most 1 MB of one sequence
	for range 18 {
		if _, err := f.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
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
