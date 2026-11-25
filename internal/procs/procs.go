// Package procs finds the servers a thread started. A thread's processes are found by the marker the app starts its
// agent with, which every process the agent starts inherits: the thread's id in AGENTOS_SESSION and this app's hook
// socket in AGENTOS_SOCKET. Nothing else names them: a process is never matched by its name, folder, command or port,
// so a server the owner started by hand, or another app's thread, is never one.
package procs

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	sessionVar = "AGENTOS_SESSION"
	socketVar  = "AGENTOS_SOCKET"

	// stopGrace is how long a server has to exit after SIGTERM before it gets SIGKILL.
	stopGrace = 5 * time.Second
	stopPoll  = 50 * time.Millisecond
	// commandWidth is how much of a server's command line it shows, and logWidth how much a log line names.
	commandWidth = 200
	logWidth     = 60
)

// Runner runs a command and returns its stdout.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// process is one process as the kernel lists it. A pid with its start time names one process: a pid is reused.
type process struct {
	pid, ppid, pgid int
	start           int64 // microseconds since the epoch
}

// system is what the table reads and signals; tests give a fake.
type system interface {
	// processes are the live processes, without zombies.
	processes() ([]process, error)
	// args are the arguments and the environment the process was started with; an error for another user's process.
	args(pid int) (argv, env []string, err error)
	signal(pid int, sig syscall.Signal) error
}

// Table finds and stops the servers of this app's threads.
type Table struct {
	socket string
	sys    system
	run    Runner
	grace  time.Duration
	logf   func(format string, args ...any)
}

// New is a table for the threads of the app whose hook socket is socket. lsof runs through run.
func New(socket string, run Runner) *Table {
	return &Table{socket: socket, sys: kernel{}, run: run, grace: stopGrace, logf: log.Printf}
}

// Server is a process of a thread's that listens on TCP ports.
type Server struct {
	Thread  string
	PID     int
	Ports   []int
	Command string // its command line, the program by its base name
	start   int64
	// agent is the thread's host when it runs: the host and the agent it runs are never servers, and a group that
	// holds either is never signalled whole.
	agent int
}

// snapshot is every process at one moment, and which of them belong to the threads asked about.
type snapshot struct {
	procs   map[int]process
	threads map[int]string // pid: the thread whose marker it carries
	argv    map[int][]string
}

// scan reads every process and the environment of each, and keeps the thread of those whose marker names a thread
// want says yes to.
func (t *Table) scan(want func(id string) bool) (snapshot, error) {
	list, err := t.sys.processes()
	if err != nil {
		return snapshot{}, fmt.Errorf("listing processes: %w", err)
	}
	snap := snapshot{procs: make(map[int]process, len(list)), threads: map[int]string{}, argv: map[int][]string{}}
	for _, p := range list {
		snap.procs[p.pid] = p
		argv, env, err := t.sys.args(p.pid)
		if err != nil {
			continue
		}
		if id, ok := t.marker(env); ok && want(id) {
			snap.threads[p.pid] = id
			snap.argv[p.pid] = argv
		}
	}
	return snap, nil
}

// marker is the thread the environment names, when it names one of this app's.
func (t *Table) marker(env []string) (string, bool) {
	id, ok := lookup(env, sessionVar)
	if !ok || id == "" {
		return "", false
	}
	if socket, ok := lookup(env, socketVar); !ok || socket != t.socket {
		return "", false
	}
	return id, true
}

// lookup is the value of the variable name in env, the first entry's as getenv reads it.
func lookup(env []string, name string) (string, bool) {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == name {
			return v, true
		}
	}
	return "", false
}

// isAgent says whether pid is the thread's host or the agent the host runs, its child.
func (snap snapshot) isAgent(pid, agent int) bool {
	if agent <= 0 {
		return false
	}
	return pid == agent || snap.procs[pid].ppid == agent
}

