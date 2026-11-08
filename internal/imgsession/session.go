// Package imgsession is the image state of the post in the reader: which
// images it has, their kitty ids, whether each is loaded, uploaded or failed,
// and how they are fitted to the page. It takes events (post opened, size
// changed, image loaded, frame drawn) and answers with terminal escape
// strings and loads to run. It knows nothing of Bubble Tea: the caller writes
// the escapes, runs the jobs, and sends the results back as events.
package imgsession

import (
	"log"
	"strings"

	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/reader"
)

// Loader fetches an image's bytes.
type Loader interface {
	Load(url string) ([]byte, error)
}

// Layout is the room images are fitted to: the most cells wide and high one
// may be, and the terminal's cell size.
type Layout struct {
	Cols, Rows int
	Cell       images.Cell
}

// Session is the images of the open post.
type Session struct {
	loader  Loader
	on      bool
	ids     *images.IDs
	live    bool // the first frame is out, so the alt screen is active
	post    string
	imgs    map[string]*entry // by resolved src
	toasted bool              // the post's failures were reported
}

type entry struct {
	prep     images.Prepared // sizes only; pixels are dropped once encoded
	id       int
	upload   string // the kitty upload escape, kept for re-sending
	del      string // the delete of the id this one displaced, sent with the upload
	loaded   bool
	uploaded bool
	failed   bool

	rows             []string // placeholder rows, cached for rowCols×rowRows
	rowCols, rowRows int
}

// New is a session loading with loader. A session that is not enabled does
// nothing and shows no images.
func New(loader Loader, enabled bool) *Session {
	return &Session{loader: loader, on: enabled && loader != nil, ids: images.NewIDs(), imgs: map[string]*entry{}}
}

// Enabled reports whether images are drawn at all.
func (s *Session) Enabled() bool { return s.on }

// Job is one image load to run off the main loop, then report with Loaded.
type Job struct {
	Post, Src string
	Run       func() Result
}

// Result is a finished load, already decoded, scaled and encoded by its job.
type Result struct {
	Post, Src   string
	ID, Evicted int
	Prep        images.Prepared
	Upload      string
	Err         error
}

// Open makes post the open one, starting afresh if it is a different post.
// Images already uploaded are re-fitted to l, which may have changed since
// the post was last open; the returned escapes re-place them.
func (s *Session) Open(post string, l Layout) string {
	if !s.on {
		return ""
	}
	if post != s.post {
		s.post, s.imgs, s.toasted = post, map[string]*entry{}, false
	}
	return s.Resized(l)
}

// Resized re-fits the uploaded images to l, returning escapes that re-place
// them without re-sending pixels.
func (s *Session) Resized(l Layout) string {
	var b strings.Builder
	for _, e := range s.imgs {
		if !e.uploaded {
			continue
		}
		cols, rows := images.Fit(e.prep.CSSW, e.prep.CSSH, l.Cols, l.Rows, l.Cell)
		if cols == e.prep.Cols && rows == e.prep.Rows {
			continue
		}
		e.prep.Cols, e.prep.Rows = cols, rows
		b.WriteString(images.Place(e.id, cols, rows))
	}
	return b.String()
}

// Fetch starts loading those of imgs the open post has not begun on. Ids are
// assigned here; the slow work (fetch, decode, scale, encode) is in the jobs.
func (s *Session) Fetch(imgs []reader.Image, l Layout) []Job {
	if !s.on {
		return nil
	}
	var jobs []Job
	post, loader := s.post, s.loader
	for _, img := range imgs {
		if _, known := s.imgs[img.Src]; known {
			continue
		}
		id, evicted := s.ids.Acquire(img.Src)
		s.imgs[img.Src] = &entry{id: id}
		src, cssW := img.Src, img.Width
		fetchURL := pickSource(img, l.Cols*cellWidth(l.Cell))
		jobs = append(jobs, Job{Post: post, Src: src, Run: func() Result {
			res := Result{Post: post, Src: src, ID: id, Evicted: evicted}
			data, err := loader.Load(fetchURL)
			if err != nil {
				res.Err = err
				return res
			}
			if res.Prep, err = images.Prepare(data, cssW, l.Cols, l.Rows, l.Cell); err != nil {
				res.Err = err
				return res
			}
			if res.Upload, err = images.Upload(id, res.Prep); err != nil {
				res.Err = err
				return res
			}
			res.Prep.Img = nil // only the encoded upload is needed from here on
			return res
		}})
	}
	return jobs
}

// Outcome is what an event asks of the caller, in this order.
type Outcome struct {
	Post, Src string
	// Write is terminal escapes to write now.
	Write string
	// Confirm: once Write is out, report Uploaded(Post, Src).
	Confirm bool
	// Rerender: the post must be rendered again (an image failed).
	Rerender bool
	// Failed is the number of images that failed, set once, when the last
	// image of the post has finished loading: for one toast per post.
	Failed int
}

