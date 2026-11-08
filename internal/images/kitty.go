package images

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	"golang.org/x/sys/unix"
)

// Upload returns the escape sequence that transmits p as image id with a
// virtual placement of p.Cols×p.Rows cells, drawn wherever Placeholders appear.
func Upload(id int, p Prepared) (string, error) {
	var b strings.Builder
	err := kitty.EncodeGraphics(&b, p.Img, &kitty.Options{
		Action:           kitty.TransmitAndPut,
		Transmission:     kitty.Direct,
		Format:           kitty.PNG,
		ID:               id,
		Columns:          p.Cols,
		Rows:             p.Rows,
		VirtualPlacement: true,
		Quiet:            2,
		Chunk:            true,
	})
	if err != nil {
		return "", fmt.Errorf("encoding image: %w", err)
	}
	return b.String(), nil
}

// Place re-sizes an uploaded image's virtual placement without re-sending it.
func Place(id, cols, rows int) string {
	return ansi.KittyGraphics(nil, "a=p", "U=1", "q=2", fmt.Sprintf("i=%d", id), fmt.Sprintf("c=%d", cols), fmt.Sprintf("r=%d", rows))
}

// Delete frees an uploaded image and its data in the terminal.
func Delete(id int) string {
	return ansi.KittyGraphics(nil, "a=d", "d=I", "q=2", fmt.Sprintf("i=%d", id))
}

// Placeholders returns the text rows kitty replaces with image id: each cell
// is the placeholder rune plus row and column diacritics, coloured with the id.
func Placeholders(id, cols, rows int) []string {
	st := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(id))
	out := make([]string, rows)
	var line strings.Builder
	for r := range rows {
		line.Reset()
		for c := range cols {
			line.WriteRune(kitty.Placeholder)
			line.WriteRune(kitty.Diacritic(r))
			line.WriteRune(kitty.Diacritic(c))
		}
		out[r] = st.Render(line.String())
	}
	return out
}

// Enabled reports whether to draw images: "auto" (or empty) means terminals
// known to support kitty Unicode placeholders, outside tmux/screen.
func Enabled(mode string, getenv func(string) string) bool {
	switch mode {
	case "on":
		return true
	case "off":
		return false
	}
	if getenv("TMUX") != "" || getenv("STY") != "" {
		return false
	}
	term := getenv("TERM")
	return getenv("TERM_PROGRAM") == "ghostty" || term == "xterm-ghostty" ||
		term == "xterm-kitty" || getenv("KITTY_WINDOW_ID") != ""
}

// CellSize reads the cell size in pixels from the tty, without querying the
// terminal. It returns the zero Cell when the size is unavailable.
func CellSize(fd int) Cell {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
		return Cell{}
	}
	return Cell{W: int(ws.Xpixel / ws.Col), H: int(ws.Ypixel / ws.Row)}
}
