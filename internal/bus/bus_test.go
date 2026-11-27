package bus

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nednella/orchestrator/internal/thread"
)

func tempDir(t *testing.T) string {
	t.Helper()
	// Unix socket paths are short, so not t.TempDir().
	dir, err := os.MkdirTemp("", "aosbus")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestStateFiles(t *testing.T) {
	dir := tempDir(t)
	rec := thread.Record{Thread: "p/1", State: thread.Waiting, Event: "Notification", At: time.Now().UTC().Truncate(time.Second), Detail: "needs you", Changes: []thread.Change{{State: thread.Working, At: 1}, {State: thread.Waiting, At: 2}}}
	if got, err := ReadState(dir, "p/1"); err != nil || !reflect.DeepEqual(got, thread.Record{}) {
		t.Fatalf("ReadState of nothing = %+v, %v", got, err)
	}
	if err := WriteState(dir, rec); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadState(dir, "p/1"); err != nil || !reflect.DeepEqual(got, rec) {
		t.Errorf("ReadState = %+v, %v", got, err)
	}

	other := thread.Record{Thread: "q/2", State: thread.Working}
	if err := WriteState(dir, other); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions", "junk.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	all := ReadAll(dir, nil)
	if len(all) != 2 || !reflect.DeepEqual(all["p/1"], rec) || !reflect.DeepEqual(all["q/2"], other) {
		t.Errorf("ReadAll = %+v", all)
	}
	skipQ := func(name string, info fs.FileInfo) bool { return name == "q/2" && info.Size() > 0 }
	if got := ReadAll(dir, skipQ); len(got) != 1 || !reflect.DeepEqual(got["p/1"], rec) {
		t.Errorf("ReadAll skipping q/2 = %+v", got)
	}

	if err := RemoveState(dir, "p/1"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveState(dir, "p/1"); err != nil {
		t.Errorf("removing a missing file failed: %v", err)
	}
	if got := ReadAll(dir, nil); len(got) != 1 {
		t.Errorf("after remove: %+v", got)
	}
}

func TestDefaultDirHonoursTheOverride(t *testing.T) {
	tests := []struct {
		name    string
		env     []string
		want    string
		wantErr bool
	}{
		{"the override", []string{"HOME=/home/x", "ORCHESTRATOR_STATE_DIR=/somewhere"}, "/somewhere", false},
		{"an empty override", []string{"HOME=/home/x", "ORCHESTRATOR_STATE_DIR="}, "/home/x/.local/state/orchestrator", false},
		{"home alone", []string{"HOME=/home/x"}, "/home/x/.local/state/orchestrator", false},
		{"no home", nil, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if dir, err := DefaultDir(tt.env); (err != nil) != tt.wantErr || dir != tt.want {
				t.Errorf("DefaultDir = %q, %v, want %q, error %v", dir, err, tt.want, tt.wantErr)
			}
		})
	}
}
