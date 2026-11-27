package tmuxtest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestStopKillsTheServerAndRemovesItsSocketFile(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	socket := fmt.Sprintf("orctest-tmuxtest-%d", os.Getpid())
	if out, err := exec.Command("tmux", "-L", socket, "new-session", "-d", "-s", "x", "sleep", "60").CombinedOutput(); err != nil {
		t.Fatalf("starting a server: %v: %s", err, out)
	}
	out, err := exec.Command("tmux", "-L", socket, "display-message", "-p", "#{socket_path}").Output()
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(string(out))
	if want, _ := filepath.EvalSymlinks(filepath.Dir(Path(socket))); filepath.Dir(path) != want {
		t.Fatalf("Path = %s, tmux says %s", Path(socket), path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("a running server has no socket file %s: %v", path, err)
	}

	Stop(socket)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the socket file %s is still there: %v", path, err)
	}
	if exec.Command("tmux", "-L", socket, "has-session", "-t", "x").Run() == nil {
		t.Error("the server still runs")
	}
	Stop(socket) // none is running: nothing to stop, nothing to fail
}

func TestStopWaitsForWhatThePanesRan(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	socket := fmt.Sprintf("orctest-tmuxtest-wait-%d", os.Getpid())
	pidFile := filepath.Join(t.TempDir(), "pid")
	// The pane ignores the signals a kill-server may send and lives on a moment, as a host that is still closing does.
	script := "trap '' HUP TERM INT; echo $$ > '" + pidFile + "'; sleep 0.5"
	if out, err := exec.Command("tmux", "-L", socket, "new-session", "-d", "-s", "x", "sh", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("starting a server: %v: %s", err, out)
	}
	var pid int
	for deadline := time.Now().Add(5 * time.Second); pid == 0 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		b, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	if pid == 0 {
		t.Fatal("the pane never started")
	}
	Stop(socket)
	if err := syscall.Kill(-pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("Stop returned while the pane's process group %d still ran: %v", pid, err)
	}
}

func TestStopRemovesTheFileOfAServerThatIsGone(t *testing.T) {
	socket := fmt.Sprintf("orctest-tmuxtest-gone-%d", os.Getpid())
	path := Path(socket)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	Stop(socket)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the stale socket file is still there: %v", err)
	}
}

func TestStopLeavesAnyOtherSocketAlone(t *testing.T) {
	socket := fmt.Sprintf("keep-tmuxtest-%d", os.Getpid())
	path := Path(socket)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	Stop(socket)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("a socket that is no test's was removed: %v", err)
	}
}
