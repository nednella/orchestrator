package issues_test

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	chatlog "github.com/nednella/agentos/internal/chat"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

var (
	eventually = apptest.Eventually
	exists     = apptest.Exists
)

func newHarness(t *testing.T) *apptest.Harness { return apptest.New(t) }

// sent says whether the session's claude read a user message with the text.
func sent(t *testing.T, h *apptest.Harness, id, text string) bool {
	t.Helper()
	quoted, _ := json.Marshal(text)
	return slices.ContainsFunc(h.Stdin(t, id), func(line string) bool {
		return strings.HasPrefix(line, `{"type":"user"`) && strings.Contains(line, `"text":`+string(quoted))
	})
}

// drafted says whether the session's chat log holds a draft of the text.
func drafted(h *apptest.Harness, id, text string) bool {
	events, _ := h.ChatHistory(id, 0)
	return slices.ContainsFunc(events, func(e chatlog.Event) bool { return e.Kind == chatlog.KindDraft && e.Text == text })
}
