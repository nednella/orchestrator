// Package app wires the services of the desktop app together.
package app

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nednella/orchestrator/desktop/projects"
	"github.com/nednella/orchestrator/desktop/stats"
	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/environ"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/session"
	"github.com/nednella/orchestrator/internal/term"
)

const defaultTmuxSocket = "orchestrator"

// Config is what the app reads from the config file and the environment.
type Config struct {
	Registry     *projects.Registry
	Project      project.Project
	StateDir     string
	Machine      string // names this machine's event files in a data folder other machines share
	DataDir      string // notes, evidence and stats: may be a synced folder
	LocalDir     string // PR tracking and the clean-up log: never synced
	DataDirFixed bool   // ORCHESTRATOR_DEV_DATA_DIR sets the data folder, so the settings cannot change it
	Tmux         *term.Tmux
	Agents       agent.Registry
	Binary       string // this app's own binary: hosted agents run under its host command
	Env          []string
}

// Load reads the config file and the process's environment.
func Load() (Config, error) { return LoadEnv(os.Environ()) }

// LoadEnv reads the config file and the environment env, which the app and the services it starts keep in place of the
// process's. The project is the one holding ORCHESTRATOR_DIR or the current folder. An app started from Finder
// (folder "/") gets the first configured project, else the home folder.
func LoadEnv(env []string) (Config, error) {
	home := environ.Get(env, "HOME")
	if home == "" {
		return Config{}, errors.New("finding home dir: $HOME is not defined")
	}
	path := configPath(env, home)
	cfg, err := project.Load(path)
	if err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}
	stateDir, err := bus.DefaultDir(env)
	if err != nil {
		return Config{}, err
	}
	machine, err := machineID(stateDir)
	if err != nil {
		return Config{}, err
	}
	dataDir := cmp.Or(cfg.DataDir, filepath.Join(home, ".local", "share", "orchestrator"))
	localDir := filepath.Join(home, ".local", "share", "orchestrator")
	override := environ.Get(env, "ORCHESTRATOR_DEV_DATA_DIR")
	if override != "" {
		dataDir, localDir = override, override
	}
	dir := environ.Get(env, "ORCHESTRATOR_DIR")
	if dir == "" {
		if dir, err = os.Getwd(); err != nil {
			return Config{}, fmt.Errorf("finding the current folder: %w", err)
		}
	}
	current := project.Project{}
	if dir == "/" {
		last := projects.ReadLast(stateDir)
		here := slices.DeleteFunc(slices.Clone(cfg.Projects), func(p project.Project) bool { return !p.Present() })
		switch i := slices.IndexFunc(here, func(p project.Project) bool { return p.Name == last }); {
		case i >= 0:
			current = here[i]
		case len(here) > 0:
			current = here[0]
		default:
			current = cfg.Resolve(home)
		}
	} else {
		current = cfg.Resolve(dir)
	}
	socket := environ.Get(env, "ORCHESTRATOR_TMUX_SOCKET")
	if socket == "" {
		socket = defaultTmuxSocket
	}
	tmux, err := term.NewTmux(socket, stateDir)
	if err != nil {
		return Config{}, err
	}
	tmux = tmux.WithEnv(env)
	binary, err := os.Executable()
	if err != nil {
		return Config{}, fmt.Errorf("finding the app's own binary: %w", err)
	}
	if real, err := filepath.EvalSymlinks(binary); err == nil {
		binary = real
	}
	return Config{Registry: projects.NewRegistry(path, cfg), Project: current, StateDir: stateDir, Machine: machine, DataDir: dataDir, LocalDir: localDir, DataDirFixed: override != "", Tmux: tmux, Agents: agent.Registry{Dirs: slices.Compact([]string{dataDir, localDir}), Env: env}, Binary: binary, Env: env}, nil
}

// ConfigPath is the config file: ORCHESTRATOR_CONFIG, else the one in home's config folder.
func ConfigPath(home string) string { return configPath(os.Environ(), home) }

func configPath(env []string, home string) string {
	return cmp.Or(environ.Get(env, "ORCHESTRATOR_CONFIG"), filepath.Join(home, ".config", "orchestrator", "config.yaml"))
}

// machineID is this machine's id, made on first use and kept in the state folder, which is never synced.
func machineID(stateDir string) (string, error) {
	path := filepath.Join(stateDir, "machine-id")
	b, err := os.ReadFile(path)
	if id := strings.TrimSpace(string(b)); err == nil && stats.ValidMachine(id) {
		return id, nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("reading machine id: %w", err)
	}
	id := session.NewToken()
	if err := atomicfile.Write(path, []byte(id+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("saving machine id: %w", err)
	}
	return id, nil
}
