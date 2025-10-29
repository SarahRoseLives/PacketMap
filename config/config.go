package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// --- NEW ---
// MapConfig holds map-specific settings
type MapConfig struct {
	DefaultZoom float64 `toml:"defaultzoom"`
}

// Config holds all application configuration
type Config struct {
	Station StationConfig `toml:"station"`
	Map     MapConfig     `toml:"map"` // --- ADDED ---
}

// StationConfig holds settings specific to the user's station
type StationConfig struct {
	Callsign   string `toml:"callsign"`
	Passcode   int    `toml:"passcode"`
	GridSquare string `toml:"gridsquare"`
}

// LoadConfig reads the configuration from the specified path
func LoadConfig() (Config, error) {
	path := "config.toml" // Assumes config is in the root
	var conf Config

	data, err := os.ReadFile(path)
	if err != nil {
		return conf, err
	}

	if err := toml.Unmarshal(data, &conf); err != nil {
		return conf, err
	}

	return conf, nil
}