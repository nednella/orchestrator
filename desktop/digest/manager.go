package digest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/internal/scoped"
	"github.com/nednella/orchestrator/desktop/notes"
	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/util"
)

// Projects knows the current project.
type Projects interface{ Current() project.Project }

// Notes saves digest items as notes.
type Notes interface {
	Add(key, text string) (notes.Note, error)
	List(key string) ([]notes.Note, error)
}

// Manager runs the digests of the current project.
type Manager struct {
	digests *Digests
	project Projects
	notes   Notes
	run     run.EnvRunner
	emit    func(event string, payload any)
	ctx     func() context.Context
	walk    func(dir string) []string // lists the project's dependencies
	first   time.Duration             // wait before the first automatic check
	tick    time.Duration             // between checks after that
}

// NewManager runs the digests with run in the project's folder.
func NewManager(d *Digests, p Projects, n Notes, runner run.EnvRunner, emit func(string, any), ctx func() context.Context) *Manager {
	return &Manager{digests: d, project: p, notes: n, run: runner, emit: emit, ctx: ctx, walk: dependencies, first: time.Minute, tick: time.Hour}
}

// SetLoop sets how long to wait before the first automatic check, and between checks.
func (m *Manager) SetLoop(first, tick time.Duration) { m.first, m.tick = first, tick }

func (m *Manager) emitNotes(key string) {
	if list, err := m.notes.List(key); err == nil {
		m.emit("notes", scoped.Of(key, list))
	}
}

func (m *Manager) view() Digest {
	cur := m.project.Current()
	return m.digests.View(cur.Key(), cur.DigestOn())
}

func (m *Manager) emitDigest(key string) {
	if cur := m.project.Current(); cur.Key() == key {
		m.emit("digest", m.digests.View(key, cur.DigestOn()))
	}
}

// runCurrent starts the current project's digest run in the background.
func (m *Manager) runCurrent() error { return m.start(m.project.Current()) }

func (m *Manager) start(proj project.Project) error {
	key := proj.Key()
	if err := m.digests.Begin(key, time.Now()); err != nil {
		return err
	}
	m.emitDigest(key)
	go func() {
		ctx, cancel := context.WithTimeout(m.ctx(), digestTimeout)
		defer cancel()
		deps := m.walk(proj.Dir)
		m.digests.SetTools(key, len(deps))
		m.emitDigest(key)
		err := m.runAgent(ctx, key, deps)
		failure := ""
		switch {
		case ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded):
			failure = "the run took longer than 10 minutes"
		case err != nil:
			failure = util.FirstLine(err.Error())
		}
		m.digests.Finish(key, time.Now(), failure)
		m.emitDigest(key)
	}()
	return nil
}

// available returns the first agent that is installed and signed in. Checking spends no request.
func (m *Manager) available(ctx context.Context) (agent.Headless, error) {
	var tried []string
	for _, a := range agent.Headlesses {
		if _, err := m.run(ctx, "", os.Environ(), a.Check[0], a.Check[1:]...); err == nil {
			return a, nil
		} else if ctx.Err() != nil {
			return agent.Headless{}, err
		}
		tried = append(tried, a.Name)
	}
	return agent.Headless{}, fmt.Errorf("no agent is installed and signed in (tried %s)", strings.Join(tried, ", "))
}

// runAgent runs the digest prompt in an empty folder, with the few tools the digest needs and
// only the environment the agent needs: the run reads the web, so it must not see the project or its secrets. The
// agent hands in its items as a JSON array that ends its reply.
func (m *Manager) runAgent(ctx context.Context, key string, deps []string) error {
	a, err := m.available(ctx)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "orchestrator-digest")
	if err != nil {
		return fmt.Errorf("making the digest folder: %w", err)
	}
	defer os.RemoveAll(dir)
	argv := a.Argv(prompts.Digest(deps))
	reply, err := m.run(ctx, dir, a.Env(os.Environ()), argv[0], argv[1:]...)
	if err != nil {
		return err
	}
	items, err := parseItems(string(reply))
	if err != nil {
		return err
	}
	added := 0
	for _, item := range items {
		if _, err := m.digests.Add(key, item); err == nil {
			added++
		}
	}
	if len(items) > 0 && added == 0 {
		return errors.New("the reply held no item with a title and a web link")
	}
	return nil
}

// parseItems reads the items from the JSON array that ends the agent's reply, which may sit in a code fence, after
// some words. It reads at most digestPerRun.
func parseItems(reply string) ([]DigestItem, error) {
	text := strings.TrimSpace(reply)
	if body, ok := strings.CutSuffix(text, "```"); ok {
		if i := strings.LastIndex(body, "```"); i >= 0 {
			text = strings.TrimSpace(strings.TrimPrefix(body[i+3:], "json"))
		}
	}
	if !strings.HasPrefix(text, "[") {
		if i := strings.LastIndex(text, "\n["); i >= 0 {
			text = strings.TrimSpace(text[i:])
		}
	}
	var found []struct {
		Title  string `json:"title"`
		Why    string `json:"why"`
		URL    string `json:"url"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(text), &found); err != nil {
		return nil, errors.New("the reply did not end in a JSON array of items")
	}
	items := make([]DigestItem, 0, min(len(found), digestPerRun))
	for _, f := range found[:min(len(found), digestPerRun)] {
		items = append(items, DigestItem{Title: f.Title, Why: f.Why, URL: f.URL, Source: f.Source})
	}
	return items, nil
}

// Loop starts the current project's digest when it is due, checking first after m.first, then every m.tick, until ctx ends.
func (m *Manager) Loop(ctx context.Context) {
	timer := time.NewTimer(m.first)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if cur := m.project.Current(); cur.DigestOn() && m.digests.Due(cur.Key(), time.Now()) {
				_ = m.start(cur)
			}
			timer.Reset(m.tick)
		}
	}
}

func (m *Manager) toNote(itemID string) (notes.Note, error) {
	key := m.project.Current().Key()
	item, ok := m.digests.Item(key, itemID)
	if !ok {
		return notes.Note{}, errors.New("no such digest item")
	}
	n, err := m.notes.Add(key, item.Title+"\n\n"+item.Why+"\n"+item.URL)
	if err != nil {
		return notes.Note{}, err
	}
	if err := m.digests.Update(key, itemID, func(it *DigestItem) { it.NoteID = n.ID }); err != nil {
		return notes.Note{}, err
	}
	m.emitNotes(key)
	m.emitDigest(key)
	return n, nil
}

func (m *Manager) dismiss(itemID string) error {
	key := m.project.Current().Key()
	if err := m.digests.Dismiss(key, itemID); err != nil {
		return err
	}
	m.emitDigest(key)
	return nil
}
