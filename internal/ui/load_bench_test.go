package ui

import "testing"

var benchSink string

func BenchmarkBoldUnread(b *testing.B) {
	// Short titles skip truncation; the long one exercises the truncation path.
	inputs := []struct {
		s     string
		width int
	}{
		{"News", 40},
		{"Hacker News: Front Page", 40},
		{"a very long feed title that does not fit within the column width", 20},
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		in := inputs[i%len(inputs)]
		benchSink = boldUnread(in.s, in.width)
		i++
	}
}
