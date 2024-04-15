package main

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/isabelroses/izrss/cmd"
)

var Version = "unstable"

func main() {
	cli.AppHelpTemplate = fmt.Sprintf(`%s
CUSTOMIZATION:
    You can customise the colours using "GLAMOUR_STYLE" for a good example see https://github.com/catppuccin/glamour`,
		cli.AppHelpTemplate,
	)

	app := &cli.App{
		Name:    "izrss",
		Version: Version,
		Authors: []*cli.Author{{
			Name:  "Isabel Roses",
			Email: "isabel@isabelroses.com",
		}},
		Usage: "Read your favorite news stories from the terminal",

		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				cmd.Run()
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
