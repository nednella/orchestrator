package projects

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/run"
)

// ComputerName is this Mac's name as its owner set it in System Settings, else its host name.
func ComputerName(ctx context.Context, runner run.Runner) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if out, err := runner(ctx, "", "scutil", "--get", "ComputerName"); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	name, _ := os.Hostname()
	return name
}
