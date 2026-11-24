package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/nednella/agentos/internal/host"
)

// newHostCmd is what each thread's tmux pane runs: it starts the agent headless, keeps the chat log and
// serves the thread's socket. SIGINT and SIGTERM cancel the run, since Claude is in its own process group
// and a Ctrl-C in the pane reaches only this process.
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
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return host.Run(ctx, host.Config{
				Log:    logPath,
				Socket: socket,
				Dir:    dir,
				Argv:   args,
				Env:    os.Environ(),
				Echo:   cmd.OutOrStdout(),
			})
		},
	}
	cmd.Flags().StringVar(&logPath, "log", "", "chat log file")
	cmd.Flags().StringVar(&socket, "socket", "", "unix socket to serve")
	cmd.MarkFlagRequired("log")
	cmd.MarkFlagRequired("socket")
	return cmd
}
