// Package main is the entry point for the izrss application
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/alecthomas/kong"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
	"github.com/isabelroses/izrss/internal/ui"
)

var version = "unstable"

// CLI describes the command-line interface.
type CLI struct {
	Config      string           `help:"The path to your config file."`
	CountUnread bool             `help:"Count the number of unread posts."`
	Version     kong.VersionFlag `help:"Print the version and exit."`
}

const description = `An RSS feed reader for the terminal.

The main bulk of customization is done via the "~/.config/izrss/config.toml"
file. You can find an example file on the github page.

The rest of the config is done via the "GLAMOUR_STYLE" environment variable.
For a good example see: https://github.com/catppuccin/glamour`

func main() {
	var cli CLI
	kctx := kong.Parse(&cli,
		kong.Name("izrss"),
		kong.Description(description),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	)

	kctx.FatalIfErrorf(run(&cli))
}

func run(cli *CLI) error {
	// Load configuration
	cfg, err := config.Load(cli.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Urls) == 0 {
		fmt.Println("No urls were found in config file, please add some and try again")
		fmt.Println("You can find an example config file on the github page")
		os.Exit(1)
	}

	// Initialize database
	db, err := storage.NewDefault()
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	defer func() { _ = db.Close() }()

	fetcher := rss.NewFetcher(db, cfg.DateFormat)

	if cli.CountUnread {
		feeds := fetcher.GetAllContent(cfg.Urls, true)
		if err := feeds.ReadTracking(db); err != nil {
			return fmt.Errorf("reading tracking data: %w", err)
		}
		fmt.Print(feeds.GetTotalUnreads())
		return nil
	}

	m := ui.NewModel(cfg, db, fetcher)

	// Buffer log output while the alt screen is active so stray errors can't
	// corrupt the display; flush it once the TUI closes.
	var logs bytes.Buffer
	log.SetOutput(&logs)

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, runErr := p.Run()

	log.SetOutput(os.Stderr)
	if logs.Len() > 0 {
		fmt.Fprint(os.Stderr, logs.String())
	}

	if runErr != nil {
		return fmt.Errorf("running TUI: %w", runErr)
	}

	return nil
}
