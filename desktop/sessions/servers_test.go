package sessions_test

import (
	"bufio"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
)

// freePort is a port in 18000–18999 that nothing listens on now.
func freePort(t *testing.T) int {
	t.Helper()
	first := rand.IntN(1000)
	for i := range 1000 {
		port := 18000 + (first+i)%1000
		if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
			ln.Close()
			return port
		}
	}
	t.Fatal("no free port in 18000–18999")
	return 0
}

// startOf is when process pid started, as ps says, "" when it is gone.
func startOf(pid int) string {
	out, _ := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	return strings.TrimSpace(string(out))
}

// stopAtCleanup ends process pid at the test's end while it is still the process that started then.
func stopAtCleanup(t *testing.T, pid int) {
	t.Helper()
	started := startOf(pid)
	if started == "" {
		t.Fatalf("process %d is gone", pid)
	}
	t.Cleanup(func() {
		if startOf(pid) == started {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
}

func running(pid int, started string) bool { return started != "" && startOf(pid) == started }

// threadServing starts a thread whose stand-in claude starts a listener on a free port, as a Bash call that starts a
// server with & does, and returns the thread, the port and the listener's pid.
func threadServing(t *testing.T, h *apptest.Harness) (sessions.Session, int, int) {
	t.Helper()
	port := freePort(t)
	thread, err := h.NewSession("serve", fmt.Sprintf("serve %d", port))
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(h.ClaudeDir(), strings.ReplaceAll(thread.ID, "/", "_")+".serve"+strconv.Itoa(port))
	var pid int
	eventually(t, "the thread's listener", func() bool {
		data, err := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(string(data))
		return err == nil && pid > 0
	})
	stopAtCleanup(t, pid)
	return thread, port, pid
}

// listener starts a listener on a free port with env on top of this environment without an agentos marker, in a
// session of its own, and returns its port and pid. The cleanup ends it.
func listener(t *testing.T, env ...string) (int, int) {
	t.Helper()
	port := freePort(t)
	cmd := exec.Command("python3", "-c", `import socket, sys, time
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", int(sys.argv[1])))
s.listen(5)
print("ready", flush=True)
time.sleep(120)`, strconv.Itoa(port))
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "AGENTOS_SESSION=") || strings.HasPrefix(kv, "AGENTOS_SOCKET=")
	}), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("the listener on %d did not start: %q, %v", port, line, err)
	}
	return port, cmd.Process.Pid
}

// marker is the environment of a process a thread of the harness's app started.
func marker(h *apptest.Harness, id string) []string {
	return []string{"AGENTOS_SESSION=" + id, "AGENTOS_SOCKET=" + bus.SocketPath(h.State)}
}

func TestServersListAThreadsListener(t *testing.T) {
	h := newHarness(t)
	thread, port, pid := threadServing(t, h)
	listener(t) // the owner's, with no marker

	var list []sessions.Server
	eventually(t, "the thread's server listed", func() bool {
		var err error
		list, err = h.Servers(thread.ID)
		return err == nil && len(list) > 0
	})
	if len(list) != 1 || list[0].Port != port || list[0].PID != pid || !strings.Contains(list[0].Command, "-c import os, socket") {
		t.Errorf("Servers = %+v, want the listener %d on %d alone", list, pid, port)
	}
	if err := h.OpenServer(port); err != nil {
		t.Fatal(err)
	}
	if h.GH.Calls(fmt.Sprintf("open http://localhost:%d", port)) != 1 {
		t.Errorf("calls = %q, want localhost:%d opened", h.GH.CallLog(), port)
	}
}

func TestServersOfNoThread(t *testing.T) {
	h := newHarness(t)
	shell, err := h.ShellOpen("")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, id string
	}{
		{"an id this app does not know", "main/zzzzzzzz"},
		{"a shell", shell.ID},
		{"no id", "nonsense"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if list, err := h.Servers(tt.id); err == nil {
				t.Errorf("Servers(%q) = %v, want an error", tt.id, list)
			}
		})
	}
}

func TestOpenServerTakesOnlyAPort(t *testing.T) {
	h := newHarness(t)
	for _, port := range []int{0, -1, 65536} {
		if err := h.OpenServer(port); err == nil {
			t.Errorf("OpenServer(%d) opened it", port)
		}
	}
	if n := h.GH.Calls("open"); n != 0 {
		t.Errorf("%d open calls", n)
	}
}

func TestSleepStopsAThreadsServers(t *testing.T) {
	h := newHarness(t)
	thread, _, pid := threadServing(t, h)
	started := startOf(pid)
	_, owners := listener(t)
	ownersStart := startOf(owners)
	other, err := h.NewSession("other", "")
	if err != nil {
		t.Fatal(err)
	}
	_, others := listener(t, marker(h, other.ID)...)
	othersStart := startOf(others)
	_, unknown := listener(t, marker(h, "main/zzzzzzzz")...)
	unknownStart := startOf(unknown)

	if err := h.SettleThread(thread.ID); err != nil {
		t.Fatal(err)
	}
	if running(pid, started) {
		t.Error("the thread's server runs after the thread slept")
	}
	if !running(owners, ownersStart) || !running(others, othersStart) || !running(unknown, unknownStart) {
		t.Error("the thread's sleep stopped a process that is not its own")
	}
}

