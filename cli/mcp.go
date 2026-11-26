package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/mcp"
	"github.com/nednella/agentos/internal/version"
)

const (
	notInAThread = "agentos tools work only inside an agentos thread"
	waitSlice    = time.Minute
	// A slice's control call outlasts the slice, so the app answers before the call times out.
	waitSliceCall    = 65 * time.Second
	defaultWaitLimit = 300
	maxWaitLimit     = 3600
	// An answer that comes well before its slice ends, as while the app starts, is asked again after a pause that
	// doubles each time, up to waitPauseMost.
	waitPauseFirst = 500 * time.Millisecond
	waitPauseMost  = 8 * time.Second
)

// newMCPCmd serves the orchestration tools to the agent that started it, over stdin and stdout.
func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "mcp",
		Short:  "Serve the agentos tools over MCP (called by Claude)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			socket, err := controlSocket()
			if err != nil {
				return err
			}
			// The token comes from the environment of this process only; Claude's own environment never holds it.
			token := os.Getenv(mcp.TokenEnv)
			call := func(ctx context.Context, tool string, input json.RawMessage) (string, error) {
				if token == "" {
					return "", errors.New(notInAThread)
				}
				if tool == "wait" {
					return waitInSlices(ctx, socket, token, input)
				}
				return orchestrate(ctx, socket, token, tool, input, nil, 0)
			}
			return mcp.Serve(cmd.Context(), mcp.Info{Name: "agentos", Version: version.Version}, mcp.Tools(), call, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

// orchestrate sends one tool call as the thread the token names. It sends no session or project:
// the app takes both from the token.
func orchestrate(ctx context.Context, socket, token, tool string, input json.RawMessage, opts map[string]string, timeout time.Duration) (string, error) {
	o := map[string]string{"input": string(input)}
	for k, v := range opts {
		o[k] = v
	}
	resp, err := control.Call(ctx, socket, control.Request{
		Cmd: "orchestrate", Token: token, Args: []string{tool}, Opts: o, TimeoutMs: int(timeout.Milliseconds()),
	})
	if err != nil {
		return "", err
	}
	if !resp.OK {
		return "", errors.New(resp.Error)
	}
	return resp.Out, nil
}

// waitInSlices asks the app to wait one slice at a time until the thread is done or the limit passes,
// so no control call outlasts control.MaxTimeout.
func waitInSlices(ctx context.Context, socket, token string, input json.RawMessage) (string, error) {
	var in struct {
		TimeoutSeconds int `json:"timeout_seconds"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", errors.New("wait needs a JSON object")
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = defaultWaitLimit
	}
	if in.TimeoutSeconds > maxWaitLimit {
		return "", errors.New("timeout_seconds is at most 3600")
	}
	deadline := time.Now().Add(time.Duration(in.TimeoutSeconds) * time.Second)
	pause := waitPauseFirst
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		slice := min(waitSlice, time.Until(deadline))
		slice = max(slice, time.Millisecond)
		opts := map[string]string{"slice_ms": strconv.FormatInt(slice.Milliseconds(), 10), "until_ms": strconv.FormatInt(deadline.UnixMilli(), 10)}
		asked := time.Now()
		out, err := orchestrate(ctx, socket, token, "wait", input, opts, waitSliceCall)
		if err != nil {
			return "", err
		}
		var answer struct {
			Done bool   `json:"done"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(out), &answer); err != nil {
			return "", errors.New("the app's answer to wait was not understood")
		}
		// The app is given whole milliseconds, so a slice can end just short of the deadline.
		if answer.Done || time.Until(deadline) < time.Millisecond {
			return answer.Text, nil
		}
		if time.Since(asked) >= slice/2 {
			pause = waitPauseFirst
			continue
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(min(pause, time.Until(deadline))):
		}
		pause = min(2*pause, waitPauseMost)
	}
}
