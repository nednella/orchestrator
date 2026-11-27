package update

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/update"
)

const seenFile = "seen-version"

// Release is one version's notes from the changelog.
type Release struct {
	Version  string    `json:"version"`
	Date     string    `json:"date"` // "2026-10-07"
	Sections []Section `json:"sections"`
}

// Section is a group of changes, such as "Features" or "Bug Fixes".
type Section struct {
	Title   string   `json:"title"`
	Changes []Change `json:"changes"`
}

// Change is one line of a release.
type Change struct {
	Scope string `json:"scope"` // "desktop", or "" when the commit named none
	Text  string `json:"text"`
	Issue int    `json:"issue"` // 0 when it closed none
	URL   string `json:"url"`   // the issue's page, when it has one
}

var (
	releaseHeading = regexp.MustCompile(`^## \[?(\d+\.\d+\.\d+)\]?(?:\([^)]*\))?(?: \((\d{4}-\d{2}-\d{2})\))?`)
	scoped         = regexp.MustCompile(`^\*\*([^*]+):\*\* `)
	issueRef       = regexp.MustCompile(`\[#(\d+)\]\(([^)]+)\)`)
)

// ParseChangelog reads release-please's CHANGELOG.md into its releases, newest first.
func ParseChangelog(text string) []Release {
	var releases []Release
	for _, line := range strings.Split(text, "\n") {
		if m := releaseHeading.FindStringSubmatch(line); m != nil {
			releases = append(releases, Release{Version: m[1], Date: m[2]})
			continue
		}
		if len(releases) == 0 {
			continue
		}
		r := &releases[len(releases)-1]
		if title, ok := strings.CutPrefix(line, "### "); ok {
			r.Sections = append(r.Sections, Section{Title: strings.TrimSpace(title)})
			continue
		}
		item, ok := strings.CutPrefix(line, "* ")
		if !ok || len(r.Sections) == 0 {
			continue
		}
		s := &r.Sections[len(r.Sections)-1]
		s.Changes = append(s.Changes, parseChange(item))
	}
	return releases
}

func parseChange(item string) Change {
	var c Change
	if m := issueRef.FindStringSubmatch(item); m != nil {
		c.Issue, _ = strconv.Atoi(m[1])
		c.URL = m[2]
	}
	if m := scoped.FindStringSubmatch(item); m != nil {
		c.Scope = m[1]
		item = item[len(m[0]):]
	}
	if i := strings.Index(item, " (["); i >= 0 {
		item = item[:i]
	}
	c.Text = item
	return c
}

// ReleaseNotes reads the notes of the running version from its GitHub release. The error is a sentence for the
// owner: a dev build has no release, and GitHub may not answer or know the version.
func (u *Updater) ReleaseNotes(ctx context.Context) (Release, error) {
	if u.current == "dev" {
		return Release{}, errors.New("A development build has no release notes")
	}
	body, err := update.Notes(ctx, u.run, u.current)
	switch {
	case errors.Is(err, update.ErrNoNotes):
		return Release{}, fmt.Errorf("There are no release notes for %s", u.current)
	case err != nil:
		log.Printf("orchestrator: reading the release notes: %v", err)
		return Release{}, errors.New("The release notes could not be read. Check the connection")
	}
	for _, r := range ParseChangelog(body) {
		if r.Version == u.current {
			return r, nil
		}
	}
	// A release body that carries no version heading holds the changes alone.
	for _, r := range ParseChangelog("## " + u.current + "\n" + body) {
		if r.Version == u.current && len(r.Sections) > 0 {
			return r, nil
		}
	}
	return Release{}, fmt.Errorf("There are no release notes for %s", u.current)
}

// NewReleaseNotes is the notes of the running version when the app has not shown them before, in a list of one, and
// remembers the version so they show once. It is empty for a dev build and when the notes cannot be read, which
// leaves the version unseen for the next start.
func (u *Updater) NewReleaseNotes(ctx context.Context) []Release {
	path := filepath.Join(u.stateDir, seenFile)
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("orchestrator: reading the last version shown: %v", err)
		return []Release{}
	}
	if strings.TrimSpace(string(raw)) == u.current {
		return []Release{}
	}
	rel, err := u.ReleaseNotes(ctx)
	if err != nil {
		return []Release{}
	}
	if err := atomicfile.Write(path, []byte(u.current+"\n"), 0o644); err != nil {
		log.Printf("orchestrator: remembering the last version shown: %v", err)
		return []Release{}
	}
	return []Release{rel}
}
