// Package config handles application configuration
package config

import (
	"fmt"
	"log"
	"os"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Home       string   `toml:"home"`
	DateFormat string   `toml:"dateformat"`
	Urls       []string `toml:"urls"`
	Reader     Reader   `toml:"reader"`
	Colors     Colors   `toml:"colors"`
}

type Reader struct {
	Size          any     `toml:"size"` // as written; Load resolves it into Width
	Width         Width   `toml:"-"`
	ReadThreshold float64 `toml:"read_threshold"` // 0 to 1

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
			Size:          "recommended",
			Width:         Width{Cols: 80},
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
	cfg.Reader.resolve()

	return cfg, nil
}

// dedupeUrls drops repeated urls, keeping the first of each in order, and
// logs them.
func (c *Config) dedupeUrls() {
	seen := make(map[string]bool, len(c.Urls))
	urls := c.Urls[:0]
	for _, u := range c.Urls {
		if seen[u] {
			log.Print("duplicate feed url ", u)
			continue
		}
		seen[u] = true
		urls = append(urls, u)
	}
	c.Urls = urls
}

// Width is how wide the reader wraps text: Cols columns, or, when Percent is
// set, that share of the room there is.
type Width struct{ Cols, Percent int }

// Of is the column the reader wraps at with room columns to fill: never
// under 20 columns, never over room.
func (w Width) Of(room int) int {
	cols := w.Cols
	if w.Percent > 0 {
		cols = room * w.Percent / 100
	}
	return min(max(cols, 20), room)
}

// widthOf reads a reader.size value; ok is false for one it doesn't know.
func widthOf(size any) (w Width, ok bool) {
	switch s := size.(type) {
	case string:
		switch s {
		case "recommended", "recomended": // the old default's spelling
			return Width{Cols: 80}, true
		case "most":
			return Width{Percent: 75}, true
		case "full", "fullscreen":
			return Width{Percent: 100}, true
		}
	case int64:
		return Width{Cols: int(s)}, true
	}
	return Width{Cols: 80}, false
}

// resolve fills in Width and replaces values Load can't use with their
// defaults, logging each.
func (r *Reader) resolve() {
	def := Default().Reader
	var ok bool
	if r.Width, ok = widthOf(r.Size); !ok {
		log.Printf(`reader.size %v is not "recommended", "most", "full" or a number of columns; using 80`, r.Size)
	}
	switch r.Images {
	case "auto", "on", "off":
	default:
		log.Printf(`reader.images %q is not "auto", "on" or "off"; using "auto"`, r.Images)
		r.Images = def.Images
	}
	if r.ReadThreshold < 0 || r.ReadThreshold > 1 {
		log.Printf("reader.read_threshold %v is not between 0 and 1; using %v", r.ReadThreshold, def.ReadThreshold)
		r.ReadThreshold = def.ReadThreshold
	}
	if r.ImageMaxHeight < 0 {
		log.Printf("reader.image_max_height %d is negative; using the reader's height", r.ImageMaxHeight)
		r.ImageMaxHeight = def.ImageMaxHeight
	}
	if r.Theme != "" {
		log.Print("reader.theme is no longer supported; see example.toml for reader colours")
	}
}
