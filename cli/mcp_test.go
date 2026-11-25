package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
			return control.Response{OK: true, Out: `{"done":false}`}
		}
		return control.Response{OK: true, Out: `{"done":true,"text":"finished"}`}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	text, isErr := callTool(t, "wait", `{"thread":"demo/x1","timeout_seconds":600}`)
	if text != `{"done":true,"text":"finished"}` || isErr || calls.Load() != 3 {
		t.Errorf("result = %q, error %v, app calls %d", text, isErr, calls.Load())
	}
	if last.Opts["slice_ms"] != "60000" || last.TimeoutMs != 65000 {
		t.Errorf("last slice = %+v", last)
	}
}

func TestMCPWaitStopsAtItsLimit(t *testing.T) {
	var calls atomic.Int32
	fakeApp(t, func(_ context.Context, req control.Request) control.Response {
		calls.Add(1)
		ms, _ := strconv.Atoi(req.Opts["slice_ms"])
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return control.Response{OK: true, Out: `{"done":false}`}
	})
	t.Setenv("AGENTOS_MCP_TOKEN", "tok-1")
	text, _ := callTool(t, "wait", `{"thread":"demo/x1","timeout_seconds":1}`)
	if text != `{"done":false}` || calls.Load() != 1 {
		t.Errorf("result = %q, app calls %d", text, calls.Load())
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