// Loaded takes a finished load. A result for a post no longer open is
// ignored. An image that loaded before the first frame waits for FrameDrawn.
func (s *Session) Loaded(r Result) Outcome {
	e, ok := s.imgs[r.Src]
	if !s.on || r.Post != s.post || !ok {
		return Outcome{}
	}
	e.loaded = true
	var del string
	if r.Evicted != 0 {
		del = images.Delete(r.Evicted)
	}
	out := Outcome{Post: r.Post, Src: r.Src}
	if r.Err != nil {
		log.Printf("loading image %s: %v", r.Src, r.Err)
		e.failed = true
		out.Write, out.Rerender = del, true
	} else {
		e.prep, e.id, e.upload, e.del = r.Prep, r.ID, r.Upload, del
		if s.live {
			out.Write, out.Confirm = del+r.Upload, true
		}
	}
	out.Failed = s.failureReport()
	return out
}

// FrameDrawn says the first frame is out and the alt screen is active, where
// uploads must happen; it returns the uploads of images that were waiting.
func (s *Session) FrameDrawn() []Outcome {
	if s.live || !s.on {
		s.live = true
		return nil
	}
	s.live = true
	var out []Outcome
	for src, e := range s.imgs {
		if e.loaded && !e.failed && !e.uploaded {
			out = append(out, Outcome{Post: s.post, Src: src, Write: e.del + e.upload, Confirm: true})
		}
	}
	return out
}

// Uploaded says the upload for src was written; it reports whether the post
// must be rendered again to show it.
func (s *Session) Uploaded(post, src string) (rerender bool) {
	e, ok := s.imgs[src]
	if post != s.post || !ok {
		return false
	}
	e.uploaded = true
	return true
}

// Resume re-sends the uploaded images, as after a suspend left and
// re-entered the alt screen, which drops the terminal's image store.
func (s *Session) Resume() string {
	var b strings.Builder
	for _, e := range s.imgs {
		if !e.uploaded {
			continue
		}
		b.WriteString(e.upload)
		b.WriteString(images.Place(e.id, e.prep.Cols, e.prep.Rows)) // the size may have changed since
	}
	return b.String()
}

// failureReport returns the number of failed images once, when every image of
// the post has finished loading and some failed.
func (s *Session) failureReport() int {
	if s.toasted {
		return 0
	}
	failed := 0
	for _, e := range s.imgs {
		switch {
		case e.failed:
			failed++
		case !e.loaded:
			return 0 // still loading
		}
	}
	if failed > 0 {
		s.toasted = true
	}
	return failed
}

// Display is what the reader draws: placeholder rows for each uploaded image
// and the images that failed, by resolved src.
func (s *Session) Display() (ready map[string][]string, failed map[string]bool) {
	ready, failed = map[string][]string{}, map[string]bool{}
	for src, e := range s.imgs {
		switch {
		case e.failed:
			failed[src] = true
		case e.uploaded:
			ready[src] = e.placeholders()
		}
	}
	return ready, failed
}

// Info is what is known of one image.
type Info struct {
	ID         int
	Cols, Rows int
	Uploaded   bool
	Failed     bool
}

// Info reports on the image with resolved src.
func (s *Session) Info(src string) (Info, bool) {
	e, ok := s.imgs[src]
	if !ok {
		return Info{}, false
	}
	return Info{ID: e.id, Cols: e.prep.Cols, Rows: e.prep.Rows, Uploaded: e.uploaded, Failed: e.failed}, true
}

// placeholders returns the image's placeholder rows, rebuilt only on resize.
func (e *entry) placeholders() []string {
	if e.rows == nil || e.rowCols != e.prep.Cols || e.rowRows != e.prep.Rows {
		e.rows = images.Placeholders(e.id, e.prep.Cols, e.prep.Rows)
		e.rowCols, e.rowRows = e.prep.Cols, e.prep.Rows
	}
	return e.rows
}

// pickSource chooses the smallest srcset candidate at least wantPx wide, so
// images stay sharp when the terminal scales them up, else the largest.
func pickSource(img reader.Image, wantPx int) string {
	best := reader.Candidate{}
	for _, c := range img.Srcset {
		switch {
		case best.URL == "":
			best = c
		case best.W < wantPx && c.W > best.W:
			best = c // still too small: take anything bigger
		case c.W >= wantPx && c.W < best.W:
			best = c // wide enough and smaller
		}
	}
	if best.URL == "" {
		return img.Src
	}
	return best.URL
}

// cellWidth is the cell width in device pixels, assuming 8 when unknown.
func cellWidth(c images.Cell) int {
	if c.W > 0 {
		return c.W
	}
	return 8
}
