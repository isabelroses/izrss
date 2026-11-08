package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault_ReaderKeys(t *testing.T) {
	r := Default().Reader
	if r.Images != "auto" || r.ImageMaxHeight != 0 || r.CodeTheme != "catppuccin-mocha" {
		t.Errorf("defaults = %+v", r)
	}
	if r.Heading != "" || r.Link != "" || r.Code != "" || r.Quote != "" {
		t.Errorf("reader colours should default to empty (use the palette): %+v", r)
	}
}

func TestLoad_ReaderKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	toml := `[reader]
heading = "#ff0000"
link = "#00ff00"
code = "#0000ff"
quote = "#ffffff"
code_theme = "dracula"
images = "off"
image_max_height = 12
`
	if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.Reader
	if r.Heading != "#ff0000" || r.Link != "#00ff00" || r.Code != "#0000ff" || r.Quote != "#ffffff" ||
		r.CodeTheme != "dracula" || r.Images != "off" || r.ImageMaxHeight != 12 {
		t.Errorf("loaded = %+v", r)
	}
}

func TestWarnings_ThemeRemoved(t *testing.T) {
	cfg := Default()
	if w := cfg.Warnings(); len(w) != 0 {
		t.Errorf("no theme set should give no warnings, got %q", w)
	}
	cfg.Reader.Theme = "environment"
	want := "reader.theme is no longer supported; see example.toml for reader colours"
	if w := cfg.Warnings(); len(w) != 1 || w[0] != want {
		t.Errorf("got %q, want [%q]", w, want)
	}
}

func TestExampleConfigLoads(t *testing.T) {
	const example = "../../example.toml"
	if _, err := os.Stat(example); err != nil {
		t.Skip("example.toml is not part of the nix build source")
	}
	cfg, err := Load(example)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Warnings()) != 0 {
		t.Errorf("example.toml should not use removed keys: %q", cfg.Warnings())
	}
	if cfg.Reader.Images != "auto" || cfg.Reader.ImageMaxHeight != 16 {
		t.Errorf("example reader keys not loaded: %+v", cfg.Reader)
	}
}
