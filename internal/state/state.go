package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

type PendingApprovalData struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Type      string    `json:"type"` // "confirm", "question", "notification"
	Options   []string  `json:"options,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ApprovalResponse struct {
	ID        string    `json:"id"`
	Decision  string    `json:"decision"`
	Answer    string    `json:"answer"`
	Index     int       `json:"index"`
	Timestamp time.Time `json:"timestamp"`
}

type Store struct {
	Version             int                  `json:"version"`
	SelectedAgentHWND   uintptr              `json:"selected_agent_hwnd"`
	SelectedAgentTitle  string               `json:"selected_agent_title"`
	SelectedAgentType   string               `json:"selected_agent_type"`
	SelectedWorkHWND    uintptr              `json:"selected_work_hwnd"`
	SelectedWorkTitle   string               `json:"selected_work_title"`
	GamingMode          bool                 `json:"gaming_mode"`           // Notification only for permission
	AutoSwitchEnabled   bool                 `json:"auto_switch_enabled"`   // Auto switch when task sent / done
	FullscreenGuard     bool                 `json:"fullscreen_guard"`      // Do not steal focus if watching fullscreen video or in game
	MeetingGuard        bool                 `json:"meeting_guard"`         // Suppress focus stealing during Zoom/Teams/Discord calls
	MediaControl        bool                 `json:"media_control"`         // Pause media on switch to agent, resume on prompt submit
	MediaPaused         bool                 `json:"media_paused"`          // Tracks if media was paused by focusmgr
	MobileControlActive bool                 `json:"mobile_control_active"` // When true, mobile is primary and PC focus switching is suppressed
	PendingApproval     *PendingApprovalData `json:"pending_approval,omitempty"`
	LastApprovalReply   *ApprovalResponse    `json:"last_approval_reply,omitempty"`
	LastMobilePrompt    string               `json:"last_mobile_prompt,omitempty"`
	LatestAgentOutput   string               `json:"latest_agent_output,omitempty"`
	LastHookEvent       string               `json:"last_hook_event"` // For debouncing duplicate hook invocations
	LastHookTime        time.Time            `json:"last_hook_time"`  // For debouncing duplicate hook invocations
	Stopped             bool                 `json:"stopped"`         // Emergency stop / Pause focusmgr completely
	CurrentStatus       string               `json:"current_status"`  // "idle", "running", "permission_needed", "completed"
	ActiveConversationID string              `json:"active_conversation_id,omitempty"`
	Sessions            map[string]SessionState `json:"sessions"`
}

var mu sync.Mutex

// GetStateFilePath returns %LOCALAPPDATA%\switchback\state.json
func GetStateFilePath() (string, error) {
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
	targetPath := filepath.Join(dir, "state.json")

	// Backward compatibility: migrate old focusmgr state.json if it exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		oldStatePath := filepath.Join(localAppData, "focusmgr", "state.json")
		if oldData, err := os.ReadFile(oldStatePath); err == nil && len(oldData) > 0 {
			_ = os.WriteFile(targetPath, oldData, 0644)
		}
	}

	return targetPath, nil
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
		Version:           1,
		AutoSwitchEnabled: true,
		GamingMode:        false,
		FullscreenGuard:   true,
		MeetingGuard:      true,
		MediaControl:      false,
		Stopped:           false,
		CurrentStatus:     "idle",
		Sessions:          make(map[string]SessionState),
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
		_ = os.Remove(filePath)
		if err := os.Rename(tmpPath, filePath); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("failed to commit state file: %w", err)
		}
	}

	return nil
}

// SetSelectedAgent stores the active agent window.
func SetSelectedAgent(hwnd uintptr, title, agentType string) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.SelectedAgentHWND = hwnd
	store.SelectedAgentTitle = title
	store.SelectedAgentType = agentType
	return store.Save()
}

// SetSelectedWork stores the active user work/gaming window.
func SetSelectedWork(hwnd uintptr, title string) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.SelectedWorkHWND = hwnd
	store.SelectedWorkTitle = title
	return store.Save()
}

// SetModes updates the gaming mode, autoswitch, fullscreen guard, meeting guard, media control, mobile control, and stopped state toggles.
func SetModes(gamingMode, autoSwitch, fullscreenGuard, meetingGuard, mediaControl, mobileControlActive, stopped bool) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.GamingMode = gamingMode
	store.AutoSwitchEnabled = autoSwitch
	store.FullscreenGuard = fullscreenGuard
	store.MeetingGuard = meetingGuard
	store.MediaControl = mediaControl
	store.MobileControlActive = mobileControlActive
	store.Stopped = stopped
	return store.Save()
}

// ToggleStopped toggles the master stop/pause state.
func ToggleStopped() (bool, error) {
	store, err := Load()
	if err != nil {
		return false, err
	}
	store.Stopped = !store.Stopped
	err = store.Save()
	return store.Stopped, err
}

// SetStopped sets the master stop/pause state directly.
func SetStopped(stopped bool) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.Stopped = stopped
	if !stopped {
		// When resuming/starting, ensure mobile suppression is disabled by default
		store.MobileControlActive = false
	}
	return store.Save()
}

// SetStatus updates the active agent workflow status.
func SetStatus(status string) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.CurrentStatus = status
	return store.Save()
}

// SaveSession updates or creates a session entry.
func SaveSession(sessionID string, s SessionState) error {
	store, err := Load()
	if err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	store.Sessions[sessionID] = s
	return store.Save()
}

// GetSession retrieves session state.
func GetSession(sessionID string) (SessionState, bool, error) {
	store, err := Load()
	if err != nil {
		return SessionState{}, false, err
	}
	s, found := store.Sessions[sessionID]
	return s, found, nil
}

// SetMobileControl toggles whether mobile control is active (and PC focus switching suppressed).
func SetMobileControl(active bool) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.MobileControlActive = active
	return store.Save()
}

// SetPendingApproval stores a live approval/question event for mobile devices.
func SetPendingApproval(data *PendingApprovalData) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.PendingApproval = data
	store.CurrentStatus = "permission_needed"
	return store.Save()
}

// SetApprovalReply registers the user's mobile answer and clears the pending approval.
func SetApprovalReply(id, decision, answer string, index int) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.PendingApproval = nil
	store.LastApprovalReply = &ApprovalResponse{
		ID:        id,
		Decision:  decision,
		Answer:    answer,
		Index:     index,
		Timestamp: time.Now(),
	}
	if store.CurrentStatus == "permission_needed" {
		store.CurrentStatus = "running"
	}
	return store.Save()
}

// ClearPendingApproval removes any pending approval.
func ClearPendingApproval() error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.PendingApproval = nil
	if store.CurrentStatus == "permission_needed" {
		store.CurrentStatus = "running"
	}
	return store.Save()
}

// SetLastMobilePrompt sets the most recent prompt submitted from mobile.
func SetLastMobilePrompt(prompt string) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.LastMobilePrompt = prompt
	store.CurrentStatus = "agent_working"
	return store.Save()
}

// SetLatestAgentOutput stores output from the AI agent to display on mobile.
func SetLatestAgentOutput(output string) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.LatestAgentOutput = output
	return store.Save()
}

// SetCurrentStatus updates the overall status string.
func SetCurrentStatus(status string) error {
	store, err := Load()
	if err != nil {
		return err
	}
	store.CurrentStatus = status
	return store.Save()
}

// GetLatestAgentOutput retrieves the current agent output.
func GetLatestAgentOutput() (string, error) {
	store, err := Load()
	if err != nil {
		return "", err
	}
	return store.LatestAgentOutput, nil
}

// SetActiveConversationID records the active conversation ID.
func SetActiveConversationID(id string) error {
	clean := strings.TrimSpace(id)
	if clean == "" || clean == "default" {
		return nil
	}
	store, err := Load()
	if err != nil {
		return err
	}
	store.ActiveConversationID = clean
	return store.Save()
}

