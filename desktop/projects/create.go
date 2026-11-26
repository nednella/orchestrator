package projects

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/internal/project"
)

const (
	// MaxProjectNameLength is the most characters a new project's name may have.
	MaxProjectNameLength = 200
	maxFolderNameLength  = 64
	maxFolderTries       = 100
	defaultBranch        = "main"
)

var foldedLetters = func() map[rune]string {
	m := map[rune]string{}
	for _, f := range latinFolds {
		for _, r := range f.from {
			m[r] = f.to
		}
	}
	return m
}()

// FolderName is the name of the folder a new project called name is made in: lower case plain letters and digits,
// with each run of anything else turned into one dash.
func FolderName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if to, ok := foldedLetters[r]; ok {
			b.WriteString(to)
		} else {
			b.WriteRune(r)
		}
	}
	var slug strings.Builder
	dash := false
	for _, r := range strings.ToLower(b.String()) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			slug.WriteRune(r)
			dash = false
		} else if !dash && slug.Len() > 0 {
			slug.WriteByte('-')
			dash = true
		}
	}
	out := slug.String()
	if len(out) > maxFolderNameLength {
		out = out[:maxFolderNameLength]
	}
	if out = strings.TrimRight(out, "-"); out == "" {
		return "project"
	}
	return out
}

// NewProjectOptions is what the add-project view needs to know before it shows.
type NewProjectOptions struct {
	Root        string `json:"root"`        // the folder a new project is made in
	NameMax     int    `json:"nameMax"`     // the most characters of a project's name
	StartFolder string `json:"startFolder"` // where the folder browser starts, with a trailing /
}

// NewProjectOptions reads them.
func (s *Service) NewProjectOptions() NewProjectOptions {
	start := "~/"
	if dir := s.registry.ProjectsDir(); dir != "" {
		start = strings.TrimSuffix(project.AbbreviateHome(dir), "/") + "/"
	}
	return NewProjectOptions{Root: s.registry.ProjectsRoot(), NameMax: MaxProjectNameLength, StartFolder: start}
}

// GitHubOwner is the login gh is signed in as; "" when it is not signed in. gh answers slowly, so it is no part of the
// options.
func (s *Service) GitHubOwner() string {
	ctx, cancel := context.WithTimeout(s.ctx(), run.GHTimeout)
	defer cancel()
	out, err := s.run(ctx, "", "gh", "api", "user", "--jq", ".login")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// FolderEntry is a folder inside a listed folder.
type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// FolderListing is the folders in a folder.
type FolderListing struct {
	Dir     string        `json:"dir"`     // the folder, as an absolute path with a trailing /
	Parent  string        `json:"parent"`  // its parent, with a trailing /; "" for the root
	Exists  bool          `json:"exists"`  // false when there is no such folder
	Entries []FolderEntry `json:"entries"` // the folders in it, by name
}

// resolveFolder is the absolute path of what the user typed: ~ is the home folder and a path that starts with ./ or ../
// is in the current project's folder.
func (s *Service) resolveFolder(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || path == "." || path == ".." {
		path = filepath.Join(s.sessions.Current().Dir, path)
	}
	path = project.ExpandHome(path)
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// ListFolders lists the folders in the folder at path.
func (s *Service) ListFolders(path string) (FolderListing, error) {
	dir := s.resolveFolder(path)
	listing := FolderListing{Dir: withSlash(dir), Entries: []FolderEntry{}}
	if parent := filepath.Dir(dir); parent != dir {
		listing.Parent = withSlash(parent)
	}
	entries, err := os.ReadDir(dir)
	switch {
	case errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return listing, nil
	case err != nil:
		log.Printf("orchestrator: listing %s: %v", dir, err)
		return FolderListing{}, fmt.Errorf("Cannot read %s.", dir)
	}
	listing.Exists = true
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if !e.IsDir() {
			if info, err := os.Stat(full); err != nil || !info.IsDir() {
				continue
			}
		}
		listing.Entries = append(listing.Entries, FolderEntry{Name: e.Name(), Path: full})
	}
	return listing, nil
}

func withSlash(dir string) string { return strings.TrimSuffix(dir, "/") + "/" }

// NewProject is a project CreateProject made and opened.
type NewProject struct {
	Snapshot    Snapshot `json:"snapshot"`
	Dir         string   `json:"dir"`
	CommitError string   `json:"commitError"` // why the first commit failed; "" when it worked
}

// CreateProject makes a folder called after name in the projects folder, starts a Git repository in it with a README,
// adds it as a project and makes it the current one. A first commit that fails does not stop it. A failure before the
// project is added removes the folder again.
func (s *Service) CreateProject(name string) (NewProject, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewProject{}, errors.New("Enter a project name.")
	}
	if utf8.RuneCountInString(name) > MaxProjectNameLength {
		return NewProject{}, fmt.Errorf("A project name has at most %d characters.", MaxProjectNameLength)
	}
	dir, err := s.claimFolder(s.registry.ProjectsRoot(), FolderName(name))
	if err != nil {
		return NewProject{}, err
	}
	commitError, err := s.initRepo(dir, name)
	if err != nil {
		_ = os.RemoveAll(dir)
		return NewProject{}, err
	}
	p, err := s.registry.Add(dir)
	if err != nil {
		log.Printf("orchestrator: adding %s: %v", dir, err)
		_ = os.RemoveAll(dir)
		return NewProject{}, errors.New("Failed to create the project.")
	}
	snap, err := s.SwitchProject(p.Name)
	if err != nil {
		return NewProject{}, fmt.Errorf("Created %s, but could not open it: %w", dir, err)
	}
	return NewProject{Snapshot: snap, Dir: dir, CommitError: commitError}, nil
}

