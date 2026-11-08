package ui

import (
	"testing"
)

func TestReaderWidth(t *testing.T) {
	tests := []struct {
		size  any
		width int
		want  int
	}{
		{"full", 120, 120},
		{"fullscreen", 120, 120},
		{"most", 120, 90},
		{"recomended", 120, 80},
		{int64(60), 120, 60},
		{int64(500), 120, 120},
		{int64(5), 120, 20},
		{"full", 10, 10},
		{nil, 120, 80},
	}
	for _, tt := range tests {
		if got := readerWidth(tt.size, tt.width); got != tt.want {
			t.Errorf("readerWidth(%v, %d) = %d, want %d", tt.size, tt.width, got, tt.want)
		}
	}
}
