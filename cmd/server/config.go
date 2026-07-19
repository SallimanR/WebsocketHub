package server

import (
	"encoding/json"
	"fmt"
	"os"
)

type ChannelConfig struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

type Config struct {
	Port     int             `json:"port"`
	Roles    []string        `json:"roles"`
	Channels []ChannelConfig `json:"channels"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{Port: 8080}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	return cfg, nil
}
