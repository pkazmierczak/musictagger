package internal

import (
	"encoding/json"
	"io"
	"os"
)

// Config holds all configuration for musictagger
type Config struct {
	Replacements map[string]string `json:"replacements"`
	Pattern      Pattern           `json:"pattern"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Replacements: make(map[string]string),
		Pattern:      DefaultPattern(),
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()

	file, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return config, err
	}

	if err := json.Unmarshal(b, &config); err != nil {
		return config, err
	}

	return config, nil
}
