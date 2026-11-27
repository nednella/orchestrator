package devhttp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/app"
	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/projects"
	"github.com/nednella/orchestrator/desktop/sessions"
)

func TestCallRunsABoundMethod(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	got, err := call(h.App.Services(), "sessions.Service", "NewThread", []byte(`["via call","","","",false,"supervised",""]`))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := got.(sessions.Session)
	if !ok || s.Title != "via call" {
		t.Fatalf("result = %+v", got)
	}
	if got, err = call(h.App.Services(), "projects.Service", "Snapshot", []byte(`[]`)); err != nil || len(got.(projects.Snapshot).Sessions) != 1 {
		t.Errorf("Snapshot = %+v, %v", got, err)
	}
	if got, err = call(h.App.Services(), "sessions.Service", "RenameSession", []byte(`["`+s.ID+`","renamed"]`)); err != nil || got != nil {
		t.Errorf("a method with no result gave %v, %v", got, err)
	}
}

func TestCallReportsProblems(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	for _, tt := range []struct{ method, body string }{
		{"sessions.Service/NoSuchMethod", `[]`},
		{"sessions.Service/NewThread", `["only one argument"]`},
		{"sessions.Service/NewThread", `[1,2]`},
		{"projects.Service/Snapshot", `not json`},
		{"sessions.Service/SettleThread", `["main/99"]`}, // the method itself fails
		{"nothing.Service/Snapshot", `[]`},
	} {
		svc, method, _ := strings.Cut(tt.method, "/")
		if got, err := call(h.App.Services(), svc, method, []byte(tt.body)); err == nil {
			t.Errorf("%s %s succeeded: %v", tt.method, tt.body, got)
		}
	}
}

func TestEventHubReachesEveryTab(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.serve))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lines := make(chan string, 4)
	for range 2 {
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			sc := bufio.NewScanner(resp.Body)
			for sc.Scan() {
				if strings.HasPrefix(sc.Text(), "data: ") {
					lines <- sc.Text()
				}
			}
		}()
	}
	// Wait until both tabs are subscribed, then emit.
	deadline := time.Now().Add(3 * time.Second)
	for {
		hub.mu.Lock()
		n := len(hub.subs)
		hub.mu.Unlock()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("tabs did not subscribe")
		}
		time.Sleep(10 * time.Millisecond)
	}
	hub.Emit("sessions", []string{"a"})
	for range 2 {
		select {
		case line := <-lines:
			var m struct {
				Event   string
				Payload []string
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &m); err != nil || m.Event != "sessions" || m.Payload[0] != "a" {
				t.Errorf("event line %q: %v", line, err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("a tab did not get the event")
		}
	}
}

const testToken = "test-token"

// serve runs a new app in browser mode and returns the harness and the address it serves on.
func serve(t *testing.T, env ...string) (*apptest.Harness, string) {
	t.Helper()
	h := apptest.NewWith(t, apptest.Options{Env: env})
	hub := NewHub()
	cfg, err := app.LoadEnv(h.Env)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(cfg, app.Host{Emit: hub.Emit, Clipboard: func(string) {}}, apptest.NoGH, apptest.NoStream, apptest.NoClaude)
	assets := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><html><head></head><body>stub</body></html>")}}
	addr := "127.0.0.1:" + freePort(t)
	go func() {
		_ = Serve(Options{Services: a.Services(), Start: a.Start, Stop: a.Stop, Hub: hub, Assets: assets, Handler: a.Assets(), Addr: addr, Token: testToken})
	}()
	base := "http://" + addr
	eventually(t, "the server", func() bool {
		resp, err := http.Get(base + "/__shim.js")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == 200
	})
	return h, base
}

func TestBrowserModeServesTheAppAndTheShim(t *testing.T) {
	t.Parallel()
	h, base := serve(t)
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), `<script src="/__shim.js"></script>`) {
		t.Errorf("the page does not load the shim:\n%s", page)
	}

	resp, err = post(base+"/__call/projects.Service/Snapshot", nil)
	if err != nil {
		t.Fatal(err)
	}
	var snap projects.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil || snap.Project.Dir != h.Dir {
		t.Errorf("Snapshot over HTTP = %+v, %v", snap, err)
	}
	resp.Body.Close()

	resp, err = post(base+"/__call/projects.Service/Nope", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("an unknown method gave %d", resp.StatusCode)
	}
	resp.Body.Close()

	for _, tt := range []struct {
		name    string
		headers map[string]string
		host    string
		want    int
	}{
		{"the shim's call", nil, "", http.StatusOK},
		{"same origin", map[string]string{"Origin": base}, "", http.StatusOK},
		{"a foreign origin", map[string]string{"Origin": "https://evil.example"}, "", http.StatusForbidden},
		{"a null origin", map[string]string{"Origin": "null"}, "", http.StatusForbidden},
		{"a foreign host name", nil, "evil.example", http.StatusForbidden},
		{"no marker header", map[string]string{callHeader: ""}, "", http.StatusForbidden},
		{"no token", map[string]string{tokenHeader: ""}, "", http.StatusForbidden},
		{"a wrong token", map[string]string{tokenHeader: "guess"}, "", http.StatusForbidden},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := post(base+"/__call/projects.Service/Snapshot", tt.headers, withHost(tt.host))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
	resp, err = http.Get(base + "/__shim.js")
	if err != nil {
		t.Fatal(err)
	}
	shim, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(shim), "'"+callHeader+"': '1'") {
		t.Error("the shim does not send the marker header")
	}
}

