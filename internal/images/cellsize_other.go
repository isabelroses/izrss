//go:build !unix

package images

// CellSize returns the zero Cell: there is no TIOCGWINSZ to read the pixel
// size from on this platform.
func CellSize(int) Cell { return Cell{} }
