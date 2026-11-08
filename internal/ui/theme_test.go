package ui

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/isabelroses/izrss/internal/config"
)

func TestReaderTheme_FallsBackToPalette(t *testing.T) {
	cfg := config.Default()
	th := NewStyles(cfg).Reader

	tests := []struct {
		name string
		got  any
		hex  string
	}{
		{"Text", th.Text, cfg.Colors.Text},
		{"Subtle", th.Subtle, cfg.Colors.Subtext},
		{"Heading", th.Heading, cfg.Colors.Accent},
		{"Link", th.Link, cfg.Colors.Accent},
		{"Code", th.Code, cfg.Colors.Subtext},
		{"Quote", th.Quote, cfg.Colors.Subtext},
	}
	for _, tt := range tests {
		if tt.got != lipgloss.Color(tt.hex) {
			t.Errorf("%s = %v, want %s", tt.name, tt.got, tt.hex)
		}
	}
	if th.CodeTheme != "catppuccin-mocha" {
		t.Errorf("CodeTheme = %q", th.CodeTheme)
	}

	cfg.Reader.Heading = "#ff0000"
	if got := NewStyles(cfg).Reader.Heading; got != lipgloss.Color("#ff0000") {
		t.Errorf("Heading override = %v, want #ff0000", got)
	}
}
