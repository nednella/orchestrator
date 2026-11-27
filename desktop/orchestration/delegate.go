package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/nednella/orchestrator/desktop/threads"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/thread"
)

// delegateInput is what a call of delegate asks for, as the tool's schema has it.
type delegateInput struct {
	Task    string `json:"task"`
	Title   string `json:"title"`
	Access  string `json:"access"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
	Project string `json:"project"`
	Where   string `json:"where"`
	Issue   int    `json:"issue"`
}

// started is what delegate answers: the child it started.
type started struct {
	ID      string        `json:"id"`
	N       int           `json:"n"`
	Title   string        `json:"title"`
	Project string        `json:"project"`
	Access  thread.Access `json:"access"`
	Where   string        `json:"where"`
	Dir     string        `json:"dir"`
	Branch  string        `json:"branch"`
}

// delegate starts a child of the caller for the task the input gives, in the project the input names, else the
// caller's. The child gets the caller's access unless the input asks for less, and works in a folder when it is Read,
// the caller's in the caller's own project, else in a worktree of its own. A side chat starts none: the child would
// outlive it, since a side chat goes with its thread.
func (c *Commands) delegate(ctx context.Context, caller string, req ctl.Request) (string, error) {
	var in delegateInput
	if err := decode(req.Opts["input"], &in); err != nil {
		return "", err
	}
	if c.threads.SideChat(caller) {
		return "", errors.New("A side chat cannot delegate")
	}
	access := c.threads.AccessOf(caller)
	if in.Access != "" {
		access = thread.Access(in.Access)
	}
	where := in.Where
	if where == "" {
		where = threads.WhereWorktree
		if access == thread.Read {
			where = threads.WhereFolder
		}
	}
	child, err := c.threads.CreateChild(ctx, threads.ChildOptions{
		Parent: caller, Task: in.Task, Title: in.Title, Access: access,
		Model: project.Model{Model: in.Model, Effort: in.Effort}, Project: in.Project, Where: where, Issue: in.Issue,
	})
	if err != nil {
		return "", err
	}
	out := started{ID: child.ID, N: child.N, Title: child.Title, Project: child.Project, Access: child.Access, Where: where, Branch: child.Branch}
	if ch, err := c.threads.ChatOf(child.ID); err == nil {
		out.Dir = ch.Dir
	}
	data, err := json.Marshal(out)
	return string(data), err
}

// decode reads a tool's input, which must be a JSON object with no field its schema lacks.
func decode(input string, v any) error {
	d := json.NewDecoder(strings.NewReader(input))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return errors.New("The tool's input is not what its schema asks for: " + err.Error())
	}
	return nil
}
