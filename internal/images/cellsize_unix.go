//go:build unix

package images

import "golang.org/x/sys/unix"

// CellSize reads the cell size in pixels from the tty, without querying the
// terminal. It returns the zero Cell when the size is unavailable.
func CellSize(fd int) Cell {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
		return Cell{}
	}
	return Cell{W: int(ws.Xpixel / ws.Col), H: int(ws.Ypixel / ws.Row)}
}
