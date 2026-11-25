package stats

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/usage"
)

// Ledger is what the agents did in one project or across every project, from the event log.
type Ledger struct {
	Days     int          `json:"days"`     // 0 for lifetime
	Since    string       `json:"since"`    // the first day with any record, "" when there is none
	Totals   LedgerRow    `json:"totals"`   // every project summed
	Projects []LedgerRow  `json:"projects"` // most prompts first, then by name
	Previous LedgerRow    `json:"previous"` // every project summed over the days days before these; zero for lifetime
	PerDay   []DayFigures `json:"perDay"`   // each of the ledger's days, oldest first; for lifetime from Since to today
	Usage    Usage        `json:"usage"`
	Work     []WorkRow    `json:"work"` // latest first
}

// DayFigures is what the agents did on one day, every project in the ledger summed.
type DayFigures struct {
	Day     string  `json:"day"`
	Prompts int     `json:"prompts"`
	WorkMs  int64   `json:"workMs"`
	Cost    float64 `json:"cost"` // US dollars at list prices
}

// LedgerRow is one project's figures, or all of them in the totals.
type LedgerRow struct {
	Project       string  `json:"project"`
	Prompts       int     `json:"prompts"`
	Chats         int     `json:"chats"`
	Sessions      int     `json:"sessions"`
	IssueSessions int     `json:"issueSessions"`
	WorkMs        int64   `json:"workMs"`
	PRsOpened     int     `json:"prsOpened"`
	PRsMerged     int     `json:"prsMerged"`
	PRsClosed     int     `json:"prsClosed"` // closed without a merge
	IssuesFiled   int     `json:"issuesFiled"`
	IssuesClosed  int     `json:"issuesClosed"`
	SessionMs     int64   `json:"sessionMs"`    // how long the ended sessions ran, start to end, all together
	AvgSessionMs  int64   `json:"avgSessionMs"` // 0 when no session ended
	AvgLeadMs     int64   `json:"avgLeadMs"`    // from a PR first seen open to merged; 0 when none merged
	Tokens        int64   `json:"tokens"`
	Cost          float64 `json:"cost"` // US dollars at list prices
}

// total is days added up, and the issues closed over them.
type total struct {
	Day
	issuesClosed int
}

func (t total) row(project string) LedgerRow {
	r := LedgerRow{
		Project: project, Prompts: t.Prompts, Chats: t.Chats, Sessions: t.Sessions, IssueSessions: t.IssueSessions, WorkMs: t.WorkMs, SessionMs: t.SessionMs,
		PRsOpened: t.PRsOpened, PRsMerged: t.PRsMerged, PRsClosed: t.PRsClosed, IssuesFiled: t.IssuesFiled, IssuesClosed: t.issuesClosed,
		Tokens: t.spending.all.Total(), Cost: t.spending.all.Dollars(),
	}
	if t.Ended > 0 {
		r.AvgSessionMs = t.SessionMs / int64(t.Ended)
	}
	if t.PRsMerged > 0 {
		r.AvgLeadMs = t.LeadMs / int64(t.PRsMerged)
	}
	return r
}

// dayFigures is one day's figures, summed over projects, with the cost kept in millionths of a dollar.
type dayFigures struct {
	prompts int
	workMs  int64
	cost    int64
}

func (f *dayFigures) add(d Day) {
	f.prompts += d.Prompts
	f.workMs += d.WorkMs
	f.cost += d.spending.all.Cost
}

// of is the day's figures, zero for a day with no record.
func (f *dayFigures) of(day string) DayFigures {
	if f == nil {
		return DayFigures{Day: day}
	}
	return DayFigures{Day: day, Prompts: f.prompts, WorkMs: f.workMs, Cost: usage.Tokens{Cost: f.cost}.Dollars()}
}

// Registry lists the configured projects.
type Registry interface{ List() []project.Project }

// Books builds the ledger from the event log.
type Books struct {
	log      *Log
	projects Registry
}

func NewBooks(l *Log, p Registry) *Books {
	return &Books{log: l, projects: p}
}

// Ledger sums the last days days, today included; 0 is all of the record. It covers the project whose key
// is given, or every project for "". An issue closed on two days counts once.
func (b *Books) Ledger(days int, key string, now time.Time) (Ledger, error) {
	days = max(days, 0)
	y, m, d := now.Date()
	date := func(day int) string { return time.Date(y, m, day, 0, 0, 0, 0, now.Location()).Format(time.DateOnly) }
	today, from, before := date(d), "", ""
	if days > 0 {
		from, before = date(d-(days-1)), date(d-(2*days-1))
	}
	led := Ledger{Days: days, Projects: []LedgerRow{}, PerDay: []DayFigures{}, Work: []WorkRow{}}
	figures := map[string]*dayFigures{}
	var totals, previous total
	for _, p := range b.projects.List() {
		if key != "" && p.Key() != key {
			continue
		}
		var sum, prev total
		closed, prevClosed := map[int]bool{}, map[int]bool{}
		for day, v := range b.log.Days(p.Key()) {
			if led.Since == "" || day < led.Since {
				led.Since = day
			}
			switch {
			case day >= from:
				sum.add(v)
				for _, n := range v.ClosedIssues {
					closed[n] = true
				}
				// A day after today, from a Mac whose clock runs ahead, counts in today's bar, so the bars add up.
				bar := min(day, today)
				if figures[bar] == nil {
					figures[bar] = &dayFigures{}
				}
				figures[bar].add(v)
			case day >= before:
				prev.add(v)
				for _, n := range v.ClosedIssues {
					prevClosed[n] = true
				}
			}
		}
		sum.issuesClosed = len(closed)
		totals.add(sum.Day)
		totals.issuesClosed += sum.issuesClosed
		prev.issuesClosed = len(prevClosed)
		previous.add(prev.Day)
		previous.issuesClosed += prev.issuesClosed
		led.Projects = append(led.Projects, sum.row(p.Name))
		led.Work = append(led.Work, sum.spending.work(p.Name)...)
	}
	last := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	start, err := time.ParseInLocation(time.DateOnly, cmp.Or(from, led.Since), now.Location())
	for t := start; err == nil && !t.After(last); t = t.AddDate(0, 0, 1) {
		day := t.Format(time.DateOnly)
		led.PerDay = append(led.PerDay, figures[day].of(day))
	}

	slices.SortFunc(led.Projects, func(a, b LedgerRow) int {
		if a.Prompts != b.Prompts {
			return b.Prompts - a.Prompts
		}
		return strings.Compare(a.Project, b.Project)
	})
	slices.SortFunc(led.Work, func(a, b WorkRow) int {
		return cmp.Or(cmp.Compare(b.LastAt, a.LastAt), cmp.Compare(a.Project, b.Project), cmp.Compare(a.Issue, b.Issue), cmp.Compare(a.Session, b.Session))
	})
	led.Totals = totals.row("")
	led.Previous = previous.row("")
	led.Usage = totals.spending.usage()
	return led, nil
}
