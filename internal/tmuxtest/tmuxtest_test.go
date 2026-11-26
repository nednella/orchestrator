package tmuxtest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStopKillsTheServerAndRemovesItsSocketFile(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	socket := fmt.Sprintf("aostest-tmuxtest-%d", os.Getpid())
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
	socket := fmt.Sprintf("aostest-tmuxtest-wait-%d", os.Getpid())
	done := filepath.Join(t.TempDir(), "done")
	// The pane ignores the hangup kill-server sends and writes a file a moment later, as a host that is still closing does.
	script := "trap '' HUP; sleep 0.5; touch '" + done + "'"
	if out, err := exec.Command("tmux", "-L", socket, "new-session", "-d", "-s", "x", "sh", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("starting a server: %v: %s", err, out)
	}
	Stop(socket)
	if _, err := os.Stat(done); err != nil {
		t.Errorf("Stop returned while the pane's process still ran: %v", err)
	}
}

func TestStopRemovesTheFileOfAServerThatIsGone(t *testing.T) {
	socket := fmt.Sprintf("aostest-tmuxtest-gone-%d", os.Getpid())
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
