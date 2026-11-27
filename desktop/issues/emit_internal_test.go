package issues

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/scoped"
	"github.com/nednella/orchestrator/internal/project"
)

// endingThreads says issue 7 has a thread the first time it is asked and none after.
type endingThreads struct {
	Threads
	asked atomic.Int32
}

func (e *endingThreads) IssueThreads() map[int]string {
	if e.asked.Add(1) == 1 {
		return map[int]string{7: "main/aaaaaaaa"}
	}
	return map[int]string{}
}

func TestIssuesEventsAreSentInTheOrderTheyAreMade(t *testing.T) {
	t.Parallel()
	proj := project.Project{Name: "main", Dir: "/main"}
	var mu sync.Mutex
	var sent []string
	blocked, release := make(chan struct{}), make(chan struct{})
	var first atomic.Bool
	i := New(nil, &endingThreads{}, func(name string, payload any) {
		if name != "issues" {
			return
		}
		if first.CompareAndSwap(false, true) {
			close(blocked)
			<-release
		}
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, payload.(scoped.List[Issue]).Items[0].ThreadID)
	})
	i.lists[proj.Dir] = []Issue{{Number: 7}}

	var done sync.WaitGroup
	done.Add(2)
	go func() { defer done.Done(); i.emitFor(proj) }()
	<-blocked
	go func() { defer done.Done(); i.emitFor(proj) }()
	time.Sleep(100 * time.Millisecond)
	close(release)
	done.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 2 || sent[1] != "" {
		t.Errorf("thread ids sent = %q, want the newest, none, last", sent)
	}
}
