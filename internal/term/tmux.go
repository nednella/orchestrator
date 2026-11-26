package term

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nednella/agentos/internal/session"
)

const (
	titleOption  = "@agentos-name"
	issueOption  = "@agentos-issue"
	modelOption  = "@agentos-model"
	effortOption = "@agentos-effort"
	accessOption = "@agentos-access"
	chatOption   = "@agentos-chat" // an older build's tag of a read-only session
	actionOption = "@agentos-action"
	hostedOption = "@agentos-hosted"
	forkOption   = "@agentos-fork"
	branchOption = "@agentos-branch"
	parentOption = "@agentos-parent"
	threadOption = "@agentos-thread"
)

const conf = `
set -g status off
set -g prefix None
unbind C-b
set -g default-terminal tmux-256color
set -sa terminal-features ',*:RGB:extkeys'
set -s escape-time 0
set -g focus-events on
set -g mouse on
set -s extended-keys always
set -s extended-keys-format csi-u
set -g allow-passthrough on
set -g set-clipboard on
set -g history-limit 50000
`

// Tmux drives the private tmux server that keeps agents alive while agentos is closed.
type Tmux struct {
	socket string
	conf   string
}

// NewTmux prepares the config file in dir. socket names the private server.
func NewTmux(socket, dir string) (*Tmux, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating state dir: %w", err)
	}
	path := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		return nil, fmt.Errorf("writing tmux config: %w", err)
	}
	return &Tmux{socket: socket, conf: path}, nil
}

// Info is one agent session as tmux reports it.
type Info struct {
	Name    session.Name
	Title   string
	Path    string
	Issue   string // the GitHub issue number the session was started for, or ""
	Model   string // the model the agent runs: the start choice until its first turn, then what it used last, or ""
	Effort  string // the effort the agent runs, kept like Model, or ""
	Access  session.Access
	Hosted  bool   // the agent runs under agentos host, which takes its messages over a socket
	Action  string // the queue action the session was started with, or ""
	Fork    string // the session a fork was forked from, or ""
	Branch  string // the branch the thread works on, in the worktree it started in, or ""
	Parent  string // the thread that delegated to this one, or ""
	Created time.Time
	PID     int // the process the session's pane runs: the agent, or the shell that started it
}

// Shell is one shell session as tmux reports it.
type Shell struct {
	Name    session.Name
	Thread  string // the thread the shell belongs to, "" for a shell of the project
	Command string // the name of the process in front in the shell's pane, such as zsh, vim or node, or ""
	PID     int    // the shell's own process, which its pane runs
	Path    string // the folder the shell is in now; only ShellPaths reads it
}

// argv runs tmux with -u: without a UTF-8 locale (an app started from Finder has none)
// tmux turns the tabs in list output into \011 and draws non-ASCII text as underscores.
func (t *Tmux) argv(args ...string) []string {
	return append([]string{"tmux", "-u", "-L", t.socket, "-f", t.conf}, args...)
}

func (t *Tmux) run(ctx context.Context, args ...string) (string, error) {
	return t.runWith(ctx, cleanEnv(), args...)
}

