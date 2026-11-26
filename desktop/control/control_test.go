package control

import (
	"context"
	"errors"
	"os"
	"testing"

	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/project"
)

type fakeSessions struct{ running map[string]bool }

func (f fakeSessions) Has(id string) bool                  { return f.running[id] }
func (f fakeSessions) Current() project.Project            { return project.Project{Name: "Current One"} }
func (f fakeSessions) ThreadOfToken(string) (string, bool) { return "", false }

func TestRouterAnswers(t *testing.T) {
	dir, err := os.MkdirTemp("", "aosrt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	r := New(fakeSessions{})
	r.HandleExternal("echo", func(_ context.Context, req ctl.Request) (string, error) { return "got " + req.Args[0], nil })
	r.HandleExternal("fail", func(context.Context, ctl.Request) (string, error) { return "", errors.New("it failed") })
	r.Handle("inside", func(_ context.Context, req ctl.Request) (string, error) { return "as " + req.Session, nil })
	r.Listen(dir)
	defer r.Close()

	for _, tt := range []struct {
		req  ctl.Request
		want ctl.Response
	}{
		{ctl.Request{Cmd: "echo", Args: []string{"x"}}, ctl.Response{OK: true, Out: "got x"}},
		{ctl.Request{Cmd: "fail"}, ctl.Response{Error: "it failed"}},
		{ctl.Request{Cmd: "nope"}, ctl.Response{Error: `unknown command "nope"`}},
		// A command only a tool call reaches is unknown to a client, which could name any thread as its session.
		{ctl.Request{Cmd: "inside", Session: "p/2"}, ctl.Response{Error: `unknown command "inside"`}},
	} {
		got, err := ctl.Call(context.Background(), ctl.SocketPath(dir), tt.req)
		if err != nil || got != tt.want {
			t.Errorf("%s: %+v, %v; want %+v", tt.req.Cmd, got, err, tt.want)
		}
	}
}

func TestAnswerRunsTheCommandsHandler(t *testing.T) {
	r := New(fakeSessions{})
	r.Handle("echo", func(_ context.Context, req ctl.Request) (string, error) { return req.Session + " " + req.Args[0], nil })
	r.Handle("fail", func(context.Context, ctl.Request) (string, error) { return "", errors.New("it failed") })
	for _, tt := range []struct {
		req  ctl.Request
		want ctl.Response
	}{
		{ctl.Request{Cmd: "echo", Session: "p/1", Args: []string{"x"}}, ctl.Response{OK: true, Out: "p/1 x"}},
		{ctl.Request{Cmd: "fail"}, ctl.Response{Error: "it failed"}},
		{ctl.Request{Cmd: ""}, ctl.Response{Error: `unknown command ""`}},
	} {
		if got := r.Answer(context.Background(), tt.req); got != tt.want {
			t.Errorf("Answer(%+v) = %+v, want %+v", tt.req, got, tt.want)
		}
	}
}

func TestAskers(t *testing.T) {
	r := New(fakeSessions{running: map[string]bool{"p/1": true}})
	if id, err := r.AskerSession(ctl.Request{Session: "p/1"}); err != nil || id != "p/1" {
		t.Errorf("AskerSession = %q, %v", id, err)
	}
	for _, bad := range []string{"", "p/2", "not a session"} {
		if _, err := r.AskerSession(ctl.Request{Session: bad}); err == nil {
			t.Errorf("session %q was accepted", bad)
		}
	}
	tests := []struct {
		req  ctl.Request
		want string
	}{
		{ctl.Request{Project: "digest-project", Session: "p/1"}, "digest-project"},
		{ctl.Request{Session: "p/1"}, "p"},
		{ctl.Request{}, "current-one"},
	}
	for _, tt := range tests {
		if got := r.AskerProject(tt.req); got != tt.want {
			t.Errorf("AskerProject(%+v) = %q, want %q", tt.req, got, tt.want)
		}
	}
}
