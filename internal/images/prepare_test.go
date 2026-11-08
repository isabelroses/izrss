package images

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi/kitty"
)

func TestFit(t *testing.T) {
	tests := []struct {
		name               string
		cssW, cssH, mc, mr int
		cell               Cell
		wantCols, wantRows int
	}{
		// 8 CSS px per column; rows follow the image aspect and the cell shape.
		{"css width to columns", 320, 160, 80, 50, Cell{10, 20}, 40, 10},
		{"hidpi cell does not shrink it", 320, 160, 80, 50, Cell{20, 40}, 40, 10},
		{"capped at reader width", 1600, 800, 80, 50, Cell{10, 20}, 80, 20},
		{"height cap keeps aspect", 800, 4000, 80, 20, Cell{10, 20}, 8, 20},
		{"unknown cell assumes 8x16", 640, 640, 80, 100, Cell{}, 80, 40},
		{"tiny stays at least 1x1", 3, 3, 80, 20, Cell{10, 20}, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := Fit(tt.cssW, tt.cssH, tt.mc, tt.mr, tt.cell)
			if cols != tt.wantCols || rows != tt.wantRows {
				t.Errorf("Fit = %dx%d, want %dx%d", cols, rows, tt.wantCols, tt.wantRows)
			}
		})
	}
}

func TestPrepare_UsesDeclaredCSSWidth(t *testing.T) {
	// A 2x asset (800px) declared as 400 CSS px wide on a hidpi terminal.
	p, err := Prepare(encoded(t, "png", testImage(800, 400)), 400, 80, 100, Cell{16, 32})
	if err != nil {
		t.Fatal(err)
	}
	if p.Cols != 50 || p.Rows != 13 {
		t.Errorf("cells = %dx%d, want 50x13", p.Cols, p.Rows)
	}
	if p.CSSW != 400 || p.CSSH != 200 {
		t.Errorf("css size = %dx%d, want 400x200", p.CSSW, p.CSSH)
	}
	if b := p.Img.Bounds(); b.Dx() != 800 {
		t.Errorf("pixels needed (50 cols x 16px = 800) so no downscale, got %v", b)
	}
}

func testImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 50, 255})
		}
	}
	return img
}

func encoded(t *testing.T, format string, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&b, img)
	case "jpeg":
		err = jpeg.Encode(&b, img, nil)
	case "gif":
		pal := image.NewPaletted(img.Bounds(), []color.Color{color.Black, color.White})
		err = gif.EncodeAll(&b, &gif.GIF{Image: []*image.Paletted{pal, pal}, Delay: []int{10, 10}})
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPrepare_DecodesFormats(t *testing.T) {
	webp, err := os.ReadFile("testdata/small.webp")
	if err != nil {
		t.Fatal(err)
	}
	inputs := map[string][]byte{
		"png":  encoded(t, "png", testImage(40, 20)),
		"jpeg": encoded(t, "jpeg", testImage(40, 20)),
		"gif":  encoded(t, "gif", testImage(40, 20)),
		"webp": webp,
	}
	cell := Cell{4, 8}
	for name, data := range inputs {
		t.Run(name, func(t *testing.T) {
			p, err := Prepare(data, 0, 80, 20, cell)
			if err != nil {
				t.Fatal(err)
			}
			if p.CSSW != 40 || p.CSSH != 20 || p.Cols != 5 || p.Rows != 2 {
				t.Errorf("prepared = css %dx%d cells %dx%d", p.CSSW, p.CSSH, p.Cols, p.Rows)
			}
			b := p.Img.Bounds()
			if b.Dx() > p.Cols*cell.W || b.Dy() > p.Rows*cell.H {
				t.Errorf("scaled image %v larger than its %dx%d cells", b, p.Cols, p.Rows)
			}
		})
	}
}

func TestPrepare_DownscalesLargeImages(t *testing.T) {
	p, err := Prepare(encoded(t, "png", testImage(800, 400)), 0, 40, 20, Cell{10, 20})
	if err != nil {
		t.Fatal(err)
	}
	if b := p.Img.Bounds(); b.Dx() != 400 || b.Dy() != 200 || p.Cols != 40 || p.Rows != 10 {
		t.Errorf("expected a 400x200 downscale for 40x10 cells, got %v (%dx%d cells)", b, p.Cols, p.Rows)
	}
}

func TestPrepare_TrackingPixel(t *testing.T) {
	if _, err := Prepare(encoded(t, "png", testImage(2, 2)), 0, 80, 20, Cell{10, 20}); !errors.Is(err, ErrTooSmall) {
		t.Errorf("err = %v, want ErrTooSmall", err)
	}
}

// pngHeader returns a valid PNG signature + IHDR claiming w×h, with no pixels.
func pngHeader(w, h uint32) []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // 8-bit RGBA
	_ = binary.Write(&b, binary.BigEndian, uint32(len(ihdr)))
	chunk := append([]byte("IHDR"), ihdr...)
	b.Write(chunk)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

func TestPrepare_RejectsHugeDimensions(t *testing.T) {
	if _, err := Prepare(pngHeader(50000, 50000), 0, 80, 20, Cell{10, 20}); !errors.Is(err, ErrTooBig) {
		t.Errorf("err = %v, want ErrTooBig", err)
	}
}

func TestPrepare_NotAnImage(t *testing.T) {
	for name, data := range map[string]string{
		"html": "<html><body>nope</body></html>",
		"svg":  `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`,
	} {
		if _, err := Prepare([]byte(data), 0, 80, 20, Cell{10, 20}); err == nil {
			t.Errorf("%s should fail to decode", name)
		}
	}
}

func TestPlaceholders(t *testing.T) {
	rows := Placeholders(42, 30, 10)
	if len(rows) != 10 {
		t.Fatalf("got %d rows", len(rows))
	}
	for r, row := range rows {
		if w := lipgloss.Width(row); w != 30 {
			t.Errorf("row %d width = %d, want 30", r, w)
		}
		if !strings.Contains(row, "38;5;42") {
			t.Errorf("row %d lacks the id colour: %q", r, row)
		}
		if !strings.ContainsRune(row, kitty.Diacritic(r)) {
			t.Errorf("row %d lacks its row diacritic", r)
		}
	}
}

func TestUpload(t *testing.T) {
	p, err := Prepare(encoded(t, "png", testImage(40, 20)), 0, 80, 20, Cell{4, 8})
	if err != nil {
		t.Fatal(err)
	}
	s, err := Upload(7, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "\x1b_G") || !strings.HasSuffix(s, "\x1b\\") {
		t.Errorf("not a kitty APC sequence: %q…", s[:min(len(s), 40)])
	}
	for _, want := range []string{"i=7", "U=1", "a=T", "c=5", "r=2", "q=2"} {
		if !strings.Contains(s, want) {
			t.Errorf("upload lacks %s", want)
		}
	}
}

func TestPlaceAndDelete(t *testing.T) {
	place := Place(9, 12, 4)
	for _, want := range []string{"a=p", "U=1", "i=9", "c=12", "r=4", "q=2"} {
		if !strings.Contains(place, want) {
			t.Errorf("place lacks %s: %q", want, place)
		}
	}
	del := Delete(9)
	for _, want := range []string{"a=d", "d=I", "i=9"} {
		if !strings.Contains(del, want) {
			t.Errorf("delete lacks %s: %q", want, del)
		}
	}
}