// Servers are the processes of the threads that listen on a TCP port, each thread's mapped to its host's pid, 0 when
// it has none; the host and the agent it runs are left out. They come by thread, then by lowest port.
func (t *Table) Servers(ctx context.Context, threads map[string]int) ([]Server, error) {
	snap, err := t.scan(func(id string) bool { _, ok := threads[id]; return ok })
	if err != nil {
		return nil, err
	}
	if len(snap.threads) == 0 {
		return nil, nil
	}
	ports, err := t.listening(ctx)
	if err != nil {
		return nil, err
	}
	var servers []Server
	for pid, list := range ports {
		id, ok := snap.threads[pid]
		if !ok || snap.isAgent(pid, threads[id]) || pid == os.Getpid() {
			continue
		}
		servers = append(servers, Server{Thread: id, PID: pid, Ports: list, Command: command(snap.argv[pid]), start: snap.procs[pid].start, agent: threads[id]})
	}
	slices.SortFunc(servers, func(a, b Server) int {
		return cmp.Or(cmp.Compare(a.Thread, b.Thread), cmp.Compare(a.Ports[0], b.Ports[0]))
	})
	return servers, nil
}

// listening are the TCP ports each process listens on, through lsof. It lists every listener and not only the
// thread's pids: lsof fails when a pid it is given has exited, as a thread's short-lived commands do, and the failure
// would drop what it found. lsof exits 1 when nothing listens.
func (t *Table) listening(ctx context.Context) (map[int][]int, error) {
	out, err := t.run(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pn")
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return nil, fmt.Errorf("listing the ports processes listen on: %w", err)
	}
	return parseLsof(out), nil
}

// parseLsof reads lsof's -F pn output: a p line names a process, and each n line after it an address it listens on.
func parseLsof(out []byte) map[int][]int {
	ports := map[int][]int{}
	pid := 0
	for line := range strings.Lines(string(out)) {
		line = strings.TrimRight(line, "\n")
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'n':
			at := strings.LastIndexByte(line, ':')
			port, err := strconv.Atoi(line[at+1:])
			if pid > 0 && at >= 0 && err == nil && port > 0 && !slices.Contains(ports[pid], port) {
				ports[pid] = append(ports[pid], port)
			}
		}
	}
	for _, list := range ports {
		slices.Sort(list)
	}
	return ports
}

// command is a process's command line to show on one line: its program by its base name, then its arguments.
func command(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	return clip(strings.Join(strings.Fields(strings.Join(append([]string{filepath.Base(argv[0])}, argv[1:]...), " ")), " "), commandWidth)
}

// clip is s cut to at most width runes, with an ellipsis where it was cut.
func clip(s string, width int) string {
	if r := []rune(s); len(r) > width {
		return string(r[:width-1]) + "…"
	}
	return s
}

// Stop stops each server. SIGTERM goes to its process group when every process in the group carries its thread's
// marker; else, when the group holds neither the thread's host nor its agent, to each process of the group that carries
// the marker, one by one, so a supervisor that would start the server again stops too; else to the server alone. Each
// server is checked just before: one that is gone, no longer carries the marker or has another start time, as when its
// pid was taken by a new process, is left alone. It logs one line per server it stops and returns once the signals are
// sent; the channel closes once each process signalled has exited, or after 5 s had SIGKILL.
func (t *Table) Stop(ctx context.Context, servers []Server) <-chan struct{} {
	done := make(chan struct{})
	threads := map[string]bool{}
	for _, s := range servers {
		threads[s.Thread] = true
	}
	if len(threads) == 0 {
		close(done)
		return done
	}
	snap, err := t.scan(func(id string) bool { return threads[id] })
	if err != nil {
		t.logf("agentos: stopping the servers of a thread: %v", err)
		close(done)
		return done
	}
	sent := map[int]process{} // each process signalled, by pid
	for _, s := range servers {
		p, ok := snap.procs[s.PID]
		if !ok || p.start != s.start || snap.threads[s.PID] != s.Thread || s.PID <= 1 || s.PID == os.Getpid() {
			continue
		}
		if _, done := sent[s.PID]; done {
			continue
		}
		targets, watched := snap.targets(p, s)
		for _, pid := range targets {
			if err := t.sys.signal(pid, syscall.SIGTERM); err != nil {
				t.logf("agentos: stopping process %d of thread %s: %v", pid, s.Thread, err)
			}
		}
		for _, q := range watched {
			sent[q.pid] = q
		}
		how := ""
		switch {
		case len(targets) == 1 && targets[0] < 0:
			how = fmt.Sprintf(", with its process group %d", p.pgid)
		case len(targets) > 1:
			how = fmt.Sprintf(", with %d more processes of its group", len(targets)-1)
		}
		t.logf("agentos: stopping %s (pid %d, ports %s) of thread %s%s", clip(s.Command, logWidth), s.PID, portList(s.Ports), s.Thread, how)
	}
	go func() {
		defer close(done)
		t.killLeft(ctx, sent, snap.threads)
	}()
	return done
}

