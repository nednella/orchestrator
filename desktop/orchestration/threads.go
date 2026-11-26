package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/session"
)

// allProjects is the project the threads tool takes for every project's threads.
const allProjects = "all"

type threadsInput struct {
	Project string `json:"project"`
}

// listed is one thread as the threads tool lists it. You marks the caller's own.
type listed struct {
	ID      string         `json:"id"`
	N       int            `json:"n"`
	Title   string         `json:"title"`
	Project string         `json:"project"`
	State   session.State  `json:"state"`
	Access  session.Access `json:"access"`
	Parent  string         `json:"parent"`
	Issue   int            `json:"issue"`
	Branch  string         `json:"branch"`
	PR      int            `json:"pr"`
	You     bool           `json:"you,omitempty"`
}

// list answers the threads tool: the threads of the project the input names, of every project for all, else of the
// caller's, in the order the sidebar shows them. A deleted thread is left out.
func (c *Commands) list(_ context.Context, caller string, req ctl.Request) (string, error) {
	var in threadsInput
	if err := decode(req.Opts["input"], &in); err != nil {
		return "", err
	}
	key, err := c.projectKey(in.Project, caller)
	if err != nil {
		return "", err
	}
	out := []listed{}
	for _, v := range c.threads.List() {
		if key != allProjects && v.Project != key {
			continue
		}
		l := listed{ID: v.ID, N: v.N, Title: v.Title, Project: v.Project, State: v.State, Access: v.Access, Parent: v.Parent, Issue: v.Issue, Branch: v.Branch, You: v.ID == caller}
		if v.PR != nil {
			l.PR = v.PR.Number
		}
		out = append(out, l)
	}
	data, err := json.Marshal(struct {
		Threads []listed `json:"threads"`
	}{out})
	if err != nil {
		return "", fmt.Errorf("writing the thread list: %w", err)
	}
	return string(data), nil
}

// projectKey is the key of the project name gives, by its name in any case or its key: the caller's for "", and
// allProjects for all.
func (c *Commands) projectKey(name, caller string) (string, error) {
	switch name {
	case "":
		return session.ProjectKey(caller), nil
	case allProjects:
		return allProjects, nil
	}
	var names []string
	for _, p := range c.threads.Projects() {
		if strings.EqualFold(p.Name, name) || p.Key == name {
			return p.Key, nil
		}
		names = append(names, p.Name)
	}
	return "", fmt.Errorf("No project %q: the projects are %s", name, strings.Join(names, ", "))
}
