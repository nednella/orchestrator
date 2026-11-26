// Package cli holds the binary's entry points: the app, and the hidden commands the app and its agents start.
package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

const noCommandLine = "agentos has no command line"

// Execute runs the binary. With no arguments it runs the app through window; the only commands are the hidden
// ones the app and Claude start.
func Execute(ctx context.Context, window func() error) error {
	return newRootCmd(window).ExecuteContext(ctx)
}

func newRootCmd(window func() error) *cobra.Command {
	root := &cobra.Command{
		Use: "agentos",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return errors.New(noCommandLine)
			}
			return nil
		},
		RunE:          func(*cobra.Command, []string) error { return window() },
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(&cobra.Command{
		Use:    "help",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE:   func(*cobra.Command, []string) error { return errors.New(noCommandLine) },
	})
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) { fmt.Fprintln(cmd.ErrOrStderr(), noCommandLine) })
	root.AddCommand(newHookCmd(), newHostCmd(), newMCPCmd())
	return root
}
