// Package main is the entry point for the application
package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v2"

	"github.com/isabelroses/izrss/cmd"
)

const version = "unstable"

func main() {
	cli.AppHelpTemplate = fmt.Sprintf(`%s
CUSTOMIZATION:
    You can customise the colours using "GLAMOUR_STYLE" for a good example see https://github.com/catppuccin/glamour`,
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

		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				p := tea.NewProgram(cmd.NewModel(), tea.WithAltScreen())
				if _, err := p.Run(); err != nil {
					log.Fatal(err)
				}
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
