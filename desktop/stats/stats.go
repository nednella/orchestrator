// Package stats records every time a session waits on the user, and sums it up.
package stats

import (
	"cmp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/session"
)

// Opened starts a wait for a session that now needs the user: the reason comes from its record.
func (w *Waits) Opened(id, title string, issue int, model string, rec session.Record, at time.Time) {
	kind, label := CauseOf(rec)
	w.Begin(id, Wait{SessionTitle: title, Issue: issue, Model: model, Kind: kind, Label: label, StartedAt: at.UnixMilli()})
}

// Closed ends the session's open wait, if any, and reports whether there was one.
func (w *Waits) Closed(id string, at time.Time) bool { return w.End(id, at) }

// Wait is one time a session waited on the user.
type Wait struct {
	SessionTitle string `json:"sessionTitle"`
	Issue        int    `json:"issue"`
	Model        string `json:"model"` // the model the session ran, "" when it was not set
	Kind         string `json:"kind"`  // permission, question or idle
	Label        string `json:"label"`
	StartedAt    int64  `json:"startedAt"`
	WaitedMs     int64  `json:"waitedMs"`
}

type CauseCount struct {
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Count       int    `json:"count"`
	TotalWaitMs int64  `json:"totalWaitMs"`
}

type DayCount struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

type Stats struct {
	Days         int          `json:"days"`
	Total        int          `json:"total"`
	TotalWaitMs  int64        `json:"totalWaitMs"`
	MedianWaitMs int64        `json:"medianWaitMs"`
	ByCause      []CauseCount `json:"byCause"`
	ByDay        []DayCount   `json:"byDay"`
	Recent       []Wait       `json:"recent"`
}

const recentWaits = 50

// CauseOf says why the record's session waits on the user, in the words the Stats view groups by.
func CauseOf(rec session.Record) (kind, label string) {
	switch {
	case rec.State == session.Idle:
		return "idle", "Reply landed"
	case rec.Event == "Stop":
		return "question", "Question in reply"
	case rec.Notify == "permission_prompt" || (rec.Notify == "" && strings.Contains(strings.ToLower(rec.Detail), "permission")):
		return "permission", permissionLabel(rec.Tool, rec.Command)
	}
	return "question", "Question"
}

func permissionLabel(tool, command string) string {
	switch {
	case tool == "":
		return "Permission"
	case tool == "Bash" && command != "":
		return "Bash: " + strings.Join(firstWords(command, 3), " ")
	}
	return tool
}

func firstWords(s string, n int) []string {
	w := strings.Fields(s)
	return w[:min(len(w), n)]
}

// Waits records when sessions wait on the user: open ones in memory, finished ones in the event log.
type Waits struct {
	log *Log

	mu   sync.Mutex
	open map[string]Wait // by session id
}

// New records each finished wait in log.
func New(log *Log) *Waits { return &Waits{log: log, open: map[string]Wait{}} }

// Begin opens a wait for the session.
func (w *Waits) Begin(id string, wait Wait) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.open[id] = wait
}

// End closes the session's open wait, if any, and reports whether there was one.
func (w *Waits) End(id string, at time.Time) bool {
	w.mu.Lock()
	wait, ok := w.open[id]
	delete(w.open, id)
	w.mu.Unlock()
	if !ok {
		return false
	}
	wait.WaitedMs = max(at.UnixMilli()-wait.StartedAt, 1)
	w.log.Record(session.ProjectKey(id), activity.Event{At: at.UnixMilli(), Kind: activity.Wait, Session: id, Issue: wait.Issue, Model: wait.Model, Ms: wait.WaitedMs, Label: wait.Kind, Title: wait.SessionTitle, Cause: wait.Label})
	return true
}

