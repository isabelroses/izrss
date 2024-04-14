package main

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/isabelroses/izrss/cmd"
)

var Version = "unstable"

func main() {
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

	app.Run(os.Args)
}
