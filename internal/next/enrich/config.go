package enrich

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds per-project enrichment configuration.
type Config struct {
	Provider            string `json:"provider"`
	Model               string `json:"model"`
	Reasoning           string `json:"reasoning"`
	StalenessExpiryDays int    `json:"staleness_expiry_days"`
}

func DefaultConfig() Config {
	return Config{
		Provider:            "claude",
		Model:               "sonnet",
		Reasoning:           "medium",
		StalenessExpiryDays: 30,
	}
}

func SaveConfig(cellPath string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cellPath, "enrichment.json"), data, 0o644)
}

func LoadConfig(cellPath string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(cellPath, "enrichment.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
