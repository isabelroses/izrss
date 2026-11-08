package ui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/isabelroses/izrss/internal/rss"
)

// notify shows a toast and records it in the log file.
func (m *Model) notify(kind toastKind, text string) tea.Cmd {
	log.Print(text)
	return m.toasts.add(kind, text)
}

func (m Model) toastTheme() toastTheme {
	c := m.cfg.Colors
	return toastTheme{text: hex(c.Text), bg: hex(c.Borders), err: lipgloss.Color("1"), warn: lipgloss.Color("3")}
}

// feedName is how toasts refer to a feed: its title, else its URL.
func feedName(f rss.Feed) string {
	if f.Title != "" {
		return f.Title
	}
	return f.URL
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

// imageFailureToast reports the open post's failed images once all of them
// have finished loading, so several failures make one toast.
func (m *Model) imageFailureToast() tea.Cmd {
	if m.imgToasted {
		return nil
	}
	failed := 0
	for _, pi := range m.imgs {
		switch {
		case pi.failed:
			failed++
		case !pi.loaded:
			return nil // still loading
		}
	}
	if failed == 0 {
		return nil
	}
	m.imgToasted = true
	if failed == 1 {
		return m.notify(toastError, "1 image in this post couldn't be loaded")
	}
	return m.notify(toastError, fmt.Sprintf("%d images in this post couldn't be loaded", failed))
}
