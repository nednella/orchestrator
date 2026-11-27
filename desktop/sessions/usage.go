package sessions

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/nednella/orchestrator/internal/activity"
	"github.com/nednella/orchestrator/internal/agent"
	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/session"
	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/usage"
)

// reader reads one session's transcripts, one read at a time.
type reader struct {
	mu      sync.Mutex
	cursor  usage.Reader // nil until the event log told what was counted before
	pending []usage.Part // read and in the session's total, but not in the event log yet
}

// countUsage adds what the session's agent used since the last count to the session's total, and shows on the row
// what it used last. model is the one the agent reports, for a transcript that names none. With record, it also
// records all that is not in the event log yet, one event for each model. The first count of a run of the app starts
// after the latest usage the event log holds for the session, so a restart reads again what was not recorded, and
// counts nothing twice. A woken thread's count goes on under its one id from its first start.
func (s *Sessions) countUsage(id, transcript, model string, record bool) {
	s.usageReads.Add(1)
	s.mu.Lock()
	in, ok := s.infoOf(id)
	if !ok {
		s.mu.Unlock()
		return
	}
	created, _ := s.startsOf(id, in)
	live := slices.ContainsFunc(s.info, func(i term.Info) bool { return i.Name.String() == id })
	ctx := s.ctx
	r := s.readers[id]
	if r == nil {
		r = &reader{}
		s.readers[id] = r
	}
	s.mu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	var before usage.Tokens
	name, nameErr := session.ParseName(id)
	if r.cursor == nil {
		after := created
		s.activity.Since(session.ProjectKey(id), created.In(time.Local).Format(time.DateOnly), func(e activity.Event) {
			if e.Session == id && e.Kind == activity.Usage {
				before.Add(e.Tokens)
				if at := time.UnixMilli(e.At); at.After(after) {
					after = at
				}
			}
		})
		mark := usage.Mark{After: after}
		if nameErr == nil {
			mark = s.loadUsageMark(name, after)
		}
		r.cursor = agent.Claude{}.Usage(mark)
	}
	parts, cur, err := r.cursor.Read(transcript, model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: reading the usage of %s: %v\n", id, err)
	}
	// Claude writes ultracode into its transcript as the xhigh it runs at.
	if cur.Effort == "xhigh" && in.Effort == agent.Ultracode {
		cur.Effort = in.Effort
	}
	if live && cur.Model != "" && (in.Model != cur.Model || in.Effort != cur.Effort) {
		in.Model, in.Effort = cur.Model, cur.Effort
		s.ran(ctx, in)
	}
	for _, p := range parts {
		r.pending = addPart(r.pending, p)
	}
	if len(parts) == 0 && before == (usage.Tokens{}) && (!record || len(r.pending) == 0) {
		return
	}
	issue, _ := strconv.Atoi(in.Issue)
	s.mu.Lock()
	defer s.mu.Unlock()
	total := s.spent[id]
	total.Add(before)
	for _, p := range parts {
		total.Add(p.Tokens)
	}
	if record {
		for _, p := range r.pending {
			e := activity.Event{At: r.cursor.After().UnixMilli(), Kind: activity.Usage, Issue: issue, Model: p.Model, Effort: in.Effort, Title: cmp.Or(in.Title, defaultTitle), Tokens: p.Tokens}
			if p.Subagent {
				e.Label = activity.Subagent
			}
			s.record(id, e)
		}
		r.pending = nil
	}
	if _, kept := s.readers[id]; kept { // a deleted thread has no row to show it
		s.spent[id] = total
		s.sync()
		if record && nameErr == nil {
			s.saveUsageMark(name, r.cursor.Mark())
		}
	}
}

// usageMarkPath is where the session's reader keeps how far it read its transcripts.
func (s *Sessions) usageMarkPath(name session.Name) string {
	return filepath.Join(s.localDir, name.Project, "usage", name.Token+".json")
}

// loadUsageMark is the mark a reader of the session starts from: the saved one when it stands at or before after,
// the time the event log holds, since offsets from a later time could skip lines the log never got. Else only after.
func (s *Sessions) loadUsageMark(name session.Name, after time.Time) usage.Mark {
	var m usage.Mark
	data, err := os.ReadFile(s.usageMarkPath(name))
	if err != nil || json.Unmarshal(data, &m) != nil || m.After.Truncate(time.Millisecond).After(after) {
		return usage.Mark{After: after}
	}
	m.After = after
	return m
}

// saveUsageMark keeps the mark of a reader whose reads are all in the event log. A failure costs only a longer read
// after the next restart.
func (s *Sessions) saveUsageMark(name session.Name, m usage.Mark) {
	data, err := json.Marshal(m)
	if err == nil {
		err = atomicfile.Write(s.usageMarkPath(name), data, 0o600)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: saving the usage mark of %s: %v\n", name, err)
	}
}

// ran shows on the row the model and effort the agent used last, and keeps them where the start choice is kept, the
// tmux session and the state file, so a restart of the app shows them without a new turn.
func (s *Sessions) ran(ctx context.Context, in term.Info) {
	ctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	if err := s.tmux.SetModel(ctx, in.Name, in.Model, in.Effort, in.Fast); err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: %v\n", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.info {
		if s.info[i].Name == in.Name {
			s.info[i].Model, s.info[i].Effort = in.Model, in.Effort
			s.saveIdentity(s.info[i])
			s.sync()
		}
	}
}

// addPart adds p to the part of parts with its model and kind, or appends it.
func addPart(parts []usage.Part, p usage.Part) []usage.Part {
	for i := range parts {
		if parts[i].Model == p.Model && parts[i].Subagent == p.Subagent {
			parts[i].Add(p.Tokens)
			return parts
		}
	}
	return append(parts, p)
}

// infoOf is what is known of the session, live or ended. It needs mu.
func (s *Sessions) infoOf(id string) (term.Info, bool) {
	for _, in := range s.info {
		if in.Name.String() == id {
			return in, true
		}
	}
	if e, ok := s.ended[id]; ok {
		return e.info, true
	}
	in, ok := s.known[id]
	return in, ok
}
