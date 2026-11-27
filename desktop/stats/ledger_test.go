package stats

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/internal/activity"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/usage"
)

type fakeProjects []project.Project

func (f fakeProjects) List() []project.Project { return f }

func TestLedger(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	day := func(ago int) time.Time { return now.AddDate(0, 0, -ago) }
	log := loaded(t, t.TempDir())
	projects := fakeProjects{{Name: "Alpha", Dir: "/a"}, {Name: "Beta", Dir: "/b"}, {Name: "Gamma", Dir: "/g"}, {Name: "Delta", Dir: "/d"}}
	record := func(p string, ago, prompts int, threads int, issue bool, workMs int64) {
		key := strings.ToLower(p)
		at := day(ago).UnixMilli()
		for range prompts {
			log.Record(key, activity.Event{At: at, Kind: activity.Prompt})
		}
		for range threads {
			e := activity.Event{At: at, Kind: activity.ThreadStart}
			if issue {
				e.Issue = 1
			}
			log.Record(key, e)
		}
		log.Record(key, activity.Event{At: at, Kind: activity.Worked, Ms: workMs})
	}
	record("Alpha", 0, 2, 1, true, 1000)
	record("Alpha", 10, 1, 1, false, 500)
	record("Alpha", 100, 5, 2, true, 2000)
	record("Beta", 3, 3, 1, true, 4000)
	record("Beta", 400, 9, 3, false, 100)
	record("Delta", 3, 3, 0, false, 0)

	books := NewBooks(log, projects)
	build := func(days int) Ledger {
		t.Helper()
		led, err := books.Ledger(days, "", now)
		if err != nil {
			t.Fatal(err)
		}
		return led
	}

	t.Run("thirty days", func(t *testing.T) {
		led := build(30)
		if led.Days != 30 || led.Since != day(400).Format(time.DateOnly) {
			t.Errorf("days %d, since %s", led.Days, led.Since)
		}
		var names []string
		for _, r := range led.Projects {
			names = append(names, fmt.Sprintf("%s:%d", r.Project, r.Prompts))
		}
		if want := []string{"Alpha:3", "Beta:3", "Delta:3", "Gamma:0"}; !slices.Equal(names, want) {
			t.Errorf("projects = %v, want %v", names, want)
		}
		alpha := led.Projects[0]
		if alpha.Threads != 2 || alpha.IssueThreads != 1 || alpha.WorkMs != 1500 {
			t.Errorf("alpha = %+v", alpha)
		}
		tot := led.Totals
		if tot.Project != "" || tot.Prompts != 9 || tot.Threads != 3 || tot.IssueThreads != 2 || tot.WorkMs != 5500 {
			t.Errorf("totals = %+v", tot)
		}
	})

	t.Run("year and lifetime", func(t *testing.T) {
		if year := build(365).Totals; year.Prompts != 14 || year.Threads != 5 {
			t.Errorf("year totals = %+v", year)
		}
		life := build(0)
		if life.Days != 0 || life.Totals.Prompts != 23 || life.Totals.Threads != 8 || life.Totals.WorkMs != 7600 {
			t.Errorf("lifetime = %+v", life)
		}
	})

	t.Run("today starts at midnight", func(t *testing.T) {
		log := loaded(t, t.TempDir())
		lastNight := time.Date(2026, 10, 8, 23, 0, 0, 0, time.Local)
		log.Record("alpha", activity.Event{At: lastNight.UnixMilli(), Kind: activity.Prompt})
		log.Record("alpha", activity.Event{At: now.UnixMilli(), Kind: activity.Prompt})
		led, err := NewBooks(log, fakeProjects{{Name: "Alpha", Dir: "/a"}}).Ledger(1, "", now)
		if err != nil {
			t.Fatal(err)
		}
		if led.Totals.Prompts != 1 {
			t.Errorf("today's prompts = %d, want 1", led.Totals.Prompts)
		}
	})

	t.Run("one project", func(t *testing.T) {
		led, err := books.Ledger(30, "beta", now)
		if err != nil {
			t.Fatal(err)
		}
		if len(led.Projects) != 1 || led.Projects[0].Project != "Beta" || led.Totals.Prompts != 3 || led.Since != day(400).Format(time.DateOnly) {
			t.Errorf("projects %+v, totals %+v, since %s", led.Projects, led.Totals, led.Since)
		}
	})
}

