package config_test

import (
	"testing"

	"switchback/internal/config"
)

func TestConfigLoadAndSave(t *testing.T) {
	cfg := config.LoadConfig()
	if cfg == nil {
		t.Fatalf("Expected non-nil config")
	}

	if len(cfg.Agents) == 0 {
		t.Errorf("Expected default agents to be populated")
	}

	if _, ok := cfg.Agents["antigravity"]; !ok {
		t.Errorf("Expected antigravity agent in default config")
	}

	// Test saving
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	reloaded := config.LoadConfig()
	if reloaded == nil {
		t.Fatalf("Expected reloaded config to be non-nil")
	}
}
