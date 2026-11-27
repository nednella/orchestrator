package threads

import "testing"

func TestHeldSumsAProcessAndEveryProcessUnderIt(t *testing.T) {
	t.Parallel()
	sizes := map[int]int64{1: 10000, 100: 1000, 101: 2000, 102: 4000, 200: 8000}
	procs := readProcesses([]byte(`    1     0
  100     1
  101   100
  102   101
  200     1
  bad line
`), func(pid int) int64 { return sizes[pid] })
	for pid, want := range map[int]int64{100: 7000, 101: 6000, 200: 8000, 999: 0, 0: 0} {
		if got := procs.held(pid); got != want {
			t.Errorf("held(%d) = %d, want %d", pid, got, want)
		}
	}
}

func TestTheAppHoldsNoBrowserItTookOver(t *testing.T) {
	t.Parallel()
	// The app, 10, started the browser 11 and its helper 12; the browser 20 is one it took over, under launchd.
	sizes := map[int]int64{10: 100, 11: 1000, 12: 2000, 13: 10, 20: 4000, 21: 8000}
	procs := readProcesses([]byte(`1 0
10 1
11 10
12 11
13 10
20 1
21 20
`), func(pid int) int64 { return sizes[pid] })
	if app, browsers := procs.appAndBrowsers(10, []int{11, 20}); app != 110 || browsers != 15000 {
		t.Errorf("app %d, browsers %d; want 110 and 15000", app, browsers)
	}
	if app, browsers := procs.appAndBrowsers(10, []int{20}); app != 3110 || browsers != 12000 {
		t.Errorf("with only the browser taken over: app %d, browsers %d; want 3110 and 12000", app, browsers)
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