// runWith is run with env as the environment of the tmux client.
func (t *Tmux) runWith(ctx context.Context, env []string, args ...string) (string, error) {
	argv := t.argv(args...)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("tmux %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// inherited are dropped from the environment the tmux server starts with: TMUX so an app started
// inside another tmux can still attach, and a session's identity so an app or test started inside
// an agentos session does not pass it on to every session and shell it starts.
var inherited = []string{"TMUX", "AGENTOS_SESSION", "AGENTOS_ISSUE", "AGENTOS_PROJECT"}

func cleanEnv() []string {
	return slices.DeleteFunc(os.Environ(), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.Contains(inherited, name)
	})
}

// ListAll returns the agent sessions and the shell sessions.
// No running server means no sessions; any other failure is an error, because
// callers must not read it as "everything ended".
func (t *Tmux) ListAll(ctx context.Context) ([]Info, []Shell, error) {
	const fields = 17
	// The process name comes last, so a tab in it stays in its own field.
	out, err := t.run(ctx, "list-sessions", "-F", "#{session_name}\t#{"+titleOption+"}\t#{session_path}\t#{"+issueOption+"}\t#{"+modelOption+"}\t#{"+effortOption+"}\t#{"+accessOption+"}\t#{"+chatOption+"}\t#{"+actionOption+"}\t#{"+hostedOption+"}\t#{"+forkOption+"}\t#{session_created}\t#{pane_pid}\t#{"+branchOption+"}\t#{"+parentOption+"}\t#{"+threadOption+"}\t#{pane_current_command}")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && noServer(err.Error()) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var infos []Info
	var shells []Shell
	for line := range strings.SplitSeq(out, "\n") {
		f := strings.SplitN(line, "\t", fields)
		if len(f) != fields {
			continue
		}
		name, err := session.ParseName(f[0])
		if err != nil {
			continue
		}
		if name.IsShell() {
			pid, _ := strconv.Atoi(f[12])
			shells = append(shells, Shell{Name: name, Thread: f[15], Command: f[16], PID: pid})
			continue
		}
		info := Info{Name: name, Title: f[1], Path: f[2], Issue: f[3], Model: f[4], Effort: f[5], Access: accessOf(f[6], f[7]), Action: f[8], Hosted: f[9] != "", Fork: f[10], Branch: f[13], Parent: f[14]}
		if secs, err := strconv.ParseInt(f[11], 10, 64); err == nil {
			info.Created = time.Unix(secs, 0)
		}
		info.PID, _ = strconv.Atoi(f[12])
		infos = append(infos, info)
	}
	return infos, shells, nil
}

// ShellPaths are the shell sessions with the thread each belongs to and the folder each is in now: a shell's folder
// follows its cd, so it is the pane's current folder, not the one the session started in. No running server means no
// shells.
func (t *Tmux) ShellPaths(ctx context.Context) ([]Shell, error) {
	out, err := t.run(ctx, "list-panes", "-a", "-F", "#{session_name}\t#{"+threadOption+"}\t#{pane_current_path}")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && noServer(err.Error()) {
			return nil, nil
		}
		return nil, err
	}
	var shells []Shell
	for line := range strings.SplitSeq(out, "\n") {
		// The folder comes last, so a tab in it stays in its own field.
		f := strings.SplitN(line, "\t", 3)
		if len(f) != 3 {
			continue
		}
		if name, err := session.ParseName(f[0]); err == nil && name.IsShell() {
			shells = append(shells, Shell{Name: name, Thread: f[1], Path: f[2]})
		}
	}
	return shells, nil
}

// accessOf is the access a session's tags give: its access tag, else Read for an older build's chat tag.
func accessOf(tag, chat string) session.Access {
	if tag == "" && chat != "" {
		return session.Read
	}
	return session.StoredAccess(tag)
}

// ServerPID is the tmux server's process, or 0 when no server runs.
func (t *Tmux) ServerPID(ctx context.Context) (int, error) {
	out, err := t.run(ctx, "display-message", "-p", "#{pid}")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && noServer(err.Error()) {
			return 0, nil
		}
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// noServer recognises tmux's message for a socket nobody listens on.
func noServer(msg string) bool {
	return strings.Contains(msg, "no server running") ||
		strings.Contains(msg, "error connecting") && (strings.Contains(msg, "No such file") || strings.Contains(msg, "Connection refused"))
}

// target names a session for -t. The "=" makes tmux match the whole name: without
// it "p/1" also matches "p/10". The ":" lets commands that take a pane accept it.
func target(name session.Name) string { return "=" + name.String() + ":" }

// arg escapes a trailing ";", which tmux would take as the end of the command.
func arg(s string) string {
	if strings.HasSuffix(s, ";") {
		return s[:len(s)-1] + `\;`
	}
	return s
}

// Has says whether a tmux session of that name exists.
func (t *Tmux) Has(ctx context.Context, name session.Name) bool {
	_, err := t.run(ctx, "has-session", "-t", target(name))
	return err == nil
}

// NewSession starts argv detached in dir at the given size.
func (t *Tmux) NewSession(ctx context.Context, name session.Name, title, dir string, env []string, argv []string, cols, rows int) error {
	args := []string{
		"new-session", "-d", "-s", name.String(), "-c", dir,
		"-x", strconv.Itoa(cols), "-y", strconv.Itoa(rows),
	}
	client := cleanEnv()
	for _, kv := range env {
		args = append(args, "-e", kv)
		// tmux gives a new pane the PATH of the client that starts it, whatever -e says.
		if strings.HasPrefix(kv, "PATH=") {
			client = append(client, kv)
		}
	}
	args = append(args, "--")
	args = append(args, argv...)
	args = append(args, ";", "set-option", "-t", target(name), titleOption, arg(oneLine(title)))
	if _, err := t.runWith(ctx, client, args...); err != nil {
		return fmt.Errorf("starting session %s: %w", name, err)
	}
	return nil
}

// Kill stops a session and every process under its pane. tmux only hangs the pane up, and a child that ignores
// SIGHUP or runs in a process group of its own would outlive the session.
func (t *Tmux) Kill(ctx context.Context, name session.Name) error {
	var procs map[int]proc
	if out, err := t.run(ctx, "display-message", "-p", "-t", target(name), "#{pane_pid}"); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(out))
		if all, err := processes(ctx); err == nil && pid > 0 {
			procs = tree(all, pid)
		}
	}
	if _, err := t.run(ctx, "kill-session", "-t", target(name)); err != nil {
		return fmt.Errorf("stopping session %s: %w", name, err)
	}
	stopTree(ctx, procs)
	return nil
}

