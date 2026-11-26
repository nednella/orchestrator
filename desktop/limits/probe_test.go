package limits

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/nednella/agentos/internal/host"
)

// fakeClaude is a Starter that acts as Claude in stream-json: it answers a get_usage line with reply, after delay, once
// it has written a hook line and a line that is not JSON; with no reply it never answers. It exits when its stdin
// closes or its context ends.
type fakeClaude struct {
	reply string
	delay time.Duration

	mu        sync.Mutex
	empty     []bool // whether dir was empty at each start
	starts    int
	running   int
	most      int
	waited    int
	argv      []string
	dirs      []string
	lines     []string
	deadlines []time.Time
}

func (f *fakeClaude) start(ctx context.Context, dir string, argv []string) (io.WriteCloser, io.Reader, func() error, error) {
	in, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	deadline, _ := ctx.Deadline()
	f.mu.Lock()
	f.starts++
	f.empty = append(f.empty, len(in) == 0)
	f.running++
	f.most = max(f.most, f.running)
	f.argv, f.dirs, f.deadlines = argv, append(f.dirs, dir), append(f.deadlines, deadline)
	f.mu.Unlock()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		sc := bufio.NewScanner(inR)
		for sc.Scan() {
			f.mu.Lock()
			f.lines = append(f.lines, sc.Text())
			f.mu.Unlock()
			if f.reply == "" || !strings.Contains(sc.Text(), `"subtype":"get_usage"`) {
				continue
			}
			go func() {
				time.Sleep(f.delay)
				io.WriteString(outW, `{"type":"system","subtype":"hook_started","hook_name":"SessionStart:startup"}`+"\nnot json\n"+f.reply+"\n")
			}()
		}
	}()
	wait := func() error {
		select {
		case <-closed:
		case <-ctx.Done():
		}
		inR.Close()
		outW.Close()
		f.mu.Lock()
		f.running--
		f.waited++
		f.mu.Unlock()
		return ctx.Err()
	}
	return inW, outR, wait, nil
}

func (f *fakeClaude) seen() (starts, most, waited int, lines []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts, f.most, f.waited, slices.Clone(f.lines)
}

const usageReply = `{"type":"control_response","response":{"subtype":"success","request_id":"aos-usage","response":` + weekly96 + `}}`

func TestNoLiveThreadStartsAProbe(t *testing.T) {
	f := &fakeClaude{reply: usageReply}
	tr, _, _ := tracker(t, threads{{ID: "main/settled", Socket: "/tmp/s.sock"}}, nil, f.start)
	// What a probe left, as one the app was quit during, is gone before the next starts.
	if err := os.MkdirAll(tr.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tr.dir, "left.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := tr.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Windows) != 1 || got.Windows[0].Name != "seven_day" || got.Windows[0].Left != 4 || got.Error != "" {
		t.Errorf("limits = %+v, want the probe's weekly window", got)
	}
	starts, _, waited, lines := f.seen()
	if starts != 1 || waited != 1 {
		t.Fatalf("%d starts, %d waits; want one probe, waited for", starts, waited)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], `"subtype":"get_usage"`) {
		t.Errorf("stdin = %q, want only the get_usage request", lines)
	}
	for _, l := range lines {
		if strings.Contains(l, `"type":"user"`) {
			t.Errorf("the probe sent a user message: %s", l)
		}
	}
	want := []string{"claude", "-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--strict-mcp-config", "--tools", "", "--setting-sources", "project,local"}
	if !slices.Equal(f.argv, want) {
		t.Errorf("argv = %q, want %q", f.argv, want)
	}
	if f.dirs[0] != tr.dir || !f.empty[0] {
		t.Errorf("the probe ran in %s, empty: %v; want %s, emptied", f.dirs[0], f.empty[0], tr.dir)
	}
	if _, err := os.Stat(tr.dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the probe's folder is still there: %v", err)
	}
	// The next probe runs in the same folder.
	tr.fetched = time.Time{}
	if _, err := tr.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.dirs) != 2 || f.dirs[1] != tr.dir {
		t.Errorf("the probes ran in %q, want %s each time", f.dirs, tr.dir)
	}
}

