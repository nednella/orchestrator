package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/term"
)

const defaultTmuxSocket = "agentos"

// env is what the commands that touch sessions need.
type env struct {
	project  project.Project
	stateDir string
	tmux     *term.Tmux
}

func loadEnv() (env, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return env{}, fmt.Errorf("finding the current folder: %w", err)
	}
	path := os.Getenv("AGENTOS_CONFIG")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return env{}, fmt.Errorf("finding home dir: %w", err)
		}
		path = filepath.Join(home, ".config", "agentos", "config.yaml")
	}
	cfg, err := project.Load(path)
	if err != nil {
		return env{}, fmt.Errorf("loading config: %w", err)
	}
	stateDir, err := bus.DefaultDir()
	if err != nil {
		return env{}, err
	}
	socket := os.Getenv("AGENTOS_TMUX_SOCKET")
	if socket == "" {
		socket = defaultTmuxSocket
	}
	tmux, err := term.NewTmux(socket, stateDir)
	if err != nil {
		return env{}, err
	}
	return env{project: cfg.Resolve(cwd), stateDir: stateDir, tmux: tmux}, nil
}

func newSettleCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "settle [n]",
		Short:   "Put thread n to sleep in the app, or without n this project's threads (--all: every project, shells too)",
		GroupID: groupWork,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return askApp(cmd, control.Request{Cmd: "settle", Args: args})
			}
			e, err := loadEnv()
			if err != nil {
				return err
			}
			infos, shells, err := e.tmux.ListAll(cmd.Context())
			if err != nil {
				return fmt.Errorf("listing threads: %w", err)
			}
			out := cmd.OutOrStdout()
			stopped, left := 0, 0
			for _, in := range infos {
				if !all && in.Name.Project != e.project.Key() {
					continue
				}
				// A session that ran in a terminal before the host is the owner's to end there.
				if !in.Hosted {
					left++
					continue
				}
				// The state file stays: the app shows the thread as settled from it.
				fmt.Fprintf(out, "putting %s (%s) to sleep\n", in.Name, in.Title)
				if err := e.tmux.Kill(cmd.Context(), in.Name); err != nil {
					return err
				}
				stopped++
			}
			if all { // a shell is only stopped when everything is
				for _, name := range shells {
					fmt.Fprintf(out, "stopping %s\n", name)
					if err := e.tmux.Kill(cmd.Context(), name); err != nil {
						return err
					}
					stopped++
				}
			}
			if stopped == 0 {
				fmt.Fprintln(out, "no threads to put to sleep")
			}
			switch {
			case left == 1:
				fmt.Fprintln(out, "left 1 session that ran in a terminal")
			case left > 1:
				fmt.Fprintf(out, "left %d sessions that ran in a terminal\n", left)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "put the threads of every project to sleep, and stop the shells")
	return cmd
}
