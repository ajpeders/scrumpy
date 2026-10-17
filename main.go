package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ajpeders/cider-tui/cider"
	"github.com/ajpeders/cider-tui/ui"
	"github.com/ajpeders/cider-tui/viz"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	flags := flag.NewFlagSet("cider-tui", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	check := flags.Bool("check", false, "check Cider connectivity and library")
	showVersion := flags.Bool("version", false, "print the version")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), `Usage:
  cider-tui                          open the player
  cider-tui --check                  check Cider connectivity and library
  cider-tui --version                print the version

Environment:
  CIDER_TOKEN    Cider API token (default: read from Cider's config)
  CIDER_URL      Cider API address (default: `+cider.DefaultURL+`)
`)
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		return 2
	}
	if *showVersion {
		fmt.Printf("cider-tui %s\n", versionString())
		return 0
	}
	if *check {
		return checkCider()
	}
	client, _, err := cider.NewFromEnvironment()
	if err != nil {
		printError(err)
		return 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames := viz.NewAnalyzer(vizBands).RunFrames(ctx)
	model := ui.New(client).WithSettings().WithFrames(frames)
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		printError(err)
		return 1
	}
	return 0
}

// version is set for release builds with -ldflags "-X main.version=v0.1.0".
var version string

// versionString falls back to the module version, which `go install
// github.com/ajpeders/cider-tui@v0.1.0` records, and "(devel)" for local builds.
func versionString() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

// vizBands is the analyzer's resolution; the UI spreads them across the width.
const vizBands = 48

func checkCider() int {
	client, source, err := cider.NewFromEnvironment()
	if err != nil {
		printError(err)
		return 1
	}
	volume, err := client.Volume()
	if err != nil {
		printError(err)
		return 1
	}
	now, err := client.NowPlaying()
	if err != nil {
		printError(err)
		return 1
	}
	count, err := client.LibraryCount()
	if err != nil {
		printError(err)
		return 1
	}
	title := now.Title()
	if title == "" {
		title = "idle"
	}
	fmt.Printf("Cider: reachable\nToken source: %s\nVolume: %.0f%%\nNow playing: %s\nLibrary songs: %d\n", source, volume*100, title, count)
	if err := viz.Available(); err != nil {
		fmt.Printf("Visualizer: unavailable (%v)\n", err)
	} else {
		fmt.Println("Visualizer: ready")
	}
	return 0
}

func printError(err error) {
	switch {
	case errors.Is(err, cider.ErrNotRunning):
		fmt.Fprintln(os.Stderr, "Cider isn't running; start Cider and retry")
	case errors.Is(err, cider.ErrNoToken), errors.Is(err, cider.ErrBadToken):
		fmt.Fprintln(os.Stderr, "Cider API token missing or rejected; create a token in Cider Settings → Connectivity → Manage External Application Access")
	default:
		fmt.Fprintln(os.Stderr, err)
	}
}
