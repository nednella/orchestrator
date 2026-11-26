package cli

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// startHost runs agentos host on argv and waits for its socket. It returns the host's folder and a channel that gets
// the command's result.
func startHost(t *testing.T, argv ...string) (dir string, done <-chan error) {
	t.Helper()
	// A short folder under /tmp keeps the socket path inside a unix socket's length limit.
	dir, err := os.MkdirTemp("/tmp", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s")

	root := newRootCmd(nil)
	root.SetArgs(append([]string{"host", "--log", filepath.Join(dir, "chat.jsonl"), "--socket", socket, "--"}, argv...))
	result := make(chan error, 1)
	go func() { result <- root.Execute() }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(socket); err == nil {
			return dir, result
		}
		if time.Now().After(deadline) {
			t.Fatal("the host never served its socket")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func stopsOnSIGTERM(t *testing.T, done <-chan error) {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the host kept running after SIGTERM")
	}
}

func TestHostStopsOnSIGTERM(t *testing.T) {
	_, done := startHost(t, "sleep", "30")
	stopsOnSIGTERM(t, done)
}

func TestSIGINTInterruptsTheTurn(t *testing.T) {
	stdin := filepath.Join(t.TempDir(), "stdin")
	_, done := startHost(t, "sh", "-c", "cat >> "+stdin)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, _ := os.ReadFile(stdin)
		if strings.Contains(string(data), `"type":"control_request"`) && strings.Contains(string(data), `"subtype":"interrupt"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("claude's stdin has no interrupt:\n%s", data)
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case err := <-done:
		t.Fatalf("the host stopped after SIGINT: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	stopsOnSIGTERM(t, done)
}
