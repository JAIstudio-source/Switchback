package state

import (
	"crypto/rand"
	"encoding/hex"
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
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Message        string    `json:"message"`
	Type           string    `json:"type"` // "confirm", "question", "notification"
	Options        []string  `json:"options,omitempty"`
	ToolName       string    `json:"tool_name,omitempty"`
	ContextSnippet string    `json:"context_snippet,omitempty"`
	DangerLevel    string    `json:"danger_level,omitempty"` // "safe", "warning", "danger"
	CreatedAt      time.Time `json:"created_at"`
}

type ApprovalResponse struct {
	ID        string    `json:"id"`
	Decision  string    `json:"decision"`
	Answer    string    `json:"answer"`
	Index     int       `json:"index"`
	Timestamp time.Time `json:"timestamp"`
}

type Store struct {
	Version               int                     `json:"version"`
	SessionToken          string                  `json:"session_token,omitempty"`
	SelectedAgentHWND     uintptr                 `json:"selected_agent_hwnd"`
	SelectedAgentTitle    string                  `json:"selected_agent_title"`
	SelectedAgentType     string                  `json:"selected_agent_type"`
	SelectedWorkHWND      uintptr                 `json:"selected_work_hwnd"`
	SelectedWorkTitle     string                  `json:"selected_work_title"`
	GamingMode            bool                    `json:"gaming_mode"`             // Notification only for permission
	AutoSwitchEnabled     bool                    `json:"auto_switch_enabled"`     // Auto switch when task sent / done
	FullscreenGuard       bool                    `json:"fullscreen_guard"`        // Do not steal focus if watching fullscreen video or in game
	MeetingGuard          bool                    `json:"meeting_guard"`           // Suppress focus stealing during Zoom/Teams/Discord calls
	MediaControl          bool                    `json:"media_control"`           // Pause media on switch to agent, resume on prompt submit
	MediaPaused           bool                    `json:"media_paused"`            // Tracks if media was paused by switchback
	MobileControlActive   bool                    `json:"mobile_control_active"`   // When true, mobile is primary and PC focus switching is suppressed
	AutoApproveSafe       bool                    `json:"auto_approve_safe"`       // Silently auto-approves safe read-only commands
	KeepInTray            bool                    `json:"keep_in_tray"`            // Keep running in Windows system tray when browser closes
	TotalTasksCompleted   int                     `json:"total_tasks_completed"`   // Number of tasks completed
	TotalApprovalsHandled int                     `json:"total_approvals_handled"` // Number of approvals answered
	TotalTimeSavedSecs    int64                   `json:"total_time_saved_secs"`    // Accumulated time saved (in seconds)
	SessionStartTime      time.Time               `json:"session_start_time"`      // Session start timestamp
	LastTaskStartTime     time.Time               `json:"last_task_start_time"`     // Last task execution start timestamp
	PendingApproval       *PendingApprovalData    `json:"pending_approval,omitempty"`
	LastApprovalReply     *ApprovalResponse       `json:"last_approval_reply,omitempty"`
	LastMobilePrompt      string                  `json:"last_mobile_prompt,omitempty"`
	LatestAgentOutput     string                  `json:"latest_agent_output,omitempty"`
	LastHookEvent         string                  `json:"last_hook_event"` // For debouncing duplicate hook invocations
	LastHookTime          time.Time               `json:"last_hook_time"`  // For debouncing duplicate hook invocations
	Stopped               bool                    `json:"stopped"`         // Emergency stop / Pause switchback completely
	CurrentStatus         string                  `json:"current_status"`  // "idle", "running", "permission_needed", "completed"
	ActiveConversationID  string                  `json:"active_conversation_id,omitempty"`
	Sessions              map[string]SessionState `json:"sessions"`
}

var mu sync.Mutex

func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sb_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

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

// loadLocked reads and parses the state file without acquiring mu.
func loadLocked() (*Store, error) {
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
			store.SessionToken = generateToken()
			return store, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		store.SessionToken = generateToken()
		return store, nil
	}

	if err := json.Unmarshal(data, store); err != nil {
		store.SessionToken = generateToken()
		return store, nil
	}
	if store.Sessions == nil {
		store.Sessions = make(map[string]SessionState)
	}
	if store.SessionToken == "" {
		store.SessionToken = generateToken()
	}

	return store, nil
}

// saveLocked writes the store atomically via temporary file replacement without acquiring mu.
func saveLocked(s *Store) error {
	filePath, err := GetStateFilePath()
	if err != nil {
		return err
	}

	if s.SessionToken == "" {
		s.SessionToken = generateToken()
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp state file: %w", err)
	}

	// Atomic replace: Do NOT delete filePath if rename fails (preserves existing valid state)
	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit state file: %w", err)
	}

	return nil
}

// Load reads and parses the state file safely.
func Load() (*Store, error) {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked()
}

// Save writes the store atomically via temporary file replacement.
func (s *Store) Save() error {
	mu.Lock()
	defer mu.Unlock()
	return saveLocked(s)
}

