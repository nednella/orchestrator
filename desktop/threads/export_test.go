package threads

// UsageReads is how many times the threads have read a thread's transcripts for its usage.
func (s *Threads) UsageReads() int64 { return s.usageReads.Load() }