func TestLedgerWithoutRecord(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	books := NewBooks(loaded(t, t.TempDir()), fakeProjects{{Name: "Solo", Dir: "/s"}})
	led, err := books.Ledger(30, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if led.Since != "" || len(led.Projects) != 1 || led.Totals.Prompts != 0 {
		t.Errorf("since %q, projects %d, totals %+v", led.Since, len(led.Projects), led.Totals)
	}
}

func TestLedgerPullRequestsAndIssues(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	at := func(ago int) int64 { return now.AddDate(0, 0, -ago).UnixMilli() }
	log := loaded(t, t.TempDir())
	for _, e := range []activity.Event{
		{At: at(0), Kind: activity.PROpened},
		{At: at(0), Kind: activity.PRMerged, Issue: 5, Ms: 1000},
		{At: at(2), Kind: activity.PRMerged, Issue: 5, Ms: 3000},
		{At: at(2), Kind: activity.PRMerged, Ms: 2000},
		{At: at(1), Kind: activity.PRClosed, Ms: 9000},
		{At: at(1), Kind: activity.IssueFiled},
		{At: at(1), Kind: activity.ThreadEnd, Ms: 100},
		{At: at(1), Kind: activity.ThreadEnd, Ms: 300},
		{At: at(50), Kind: activity.PRMerged, Issue: 6, Ms: 5000},
	} {
		log.Record("alpha", e)
	}
	log.Record("beta", activity.Event{At: at(0), Kind: activity.PRMerged, Issue: 5, Ms: 4000})
	led, err := NewBooks(log, fakeProjects{{Name: "Alpha", Dir: "/a"}, {Name: "Beta", Dir: "/b"}}).Ledger(30, "", now)
	if err != nil {
		t.Fatal(err)
	}
	alpha := led.Projects[0]
	want := LedgerRow{Project: "Alpha", PRsOpened: 1, PRsMerged: 3, PRsClosed: 1, IssuesFiled: 1, IssuesClosed: 1, ThreadMs: 400, AvgThreadMs: 200, AvgLeadMs: 2000}
	if alpha != want {
		t.Errorf("alpha = %+v, want %+v", alpha, want)
	}
	if tot := led.Totals; tot.PRsMerged != 4 || tot.IssuesClosed != 2 || tot.AvgLeadMs != 2500 || tot.AvgThreadMs != 200 {
		t.Errorf("totals = %+v: issue 5 of each project counts once per project", tot)
	}
}

func TestLedgerUsage(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	at := func(ago int) int64 { return now.AddDate(0, 0, -ago).UnixMilli() }
	log := loaded(t, t.TempDir())
	use := func(key, thread string, ago, issue int, model, effort, title string, input, cost int64) {
		log.Record(key, activity.Event{At: at(ago), Kind: activity.Usage, Thread: thread, Issue: issue, Model: model, Effort: effort, Title: title,
			Tokens: usage.Tokens{Input: input, Output: 10, Cost: cost}})
	}
	use("alpha", "alpha/aaaaaaaa", 3, 7, "claude-opus-5-5", "high", "#7 first", 100, 2_000_000)
	use("alpha", "alpha/bbbbbbbb", 1, 7, "claude-opus-5-5", "high", "#7 resumed", 200, 1_000_000)
	use("alpha", "alpha/bbbbbbbb", 1, 7, "claude-haiku-5-5", "high", "#7 resumed", 50, 10_000)
	use("alpha", "alpha/cccccccc", 0, 0, "claude-sonnet-5-5", "", "Plain task", 30, 500_000)
	use("beta", "beta/dddddddd", 0, 7, "claude-opus-5-5", "medium", "#7 beta's own", 1, 1)
	use("alpha", "alpha/eeeeeeee", 40, 9, "claude-opus-5-5", "high", "#9 old", 1000, 9_000_000)

	led, err := NewBooks(log, fakeProjects{{Name: "Alpha", Dir: "/a"}, {Name: "Beta", Dir: "/b"}}).Ledger(30, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if a := led.Projects[0]; a.Project != "Alpha" || a.Tokens != 380+40 || a.Cost != 3.51 {
		t.Errorf("alpha = %+v", a)
	}
	tot := led.Usage.Totals
	if tot.Threads != 4 || tot.Input != 381 || tot.Output != 50 || tot.Tokens != 431 || tot.Cost != 3.510001 || led.Usage.PricesAsOf == "" {
		t.Errorf("usage totals = %+v", led.Usage)
	}
	var models []string
	for _, r := range led.Usage.ByModel {
		models = append(models, fmt.Sprintf("%s/%s:%d:%v", r.Model, r.Effort, r.Threads, r.Cost))
	}
	if want := []string{"claude-opus-5-5/high:2:3", "claude-sonnet-5-5/:1:0.5", "claude-haiku-5-5/high:1:0.01", "claude-opus-5-5/medium:1:1e-06"}; !slices.Equal(models, want) {
		t.Errorf("by model = %v, want %v", models, want)
	}
	var work []string
	for _, w := range led.Work {
		work = append(work, fmt.Sprintf("%s#%d%s:%s:%d:%d", w.Project, w.Issue, w.Thread, w.Title, w.Threads, w.Tokens))
	}
	if want := []string{"Alpha#0alpha/cccccccc:Plain task:1:40", "Beta#7:#7 beta's own:1:11", "Alpha#7:#7 resumed:2:380"}; !slices.Equal(work, want) {
		t.Errorf("work = %v, want %v", work, want)
	}
}

func TestLedgerPerDay(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	at := func(ago int) int64 { return now.AddDate(0, 0, -ago).UnixMilli() }
	day := func(ago int) string { return now.AddDate(0, 0, -ago).Format(time.DateOnly) }
	log := loaded(t, t.TempDir())
	for _, r := range []struct {
		key  string
		ago  int
		kind string
		ms   int64
		cost int64
	}{
		{"alpha", 0, activity.Prompt, 0, 0},
		{"alpha", 0, activity.Worked, 1000, 0},
		{"beta", 0, activity.Prompt, 0, 0},
		{"beta", 0, activity.Usage, 0, 1_500_000},
		{"alpha", 2, activity.Prompt, 0, 0},
		{"alpha", 2, activity.Usage, 0, 250_000},
		{"alpha", 6, activity.Prompt, 0, 0},
		{"alpha", 6, activity.Worked, 3000, 0},
		{"alpha", 7, activity.Prompt, 0, 0},
		{"alpha", 13, activity.Worked, 500, 0},
		{"alpha", 14, activity.Prompt, 0, 0},
		{"beta", -1, activity.Prompt, 0, 0},
	} {
		log.Record(r.key, activity.Event{At: at(r.ago), Kind: r.kind, Thread: r.key + "/aaaaaaaa", Ms: r.ms, Tokens: usage.Tokens{Input: 1, Cost: r.cost}})
	}
	books := NewBooks(log, fakeProjects{{Name: "Alpha", Dir: "/a"}, {Name: "Beta", Dir: "/b"}})
	build := func(days int, key string) Ledger {
		t.Helper()
		led, err := books.Ledger(days, key, now)
		if err != nil {
			t.Fatal(err)
		}
		return led
	}

	t.Run("a day each, every project summed, a day after today in today", func(t *testing.T) {
		led := build(7, "")
		sum := 0
		for _, f := range led.PerDay {
			sum += f.Prompts
		}
		if sum != led.Totals.Prompts {
			t.Errorf("the bars add up to %d prompts, the total is %d", sum, led.Totals.Prompts)
		}
		want := []DayFigures{
			{Day: day(6), Prompts: 1, WorkMs: 3000},
			{Day: day(5)}, {Day: day(4)}, {Day: day(3)},
			{Day: day(2), Prompts: 1, Cost: 0.25},
			{Day: day(1)},
			{Day: day(0), Prompts: 3, WorkMs: 1000, Cost: 1.5},
		}
		if !slices.Equal(led.PerDay, want) {
			t.Errorf("per day = %+v, want %+v", led.PerDay, want)
		}
	})

	t.Run("the period before ends where the range starts", func(t *testing.T) {
		prev := build(7, "").Previous
		if prev.Prompts != 1 || prev.WorkMs != 500 || prev.Cost != 0 {
			t.Errorf("previous = %+v, want day 7's prompt and day 13's work, not day 14's prompt", prev)
		}
	})

	t.Run("one project", func(t *testing.T) {
		led := build(1, "beta")
		if want := []DayFigures{{Day: day(0), Prompts: 2, Cost: 1.5}}; !slices.Equal(led.PerDay, want) {
			t.Errorf("per day = %+v, want %+v", led.PerDay, want)
		}
		if led.Previous != (LedgerRow{}) {
			t.Errorf("previous = %+v, want nothing: beta had no record yesterday", led.Previous)
		}
	})

	t.Run("lifetime runs from the first record to today", func(t *testing.T) {
		led := build(0, "")
		if len(led.PerDay) != 15 || led.PerDay[0] != (DayFigures{Day: day(14), Prompts: 1}) || led.PerDay[14].Day != day(0) {
			t.Errorf("per day = %d days, %+v .. %+v", len(led.PerDay), led.PerDay[0], led.PerDay[len(led.PerDay)-1])
		}
		if led.Previous != (LedgerRow{}) {
			t.Errorf("previous = %+v, want none for lifetime", led.Previous)
		}
	})
}

func TestLedgerPerDayWithoutRecord(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	books := NewBooks(loaded(t, t.TempDir()), fakeProjects{{Name: "Solo", Dir: "/s"}})
	for _, days := range []int{0, 30} {
		led, err := books.Ledger(days, "", now)
		if err != nil {
			t.Fatal(err)
		}
		if want := map[int]int{0: 0, 30: 30}[days]; len(led.PerDay) != want || led.PerDay == nil {
			t.Errorf("%d days: per day = %d days, want %d", days, len(led.PerDay), want)
		}
		for _, f := range led.PerDay {
			if f.Prompts != 0 || f.WorkMs != 0 || f.Cost != 0 {
				t.Errorf("%d days: %+v, want an empty day", days, f)
			}
		}
	}
}
