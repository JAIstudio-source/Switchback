package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type AgentConfig struct {
	TitlePatterns    []string `json:"title_patterns"`
	AutoDetectParent bool     `json:"auto_detect_parent"`
	DebounceMS       int      `json:"debounce_ms"`
}

type Config struct {
	LogToFile bool                   `json:"log_to_file"`
	LogLevel  string                 `json:"log_level"` // "error", "info", "debug"
	Agents    map[string]AgentConfig `json:"agents"`
}

func DefaultConfig() *Config {
	return &Config{
		LogToFile: true,
		LogLevel:  "info",
		Agents: map[string]AgentConfig{
			"claude-code": {
				TitlePatterns:    []string{"Claude", "Windows Terminal", "Visual Studio Code"},
				AutoDetectParent: true,
				DebounceMS:       0,
			},
			"codex": {
				TitlePatterns:    []string{"Codex", "Windows Terminal"},
				AutoDetectParent: true,
				DebounceMS:       0,
			},
			"antigravity": {
				TitlePatterns:    []string{"Antigravity", "Visual Studio Code"},
				AutoDetectParent: true,
				DebounceMS:       2000,
			},
		},
	}
}

func GetConfigFilePath() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	dir := filepath.Join(localAppData, "switchback")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadConfig() *Config {
	cfg := DefaultConfig()
	filePath, err := GetConfigFilePath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		// Does not exist yet, write default
		_ = SaveConfig(cfg)
		return cfg
	}
	_ = json.Unmarshal(data, cfg)
	return cfg
}

func SaveConfig(cfg *Config) error {
	filePath, err := GetConfigFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}