func TestDeleteStopsAThreadsServers(t *testing.T) {
	h := newHarness(t)
	thread, _, pid := threadServing(t, h)
	started := startOf(pid)
	if err := h.DeleteThread(thread.ID, false); err != nil {
		t.Fatal(err)
	}
	if running(pid, started) {
		t.Error("the thread's server runs after the thread was deleted")
	}
}

func TestStopServer(t *testing.T) {
	h := newHarness(t)
	thread, _, pid := threadServing(t, h)
	started := startOf(pid)
	_, owners := listener(t)
	ownersStart := startOf(owners)
	_, others := listener(t, marker(h, "main/zzzzzzzz")...)
	othersStart := startOf(others)
	eventually(t, "the thread's server listed", func() bool { list, _ := h.Servers(thread.ID); return len(list) > 0 })

	for _, not := range []int{owners, others, os.Getpid()} {
		if err := h.StopServer(thread.ID, not); err == nil {
			t.Errorf("StopServer(%d) took a process that is not the thread's server", not)
		}
	}
	if !running(owners, ownersStart) || !running(others, othersStart) {
		t.Fatal("StopServer stopped a process that is not the thread's")
	}
	if err := h.StopServer(thread.ID, pid); err != nil {
		t.Fatal(err)
	}
	if running(pid, started) {
		t.Error("the thread's server runs after StopServer")
	}
	if list, err := h.Servers(thread.ID); err != nil || len(list) != 0 {
		t.Errorf("Servers = %v, %v; want none", list, err)
	}
	if s, ok := h.Session(thread.ID); !ok || s.State == session.Settled {
		t.Errorf("the thread is %+v after its server stopped, want it running", s)
	}
}

// A host killed, as by SIGKILL or a crash, settles its thread without the app stopping anything; the next start of the
// app stops the servers it left, and no process of a thread it does not know.
func TestStartStopsTheServersOfSettledThreads(t *testing.T) {
	h := newHarness(t)
	thread, _, pid := threadServing(t, h)
	started := startOf(pid)
	_, unknown := listener(t, marker(h, "main/zzzzzzzz")...)
	unknownStart := startOf(unknown)
	_, otherApps := listener(t, "AGENTOS_SESSION="+thread.ID, "AGENTOS_SOCKET=/another/app/agentos.sock")
	otherAppsStart := startOf(otherApps)
	if err := exec.Command("tmux", "-L", h.Socket, "kill-session", "-t", "="+thread.ID).Run(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { s, ok := h.Session(thread.ID); return ok && s.State == session.Settled })
	if !running(pid, started) {
		t.Fatal("the thread's server stopped with its tmux session; this test needs one that outlives it")
	}

	second := h.Restart(t)
	eventually(t, "the settled thread's server stopped", func() bool { return !running(pid, started) })
	if !running(unknown, unknownStart) || !running(otherApps, otherAppsStart) {
		t.Error("the start stopped a process of a thread this app does not know")
	}
	if list, err := second.Servers(thread.ID); err != nil || len(list) != 0 {
		t.Errorf("Servers = %v, %v; want none", list, err)
	}
}

// An app started on the same state folder with another tmux socket, as a check that sets only part of its isolation,
// finds the threads of the first settled. A thread whose host answers runs, so it keeps its servers; one whose host was
// killed loses them.
func TestStartLeavesTheServersOfAThreadWhoseHostAnswers(t *testing.T) {
	h := newHarness(t)
	live, _, livePID := threadServing(t, h)
	liveStart := startOf(livePID)
	killed, _, killedPID := threadServing(t, h)
	killedStart := startOf(killedPID)
	if err := exec.Command("tmux", "-L", h.Socket, "kill-session", "-t", "="+killed.ID).Run(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the killed thread settled", func() bool { s, ok := h.Session(killed.ID); return ok && s.State == session.Settled })

	// Old enough that the second app does not take it for a thread tmux has not listed yet.
	rec, err := bus.ReadState(h.State, live.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec.At = time.Now().Add(-time.Minute)
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AGENTOS_TMUX_SOCKET", h.Socket+"-other")
	second := h.Restart(t)
	eventually(t, "the second app sees the live thread settled", func() bool {
		for _, s := range second.Sessions().List() {
			if s.ID == live.ID {
				return s.State == session.Settled
			}
		}
		return false
	})
	// One sweep stops both or neither, so once the killed thread's server is gone it has passed the live one.
	eventually(t, "the killed thread's server stopped", func() bool { return !running(killedPID, killedStart) })
	if !running(livePID, liveStart) {
		t.Errorf("the second app stopped a server of thread %s, which runs under the first", live.ID)
	}
}

func TestShellCarriesNoThreadMarker(t *testing.T) {
	t.Setenv("AGENTOS_SESSION", "main/aaaaaaaa") // as an app started from a thread's terminal has
	h := newHarness(t)
	thread, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	shell, err := h.ShellOpen(thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "env")
	if err := exec.Command("tmux", "-L", h.Socket, "send-keys", "-t", "="+shell.ID+":", "env > "+out, "Enter").Run(); err != nil {
		t.Fatal(err)
	}
	var env string
	eventually(t, "the shell's environment", func() bool {
		data, err := os.ReadFile(out)
		env = string(data)
		return err == nil && strings.Contains(env, "AGENTOS_PROJECT=")
	})
	if strings.Contains(env, "AGENTOS_SESSION=") {
		t.Errorf("the shell has a thread marker:\n%s", env)
	}
}
