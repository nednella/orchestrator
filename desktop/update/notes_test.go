package update_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/update"
)

const changelog = `# Changelog

## [0.14.0](https://github.com/nednella/orchestrator/compare/v0.13.0...v0.14.0) (2026-10-07)


### Features

* **desktop:** let agents close their browser window ([a796cf6](https://github.com/nednella/orchestrator/commit/a796cf6)), closes [#218](https://github.com/nednella/orchestrator/issues/218)


### Bug Fixes

* widen settings panel so every row shows ([29a0500](https://github.com/nednella/orchestrator/commit/29a0500))
* keep the Mac awake while sessions work ([#88](https://github.com/nednella/orchestrator/issues/88)) ([0313ac2](https://github.com/nednella/orchestrator/commit/0313ac2))

## [0.13.0](https://github.com/nednella/orchestrator/compare/v0.1.0...v0.13.0) (2026-10-06)


### Bug Fixes

* a fix in between ([2222222](https://github.com/nednella/orchestrator/commit/2222222))

## 0.1.0 (2026-10-05)


### Features

* **ui:** first window ([1111111](https://github.com/nednella/orchestrator/commit/1111111))
`

func TestParseChangelogReadsEachReleaseAndItsChanges(t *testing.T) {
	want := []update.Release{
		{Version: "0.14.0", Date: "2026-10-07", Sections: []update.Section{
			{Title: "Features", Changes: []update.Change{{Scope: "desktop", Text: "let agents close their browser window", Issue: 218, URL: "https://github.com/nednella/orchestrator/issues/218"}}},
			{Title: "Bug Fixes", Changes: []update.Change{
				{Text: "widen settings panel so every row shows"},
				{Text: "keep the Mac awake while sessions work", Issue: 88, URL: "https://github.com/nednella/orchestrator/issues/88"},
			}},
		}},
		{Version: "0.13.0", Date: "2026-10-06", Sections: []update.Section{
			{Title: "Bug Fixes", Changes: []update.Change{{Text: "a fix in between"}}},
		}},
		{Version: "0.1.0", Date: "2026-10-05", Sections: []update.Section{
			{Title: "Features", Changes: []update.Change{{Scope: "ui", Text: "first window"}}},
		}},
	}
	if got := update.ParseChangelog(changelog); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseChangelog =\n%+v\nwant\n%+v", got, want)
	}
}

const body014 = `## [0.14.0](https://github.com/nednella/orchestrator/compare/v0.13.0...v0.14.0) (2026-10-07)


### Features

* **desktop:** let agents close their browser window ([a796cf6](https://github.com/nednella/orchestrator/commit/a796cf6)), closes [#218](https://github.com/nednella/orchestrator/issues/218)
`

func seen(h *apptest.Harness) string {
	raw, _ := os.ReadFile(filepath.Join(h.State, "seen-version"))
	return strings.TrimSpace(string(raw))
}

func newNotes(t *testing.T, h *apptest.Harness) []string {
	t.Helper()
	notes := h.App.Updates().NewReleaseNotes(context.Background())
	if notes == nil {
		t.Fatal("NewReleaseNotes returned nil, which the front end reads as null")
	}
	var out []string
	for _, r := range notes {
		out = append(out, r.Version)
	}
	return out
}

func notesHarness(t *testing.T, o apptest.Options) *apptest.Harness {
	o.UpdateTick = time.Hour
	return apptest.NewWith(t, o)
}

func TestTheFirstStartShowsTheRunningVersionsNotesAndRemembersIt(t *testing.T) {
	h := notesHarness(t, apptest.Options{Version: "0.14.0", Notes: map[string]string{"0.14.0": body014}})
	if got := newNotes(t, h); !reflect.DeepEqual(got, []string{"0.14.0"}) {
		t.Errorf("first start = %v", got)
	}
	if got := seen(h); got != "0.14.0" {
		t.Errorf("seen-version = %q", got)
	}
}

func TestTheSameVersionShowsItsNotesOnce(t *testing.T) {
	h := notesHarness(t, apptest.Options{Version: "0.14.0", Notes: map[string]string{"0.14.0": body014}})
	newNotes(t, h)
	if got := newNotes(t, h); got != nil {
		t.Errorf("second start = %v, want none", got)
	}
}

func TestANewVersionShowsItsNotesAfterAnOldOne(t *testing.T) {
	h := notesHarness(t, apptest.Options{Version: "0.14.0", Notes: map[string]string{"0.14.0": body014}})
	if err := os.WriteFile(filepath.Join(h.State, "seen-version"), []byte("0.13.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := newNotes(t, h); !reflect.DeepEqual(got, []string{"0.14.0"}) {
		t.Errorf("start after the update = %v", got)
	}
	if got := seen(h); got != "0.14.0" {
		t.Errorf("seen-version = %q", got)
	}
}

func TestADevBuildShowsNothingByItselfAndRemembersNothing(t *testing.T) {
	h := notesHarness(t, apptest.Options{})
	if got := newNotes(t, h); got != nil {
		t.Errorf("dev build = %v, want none", got)
	}
	if got := seen(h); got != "" {
		t.Errorf("seen-version = %q, want none", got)
	}
}

func TestNotesThatCannotBeReadShowNothingAndStayUnseen(t *testing.T) {
	for name, o := range map[string]apptest.Options{
		"offline":    {Version: "0.14.0", NotesErr: errors.New("curl: could not resolve host")},
		"no release": {Version: "0.14.0"},
	} {
		t.Run(name, func(t *testing.T) {
			h := notesHarness(t, o)
			if got := newNotes(t, h); got != nil {
				t.Errorf("start = %v, want none", got)
			}
			if got := seen(h); got != "" {
				t.Errorf("seen-version = %q, want none, so the next start tries again", got)
			}
		})
	}
}

func TestReleaseNotesSaySoInOneLineWhenTheyCannotBeRead(t *testing.T) {
	for name, tt := range map[string]struct {
		o    apptest.Options
		want string
	}{
		"dev build":  {apptest.Options{}, "A development build has no release notes"},
		"offline":    {apptest.Options{Version: "0.14.0", NotesErr: errors.New("curl: could not resolve host")}, "The release notes could not be read. Check the connection"},
		"no release": {apptest.Options{Version: "0.14.0"}, "There are no release notes for 0.14.0"},
	} {
		t.Run(name, func(t *testing.T) {
			h := notesHarness(t, tt.o)
			_, err := h.App.Updates().ReleaseNotes(context.Background())
			if err == nil || err.Error() != tt.want {
				t.Errorf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestReleaseNotesOpenedByHandDoNotTouchWhatWasShown(t *testing.T) {
	h := notesHarness(t, apptest.Options{Version: "0.14.0", Notes: map[string]string{"0.14.0": body014}})
	rel, err := h.App.Updates().ReleaseNotes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "0.14.0" || len(rel.Sections) != 1 || rel.Sections[0].Changes[0].Issue != 218 {
		t.Errorf("release = %+v", rel)
	}
	if got := seen(h); got != "" {
		t.Errorf("seen-version = %q, want none", got)
	}
}

func TestAReleaseBodyWithNoVersionHeadingIsTheRunningVersionsChanges(t *testing.T) {
	h := notesHarness(t, apptest.Options{Version: "0.14.0", Notes: map[string]string{"0.14.0": "### Bug Fixes\n\n* mend the thing\n"}})
	rel, err := h.App.Updates().ReleaseNotes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "0.14.0" || rel.Sections[0].Changes[0].Text != "mend the thing" {
		t.Errorf("release = %+v", rel)
	}
}
