package imgsession

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"

	"github.com/isabelroses/izrss/internal/reader"
)

func pngBytes(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// fakeLoader serves one PNG for every url and records what was asked.
type fakeLoader struct {
	data []byte
	urls []string
	err  error
}

func (f *fakeLoader) Load(url string) ([]byte, error) {
	f.urls = append(f.urls, url)
	return f.data, f.err
}

var wide = Layout{Cols: 100, Rows: 30}
var narrow = Layout{Cols: 20, Rows: 30}

func newSession(t *testing.T) (*Session, *fakeLoader) {
	t.Helper()
	l := &fakeLoader{data: pngBytes(t, 400, 200)} // 50x13 cells at the default 8x16 cell
	return New(l, true), l
}

func imgs(srcs ...string) []reader.Image {
	out := make([]reader.Image, len(srcs))
	for i, s := range srcs {
		out[i] = reader.Image{Src: s}
	}
	return out
}

// shown runs a session through load, first frame and upload of one image.
func shown(t *testing.T, s *Session, post, src string, l Layout) {
	t.Helper()
	s.Open(post, l)
	jobs := s.Fetch(imgs(src), l)
	if len(jobs) != 1 {
		t.Fatalf("want 1 job, got %d", len(jobs))
	}
	s.FrameDrawn()
	o := s.Loaded(jobs[0].Run())
	if !o.Confirm || !strings.Contains(o.Write, "a=T") {
		t.Fatalf("outcome %+v", o)
	}
	if !s.Uploaded(o.Post, o.Src) {
		t.Fatal("Uploaded found nothing")
	}
}

func TestUploadWaitsForFirstFrame(t *testing.T) {
	s, _ := newSession(t)
	s.Open("p", wide)
	jobs := s.Fetch(imgs("a"), wide)
	o := s.Loaded(jobs[0].Run())
	if o.Write != "" || o.Confirm {
		t.Fatalf("no upload may be sent before the first frame, got %+v", o)
	}
	out := s.FrameDrawn()
	if len(out) != 1 || !strings.Contains(out[0].Write, "a=T") || !out[0].Confirm {
		t.Fatalf("the first frame should release the waiting upload, got %+v", out)
	}
	if more := s.FrameDrawn(); len(more) != 0 {
		t.Errorf("a second frame uploaded again: %+v", more)
	}
}

// The slow work is in the job, not in Loaded, which only forwards the result.
func TestEncodingHappensInTheJob(t *testing.T) {
	s, _ := newSession(t)
	s.Open("p", wide)
	res := s.Fetch(imgs("a"), wide)[0].Run()
	if !strings.Contains(res.Upload, "a=T") {
		t.Fatalf("the job should encode the upload, got %q", res.Upload[:min(len(res.Upload), 40)])
	}
	if res.Prep.Img != nil {
		t.Error("decoded pixels should be dropped once encoded")
	}
	s.FrameDrawn()
	res.Upload = "\x1b_Gsentinel\x1b\\"
	if o := s.Loaded(res); o.Write != res.Upload {
		t.Errorf("Loaded should send the pre-encoded upload as is, got %q", o.Write)
	}
}

func TestStaleResultIgnored(t *testing.T) {
	s, _ := newSession(t)
	s.Open("a", wide)
	res := s.Fetch(imgs("img"), wide)[0].Run()
	s.Open("b", wide)
	s.FrameDrawn()
	if o := s.Loaded(res); o != (Outcome{}) {
		t.Errorf("a result for a closed post must be ignored, got %+v", o)
	}
}

// A resize while the reader was closed must still re-fit when the post is
// opened again.
func TestReopenAfterResizeRefits(t *testing.T) {
	s, _ := newSession(t)
	shown(t, s, "p", "a", wide)
	before := s.imgs["a"].prep.Cols

	place := s.Open("p", narrow) // reopened at another width
	after := s.imgs["a"]
	if after.prep.Cols >= before {
		t.Errorf("cols %d not re-fitted from %d", after.prep.Cols, before)
	}
	if !strings.Contains(place, "a=p") || !strings.Contains(place, "i="+strconv.Itoa(after.id)) {
		t.Errorf("reopen should re-place image %d, got %q", after.id, place)
	}
	if again := s.Open("p", narrow); again != "" {
		t.Errorf("an unchanged fit should place nothing, got %q", again)
	}
}

func TestResizedReplacesWithoutRefetch(t *testing.T) {
	s, l := newSession(t)
	shown(t, s, "p", "a", wide)
	n := len(l.urls)
	if place := s.Resized(narrow); !strings.Contains(place, "a=p") {
		t.Errorf("resize should re-place, got %q", place)
	}
	if jobs := s.Fetch(imgs("a"), narrow); len(jobs) != 0 || len(l.urls) != n {
		t.Error("a known image must not be fetched again")
	}
}

func TestOneFailureReportPerPost(t *testing.T) {
	l := &fakeLoader{err: errors.New("404")}
	s := New(l, true)
	s.Open("p", wide)
	s.FrameDrawn()
	jobs := s.Fetch(imgs("a", "b", "c"), wide)
	var reports []int
	for _, j := range jobs {
		o := s.Loaded(j.Run())
		if !o.Rerender {
			t.Error("a failure should ask for a re-render")
		}
		if o.Failed > 0 {
			reports = append(reports, o.Failed)
		}
	}
	if len(reports) != 1 || reports[0] != 3 {
		t.Errorf("failure reports = %v, want one report of 3 (after the last finished)", reports)
	}
	_, failed := s.Display()
	if len(failed) != 3 {
		t.Errorf("failed = %v", failed)
	}
	// Reopening the same post reports nothing more.
	s.Open("p", wide)
	if len(s.Fetch(imgs("a", "b", "c"), wide)) != 0 {
		t.Error("failed images are not retried")
	}
}

// Each post numbers its images from 1, and leaving a post deletes its images
// from the terminal before the next post's ids are handed out again.
func TestSwitchingPostsDeletesAndRenumbers(t *testing.T) {
	s, _ := newSession(t)
	shown(t, s, "p", "a", wide)
	if id := s.imgs["a"].id; id != 1 {
		t.Fatalf("first image of a post has id %d, want 1", id)
	}
	if again := s.Open("p", wide); strings.Contains(again, "a=d") {
		t.Errorf("reopening the same post must keep its images: %q", again)
	}
	if del := s.Open("q", wide); !strings.Contains(del, "a=d") || !strings.Contains(del, "i=1") {
		t.Errorf("leaving a post should delete its image 1, got %q", del)
	}
	s.Fetch(imgs("b"), wide)
	if id := s.imgs["b"].id; id != 1 {
		t.Errorf("the next post's first image has id %d, want 1", id)
	}
}

func TestIDsStayWithinRange(t *testing.T) {
	s, _ := newSession(t)
	s.Open("p", wide)
	var all []reader.Image
	for i := range 300 {
		all = append(all, reader.Image{Src: "img" + strconv.Itoa(i)})
	}
	jobs := s.Fetch(all, wide)
	if len(jobs) != maxIDs {
		t.Errorf("%d jobs, want one per id (%d)", len(jobs), maxIDs)
	}
	seen := map[int]bool{}
	for src, e := range s.imgs {
		if e.failed {
			continue
		}
		if e.id < 1 || e.id > maxIDs || seen[e.id] {
			t.Fatalf("image %s got id %d, outside 1..255 or reused", src, e.id)
		}
		seen[e.id] = true
	}
}

func TestResumeReuploads(t *testing.T) {
	s, _ := newSession(t)
	shown(t, s, "p", "a", wide)
	id := s.imgs["a"].id
	if r := s.Resume(); !strings.Contains(r, "a=T") || !strings.Contains(r, "i="+strconv.Itoa(id)) {
		t.Errorf("resume should re-upload image %d, got %q", id, r[:min(len(r), 80)])
	}
}

func TestDisplayHasPlaceholdersOnceUploaded(t *testing.T) {
	s, _ := newSession(t)
	s.Open("p", wide)
	s.FrameDrawn()
	o := s.Loaded(s.Fetch(imgs("a"), wide)[0].Run())
	if ready, _ := s.Display(); len(ready) != 0 {
		t.Error("an image is not drawn before its upload is confirmed")
	}
	s.Uploaded(o.Post, o.Src)
	ready, _ := s.Display()
	if rows := s.imgs["a"].prep.Rows; len(ready["a"]) != rows {
		t.Errorf("want %d placeholder rows, got %d", rows, len(ready["a"]))
	}
}

func TestDisabledSessionDoesNothing(t *testing.T) {
	l := &fakeLoader{data: pngBytes(t, 40, 40)}
	s := New(l, false)
	if s.Enabled() || s.Open("p", wide) != "" || len(s.Fetch(imgs("a"), wide)) != 0 || s.Resume() != "" {
		t.Error("a disabled session should do nothing")
	}
	if len(l.urls) != 0 {
		t.Error("no request should be made")
	}
}

func TestPickSource(t *testing.T) {
	set := []reader.Candidate{{URL: "s", W: 640}, {URL: "l", W: 2880}, {URL: "m", W: 1080}}
	tests := []struct {
		name   string
		img    reader.Image
		wantPx int
		want   string
	}{
		{"no srcset uses src", reader.Image{Src: "src"}, 1000, "src"},
		{"smallest wide enough", reader.Image{Src: "src", Srcset: set}, 1000, "m"},
		{"exact match", reader.Image{Src: "src", Srcset: set}, 640, "s"},
		{"none wide enough takes largest", reader.Image{Src: "src", Srcset: set}, 5000, "l"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pickSource(tt.img, tt.wantPx); got != tt.want {
				t.Errorf("pickSource = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFetchUsesSrcsetAndDeclaredWidth(t *testing.T) {
	s, l := newSession(t)
	s.Open("p", wide)
	img := reader.Image{Src: "small", Width: 320, Srcset: []reader.Candidate{{URL: "small", W: 100}, {URL: "big", W: 4000}}}
	res := s.Fetch([]reader.Image{img}, wide)[0].Run()
	if len(l.urls) != 1 || l.urls[0] != "big" {
		t.Errorf("fetched %v, want the srcset candidate wide enough", l.urls)
	}
	if res.Prep.CSSW != 320 || res.Prep.Cols != 40 || res.Src != "small" {
		t.Errorf("declared width 320 css px should give 40 cols and keep the src: css %d cols %d src %q", res.Prep.CSSW, res.Prep.Cols, res.Src)
	}
}
