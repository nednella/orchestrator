package issues

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nednella/orchestrator/desktop/sessions"
	ctl "github.com/nednella/orchestrator/internal/control"
)

// PRs polls the pull requests of the sessions.
type PRs interface{ RefreshPRs() }

// Commands answers the orchestrator commands that work on issues.
type Commands struct {
	issues *Issues
	prs    PRs
}

func NewCommands(i *Issues, prs PRs) *Commands { return &Commands{issues: i, prs: prs} }

// Issue starts a session for each issue number given, wakes its settled one, or finds the one already running. A
// prompt is the asking session's handover; it reaches a session that starts or wakes, not one already running. A thread
// that is not Full access cannot start or wake one that is.
func (c *Commands) Issue(ctx context.Context, req ctl.Request) (string, error) {
	if len(req.Args) == 0 {
		return "", errors.New("issue needs a number")
	}
	proj, ok := c.issues.sessions.ProjectByKey(req.Project)
	if !ok {
		return "", fmt.Errorf("no project %q", req.Project)
	}
	var started, woken, running, failed []string
	for _, arg := range req.Args {
		n, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s (not a number)", arg))
			continue
		}
		s, did, err := c.issues.Start(ctx, proj, n, "", req.Opts["prompt"], sessions.StartOptions{Asker: req.Session})
		switch {
		case err != nil:
			failed = append(failed, fmt.Sprintf("#%d (%v)", n, err))
		case did == sessions.Woken && req.Opts["prompt"] != "":
			woken = append(woken, fmt.Sprintf("#%d (%d, with the prompt)", n, s.N))
		case did == sessions.Woken:
			woken = append(woken, fmt.Sprintf("#%d (%d)", n, s.N))
		case did == sessions.Kept && req.Opts["prompt"] != "":
			running = append(running, fmt.Sprintf("#%d (%d, the prompt was not given to it)", n, s.N))
		case did == sessions.Kept:
			running = append(running, fmt.Sprintf("#%d (%d)", n, s.N))
		default:
			started = append(started, fmt.Sprintf("#%d (%d)", n, s.N))
		}
	}
	var parts []string
	if len(started) > 0 {
		noun := "sessions"
		if len(started) == 1 {
			noun = "session"
		}
		parts = append(parts, fmt.Sprintf("started %d %s: %s", len(started), noun, strings.Join(started, ", ")))
	}
	if len(woken) > 0 {
		parts = append(parts, "woke the sleeping: "+strings.Join(woken, ", "))
	}
	if len(running) > 0 {
		parts = append(parts, "already running: "+strings.Join(running, ", "))
	}
	if len(failed) > 0 {
		parts = append(parts, "failed: "+strings.Join(failed, ", "))
	}
	out := strings.Join(parts, "; ")
	if len(started)+len(woken)+len(running) == 0 {
		return "", errors.New(out)
	}
	return out, nil
}

// Refresh reads the issues and the pull requests afresh.
func (c *Commands) Refresh(ctx context.Context, req ctl.Request) (string, error) {
	proj, ok := c.issues.sessions.ProjectByKey(req.Project)
	if !ok {
		return "", fmt.Errorf("no project %q", req.Project)
	}
	list, err := c.issues.List(ctx, proj, true)
	if err != nil {
		return "", err
	}
	c.issues.emitFor(proj)
	c.prs.RefreshPRs()
	return fmt.Sprintf("refreshed %d issues and the pull requests", len(list)), nil
}
