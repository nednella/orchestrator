package procs

import (
	"bufio"
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// freePort is a port in 18000–18999 that nothing listens on now.
func freePort(t *testing.T) int {
	t.Helper()
	first := rand.IntN(1000)
	for i := range 1000 {
		port := 18000 + (first+i)%1000
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	t.Fatal("no free port in 18000–18999")
	return 0
}

// listen is a listener on the port given, which says ready and lives two minutes. It is python and not one of the
// system's own programs, such as perl: macOS shows no one the environment of those.
const listen = `import socket, sys, time
s = socket.socket()
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", int(sys.argv[1])))
s.listen(5)
print("ready", flush=True)
time.sleep(120)`

// listener starts a listener on port in a session of its own, with env on top of this environment without any
// agentos marker. The cleanup ends it by its pid, which is this test's child until it is waited for.
func listener(t *testing.T, port int, env ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("python3", "-c", listen, fmt.Sprint(port))
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, sessionVar+"=") || strings.HasPrefix(kv, socketVar+"=")
	}), env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
	})
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || line != "ready\n" {
		t.Fatalf("the listener on %d did not start: %q, %v", port, line, err)
	}
	return cmd
}

func execRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func running(pid int) bool { return syscall.Kill(pid, 0) == nil }

func TestKernelReadsAProcess(t *testing.T) {
	cmd := listener(t, freePort(t), sessionVar+"=test/kernel", socketVar+"="+testSocket)
	argv, env, err := kernel{}.args(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 4 || argv[1] != "-c" || !slices.Contains(env, sessionVar+"=test/kernel") {
		t.Errorf("args = %q, env has the marker: %v", argv, slices.Contains(env, sessionVar+"=test/kernel"))
	}
	list, err := kernel{}.processes()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(list, func(p process) bool { return p.pid == cmd.Process.Pid })
	if i < 0 || list[i].ppid != os.Getpid() || list[i].pgid != cmd.Process.Pid || list[i].start == 0 {
		t.Errorf("the listener is listed as %+v", list[i:min(i+1, len(list))])
	}
}

func TestProcessWithNoTerminalIsNotBusy(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	if busy, err := Busy(cmd.Process.Pid); err != nil || busy {
		t.Errorf("Busy = %v, %v; want false", busy, err)
	}
}

func TestServersAndStopOnThisMac(t *testing.T) {
	socket := t.TempDir() + "/agentos.sock"
	id := fmt.Sprintf("test/%d", os.Getpid())
	mine := freePort(t)
	server := listener(t, mine, sessionVar+"="+id, socketVar+"="+socket)
	owners := listener(t, freePort(t))
	otherApps := listener(t, freePort(t), sessionVar+"="+id, socketVar+"=/another/app/agentos.sock")
	table := New(socket, execRunner)

	servers, err := table.Servers(context.Background(), map[string]int{id: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].PID != server.Process.Pid || !slices.Equal(servers[0].Ports, []int{mine}) || !strings.Contains(servers[0].Command, " -c import socket, sys, time s = socket.socket()") {
		t.Fatalf("Servers = %+v, want only the thread's listener on %d", servers, mine)
	}
	<-table.Stop(context.Background(), servers)
	// Stop returns once the kernel lists it no more, or lists it as a zombie, which this test reaps.
	list, err := kernel{}.processes()
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(list, func(p process) bool { return p.pid == server.Process.Pid }) {
		t.Error("the thread's listener still runs after Stop")
	}
	if !running(owners.Process.Pid) || !running(otherApps.Process.Pid) {
		t.Error("Stop stopped a listener without this app's marker")
	}
}
