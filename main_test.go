package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
)

func TestOpenLog_CreatesStateFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	xdg.Reload()
	t.Cleanup(xdg.Reload)

	path, f, err := openLog()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if want := filepath.Join(dir, "izrss", "izrss.log"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if _, err := f.WriteString("x\n"); err != nil {
		t.Fatal(err)
	}
	// Appends rather than truncating across runs.
	_, f2, err := openLog()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f2.WriteString("y\n")
	_ = f2.Close()
	if b, _ := os.ReadFile(path); string(b) != "x\ny\n" {
		t.Errorf("log content = %q", b)
	}
}
