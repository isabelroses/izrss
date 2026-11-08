package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/imgsession"
	"github.com/isabelroses/izrss/internal/rss"
)

// newPage is a reader page on its own: no Model, no database.
func newPage(t *testing.T, width, height int) readerPage {
	t.Helper()
	cfg := config.Default()
	p := newReaderPage(&cfg.Reader, cfg.DateFormat, NewStyles(cfg).Reader, imgsession.New(nil, false))
	p.Resize(width, height)
	return p
}

func TestPage_ReadEnoughFollowsScroll(t *testing.T) {
	p := newPage(t, 80, 20)
	if p.ReadEnough() {
		t.Error("a closed page has read nothing")
	}
	var b strings.Builder
	for range 100 {
		b.WriteString("<p>line</p>")
	}
	p.Open(rss.PostRef{FeedURL: "f", Key: "long"}, rss.Post{Content: b.String()})
	if p.ReadEnough() {
		t.Error("a long post is not read on opening")
	}
	for range 20 {
		p.Scroll(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if !p.ReadEnough() {
		t.Error("scrolled to the end, the post is read")
	}
}

func TestPage_RefreshKeepsScrollAndShowsNewContent(t *testing.T) {
	p := newPage(t, 80, 20)
	var b strings.Builder
	for range 100 {
		b.WriteString("<p>old</p>")
	}
	ref := rss.PostRef{FeedURL: "f", Key: "k"}
	p.Open(ref, rss.Post{Content: b.String()})
	p.viewport.SetYOffset(30)
	p.Refresh(rss.Post{Content: strings.ReplaceAll(b.String(), "old", "new")})
	if p.viewport.YOffset() != 30 {
		t.Errorf("refresh moved the reader to %d", p.viewport.YOffset())
	}
	if view := ansi.Strip(p.View("feed", NewStyles(config.Default()), "")); !strings.Contains(view, "new") || strings.Contains(view, "old") {
		t.Errorf("refresh should show the new content:\n%s", view)
	}
}
