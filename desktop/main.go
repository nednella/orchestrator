// Command desktop is orchestrator: the app, and the hidden commands its host and tools' server run.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/nednella/orchestrator/desktop/awake"
	"github.com/nednella/orchestrator/desktop/devhttp"
	"github.com/nednella/orchestrator/desktop/internal/app"
	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/internal/subcommand"
	"github.com/nednella/orchestrator/desktop/projects"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	if err := subcommand.Execute(ctx, launch); err != nil {
		fmt.Fprintln(os.Stderr, "orchestrator:", err)
		os.Exit(1)
	}
}

func launch() error {
	fixLocale()
	if err := fixPath(); err != nil {
		log.Printf("orchestrator: %v", err)
	}
	cfg, err := app.Load()
	if err != nil {
		if os.Getenv("ORCHESTRATOR_HTTP") == "" { // a check in a browser has a terminal to read
			home, _ := os.UserHomeDir()
			showAlert(alertTitle, configFailure(err, app.ConfigPath(home)))
		}
		return err
	}
	assets, err := fs.Sub(assetsFS, assetsRoot)
	if err != nil {
		return err
	}
	if addr := os.Getenv("ORCHESTRATOR_HTTP"); addr != "" {
		hub := devhttp.NewHub()
		a := app.New(cfg, app.Host{
			Emit: hub.Emit, Clipboard: func(string) {}, PickDir: func() (string, error) { return "", nil },
			Quit:  func() { _ = syscall.Kill(os.Getpid(), syscall.SIGTERM) },
			Awake: awake.Caffeinate, Machine: computerName,
		}, run.Exec, run.Stream, run.ExecEnv)
		quitOnSignal(a)
		return devhttp.Serve(devhttp.Options{Services: a.Services(), Start: a.Start, Stop: func() { stop(a) }, Hub: hub, Assets: assets, Handler: a.Assets(), Addr: addr})
	}

	var window atomic.Pointer[context.Context]
	a := app.New(cfg, app.Host{
		Emit: func(event string, payload any) {
			if c := window.Load(); c != nil {
				runtime.EventsEmit(*c, event, payload)
			}
		},
		Clipboard: func(text string) {
			if c := window.Load(); c != nil {
				_ = runtime.ClipboardSetText(*c, text)
			}
		},
		PickDir: func() (string, error) {
			return runtime.OpenDirectoryDialog(*window.Load(), runtime.OpenDialogOptions{Title: "Choose a folder", CanCreateDirectories: true})
		},
		Quit: func() {
			if c := window.Load(); c != nil {
				runtime.Quit(*c)
			}
		},
		Awake:   awake.Caffeinate,
		Machine: computerName,
	}, run.Exec, run.Stream, run.ExecEnv)
	quitOnSignal(a)

	return wails.Run(&options.App{
		Title:            "orchestrator",
		Width:            1500,
		Height:           950,
		MinWidth:         420,
		MinHeight:        360,
		BackgroundColour: &options.RGBA{R: 7, G: 9, B: 13, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets, Handler: a.Assets()},
		OnStartup: func(c context.Context) {
			window.Store(&c)
			stripMenuShortcuts()
			compactTitleBar()
			acceptFirstMouse()
			if err := a.Start(c); err != nil {
				log.Printf("orchestrator: %v", err)
				_, _ = runtime.MessageDialog(c, runtime.MessageDialogOptions{Type: runtime.ErrorDialog, Title: alertTitle, Message: startFailure(err)})
			}
		},
		OnDomReady: func(context.Context) { refreshKeyWindow() },
		OnShutdown: func(context.Context) { stop(a) },
		Bind:       a.Services(),
		// The Edit menu is what makes ⌘C, ⌘V, ⌘X and ⌘A reach the web view on macOS.
		Menu: menu.NewMenuFromItems(menu.AppMenu(), menu.EditMenu(), menu.WindowMenu()),
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
	})
}

// quitOnSignal stops the app before the process ends.
func quitOnSignal(a *app.App) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		<-quit
		stop(a)
		os.Exit(0)
	}()
}

// stop stops the app and every command it still runs, which would outlive it.
func stop(a *app.App) {
	a.Stop()
	run.StopAll()
}

func computerName() string { return projects.ComputerName(context.Background(), run.Exec) }
