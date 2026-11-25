package stats

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/usage"
)

type fakeProjects []project.Project

func (f fakeProjects) List() []project.Project { return f }

func TestLedger(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	day := func(ago int) time.Time { return now.AddDate(0, 0, -ago) }
	log := loaded(t, t.TempDir())
	projects := fakeProjects{{Name: "Alpha", Dir: "/a"}, {Name: "Beta", Dir: "/b"}, {Name: "Gamma", Dir: "/g"}, {Name: "Delta", Dir: "/d"}}
	record := func(p string, ago, prompts int, sessions int, issue bool, workMs int64) {
		key := strings.ToLower(p)
		at := day(ago).UnixMilli()
		for range prompts {
			log.Record(key, activity.Event{At: at, Kind: activity.Prompt})
		}
		for range sessions {
			e := activity.Event{At: at, Kind: activity.SessionStart}
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
		if alpha.Sessions != 2 || alpha.IssueSessions != 1 || alpha.WorkMs != 1500 {
			t.Errorf("alpha = %+v", alpha)
		}
		tot := led.Totals
		if tot.Project != "" || tot.Prompts != 9 || tot.Sessions != 3 || tot.IssueSessions != 2 || tot.WorkMs != 5500 {
			t.Errorf("totals = %+v", tot)
		}
	})

	t.Run("year and lifetime", func(t *testing.T) {
		if year := build(365).Totals; year.Prompts != 14 || year.Sessions != 5 {
			t.Errorf("year totals = %+v", year)
		}
		life := build(0)
		if life.Days != 0 || life.Totals.Prompts != 23 || life.Totals.Sessions != 8 || life.Totals.WorkMs != 7600 {
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
		sum := 0
		for _, d := range led.Heat {
			sum += d.Count
		}
		if sum != 3 {
			t.Errorf("heat sum = %d, want beta's 3", sum)
		}
	})

	t.Run("heat", func(t *testing.T) {
		led := build(30)
		if len(led.Heat) != 364 || led.Heat[363].Day != "2026-10-09" || led.Heat[0].Day != day(363).Format(time.DateOnly) {
			t.Fatalf("heat = %d days, %v .. %v", len(led.Heat), led.Heat[0], led.Heat[len(led.Heat)-1])
		}
		counts := map[string]int{}
		sum := 0
		for _, d := range led.Heat {
			counts[d.Day] = d.Count
			sum += d.Count
		}
		if counts["2026-10-09"] != 2 || counts[day(3).Format(time.DateOnly)] != 6 || counts[day(100).Format(time.DateOnly)] != 5 || sum != 14 {
			t.Errorf("counts = %v, sum %d", counts, sum)
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
	if led.Since != "" || len(led.Heat) != 364 || len(led.Projects) != 1 || led.Totals.Prompts != 0 {
		t.Errorf("since %q, heat %d, projects %d, totals %+v", led.Since, len(led.Heat), len(led.Projects), led.Totals)
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
		{At: at(1), Kind: activity.SessionEnd, Ms: 100},
		{At: at(1), Kind: activity.SessionEnd, Ms: 300},
		{At: at(1), Kind: activity.ChatStart},
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
	want := LedgerRow{Project: "Alpha", Chats: 1, PRsOpened: 1, PRsMerged: 3, PRsClosed: 1, IssuesFiled: 1, IssuesClosed: 1, SessionMs: 400, AvgSessionMs: 200, AvgLeadMs: 2000}
	if alpha != want {
		t.Errorf("alpha = %+v, want %+v", alpha, want)
	}
	if tot := led.Totals; tot.PRsMerged != 4 || tot.IssuesClosed != 2 || tot.AvgLeadMs != 2500 || tot.AvgSessionMs != 200 {
		t.Errorf("totals = %+v: issue 5 of each project counts once per project", tot)
	}
}

func TestLedgerUsage(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	at := func(ago int) int64 { return now.AddDate(0, 0, -ago).UnixMilli() }
	log := loaded(t, t.TempDir())
	use := func(key, session string, ago, issue int, model, effort, title string, input, cost int64) {
		log.Record(key, activity.Event{At: at(ago), Kind: activity.Usage, Session: session, Issue: issue, Model: model, Effort: effort, Title: title,
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
	if tot.Sessions != 4 || tot.Input != 381 || tot.Output != 50 || tot.Tokens != 431 || tot.Cost != 3.510001 || led.Usage.PricesAsOf == "" {
		t.Errorf("usage totals = %+v", led.Usage)
	}
	var models []string
	for _, r := range led.Usage.ByModel {
		models = append(models, fmt.Sprintf("%s/%s:%d:%v", r.Model, r.Effort, r.Sessions, r.Cost))
	}
	if want := []string{"claude-opus-5-5/high:2:3", "claude-sonnet-5-5/:1:0.5", "claude-haiku-5-5/high:1:0.01", "claude-opus-5-5/medium:1:1e-06"}; !slices.Equal(models, want) {
		t.Errorf("by model = %v, want %v", models, want)
	}
	var work []string
	for _, w := range led.Work {
		work = append(work, fmt.Sprintf("%s#%d%s:%s:%d:%d", w.Project, w.Issue, w.Session, w.Title, w.Sessions, w.Tokens))
	}
	if want := []string{"Alpha#0alpha/cccccccc:Plain task:1:40", "Beta#7:#7 beta's own:1:11", "Alpha#7:#7 resumed:2:380"}; !slices.Equal(work, want) {
		t.Errorf("work = %v, want %v", work, want)
	}
}

func TestLedgerChatUsage(t *testing.T) {
	now := time.Date(2026, 10, 9, 15, 0, 0, 0, time.Local)
	log := loaded(t, t.TempDir())
	for _, e := range []activity.Event{
		{Kind: activity.Usage, Session: "alpha/aaaaaaaa", Issue: 7, Model: "claude-opus-5-5", Title: "#7 work", Tokens: usage.Tokens{Input: 100, Cost: 2_000_000}},
		{Kind: activity.ChatStart, Session: "alpha/cccccccc"},
		{Kind: activity.ChatUsage, Session: "alpha/cccccccc", Model: "claude-opus-5-5", Title: "ask", Tokens: usage.Tokens{Input: 30, Cost: 500_000}},
		{Kind: activity.ChatUsage, Session: "alpha/cccccccc", Model: "claude-haiku-5-5", Label: activity.Subagent, Title: "ask", Tokens: usage.Tokens{Input: 5, Cost: 10_000}},
	} {
		e.At = now.UnixMilli()
		log.Record("alpha", e)
	}
	led, err := NewBooks(log, fakeProjects{{Name: "Alpha", Dir: "/a"}}).Ledger(30, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if a := led.Projects[0]; a.Tokens != 135 || a.Cost != 2.51 || a.Chats != 1 || a.Sessions != 0 {
		t.Errorf("alpha = %+v, want the chat's cost in the project's", a)
	}
	if tot := led.Usage.Totals; tot.Sessions != 2 || tot.Tokens != 135 || tot.Cost != 2.51 {
		t.Errorf("usage totals = %+v, want the task and the chat", tot)
	}
	if task := led.Usage.Tasks; task.Sessions != 1 || task.Tokens != 100 || task.Cost != 2 {
		t.Errorf("tasks = %+v", task)
	}
	var models []string
	for _, r := range led.Usage.ByModel {
		models = append(models, fmt.Sprintf("%s:%d:%d", r.Model, r.Sessions, r.Tokens))
	}
	if want := []string{"claude-opus-5-5:2:130", "claude-haiku-5-5:1:5"}; !slices.Equal(models, want) {
		t.Errorf("by model = %v, want the task and the chat", models)
	}
	if len(led.Work) != 1 || led.Work[0].Issue != 7 {
		t.Errorf("work = %+v, want the task only", led.Work)
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
		{"alpha", 2, activity.ChatUsage, 0, 250_000},
		{"alpha", 6, activity.Prompt, 0, 0},
		{"alpha", 6, activity.Worked, 3000, 0},
		{"alpha", 7, activity.Prompt, 0, 0},
		{"alpha", 13, activity.Worked, 500, 0},
		{"alpha", 14, activity.Prompt, 0, 0},
	} {
		log.Record(r.key, activity.Event{At: at(r.ago), Kind: r.kind, Session: r.key + "/aaaaaaaa", Ms: r.ms, Tokens: usage.Tokens{Input: 1, Cost: r.cost}})
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

	t.Run("a day each, every project summed", func(t *testing.T) {
		led := build(7, "")
		want := []DayFigures{
			{Day: day(6), Prompts: 1, WorkMs: 3000},
			{Day: day(5)}, {Day: day(4)}, {Day: day(3)},
			{Day: day(2), Prompts: 1, Cost: 0.25},
			{Day: day(1)},
			{Day: day(0), Prompts: 2, WorkMs: 1000, Cost: 1.5},
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
		if want := []DayFigures{{Day: day(0), Prompts: 1, Cost: 1.5}}; !slices.Equal(led.PerDay, want) {
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
