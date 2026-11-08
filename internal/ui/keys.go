package ui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// keyMap is every key binding. The same bindings drive key handling, the hint
// line and the help overlay, so they can't drift apart.
type keyMap struct {
	// Movement, matched individually but shown as one hint each.
	Up, Down, JumpUp, JumpDown, Left, Right, Tab key.Binding
	Move, Jump, Pane                             key.Binding // help only

	Open, Toggle, MarkFeed, Refresh, RefreshAll, Browser key.Binding
	Search, Help, Quit                                   key.Binding

	// Reader.
	Back, ForceQuit key.Binding
	Scroll, Page    key.Binding // help only: the viewport handles these
}

func bind(keys []string, k, desc string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(k, desc))
}

var bindings = keyMap{
	Up:       key.NewBinding(key.WithKeys("up", "k")),
	Down:     key.NewBinding(key.WithKeys("down", "j")),
	JumpUp:   key.NewBinding(key.WithKeys("K", "shift+up")),
	JumpDown: key.NewBinding(key.WithKeys("J", "shift+down")),
	Left:     key.NewBinding(key.WithKeys("left", "h")),
	Right:    key.NewBinding(key.WithKeys("right", "l")),
	Tab:      key.NewBinding(key.WithKeys("tab")),
	Move:     bind([]string{"up", "down", "k", "j"}, "↑↓", "move"),
	Jump:     bind([]string{"K", "J", "shift+up", "shift+down"}, "J K", "jump 5"),
	Pane:     bind([]string{"left", "right", "h", "l", "tab"}, "←→", "pane"),

	Open:       bind([]string{"enter"}, "⏎", "open"),
	Toggle:     bind([]string{"x"}, "x", "toggle read"),
	MarkFeed:   bind([]string{"X"}, "X", "mark feed read"),
	Refresh:    bind([]string{"r"}, "r", "refresh feed"),
	RefreshAll: bind([]string{"R"}, "R", "refresh all"),
	Browser:    bind([]string{"o"}, "o", "open in browser"),
	Search:     bind([]string{"/"}, "/", "search"),
	Help:       bind([]string{"?"}, "?", "help"),
	Quit:       bind([]string{"q", "ctrl+c"}, "q", "quit"),

	Back:      bind([]string{"esc", "q", "h", "left"}, "esc", "back"),
	ForceQuit: bind([]string{"ctrl+c"}, "ctrl+c", "quit"),
	Scroll:    bind([]string{"up", "down", "k", "j"}, "↑↓", "scroll"),
	Page:      bind([]string{"pgup", "pgdown", "space"}, "pgup pgdn", "page"),
}

// shortHelp is the hint line for a screen and focus.
func (k keyMap) shortHelp(s screen, focus pane) []key.Binding {
	switch {
	case s == screenReader:
		return []key.Binding{k.Scroll, k.Back, k.Browser, k.Toggle, k.Help}
	case focus == panePosts:
		return []key.Binding{k.Move, k.Pane, k.Open, k.Toggle, k.Search, k.Help}
	}
	return []key.Binding{k.Move, k.Pane, k.Open, k.Search, k.Help}
}

// fullHelp is the help overlay for a screen, in columns.
func (k keyMap) fullHelp(s screen) [][]key.Binding {
	if s == screenReader {
		return [][]key.Binding{
			{k.Scroll, k.Page, k.Back},
			{k.Toggle, k.Browser, k.Search, k.ForceQuit},
		}
	}
	return [][]key.Binding{
		{k.Move, k.Jump, k.Pane, k.Open, k.Search, k.Quit},
		{k.Toggle, k.MarkFeed, k.Refresh, k.RefreshAll, k.Browser},
	}
}

// newHelp styles bubbles' help renderer with the palette.
func newHelp(st *Styles) help.Model {
	h := help.New()
	h.ShortSeparator = "  "
	h.FullSeparator = "    "
	h.Styles = help.Styles{
		ShortKey:       st.Key,
		ShortDesc:      st.Hint,
		ShortSeparator: st.Hint,
		Ellipsis:       st.Hint,
		FullKey:        st.Key,
		FullDesc:       st.Hint,
		FullSeparator:  lipgloss.NewStyle(),
	}
	return h
}
