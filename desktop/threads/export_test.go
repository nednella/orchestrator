package threads

import (
	"slices"

	"github.com/nednella/orchestrator/internal/thread"
)

// UsageReads is how many times the threads have read a thread's transcripts for its usage.
func (s *Threads) UsageReads() int64 { return s.usageReads.Load() }

// History is the thread's timeline of state changes.
func (s *Threads) History(id string) []thread.Change {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.history[id])
}
