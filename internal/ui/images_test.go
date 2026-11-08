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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/imgsession"
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
	m := *NewModel(cfg, db, nil, images.NewFetcher(t.TempDir(), 1<<20, 5*time.Second))
	m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func openPost(m Model, p rss.Post) (Model, tea.Cmd) {
	f := rss.Feed{Title: "feed", URL: "https://example.com/feed", Posts: []rss.Post{p}}
	m.feeds.Reset([]rss.Feed{f})
	m.rebuildFeeds()
	cmd := m.openReader(p.Ref(f.URL))
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
	if v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
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

func TestReader_PendingThenReady(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, cmd := openPost(m, postWithImage(srv.URL+"/a.png"))
	if !strings.Contains(ansi.Strip(m.viewport.GetContent()), "[image: a picture]") {
		t.Fatalf("pending image should show a placeholder line:\n%s", ansi.Strip(m.viewport.GetContent()))
	}
	loaded, ok := find[imageLoadedMsg](run(cmd))
	if !ok || loaded.Err != nil {
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

func TestImageLoaded_ErrorShowsAlt(t *testing.T) {
	m := imageModel(t)
	m, _ = openPost(m, postWithImage("https://example.invalid/a.png"))
	m, msgs := update(m, imageLoadedMsg{imgsession.Result{Post: sessionKey(m.open), Src: "https://example.invalid/a.png", Err: fmt.Errorf("boom")}})
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

	rows := loaded.Prep.Rows
	if got := m.viewport.YOffset(); got != 100+rows-1 {
		t.Errorf("YOffset = %d, want %d", got, 100+rows-1)
	}
	if topAfter := strings.SplitN(ansi.Strip(m.viewport.View()), "\n", 2)[0]; topAfter != topBefore {
		t.Errorf("top line moved: %q -> %q", topBefore, topAfter)
	}
}

func TestView_PlaceholderKeepsIDColour(t *testing.T) {
	srv := newImgServer(t)
	m := imageModel(t)
	m, _ = loadAndShow(t, m, postWithImage(srv.URL+"/a.png"))
	sgr := "38;5;1" // a post numbers its images from 1

	found := false
	for line := range strings.SplitSeq(m.View().Content, "\n") {
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

func TestImageMaxRows_DefaultsToReaderHeight(t *testing.T) {
	m := imageModel(t)
	m.cfg.Reader.ImageMaxHeight = 0
	m, _ = openPost(m, postWithImage("https://example.com/a.png"))
	// 40 rows less the header, hint line, the gap above it and the blank row under the header.
	if got, want := m.imageMaxRows(), 36; got != want {
		t.Errorf("imageMaxRows = %d, want the visible reader height %d", got, want)
	}
	m.cfg.Reader.ImageMaxHeight = 7
	if got := m.imageMaxRows(); got != 7 {
		t.Errorf("configured height ignored: %d", got)
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
	updated.(Model).Update(imageReadyMsg{post: loaded.Post, src: loaded.Src})
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Errorf("Update took %v for one image; heavy work belongs in commands", d)
	}
}
