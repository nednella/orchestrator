package evidence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ctl "github.com/nednella/agentos/internal/control"
)

type fixedAsker struct{}

func (fixedAsker) AskerSession(ctl.Request) (string, error) { return "main/3", nil }

type fixedRoots []string

func (r fixedRoots) ShowRoots(string) []string { return r }

func TestShowRefusesAFileSwappedForALinkAfterTheCheck(t *testing.T) {
	tests := []struct {
		name    string
		swap    func(t *testing.T, path, outside string)
		wantErr string
	}{
		{"nothing swapped", func(*testing.T, string, string) {}, ""},
		{"a link to a file outside", func(t *testing.T, path, outside string) {
			t.Helper()
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
		}, "is not a file"},
		{"another file in its place", func(t *testing.T, path, outside string) {
			t.Helper()
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("other"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "changed while it was read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, outside := t.TempDir(), filepath.Join(t.TempDir(), "secret.txt")
			if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(root, "a.txt")
			if err := os.WriteFile(file, []byte("inside"), 0o600); err != nil {
				t.Fatal(err)
			}
			store := New(t.TempDir(), &events{})
			c := NewCommands(store, fixedAsker{}, fixedRoots{root}, Changes{Emit: func(string, any) {}, Touch: func() {}})
			c.beforeOpen = func(path string) { tt.swap(t, path, outside) }
			_, err := c.Show(context.Background(), ctl.Request{Cmd: "show", Args: []string{file}})
			items := store.List("main/3")
			if tt.wantErr == "" {
				if err != nil || len(items) != 1 || items[0].Text != "inside" {
					t.Fatalf("Show = %v, items %+v; want the file filed", err, items)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Show = %v, want %q", err, tt.wantErr)
			}
			for _, it := range items {
				if strings.Contains(it.Text, "secret") {
					t.Errorf("the outside file was filed: %+v", it)
				}
			}
			if len(items) != 0 {
				t.Errorf("items = %+v, want none", items)
			}
		})
	}
}
