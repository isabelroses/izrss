package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/images"
)

// photoPNG is a noisy image, so PNG encoding costs what a real photo does.
func photoPNG(tb testing.TB, w, h int) []byte {
	tb.Helper()
	rng := rand.New(rand.NewPCG(1, 2))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x + rng.IntN(40)), uint8(y + rng.IntN(40)), uint8(rng.IntN(255)), 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		tb.Fatal(err)
	}
	return b.Bytes()
}

// loadedPhoto opens a post with one large photo and returns the model and
// the finished load, ready to be handed to Update.
func loadedPhoto(tb testing.TB) (Model, imageLoadedMsg) {
	tb.Helper()
	data := photoPNG(tb, 1600, 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	}))
	tb.Cleanup(srv.Close)
	db := testDB(tb)
	cfg := config.Default()
	cfg.Reader.Images = "on"
	cfg.Reader.Size = "full"
	m := *NewModel(cfg, db, nil, images.NewFetcher(tb.TempDir(), 16<<20, 5*time.Second), nil)
	m, _ = update(m, tea.WindowSizeMsg{Width: 160, Height: 50})
	m, cmd := openPost(m, postWithImage(srv.URL+"/photo.png"))
	loaded, ok := find[imageLoadedMsg](run(cmd))
	if !ok || loaded.err != nil {
		tb.Fatalf("load failed: %+v", loaded.err)
	}
	return m, loaded
}

// BenchmarkImageUpdate is the time Update spends on one image arriving and
// becoming ready: this blocks input and redraws.
func BenchmarkImageUpdate(b *testing.B) {
	m, loaded := loadedPhoto(b)
	b.ReportAllocs()
	for b.Loop() {
		updated, cmd := m.Update(loaded)
		_ = cmd
		mm := updated.(Model)
		mm.Update(imageReadyMsg{post: loaded.post, src: loaded.src})
	}
}
