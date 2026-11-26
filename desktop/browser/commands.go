package browser

import (
	"context"
	"errors"
	"fmt"

	"github.com/nednella/orchestrator/desktop/evidence"
	"github.com/nednella/orchestrator/internal/control"
)

// Asker checks which session a command comes from.
type Asker interface {
	AskerSession(req control.Request) (string, error)
}

// Commands answers the browser tool.
type Commands struct {
	browsers *Browsers
	asker    Asker
	store    *evidence.Store
	changes  evidence.Changes
}

func NewCommands(b *Browsers, a Asker, s *evidence.Store, c evidence.Changes) *Commands {
	return &Commands{browsers: b, asker: a, store: s, changes: c}
}

// Browser runs one browser action (open, screenshot, close) for the asking session.
func (c *Commands) Browser(ctx context.Context, req control.Request) (string, error) {
	if len(req.Args) == 0 {
		return "", errors.New("browser needs an action: open, screenshot or close")
	}
	sub, args := req.Args[0], req.Args[1:]
	if sub != "open" && sub != "screenshot" && sub != "close" {
		return "", fmt.Errorf("unknown browser action %q: it is open, screenshot or close", sub)
	}
	id, err := c.asker.AskerSession(req)
	if err != nil {
		return "", err
	}
	if sub == "close" {
		c.browsers.Close(ctx, id)
		return "closed your window", nil
	}
	if !c.browsers.Available() {
		return "", errNoBrowser
	}
	if sub == "open" {
		if len(args) < 1 {
			return "", errors.New("open needs a url")
		}
		return c.browsers.AgentOpen(ctx, id, args[0], req.Opts["front"] != "")
	}
	if _, err := c.browsers.tabFor(ctx, id); err != nil {
		return "", err
	}
	png, err := c.browsers.Screenshot(ctx, id, req.Opts["full"] != "")
	if err != nil {
		return "", err
	}
	item, err := c.store.AddImage(id, png, req.Opts["caption"], "agent")
	if err != nil {
		return "", err
	}
	c.changes.Changed(c.store, id, true)
	return c.store.File(item.URL), nil
}
