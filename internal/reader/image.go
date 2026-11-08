package reader

import (
	"net/url"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// image renders an <img> as its own block. ok is false for images that should
// leave no trace at all (tracking pixels).
func (r *renderer) image(n *html.Node) (block, bool) {
	if isTrackingPixel(n) {
		return block{}, false
	}
	// Alt text is drawn on one line; collapse any line breaks and tabs in it.
	alt := strings.Join(strings.Fields(attr(n, "alt")), " ")
	if alt == "" {
		alt = "image"
	}
	src := resolve(r.o.BaseURL, attr(n, "src"))
	if !isHTTP(src) {
		return block{lines: []string{r.subtle(ansi.Truncate(alt, r.width, "…"))}}, true
	}

	width, _ := strconv.Atoi(attr(n, "width"))
	img := Image{Src: src, Width: width, Srcset: r.srcset(attr(n, "srcset"))}
	var lines []string
	rows, ready := r.o.Ready[src]
	switch {
	case r.o.Failed[src] || !r.o.ShowImages:
		st := lipgloss.NewStyle().Foreground(r.o.Theme.Link).Underline(true).Hyperlink(src)
		lines = []string{st.Render(ansi.Truncate(alt, r.width, "…"))}
	case ready:
		// Inside a list or quote rows may be wider than the container, so crop.
		// Rows are equally wide and slow to measure, so check only the first.
		if len(rows) > 0 && ansi.StringWidth(rows[0]) <= r.width {
			lines = append(lines, rows...)
			break
		}
		for _, row := range rows {
			lines = append(lines, ansi.Truncate(row, r.width, ""))
		}
	default:
		lines = []string{r.subtle(ansi.Truncate("[image: "+alt+"]", r.width, "…"))}
	}
	img.Height = len(lines)
	return block{lines: lines, images: []Image{img}}, true
}

// srcset parses width-descriptor candidates ("a.png 640w, b.png 914w").
// Density descriptors ("2x") are skipped: they say nothing about pixel width.
func (r *renderer) srcset(s string) []Candidate {
	var out []Candidate
	for entry := range strings.SplitSeq(s, ",") {
		fields := strings.Fields(entry)
		if len(fields) != 2 || !strings.HasSuffix(fields[1], "w") {
			continue
		}
		w, err := strconv.Atoi(strings.TrimSuffix(fields[1], "w"))
		if err != nil || w <= 0 {
			continue
		}
		if u := resolve(r.o.BaseURL, fields[0]); isHTTP(u) {
			out = append(out, Candidate{URL: u, W: w})
		}
	}
	return out
}

// isTrackingPixel reports an image declared as at most 2×2 pixels.
func isTrackingPixel(n *html.Node) bool {
	w, errW := strconv.Atoi(attr(n, "width"))
	h, errH := strconv.Atoi(attr(n, "height"))
	return errW == nil && errH == nil && w <= 2 && h <= 2
}

func isHTTP(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
