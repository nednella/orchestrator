package sessions

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/project"
)

func prJSON(state string, draft bool, rollup string, comments, reviews int) string {
	repeat := func(n int) string {
		return "[" + strings.TrimSuffix(strings.Repeat(`{"id":"x"},`, n), ",") + "]"
	}
	return fmt.Sprintf(`[{"number":12,"url":"https://github.com/acme/widgets/pull/12","state":%q,"isDraft":%v,"statusCheckRollup":%s,"comments":%s,"reviews":%s,"updatedAt":"2026-10-01T12:00:00Z"}]`,
		state, draft, rollup, repeat(comments), repeat(reviews))
}

func TestParsePR(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		state, check string
		comments     int
	}{
		{"draft", prJSON("OPEN", true, "[]", 0, 0), "draft", "none", 0},
		{"open", prJSON("OPEN", false, "[]", 1, 2), "open", "none", 3},
		{"approval is not a review to address", strings.Replace(prJSON("OPEN", false, "[]", 1, 2), `[{"id":"x"},{"id":"x"}]`, `[{"state":"APPROVED"},{"state":"CHANGES_REQUESTED"}]`, 1), "open", "none", 2},
		{"the gh user's own comments and replies are not news", strings.Replace(strings.Replace(prJSON("OPEN", false, "[]", 1, 2), `[{"id":"x"},{"id":"x"}]`, `[{"author":{"login":"bot"},"state":"CHANGES_REQUESTED"},{"author":{"login":"me"},"state":"COMMENTED"}]`, 1), `[{"id":"x"}]`, `[{"author":{"login":"me"}}]`, 1), "open", "none", 1},
		{"merged", prJSON("MERGED", false, "[]", 0, 0), "merged", "none", 0},
		{"closed", prJSON("CLOSED", false, "[]", 0, 0), "closed", "none", 0},
		{"passing", prJSON("OPEN", false, `[{"status":"COMPLETED","conclusion":"SUCCESS"},{"status":"COMPLETED","conclusion":"SKIPPED"},{"state":"SUCCESS"}]`, 0, 0), "open", "passing", 0},
		{"pending", prJSON("OPEN", false, `[{"status":"COMPLETED","conclusion":"SUCCESS"},{"status":"IN_PROGRESS","conclusion":""}]`, 0, 0), "open", "pending", 0},
		{"failing wins", prJSON("OPEN", false, `[{"status":"IN_PROGRESS"},{"status":"COMPLETED","conclusion":"FAILURE"},{"status":"COMPLETED","conclusion":"SUCCESS"}]`, 0, 0), "open", "failing", 0},
		{"status context error", prJSON("OPEN", false, `[{"state":"ERROR"}]`, 0, 0), "open", "failing", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr, err := parsePR([]byte(tt.data), "me")
			if err != nil || pr == nil {
				t.Fatalf("parsePR = %v, %v", pr, err)
			}
			if pr.State != tt.state || pr.Checks != tt.check || pr.Comments != tt.comments || pr.Number != 12 || pr.UpdatedAt == 0 {
				t.Errorf("pr = %+v", pr)
			}
		})
	}
	if pr, err := parsePR([]byte("[]"), "me"); pr != nil || err != nil {
		t.Errorf("no PR gave %v, %v", pr, err)
	}
	if _, err := parsePR([]byte("nope"), "me"); err == nil {
		t.Error("bad json accepted")
	}
}

func TestParsePRChecksAndTitle(t *testing.T) {
	data := `[{"number":12,"title":"Show the status","url":"u","state":"OPEN","isDraft":false,"statusCheckRollup":[` +
		`{"name":"lint","status":"COMPLETED","conclusion":"FAILURE"},{"context":"ci/test","state":"ERROR"},` +
		`{"name":"build","status":"IN_PROGRESS"},{"name":"vet","status":"COMPLETED","conclusion":"SUCCESS"}],"comments":[],"reviews":[],"updatedAt":"2026-10-01T12:00:00Z"}]`
	pr, err := parsePR([]byte(data), "me")
	if err != nil {
		t.Fatal(err)
	}
	if pr.Title != "Show the status" || pr.Checks != "failing" || !slices.Equal(pr.FailedChecks, []string{"lint", "ci/test"}) || pr.RunningChecks != 1 || pr.PassedChecks != 1 {
		t.Errorf("pr = %+v", pr)
	}
	if pr, _ := parsePR([]byte(prJSON("OPEN", false, "[]", 0, 0)), "me"); pr.FailedChecks == nil {
		t.Error("no failing checks must be an empty list, not null")
	}
}

