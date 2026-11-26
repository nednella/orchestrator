package digest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/notes"
	"github.com/nednella/orchestrator/internal/project"
)

type oneProject struct{ p project.Project }

func (o oneProject) Current() project.Project { return o.p }

type noNotes struct{}

func (noNotes) Add(string, string) (notes.Note, error) {
	return notes.Note{}, errors.New("no notes here")
}
func (noNotes) List(string) ([]notes.Note, error) { return nil, nil }

func TestRunStartsBeforeItCountsTheTools(t *testing.T) {
	events := make(chan Digest, 10)
	notSignedIn := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("signed out")
	}
	m := NewManager(New(t.TempDir()), oneProject{project.Project{Name: "demo", Dir: t.TempDir()}}, noNotes{}, notSignedIn, nil,
		func(_ string, payload any) { events <- payload.(Digest) }, context.Background)
	release := make(chan struct{})
	m.walk = func(string) []string {
		<-release
		return []string{"react", "wails"}
	}

	returned := make(chan error, 1)
	go func() { returned <- m.runCurrent() }()
	select {
	case err := <-returned:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("starting a run waited for the dependency walk")
	}
	if d := <-events; !d.Running || d.Tools != 0 {
		t.Errorf("first event = %+v, want a running digest with no count yet", d)
	}

	close(release)
	if d := <-events; d.Tools != 2 {
		t.Errorf("tools = %d after the walk, want 2", d.Tools)
	}
	if d := <-events; d.Running || d.Tools != 2 {
		t.Errorf("last event = %+v, want the finished run with its count", d)
	}
}
