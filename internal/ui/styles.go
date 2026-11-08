// Package ui provides the terminal user interface for izrss
package ui

import (
	"cmp"
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/reader"
)

// Styles are the UI's fixed styles, built from the configured palette.
type Styles struct {
	Header lipgloss.Style // reader title line
	Hint   lipgloss.Style // hint descriptions, status, placeholders
	Key    lipgloss.Style // hint keys
	Border lipgloss.Style // pane separator
}

func hex(s string) color.Color { return lipgloss.Color(s) }

// NewStyles creates styles based on the configuration
func NewStyles(cfg *config.Config) *Styles {
	c := cfg.Colors
	return &Styles{
		Header: lipgloss.NewStyle().Bold(true).Foreground(hex(c.Text)),
		Hint:   lipgloss.NewStyle().Foreground(hex(c.Subtext)),
		Key:    lipgloss.NewStyle().Foreground(hex(c.Accent)),
		Border: lipgloss.NewStyle().Foreground(hex(c.Borders)),
	}
}

// readerTheme builds the reader's colours, falling back to the palette.
func readerTheme(cfg *config.Config) reader.Theme {
	c, r := cfg.Colors, cfg.Reader
	return reader.Theme{
		Text:      hex(c.Text),
		Subtle:    hex(c.Subtext),
		Heading:   hex(cmp.Or(r.Heading, c.Accent)),
		Link:      hex(cmp.Or(r.Link, c.Accent)),
		Code:      hex(cmp.Or(r.Code, c.Subtext)),
		Quote:     hex(cmp.Or(r.Quote, c.Subtext)),
		CodeTheme: r.CodeTheme,
	}
}
