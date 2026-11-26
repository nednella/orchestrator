package term

import (
	"context"
	"strings"
	"testing"

	"github.com/nednella/agentos/internal/session"
)

func TestTmuxTagsCarryTheNewName(t *testing.T) {
	tmux := newTestTmux(t)
	ctx := context.Background()
	name := session.Name{Project: "demo", Token: "a1"}
	if err := tmux.NewSession(ctx, name, "title", t.TempDir(), nil, []string{"sleep", "60"}, 100, 30); err != nil {
		t.Fatal(err)
	}
	if err := tmux.SetAccess(ctx, name, session.Read); err != nil {
		t.Fatal(err)
	}
	out, err := tmux.run(ctx, "show-options", "-t", target(name))
	if err != nil {
		t.Fatal(err)
	}
	tags := 0
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "@") {
			continue
		}
		tags++
		if !strings.HasPrefix(line, "@orchestrator-") {
			t.Errorf("tag %q does not start with @orchestrator-", line)
		}
	}
	if tags < 2 {
		t.Errorf("read %d tags from %q, want the name and the access", tags, out)
	}
}
