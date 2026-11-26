package apptest

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// A test that runs an app waits for one of a few slots first, and gives it back when it ends. The slots are files
// under the temp folder that a lock marks, so the test processes of one `go test ./...` share them: the packages run at
// the same time, and the apps of all of them at once would make each test wait on the machine for what takes it a
// moment alone, which the tests that wait for a reply give up on.
const slotWait = time.Minute

// takeSlot waits for a free slot, and gives it back when the test ends. After slotWait it goes on without one, so a
// test that makes a second app while it holds the first cannot hold the others up for good.
func takeSlot(t *testing.T) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "orchestrator-apptest-slots")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Logf("apptest: no folder for the slots: %v", err)
		return
	}
	slots := max(runtime.NumCPU()*3/4, 2)
	deadline := time.Now().Add(slotWait)
	for {
		for _, i := range rand.Perm(slots) {
			f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("slot-%d", i)), os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				t.Logf("apptest: opening a slot: %v", err)
				return
			}
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				f.Close()
				continue
			}
			t.Cleanup(func() { f.Close() }) // closing the file gives the lock up
			return
		}
		if time.Now().After(deadline) {
			t.Logf("apptest: no free slot in %s; going on without one", slotWait)
			return
		}
		time.Sleep(time.Duration(20+rand.IntN(40)) * time.Millisecond)
	}
}
