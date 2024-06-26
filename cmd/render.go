package cmd

import (
	"log"

	tomd "github.com/JohannesKaufmann/html-to-markdown"
)

var htom = tomd.NewConverter("", true, nil)

func (m Model) loadReader() Model {
	id := m.table.Cursor()
	post := m.feed.Posts[id]
	post.ID = id

	m.context = "reader"
	m.post = post
	m.viewport.YPosition = 0 // reset the viewport position

	// render the post
	fromMd, err := htom.ConvertString(post.Content)
	if err != nil {
		log.Fatalf("could not convert html to markdown: %v", err)
	}

	out, err := m.glam.Render(fromMd)
	if err != nil {
		log.Fatalf("could not render markdown: %v", err)
	}
	m.viewport.SetContent(out)

	return m
}
