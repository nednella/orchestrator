package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/environ"
	"github.com/nednella/orchestrator/internal/thread"
)

// findAgentBrowser returns the agent-browser that threads drive their tab with: ORCHESTRATOR_AGENT_BROWSER, else the one
// the app bundle carries beside this binary, else "".
func findAgentBrowser(env []string) string {
	if p := environ.Get(env, "ORCHESTRATOR_AGENT_BROWSER"); p != "" {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return ""
	}
	p := filepath.Join(filepath.Dir(exe), "..", "Helpers", "agent-browser")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return filepath.Clean(p)
}

// AgentReady says whether threads can drive a tab of their own with agent-browser.
func (b *Browsers) AgentReady() bool { return b.Available() && b.agentBrowser != "" }

// agentDir is where the threads' agent-browser daemons keep their sockets and bindings, each as <token>.*. It is
// short and the same for every project, because a socket path may not pass 103 bytes.
func (b *Browsers) agentDir() string { return filepath.Join(b.dataDir, "agent-browser") }

// AgentEnv is what the thread's agent needs in its environment to drive its own tab of the project's browser with
// agent-browser; tmpDir is the thread's temp folder. It binds the thread to no tab first, so a command before the
// thread's window exists fails rather than drive the tab of another thread.
func (b *Browsers) AgentEnv(id, tmpDir string) ([]string, error) {
	name, err := thread.ParseName(id)
	if err != nil {
		return nil, err
	}
	if !b.AgentReady() {
		return nil, errors.New("there is no agent-browser to drive the browser with")
	}
	port := b.Port(name.Project)
	if port == 0 {
		return nil, errors.New("the project's browser has no port")
	}
	if err := os.MkdirAll(b.agentDir(), 0o700); err != nil {
		return nil, err
	}
	if err := b.writeBinding(name, ""); err != nil {
		return nil, err
	}
	return []string{
		"PATH=" + filepath.Dir(b.agentBrowser) + string(os.PathListSeparator) + environ.Get(b.env, "PATH"),
		"AGENT_BROWSER_SESSION=" + name.Token,
		"AGENT_BROWSER_SOCKET_DIR=" + b.agentDir(),
		"AGENT_BROWSER_CDP=" + strconv.Itoa(port),
		"AGENT_BROWSER_PIN_TAB=1",
		// A wait that never matches blocks for the whole timeout, 25 s by default.
		"AGENT_BROWSER_DEFAULT_TIMEOUT=5000",
		// A relative path is saved in the folder of the daemon, not the agent's, so screenshots go here by default.
		"AGENT_BROWSER_SCREENSHOT_DIR=" + tmpDir,
		// The check fetches Chrome's release notes and tells the agent to install another Chrome.
		"AGENT_BROWSER_NO_VERSION_CHECK=1",
	}, nil
}

// agentBinding is the file in which agent-browser's daemon keeps the tab a thread drives,
// <socket dir>/<session>.target (agent-browser 0.43.0). Pinned, the daemon stays on that tab and fails with tab_gone
// when it is not there, rather than take another. It reads the file only when it attaches to the browser.
//
// orchestrator writes this file because agent-browser has no command that binds a thread to a tab of a browser it shares
// without bringing Chrome in front of the owner's windows: tab <id> does, and so does --pin-tab with no binding.
type agentBinding struct {
	TargetID string `json:"targetId"`
	URL      string `json:"url"`
	Pinned   bool   `json:"pinned"`
}

func (b *Browsers) writeBinding(name thread.Name, targetID string) error {
	data, _ := json.Marshal(agentBinding{TargetID: targetID, Pinned: true})
	return atomicfile.Write(filepath.Join(b.agentDir(), name.Token+".target"), data, 0o600)
}

// bindAgent points the thread's agent-browser at the tab targetID, "" for none. It stops the thread's daemon first,
// since the daemon reads the binding only when it attaches; the agent's next command starts it again. A thread that
// never got agent-browser has nothing to bind.
func (b *Browsers) bindAgent(ctx context.Context, id, targetID string) error {
	name, ok := b.agentThread(id)
	if !ok {
		return nil
	}
	if err := b.stopAgent(ctx, name); err != nil {
		return err
	}
	return b.writeBinding(name, targetID)
}

// agentThread is the thread's name, when it has an agent-browser binding.
func (b *Browsers) agentThread(id string) (thread.Name, bool) {
	name, err := thread.ParseName(id)
	if err != nil || b.agentBrowser == "" {
		return thread.Name{}, false
	}
	_, err = os.Stat(filepath.Join(b.agentDir(), name.Token+".target"))
	return name, err == nil
}

// stopAgent stops the thread's agent-browser daemon, which leaves its tab alone.
func (b *Browsers) stopAgent(ctx context.Context, name thread.Name) error {
	ctx, cancel := context.WithTimeout(ctx, stopLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, b.agentBrowser, "close")
	cmd.Env = append(slices.Clone(b.env), "AGENT_BROWSER_SESSION="+name.Token, "AGENT_BROWSER_SOCKET_DIR="+b.agentDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stopping the agent-browser of %s: %v: %s", name, err, out)
	}
	return nil
}

// Forget closes the thread's window and removes its agent-browser files: the thread is gone.
func (b *Browsers) Forget(ctx context.Context, id string) {
	b.Close(ctx, id)
	name, ok := b.agentThread(id)
	if !ok {
		return
	}
	files, _ := filepath.Glob(filepath.Join(b.agentDir(), name.Token+".*"))
	for _, f := range files {
		_ = os.Remove(f)
	}
}
