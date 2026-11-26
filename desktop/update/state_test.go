package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeGitHub answers curl for the releases, without the network, and unpacks a zip as ditto would into a bundle
// whose binary is the zip's bytes; neither curl nor ditto runs.
type fakeGitHub struct {
	mu      sync.Mutex
	tag     string        // the latest release's tag; "" fails the check
	badSum  bool          // the sha256 file does not match the zip
	hold    chan struct{} // when set, the ask for the latest release waits until it closes
	asked   chan struct{} // gets a value at each ask for the latest release, when set
	written func(i int)   // called after each tenth of a zip is written, when set
}

func (f *fakeGitHub) set(change func(f *fakeGitHub)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change(f)
}

func body(version string) []byte { return []byte(strings.Repeat("agentos "+version+"\n", 100)) }

func (f *fakeGitHub) run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	tag, badSum, hold, asked, written := f.tag, f.badSum, f.hold, f.asked, f.written
	f.mu.Unlock()
	if name == "ditto" {
		data, err := os.ReadFile(args[2])
		if err != nil {
			return nil, err
		}
		bin := filepath.Join(args[3], "agentos.app", "Contents", "MacOS")
		if err := os.MkdirAll(bin, 0o700); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(filepath.Join(bin, "agentos"), data, 0o700)
	}
	if name != "curl" {
		return nil, fmt.Errorf("unexpected %s", name)
	}
	url := args[len(args)-1]
	if strings.HasSuffix(url, "/releases/latest") {
		if asked != nil {
			asked <- struct{}{}
		}
		if hold != nil {
			<-hold
		}
		if tag == "" {
			return nil, errors.New("curl: (6) Could not resolve host: github.com")
		}
		return []byte("https://github.com/nednella/agentos/releases/tag/" + tag), nil
	}
	_, rest, ok := strings.Cut(url, "/releases/download/v")
	if !ok {
		return nil, fmt.Errorf("curl: (22) 404 for %s", url)
	}
	version, file, _ := strings.Cut(rest, "/")
	data := body(version)
	switch {
	case slices.Contains(args, "-fsSI"):
		return nil, nil
	case slices.Contains(args, "-fsSIL"):
		return []byte(fmt.Sprintf("HTTP/2 200\r\ncontent-length: %d\r\n\r\n", len(data))), nil
	case slices.Contains(args, "-o"):
		out, err := os.Create(args[slices.Index(args, "-o")+1])
		if err != nil {
			return nil, err
		}
		defer out.Close()
		for i := 1; i <= 10; i++ {
			if _, err := out.Write(data[(i-1)*len(data)/10 : i*len(data)/10]); err != nil {
				return nil, err
			}
			if written != nil {
				written(i)
			}
		}
		return nil, nil
	case strings.HasSuffix(file, ".sha256"):
		h := sha256.Sum256(data)
		if badSum {
			h = [32]byte{}
		}
		return []byte(hex.EncodeToString(h[:]) + "  agentos-darwin-arm64.zip\n"), nil
	}
	return nil, fmt.Errorf("curl: (22) 404 for %s", url)
}

type rig struct {
	u          *Updater
	gh         *fakeGitHub
	bundle     string
	mu         sync.Mutex
	states     []State
	relaunched []string
	quits      atomic.Int32
}

// newRig makes an updater on version 1.0.0 whose latest release is tag, for an app installed in a temp folder.
func newRig(t *testing.T, tag string) *rig {
	t.Helper()
	r := &rig{gh: &fakeGitHub{tag: tag}, bundle: filepath.Join(t.TempDir(), "Applications", "agentos.app")}
	if err := os.MkdirAll(filepath.Join(r.bundle, "Contents", "MacOS"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.bundle, "Contents", "MacOS", "agentos"), []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	emit := func(event string, payload any) {
		if event == "update" {
			r.mu.Lock()
			r.states = append(r.states, payload.(State))
			r.mu.Unlock()
		}
	}
	relaunch := func(bundle string) error {
		r.mu.Lock()
		r.relaunched = append(r.relaunched, bundle)
		r.mu.Unlock()
		return nil
	}
	r.u = New(r.gh.run, t.TempDir(), r.bundle, emit, relaunch, func() { r.quits.Add(1) })
	r.u.SetVersion("1.0.0")
	return r
}

func (r *rig) emitted() []State {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.states)
}

