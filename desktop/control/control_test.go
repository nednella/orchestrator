package control

import (
	"context"
	"errors"
	"os"
	"testing"

	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/project"
)

type fakeThreads struct{ running map[string]bool }

func (f fakeThreads) Has(id string) bool                  { return f.running[id] }
func (f fakeThreads) Current() project.Project            { return project.Project{Name: "Current One"} }
func (f fakeThreads) ThreadOfToken(string) (string, bool) { return "", false }

func TestRouterAnswers(t *testing.T) {
	dir, err := os.MkdirTemp("", "orcrt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	r := New(fakeThreads{})
	r.HandleExternal("echo", func(_ context.Context, req ctl.Request) (string, error) { return "got " + req.Args[0], nil })
	r.HandleExternal("fail", func(context.Context, ctl.Request) (string, error) { return "", errors.New("it failed") })
	r.Handle("inside", func(_ context.Context, req ctl.Request) (string, error) { return "as " + req.Thread, nil })
	r.Listen(dir)
	defer r.Close()

	for _, tt := range []struct {
		req  ctl.Request
		want ctl.Response
	}{
		{ctl.Request{Cmd: "echo", Args: []string{"x"}}, ctl.Response{OK: true, Out: "got x"}},
		{ctl.Request{Cmd: "fail"}, ctl.Response{Error: "it failed"}},
		{ctl.Request{Cmd: "nope"}, ctl.Response{Error: `unknown command "nope"`}},
		// A command only a tool call reaches is unknown to a client, which could name any thread as the asker.
		{ctl.Request{Cmd: "inside", Thread: "p/2"}, ctl.Response{Error: `unknown command "inside"`}},
	} {
		got, err := ctl.Call(context.Background(), ctl.SocketPath(dir), tt.req)
		if err != nil || got != tt.want {
			t.Errorf("%s: %+v, %v; want %+v", tt.req.Cmd, got, err, tt.want)
		}
	}
}

func TestAnswerRunsTheCommandsHandler(t *testing.T) {
	r := New(fakeThreads{})
	r.Handle("echo", func(_ context.Context, req ctl.Request) (string, error) { return req.Thread + " " + req.Args[0], nil })
	r.Handle("fail", func(context.Context, ctl.Request) (string, error) { return "", errors.New("it failed") })
	for _, tt := range []struct {
		req  ctl.Request
		want ctl.Response
	}{
		{ctl.Request{Cmd: "echo", Thread: "p/1", Args: []string{"x"}}, ctl.Response{OK: true, Out: "p/1 x"}},
		{ctl.Request{Cmd: "fail"}, ctl.Response{Error: "it failed"}},
		{ctl.Request{Cmd: ""}, ctl.Response{Error: `unknown command ""`}},
	} {
		if got := r.Answer(context.Background(), tt.req); got != tt.want {
			t.Errorf("Answer(%+v) = %+v, want %+v", tt.req, got, tt.want)
		}
	}
}

func TestAskers(t *testing.T) {
	r := New(fakeThreads{running: map[string]bool{"p/1": true}})
	if id, err := r.AskerThread(ctl.Request{Thread: "p/1"}); err != nil || id != "p/1" {
		t.Errorf("AskerThread = %q, %v", id, err)
	}
	for _, bad := range []string{"", "p/2", "not a thread"} {
		if _, err := r.AskerThread(ctl.Request{Thread: bad}); err == nil {
			t.Errorf("thread %q was accepted", bad)
		}
	}
	tests := []struct {
		req  ctl.Request
		want string
	}{
		{ctl.Request{Project: "digest-project", Thread: "p/1"}, "digest-project"},
		{ctl.Request{Thread: "p/1"}, "p"},
		{ctl.Request{}, "current-one"},
	}
	for _, tt := range tests {
		if got := r.AskerProject(tt.req); got != tt.want {
			t.Errorf("AskerProject(%+v) = %q, want %q", tt.req, got, tt.want)
		}
	}
}