// claimFolder makes the first of base, base-2, base-3 ... in root that is not there yet.
func (s *Service) claimFolder(root, base string) (string, error) {
	failed := errors.New("Failed to create the project folder.")
	if err := os.MkdirAll(root, 0o755); err != nil {
		log.Printf("orchestrator: creating %s: %v", root, err)
		return "", failed
	}
	for n := 1; n <= maxFolderTries; n++ {
		dir := filepath.Join(root, base)
		if n > 1 {
			dir = fmt.Sprintf("%s-%d", dir, n)
		}
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, os.ErrExist) {
			log.Printf("orchestrator: creating %s: %v", dir, err)
			return "", failed
		}
	}
	return "", failed
}

// initRepo starts the repository in dir and commits the README. It gives why the commit failed.
func (s *Service) initRepo(dir, name string) (commitError string, err error) {
	ctx := s.ctx()
	branch := defaultBranch
	if out, err := s.run(ctx, dir, "git", "config", "--get", "init.defaultBranch"); err == nil && strings.TrimSpace(string(out)) != "" {
		branch = strings.TrimSpace(string(out))
	}
	if _, err := s.run(ctx, dir, "git", "init", "-b", branch); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("Git isn't installed on this machine. Install Git to create a project.")
		}
		log.Printf("orchestrator: git init in %s: %v", dir, err)
		return "", errors.New("Failed to create the project folder.")
	}
	title := strings.Join(strings.Fields(name), " ")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+title+"\n"), 0o644); err != nil {
		log.Printf("orchestrator: writing the README in %s: %v", dir, err)
		return "", errors.New("Failed to create the project folder.")
	}
	for _, args := range [][]string{{"add", "README.md"}, {"commit", "-m", "Initial commit"}} {
		if _, err := s.run(ctx, dir, "git", args...); err != nil {
			return reason(err), nil
		}
	}
	return "", nil
}

// reason is what a command said when it failed, without the command's own words around it.
func reason(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, "exit status"); i >= 0 {
		if _, after, ok := strings.Cut(msg[i:], ": "); ok {
			return after
		}
	}
	return msg
}

// PublishProject makes a private GitHub repository for the project, as its origin, and pushes it. It gives the
// repository's name with its owner.
func (s *Service) PublishProject(key string) (string, error) {
	dir := s.registry.Dir(key)
	if dir == "" {
		return "", errors.New("No such project.")
	}
	ctx, cancel := context.WithTimeout(s.ctx(), 2*run.GHTimeout)
	defer cancel()
	if _, err := s.run(ctx, dir, "gh", "repo", "create", filepath.Base(dir), "--private", "--source", ".", "--push"); err != nil {
		return "", errors.New(reason(err))
	}
	repo := s.repos.FreshRepo(ctx, dir)
	if repo == "" {
		return "", errors.New("gh made the repository, but did not say its name.")
	}
	return repo, nil
}
