package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/reader"
	"github.com/isabelroses/izrss/internal/rss"
)

const placeholder = "\U0010EEEE"

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

type imgServer struct {
	*httptest.Server
	hits atomic.Int32
}

func newImgServer(t *testing.T) *imgServer {
	t.Helper()
	data := pngBytes(t, 400, 200) // 50x13 cells at the default 8x16 cell
	s := &imgServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	}))
	t.Cleanup(s.Close)
	return s
}

// imageModel is a ready model with images forced on.
func imageModel(t *testing.T) Model {
	t.Helper()
	db := testDB(t)
	cfg := config.Default()
	cfg.Reader.Images = "on"
	cfg.Reader.Size = "full"
	m := *NewModel(cfg, db, nil, images.NewFetcher(t.TempDir(), 1<<20, 5*time.Second), nil)
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func openPost(m Model, p rss.Post) (Model, tea.Cmd) {
	m.feeds = rss.Feeds{{Title: "feed", URL: "https://example.com/feed", Posts: []rss.Post{p}}}
	m.rebuildFeeds()
	cmd := m.openReader(rss.Loc{Feed: 0, Post: 0})
	return m, cmd
}

// run executes cmd and returns every message it produces, expanding batches
// and sequences.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		var out []tea.Msg
		for i := range v.Len() {
			out = append(out, run(v.Index(i).Interface().(tea.Cmd))...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func find[T any](msgs []tea.Msg) (T, bool) {
	for _, m := range msgs {
		if v, ok := m.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func raws(msgs []tea.Msg) string {
	var b strings.Builder
	for _, m := range msgs {
		if r, ok := m.(tea.RawMsg); ok {
			b.WriteString(fmt.Sprint(r.Msg))
		}
	}
	return b.String()
}

func update(m Model, msg tea.Msg) (Model, []tea.Msg) {
	updated, cmd := m.Update(msg)
	return updated.(Model), run(cmd)
}

func postWithImage(src string) rss.Post {
	return rss.Post{UUID: "p1", Title: "t", Link: "https://example.com/p1", Content: `<p>intro</p><img src="` + src + `" alt="a picture"><p>outro</p>`}
}

// loadAndShow opens post, delivers its image, and returns the model after the
// image is on screen plus the upload escapes that were sent.
func loadAndShow(t *testing.T, m Model, p rss.Post) (Model, string) {
	t.Helper()
	m, cmd := openPost(m, p)
	loaded, ok := find[imageLoadedMsg](run(cmd))
	if !ok {
		t.Fatal("opening the post should start loading its image")
	}
	m, msgs := update(m, loaded)
	ready, ok := find[imageReadyMsg](msgs)
	if !ok {
		t.Fatalf("loaded image should become ready, got %v", msgs)
	}
	m, _ = update(m, ready)
	return m, raws(msgs)
}

func TestReader_ImagesOff_NoCmds(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m.imgOn = false
	m, cmd := openPost(m, postWithImage(srv.URL+"/a.png"))
	if _, ok := find[imageLoadedMsg](run(cmd)); ok {
		t.Error("images off should not load images")
	}
	if !strings.Contains(ansi.Strip(m.viewport.GetContent()), "a picture") {
		t.Error("alt text should show")
	}
	if srv.hits.Load() != 0 {
		t.Error("no request should be made")
	}
}

func TestReader_PendingThenReady(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, cmd := openPost(m, postWithImage(srv.URL+"/a.png"))
	if !strings.Contains(ansi.Strip(m.viewport.GetContent()), "[image: a picture]") {
		t.Fatalf("pending image should show a placeholder line:\n%s", ansi.Strip(m.viewport.GetContent()))
	}
	loaded, ok := find[imageLoadedMsg](run(cmd))
	if !ok || loaded.err != nil {
		t.Fatalf("expected imageLoadedMsg, got %+v", loaded)
	}

	m, msgs := update(m, loaded)
	if len(msgs) < 2 {
		t.Fatalf("expected upload then ready, got %v", msgs)
	}
	rawIdx, readyIdx := -1, -1
	for i, msg := range msgs {
		switch v := msg.(type) {
		case tea.RawMsg:
			if strings.Contains(fmt.Sprint(v.Msg), "\x1b_G") && rawIdx < 0 {
				rawIdx = i
			}
		case imageReadyMsg:
			readyIdx = i
		}
	}
	if rawIdx < 0 || readyIdx < 0 || rawIdx > readyIdx {
		t.Fatalf("upload must precede ready: raw %d ready %d", rawIdx, readyIdx)
	}

	m, _ = update(m, msgs[readyIdx])
	if !strings.Contains(m.viewport.GetContent(), placeholder) {
		t.Error("ready image should be drawn with placeholders")
	}
}

func TestImageLoaded_NotReadyNoUpload(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, cmd := openPost(m, postWithImage(srv.URL+"/a.png"))
	loaded, _ := find[imageLoadedMsg](run(cmd))
	m.ready = false
	if _, msgs := update(m, loaded); strings.Contains(raws(msgs), "\x1b_G") {
		t.Error("no upload may be sent before the first frame")
	}
}

func TestImageLoaded_StalePostIgnored(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, cmdA := openPost(m, postWithImage(srv.URL+"/a.png"))
	loadedA, _ := find[imageLoadedMsg](run(cmdA))

	b := rss.Post{UUID: "p2", Title: "b", Content: "<p>post b</p>"}
	m, _ = openPost(m, b)
	before := m.viewport.GetContent()

	m, msgs := update(m, loadedA)
	if strings.Contains(raws(msgs), "\x1b_G") {
		t.Error("a stale image must not be uploaded")
	}
	if _, ok := find[imageReadyMsg](msgs); ok {
		t.Error("a stale image must not become ready")
	}
	if m.viewport.GetContent() != before {
		t.Error("post B's content changed")
	}
}

func TestImageLoaded_ErrorShowsAlt(t *testing.T) {
	m := imageModel(t)
	m, _ = openPost(m, postWithImage("https://example.invalid/a.png"))
	m, msgs := update(m, imageLoadedMsg{post: m.imgPost, src: "https://example.invalid/a.png", err: fmt.Errorf("boom")})
	if raws(msgs) != "" {
		t.Error("failed image should send nothing")
	}
	content := ansi.Strip(m.viewport.GetContent())
	if strings.Contains(content, "[image:") || !strings.Contains(content, "a picture") {
		t.Errorf("failed image should fall back to alt text:\n%s", content)
	}
}

func TestImageReady_KeepsReadingPosition(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	var b strings.Builder
	b.WriteString(`<p>top</p><img src="` + srv.URL + `/a.png" alt="pic">`)
	for i := range 200 {
		fmt.Fprintf(&b, "<p>line %d</p>", i)
	}
	p := rss.Post{UUID: "long", Content: b.String()}
	m, cmd := openPost(m, p)
	m.viewport.SetYOffset(100)
	topBefore := strings.SplitN(ansi.Strip(m.viewport.View()), "\n", 2)[0]

	loaded, _ := find[imageLoadedMsg](run(cmd))
	m, msgs := update(m, loaded)
	ready, _ := find[imageReadyMsg](msgs)
	m, _ = update(m, ready)

	rows := loaded.prep.Rows
	if got := m.viewport.YOffset(); got != 100+rows-1 {
		t.Errorf("YOffset = %d, want %d", got, 100+rows-1)
	}
	if topAfter := strings.SplitN(ansi.Strip(m.viewport.View()), "\n", 2)[0]; topAfter != topBefore {
		t.Errorf("top line moved: %q -> %q", topBefore, topAfter)
	}
}

func TestResize_RePlacesWithoutRefetch(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, _ = loadAndShow(t, m, postWithImage(srv.URL+"/a.png"))
	hits := srv.hits.Load()

	m, msgs := update(m, tea.WindowSizeMsg{Width: 40, Height: 40})
	r := raws(msgs)
	id := m.imgs[srv.URL+"/a.png"].id
	if !strings.Contains(r, "a=p") || !strings.Contains(r, "i="+strconv.Itoa(id)) {
		t.Errorf("resize should re-place image %d, got %q", id, r)
	}
	if srv.hits.Load() != hits {
		t.Error("resize must not re-fetch")
	}
	for _, line := range strings.Split(m.viewport.GetContent(), "\n") {
		if w := ansi.StringWidth(line); w > 40 {
			t.Errorf("line wider than the new width: %d", w)
		}
	}
}

// A resize while the reader is closed must still re-fit the images when the
// same post is opened again.
func TestResizeOnMain_ReopenRefits(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, _ = loadAndShow(t, m, postWithImage(srv.URL+"/a.png"))
	pi := m.imgs[srv.URL+"/a.png"]
	oldCols := pi.prep.Cols

	m, _ = keys(m, "esc")
	if m.screen != screenMain {
		t.Fatal("esc should leave the reader")
	}
	m, _ = update(m, tea.WindowSizeMsg{Width: 40, Height: 40})
	cmd := m.openReader(rss.Loc{Feed: 0, Post: 0})
	r := raws(run(cmd))
	if pi.prep.Cols >= oldCols {
		t.Errorf("cols %d not re-fitted from %d", pi.prep.Cols, oldCols)
	}
	if !strings.Contains(r, "a=p") || !strings.Contains(r, "i="+strconv.Itoa(pi.id)) {
		t.Errorf("reopen should re-place image %d, got %q", pi.id, r)
	}
	for _, line := range strings.Split(m.viewport.GetContent(), "\n") {
		if w := ansi.StringWidth(line); w > 40 {
			t.Errorf("line wider than the new width: %d", w)
		}
	}
}

func TestResume_ReUploads(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, _ = loadAndShow(t, m, postWithImage(srv.URL+"/a.png"))
	id := m.imgs[srv.URL+"/a.png"].id

	_, msgs := update(m, tea.ResumeMsg{})
	if r := raws(msgs); !strings.Contains(r, "a=T") || !strings.Contains(r, "i="+strconv.Itoa(id)) {
		t.Errorf("resume should re-upload image %d, got %q", id, r[:min(len(r), 80)])
	}
}

func TestView_PlaceholderKeepsIDColour(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, _ = loadAndShow(t, m, postWithImage(srv.URL+"/a.png"))
	id := m.imgs[srv.URL+"/a.png"].id
	sgr := "38;5;" + strconv.Itoa(id)

	found := false
	for _, line := range strings.Split(m.View().Content, "\n") {
		i := strings.Index(line, placeholder)
		if i < 0 {
			continue
		}
		found = true
		if j := strings.LastIndex(line[:i], "\x1b["); j < 0 || !strings.Contains(line[j:i], sgr) {
			t.Errorf("placeholder not preceded by its id colour %s: %q", sgr, line[:i])
		}
	}
	if !found {
		t.Error("no placeholder rows in the view")
	}
}

func TestIDEviction_DeletesOld(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	for i := range 255 {
		m.ids.Acquire("filler" + strconv.Itoa(i))
	}
	_, upload := loadAndShow(t, m, postWithImage(srv.URL+"/a.png"))
	del := strings.Index(upload, "a=d")
	put := strings.Index(upload, "a=T")
	if del < 0 || put < 0 || del > put {
		t.Errorf("evicted image must be deleted before the upload: %q", upload[:min(len(upload), 120)])
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

func TestImageMaxRows_DefaultsToReaderHeight(t *testing.T) {
	m := imageModel(t)
	m.cfg.Reader.ImageMaxHeight = 0
	m, _ = openPost(m, postWithImage("https://example.com/a.png"))
	// 40 rows less the header, hint line and the blank row under the header.
	if got, want := m.imageMaxRows(), 37; got != want {
		t.Errorf("imageMaxRows = %d, want the visible reader height %d", got, want)
	}
	m.cfg.Reader.ImageMaxHeight = 7
	if got := m.imageMaxRows(); got != 7 {
		t.Errorf("configured height ignored: %d", got)
	}
}

func TestLoadImages_FetchesSrcsetAndUsesDeclaredWidth(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	p := rss.Post{UUID: "s", Content: `<img src="` + srv.URL + `/small.png" width="320" srcset="` + srv.URL + `/small.png 100w, ` + srv.URL + `/big.png 4000w">`}
	var paths []string
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes(t, 400, 200))
	})
	_, cmd := openPost(m, p)
	loaded, _ := find[imageLoadedMsg](run(cmd))
	if len(paths) != 1 || paths[0] != "/big.png" {
		t.Errorf("fetched %v, want the srcset candidate wide enough for the reader", paths)
	}
	if loaded.prep.CSSW != 320 || loaded.prep.Cols != 40 {
		t.Errorf("declared width 320 css px should give 40 cols, got css %d cols %d", loaded.prep.CSSW, loaded.prep.Cols)
	}
	if loaded.src != srv.URL+"/small.png" {
		t.Errorf("the image keeps its src identity, got %q", loaded.src)
	}
}

// Encoding an image (PNG + base64) takes tens of milliseconds for a photo, so
// it must happen in the load command; Update only forwards the result.
func TestImageLoaded_NoEncodingInUpdate(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, cmd := openPost(m, postWithImage(srv.URL+"/a.png"))
	loaded, ok := find[imageLoadedMsg](run(cmd))
	if !ok || loaded.err != nil {
		t.Fatalf("expected imageLoadedMsg, got %+v", loaded)
	}
	if !strings.Contains(loaded.upload, "a=T") {
		t.Fatalf("the load command should encode the upload, got %q", loaded.upload[:min(len(loaded.upload), 40)])
	}
	if loaded.prep.Img != nil {
		t.Error("decoded pixels should be dropped once encoded")
	}

	loaded.upload = "\x1b_Gsentinel\x1b\\"
	_, msgs := update(m, loaded)
	if r := raws(msgs); r != loaded.upload {
		t.Errorf("Update should send the pre-encoded upload as is, got %q", r[:min(len(r), 80)])
	}
}

// Update must stay responsive while a large photo arrives.
func TestImageUpdate_Fast(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("timing test")
	}
	m, loaded := loadedPhoto(t)
	start := time.Now()
	updated, _ := m.Update(loaded)
	updated.(Model).Update(imageReadyMsg{post: loaded.post, src: loaded.src})
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Errorf("Update took %v for one image; heavy work belongs in commands", d)
	}
}
