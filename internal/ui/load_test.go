package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/mattn/go-runewidth"
)

func TestLoadNewTable_PreservesCursor(t *testing.T) {
	cols := []table.Column{{Title: "Title", Width: 20}}
	rows := []table.Row{{"r0"}, {"r1"}, {"r2"}, {"r3"}}

	m := &Model{table: table.New(table.WithFocused(true))}
	m.loadNewTable(cols, rows)
	m.table.SetCursor(2)

	// Reloading the view (clear + swap columns/rows, as every load* does) must
	// keep the cursor in place rather than detaching it. Regression test for the
	// bubbles v1 SetRows cursor clamp.
	m.loadNewTable(cols, rows)

	if got := m.table.Cursor(); got != 2 {
		t.Errorf("expected cursor preserved at 2, got %d", got)
	}
}

// The bubbles v2 table cursor is -1 while the feed list is empty; opening a
// feed then must not panic indexing the empty slice.
func TestLoadContent_EmptyFeedsIsNoOp(t *testing.T) {
	m := &Model{table: table.New(table.WithFocused(true))}

	m.loadContent(-1)
	m.loadContent(0)

	if m.context.curr == "content" {
		t.Errorf("loadContent should not switch to content view with no feeds")
	}
}

func TestLoadReader_EmptyPostsIsNoOp(t *testing.T) {
	m := &Model{table: table.New(table.WithFocused(true))}

	m.loadReader()

	if m.context.curr == "reader" {
		t.Errorf("loadReader should not switch to reader view with no posts")
	}
}

func TestBoldUnread_WrapsWithBoldOff(t *testing.T) {
	out := boldUnread("News", 40)

	if !strings.HasPrefix(out, "\x1b[1m") {
		t.Errorf("expected bold-on prefix, got %q", out)
	}
	// A bold-off (SGR 22) rather than a full reset keeps the selected-row
	// background highlight intact.
	if !strings.HasSuffix(out, "\x1b[22m") {
		t.Errorf("expected bold-off suffix, got %q", out)
	}
	if strings.Contains(out, "…") {
		t.Errorf("a short title should not be truncated: %q", out)
	}
}

func TestBoldUnread_StaysWithinColumnWidth(t *testing.T) {
	const width = 20
	out := boldUnread("a very long feed title that does not fit the column", width)

	// The bubbles table truncates cells with an ANSI-unaware width function, so
	// the styled string must measure no wider than the column or the table would
	// cut one of our escape sequences.
	if got := runewidth.StringWidth(out); got > width {
		t.Errorf("styled width %d exceeds column width %d: %q", got, width, out)
	}
	if !strings.HasSuffix(out, "\x1b[22m") {
		t.Errorf("expected trailing bold-off after truncation, got %q", out)
	}
}
