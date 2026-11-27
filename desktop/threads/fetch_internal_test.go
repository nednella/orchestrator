package threads

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Two fetches into one repository at once that bring in the same branch fail, one of them with cannot lock ref. The
// clean-up's fetch and the fetch after a new worktree met so, and the clean-up was refused.
func TestFetchesIntoOneRepositoryTakeTurns(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	root := t.TempDir()
	origin, pusher := filepath.Join(root, "origin.git"), filepath.Join(root, "pusher")
	gitT(t, root, "init", "--bare", origin)
	gitT(t, dir, "remote", "add", "origin", origin)
	gitT(t, dir, "push", "origin", "HEAD:main")
	gitT(t, root, "clone", origin, pusher)
	for n := range 25 {
		if err := os.WriteFile(filepath.Join(pusher, "news"), []byte(fmt.Sprint(n)), 0o600); err != nil {
			t.Fatal(err)
		}
		gitT(t, pusher, "add", "news")
		gitT(t, pusher, "commit", "-m", fmt.Sprint(n))
		gitT(t, pusher, "push", "origin", "HEAD:main")

		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); errs[0] = l.fetchOrigin(context.Background(), dir) }()
		go func() { defer wg.Done(); _, errs[1] = l.updateBase(context.Background(), dir) }()
		wg.Wait()
		if errs[0] != nil || errs[1] != nil {
			t.Fatalf("round %d: the fetches ended with %v and %v, want both to go through", n, errs[0], errs[1])
		}
	}
}
