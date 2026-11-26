package projects_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/projects"
)

func TestComputerName(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		what string
		out  string
		err  error
		want string
	}{
		{"the name scutil gives", "Ned’s MacBook Pro\n", nil, "Ned’s MacBook Pro"},
		{"the host name when scutil fails", "", errors.New("scutil: not found"), host},
		{"the host name when no name is set", "  \n", nil, host},
	} {
		t.Run(tc.what, func(t *testing.T) {
			var called string
			runner := func(_ context.Context, dir, name string, args ...string) ([]byte, error) {
				called = strings.Join(append([]string{name}, args...), " ")
				return []byte(tc.out), tc.err
			}
			if got := projects.ComputerName(context.Background(), runner); got != tc.want {
				t.Errorf("ComputerName = %q, want %q", got, tc.want)
			}
			if called != "scutil --get ComputerName" {
				t.Errorf("ran %q", called)
			}
		})
	}
}
