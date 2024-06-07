// Package lib common library functions
package lib

import (
	"log"
	"os"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

func getConfigFile(file string) string {
	configFile, err := xdg.ConfigFile("izrss/" + file)
	if err != nil {
		log.Fatalf("could not find config file: %v", err)
	}
	return configFile
}

// LoadConfig loads the users configuration file and applies it to the config struct
func LoadConfig(config string) {
	if config == "" {
		config = getConfigFile("config.toml")
	}
	// ignore error since we can just use the default config
	configRaw, _ := os.ReadFile(config)

	if err := toml.Unmarshal(configRaw, &UserConfig); err != nil {
		log.Fatalf("could not unmarshal config: %v", err)
	}
}

// UserConfig is the global user configuration
var UserConfig = config{
	DateFormat: "02/01/2006",
	Urls:       []string{},
	Reader: reader{
		Size:          "recomended",
		ReadThreshold: 0.8,
	},
	Colors: colors{
		Text:       "#cdd6f4",
		Inverttext: "#1e1e2e",
		Subtext:    "#a6adc8",
		Accent:     "#74c7ec",
		Borders:    "#313244",
	},
}

// Config is the struct that holds the configuration
type config struct {
	Colors     colors   `toml:"colors"`
	Reader     reader   `toml:"reader"`
	DateFormat string   `toml:"dateformat"`
	Urls       []string `toml:"urls"`
}

type colors struct {
	Text       string `toml:"text"`
	Inverttext string `toml:"inverttext"`
	Subtext    string `toml:"subtext"`
	Accent     string `toml:"accent"`
	Borders    string `toml:"borders"`
}

type reader struct {
	Size          interface{} `toml:"size"`
	ReadThreshold float64     `toml:"read_threshold"`
}