// SetSelectedAgent stores the active agent window.
func SetSelectedAgent(hwnd uintptr, title, agentType string) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.SelectedAgentHWND = hwnd
	store.SelectedAgentTitle = title
	store.SelectedAgentType = agentType
	return saveLocked(store)
}

// SetSelectedWork stores the active user work/gaming window.
func SetSelectedWork(hwnd uintptr, title string) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.SelectedWorkHWND = hwnd
	store.SelectedWorkTitle = title
	return saveLocked(store)
}

// SetModes updates the gaming mode, autoswitch, fullscreen guard, meeting guard, media control, mobile control, and stopped state toggles.
func SetModes(gamingMode, autoSwitch, fullscreenGuard, meetingGuard, mediaControl, mobileControlActive, stopped bool) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
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
	return saveLocked(store)
}

// SetSafeAutoApprove sets the auto-approval mode directly.
func SetSafeAutoApprove(val bool) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.AutoApproveSafe = val
	return saveLocked(store)
}

// SetKeepInTray sets the background tray persistence mode directly.
func SetKeepInTray(val bool) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.KeepInTray = val
	return saveLocked(store)
}

// ToggleStopped toggles the master stop/pause state.
func ToggleStopped() (bool, error) {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return false, err
	}
	store.Stopped = !store.Stopped
	err = saveLocked(store)
	return store.Stopped, err
}

// SetStopped sets the master stop/pause state directly without mutating mobile control state.
func SetStopped(stopped bool) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.Stopped = stopped
	return saveLocked(store)
}

// SaveSession updates or creates a session entry.
func SaveSession(sessionID string, s SessionState) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	store.Sessions[sessionID] = s
	return saveLocked(store)
}

// GetSession retrieves session state.
func GetSession(sessionID string) (SessionState, bool, error) {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return SessionState{}, false, err
	}
	s, found := store.Sessions[sessionID]
	return s, found, nil
}

// SetMobileControl toggles whether mobile control is active (and PC focus switching suppressed).
func SetMobileControl(active bool) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.MobileControlActive = active
	return saveLocked(store)
}

// SetPendingApproval stores a live approval/question event for mobile devices.
func SetPendingApproval(data *PendingApprovalData) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.PendingApproval = data
	store.LastApprovalReply = nil // Clear any stale reply
	store.CurrentStatus = "permission_needed"
	return saveLocked(store)
}

// SetApprovalReply registers the user's mobile answer and clears the pending approval.
func SetApprovalReply(id, decision, answer string, index int) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
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
	return saveLocked(store)
}

// ClearPendingApproval removes any pending approval.
func ClearPendingApproval() error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.PendingApproval = nil
	if store.CurrentStatus == "permission_needed" {
		store.CurrentStatus = "running"
	}
	return saveLocked(store)
}

// SetLastMobilePrompt sets the most recent prompt submitted from mobile.
func SetLastMobilePrompt(prompt string) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.LastMobilePrompt = prompt
	store.CurrentStatus = "agent_working"
	return saveLocked(store)
}

// SetLatestAgentOutput stores output from the AI agent to display on mobile.
func SetLatestAgentOutput(output string) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.LatestAgentOutput = output
	return saveLocked(store)
}

// SetCurrentStatus updates the overall status string.
func SetCurrentStatus(status string) error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.CurrentStatus = status
	return saveLocked(store)
}

// GetLatestAgentOutput retrieves the current agent output.
func GetLatestAgentOutput() (string, error) {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
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
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.ActiveConversationID = clean
	return saveLocked(store)
}

// ToggleAutoApproveSafe toggles silent approval of safe read-only commands.
func ToggleAutoApproveSafe() (bool, error) {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return false, err
	}
	store.AutoApproveSafe = !store.AutoApproveSafe
	return store.AutoApproveSafe, saveLocked(store)
}

// ToggleKeepInTray toggles whether switchback persists in system tray when browser closes.
func ToggleKeepInTray() (bool, error) {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return false, err
	}
	store.KeepInTray = !store.KeepInTray
	return store.KeepInTray, saveLocked(store)
}

// RecordTaskStart marks the start of an agent task execution.
func RecordTaskStart() error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.LastTaskStartTime = time.Now()
	if store.SessionStartTime.IsZero() {
		store.SessionStartTime = time.Now()
	}
	store.CurrentStatus = "agent_working"
	return saveLocked(store)
}

// RecordTaskEnd records completion of an agent task and accumulates time saved.
func RecordTaskEnd() error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.TotalTasksCompleted++
	if !store.LastTaskStartTime.IsZero() {
		diff := time.Since(store.LastTaskStartTime)
		if diff > 0 && diff < 3*time.Hour { // Avoid bogus large times on resume
			store.TotalTimeSavedSecs += int64(diff.Seconds())
		}
	}
	store.CurrentStatus = "completed"
	return saveLocked(store)
}

// RecordApprovalHandled increments the count of approvals processed.
func RecordApprovalHandled() error {
	mu.Lock()
	defer mu.Unlock()
	store, err := loadLocked()
	if err != nil {
		return err
	}
	store.TotalApprovalsHandled++
	return saveLocked(store)
}

