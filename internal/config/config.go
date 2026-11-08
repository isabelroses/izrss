// Package config handles application configuration
package config

import (
	"fmt"
	"os"
	"slices"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Home       string   `toml:"home"`
	DateFormat string   `toml:"dateformat"`
	Urls       []string `toml:"urls"`
	Reader     Reader   `toml:"reader"`
	Colors     Colors   `toml:"colors"`

	duplicates []string // urls listed more than once, dropped by Load
}

type Reader struct {
	Size          any     `toml:"size"`
	ReadThreshold float64 `toml:"read_threshold"`

	// Colours; empty means "use the [colors] palette".
	Heading   string `toml:"heading"`
	Link      string `toml:"link"`
	Code      string `toml:"code"`
	Quote     string `toml:"quote"`
	CodeTheme string `toml:"code_theme"` // chroma style name

	Images         string `toml:"images"`           // "auto", "on" or "off"
	ImageMaxHeight int    `toml:"image_max_height"` // 0: the reader's height

	// Theme is the obsolete glamour style, parsed only to warn about it.
	Theme string `toml:"theme"`
}

type Colors struct {
	Text       string `toml:"text"`
	Inverttext string `toml:"inverttext"`
	Subtext    string `toml:"subtext"`
	Accent     string `toml:"accent"`
	Borders    string `toml:"borders"`
}

func Default() *Config {
	return &Config{
		Home:       "home",
		DateFormat: "02/01/2006",
		Urls:       []string{},
		Reader: Reader{
			Size:          "recomended",
			ReadThreshold: 0.8,
			CodeTheme:     "catppuccin-mocha",
			Images:        "auto",
		},
		Colors: Colors{
			Text:       "#cdd6f4",
			Inverttext: "#1e1e2e",
			Subtext:    "#a6adc8",
			Accent:     "#74c7ec",
			Borders:    "#313244",
		},
	}
}

// Warnings describes settings that load but no longer do anything.
func (c *Config) Warnings() []string {
	var w []string
	if c.Reader.Theme != "" {
		w = append(w, "reader.theme is no longer supported; see example.toml for reader colours")
	}
	for _, u := range c.duplicates {
		w = append(w, "duplicate feed url "+u)
	}
	return w
}

// Load loads configuration from the specified path, or the default location if empty
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		var err error
		path, err = xdg.ConfigFile("izrss/config.toml")
		if err != nil {
			return nil, fmt.Errorf("getting config file path: %w", err)
		}
	}

	configRaw, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil
	}

	if err := toml.Unmarshal(configRaw, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	cfg.dedupeUrls()

	return cfg, nil
}

// dedupeUrls drops repeated urls, keeping the first of each in order, and
// remembers them for Warnings.
func (c *Config) dedupeUrls() {
	seen := make(map[string]bool, len(c.Urls))
	urls := c.Urls[:0]
	for _, u := range c.Urls {
		if seen[u] {
			if !slices.Contains(c.duplicates, u) {
				c.duplicates = append(c.duplicates, u)
			}
			continue
		}
		seen[u] = true
		urls = append(urls, u)
	}
	c.Urls = urls
}
