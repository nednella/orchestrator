package environ

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		key  string
		want string
	}{
		{"set", []string{"A=1", "B=2"}, "B", "2"},
		{"the last wins", []string{"A=1", "A=2"}, "A", "2"},
		{"missing", []string{"A=1"}, "B", ""},
		{"empty", []string{"A="}, "A", ""},
		{"a longer name does not match", []string{"AB=1"}, "A", ""},
		{"a value with =", []string{"A=b=c"}, "A", "b=c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Get(tt.env, tt.key); got != tt.want {
				t.Errorf("Get = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWith(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		set  []string
		want []string
	}{
		{"adds", []string{"A=1"}, []string{"B=2"}, []string{"A=1", "B=2"}},
		{"replaces every earlier entry", []string{"A=1", "B=2", "A=3"}, []string{"A=4"}, []string{"B=2", "A=4"}},
		{"sets an empty value", []string{"A=1"}, []string{"A="}, []string{"A="}},
		{"a longer name stays", []string{"AB=1"}, []string{"A=2"}, []string{"AB=1", "A=2"}},
		{"nothing to set", []string{"A=1"}, nil, []string{"A=1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			before := append([]string(nil), tt.env...)
			if got := With(tt.env, tt.set...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("With = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.env, before) {
				t.Errorf("With changed env to %v", tt.env)
			}
		})
	}
}

func TestLookPath(t *testing.T) {
	t.Parallel()
	bin, other := t.TempDir(), t.TempDir()
	write := func(dir, name string, mode os.FileMode) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	tool := write(bin, "tool", 0o755)
	write(bin, "plain", 0o644)
	if err := os.Mkdir(filepath.Join(bin, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := bin + string(os.PathListSeparator) + other
	tests := []struct {
		name string
		env  []string
		file string
		want string
		fail bool
	}{
		{"found on PATH", []string{"PATH=" + path}, "tool", tool, false},
		{"not executable", []string{"PATH=" + path}, "plain", "", true},
		{"a folder", []string{"PATH=" + path}, "dir", "", true},
		{"missing", []string{"PATH=" + path}, "nothing", "", true},
		{"no PATH", nil, "tool", "", true},
		{"a path", nil, tool, tool, false},
		{"a path that is missing", nil, filepath.Join(other, "tool"), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := LookPath(tt.env, tt.file)
			if tt.fail {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("err = %v, want not exist", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("LookPath = %q, %v, want %q", got, err, tt.want)
			}
		})
	}
}

func TestPathWithout(t *testing.T) {
	t.Parallel()
	a, b, c := t.TempDir(), t.TempDir(), t.TempDir()
	for dir, name := range map[string]string{a: "claude", b: "other", c: "claude"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{"PATH=" + a + ":" + b + ":" + c}
	tests := []struct {
		name     string
		commands []string
		want     string
	}{
		{"drops the folders of the command", []string{"claude"}, b},
		{"drops the folders of each command", []string{"claude", "other"}, ""},
		{"a command nobody has", []string{"nothing"}, a + ":" + b + ":" + c},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PathWithout(env, tt.commands...); got != tt.want {
				t.Errorf("PathWithout = %q, want %q", got, tt.want)
			}
		})
	}
}
