package config

import (
	"bytes"
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

func TestLoad_LogsRemovedTheme(t *testing.T) {
	logged := captureLog(t)
	if _, err := Load(writeConfig(t, "[reader]\ntheme = \"environment\"\n")); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(logged.Bytes(), []byte("reader.theme is no longer supported")) {
		t.Errorf("log = %q", logged)
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
	if cfg.Reader.Theme != "" {
		t.Errorf("example.toml should not use removed keys: theme %q", cfg.Reader.Theme)
	}
	if cfg.Reader.Images != "auto" || cfg.Reader.ImageMaxHeight != 16 {
		t.Errorf("example reader keys not loaded: %+v", cfg.Reader)
	}
}
