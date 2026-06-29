package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
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
