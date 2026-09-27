package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SessionState struct {
	SavedHWND  uintptr   `json:"saved_hwnd"`
	SavedTitle string    `json:"saved_title,omitempty"`
	AgentHWND  uintptr   `json:"agent_hwnd,omitempty"`
	AgentType  string    `json:"agent_type,omitempty"`
	State      string    `json:"state,omitempty"` // "waiting_user", "agent_running"
	UpdatedAt  time.Time `json:"updated_at"`
}

type Store struct {
	Version  int                     `json:"version"`
	Sessions map[string]SessionState `json:"sessions"`
}

var mu sync.Mutex

// GetStateFilePath returns %LOCALAPPDATA%\focusmgr\state.json
func GetStateFilePath() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	dir := filepath.Join(localAppData, "focusmgr")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

// Load reads and parses the state file safely.
func Load() (*Store, error) {
	mu.Lock()
	defer mu.Unlock()

	filePath, err := GetStateFilePath()
	if err != nil {
		return nil, err
	}

	store := &Store{
		Version:  1,
		Sessions: make(map[string]SessionState),
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return store, nil
	}

	if err := json.Unmarshal(data, store); err != nil {
		// Corrupted or incompatible, reset cleanly
		return store, nil
	}
	if store.Sessions == nil {
		store.Sessions = make(map[string]SessionState)
	}

	return store, nil
}

// Save writes the store atomically via temporary file replacement.
func (s *Store) Save() error {
	mu.Lock()
	defer mu.Unlock()

	filePath, err := GetStateFilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp state file: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tmpPath, filePath); err != nil {
		// If atomic rename fails on Windows because file exists, remove and rename
		_ = os.Remove(filePath)
		if err := os.Rename(tmpPath, filePath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("failed to commit state file: %w", err)
		}
	}

	return nil
}

// SaveSession updates or creates a session entry and saves the store.
func SaveSession(sessionID string, s SessionState) error {
	store, err := Load()
	if err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	store.Sessions[sessionID] = s
	return store.Save()
}

// GetSession retrieves the session state for a given ID.
func GetSession(sessionID string) (SessionState, bool, error) {
	store, err := Load()
	if err != nil {
		return SessionState{}, false, err
	}
	s, found := store.Sessions[sessionID]
	return s, found, nil
}

// ClearSession removes a session from tracking.
func ClearSession(sessionID string) error {
	store, err := Load()
	if err != nil {
		return err
	}
	delete(store.Sessions, sessionID)
	return store.Save()
}
