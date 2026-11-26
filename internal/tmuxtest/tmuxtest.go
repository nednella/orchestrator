// Package tmuxtest stops the tmux server of a test: tmux leaves the socket file of a server that was killed, one per
// test run, in the shared tmux folder.
package tmuxtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// prefix starts the name of every test socket. Stop touches no other, so it can never reach a socket of a live app.
const prefix = "aostest"

// Stop kills the tmux server on the named test socket and removes its socket file, which stays behind when the server
// is killed and when it exits with its last session. A socket that has no server is a no-op. A name that does not start
// with "aostest" is left alone.
func Stop(socket string) {
	if !strings.HasPrefix(socket, prefix) || strings.ContainsRune(socket, filepath.Separator) {
		return
	}
	_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
	_ = os.Remove(Path(socket))
}

// Path is where tmux puts the socket named socket: tmux-<uid> in $TMUX_TMPDIR, else in /tmp.
func Path(socket string) string {
	dir := os.Getenv("TMUX_TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "tmux-"+strconv.Itoa(os.Getuid()), socket)
}
