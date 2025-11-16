package server

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	Cameras []CameraConfig `json:"cameras"`
}

type CameraConfig struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &c, nil
}
