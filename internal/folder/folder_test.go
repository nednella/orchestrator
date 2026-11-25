package folder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInsideResolvesLinksAndGoneParts(t *testing.T) {
	root := t.TempDir()
	tree := filepath.Join(root, "trees", "a")
	if err := os.MkdirAll(tree, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "trees"), link); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		path, dir string
		want      bool
	}{
		{tree, tree, true},
		{filepath.Join(tree, "gone", "new.txt"), tree, true},
		{filepath.Join(link, "a", "made.txt"), tree, true},
		{filepath.Join(tree, "..", "b", "x"), tree, false},
		{filepath.Join(root, "trees", "ab"), tree, false},
		{root, tree, false},
	} {
		if got := Inside(c.path, c.dir); got != c.want {
			t.Errorf("Inside(%q, %q) = %v, want %v", c.path, c.dir, got, c.want)
		}
	}
	if Canonical(filepath.Join(link, "a", "gone")) != filepath.Join(Canonical(tree), "gone") {
		t.Errorf("Canonical does not resolve a link before a part that is gone")
	}
}
