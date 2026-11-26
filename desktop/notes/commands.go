package notes

import (
	"context"
	"log"
	"strings"

	"github.com/nednella/orchestrator/desktop/internal/scoped"
	"github.com/nednella/orchestrator/internal/control"
)

// Asker says which project an agent command is about.
type Asker interface {
	AskerProject(req control.Request) string
}

// Commands answers the note tool.
type Commands struct {
	notes   *Notes
	asker   Asker
	project Projects
	emit    func(event string, payload any)
}

func NewCommands(n *Notes, a Asker, p Projects, emit func(string, any)) *Commands {
	return &Commands{notes: n, asker: a, project: p, emit: emit}
}

// Note adds a note to the asking session's project.
func (c *Commands) Note(_ context.Context, req control.Request) (string, error) {
	key := c.asker.AskerProject(req)
	n, err := c.notes.Add(key, strings.Join(req.Args, " "))
	if err != nil {
		return "", err
	}
	if key == c.project.Current().Key() {
		if list, err := c.notes.List(key); err == nil {
			c.emit("notes", scoped.Of(key, list))
		} else {
			log.Printf("orchestrator: %v", err)
		}
	}
	return "note saved: " + n.title(), nil
}
