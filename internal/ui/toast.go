package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"log"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/rss"
)

const (
	toastTTL  = 6500 * time.Millisecond
	maxToasts = 3
)

// toastExpiredMsg ends a toast. gen changes when a toast's timer restarts, so
// an older timer can't remove it early.
type toastExpiredMsg struct{ id, gen int }

type toast struct {
	id, gen int
	text    string
	count   int
}

type toastTheme struct{ text, subtle, err color.Color }

// toasts is a short stack of transient notifications, newest first.
type toasts struct {
	items []toast
	next  int
	tick  func(time.Duration, func(time.Time) tea.Msg) tea.Cmd // tea.Tick; swapped in tests
}

// add shows a toast, merging it into an identical visible one, and returns
// the command that expires it.
func (t *toasts) add(text string) tea.Cmd {
	for i := range t.items {
		if t.items[i].text == text {
			t.items[i].count++
			t.items[i].gen++
			return t.timer(t.items[i])
		}
	}
	t.next++
	item := toast{id: t.next, text: text, count: 1}
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
			t.items = slices.Delete(t.items, i, i+1)
			return
		}
	}
}

// view renders each toast, newest first, as its own rounded box like the
// help overlay's, at most maxWidth wide, its text cut to one line.
func (t toasts) view(maxWidth int, th toastTheme) []string {
	const frame = 4 // the border and a cell of padding each side
	if maxWidth < frame+4 {
		return nil
	}
	mark := lipgloss.NewStyle().Foreground(th.err).Render("✗ ")
	text := lipgloss.NewStyle().Foreground(th.text)
	subtle := lipgloss.NewStyle().Foreground(th.subtle)
	boxes := make([]string, len(t.items))
	for i, item := range t.items {
		count := ""
		if item.count > 1 {
			count = fmt.Sprintf(" ×%d", item.count)
		}
		room := max(maxWidth-frame-2-ansi.StringWidth(count), 1) // 2: the mark
		line := mark + text.Render(ansi.Truncate(item.text, room, "…")) + subtle.Render(count)
		boxes[i] = box(line, ansi.StringWidth(line)+frame, th.err)
	}
	return boxes
}

// notify shows a toast and records it in the log file. Error text can carry
// server-supplied bytes, so control characters are dropped before display.
func (m *Model) notify(text string) tea.Cmd {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	log.Print(text)
	return m.toasts.add(text)
}

// shortReason turns a fetch error into a few words for a toast.
func shortReason(err error) string {
	var (
		ne     net.Error
		dns    *net.DNSError
		parse  *rss.ParseError
		status *rss.StatusError
	)
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "timed out"
	case errors.As(err, &dns) && dns.IsNotFound:
		return "couldn't find host"
	case errors.As(err, &parse):
		return "couldn't parse feed"
	case errors.As(err, &status):
		return "HTTP " + strconv.Itoa(status.Code)
	}
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}