func TestVerdict(t *testing.T) {
	by := func(login, state string) review {
		r := review{State: state}
		r.Author.Login = login
		return r
	}
	tests := []struct {
		name    string
		reviews []review
		want    string
	}{
		{"none", nil, ""},
		{"a comment", []review{by("a", "COMMENTED")}, "commented"},
		{"an approval", []review{by("a", "APPROVED")}, "approved"},
		{"a comment after an approval keeps it", []review{by("a", "APPROVED"), by("a", "COMMENTED")}, "approved"},
		{"changes by one reviewer win over another's approval", []review{by("a", "CHANGES_REQUESTED"), by("b", "APPROVED")}, "changes"},
		{"a reviewer's later approval replaces their changes", []review{by("a", "CHANGES_REQUESTED"), by("a", "APPROVED")}, "approved"},
		{"a dismissal undoes the approval", []review{by("a", "APPROVED"), by("a", "DISMISSED")}, ""},
		{"a pending review is not one", []review{by("a", "PENDING")}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := verdict(tt.reviews); got != tt.want {
				t.Errorf("verdict = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseWorktrees(t *testing.T) {
	w := parseWorktrees("worktree /p\nHEAD abc\nbranch refs/heads/main\n\nworktree /p/trees/issue-7\nHEAD def\nbranch refs/heads/issue-7\n\nworktree /p/detached\nHEAD 123\ndetached\n")
	if w.main != "/p" || w.byBranch["issue-7"] != "/p/trees/issue-7" || w.byBranch["main"] != "/p" || len(w.byBranch) != 2 || !slices.Equal(w.detached, []string{"/p/detached"}) {
		t.Errorf("worktrees = %+v", w)
	}
}

type noActivity struct{}

func (noActivity) Record(string, activity.Event)              {}
func (noActivity) Since(string, string, func(activity.Event)) {}

func TestPollResultForAForgottenSessionIsDropped(t *testing.T) {
	l := newLifecycle(nil, nil, t.TempDir(), t.TempDir(), nil, noActivity{}, func(string, any) {})
	proj := project.Project{Name: "p"}
	for _, tt := range []struct {
		name   string
		forget bool
		want   bool
	}{{"forgotten", true, false}, {"kept", false, true}} {
		t.Run(tt.name, func(t *testing.T) {
			id := "p/" + tt.name
			if tt.forget {
				l.forget(id)
			}
			o := l.update(target{id: id, branch: "issue-7", proj: proj}, "", &PR{Number: 7, State: "open"})
			_, tracked := l.tracks[id]
			if tracked != tt.want || (!tt.want && o != (outcome{})) {
				t.Errorf("tracked %v, outcome %+v, want tracked %v", tracked, o, tt.want)
			}
		})
	}
}

func TestThreadGetsItsPRsCommandsAndOpenedAlert(t *testing.T) {
	l := newLifecycle(nil, nil, t.TempDir(), t.TempDir(), nil, noActivity{}, func(string, any) {})
	proj := project.Project{Name: "p", OnReview: "/address-review {n}"}
	tg := target{id: "p/a", proj: proj, branch: "x"}
	l.update(tg, "", &PR{Number: 7, State: "open"})
	if !l.opened(tg.id) {
		t.Error("the thread did not hear of its new PR")
	}
	if o := l.update(tg, "", &PR{Number: 7, State: "open", Comments: 1}); o.wake != "/address-review 7" {
		t.Errorf("the thread got %q, want the review command", o.wake)
	}
}

func TestReviewAfterACommentIsDeletedWakesTheSession(t *testing.T) {
	l := newLifecycle(nil, nil, t.TempDir(), t.TempDir(), nil, noActivity{}, func(string, any) {})
	proj := project.Project{Name: "p", OnReview: "/address-review {n}"}
	tg := target{id: "p/a", proj: proj, branch: "x"}
	for i, tt := range []struct {
		comments int
		want     string
	}{{3, ""}, {2, ""}, {3, "/address-review 7"}, {3, ""}} {
		if o := l.update(tg, "", &PR{Number: 7, State: "open", Comments: tt.comments}); o.wake != tt.want {
			t.Errorf("fetch %d with %d comments woke with %q, want %q", i, tt.comments, o.wake, tt.want)
		}
	}
}
