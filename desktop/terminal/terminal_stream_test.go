package terminal_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTerminalStream(t *testing.T) {
	h, id := newShell(t)
	out := openRead(t, h, id, 100, 30)
	eventually(t, "shell output", func() bool { return len(out.String()) > 0 })

	if err := h.TermWrite(id, "echo $((40+2))\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "echo result", func() bool { return strings.Contains(out.String(), "\r\n42\r\n") })

	if err := h.TermResize(id, 77, 20); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := h.TermWrite(id, "stty size\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "resized pty", func() bool { return strings.Contains(out.String(), "\r\n20 77\r\n") })

	h.TermClose(id)
	if err := h.TermWrite(id, "x"); err == nil {
		t.Error("writing to a closed terminal succeeded")
	}
	if err := h.TermOpen("not a session", 80, 24); err == nil {
		t.Error("a name that is no session was attached")
	}
}

func TestOpenRefusesAThreadID(t *testing.T) {
	h := newHarness(t)
	s, _ := h.NewSession("a thread", "")
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
	h, id := newShell(t)
	first := openRead(t, h, id, 80, 24)
	eventually(t, "the first draw", func() bool { return len(first.String()) > 0 })
	if err := h.TermWrite(id, "echo before-reopen\n"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the command and its output", func() bool { return strings.Count(first.String(), "before-reopen") == 2 })
	time.Sleep(300 * time.Millisecond)
	second := openRead(t, h, id, 80, 24)
	// Soon, so a later redraw for some other reason does not pass for this one.
	deadline := time.Now().Add(time.Second)
	for strings.Count(second.String(), "before-reopen") < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("the reopened stream got no full redraw that still shows the screen; got %q", second.String())
		}
		time.Sleep(25 * time.Millisecond)
	}
	if n := strings.Count(first.String(), "before-reopen"); n != 2 {
		t.Errorf("the replaced reader saw before-reopen %d times, want the 2 from before the reopen", n)
	}
}

func TestReopeningBeforeTheFirstDrawWorks(t *testing.T) {
	h, id := newShell(t)
	openRead(t, h, id, 80, 24)
	out := openRead(t, h, id, 80, 24)
	eventually(t, "the first draw", func() bool { return len(out.String()) > 0 })
}

func TestHeavyOutputIsMerged(t *testing.T) {
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
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("HOME", t.TempDir())
	h := newHarness(t)
	shell, err := h.ShellNew("")
	if err != nil {
		t.Fatal(err)
	}
	out := openRead(t, h, shell.ID, 100, 30)
	eventually(t, "shell output", func() bool { return len(out.String()) > 0 })
	if err := h.TermWrite(shell.ID, "echo \"$AGENTOS_PROJECT|${AGENTOS_SESSION-unset}|${AGENTOS_SOCKET:+set}|$(pwd -P)\"\n"); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(h.Dir)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the shell's environment", func() bool {
		return strings.Contains(out.String(), "\r\nmain|unset|set|"+want+"\r\n")
	})
	h.TermClose(shell.ID)
}

func TestOpeningAnOpenTerminalKeepsItsStream(t *testing.T) {
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
