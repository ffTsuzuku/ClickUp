package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ClickupAPIKey   string `json:"clickup_api_key"`
	ClickupUserName string `json:"clickup_user_name"`
	ClickupListID   string `json:"clickup_list_id"`
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".config", "totui", "totui.json")
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