func TestFreshValuesStartNoProbe(t *testing.T) {
	f := &fakeClaude{reply: usageReply}
	tr, c, _ := tracker(t, threads{}, nil, f.start)
	for _, step := range []struct {
		name   string
		after  time.Duration
		owner  bool
		starts int
	}{
		{"the owner's first ask", 0, true, 1},
		{"the owner's ask four minutes on", 4 * time.Minute, true, 1},
		{"the loop, values stale", 2 * time.Minute, false, 1},
		{"the owner's ask, values stale", 0, true, 2},
	} {
		c.add(step.after)
		if err := tr.read(context.Background(), step.owner); err != nil {
			t.Fatal(err)
		}
		if starts, _, _, _ := f.seen(); starts != step.starts {
			t.Fatalf("%s: %d probes, want %d", step.name, starts, step.starts)
		}
	}
}

func TestFailedProbeIsNotCached(t *testing.T) {
	for _, tt := range []struct {
		name        string
		reply       string
		errText     string
		unsupported bool
	}{
		{"Claude's error", `{"type":"control_response","response":{"subtype":"error","request_id":"aos-usage","error":"Not logged in"}}`, "the usage probe: Not logged in", false},
		{"Claude does not know get_usage", `{"type":"control_response","response":{"subtype":"error","request_id":"aos-usage","error":"Unsupported control request subtype: get_usage"}}`, "", true},
		{"no plan limits", `{"type":"control_response","response":{"subtype":"success","request_id":"aos-usage","response":{"rate_limits_available":false}}}`, "", false},
		{"an empty answer", `{"type":"control_response","response":{"subtype":"success","request_id":"aos-usage"}}`, "the usage probe's answer was empty", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeClaude{reply: tt.reply}
			tr, _, _ := tracker(t, threads{}, nil, f.start)
			got, err := tr.Refresh(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got.Error != tt.errText || got.Unsupported != tt.unsupported || len(got.Windows) != 0 {
				t.Errorf("limits = %+v, want error %q, unsupported %v", got, tt.errText, tt.unsupported)
			}
			if _, err := tr.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			if starts, _, waited, _ := f.seen(); starts != 2 || waited != 2 {
				t.Errorf("%d probes, %d waits; want the second refresh to probe again", starts, waited)
			}
		})
	}
}

func TestProbeTimesOut(t *testing.T) {
	f := &fakeClaude{}
	dir := filepath.Join(t.TempDir(), "usage-probe")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	began := time.Now()
	_, err := probe(ctx, f.start, dir)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("err = %v, want the probe not to have answered in time", err)
	}
	if took := time.Since(began); took > 2*time.Second {
		t.Errorf("the probe took %v", took)
	}
	if _, _, waited, _ := f.seen(); waited != 1 {
		t.Errorf("%d waits, want the probe waited for", waited)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the probe's folder is still there: %v", err)
	}
	// Without a nearer deadline of the caller's, a probe runs for at most 15 s, so with its 5 s for the pipes it ends
	// within 20 s.
	f = &fakeClaude{reply: usageReply}
	if _, err := probe(context.Background(), f.start, dir); err != nil {
		t.Fatal(err)
	}
	if deadline := f.deadlines[0]; deadline.IsZero() || deadline.After(time.Now().Add(15*time.Second)) {
		t.Errorf("the probe's deadline is %v, want one at most 15 s on", deadline)
	}
	if probeTimeout+waitDelay > 20*time.Second {
		t.Errorf("a probe may take %v, over 20 s", probeTimeout+waitDelay)
	}
}

