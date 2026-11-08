// Package main is the entry point for the izrss application
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/adrg/xdg"
	"github.com/alecthomas/kong"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/rss"
	"github.com/isabelroses/izrss/internal/storage"
	"github.com/isabelroses/izrss/internal/ui"
)

var Version = "unstable"

// CLI describes the command-line interface.
type CLI struct {
	Config      string           `help:"The path to your config file."`
	CountUnread bool             `help:"Count the number of unread posts."`
	Version     kong.VersionFlag `help:"Print the version and exit."`
}

const description = `An RSS feed reader for the terminal.

The main bulk of customization is done via the "~/.config/izrss/config.toml"
file. You can find an example file on the github page.`

func main() {
	var cli CLI
	kctx := kong.Parse(&cli,
		kong.Name("izrss"),
		kong.Description(description),
		kong.Vars{"version": Version},
		kong.UsageOnError(),
	)

	kctx.FatalIfErrorf(run(&cli))
}

// openLog opens (appending) $XDG_STATE_HOME/izrss/izrss.log.
func openLog() (path string, f *os.File, err error) {
	path, err = xdg.StateFile("izrss/izrss.log")
	if err != nil {
		return "", nil, fmt.Errorf("finding log file: %w", err)
	}
	f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", nil, fmt.Errorf("opening log file: %w", err)
	}
	return path, f, nil
}

func run(cli *CLI) error {
	// Everything logged goes to the log file, never over the TUI.
	logPath, logFile, err := openLog()
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	log.SetOutput(logFile)

	cfg, err := config.Load(cli.Config)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Urls) == 0 {
		fmt.Println("No urls were found in config file, please add some and try again")
		fmt.Println("You can find an example config file on the github page")
		os.Exit(1)
	}

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

	warnings := cfg.Warnings()
	var imgFetcher *images.Fetcher
	if dir, err := images.DefaultCacheDir(); err != nil {
		warnings = append(warnings, "images are off: "+err.Error())
	} else {
		imgFetcher = images.NewFetcher(dir, 10<<20, 15*time.Second)
	}

	_, err = tea.NewProgram(ui.NewModel(cfg, db, fetcher, imgFetcher, warnings)).Run()
	if errors.Is(err, tea.ErrProgramPanic) {
		log.Printf("izrss crashed: %v", err)
		fmt.Fprintf(os.Stderr, "izrss crashed; details were written to %s\n", logPath)
		os.Exit(1)
	}
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}
