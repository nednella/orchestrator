package threads

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/nednella/orchestrator/internal/procs"
)

// fakeTree is a process list of pid: parent pairs, each process started at 5 µs, and what each process uses.
func fakeTree(parents map[int]int, sizes map[int]int64) processes {
	var list []procs.Process
	for pid, ppid := range parents {
		list = append(list, procs.Process{PID: pid, PPID: ppid, Start: 5})
	}
	return newProcesses(list, func(pid int) int64 { return sizes[pid] })
}

func TestHeldSumsAProcessAndEveryProcessUnderIt(t *testing.T) {
	t.Parallel()
	tree := fakeTree(map[int]int{1: 0, 100: 1, 101: 100, 102: 101, 200: 1}, map[int]int64{1: 10000, 100: 1000, 101: 2000, 102: 4000, 200: 8000})
	for pid, want := range map[int]int64{100: 7000, 101: 6000, 200: 8000, 999: 0, 0: 0} {
		if got := tree.held(pid); got != want {
			t.Errorf("held(%d) = %d, want %d", pid, got, want)
		}
	}
}

func TestTheAppHoldsNoBrowserItTookOver(t *testing.T) {
	t.Parallel()
	// The app, 10, started the browser 11 and its helper 12; the browser 20 is one it took over, under launchd.
	tree := fakeTree(map[int]int{1: 0, 10: 1, 11: 10, 12: 11, 13: 10, 20: 1, 21: 20}, map[int]int64{10: 100, 11: 1000, 12: 2000, 13: 10, 20: 4000, 21: 8000})
	if app, browsers := tree.appAndBrowsers(10, []int{11, 20}); app != 110 || browsers != 15000 {
		t.Errorf("app %d, browsers %d; want 110 and 15000", app, browsers)
	}
	if app, browsers := tree.appAndBrowsers(10, []int{20}); app != 3110 || browsers != 12000 {
		t.Errorf("with only the browser taken over: app %d, browsers %d; want 3110 and 12000", app, browsers)
	}
}

func TestAThreadHoldsWhatItStartedOutsideItsPane(t *testing.T) {
	t.Parallel()
	// The app 10 with its browser 11; the tmux server 20 with the panes 21 (thread a) and 22 (thread b), and 23 under
	// a. Outside every tree: 30 carries a's marker, with 31 under it, which carries none; 40 is a's agent-browser
	// daemon, which carries no marker, with 41 under it; 50 carries the marker of a thread that does not run; 60 is no
	// thread's. 24, under a's pane, carries b's marker too, and 12, under the app, a's.
	tree := fakeTree(
		map[int]int{1: 0, 10: 1, 11: 10, 12: 10, 20: 1, 21: 20, 22: 20, 23: 21, 24: 21, 30: 1, 31: 30, 40: 1, 41: 40, 50: 1, 60: 1},
		map[int]int64{10: 1, 11: 2, 12: 4, 20: 8, 21: 16, 22: 32, 23: 64, 24: 128, 30: 256, 31: 512, 40: 1024, 41: 2048, 50: 4096, 60: 8192},
	)
	marks := map[int]string{12: "p/a", 24: "p/b", 30: "p/a", 50: "p/gone"}
	var asked []int
	held, outside := tree.threads(
		map[string]int{"p/a": 21, "p/b": 22},
		map[string]int{"p/a": 40},
		[]int{10, 20, 11},
		func(list []procs.Process) map[int]string {
			found := map[int]string{}
			for _, p := range list {
				asked = append(asked, p.PID)
				if id, ok := marks[p.PID]; ok {
					found[p.PID] = id
				}
			}
			return found
		},
	)
	if want := map[string]int64{"p/a": 16 + 64 + 128 + 256 + 512 + 1024 + 2048, "p/b": 32}; !maps.Equal(held, want) {
		t.Errorf("held = %v, want %v", held, want)
	}
	if want := int64(256 + 512 + 1024 + 2048); outside != want {
		t.Errorf("outside = %d, want %d", outside, want)
	}
	slices.Sort(asked)
	if want := []int{1, 30, 31, 40, 41, 50, 60}; !slices.Equal(asked, want) {
		t.Errorf("marks were read for %v, want only the processes outside every tree %v", asked, want)
	}
}

func TestADaemonUnderAPaneCountsOnce(t *testing.T) {
	t.Parallel()
	tree := fakeTree(map[int]int{1: 0, 20: 1, 21: 20, 22: 21}, map[int]int64{20: 1, 21: 2, 22: 4})
	held, outside := tree.threads(map[string]int{"p/a": 21}, map[string]int{"p/a": 22}, []int{20}, func([]procs.Process) map[int]string { return nil })
	if held["p/a"] != 6 || outside != 0 {
		t.Errorf("held %v, outside %d; want 6 and 0", held, outside)
	}
}

func TestStartedByTellsAReusedPid(t *testing.T) {
	t.Parallel()
	tree := newProcesses([]procs.Process{{PID: 40, PPID: 1, Start: 2_000_000}}, func(int) int64 { return 0 })
	for _, tt := range []struct {
		pid     int
		written time.Time
		want    bool
	}{
		{40, time.UnixMicro(2_000_000), true},
		{40, time.UnixMicro(3_000_000), true},
		{40, time.UnixMicro(1_000_000), false},
		{41, time.UnixMicro(3_000_000), false},
		{0, time.UnixMicro(3_000_000), false},
	} {
		if got := tree.startedBy(tt.pid, tt.written); got != tt.want {
			t.Errorf("startedBy(%d, %v) = %v, want %v", tt.pid, tt.written, got, tt.want)
		}
	}
}

func TestRoundUpKeepsAnyUseVisible(t *testing.T) {
	t.Parallel()
	for n, want := range map[int64]int64{0: 0, 1: memoryStep, memoryStep: memoryStep, memoryStep + 1: 2 * memoryStep} {
		if got := roundUp(n); got != want {
			t.Errorf("roundUp(%d) = %d, want %d", n, got, want)
		}
	}
}
