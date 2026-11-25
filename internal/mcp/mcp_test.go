package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type harness struct {
	t     *testing.T
	in    *io.PipeWriter
	out   *bufio.Reader
	done  chan error
	stop  context.CancelFunc
	lines chan string
}

func start(t *testing.T, tools []Tool, call Call) *harness {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	h := &harness{t: t, in: inW, out: bufio.NewReader(outR), done: make(chan error, 1), stop: cancel, lines: make(chan string, 16)}
	go func() {
		err := Serve(ctx, Info{Name: "agentos", Version: "1"}, tools, call, inR, outW)
		outW.Close()
		h.done <- err
	}()
	go func() {
		for {
			l, err := h.out.ReadString('\n')
			if err != nil {
				close(h.lines)
				return
			}
			h.lines <- l
		}
	}()
	t.Cleanup(func() {
		cancel()
		inW.Close()
	})
	return h
}

func (h *harness) send(line string) {
	h.t.Helper()
	if _, err := io.WriteString(h.in, line+"\n"); err != nil {
		h.t.Fatalf("write: %v", err)
	}
}

func (h *harness) read() map[string]any {
	h.t.Helper()
	select {
	case l, ok := <-h.lines:
		if !ok {
			h.t.Fatal("output closed")
		}
		if !strings.HasSuffix(l, "\n") || strings.Count(l, "\n") != 1 {
			h.t.Fatalf("not one whole line: %q", l)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			h.t.Fatalf("bad json %q: %v", l, err)
		}
		return m
	case <-time.After(2 * time.Second):
		h.t.Fatal("no answer")
	}
	return nil
}

func (h *harness) result(m map[string]any) map[string]any {
	h.t.Helper()
	r, ok := m["result"].(map[string]any)
	if !ok {
		h.t.Fatalf("no result in %v", m)
	}
	return r
}

func callText(t *testing.T, res map[string]any) (string, bool) {
	t.Helper()
	content := res["content"].([]any)
	first := content[0].(map[string]any)
	if first["type"] != "text" {
		t.Fatalf("content type = %v", first["type"])
	}
	return first["text"].(string), res["isError"].(bool)
}

func echo(_ context.Context, tool string, input json.RawMessage) (string, error) {
	return tool + ":" + string(input), nil
}

func TestInitializeEchoesTheProtocolVersion(t *testing.T) {
	tests := []string{"2025-06-18", "2024-11-05"}
	for _, version := range tests {
		t.Run(version, func(t *testing.T) {
			h := start(t, nil, echo)
			h.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` + version + `"}}`)
			m := h.read()
			res := h.result(m)
			if res["protocolVersion"] != version {
				t.Errorf("protocolVersion = %v", res["protocolVersion"])
			}
			if _, ok := res["capabilities"].(map[string]any)["tools"]; !ok {
				t.Errorf("capabilities = %v", res["capabilities"])
			}
			if res["serverInfo"].(map[string]any)["name"] != "agentos" {
				t.Errorf("serverInfo = %v", res["serverInfo"])
			}
			if m["id"] != float64(1) {
				t.Errorf("id = %v", m["id"])
			}
		})
	}
}

