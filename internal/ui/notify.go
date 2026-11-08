package ui

import (
	"context"
	"errors"
	"log"
	"net"
	"regexp"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// notify shows a toast and records it in the log file. Error text can carry
// server-supplied bytes, so control characters are dropped before display.
func (m *Model) notify(kind toastKind, text string) tea.Cmd {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
	log.Print(text)
	return m.toasts.add(kind, text)
}

func (m Model) toastTheme() toastTheme {
	c := m.cfg.Colors
	return toastTheme{text: hex(c.Text), bg: hex(c.Borders), err: lipgloss.Color("1"), warn: lipgloss.Color("3")}
}

var httpStatus = regexp.MustCompile(`: (\d{3}) [A-Z]`)

// shortReason turns a fetch error into a few words for a toast.
func shortReason(err error) string {
	var ne net.Error
	msg := err.Error()
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "timed out"
	case strings.Contains(msg, "no such host"):
		return "couldn't find host"
	case strings.HasPrefix(msg, "parsing "):
		return "couldn't parse feed"
	}
	if s := httpStatus.FindStringSubmatch(msg); s != nil {
		return "HTTP " + s[1]
	}
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		return msg[i+2:]
	}
	return msg
}