func TestOneProbeAtATime(t *testing.T) {
	f := &fakeClaude{reply: usageReply, delay: 100 * time.Millisecond}
	tr, _, _ := tracker(t, threads{}, nil, f.start)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if _, err := tr.Refresh(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if starts, most, _, _ := f.seen(); starts != 1 || most != 1 {
		t.Errorf("%d probes, %d at once; want one, the others taking its answer", starts, most)
	}
}

func TestAProbeThatEndsWithoutAnAnswerFails(t *testing.T) {
	start := func(ctx context.Context, dir string, argv []string) (io.WriteCloser, io.Reader, func() error, error) {
		return Exec(ctx, dir, []string{"sh", "-c", "read line; echo 'not logged in' >&2; exit 1"})
	}
	_, err := probe(context.Background(), start, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "ended without an answer") {
		t.Errorf("err = %v, want the probe to have ended without an answer", err)
	}
}

func TestExec(t *testing.T) {
	t.Setenv("AGENTOS_SESSION", "main/abcdefgh")
	for _, tt := range []struct {
		name    string
		argv    []string
		stdin   string
		stdout  string
		cancel  bool
		waitErr string
	}{
		{"reads stdin and writes stdout", []string{"sh", "-c", "read line; echo \"got $line\""}, "hi\n", "got hi\n", false, ""},
		{"keeps only what Claude needs", []string{"sh", "-c", "echo \"[$AGENTOS_SESSION] $(basename \"$TMPDIR\")\""}, "", "[] dir\n", false, ""},
		{"names stderr when it fails", []string{"sh", "-c", "echo broken >&2; exit 3"}, "", "", false, "broken"},
		{"is stopped when its context ends", []string{"sh", "-c", "sleep 30"}, "", "", true, "killed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "dir")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stdin, stdout, wait, err := Exec(ctx, dir, tt.argv)
			if err != nil {
				t.Fatal(err)
			}
			io.WriteString(stdin, tt.stdin)
			stdin.Close()
			if tt.cancel {
				cancel()
			}
			out, _ := io.ReadAll(stdout)
			done := make(chan error, 1)
			go func() { done <- wait() }()
			var werr error
			select {
			case werr = <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("wait did not return")
			}
			if string(out) != tt.stdout {
				t.Errorf("stdout = %q, want %q", out, tt.stdout)
			}
			if tt.waitErr == "" && werr != nil || tt.waitErr != "" && (werr == nil || !strings.Contains(werr.Error(), tt.waitErr)) {
				t.Errorf("wait = %v, want %q", werr, tt.waitErr)
			}
		})
	}
	if _, _, _, err := Exec(context.Background(), t.TempDir(), nil); err == nil {
		t.Error("Exec with no command started one")
	}
}

func TestProbeSaysAClaudeWithoutGetUsage(t *testing.T) {
	f := &fakeClaude{reply: `{"type":"control_response","response":{"subtype":"error","request_id":"aos-usage","error":"Unsupported control request subtype: get_usage"}}`}
	if _, err := probe(context.Background(), f.start, t.TempDir()); !errors.Is(err, host.ErrNoUsage) {
		t.Errorf("err = %v, want host.ErrNoUsage", err)
	}
}

func TestAProbesGrandchildIsGone(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	// The grandchild leaves the pipes, so nothing waits on it, and runs on after the probe's claude answered and exited.
	script := `sleep 300 </dev/null >/dev/null 2>&1 & echo $! > ` + pidFile + `
read line
echo '` + usageReply + `'
cat >/dev/null`
	start := func(ctx context.Context, dir string, argv []string) (io.WriteCloser, io.Reader, func() error, error) {
		return Exec(ctx, dir, []string{"sh", "-c", script})
	}
	raw, err := probe(context.Background(), start, filepath.Join(t.TempDir(), "usage-probe"))
	if err != nil || len(raw) == 0 {
		t.Fatalf("probe = %s, %v; want the answer", raw, err)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if syscall.Kill(pid, 0) == nil {
		t.Errorf("the probe's grandchild %d still runs", pid)
	}
}
