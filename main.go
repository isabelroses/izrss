// Package main is the entry point for the izrss application
package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
	"github.com/isabelroses/izrss/internal/ui"
)

var version = "unstable"

func main() {
	cli.AppHelpTemplate = fmt.Sprintf(`%s
CUSTOMIZATION:
    The main bulk of customization is done via the "~/.config/izrss/config.toml" file. You can find an example file on the github page.

    The rest of the config is done via using the environment variables "GLAMOUR_STYLE".
    For a good example see: <https://github.com/catppuccin/glamour>`,
		cli.AppHelpTemplate,
	)

	app := &cli.App{
		Name:    "izrss",
		Version: version,
		Authors: []*cli.Author{{
			Name:  "Isabel Roses",
			Email: "isabel@isabelroses.com",
		}},
		Usage: "An RSS feed reader for the terminal.",

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "",
				Usage: "the path to your config file",
			},
			&cli.BoolFlag{
				Name:  "count-unread",
				Usage: "count the number of unread posts",
			},
		},

		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	// Load configuration
	cfg, err := config.Load(c.String("config"))
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

	// Create fetcher and load feeds
	fetcher := rss.NewFetcher(db, cfg.DateFormat)

	// Handle count-unread flag - needs synchronous loading
	if c.Bool("count-unread") {
		feeds := fetcher.GetAllContent(cfg.Urls, fetcher.CheckCache())
		if err := feeds.ReadTracking(db); err != nil {
			return fmt.Errorf("reading tracking data: %w", err)
		}
		fmt.Print(feeds.GetTotalUnreads())
		return nil
	}

	// Run the TUI with async feed loading
	m := ui.NewModel(cfg, db, fetcher)
	m.StartAsyncLoading()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
