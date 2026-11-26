package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const maxTreeEntries = 20000

// FileEntry is a file or folder of the Files view's tree: its path, relative to the folder and with / between names,
// and whether git ignores it.
type FileEntry struct {
	Path    string `json:"path"`
	Dir     bool   `json:"dir"`
	Ignored bool   `json:"ignored"`
}

// FileListing is entries of a folder, sorted by path, at most maxTreeEntries of them; Truncated says there are more.
type FileListing struct {
	Entries   []FileEntry `json:"entries"`
	Truncated bool        `json:"truncated"`
}

// ChatFileTree is the files of the thread's folder, its worktree else the project folder: those git tracks or does not
// ignore, and the ignored files and folders, each folder git ignores whole as one entry that ChatFileDir opens. A
// folder outside git lists everything but .git. The files are read again at each call.
func (v *Service) ChatFileTree(id string) (FileListing, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return FileListing{}, err
	}
	return v.treeIn(v.threadDir(ch), fmt.Sprintf("thread %d", ch.N))
}

// ProjectFileTree is ChatFileTree for the project's folder.
func (v *Service) ProjectFileTree(project string) (FileListing, error) {
	dir := v.projectDir(project)
	if dir == "" {
		return FileListing{}, fmt.Errorf("No project %s", project)
	}
	return v.treeIn(dir, "project "+project)
}

// ChatFileDir is what is directly in the folder at dir, relative to the thread's folder, for a folder git ignores: all
// of its entries are ignored. Only a folder inside the thread's folder, symlinks resolved, is listed.
func (v *Service) ChatFileDir(id, dir string) (FileListing, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return FileListing{}, err
	}
	return listDir(v.threadDir(ch), dir, fmt.Sprintf("thread %d", ch.N))
}

// ProjectFileDir is ChatFileDir for the project's folder.
func (v *Service) ProjectFileDir(project, dir string) (FileListing, error) {
	folder := v.projectDir(project)
	if folder == "" {
		return FileListing{}, fmt.Errorf("No project %s", project)
	}
	return listDir(folder, dir, "project "+project)
}

// treeIn is the tree of dir; of names whose folder it is, for the error.
func (v *Service) treeIn(dir, of string) (FileListing, error) {
	if dir == "" {
		return FileListing{Entries: []FileEntry{}}, nil
	}
	entries, err := v.gitEntries(v.ctx(), dir)
	if errors.Is(err, errNotGit) {
		entries, err = walkEntries(dir)
	}
	if err != nil {
		log.Printf("orchestrator: listing the files of %s in %s: %v", of, dir, err)
		return FileListing{}, fmt.Errorf("Could not list the files of %s", of)
	}
	slices.SortFunc(entries, func(a, b FileEntry) int { return strings.Compare(a.Path, b.Path) })
	listing := FileListing{Entries: entries}
	if len(entries) > maxTreeEntries {
		listing.Entries, listing.Truncated = entries[:maxTreeEntries], true
	}
	return listing, nil
}

var errNotGit = errors.New("not a git repository")

// gitEntries is the files git lists in dir, and its ignored files and folders.
func (v *Service) gitEntries(ctx context.Context, dir string) ([]FileEntry, error) {
	list := func(args ...string) ([]string, error) {
		ctx, cancel := context.WithTimeout(ctx, gitTimeout)
		defer cancel()
		out, err := v.files.run(ctx, dir, "git", append([]string{"ls-files", "-z"}, args...)...)
		if err != nil {
			if strings.Contains(err.Error(), "not a git repository") {
				return nil, errNotGit
			}
			return nil, err
		}
		text := string(out)
		if text != "" && !strings.HasSuffix(text, "\x00") {
			log.Printf("orchestrator: git listed more files in %s than the app reads; the list is cut short", dir)
			text = text[:strings.LastIndex(text, "\x00")+1]
		}
		return slices.DeleteFunc(strings.Split(text, "\x00"), func(p string) bool { return p == "" }), nil
	}
	ignored, err := list("-o", "-i", "--exclude-standard", "--directory")
	if err != nil {
		return nil, err
	}
	kept, err := list("-c", "-o", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	deleted, err := list("-d")
	if err != nil {
		return nil, err
	}
	gone := map[string]bool{}
	for _, p := range deleted {
		gone[p] = true
	}
	entries := make([]FileEntry, 0, len(kept)+len(ignored))
	for _, p := range slices.Compact(slices.Sorted(slices.Values(kept))) { // git lists a file with a merge conflict once per side
		if !gone[p] {
			entries = append(entries, FileEntry{Path: p})
		}
	}
	for _, p := range ignored {
		name, isDir := strings.CutSuffix(p, "/")
		entries = append(entries, FileEntry{Path: name, Dir: isDir, Ignored: true})
	}
	return entries, nil
}

// walkEntries is every file and folder of dir but .git, for a folder outside git, at most maxTreeEntries+1. A symlink
// is listed as a file and not followed.
func walkEntries(dir string) ([]FileEntry, error) {
	var entries []FileEntry
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir {
				return err
			}
			return nil
		}
		if p == dir {
			return nil
		}
		if d.Name() == ".git" && d.IsDir() {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return nil
		}
		entries = append(entries, FileEntry{Path: filepath.ToSlash(rel), Dir: d.IsDir()})
		if len(entries) > maxTreeEntries {
			return filepath.SkipAll
		}
		return nil
	})
	return entries, err
}

// listDir is what is directly in the folder rel of dir, but .git, as ignored entries. The folder is opened through a
// root, which refuses a path whose symlinks lead out of dir.
func listDir(dir, rel, of string) (FileListing, error) {
	outside := fmt.Errorf("This folder is outside %s's folder", of)
	if rel != "" && !filepath.IsLocal(rel) {
		return FileListing{}, outside
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil || !filepath.IsAbs(dir) {
		return FileListing{}, fmt.Errorf("The folder of %s is gone", of)
	}
	root, err := os.OpenRoot(realDir)
	if err != nil {
		return FileListing{}, fmt.Errorf("The folder of %s is gone", of)
	}
	defer root.Close()
	folder, err := root.Open(filepath.Clean(rel))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return FileListing{}, errors.New("This folder does not exist")
	case err != nil:
		return FileListing{}, outside
	}
	defer folder.Close()
	names, err := folder.ReadDir(maxTreeEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Printf("orchestrator: listing %s in the folder of %s: %v", rel, of, err)
		return FileListing{}, errors.New("Could not list this folder")
	}
	listing := FileListing{Entries: []FileEntry{}}
	for _, d := range names {
		if d.Name() == ".git" && rel == "" {
			continue
		}
		if len(listing.Entries) == maxTreeEntries {
			listing.Truncated = true
			break
		}
		listing.Entries = append(listing.Entries, FileEntry{Path: path.Join(filepath.ToSlash(rel), d.Name()), Dir: d.IsDir(), Ignored: true})
	}
	slices.SortFunc(listing.Entries, func(a, b FileEntry) int { return strings.Compare(a.Path, b.Path) })
	return listing, nil
}