func TestTermStreamNeedsTheAppHeader(t *testing.T) {
	t.Parallel()
	_, base := serve(t, "SHELL=/bin/sh", "HOME="+t.TempDir())
	var shell sessions.ShellInfo
	callJSON(t, base, "sessions.Service/ShellNew", `[""]`, &shell)
	callJSON(t, base, "terminal.Service/TermOpen", `["`+shell.ID+`",80,24]`, nil)

	for _, tt := range []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"no marker header", map[string]string{callHeader: ""}, http.StatusForbidden},
		{"no token", map[string]string{tokenHeader: ""}, http.StatusForbidden},
		{"a wrong token", map[string]string{tokenHeader: "guess"}, http.StatusForbidden},
		{"another origin", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"the shim's read", map[string]string{"Origin": base}, http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "GET", base+"/term/"+url.PathEscape(shell.ID), nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set(callHeader, "1")
			req.Header.Set(tokenHeader, testToken)
			for k, v := range tt.headers {
				if v == "" {
					req.Header.Del(k)
				} else {
					req.Header.Set(k, v)
				}
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Fatalf("status %d, want %d", resp.StatusCode, tt.want)
			}
			if tt.want != http.StatusOK {
				return
			}
			if got := resp.Header.Get("Content-Type"); got != "application/octet-stream" {
				t.Errorf("Content-Type = %q", got)
			}
			if _, err := io.ReadFull(resp.Body, make([]byte, 1)); err != nil {
				t.Errorf("no output from the shell: %v", err)
			}
		})
	}
}

// callJSON makes a call as the shim does and decodes its result into out.
func callJSON(t *testing.T, base, method, args string, out any) {
	t.Helper()
	req, err := http.NewRequest("POST", base+"/__call/"+method, strings.NewReader(args))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(callHeader, "1")
	req.Header.Set(tokenHeader, testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s: %d %s", method, resp.StatusCode, body)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			t.Fatalf("%s: %v in %s", method, err, body)
		}
	}
}

func withHost(host string) func(*http.Request) {
	return func(r *http.Request) {
		if host != "" {
			r.Host = host
		}
	}
}

// post sends an empty-argument call as the shim does, with headers changed by the given ones.
func post(url string, headers map[string]string, edits ...func(*http.Request)) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(`[]`))
	if err != nil {
		return nil, err
	}
	req.Header.Set(callHeader, "1")
	req.Header.Set(tokenHeader, testToken)
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	for _, edit := range edits {
		edit(req)
	}
	return http.DefaultClient.Do(req)
}

// ports hands each test its own port: tests run at the same time, and a port that was free when one asked is not yet
// in use while another asks.
var ports struct {
	sync.Mutex
	taken map[int]bool
}

// freePort is a free port among 18000–18999, the ports tests may listen on, that no other test of this run was given.
func freePort(t *testing.T) string {
	t.Helper()
	ports.Lock()
	defer ports.Unlock()
	if ports.taken == nil {
		ports.taken = map[int]bool{}
	}
	start := os.Getpid() % 1000
	for i := range 1000 {
		port := 18000 + (start+i)%1000
		if ports.taken[port] {
			continue
		}
		if ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port)); err == nil {
			ln.Close()
			ports.taken[port] = true
			return strconv.Itoa(port)
		}
	}
	t.Fatal("no free port among 18000–18999")
	return ""
}

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

func eventually(t *testing.T, what string, cond func() bool) { apptest.Eventually(t, what, cond) }

func TestEventStreamNeedsTheToken(t *testing.T) {
	t.Parallel()
	_, base := serve(t)
	for _, tt := range []struct {
		name, query string
		want        int
	}{
		{"no token", "", http.StatusForbidden},
		{"a wrong token", "?k=guess", http.StatusForbidden},
		{"the token", "?k=" + testToken, http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "GET", base+"/__events"+tt.query, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Errorf("status %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}
}

func TestShimSendsTheToken(t *testing.T) {
	t.Parallel()
	_, base := serve(t)
	resp, err := http.Get(base + "/__shim.js")
	if err != nil {
		t.Fatal(err)
	}
	shim, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	for _, want := range []string{"'" + tokenHeader + "': token", "/__events?k=", "startsWith('/term/')"} {
		if !strings.Contains(string(shim), want) {
			t.Errorf("the shim lacks %q", want)
		}
	}
}

func TestServeRefusesAnAddressThatIsNotLoopback(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"0.0.0.0:34000", ":34000", "192.168.1.5:34000", "example.com:34000", "[::]:34000", "34000"} {
		t.Run(addr, func(t *testing.T) {
			err := Serve(Options{Addr: addr, Start: func(context.Context) error {
				t.Error("the app started on an address it refuses")
				return nil
			}})
			if err == nil {
				t.Fatal("Serve took the address")
			}
		})
	}
	for _, addr := range []string{"127.0.0.1:34000", "[::1]:34000", "localhost:34000"} {
		if err := loopback(addr); err != nil {
			t.Errorf("loopback(%q) = %v", addr, err)
		}
	}
}
