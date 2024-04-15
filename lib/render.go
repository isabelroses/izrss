package lib

import (
	"log"

	tomd "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/charmbracelet/glamour"
)

var (
	glam, _ = glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithAutoStyle(),
		glamour.WithEnvironmentConfig(),

		glamour.WithWordWrap(100),
	)

	htom = tomd.NewConverter("", true, nil)
)

func RenderMarkdown(content string) string {
	fromMd, err := htom.ConvertString(content)
	if err != nil {
		log.Fatalf("could not convert html to markdown: %v", err)
	}

	out, err := glam.Render(fromMd)
	if err != nil {
		log.Fatalf("could not render markdown: %v", err)
	}

	return out
}
