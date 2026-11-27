package terminal_test

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTerminalStream(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	out := openRead(t, h, id, 100, 30)
	eventually(t, "shell output", func() bool { return len(out.String()) > 0 })

	// Output that comes while tmux's paint of the screen waits is drawn in that paint, each line ended by an erase, not a
	// line break: each output is looked for anywhere in the stream, in a form the typed command does not hold.
	if err := h.TermWrite(id, "echo $((40+2))xy\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "echo result", func() bool { return strings.Contains(out.String(), "42xy") })

	if err := h.TermResize(id, 77, 20); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := h.TermWrite(id, "stty size | tr ' ' x\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "resized pty", func() bool { return strings.Contains(out.String(), "20x77") })

	h.TermClose(id)
	if err := h.TermWrite(id, "x"); err == nil {
		t.Error("writing to a closed terminal succeeded")
	}
	if err := h.TermOpen("not a thread", 80, 24); err == nil {
		t.Error("a name that is no thread was attached")
	}
}

func TestOpenRefusesAThreadID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, _ := h.StartThread("a thread", "")
	if err := h.TermOpen(s.ID, 80, 24); err == nil || err.Error() != "Only a shell has a terminal" {
		t.Errorf("open = %v, want %q", err, "Only a shell has a terminal")
	}
	if out, err := exec.Command("tmux", "-L", h.Socket, "list-clients").Output(); err != nil || len(out) != 0 {
		t.Errorf("tmux clients = %q (%v), want none", out, err)
	}
	if err := h.TermWrite(s.ID, "x"); err == nil {
		t.Error("a refused open left a stream to write to")
	}
}

func TestClosingATerminalKeepsTheShellRunning(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	if err := h.TermOpen(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	h.TermClose(id)
	time.Sleep(300 * time.Millisecond)
	if err := exec.Command("tmux", "-L", h.Socket, "has-session", "-t", "="+id).Run(); err != nil {
		t.Errorf("the shell ended with its terminal: %v", err)
	}
	if h.Rec.Count("term:exit") != 0 {
		t.Error("a terminal we closed ourselves reported an exit")
	}
}

func TestTerminalReportsAShellThatEnds(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	if err := h.TermOpen(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("tmux", "-L", h.Socket, "kill-session", "-t", "="+id).Run(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "term:exit", func() bool { return h.Rec.Count("term:exit") == 1 })
}

func TestReopeningRedrawsTheScreen(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	first := openRead(t, h, id, 80, 24)
	eventually(t, "the first draw", func() bool { return len(first.String()) > 0 })
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("the first reader got %q", first.String())
		}
	})
	if err := h.TermWrite(id, "echo before-reopen\n"); err != nil {
		t.Fatal(err)
	}
	// A redraw of the line, from a resize or the shell's own line editor, can show the text more than twice.
	eventually(t, "the command and its output", func() bool { return strings.Count(first.String(), "before-reopen") >= 2 })
	time.Sleep(300 * time.Millisecond)
	seen := strings.Count(first.String(), "before-reopen")
	second := openRead(t, h, id, 80, 24)
	// Soon, so a later redraw for some other reason does not pass for this one.
	deadline := time.Now().Add(time.Second)
	for strings.Count(second.String(), "before-reopen") < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("the reopened stream got no full redraw that still shows the screen; got %q", second.String())
		}
		time.Sleep(25 * time.Millisecond)
	}
	if n := strings.Count(first.String(), "before-reopen"); n != seen {
		t.Errorf("the replaced reader saw before-reopen %d times, want the %d from before the reopen", n, seen)
	}
}

func TestReopeningBeforeTheFirstDrawWorks(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	first := openRead(t, h, id, 80, 24)
	eventually(t, "the first reader attached", func() bool { return first.Status() == http.StatusOK })
	out := openRead(t, h, id, 80, 24)
	eventually(t, "the first draw", func() bool { return len(out.String()) > 0 })
}

func TestHeavyOutputIsMerged(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	out := openRead(t, h, id, 120, 30)
	eventually(t, "the prompt", func() bool { return len(out.String()) > 0 })
	before := out.Writes()
	if err := h.TermWrite(id, "seq 1 30000; echo done-seq\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the end of the output", func() bool { return strings.Contains(out.String(), "done-seq") })
	writes := out.Writes() - before
	if bytes := len(out.String()); writes > bytes/200 {
		t.Errorf("%d writes for %d bytes: output is not merged", writes, bytes)
	}
}

func TestClipboardFromTheAgent(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	out := openRead(t, h, id, 100, 30)
	eventually(t, "shell output", func() bool { return len(out.String()) > 0 })
	cmd := `printf '\033]52;c;%s\a' $(printf copied-text | base64)` + "\n"
	if err := h.TermWrite(id, cmd); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the clipboard", func() bool { return h.Rec.HasClip("copied-text") })
	// The clipboard copy runs synchronously off each PTY read; the output waits for the flush
	// timer, so it can still be pending once HasClip turns true.
	eventually(t, "the OSC 52 bytes forwarded in the output", func() bool { return strings.Contains(out.String(), "\x1b]52;") })
}

func TestTerminalAttachesToTheShell(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	// Wide, so the path in the shell's answer, which has random digits in it, never wraps.
	out := openRead(t, h, id, 300, 30)
	eventually(t, "shell output", func() bool { return len(out.String()) > 0 })
	// The answer is looked for anywhere in the stream, as in TestTerminalStream; the | after the folder ends it.
	if err := h.TermWrite(id, "echo \"$ORCHESTRATOR_PROJECT|${ORCHESTRATOR_THREAD-unset}|${ORCHESTRATOR_CONTROL-unset}|$(pwd -P)|\"\n"); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(h.Dir)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the shell's environment", func() bool {
		return strings.Contains(out.String(), "main|unset|unset|"+want+"|")
	})
	h.TermClose(id)
}

func TestOpeningAnOpenTerminalKeepsItsStream(t *testing.T) {
	t.Parallel()
	h, id := newShell(t)
	out := openRead(t, h, id, 80, 24)
	eventually(t, "the first draw", func() bool { return len(out.String()) > 0 })
	clients := func() string {
		out, err := exec.Command("tmux", "-L", h.Socket, "list-clients", "-F", "#{client_pid} #{client_width}x#{client_height}").Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	before := clients()
	if strings.Count(before, "\n") != 0 || before == "" {
		t.Fatalf("clients = %q, want one", before)
	}
	pid, _, _ := strings.Cut(before, " ")

	if err := h.TermOpen(id, 90, 26); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the same client at the new size", func() bool { return clients() == pid+" 90x26" })
	if h.Rec.Count("term:exit") != 0 {
		t.Error("reopening ended the stream")
	}
	if err := h.TermWrite(id, "echo still-here\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the stream to carry on", func() bool { return strings.Contains(out.String(), "still-here") })
}
