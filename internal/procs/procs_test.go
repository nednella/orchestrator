package procs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const testSocket = "/state/agentos.sock"

func marked(id string) []string {
	return []string{"HOME=/h", sessionVar + "=" + id, socketVar + "=" + testSocket}
}

type fakeProc struct {
	process
	argv, env []string
	stubborn  bool // ignores SIGTERM
}

type signalled struct {
	pid int
	sig syscall.Signal
}

// fakeSystem is a process table: a signal ends the processes it reaches, but SIGTERM spares a stubborn one.
type fakeSystem struct {
	mu    sync.Mutex
	procs map[int]*fakeProc
	sent  []signalled
}

func newFake(procs ...fakeProc) *fakeSystem {
	f := &fakeSystem{procs: map[int]*fakeProc{}}
	for _, p := range procs {
		f.procs[p.pid] = &p
	}
	return f
}

func (f *fakeSystem) processes() ([]process, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var list []process
	for _, p := range f.procs {
		list = append(list, p.process)
	}
	return list, nil
}

func (f *fakeSystem) args(pid int) ([]string, []string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.procs[pid]
	if !ok {
		return nil, nil, syscall.ESRCH
	}
	return p.argv, p.env, nil
}

func (f *fakeSystem) signal(pid int, sig syscall.Signal) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, signalled{pid, sig})
	for _, p := range f.procs {
		if (pid < 0 && p.pgid == -pid || p.pid == pid) && (sig == syscall.SIGKILL || !p.stubborn) {
			delete(f.procs, p.pid)
		}
	}
	return nil
}

func (f *fakeSystem) signals() []signalled {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sent)
}

func (f *fakeSystem) alive(pid int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.procs[pid]
	return ok
}

// lsofSays is a runner that prints out as lsof, or fails as lsof does when nothing listens.
func lsofSays(out string) Runner {
	return func(_ context.Context, name string, _ ...string) ([]byte, error) {
		if name != "lsof" {
			return nil, fmt.Errorf("unexpected %s", name)
		}
		if out == "" {
			return nil, fmt.Errorf("lsof -nP: %w", exec.Command("sh", "-c", "exit 1").Run())
		}
		return []byte(out), nil
	}
}

func newTable(t *testing.T, sys system, run Runner) *Table {
	t.Helper()
	return &Table{socket: testSocket, sys: sys, run: run, grace: 200 * time.Millisecond, logf: t.Logf}
}

func TestParseProcArgs(t *testing.T) {
	tests := []struct {
		name      string
		buf       string
		argv, env []string
	}{
		{"arguments then environment", "\x02\x00\x00\x00/usr/bin/python3\x00\x00\x00python3\x00-m\x00A=1\x00B=2\x00\x00\x00", []string{"python3", "-m"}, []string{"A=1", "B=2"}},
		{"an empty argument", "\x02\x00\x00\x00/bin/sh\x00sh\x00\x00A=1\x00", []string{"sh", ""}, []string{"A=1"}},
		{"what follows the environment", "\x01\x00\x00\x00/bin/sh\x00sh\x00A=1\x00\x00ptr_munge=\x00", []string{"sh"}, []string{"A=1"}},
		{"no environment", "\x01\x00\x00\x00/bin/sh\x00sh\x00", []string{"sh"}, nil},
		{"too short", "\x01\x00", nil, nil},
		{"no path end", "\x01\x00\x00\x00/bin/sh", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argv, env := parseProcArgs([]byte(tt.buf))
			if !slices.Equal(argv, tt.argv) || !slices.Equal(env, tt.env) {
				t.Errorf("parseProcArgs = %q, %q; want %q, %q", argv, env, tt.argv, tt.env)
			}
		})
	}
}

func TestMarker(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		id   string
		ok   bool
	}{
		{"the thread", marked("p/ab"), "p/ab", true},
		{"a longer id is its own", marked("p/ab2"), "p/ab2", true},
		{"another app's socket", []string{sessionVar + "=p/ab", socketVar + "=/other/agentos.sock"}, "", false},
		{"no socket", []string{sessionVar + "=p/ab"}, "", false},
		{"no session", []string{socketVar + "=" + testSocket}, "", false},
		{"an empty session", []string{sessionVar + "=", socketVar + "=" + testSocket}, "", false},
		{"a name that only starts so", []string{sessionVar + "X=p/ab", socketVar + "=" + testSocket}, "", false},
		{"the first entry counts", []string{sessionVar + "=p/ab", sessionVar + "=p/cd", socketVar + "=" + testSocket}, "p/ab", true},
	}
	table := &Table{socket: testSocket}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if id, ok := table.marker(tt.env); id != tt.id || ok != tt.ok {
				t.Errorf("marker(%q) = %q, %v; want %q, %v", tt.env, id, ok, tt.id, tt.ok)
			}
		})
	}
}

