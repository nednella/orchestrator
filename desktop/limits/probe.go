package limits

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/host"
)

const (
	// probeTimeout bounds a probe's run: Claude answers get_usage in about 5 s. With waitDelay the whole probe ends
	// within 20 s.
	probeTimeout = 15 * time.Second
	probeRequest = "aos-usage"
	// waitDelay is how long a stopped probe's pipes may stay open.
	waitDelay = 5 * time.Second
)

// probeArgv is Claude in stream-json with no MCP server, no tool and no user setting, so no hook of the user's or a
// plugin's runs. The probe sends it no user message, so nothing reaches the model and it costs no tokens.
var probeArgv = []string{"claude", "-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--strict-mcp-config", "--tools", "", "--setting-sources", "project,local"}

// Starter starts argv in dir and gives its stdin, its stdout and a wait for its exit. It stops the command when ctx
// ends.
type Starter func(ctx context.Context, dir string, argv []string) (stdin io.WriteCloser, stdout io.Reader, wait func() error, err error)

// Exec is the Starter that runs the command, with only the environment Claude needs to start and find its login, in a
// process group of its own that is killed when ctx ends. wait kills what is left of the group once the command has
// exited, so nothing it started outlives it; its error names the first line of the command's stderr.
func Exec(ctx context.Context, dir string, argv []string) (io.WriteCloser, io.Reader, func() error, error) {
	if len(argv) == 0 {
		return nil, nil, nil, errors.New("start a probe: no command given")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(agent.ClaudeHeadless.Env(os.Environ()), "TMPDIR="+dir, "CLAUDE_CODE_TMPDIR="+dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = waitDelay
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	wait := func() error {
		err := cmd.Wait()
		// The group outlives its leader while a process the command started runs, also one that left its pipes.
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err != nil {
			first, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
			return fmt.Errorf("%s: %w: %s", argv[0], err, first)
		}
		return nil
	}
	return stdin, stdout, wait, nil
}

// probe starts Claude in dir, emptied first, sends it only the get_usage control request and returns its answer. It
// reads Claude's lines until the answer, for at most 15 s, then closes Claude's stdin, waits for its exit and empties
// dir again. Every probe runs in the same folder, so a Claude Code that keeps a record of each folder it ran in keeps
// one. A Claude that does not know get_usage gives host.ErrNoUsage.
func probe(ctx context.Context, start Starter, dir string) (json.RawMessage, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("empty the probe's folder: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("make the probe's folder: %w", err)
	}
	defer os.RemoveAll(dir)
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	stdin, stdout, wait, err := start(ctx, dir, probeArgv)
	if err != nil {
		return nil, fmt.Errorf("start the usage probe: %w", err)
	}
	answered := make(chan answer, 1)
	go func() { answered <- readAnswer(stdout) }()
	var a answer
	if _, err := stdin.Write(chat.GetUsage(probeRequest)); err != nil {
		a.err = fmt.Errorf("ask the usage probe: %w", err)
	} else {
		select {
		case a = <-answered:
		case <-ctx.Done():
			a.err = fmt.Errorf("the usage probe did not answer: %w", ctx.Err())
		}
	}
	stdin.Close()
	if a.err != nil {
		// Claude ends at the close of its stdin; one that has not answered is stopped, not waited on.
		cancel()
	}
	exited := wait()
	if a.err == nil && a.raw == nil && exited != nil {
		a.err = exited
	}
	return a.raw, a.err
}

type answer struct {
	raw json.RawMessage
	err error
}

// readAnswer reads Claude's lines until its reply to the probe's request. Lines that are not JSON are skipped.
func readAnswer(stdout io.Reader) answer {
	r := bufio.NewReader(stdout)
	for {
		line, err := r.ReadBytes('\n')
		if events, perr := chat.Parse(line); perr == nil {
			for _, e := range events {
				if e.Kind == chat.KindReply && e.ID == probeRequest {
					return replied(e)
				}
			}
		}
		if err != nil {
			return answer{err: fmt.Errorf("the usage probe ended without an answer: %w", err)}
		}
	}
}

func replied(e chat.Event) answer {
	switch {
	case e.Error && host.UnknownRequest(e.Text):
		return answer{err: fmt.Errorf("the usage probe: %w", host.ErrNoUsage)}
	case e.Error:
		return answer{err: fmt.Errorf("the usage probe: %s", cmp.Or(e.Text, "Claude did not report its usage"))}
	case len(e.Input) == 0:
		return answer{err: errors.New("the usage probe's answer was empty")}
	}
	return answer{raw: e.Input}
}
