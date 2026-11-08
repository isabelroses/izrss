package reader

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

type tableRow struct {
	cells  []string
	header bool
}

// table renders a simple grid with lipgloss/table, as glamour does: inner
// borders only, natural column widths, shrunk and truncated only when the
// table would overflow.
func (r *renderer) table(n *html.Node) block {
	rows := tableRows(n)
	cols := 0
	for _, row := range rows {
		cols = max(cols, len(row.cells))
	}
	if cols == 0 {
		return block{}
	}

	// Built fresh for each render: a lipgloss table keeps layout state after
	// String, and re-rendering it with a new width drops the header rule.
	build := func() *table.Table {
		text := lipgloss.NewStyle().Foreground(r.o.Theme.Text)
		t := table.New().
			Border(lipgloss.NormalBorder()).
			BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
			BorderRow(false).BorderColumn(true).
			BorderStyle(lipgloss.NewStyle().Foreground(r.o.Theme.Subtle)).
			Wrap(false).
			StyleFunc(func(row, col int) lipgloss.Style {
				st := text
				if row == table.HeaderRow {
					st = st.Bold(true)
				}
				return st.Padding(0, 1) // as glamour pads every cell
			})
		data := rows
		if rows[0].header {
			t.Headers(pad(rows[0].cells, cols)...)
			data = rows[1:]
		} else {
			t.BorderHeader(false)
		}
		for _, row := range data {
			t.Row(pad(row.cells, cols)...)
		}
		return t
	}

	out := build().String()
	if lipgloss.Width(out) > r.width {
		out = build().Width(r.width).String()
	}
	var b block
	for line := range strings.SplitSeq(out, "\n") {
		b.lines = append(b.lines, ansi.Truncate(line, r.width, "…"))
	}
	return b
}

// pad extends cells with empty strings to n columns; n is never fewer.
func pad(cells []string, n int) []string {
	return append(cells, make([]string, n-len(cells))...)
}

// tableRows collects rows from the table, its thead/tbody/tfoot sections.
func tableRows(n *html.Node) []tableRow {
	var rows []tableRow
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "thead", "tbody", "tfoot":
				visit(c)
			case "tr":
				rows = append(rows, tableRowOf(c))
			}
		}
	}
	visit(n)
	return rows
}

func tableRowOf(tr *html.Node) tableRow {
	row := tableRow{header: true}
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || (c.Data != "td" && c.Data != "th") {
			continue
		}
		row.header = row.header && c.Data == "th"
		row.cells = append(row.cells, strings.Join(strings.Fields(textContent(c)), " "))
	}
	if len(row.cells) == 0 {
		row.header = false
	}
	return row
}
