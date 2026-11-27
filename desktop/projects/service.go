package projects

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/notes"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/prompts"
)

// Threads is what the projects need of the thread list, which also knows the current project.
type Threads interface {
	Current() project.Project
	Projects() []threads.Project
	List() []threads.Thread
	Shells() []threads.Shell
	CreateBriefed(title, text, brief string) (threads.Thread, error)
	SwitchProject(name string) (project.Project, error)
}

// Notes lists a project's notes.
type Notes interface {
	List(key string) ([]notes.Note, error)
}

// Repos names the GitHub repo of a folder.
type Repos interface {
	Repo(ctx context.Context, dir string) string
	FreshRepo(ctx context.Context, dir string) string
}

// Snapshot is everything the screen needs on load.
type Snapshot struct {
	Project  threads.Project   `json:"project"`
	Projects []threads.Project `json:"projects"`
	Threads  []threads.Thread  `json:"threads"`
	Notes    []notes.Note      `json:"notes"`
	Shells   []threads.Shell   `json:"shells"`  // the current project's shells, by shell number
	Machine  string            `json:"machine"` // this Mac's name, as its owner set it
}

// Service is bound to the front end.
type Service struct {
	registry *Registry
	threads  Threads
	notes    Notes
	repos    Repos
	run      run.Runner
	stateDir string
	pickDir  func() (string, error) // "" when the user cancels
	machine  func() string
	ctx      func() context.Context
}

func NewService(r *Registry, s Threads, n Notes, repos Repos, runner run.Runner, stateDir string, pickDir func() (string, error), machine func() string, ctx func() context.Context) *Service {
	return &Service{registry: r, threads: s, notes: n, repos: repos, run: runner, stateDir: stateDir, pickDir: pickDir, machine: machine, ctx: ctx}
}

// Snapshot is everything the screen needs; call it on load.
func (s *Service) Snapshot() Snapshot {
	cur := s.threads.Current()
	repo := s.repos.Repo(s.ctx(), cur.Dir)
	projects := s.threads.Projects()
	snap := Snapshot{Projects: projects, Threads: s.threads.List(), Shells: s.threads.Shells(), Machine: s.machine()}
	snap.Project = threads.Project{Key: cur.Key(), Name: cur.Name, Dir: cur.Dir}
	for i, p := range projects {
		if p.Dir == cur.Dir {
			projects[i].Repo = repo
			snap.Project = projects[i]
		}
	}
	var err error
	if snap.Notes, err = s.notes.List(cur.Key()); err != nil {
		log.Printf("orchestrator: %v", err)
		snap.Notes = []notes.Note{}
	}
	return snap
}

// SwitchProject makes the project current; it is remembered for the next start.
func (s *Service) SwitchProject(name string) (Snapshot, error) {
	p, err := s.threads.SwitchProject(name)
	if err != nil {
		return Snapshot{}, err
	}
	s.remember(p)
	return s.Snapshot(), nil
}

func (s *Service) remember(p project.Project) {
	if err := WriteLast(s.stateDir, p.Name); err != nil {
		log.Printf("orchestrator: remembering the project: %v", err)
	}
}

// PickFolder asks for a folder and returns its path; "" when the user cancels.
func (s *Service) PickFolder() (string, error) {
	dir, err := s.pickDir()
	if err != nil {
		return "", fmt.Errorf("choosing a folder: %w", err)
	}
	return dir, nil
}

// AddProjectDir adds a folder as a project and makes it the current one. With create, a folder that is not there is
// made first.
func (s *Service) AddProjectDir(dir string, create bool) (Snapshot, error) {
	dir = s.resolveFolder(dir)
	if create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("orchestrator: creating %s: %v", dir, err)
			return Snapshot{}, fmt.Errorf("Failed to create the folder %s.", dir)
		}
	}
	p, err := s.registry.Add(dir)
	if err != nil {
		return Snapshot{}, err
	}
	return s.SwitchProject(p.Name)
}

// SetUpProject writes a basic orchestrator block into the current project's config and starts a Supervised thread that
// sets up the repository with the user. A project that has queue sections keeps its config: only the thread starts.
func (s *Service) SetUpProject() (threads.Thread, error) {
	key, err := s.configure()
	if err != nil {
		return threads.Thread{}, err
	}
	if s.registry.project(key).NeedsSetup() {
		if err := s.registry.SetUp(key); err != nil {
			return threads.Thread{}, err
		}
	}
	return s.threads.CreateBriefed("orchestrator setup", prompts.SetupStart(), prompts.SetupBrief())
}

// configure is the key of the config's entry for the current project's folder, adding the entry when the folder is
// not in the config: the folder the app started in may not be. Another machine's project of the same name is not it.
func (s *Service) configure() (string, error) {
	cur := s.threads.Current()
	if p, ok := s.registry.ByDir(cur.Dir); ok {
		return p.Key(), nil
	}
	p, err := s.registry.Add(cur.Dir)
	if err != nil {
		return "", err
	}
	if _, err := s.SwitchProject(p.Name); err != nil {
		return "", err
	}
	return p.Key(), nil
}
