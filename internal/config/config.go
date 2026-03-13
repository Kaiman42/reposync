package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	BasePaths []string `json:"base_paths"`
	Theme     string   `json:"theme"`
}

func getConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		execPath, _ := os.Executable()
		return filepath.Join(filepath.Dir(execPath), "config.json")
	}
	appConfigDir := filepath.Join(configDir, "reposync")
	os.MkdirAll(appConfigDir, 0755)
	return filepath.Join(appConfigDir, "config.json")
}

func LoadConfig() Config {
	configPath := getConfigPath()

	// Migração: Se não existe no AppData mas existe local, move para AppData
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		execPath, _ := os.Executable()
		localPath := filepath.Join(filepath.Dir(execPath), "config.json")
		if _, err := os.Stat(localPath); err == nil {
			data, _ := os.ReadFile(localPath)
			os.WriteFile(configPath, data, 0644)
			os.Remove(localPath) // Opcional: remove o antigo para limpar
		}
	}

	var cfg Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		cfg.BasePaths = []string{"D:\\Repos"}
		cfg.Theme = "dark"
		SaveConfig(cfg)
		return cfg
	}

	json.Unmarshal(data, &cfg)
	// Garantir que se o arquivo existir mas estiver vazio, tenha valores padrão
	if len(cfg.BasePaths) == 0 {
		cfg.BasePaths = []string{"D:\\Repos"}
	}
	return cfg
}

func SaveConfig(cfg Config) {
	configPath := getConfigPath()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(configPath, data, 0644)
}
