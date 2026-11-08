package ui

import (
	"image/color"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/config"
)

// NewModel must not query the terminal; it assumes dark until the
// asynchronous background-color reply arrives.
func TestBackgroundColorMsg_SetsReaderStyle(t *testing.T) {
	textFg := func(m Model) color.Color { return m.palette.input.Styles().Blurred.Text.GetForeground() }
	wantFg := func(dark bool) color.Color { return textinput.DefaultStyles(dark).Blurred.Text.GetForeground() }
	tests := []struct {
		name string
		bg   color.Color
		want bool
	}{
		{"light terminal", color.White, false},
		{"dark terminal", color.Black, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(&config.Config{}, nil, nil, nil)
			if textFg(*m) != wantFg(true) {
				t.Fatalf("expected dark default before the reply")
			}

			updated, _ := m.Update(tea.BackgroundColorMsg{Color: tt.bg})

			if got := textFg(updated.(Model)); got != wantFg(tt.want) {
				t.Errorf("text colour = %v, want the isDark=%v style %v", got, tt.want, wantFg(tt.want))
			}
		})
	}
}
