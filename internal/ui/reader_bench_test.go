package ui

import (
	"os"
	"testing"

	"github.com/isabelroses/izrss/internal/config"
	"github.com/isabelroses/izrss/internal/reader"
	"github.com/isabelroses/izrss/internal/rss"
)

var readerSink string

func BenchmarkRenderPost(b *testing.B) {
	src, err := os.ReadFile("testdata/post.html")
	if err != nil {
		b.Fatal(err)
	}
	o := reader.Options{Width: 80, Theme: readerTheme(config.Default()), BaseURL: "https://example.com/posts/1"}
	b.ReportAllocs()
	for b.Loop() {
		readerSink = reader.Render(string(src), o).String()
	}
}

// BenchmarkReaderView draws the reader screen, as every scroll keypress does.
func BenchmarkReaderView(b *testing.B) {
	b.Run("text", func(b *testing.B) {
		src, err := os.ReadFile("testdata/post.html")
		if err != nil {
			b.Fatal(err)
		}
		m, _ := openPost(benchModel(b), rss.Post{UUID: "p", Title: "t", Link: "https://example.com/p", Content: string(src)})
		b.ReportAllocs()
		for b.Loop() {
			readerSink = m.View().Content
		}
	})
	b.Run("image", func(b *testing.B) {
		m, loaded := loadedPhoto(b)
		m, _ = update(m, loaded)
		m, _ = update(m, imageReadyMsg{post: loaded.post, src: loaded.src})
		b.ReportAllocs()
		for b.Loop() {
			readerSink = m.View().Content
		}
	})
}
