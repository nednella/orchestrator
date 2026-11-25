package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/nednella/agentos/internal/host"
)

// newHostCmd is what each thread's tmux pane runs: it starts the agent headless, keeps the chat log and
// serves the thread's socket. Claude is in its own process group, so a Ctrl-C in the pane reaches only this process:
// SIGINT interrupts Claude's turn. SIGTERM cancels the run, and so does SIGHUP, through the context main gives every
// command.
func newHostCmd() *cobra.Command {
	var logPath, socket string
	cmd := &cobra.Command{
		Use:    "host --log <path> --socket <path> -- <command> [args...]",
		Short:  "Run an agent headless and serve its chat (called by the app)",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGTERM)
			defer stop()
			interrupts := make(chan os.Signal, 1)
			signal.Notify(interrupts, os.Interrupt)
			defer signal.Stop(interrupts)
			return host.Run(ctx, host.Config{
				Log:        logPath,
				Socket:     socket,
				Dir:        dir,
				Argv:       args,
				Env:        os.Environ(),
				Echo:       cmd.OutOrStdout(),
				Interrupts: interrupts,
			})
		},
	}
	cmd.Flags().StringVar(&logPath, "log", "", "chat log file")
	cmd.Flags().StringVar(&socket, "socket", "", "unix socket to serve")
	cmd.MarkFlagRequired("log")
	cmd.MarkFlagRequired("socket")
	return cmd
}
