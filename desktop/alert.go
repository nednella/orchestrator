package main

import "fmt"

const alertTitle = "orchestrator could not start"

// configFailure is what the owner reads when the config does not load: the reason and the file to fix.
func configFailure(err error, configPath string) string {
	return fmt.Sprintf("%v\n\nConfig file: %s\n\nFix the file and open orchestrator again.", err, configPath)
}

// startFailure is what the owner reads when the app opened but could not start following its threads.
func startFailure(err error) string {
	return fmt.Sprintf("orchestrator opened, but could not start its services, so no thread is followed and the tools cannot reach it.\n\n%v\n\nQuit and open orchestrator again.", err)
}
