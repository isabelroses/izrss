package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	toastTTL  = 3 * time.Second
	maxToasts = 3
)

type toastKind int

const (
	toastError toastKind = iota
	toastWarn
)

// notifyMsg asks the UI to show a toast; commands return it on failure.
type notifyMsg struct {
	kind toastKind
	text string
}

func notifyErr(text string) tea.Msg  { return notifyMsg{kind: toastError, text: text} }
func notifyWarn(text string) tea.Msg { return notifyMsg{kind: toastWarn, text: text} }

// toastExpiredMsg ends a toast. gen changes when a toast's timer restarts, so
// an older timer can't remove it early.
type toastExpiredMsg struct{ id, gen int }

type toast struct {
	id, gen int
	kind    toastKind
	text    string
	count   int
}

type toastTheme struct{ text, bg, err, warn color.Color }

// toasts is a short stack of transient notifications, newest first.
type toasts struct {
	items []toast
	next  int
	tick  func(time.Duration, func(time.Time) tea.Msg) tea.Cmd // tea.Tick; swapped in tests
}

// add shows a toast, merging it into an identical visible one, and returns
// the command that expires it.
func (t *toasts) add(kind toastKind, text string) tea.Cmd {
	for i := range t.items {
		if t.items[i].kind == kind && t.items[i].text == text {
			t.items[i].count++
			t.items[i].gen++
			return t.timer(t.items[i])
		}
	}
	t.next++
	item := toast{id: t.next, kind: kind, text: text, count: 1}
	t.items = append([]toast{item}, t.items...)
	if len(t.items) > maxToasts {
		t.items = t.items[:maxToasts]
	}
	return t.timer(item)
}

func (t *toasts) timer(item toast) tea.Cmd {
	tick := t.tick
	if tick == nil {
		tick = tea.Tick
	}
	msg := toastExpiredMsg{id: item.id, gen: item.gen}
	return tick(toastTTL, func(time.Time) tea.Msg { return msg })
}

func (t *toasts) expire(msg toastExpiredMsg) {
	for i, item := range t.items {
		if item.id == msg.id && item.gen == msg.gen {
			t.items = append(t.items[:i], t.items[i+1:]...)
			return
		}
	}
}

// view renders the stack, one toast per line, each at most maxWidth wide.
func (t toasts) view(maxWidth int, th toastTheme) string {
	if len(t.items) == 0 || maxWidth < 4 {
		return ""
	}
	lines := make([]string, len(t.items))
	for i, item := range t.items {
		prefix, fg := "✗", th.err
		if item.kind == toastWarn {
			prefix, fg = "!", th.warn
		}
		text := item.text
		if item.count > 1 {
			text = fmt.Sprintf("%s ×%d", text, item.count)
		}
		body := ansi.Truncate(text, maxWidth-4, "…") // " ✗ " + text + " "
		lines[i] = lipgloss.NewStyle().Background(th.bg).Foreground(fg).Render(" "+prefix+" ") +
			lipgloss.NewStyle().Background(th.bg).Foreground(th.text).Render(body+" ")
	}
	return strings.Join(lines, "\n")
}