func TestParseLsof(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want map[int][]int
	}{
		{"both families once", "p10\nf4\nn*:18080\nf5\nn[::1]:18080\np11\nf3\nn127.0.0.1:18090\n", map[int][]int{10: {18080}, 11: {18090}}},
		{"ports sorted", "p10\nn*:18090\nn*:18080\n", map[int][]int{10: {18080, 18090}}},
		{"nothing", "", map[int][]int{}},
		{"an address with no port", "p10\nnsomething\n", map[int][]int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLsof([]byte(tt.out))
			if len(got) != len(tt.want) {
				t.Fatalf("parseLsof = %v, want %v", got, tt.want)
			}
			for pid, ports := range tt.want {
				if !slices.Equal(got[pid], ports) {
					t.Errorf("parseLsof = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// thread is a running thread: its host 100 and Claude 101 under it, each in a group of its own.
var thread = []fakeProc{
	{process: process{pid: 100, ppid: 1, pgid: 100, start: 1}, argv: []string{"/bin/agentos", "host"}, env: marked("p/ab")},
	{process: process{pid: 101, ppid: 100, pgid: 101, start: 1}, argv: []string{"claude", "-p"}, env: marked("p/ab")},
}

func TestServers(t *testing.T) {
	procs := append(slices.Clone(thread),
		fakeProc{process: process{pid: 200, ppid: 1, pgid: 200, start: 5}, argv: []string{"/usr/bin/python3", "-m", "http.server", "18080"}, env: marked("p/ab")},
		fakeProc{process: process{pid: 201, ppid: 1, pgid: 201, start: 5}, argv: []string{"node", "b"}, env: marked("p/ab2")},
		fakeProc{process: process{pid: 202, ppid: 1, pgid: 202, start: 5}, argv: []string{"node", "c"}, env: []string{sessionVar + "=p/ab", socketVar + "=/other.sock"}},
		fakeProc{process: process{pid: 300, ppid: 1, pgid: 300, start: 5}, argv: []string{"node", "mine"}, env: []string{"HOME=/h"}},
	)
	lsof := "p101\nn*:18001\np200\nn*:18080\np201\nn*:18081\np202\nn*:18082\np300\nn*:18083\np100\nn*:18084\n"
	tests := []struct {
		name    string
		threads map[string]int
		want    []int // pids
	}{
		{"the thread's server, not its host or Claude", map[string]int{"p/ab": 100}, []int{200}},
		{"an exact id", map[string]int{"p/ab2": 0}, []int{201}},
		{"with no host, every listener with the marker", map[string]int{"p/ab": 0}, []int{101, 200, 100}},
		{"a thread asked about with no server", map[string]int{"p/cd": 0}, nil},
		{"no thread", map[string]int{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newTable(t, newFake(procs...), lsofSays(lsof)).Servers(context.Background(), tt.threads)
			if err != nil {
				t.Fatal(err)
			}
			var pids []int
			for _, s := range got {
				pids = append(pids, s.PID)
			}
			if !slices.Equal(pids, tt.want) {
				t.Errorf("Servers = %v, want pids %v", got, tt.want)
			}
		})
	}
}

func TestServersShowsTheCommand(t *testing.T) {
	sys := newFake(fakeProc{process: process{pid: 200, ppid: 1, pgid: 200, start: 5}, argv: []string{"/usr/bin/python3", "-m", "http.server", "18080"}, env: marked("p/ab")})
	got, err := newTable(t, sys, lsofSays("p200\nn*:18080\nn[::]:18080\n")).Servers(context.Background(), map[string]int{"p/ab": 0})
	if err != nil {
		t.Fatal(err)
	}
	want := Server{Thread: "p/ab", PID: 200, Ports: []int{18080}, Command: "python3 -m http.server 18080", start: 5}
	if len(got) != 1 || got[0].Thread != want.Thread || got[0].Command != want.Command || !slices.Equal(got[0].Ports, want.Ports) {
		t.Errorf("Servers = %+v, want %+v", got, want)
	}
}

func TestServersWhenNothingListens(t *testing.T) {
	got, err := newTable(t, newFake(thread...), lsofSays("")).Servers(context.Background(), map[string]int{"p/ab": 0})
	if err != nil || len(got) != 0 {
		t.Errorf("Servers = %v, %v; want none and no error", got, err)
	}
}

func TestServersWhenLsofFails(t *testing.T) {
	run := func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("lsof: not found") }
	if _, err := newTable(t, newFake(thread...), run).Servers(context.Background(), map[string]int{"p/ab": 0}); err == nil {
		t.Error("Servers with lsof failing gave no error")
	}
}

func TestStop(t *testing.T) {
	server := fakeProc{process: process{pid: 200, ppid: 1, pgid: 200, start: 5}, argv: []string{"node", "server"}, env: marked("p/ab")}
	child := fakeProc{process: process{pid: 201, ppid: 200, pgid: 200, start: 5}, argv: []string{"node", "worker"}, env: marked("p/ab")}
	tests := []struct {
		name    string
		procs   []fakeProc
		agent   int
		target  Server
		want    []signalled
		alive   []int
		stopped []int
	}{
		{
			name:    "a group of the thread's alone, whole",
			procs:   []fakeProc{server, child},
			target:  Server{Thread: "p/ab", PID: 200, start: 5},
			want:    []signalled{{-200, syscall.SIGTERM}},
			stopped: []int{200, 201},
		},
		{
			name:    "a group with a process of the owner's, the server alone",
			procs:   []fakeProc{server, {process: process{pid: 201, ppid: 200, pgid: 200, start: 5}, env: []string{"HOME=/h"}}},
			target:  Server{Thread: "p/ab", PID: 200, start: 5},
			want:    []signalled{{200, syscall.SIGTERM}},
			alive:   []int{201},
			stopped: []int{200},
		},
		{
			name:    "a group with another thread's process, the server alone",
			procs:   []fakeProc{server, {process: process{pid: 201, ppid: 200, pgid: 200, start: 5}, env: marked("p/ab2")}},
			target:  Server{Thread: "p/ab", PID: 200, start: 5},
			want:    []signalled{{200, syscall.SIGTERM}},
			alive:   []int{201},
			stopped: []int{200},
		},
		{
			name:    "Claude's group, the server alone",
			procs:   append(slices.Clone(thread), fakeProc{process: process{pid: 200, ppid: 101, pgid: 101, start: 5}, env: marked("p/ab")}),
			target:  Server{Thread: "p/ab", PID: 200, start: 5, agent: 100},
			want:    []signalled{{200, syscall.SIGTERM}},
			alive:   []int{100, 101},
			stopped: []int{200},
		},
		{
			name:   "a pid with another start time, nothing",
			procs:  []fakeProc{server},
			target: Server{Thread: "p/ab", PID: 200, start: 4},
			alive:  []int{200},
		},
		{
			name:   "a pid without the marker now, nothing",
			procs:  []fakeProc{{process: server.process, env: []string{"HOME=/h"}}},
			target: Server{Thread: "p/ab", PID: 200, start: 5},
			alive:  []int{200},
		},
		{
			name:   "a pid of another thread's now, nothing",
			procs:  []fakeProc{{process: server.process, env: marked("p/ab2")}},
			target: Server{Thread: "p/ab", PID: 200, start: 5},
			alive:  []int{200},
		},
		{
			name:   "a pid that is gone, nothing",
			target: Server{Thread: "p/ab", PID: 200, start: 5},
		},
		{
			name:    "one that ignores SIGTERM, SIGKILL after the grace",
			procs:   []fakeProc{{process: server.process, env: server.env, stubborn: true}},
			target:  Server{Thread: "p/ab", PID: 200, start: 5},
			want:    []signalled{{-200, syscall.SIGTERM}, {200, syscall.SIGKILL}},
			stopped: []int{200},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys := newFake(tt.procs...)
			newTable(t, sys, lsofSays("")).Stop(context.Background(), []Server{tt.target})
			if got := sys.signals(); !slices.Equal(got, tt.want) {
				t.Errorf("signals = %v, want %v", got, tt.want)
			}
			for _, pid := range tt.alive {
				if !sys.alive(pid) {
					t.Errorf("process %d was stopped", pid)
				}
			}
			for _, pid := range tt.stopped {
				if sys.alive(pid) {
					t.Errorf("process %d still runs", pid)
				}
			}
		})
	}
}

func TestStopSignalsAGroupOnce(t *testing.T) {
	sys := newFake(
		fakeProc{process: process{pid: 200, ppid: 1, pgid: 200, start: 5}, env: marked("p/ab")},
		fakeProc{process: process{pid: 201, ppid: 200, pgid: 200, start: 5}, env: marked("p/ab")},
	)
	newTable(t, sys, lsofSays("")).Stop(context.Background(), []Server{{Thread: "p/ab", PID: 200, start: 5}, {Thread: "p/ab", PID: 201, start: 5}})
	if got, want := sys.signals(), []signalled{{-200, syscall.SIGTERM}}; !slices.Equal(got, want) {
		t.Errorf("signals = %v, want %v", got, want)
	}
}

func TestStopLogsEachServer(t *testing.T) {
	var lines []string
	sys := newFake(fakeProc{process: process{pid: 200, ppid: 1, pgid: 200, start: 5}, env: marked("p/ab")})
	table := newTable(t, sys, lsofSays(""))
	table.logf = func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	table.Stop(context.Background(), []Server{{Thread: "p/ab", PID: 200, Ports: []int{18080, 18081}, Command: "node server", start: 5}})
	if len(lines) != 1 || !strings.Contains(lines[0], "node server (pid 200, ports 18080, 18081) of thread p/ab") {
		t.Errorf("logged %q, want one line naming the server", lines)
	}
}
