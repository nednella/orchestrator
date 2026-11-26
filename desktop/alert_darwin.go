//go:build darwin

package main

import (
	"context"
	"log"
	"os/exec"
	"time"
)

// showAlert shows a native alert and returns once the owner closes it. The text goes in as arguments of the script,
// never into its source.
func showAlert(title, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	script := []string{"-e", "on run argv", "-e", "display alert (item 1 of argv) message (item 2 of argv) as critical", "-e", "end run"}
	if out, err := exec.CommandContext(ctx, "osascript", append(script, title, message)...).CombinedOutput(); err != nil {
		log.Printf("orchestrator: showing the alert: %v: %s", err, out)
	}
}
