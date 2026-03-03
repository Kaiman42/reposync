package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	BasePaths []string `json:"base_paths"`
	Theme     string   `json:"theme"`
}

func loadConfig() Config {
	execPath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(execPath), "config.json")

	var cfg Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		cfg.BasePaths = []string{"D:\\Repos"}
		cfg.Theme = "dark"
		saveConfig(cfg)
		return cfg
	}

	json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg Config) {
	execPath, _ := os.Executable()
	configPath := filepath.Join(filepath.Dir(execPath), "config.json")
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)
}
