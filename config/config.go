package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// MapConfig holds map-specific settings
type MapConfig struct {
	DefaultZoom float64 `toml:"defaultzoom"`
}

// StationConfig holds settings specific to the user's station
type StationConfig struct {
	Callsign   string `toml:"callsign"`
	GridSquare string `toml:"gridsquare"`
	// Passcode removed from here
}

// --- NEW ---
// InterfaceConfig holds settings for the TNC/network connection
type InterfaceConfig struct {
	Type     string `toml:"type"`
	Device   string `toml:"device"`
	Passcode int    `toml:"passcode"`
}

// Config holds all application configuration
type Config struct {
	Station   StationConfig   `toml:"station"`
	Map       MapConfig       `toml:"map"`
	Interface InterfaceConfig `toml:"interface"` // --- ADDED ---
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