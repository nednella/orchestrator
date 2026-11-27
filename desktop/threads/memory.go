package threads

import (
	"context"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nednella/orchestrator/internal/procs"
	"github.com/nednella/orchestrator/internal/util"
)

const (
	sampleEvery = 10 * time.Second
	// memoryStep is what figures are rounded up to, so a thread list that only moved by a few
	// megabytes is not sent again.
	memoryStep = (1 << 30) / 10
)

// Memory is what orchestrator and its agents use, in bytes, rounded up to memoryStep. It is each process's
// footprint summed over the processes, so memory two processes share is not counted: an estimate.
type Memory struct {
	App      int64 `json:"app"`      // the app and the commands it runs, without its browsers
	Browsers int64 `json:"browsers"` // the projects' browsers and their helper processes
	Threads  int64 `json:"threads"`  // the tmux server and everything under it, every project's agents and shells, and what the agents started outside it
	Total    int64 `json:"total"`
	Machine  int64 `json:"machine"` // the Mac's memory, 0 when not known
}

// Memory is the last sample.
func (s *Threads) Memory() Memory {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memory
}

// SampleMemory reads what every process uses, and tells the front end when a rounded figure moved.
// The poll does it every sampleEvery.
func (s *Threads) SampleMemory() {
	ctx, cancel := context.WithTimeout(s.context(), tmuxTimeout)
	defer cancel()
	list, err := s.procs.Processes()
	if err != nil {
		s.warn.Report("memory", util.FirstLine(err.Error()))
		return
	}
	tree := newProcesses(list, footprint)
	s.mu.Lock()
	server, machine := s.serverPID, s.memory.Machine
	running := len(s.info) > 0 || len(s.shells) > 0 // tmux ends its server when its last session goes
	panes := map[string]int{}
	for _, in := range s.info {
		if in.PID > 0 {
			panes[in.Name.String()] = in.PID
		}
	}
	s.mu.Unlock()
	if !tree.has(server) {
		server = 0
		if running {
			if server, err = s.tmux.ServerPID(ctx); err != nil {
				s.warn.Report("memory", util.FirstLine(err.Error()))
				return
			}
		}
	}
	s.warn.Clear("memory")
	if machine == 0 {
		if out, err := s.run(ctx, "", "sysctl", "-n", "hw.memsize"); err == nil {
			machine, _ = strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		}
	}
	self, browserPIDs := os.Getpid(), s.browsers.PIDs()
	app, browsers := tree.appAndBrowsers(self, browserPIDs)
	threads, outside := tree.threads(panes, append([]int{self, server}, browserPIDs...), s.procs.Marks)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.serverPID = server
	held := map[string]int64{}
	for id, n := range threads {
		held[id] = roundUp(n)
	}
	m := Memory{
		App:      roundUp(app),
		Browsers: roundUp(browsers),
		Threads:  roundUp(tree.held(server) + outside),
		Machine:  machine,
	}
	m.Total = m.App + m.Browsers + m.Threads
	if m != s.memory {
		s.memory = m
		s.emit("memory", m)
	}
	if !maps.Equal(held, s.held) {
		s.held = held
		s.sync()
	}
}

// processes is the process tree of one listing, and how much memory a process uses, asked once per process.
type processes struct {
	list     []procs.Process
	parent   map[int]int
	children map[int][]int
	size     func(pid int) int64
	sizes    map[int]int64
}

func newProcesses(list []procs.Process, size func(pid int) int64) processes {
	p := processes{list: list, parent: map[int]int{}, children: map[int][]int{}, size: size, sizes: map[int]int64{}}
	for _, q := range list {
		p.parent[q.PID] = q.PPID
		if q.PID != q.PPID {
			p.children[q.PPID] = append(p.children[q.PPID], q.PID)
		}
	}
	return p
}

// has says whether pid is running.
func (p processes) has(pid int) bool {
	_, ok := p.parent[pid]
	return ok
}

// appAndBrowsers is what the app, self, holds without the browsers it started, and what the browsers hold. A browser
// the app took over from an earlier run is not under it, so it is not taken off the app's figure.
func (p processes) appAndBrowsers(self int, browsers []int) (app, all int64) {
	app = p.held(self)
	for _, pid := range browsers {
		held := p.held(pid)
		all += held
		if p.under(pid, self) {
			app -= held
		}
	}
	return max(app, 0), all
}

// threads is what each live thread holds, by id: the processes under its pane (panes), and the processes outside every
// tree of roots that carry its marker (marks reads them) with the processes under them. A process counts once, for the
// first thread that has it. outside is what of that runs outside the trees of roots.
func (p processes) threads(panes map[string]int, roots []int, marks func([]procs.Process) map[int]string) (held map[string]int64, outside int64) {
	placed := map[int]bool{}
	for _, root := range roots {
		p.each(root, func(pid int) { placed[pid] = true })
	}
	owner := map[int]string{}
	for id, pane := range panes {
		p.each(pane, func(pid int) {
			if _, taken := owner[pid]; !taken {
				owner[pid] = id
			}
		})
	}
	claim := func(root int, id string) {
		p.each(root, func(pid int) {
			if _, taken := owner[pid]; !taken && !placed[pid] {
				owner[pid] = id
			}
		})
	}
	var rest []procs.Process
	for _, q := range p.list {
		if !placed[q.PID] {
			rest = append(rest, q)
		}
	}
	marked := marks(rest)
	for _, pid := range slices.Sorted(maps.Keys(marked)) {
		if _, live := panes[marked[pid]]; live {
			claim(pid, marked[pid])
		}
	}
	held = make(map[string]int64, len(panes))
	for id := range panes {
		held[id] = 0
	}
	for pid, id := range owner {
		n := p.sizeOf(pid)
		held[id] += n
		if !placed[pid] {
			outside += n
		}
	}
	return held, outside
}

// under says whether pid is root or runs under it.
func (p processes) under(pid, root int) bool {
	for pid != root {
		parent, ok := p.parent[pid]
		if !ok || parent == pid {
			return false
		}
		pid = parent
	}
	return true
}

// each calls f with pid and every process under it; with none when pid is not above 0.
func (p processes) each(pid int, f func(pid int)) {
	if pid <= 0 {
		return
	}
	f(pid)
	for _, child := range p.children[pid] {
		p.each(child, f)
	}
}

// held is what pid and every process under it use; 0 when pid is gone.
func (p processes) held(pid int) int64 {
	var total int64
	p.each(pid, func(pid int) { total += p.sizeOf(pid) })
	return total
}

func (p processes) sizeOf(pid int) int64 {
	n, ok := p.sizes[pid]
	if !ok {
		n = p.size(pid)
		p.sizes[pid] = n
	}
	return n
}

func roundUp(n int64) int64 { return (n + memoryStep - 1) / memoryStep * memoryStep }
