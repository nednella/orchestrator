// Package tmuxtest stops the tmux server of a test: tmux leaves the socket file of a server that was killed, one per
// test run, in the shared tmux folder.
package tmuxtest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// prefix starts the name of every test socket. Stop touches no other, so it can never reach a socket of a live app.
const prefix = "orctest"

// Stop kills the tmux server on the named test socket and removes its socket file, which stays behind when the server
// is killed and when it exits with its last session. It returns once what the panes ran has exited, or after 5 seconds,
// so a test does not remove its folders while a process still writes in them. A socket that has no server is a no-op.
// A name that does not start with "orctest" is left alone.
func Stop(socket string) {
	if !strings.HasPrefix(socket, prefix) || strings.ContainsRune(socket, filepath.Separator) {
		return
	}
	out, _ := exec.Command("tmux", "-L", socket, "list-panes", "-a", "-F", "#{pane_pid}").Output()
	_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
	_ = os.Remove(Path(socket))
	// tmux starts each pane in a session of its own, so the pane's pid names the group of everything it started.
	deadline := time.Now().Add(5 * time.Second)
	for _, field := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(field)
		if err != nil || pid <= 1 {
			continue
		}
		for !errors.Is(syscall.Kill(-pid, 0), syscall.ESRCH) && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// Path is where tmux puts the socket named socket: tmux-<uid> in $TMUX_TMPDIR, else in /tmp.
func Path(socket string) string {
	dir := os.Getenv("TMUX_TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "tmux-"+strconv.Itoa(os.Getuid()), socket)
}