func (t *Tmux) Rename(ctx context.Context, name session.Name, title string) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), titleOption, arg(oneLine(title))); err != nil {
		return fmt.Errorf("renaming session %s: %w", name, err)
	}
	return nil
}

// SetIssue records which GitHub issue a session was started for.
func (t *Tmux) SetIssue(ctx context.Context, name session.Name, issue int) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), issueOption, strconv.Itoa(issue)); err != nil {
		return fmt.Errorf("tagging session %s: %w", name, err)
	}
	return nil
}

// SetAccess records a session's access.
func (t *Tmux) SetAccess(ctx context.Context, name session.Name, access session.Access) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), accessOption, string(access)); err != nil {
		return fmt.Errorf("recording the access of session %s: %w", name, err)
	}
	return nil
}

// SetHosted records that a session's agent runs under agentos host.
func (t *Tmux) SetHosted(ctx context.Context, name session.Name) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), hostedOption, "1"); err != nil {
		return fmt.Errorf("tagging hosted session %s: %w", name, err)
	}
	return nil
}

// SetFork records the session a fork was forked from.
func (t *Tmux) SetFork(ctx context.Context, name session.Name, from string) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), forkOption, from); err != nil {
		return fmt.Errorf("tagging fork %s: %w", name, err)
	}
	return nil
}

// SetBranch records the branch a thread works on.
func (t *Tmux) SetBranch(ctx context.Context, name session.Name, branch string) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), branchOption, branch); err != nil {
		return fmt.Errorf("recording the branch of session %s: %w", name, err)
	}
	return nil
}

// SetParent records the thread that delegated to a session.
func (t *Tmux) SetParent(ctx context.Context, name session.Name, parent string) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), parentOption, parent); err != nil {
		return fmt.Errorf("tagging the parent of session %s: %w", name, err)
	}
	return nil
}

// SetThread records the thread a shell belongs to.
func (t *Tmux) SetThread(ctx context.Context, name session.Name, thread string) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), threadOption, thread); err != nil {
		return fmt.Errorf("tagging the thread of shell %s: %w", name, err)
	}
	return nil
}

// SetAction records the queue action a session was started with.
func (t *Tmux) SetAction(ctx context.Context, name session.Name, action string) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), actionOption, action); err != nil {
		return fmt.Errorf("recording the action of session %s: %w", name, err)
	}
	return nil
}

// SetModel records the model and effort a session's agent runs.
func (t *Tmux) SetModel(ctx context.Context, name session.Name, model, effort string) error {
	if _, err := t.run(ctx, "set-option", "-t", target(name), modelOption, model, ";", "set-option", "-t", target(name), effortOption, effort); err != nil {
		return fmt.Errorf("recording the model of session %s: %w", name, err)
	}
	return nil
}

// AttachArgv is the command that shows a shell in a terminal.
func (t *Tmux) AttachArgv(name session.Name) []string {
	return t.argv("attach-session", "-t", target(name))
}

// Redraw makes tmux paint the whole screen of the client on tty again.
func (t *Tmux) Redraw(ctx context.Context, tty string) error {
	if _, err := t.run(ctx, "refresh-client", "-t", tty); err != nil {
		return fmt.Errorf("redrawing the client on %s: %w", tty, err)
	}
	return nil
}

// Env is the environment for a process that attaches to the server.
func (t *Tmux) Env() []string { return cleanEnv() }

// oneLine keeps a title from breaking the tab-separated list: tabs and line breaks become spaces.
func oneLine(title string) string { return strings.Join(strings.Fields(title), " ") }
