package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // register decoders; GIFs decode to their first frame
	_ "image/jpeg"
	_ "image/png"
	"math"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var (
	ErrTooSmall = errors.New("image is a tracking pixel")
	ErrTooBig   = errors.New("image dimensions are too large")
)

// maxPixels bounds decoding so a hostile header can't exhaust memory.
const maxPixels = 40_000_000

// Cell is the terminal cell size in pixels. The zero value means unknown.
type Cell struct{ W, H int }

func (c Cell) orDefault() Cell {
	if c.W <= 0 || c.H <= 0 {
		return Cell{8, 16}
	}
	return c
}

// cssPerColumn is the CSS pixels per terminal column; sizing by CSS pixels
// rather than device pixels keeps images from shrinking on hidpi screens.
const cssPerColumn = 8

// Prepared is a decoded image scaled for display in Cols×Rows cells.
type Prepared struct {
	Img        image.Image
	CSSW, CSSH int // intended display size in CSS pixels, for re-fitting on resize
	Cols, Rows int
}

// Fit returns the cells for a cssW×cssH CSS pixel image within
// maxCols×maxRows, keeping its aspect ratio on the given cell shape.
func Fit(cssW, cssH, maxCols, maxRows int, cell Cell) (cols, rows int) {
	cell = cell.orDefault()
	colsF := min(float64(cssW)/cssPerColumn, float64(maxCols))
	rowsF := colsF * float64(cssH) / float64(cssW) * float64(cell.W) / float64(cell.H)
	if rowsF > float64(maxRows) {
		colsF *= float64(maxRows) / rowsF
		rowsF = float64(maxRows)
	}
	ceil := func(f float64) int { return max(1, int(math.Ceil(f-1e-9))) }
	return ceil(colsF), ceil(rowsF)
}

// Prepare decodes data and scales it for display. cssW is the width the post
// declares for the image in CSS pixels; 0 means its natural width.
func Prepare(data []byte, cssW, maxCols, maxRows int, cell Cell) (Prepared, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Prepared{}, fmt.Errorf("reading image header: %w", err)
	}
	if cfg.Width <= 2 && cfg.Height <= 2 {
		return Prepared{}, ErrTooSmall
	}
	if cfg.Width*cfg.Height > maxPixels {
		return Prepared{}, fmt.Errorf("%w: %dx%d", ErrTooBig, cfg.Width, cfg.Height)
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Prepared{}, fmt.Errorf("decoding image: %w", err)
	}

	if cssW <= 0 {
		cssW = cfg.Width
	}
	cssH := max(1, int(math.Round(float64(cssW)*float64(cfg.Height)/float64(cfg.Width))))
	cols, rows := Fit(cssW, cssH, maxCols, maxRows, cell)

	// Only ever downscale: the terminal scales up the uploaded pixels itself.
	c := cell.orDefault()
	s := min(1, float64(cols*c.W)/float64(cfg.Width), float64(rows*c.H)/float64(cfg.Height))
	img := src
	if s < 1 {
		w := max(1, int(math.Round(float64(cfg.Width)*s)))
		h := max(1, int(math.Round(float64(cfg.Height)*s)))
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		img = dst
	}
	return Prepared{Img: img, CSSW: cssW, CSSH: cssH, Cols: cols, Rows: rows}, nil
}