func (r *rig) binary(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.bundle, "Contents", "MacOS", "agentos"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func (r *rig) downloaded(t *testing.T) {
	t.Helper()
	if err := r.u.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.u.Download(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func TestFirstCheckAfterFifteenSeconds(t *testing.T) {
	r := newRig(t, "v2.0.0")
	waits := make(chan time.Duration)
	fire := make(chan time.Time)
	r.u.after = func(d time.Duration) <-chan time.Time {
		waits <- d
		return fire
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.u.Loop(ctx)
	if d := <-waits; d != 15*time.Second {
		t.Fatalf("the first check waits %v", d)
	}
	if got := r.u.State().Status; got != idle {
		t.Errorf("before the first check the state is %s", got)
	}
	fire <- time.Now()
	if d := <-waits; d != 4*time.Hour {
		t.Fatalf("the next check waits %v", d)
	}
	if got := r.u.State(); got != (State{Status: available, Version: "2.0.0"}) {
		t.Errorf("after the first check: %+v", got)
	}
	r.gh.set(func(f *fakeGitHub) { f.tag = "" })
	fire <- time.Now()
	if d := <-waits; d != 4*time.Hour {
		t.Errorf("after a failed check the next waits %v, with no retry sooner", d)
	}
}

func TestStatesOfADownload(t *testing.T) {
	r := newRig(t, "v2.0.0")
	// Each tenth of the zip waits until its step is told, so every step shows however the reads fall.
	r.gh.written = func(i int) {
		if i < 10 {
			eventually(t, fmt.Sprintf("the %d%% step", i*10), func() bool { return r.u.State().Percent >= i*10 })
		}
	}
	if got := r.u.State(); got != (State{Status: idle}) {
		t.Errorf("at the start: %+v", got)
	}
	r.downloaded(t)
	want := []State{{Status: checking}, {Status: available, Version: "2.0.0"}, {Status: downloading, Version: "2.0.0"}}
	for p := 10; p <= 100; p += 10 {
		want = append(want, State{Status: downloading, Version: "2.0.0", Percent: p})
	}
	want = append(want, State{Status: downloaded, Version: "2.0.0", Percent: 100})
	if got := r.emitted(); !slices.Equal(got, want) {
		t.Errorf("states\n got %+v\nwant %+v", got, want)
	}
	if data, err := os.ReadFile(filepath.Join(r.u.downloads(), "agentos-darwin-arm64.zip")); err != nil || string(data) != string(body("2.0.0")) {
		t.Errorf("the zip waiting in the state folder: %d bytes, %v", len(data), err)
	}
}

func TestAFailedDownloadSaysHowFarItGot(t *testing.T) {
	r := newRig(t, "v2.0.0")
	if err := r.u.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	broken := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if slices.Contains(args, "-o") {
			return nil, errors.New("curl: (18) transfer closed with bytes remaining")
		}
		return (&fakeGitHub{tag: "v2.0.0"}).run(ctx, name, args...)
	}
	r.u.run = broken
	err := r.u.Download(context.Background())
	if err == nil {
		t.Fatal("a broken download succeeded")
	}
	if got := r.u.State(); got.Status != failed || got.Version != "2.0.0" || got.Percent >= 100 || got.Error != err.Error() {
		t.Errorf("after a broken download: %+v", got)
	}
	if _, err := os.Stat(r.u.downloads()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the broken download stayed: %v", err)
	}
	r.u.run = r.gh.run
	if err := r.u.Download(context.Background()); err != nil || r.u.State().Status != downloaded {
		t.Errorf("trying again: %+v, %v", r.u.State(), err)
	}
}

func TestDownloadVerifiesTheSum(t *testing.T) {
	r := newRig(t, "v2.0.0")
	r.downloaded(t)
	r.gh.set(func(f *fakeGitHub) { f.badSum = true })
	err := r.u.Install(context.Background())
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("a wrong sum installed anyway: %v", err)
	}
	if got := r.binary(t); got != "old" {
		t.Errorf("the bundle was swapped: %q", got)
	}
	if got := r.u.State(); got.Status != failed || got.Version != "2.0.0" || got.Percent != 100 {
		t.Errorf("after the failed install: %+v", got)
	}
	if _, err := os.Stat(r.u.downloads()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the zip that failed its sum stayed: %v", err)
	}
	if len(r.relaunched) != 0 {
		t.Errorf("relaunched %v", r.relaunched)
	}
}

func TestInstallChecksAgainFirst(t *testing.T) {
	r := newRig(t, "v2.0.0")
	r.downloaded(t)
	r.gh.set(func(f *fakeGitHub) { f.tag = "v2.1.0" })
	if err := r.u.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := r.binary(t); got != string(body("2.1.0")) {
		t.Errorf("installed %q", got[:min(len(got), 20)])
	}
	if got := r.emitted(); !slices.Contains(got, State{Status: downloading, Version: "2.1.0"}) {
		t.Errorf("2.1.0 was not downloaded: %+v", got)
	}
	if !slices.Equal(r.relaunched, []string{r.bundle}) {
		t.Errorf("relaunched %v", r.relaunched)
	}
	eventually(t, "the quit", func() bool { return r.quits.Load() == 1 })
	if _, err := os.Stat(r.u.downloads()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the installed zip stayed: %v", err)
	}
}

func TestOneActionAtATime(t *testing.T) {
	r := newRig(t, "v2.0.0")
	hold, asked := make(chan struct{}), make(chan struct{}, 1)
	r.gh.set(func(f *fakeGitHub) { f.hold, f.asked = hold, asked })
	checked := make(chan error)
	go func() { checked <- r.u.Check(context.Background()) }()
	<-asked
	for name, action := range map[string]func(context.Context) error{"check": r.u.Check, "download": r.u.Download} {
		if err := action(context.Background()); err == nil || err.Error() != "An update action is running" {
			t.Errorf("a %s during a check: %v", name, err)
		}
	}
	r.gh.set(func(f *fakeGitHub) { f.hold, f.asked = nil, nil })
	close(hold)
	if err := <-checked; err != nil {
		t.Fatal(err)
	}

	writing, release := make(chan struct{}), make(chan struct{})
	r.gh.set(func(f *fakeGitHub) {
		f.written = func(i int) {
			if i == 1 {
				close(writing)
				<-release
			}
		}
	})
	fetched := make(chan error)
	go func() { fetched <- r.u.Download(context.Background()) }()
	<-writing
	for name, action := range map[string]func(context.Context) error{"check": r.u.Check, "download": r.u.Download, "install": r.u.Install} {
		if err := action(context.Background()); err == nil || err.Error() != "An update action is running" {
			t.Errorf("a %s during a download: %v", name, err)
		}
	}
	close(release)
	if err := <-fetched; err != nil {
		t.Fatal(err)
	}
	if err := r.u.Check(context.Background()); err != nil {
		t.Errorf("a check once nothing runs: %v", err)
	}
}

func TestInstallWaitsForARunningCheck(t *testing.T) {
	t.Run("the check ends", func(t *testing.T) {
		r := newRig(t, "v2.0.0")
		hold, asked := make(chan struct{}), make(chan struct{}, 2)
		r.gh.set(func(f *fakeGitHub) { f.hold, f.asked = hold, asked })
		checked := make(chan error)
		go func() { checked <- r.u.Check(context.Background()) }()
		<-asked
		installed := make(chan error)
		go func() { installed <- r.u.Install(context.Background()) }()
		select {
		case err := <-installed:
			t.Fatalf("the install did not wait for the check: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		close(hold)
		if err := <-checked; err != nil {
			t.Fatal(err)
		}
		if err := <-installed; err != nil {
			t.Fatal(err)
		}
		if got := r.binary(t); got != string(body("2.0.0")) {
			t.Errorf("installed %q", got[:min(len(got), 20)])
		}
	})
	t.Run("the check does not end", func(t *testing.T) {
		r := newRig(t, "v2.0.0")
		r.u.checkWait = 100 * time.Millisecond
		hold, asked := make(chan struct{}), make(chan struct{}, 1)
		r.gh.set(func(f *fakeGitHub) { f.hold, f.asked = hold, asked })
		checked := make(chan error)
		go func() { checked <- r.u.Check(context.Background()) }()
		<-asked
		if err := r.u.Install(context.Background()); err == nil || err.Error() != "The update check did not finish" {
			t.Errorf("err = %v", err)
		}
		close(hold)
		<-checked
		if got := r.binary(t); got != "old" {
			t.Errorf("installed %q", got)
		}
	})
}

func TestFailedCheckKeepsTheDownload(t *testing.T) {
	r := newRig(t, "v2.0.0")
	r.downloaded(t)
	r.gh.set(func(f *fakeGitHub) { f.tag = "" })
	err := r.u.Check(context.Background())
	if err == nil {
		t.Fatal("the check did not fail")
	}
	if got := r.u.State(); got != (State{Status: downloaded, Version: "2.0.0", Percent: 100, Error: err.Error()}) {
		t.Errorf("after a failed check: %+v", got)
	}
	if err := r.u.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := r.binary(t); got != string(body("2.0.0")) {
		t.Errorf("the waiting download was not installed: %q", got[:min(len(got), 20)])
	}
}

func TestAFailedCheckWithNothingDownloadedInstallsNothing(t *testing.T) {
	r := newRig(t, "")
	if err := r.u.Install(context.Background()); err == nil {
		t.Error("an install with no release succeeded")
	}
	if got := r.u.State(); got.Status != idle || got.Error == "" || r.binary(t) != "old" {
		t.Errorf("state %+v, binary %q", got, r.binary(t))
	}
}