// Stats sums up the waits of the projects whose keys are given over the last days days, today included, from
// the daily rollup and the waits still open.
func (w *Waits) Stats(keys []string, days int, now time.Time) (Stats, error) {
	days = max(days, 1)
	st := Stats{Days: days, ByCause: []CauseCount{}, ByDay: []DayCount{}, Recent: []Wait{}}
	y, m, d := now.Date()
	from := time.Date(y, m, d-(days-1), 0, 0, 0, 0, now.Location())
	var tally waitTally
	perDay := map[string]int{}
	for _, key := range keys {
		for day, v := range w.log.Days(key) {
			if day >= localDay(from) {
				tally.merge(v.waits)
				perDay[day] += v.waits.total
			}
		}
	}
	w.mu.Lock()
	for id, wait := range w.open {
		if slices.Contains(keys, session.ProjectKey(id)) && wait.StartedAt >= from.UnixMilli() {
			wait.WaitedMs = 0
			tally.count(wait)
			perDay[localDay(time.UnixMilli(wait.StartedAt))]++
		}
	}
	w.mu.Unlock()
	tally.keepLatest()

	st.Total = tally.total
	waited := slices.Sorted(slices.Values(tally.waited))
	for _, ms := range waited {
		st.TotalWaitMs += ms
	}
	if n := len(waited); n > 0 {
		st.MedianWaitMs = waited[n/2]
		if n%2 == 0 {
			st.MedianWaitMs = (waited[n/2-1] + waited[n/2]) / 2
		}
	}
	for _, c := range tally.causes {
		st.ByCause = append(st.ByCause, *c)
	}
	slices.SortFunc(st.ByCause, func(a, b CauseCount) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		if a.TotalWaitMs != b.TotalWaitMs {
			return int(b.TotalWaitMs - a.TotalWaitMs)
		}
		return strings.Compare(a.Label, b.Label)
	})
	for i := range days {
		day := from.AddDate(0, 0, i).Format(time.DateOnly)
		st.ByDay = append(st.ByDay, DayCount{Day: day, Count: perDay[day]})
	}
	st.Recent = append(st.Recent, tally.latest...)
	return st, nil
}

// waitOf is the finished wait an event records.
func waitOf(e activity.Event) Wait {
	return Wait{SessionTitle: e.Title, Issue: e.Issue, Model: e.Model, Kind: e.Label, Label: cmp.Or(e.Cause, e.Label), StartedAt: e.At - e.Ms, WaitedMs: e.Ms}
}

// waitTally is the waits of a day, or of a range of days, summed up. It keeps every wait's time for the median,
// and only the latest waits, which are all a range can show of a day.
type waitTally struct {
	total  int
	causes map[[2]string]*CauseCount // by kind and label
	waited []int64                   // of the closed waits
	latest []Wait                    // latest first once kept
}

func (t *waitTally) count(w Wait) {
	if t.causes == nil {
		t.causes = map[[2]string]*CauseCount{}
	}
	t.total++
	key := [2]string{w.Kind, w.Label}
	c := t.causes[key]
	if c == nil {
		c = &CauseCount{Kind: w.Kind, Label: w.Label}
		t.causes[key] = c
	}
	c.Count++
	c.TotalWaitMs += w.WaitedMs
	if w.WaitedMs > 0 {
		t.waited = append(t.waited, w.WaitedMs)
	}
	t.latest = append(t.latest, w)
	if len(t.latest) > 2*recentWaits {
		t.keepLatest()
	}
}

// merge adds o, never sharing its maps or slices: a day's tally stays in the cache.
func (t *waitTally) merge(o waitTally) {
	if o.total == 0 {
		return
	}
	if t.causes == nil {
		t.causes = map[[2]string]*CauseCount{}
	}
	t.total += o.total
	for key, c := range o.causes {
		if t.causes[key] == nil {
			t.causes[key] = &CauseCount{Kind: c.Kind, Label: c.Label}
		}
		t.causes[key].Count += c.Count
		t.causes[key].TotalWaitMs += c.TotalWaitMs
	}
	t.waited = append(slices.Clip(t.waited), o.waited...)
	t.latest = append(slices.Clip(t.latest), o.latest...)
	t.keepLatest()
}

// keepLatest drops all but the latest waits.
func (t *waitTally) keepLatest() {
	slices.SortStableFunc(t.latest, func(a, b Wait) int { return cmp.Compare(b.StartedAt, a.StartedAt) })
	t.latest = t.latest[:min(len(t.latest), recentWaits)]
}
