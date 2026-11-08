package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/images"
	"github.com/isabelroses/izrss/internal/rss"
)

// BenchmarkReaderView draws one reader frame at 120x40: a long text post, and
// the same post with an image on screen.
func BenchmarkReaderView(b *testing.B) {
	text := strings.Repeat("<p>The quick brown fox jumps over the lazy dog, again and again and again.</p>", 80)
	for _, withImage := range []bool{false, true} {
		name := "text"
		if withImage {
			name = "image"
		}
		b.Run(name, func(b *testing.B) {
			data := pngBytes(b, 400, 200)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(data)
			}))
			b.Cleanup(srv.Close)
			cfg := config.Default()
			cfg.Reader.Images = "on"
			cfg.Reader.Size = "full"
			m := *NewModel(cfg, testDB(b), nil, images.NewFetcher(b.TempDir(), 1<<20, 5*time.Second))
			m, _ = update(m, tea.WindowSizeMsg{Width: 120, Height: 40})
			content := text
			if withImage {
				content = `<p>intro</p><img src="` + srv.URL + `/a.png" alt="pic">` + text
			}
			m, cmd := openPost(m, rss.Post{UUID: "p", Title: "t", Link: "https://example.com/p", Content: content})
			if withImage {
				loaded, ok := find[imageLoadedMsg](run(cmd))
				if !ok {
					b.Fatal("no image load")
				}
				var msgs []tea.Msg
				m, msgs = update(m, loaded)
				ready, ok := find[imageReadyMsg](msgs)
				if !ok {
					b.Fatal("image not ready")
				}
				m, _ = update(m, ready)
			}
			if got := strings.Contains(m.View().Content, placeholder); got != withImage {
				b.Fatalf("placeholder on screen = %v, want %v", got, withImage)
			}
			b.ReportAllocs()
			for b.Loop() {
				readerSink = m.View().Content
			}
		})
	}
}
