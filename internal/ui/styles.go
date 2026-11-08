// Package ui provides the terminal user interface for izrss
package ui

import (
	"cmp"
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/reader"
)

// Styles are the UI's fixed styles, built from the configured palette.
type Styles struct {
	Header lipgloss.Style // reader title line
	Hint   lipgloss.Style // hint descriptions, status, placeholders
	Key    lipgloss.Style // hint keys
	Border lipgloss.Style // pane separator
	Bar    string         // opens the hint bar's background

	List   listTheme    // feed, post and palette rows
	Toast  toastTheme   // toasts
	Reader reader.Theme // the open post
}

func hex(s string) color.Color { return lipgloss.Color(s) }

// NewStyles creates styles based on the configuration
func NewStyles(cfg *config.Config) *Styles {
	c, r := cfg.Colors, cfg.Reader
	return &Styles{
		Header: lipgloss.NewStyle().Bold(true).Foreground(hex(c.Text)),
		Hint:   lipgloss.NewStyle().Foreground(hex(c.Subtext)),
		Key:    lipgloss.NewStyle().Foreground(hex(c.Accent)),
		Border: lipgloss.NewStyle().Foreground(hex(c.Borders)),
		Bar:    ansi.Style{}.BackgroundColor(hex(c.Borders)).String(),

		List:  newListTheme(hex(c.Text), hex(c.Subtext), hex(c.Accent), hex(c.Inverttext), hex(c.Borders)),
		Toast: toastTheme{text: hex(c.Text), subtle: hex(c.Subtext), err: lipgloss.Color("1")},
		Reader: reader.Theme{
			Text:      hex(c.Text),
			Subtle:    hex(c.Subtext),
			Heading:   hex(cmp.Or(r.Heading, c.Accent)),
			Chip:      hex(c.Inverttext),
			Link:      hex(cmp.Or(r.Link, c.Accent)),
			Code:      hex(cmp.Or(r.Code, c.Subtext)),
			Quote:     hex(cmp.Or(r.Quote, c.Subtext)),
			CodeTheme: r.CodeTheme,
		},
	}
}
