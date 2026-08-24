package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads config from YAML file and applies validation.
//
// If path is empty, only default config is returned.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		if err := cfg.Validate(); err != nil {
			return nil, err
		}

		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}