// targets are what a stop of the server p sends SIGTERM to, a pid or a process group as -pgid, and the processes
// that reaches. A group that holds the thread's host or agent, or this process, gets the server alone.
func (snap snapshot) targets(p process, s Server) (targets []int, watched []process) {
	group := snap.group(p.pgid)
	if p.pgid <= 1 || p.pgid == syscall.Getpgrp() || slices.ContainsFunc(group, func(q process) bool { return snap.isAgent(q.pid, s.agent) }) {
		return []int{p.pid}, []process{p}
	}
	marked := slices.DeleteFunc(slices.Clone(group), func(q process) bool {
		return snap.threads[q.pid] != s.Thread || q.pid <= 1 || q.pid == os.Getpid()
	})
	if len(marked) == len(group) {
		return []int{-p.pgid}, group
	}
	for _, q := range marked {
		targets = append(targets, q.pid)
	}
	return targets, marked
}

// group are the processes of the process group pgid, by pid.
func (snap snapshot) group(pgid int) []process {
	var group []process
	for _, p := range snap.procs {
		if p.pgid == pgid {
			group = append(group, p)
		}
	}
	slices.SortFunc(group, func(a, b process) int { return a.pid - b.pid })
	return group
}

// killLeft waits for the signalled processes to exit, and after the grace sends SIGKILL to each that is still there,
// the same process and still its thread's.
func (t *Table) killLeft(ctx context.Context, sent map[int]process, threads map[int]string) {
	deadline := time.Now().Add(t.grace)
	for len(sent) > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(stopPoll):
		}
		list, err := t.sys.processes()
		if err != nil {
			t.logf("agentos: waiting for a thread's servers to stop: %v", err)
			return
		}
		live := make(map[int]process, len(list))
		for _, p := range list {
			live[p.pid] = p
		}
		for pid, p := range sent {
			if q, ok := live[pid]; !ok || q.start != p.start {
				delete(sent, pid)
			}
		}
		if len(sent) == 0 || time.Now().Before(deadline) {
			continue
		}
		for pid := range sent {
			if _, env, err := t.sys.args(pid); err == nil {
				if id, ok := t.marker(env); ok && id == threads[pid] {
					t.logf("agentos: process %d of thread %s did not stop in %s; killing it", pid, id, t.grace)
					_ = t.sys.signal(pid, syscall.SIGKILL)
				}
			}
		}
		return
	}
}

func portList(ports []int) string {
	list := make([]string, len(ports))
	for i, p := range ports {
		list[i] = strconv.Itoa(p)
	}
	return strings.Join(list, ", ")
}

// parseProcArgs reads kern.procargs2: the argument count as a 32-bit integer, the path the process was started from,
// NUL padding, then the arguments and the environment, each string ending in a NUL. What follows the environment, and
// a last string with no NUL to end it, are left out.
func parseProcArgs(buf []byte) (argv, env []string) {
	if len(buf) < 4 {
		return nil, nil
	}
	argc := int(binary.LittleEndian.Uint32(buf))
	rest := buf[4:]
	end := bytes.IndexByte(rest, 0)
	if end < 0 {
		return nil, nil
	}
	rest = bytes.TrimLeft(rest[end:], "\x00")
	for len(rest) > 0 {
		end := bytes.IndexByte(rest, 0)
		if end < 0 {
			break
		}
		s := string(rest[:end])
		rest = rest[end+1:]
		if len(argv) < argc {
			argv = append(argv, s)
			continue
		}
		if s == "" {
			break
		}
		env = append(env, s)
	}
	return argv, env
}
