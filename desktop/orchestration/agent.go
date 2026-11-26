package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/session"
)

const (
	maxPrompt  = 6 << 10  // the most a prompt for a new thread may hold
	maxNote    = 64 << 10 // the most a note may hold
	maxCaption = 1 << 10  // the most a caption of evidence may hold
)

// agentTools are the agent tools, each with the control command it sends: the one its agentos command sent.
func agentTools() map[string]string {
	return map[string]string{"note": "note", "issue": "issue", "new_thread": "new", "show": "show", "track": "track", "refresh": "refresh", "browser": "browser"}
}

// agentTool runs the agent tool name as the caller, through the handler its agentos command reached.
func (c *Commands) agentTool(name string) tool {
	return func(ctx context.Context, caller string, req ctl.Request) (string, error) {
		r, err := agentRequest(name, json.RawMessage(req.Opts["input"]), caller, session.ProjectKey(caller))
		if err != nil {
			return "", err
		}
		if r.Cmd == "new" {
			access, err := c.newThreadAccess(caller, r.Opts["access"])
			if err != nil {
				return "", err
			}
			r.Opts["access"] = string(access)
		}
		resp := c.router.Answer(ctx, r)
		if !resp.OK {
			return "", errors.New(resp.Error)
		}
		return resp.Out, nil
	}
}

// newThreadAccess is the access of the thread new_thread starts: the one asked for, else the caller's, and never one
// that ranks above the caller's.
func (c *Commands) newThreadAccess(caller, asked string) (session.Access, error) {
	has := c.threads.AccessOf(caller)
	if asked == "" {
		return has, nil
	}
	want, err := session.ParseAccess(asked)
	if err != nil {
		return "", err
	}
	if want.Rank() > has.Rank() {
		return "", fmt.Errorf("A thread with %s access cannot start a thread with %s access", has, want)
	}
	return want, nil
}

// agentRequest is the control request that an agent tool's input makes: the args and options its agentos command
// sent, asked by the caller in project. Nothing in the input can name another thread or project.
func agentRequest(tool string, input json.RawMessage, caller, project string) (ctl.Request, error) {
	cmd, ok := agentTools()[tool]
	if !ok {
		return ctl.Request{}, fmt.Errorf("agentos has no tool %q", tool)
	}
	args, opts, err := agentArgs(tool, string(input))
	if err != nil {
		return ctl.Request{}, err
	}
	if len(opts["prompt"]) > maxPrompt {
		return ctl.Request{}, fmt.Errorf("The prompt is %d bytes, more than the %d a thread takes: put the long parts in a file and give its path", len(opts["prompt"]), maxPrompt)
	}
	return ctl.Request{Cmd: cmd, Session: caller, Project: project, Args: args, Opts: opts}, nil
}

// agentArgs reads the input of the agent tool into the args and options its handler reads.
func agentArgs(tool, input string) ([]string, map[string]string, error) {
	switch tool {
	case "note":
		var in struct {
			Text string `json:"text"`
		}
		if err := decode(input, &in); err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(in.Text) == "" {
			return nil, nil, errors.New("The note is empty")
		}
		if len(in.Text) > maxNote {
			return nil, nil, fmt.Errorf("The note is %d bytes, more than the %d a note takes", len(in.Text), maxNote)
		}
		return []string{in.Text}, map[string]string{}, nil
	case "issue":
		var in struct {
			Number int    `json:"number"`
			Prompt string `json:"prompt"`
		}
		if err := decode(input, &in); err != nil {
			return nil, nil, err
		}
		if in.Number < 1 {
			return nil, nil, errors.New("number is an issue's number, 1 or more")
		}
		return []string{strconv.Itoa(in.Number)}, map[string]string{"prompt": strings.TrimSpace(in.Prompt)}, nil
	case "new_thread":
		var in struct {
			Title  string `json:"title"`
			Prompt string `json:"prompt"`
			Access string `json:"access"`
		}
		if err := decode(input, &in); err != nil {
			return nil, nil, err
		}
		title, prompt := strings.TrimSpace(in.Title), strings.TrimSpace(in.Prompt)
		if title == "" || prompt == "" {
			return nil, nil, errors.New("The new thread needs a title and a prompt")
		}
		return []string{title}, map[string]string{"prompt": prompt, "access": in.Access}, nil
	case "show":
		var in struct {
			File    string `json:"file"`
			Text    string `json:"text"`
			Caption string `json:"caption"`
		}
		if err := decode(input, &in); err != nil {
			return nil, nil, err
		}
		opts := map[string]string{"text": in.Text, "caption": in.Caption}
		switch {
		case len(in.Caption) > maxCaption:
			return nil, nil, fmt.Errorf("The caption is %d bytes, more than the %d a caption takes", len(in.Caption), maxCaption)
		case (in.File == "") == (in.Text == ""):
			return nil, nil, errors.New("Give file or text, not both")
		case in.Text != "":
			return nil, opts, nil
		case !filepath.IsAbs(in.File):
			return nil, nil, fmt.Errorf("%s is not an absolute path", in.File)
		}
		return []string{filepath.Clean(in.File)}, opts, nil
	case "track":
		var in struct {
			Branch string `json:"branch"`
		}
		if err := decode(input, &in); err != nil {
			return nil, nil, err
		}
		branch := strings.TrimSpace(in.Branch)
		if branch == "" {
			return nil, nil, errors.New("track needs a branch")
		}
		return nil, map[string]string{"branch": branch}, nil
	case "refresh":
		return nil, map[string]string{}, decode(input, &struct{}{})
	case "browser":
		return browserArgs(input)
	}
	return nil, nil, fmt.Errorf("agentos has no tool %q", tool)
}

// browserArgs reads the browser tool's input into the words and flags the browser command reads.
func browserArgs(input string) ([]string, map[string]string, error) {
	var in struct {
		Action  string `json:"action"`
		URL     string `json:"url"`
		Front   bool   `json:"front"`
		Caption string `json:"caption"`
		Full    bool   `json:"full"`
	}
	if err := decode(input, &in); err != nil {
		return nil, nil, err
	}
	opts := map[string]string{"caption": in.Caption}
	if in.Front {
		opts["front"] = "1"
	}
	if in.Full {
		opts["full"] = "1"
	}
	switch in.Action {
	case "open":
		url := strings.TrimSpace(in.URL)
		if url == "" {
			return nil, nil, errors.New("open needs a url")
		}
		return []string{in.Action, url}, opts, nil
	case "screenshot", "close":
		return []string{in.Action}, opts, nil
	}
	return nil, nil, fmt.Errorf("action is open, screenshot or close, not %q", in.Action)
}
