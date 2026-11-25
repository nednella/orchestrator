package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/nednella/agentos/desktop/sessions"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
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
	ID      string         `json:"id"`
	N       int            `json:"n"`
	Title   string         `json:"title"`
	Project string         `json:"project"`
	Access  session.Access `json:"access"`
	Where   string         `json:"where"`
	Dir     string         `json:"dir"`
	Branch  string         `json:"branch"`
}

// delegate starts a child of the caller for the task the input gives. The child gets the caller's access unless the
// input asks for less, and works in the caller's folder when it is Read, else in a worktree of its own.
func (c *Commands) delegate(_ context.Context, caller string, req ctl.Request) (string, error) {
	var in delegateInput
	if err := decode(req.Opts["input"], &in); err != nil {
		return "", err
	}
	access := c.threads.AccessOf(caller)
	if in.Access != "" {
		access = session.Access(in.Access)
	}
	where := in.Where
	if where == "" {
		where = sessions.WhereWorktree
		if access == session.Read {
			where = sessions.WhereFolder
		}
	}
	if in.Project != "" && !c.isOwnProject(caller, in.Project) {
		return "", errors.New("Delegating to another project is not supported yet")
	}
	child, err := c.threads.CreateChild(sessions.ChildOptions{
		Parent: caller, Task: in.Task, Title: in.Title, Access: access,
		Model: project.Model{Model: in.Model, Effort: in.Effort}, Where: where, Issue: in.Issue,
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

// isOwnProject says whether name, a project's name or key, names the caller's project.
func (c *Commands) isOwnProject(caller, name string) bool {
	key := session.ProjectKey(caller)
	for _, p := range c.threads.Projects() {
		if p.Key == key {
			return strings.EqualFold(p.Name, name) || p.Key == name
		}
	}
	return name == key
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
