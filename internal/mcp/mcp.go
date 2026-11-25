// Package mcp serves tools to an agent over stdio, as JSON-RPC 2.0 lines.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// Tool is one tool the server lists. ReadOnly becomes annotations.readOnlyHint.
type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage
	ReadOnly    bool
}

// Info names the server in the initialize answer.
type Info struct {
	Name, Version string
}

// Call runs a tool and returns the text it answers, or why it failed.
type Call func(ctx context.Context, tool string, input json.RawMessage) (string, error)

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type server struct {
	info  Info
	tools []Tool
	call  Call

	outMu sync.Mutex
	out   io.Writer

	mu       sync.Mutex
	inflight map[string]context.CancelFunc
	calls    sync.WaitGroup
}

// Serve answers requests from in on out until in ends or ctx ends, then
// cancels the calls in flight and waits for their answers to be written.
func Serve(ctx context.Context, info Info, tools []Tool, call Call, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &server{info: info, tools: tools, call: call, out: out, inflight: map[string]context.CancelFunc{}}
	defer s.calls.Wait()
	defer cancel()

	type line struct {
		data []byte
		err  error
	}
	lines := make(chan line)
	go func() {
		r := bufio.NewReader(in)
		for {
			data, err := r.ReadBytes('\n')
			if len(data) > 0 {
				select {
				case lines <- line{data: data}:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				select {
				case lines <- line{err: err}:
				case <-ctx.Done():
				}
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case l := <-lines:
			if l.err != nil {
				if errors.Is(l.err, io.EOF) {
					return nil
				}
				return fmt.Errorf("reading request: %w", l.err)
			}
			if err := s.handle(ctx, l.data); err != nil {
				return err
			}
		}
	}
}

func (s *server) handle(ctx context.Context, data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	var req request
	if json.Unmarshal(data, &req) != nil || req.Method == "" {
		return nil
	}
	isRequest := len(req.ID) > 0

	switch req.Method {
	case "notifications/cancelled":
		var p struct {
			RequestID json.RawMessage `json:"requestId"`
		}
		if json.Unmarshal(req.Params, &p) == nil {
			s.cancel(p.RequestID)
		}
		return nil
	case "tools/call":
		if isRequest {
			s.startCall(ctx, req)
		}
		return nil
	}
	if !isRequest {
		return nil
	}

	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return s.reply(req.ID, map[string]any{
			"protocolVersion": p.ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": s.info.Name, "version": s.info.Version},
		})
	case "tools/list":
		return s.reply(req.ID, map[string]any{"tools": s.listing()})
	case "ping":
		return s.reply(req.ID, map[string]any{})
	}
	return s.fail(req.ID, codeMethodNotFound, "method not found: "+req.Method)
}

func (s *server) listing() []map[string]any {
	list := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		schema := t.Schema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object"}`)
		}
		list = append(list, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": schema,
			"annotations": map[string]any{"readOnlyHint": t.ReadOnly},
		})
	}
	return list
}

func (s *server) startCall(ctx context.Context, req request) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil || p.Name == "" {
		_ = s.fail(req.ID, codeInvalidParams, "tools/call needs a tool name")
		return
	}
	if len(p.Arguments) == 0 {
		p.Arguments = json.RawMessage(`{}`)
	}

	key := idKey(req.ID)
	callCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.inflight[key] = cancel
	s.mu.Unlock()

	s.calls.Add(1)
	go func() {
		defer s.calls.Done()
		defer func() {
			s.mu.Lock()
			delete(s.inflight, key)
			s.mu.Unlock()
			cancel()
		}()
		text, err := s.call(callCtx, p.Name, p.Arguments)
		isError := err != nil
		if isError {
			text = err.Error()
		}
		_ = s.reply(req.ID, map[string]any{
			"content": []map[string]string{{"type": "text", "text": text}},
			"isError": isError,
		})
	}()
}

func (s *server) cancel(id json.RawMessage) {
	s.mu.Lock()
	cancel := s.inflight[idKey(id)]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func idKey(id json.RawMessage) string {
	var b bytes.Buffer
	if json.Compact(&b, id) != nil {
		return string(id)
	}
	return b.String()
}

func (s *server) reply(id json.RawMessage, result any) error {
	return s.write(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *server) fail(id json.RawMessage, code int, msg string) error {
	return s.write(response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *server) write(r response) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encoding response: %w", err)
	}
	data = append(data, '\n')
	s.outMu.Lock()
	defer s.outMu.Unlock()
	if _, err := s.out.Write(data); err != nil {
		return fmt.Errorf("writing response: %w", err)
	}
	return nil
}
