package sessions

// UsageReads is how many times the sessions have read a thread's transcripts for its usage.
func (s *Sessions) UsageReads() int64 { return s.usageReads.Load() }
