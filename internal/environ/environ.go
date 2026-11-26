// Package environ reads and changes an environment given as a list of KEY=value entries, so a component can be given
// an environment of its own and two of them in one process never share the process's.
package environ

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Get is the value of key, the last one when the list holds it twice, or "" when it holds none.
func Get(env []string, key string) string {
	for _, kv := range slices.Backward(env) {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v
		}
	}
	return ""
}

// With is env with each entry in set in place of any earlier entry of the same name. It leaves env as it was.
func With(env []string, set ...string) []string {
	out := slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		name, _, _ := strings.Cut(kv, "=")
		return slices.ContainsFunc(set, func(s string) bool {
			setName, _, _ := strings.Cut(s, "=")
			return setName == name
		})
	})
	return append(out, set...)
}

// LookPath is the executable that file names on env's PATH, as exec.LookPath finds it on the process's.
func LookPath(env []string, file string) (string, error) {
	if strings.Contains(file, "/") {
		if executable(file) {
			return file, nil
		}
		return "", fmt.Errorf("finding %s: %w", file, fs.ErrNotExist)
	}
	for _, dir := range filepath.SplitList(Get(env, "PATH")) {
		if dir == "" {
			dir = "."
		}
		if path := filepath.Join(dir, file); executable(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("finding %s on PATH: %w", file, fs.ErrNotExist)
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

// PathWithout is env's PATH without the folders that hold any of the commands.
func PathWithout(env []string, commands ...string) string {
	dirs := slices.DeleteFunc(filepath.SplitList(Get(env, "PATH")), func(dir string) bool {
		return slices.ContainsFunc(commands, func(command string) bool { return executable(filepath.Join(dir, command)) })
	})
	return strings.Join(dirs, string(os.PathListSeparator))
}
