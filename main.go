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
    The main bulk of customization is done via the "~/.config/izrss/config.toml" file. You can find an example file on the github page.

    The rest of the config is done via using the environment variables "GLAMOUR_STYLE".
    For a good example see: [catppuccin/glamour](https://github.com/catppuccin/glamour)
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

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "urls",
				Value: "",
				Usage: "the path to your urls file",
			},
		},

		Action: func(c *cli.Context) error {
			p := tea.NewProgram(cmd.NewModel(c.String("urls")), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				log.Fatal(err)
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
