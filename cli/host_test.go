package cli

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestHostStopsOnSIGTERM(t *testing.T) {
	// A short folder under /tmp keeps the socket path inside a unix socket's length limit.
	dir, err := os.MkdirTemp("/tmp", "aos")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "s")

	root := newRootCmd()
	root.SetArgs([]string{"host", "--log", filepath.Join(dir, "chat.jsonl"), "--socket", socket, "--", "sleep", "30"})
	done := make(chan error, 1)
	go func() { done <- root.Execute() }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the host never served its socket")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the host kept running after SIGTERM")
	}
}
