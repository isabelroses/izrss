// Package main is the entry point for the application
package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/urfave/cli/v2"

	"github.com/isabelroses/izrss/cmd"
	"github.com/isabelroses/izrss/lib"
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
		},

		Action: func(c *cli.Context) error {
			lib.LoadConfig(c.String("config"))

			if len(lib.UserConfig.Urls) == 0 {
				fmt.Println("No urls were found in config file, please add some and try again")
				fmt.Println("You can find an example config file on the github page")
				os.Exit(1)
			}

			p := tea.NewProgram(cmd.NewModel(), tea.WithAltScreen())
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