func TestToolsListCarriesSchemasAndReadOnly(t *testing.T) {
	tools := []Tool{
		{Name: "threads", Description: "list", Schema: json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`), ReadOnly: true},
		{Name: "stop", Description: "stop one"},
	}
	h := start(t, tools, echo)
	h.send(`{"jsonrpc":"2.0","id":"x","method":"tools/list"}`)
	list := h.result(h.read())["tools"].([]any)
	if len(list) != 2 {
		t.Fatalf("tools = %v", list)
	}
	first := list[0].(map[string]any)
	if first["name"] != "threads" || first["description"] != "list" {
		t.Errorf("first = %v", first)
	}
	if _, ok := first["inputSchema"].(map[string]any)["properties"]; !ok {
		t.Errorf("schema lost: %v", first["inputSchema"])
	}
	if first["annotations"].(map[string]any)["readOnlyHint"] != true {
		t.Errorf("annotations = %v", first["annotations"])
	}
	second := list[1].(map[string]any)
	if second["annotations"].(map[string]any)["readOnlyHint"] != false {
		t.Errorf("second annotations = %v", second["annotations"])
	}
	if second["inputSchema"].(map[string]any)["type"] != "object" {
		t.Errorf("default schema = %v", second["inputSchema"])
	}
}

func TestToolCallAnswersText(t *testing.T) {
	h := start(t, nil, echo)
	h.send(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"read","arguments":{"n":1}}}`)
	m := h.read()
	text, isErr := callText(t, h.result(m))
	if text != `read:{"n":1}` || isErr {
		t.Errorf("text = %q isError = %v", text, isErr)
	}
	if m["id"] != float64(7) {
		t.Errorf("id = %v", m["id"])
	}
}

func TestFailedCallIsAnErrorResult(t *testing.T) {
	h := start(t, nil, func(context.Context, string, json.RawMessage) (string, error) {
		return "", errors.New("no such thread")
	})
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"send"}}`)
	m := h.read()
	if _, ok := m["error"]; ok {
		t.Fatalf("error member present: %v", m)
	}
	text, isErr := callText(t, h.result(m))
	if text != "no such thread" || !isErr {
		t.Errorf("text = %q isError = %v", text, isErr)
	}
}

func TestCancelledCallStops(t *testing.T) {
	started := make(chan struct{})
	h := start(t, nil, func(ctx context.Context, _ string, _ json.RawMessage) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	h.send(`{"jsonrpc":"2.0","id":"w1","method":"tools/call","params":{"name":"wait"}}`)
	<-started
	h.send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"w1"}}`)
	begin := time.Now()
	m := h.read()
	if time.Since(begin) > time.Second {
		t.Errorf("took %v", time.Since(begin))
	}
	if m["id"] != "w1" {
		t.Errorf("id = %v", m["id"])
	}
	if _, isErr := callText(t, h.result(m)); !isErr {
		t.Error("cancelled call should be an error result")
	}
}

func TestCallsRunConcurrently(t *testing.T) {
	release := make(chan struct{})
	h := start(t, nil, func(ctx context.Context, tool string, _ json.RawMessage) (string, error) {
		if tool == "wait" {
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
		return tool, nil
	})
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"wait"}}`)
	h.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"threads"}}`)
	m := h.read()
	if m["id"] != float64(2) {
		t.Fatalf("first answer id = %v, want 2", m["id"])
	}
	close(release)
	if m := h.read(); m["id"] != float64(1) {
		t.Errorf("second answer id = %v", m["id"])
	}
}

func TestUnknownMethodIsMethodNotFound(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"request", `{"jsonrpc":"2.0","id":3,"method":"resources/list"}`, true},
		{"notification", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := start(t, nil, echo)
			h.send(tt.line)
			h.send(`{"jsonrpc":"2.0","id":4,"method":"ping"}`)
			m := h.read()
			if tt.want {
				if code := m["error"].(map[string]any)["code"]; code != float64(-32601) {
					t.Errorf("code = %v", code)
				}
				m = h.read()
			}
			if m["id"] != float64(4) {
				t.Errorf("ping answer = %v", m)
			}
		})
	}
}

func TestBrokenLineIsSkipped(t *testing.T) {
	h := start(t, nil, echo)
	h.send(`{not json`)
	h.send(``)
	h.send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if m := h.read(); m["id"] != float64(1) {
		t.Errorf("answer = %v", m)
	}
}

func TestLongLineIsRead(t *testing.T) {
	h := start(t, nil, func(_ context.Context, _ string, in json.RawMessage) (string, error) {
		return "", errors.New(string(rune('0' + len(in)%10)))
	})
	big := strings.Repeat("a", 1<<20)
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"send","arguments":{"m":"` + big + `"}}}`)
	if m := h.read(); m["id"] != float64(1) {
		t.Errorf("answer = %v", m)
	}
}

func TestEndOfInputCancelsCallsInFlight(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	h := start(t, nil, func(ctx context.Context, _ string, _ json.RawMessage) (string, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return "", ctx.Err()
	})
	h.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"wait"}}`)
	<-started
	h.in.Close()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("call not cancelled")
	}
	select {
	case err := <-h.done:
		if err != nil {
			t.Errorf("Serve = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return")
	}
}

func TestContextEndReturnsServe(t *testing.T) {
	h := start(t, nil, echo)
	h.stop()
	select {
	case err := <-h.done:
		if err != nil {
			t.Errorf("Serve = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return")
	}
}
