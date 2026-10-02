package config_test

import (
	"testing"

	"switchback/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg == nil {
		t.Fatalf("DefaultConfig returned nil")
	}

	if !cfg.LogToFile {
		t.Errorf("Expected LogToFile to default to true")
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel to be 'info', got %q", cfg.LogLevel)
	}

	if _, ok := cfg.Agents["antigravity"]; !ok {
		t.Errorf("Expected antigravity agent config to be present")
	}

	if _, ok := cfg.Agents["claude-code"]; !ok {
		t.Errorf("Expected claude-code agent config to be present")
	}
}

func TestConfigLoad(t *testing.T) {
	cfg := config.LoadConfig()
	if cfg == nil {
		t.Fatalf("Loaded config is nil")
	}

	path, err := config.GetConfigFilePath()
	if err != nil || path == "" {
		t.Errorf("Expected valid config file path, got %q (err: %v)", path, err)
	}
}
