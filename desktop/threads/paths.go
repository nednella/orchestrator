package threads

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/nednella/orchestrator/internal/project"
)

// ResolveLink is the absolute, cleaned path a link in a thread's chat names: ~ is the home folder and a relative path
// starts from dir. It gives "" for an empty path, or a relative path with no absolute dir to start from.
func ResolveLink(dir, path string) string {
	switch {
	case path == "":
		return ""
	case path == "~" || strings.HasPrefix(path, "~/"):
		home, err := os.UserHomeDir()
		if err != nil || !filepath.IsAbs(home) {
			return ""
		}
		return filepath.Join(home, path[1:])
	case filepath.IsAbs(path):
		return filepath.Clean(path)
	case filepath.IsAbs(dir):
		return filepath.Join(dir, path)
	}
	return ""
}

// OpenAbsolute opens the regular file at the absolute path, symlinks resolved, for reading, and gives its real path. It
// does not look at where the file is: the caller decides that. of names who asked, for the log.
func OpenAbsolute(path, of string) (*os.File, string, error) {
	if !filepath.IsAbs(path) {
		return nil, "", errors.New("This file is not at an absolute path")
	}
	real, err := filepath.EvalSymlinks(path)
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return nil, "", errors.New("This file does not exist")
	case err != nil:
		log.Printf("orchestrator: reading %s for %s: %v", path, of, err)
		return nil, "", errCannotRead
	}
	checked, err := os.Stat(real)
	if err != nil {
		return nil, "", errCannotRead
	}
	if !checked.Mode().IsRegular() {
		return nil, "", errNotFile
	}
	// Without O_NONBLOCK a named pipe put in the file's place would hold the open up.
	f, err := os.OpenFile(real, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		log.Printf("orchestrator: opening %s for %s: %v", path, of, err)
		return nil, "", errCannotRead
	}
	if opened, err := f.Stat(); err != nil || !os.SameFile(checked, opened) {
		f.Close()
		return nil, "", errors.New("This file changed while it was opened")
	}
	return f, real, nil
}

// editorSchemes are the editors that take a file and a line as a URL, by the name of their macOS app.
var editorSchemes = map[string]string{
	"Visual Studio Code":            "vscode",
	"Visual Studio Code - Insiders": "vscode-insiders",
	"VSCodium":                      "vscodium",
	"Cursor":                        "cursor",
	"Windsurf":                      "windsurf",
	"Zed":                           "zed",
}

// OpenFileInEditor opens the file or folder at path with the editor app of the config, at line when it is above zero
// and the editor takes one. A relative path starts from the thread's folder. The path is the one the owner clicked in
// the chat, so it may be anywhere on disk, but it must exist.
func (s *Threads) OpenFileInEditor(id, path string, line int) error {
	target, err := s.chipPath(id, path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return errors.New("This file does not exist")
	}
	app := s.editor()
	args := []string{"-a", app, target}
	if scheme, ok := editorSchemes[app]; ok && line > 0 && info.Mode().IsRegular() {
		link := url.URL{Scheme: scheme, Host: "file", Path: target}
		args[2] = link.String() + ":" + strconv.Itoa(line)
	}
	return s.openApp(app, args)
}

// RevealInFinder shows the file or folder at path in Finder. A relative path starts from the thread's folder.
func (s *Threads) RevealInFinder(id, path string) error {
	target, err := s.chipPath(id, path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err != nil {
		return errors.New("This file does not exist")
	}
	ctx, cancel := context.WithTimeout(s.context(), editorTimeout)
	defer cancel()
	if _, err := s.run(ctx, "", "open", "-R", target); err != nil {
		log.Printf("orchestrator: revealing %s: %v", target, err)
		return errors.New("Finder did not open")
	}
	return nil
}

func (s *Threads) chipPath(id, path string) (string, error) {
	folder, err := s.threadFolder(id)
	if err != nil && !errors.Is(err, errFolderGone) {
		return "", err
	}
	target := ResolveLink(folder, path)
	if target == "" {
		return "", errors.New("This file does not exist")
	}
	return target, nil
}

func (s *Threads) editor() string {
	if s.editorApp != nil {
		return s.editorApp()
	}
	return project.DefaultEditorApp
}

// openApp opens the editor app with args, where args[1] is its name, and says it plainly when it fails.
func (s *Threads) openApp(app string, args []string) error {
	label := app
	if app == project.DefaultEditorApp {
		label = "VS Code"
	}
	ctx, cancel := context.WithTimeout(s.context(), editorTimeout)
	defer cancel()
	if _, err := s.run(ctx, "", "open", args...); err != nil {
		log.Printf("orchestrator: opening %s in %s: %v", args[len(args)-1], app, err)
		if strings.Contains(err.Error(), "Unable to find application") {
			return fmt.Errorf("%s is not installed", label)
		}
		return fmt.Errorf("%s did not open", label)
	}
	return nil
}
