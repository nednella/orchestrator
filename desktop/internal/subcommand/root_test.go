package subcommand

import (
	"bytes"
	"slices"
	"testing"
)

func TestRootHasOnlyHiddenEntryPoints(t *testing.T) {
	root := newRootCmd(nil)
	var names []string
	for _, c := range root.Commands() {
		names = append(names, c.Name())
		if !c.Hidden {
			t.Errorf("command %s is not hidden", c.Name())
		}
	}
	slices.Sort(names)
	if want := []string{"host", "mcp"}; !slices.Equal(names, want) {
		t.Errorf("commands = %v, want %v", names, want)
	}
}

func TestNoArgumentsRunsTheApp(t *testing.T) {
	ran := 0
	root := newRootCmd(func() error { ran++; return nil })
	root.SetArgs(nil)
	if err := root.Execute(); err != nil || ran != 1 {
		t.Errorf("Execute() = %v, the app ran %d times, want nil and 1", err, ran)
	}
}

func TestAnythingElseHasNoCommandLine(t *testing.T) {
	for _, args := range [][]string{{"note"}, {"hook", "Stop"}, {"settle", "3"}, {"version"}, {"update"}, {"help"}, {"--help"}, {"completion"}, {"open", "x"}} {
		t.Run(args[0], func(t *testing.T) {
			ran := false
			var out, errOut bytes.Buffer
			root := newRootCmd(func() error { ran = true; return nil })
			root.SetOut(&out)
			root.SetErr(&errOut)
			root.SetArgs(args)
			err := root.Execute()
			if ran {
				t.Error("the app ran")
			}
			if args[0] == "--help" {
				if errOut.String() != noCommandLine+"\n" {
					t.Errorf("--help printed %q", errOut.String())
				}
				return
			}
			if err == nil || err.Error() != noCommandLine {
				t.Errorf("Execute() = %v, want %q", err, noCommandLine)
			}
		})
	}
}
