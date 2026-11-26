package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/control"
)

// callTool runs agentos mcp, sends one tools/call and returns the result's text and error flag.
func callTool(t *testing.T, tool, input string) (string, bool) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	root := newRootCmd()
	root.SetIn(inR)
	root.SetOut(outW)
	root.SetArgs([]string{"mcp"})
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(context.Background()) }()

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + input + `}}` + "\n"
	if _, err := inW.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(outR).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	inW.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Result struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(line, &resp); err != nil || len(resp.Result.Content) != 1 {
		t.Fatalf("answer = %s, %v", line, err)
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

func TestMCPCallCarriesTheToken(t *testing.T) {
	var got control.Request
	fakeApp(t, func(_ context.Context, req control.Request) control.Response {
		got = req
		return control.Response{OK: true, Out: "started demo/x1"}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	text, isErr := callTool(t, "delegate", `{"task":"fix it"}`)
	if text != "started demo/x1" || isErr {
		t.Errorf("result = %q, error %v", text, isErr)
	}
	if got.Cmd != "orchestrate" || got.Token != "tok-1" || strings.Join(got.Args, "|") != "delegate" || got.Opts["input"] != `{"task":"fix it"}` {
		t.Errorf("request = %+v", got)
	}
}

func TestMCPBrowserCallGetsTheBrowsersTime(t *testing.T) {
	tests := []struct {
		tool, input string
		timeoutMs   int
	}{
		{"browser", `{"action":"open","url":"http://127.0.0.1:18001/"}`, browserTimeoutMs},
		{"note", `{"text":"slow build"}`, 0},
		{"delegate", `{"task":"fix it"}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			var got control.Request
			fakeApp(t, func(_ context.Context, req control.Request) control.Response {
				got = req
				return control.Response{OK: true, Out: "ok"}
			})
			t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
			if text, isErr := callTool(t, tt.tool, tt.input); text != "ok" || isErr {
				t.Fatalf("result = %q, error %v", text, isErr)
			}
			if got.Cmd != "orchestrate" || got.Args[0] != tt.tool || got.Opts["input"] != tt.input || got.TimeoutMs != tt.timeoutMs {
				t.Errorf("request = %+v, want %s with a timeout of %d ms", got, tt.tool, tt.timeoutMs)
			}
		})
	}
}

func TestMCPWithoutTokenRefusesCalls(t *testing.T) {
	var calls atomic.Int32
	fakeApp(t, func(context.Context, control.Request) control.Response {
		calls.Add(1)
		return control.Response{OK: true}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "")
	text, isErr := callTool(t, "threads", `{}`)
	if text != "agentos tools work only inside an agentos thread" || !isErr || calls.Load() != 0 {
		t.Errorf("result = %q, error %v, app calls %d", text, isErr, calls.Load())
	}
}

func TestMCPWaitIsSliced(t *testing.T) {
	var calls atomic.Int32
	var last control.Request
	fakeApp(t, func(_ context.Context, req control.Request) control.Response {
		last = req
		if calls.Add(1) < 3 {
			return control.Response{OK: true, Out: `{"done":false,"text":"still running"}`}
		}
		return control.Response{OK: true, Out: `{"done":true,"text":"finished"}`}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	text, isErr := callTool(t, "wait", `{"thread":"demo/x1","timeout_seconds":600}`)
	if text != "finished" || isErr || calls.Load() != 3 {
		t.Errorf("result = %q, error %v, app calls %d", text, isErr, calls.Load())
	}
	if last.Opts["slice_ms"] != "60000" || last.TimeoutMs != 65000 {
		t.Errorf("last slice = %+v", last)
	}
	until, err := strconv.ParseInt(last.Opts["until_ms"], 10, 64)
	if left := time.Until(time.UnixMilli(until)); err != nil || left < 590*time.Second || left > 600*time.Second {
		t.Errorf("until_ms = %q, want the deadline 600 s from the start", last.Opts["until_ms"])
	}
}

func TestMCPWaitStopsAtItsLimit(t *testing.T) {
	var calls atomic.Int32
	fakeApp(t, func(_ context.Context, req control.Request) control.Response {
		calls.Add(1)
		ms, _ := strconv.Atoi(req.Opts["slice_ms"])
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return control.Response{OK: true, Out: `{"done":false,"text":"still running"}`}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	text, _ := callTool(t, "wait", `{"thread":"demo/x1","timeout_seconds":1}`)
	if text != "still running" || calls.Load() != 1 {
		t.Errorf("result = %q, app calls %d", text, calls.Load())
	}
}

func TestMCPWaitStopsAtItsTimeout(t *testing.T) {
	var calls atomic.Int32
	fakeApp(t, func(context.Context, control.Request) control.Response {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return control.Response{OK: true, Out: `{"done":false,"text":"Thread demo/x1 still runs."}`}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	begin := time.Now()
	text, isErr := callTool(t, "wait", `{"thread":"demo/x1","timeout_seconds":2}`)
	if took := time.Since(begin); took < 2*time.Second || took > 3*time.Second {
		t.Errorf("the wait of 2 s took %v", took)
	}
	if text != "Thread demo/x1 still runs." || isErr || calls.Load() < 2 {
		t.Errorf("result = %q, error %v, app calls %d; want the last answer's text after more than one slice", text, isErr, calls.Load())
	}
}

func TestMCPWaitPausesAfterAnEarlyAnswer(t *testing.T) {
	var mu sync.Mutex
	var asked []time.Time
	fakeApp(t, func(context.Context, control.Request) control.Response {
		mu.Lock()
		asked = append(asked, time.Now())
		mu.Unlock()
		return control.Response{OK: true, Out: `{"done":false,"text":"The chat service has not started"}`}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	callTool(t, "wait", `{"thread":"demo/x1","timeout_seconds":3}`)
	mu.Lock()
	defer mu.Unlock()
	// Pauses of 0.5, 1 and 2 s, the last cut at the deadline.
	if len(asked) < 3 || len(asked) > 5 {
		t.Fatalf("the app was asked %d times in 3 s, want 3 to 5", len(asked))
	}
	if first, second := asked[1].Sub(asked[0]), asked[2].Sub(asked[1]); first < 400*time.Millisecond || second < 3*first/2 {
		t.Errorf("pauses %v then %v, want at least 0.4 s, then longer", first, second)
	}
}

func TestMCPIgnoresAgentosSession(t *testing.T) {
	var got control.Request
	fakeApp(t, func(_ context.Context, req control.Request) control.Response {
		got = req
		return control.Response{OK: true, Out: "ok"}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	t.Setenv("AGENTOS_SESSION", "demo/other")
	t.Setenv("AGENTOS_PROJECT", "other")
	callTool(t, "threads", `{}`)
	if got.Cmd != "orchestrate" || got.Session != "" || got.Project != "" {
		t.Errorf("request = %+v", got)
	}
}

func TestMCPWaitRefusesMoreThanAnHour(t *testing.T) {
	fakeApp(t, func(context.Context, control.Request) control.Response { return control.Response{OK: true} })
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	text, isErr := callTool(t, "wait", `{"thread":"demo/x1","timeout_seconds":3601}`)
	if !isErr || text != "timeout_seconds is at most 3600" {
		t.Errorf("result = %q, error %v", text, isErr)
	}
}

func TestMCPWithTheAppDownReportsIt(t *testing.T) {
	dir, err := os.MkdirTemp("", "aosdown")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("AGENTOS_SOCKET", bus.SocketPath(dir))
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	text, isErr := callTool(t, "threads", `{}`)
	if !isErr || text != control.ErrNotRunning.Error() {
		t.Errorf("result = %q, error %v", text, isErr)
	}
}

func TestMCPCancelledWaitStopsAtOnce(t *testing.T) {
	var requests atomic.Int32
	release := make(chan struct{})
	fakeApp(t, func(context.Context, control.Request) control.Response {
		requests.Add(1)
		<-release
		return control.Response{OK: true, Out: `{"done":true,"text":"late"}`}
	})
	t.Cleanup(func() { close(release) })
	socket, err := controlSocket()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := waitInSlices(ctx, socket, "tok-1", json.RawMessage(`{"thread":"demo/x1"}`))
		errc <- err
	}()
	for requests.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) || errors.Is(err, control.ErrNotRunning) || requests.Load() != 1 {
			t.Errorf("err = %v, requests %d", err, requests.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("the cancelled wait did not return within a second")
	}
}
