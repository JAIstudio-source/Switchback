package state_test

import (
	"os"
	"sync"
	"testing"
	"time"

	"switchback/internal/state"
)

func TestStateSaveAndLoad(t *testing.T) {
	store, err := state.Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Modify fields
	testTitle := "Test Agent Window Title"
	store.SelectedAgentTitle = testTitle
	store.GamingMode = true
	store.AutoSwitchEnabled = true
	store.MobileControlActive = true

	if err := store.Save(); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Reload
	reloaded, err := state.Load()
	if err != nil {
		t.Fatalf("Failed to reload state: %v", err)
	}

	if reloaded.SelectedAgentTitle != testTitle {
		t.Errorf("Expected title %q, got %q", testTitle, reloaded.SelectedAgentTitle)
	}
	if !reloaded.GamingMode {
		t.Errorf("Expected GamingMode to be true")
	}
	if !reloaded.MobileControlActive {
		t.Errorf("Expected MobileControlActive to be true")
	}
}

func TestPendingApprovalFlow(t *testing.T) {
	appData := &state.PendingApprovalData{
		ID:        "test_approval_123",
		Title:     "Execute Unit Tests",
		Message:   "Agent needs approval to run tests",
		Type:      "question",
		Options:   []string{"Yes, run", "No, skip"},
		CreatedAt: time.Now(),
	}

	if err := state.SetPendingApproval(appData); err != nil {
		t.Fatalf("Failed to set pending approval: %v", err)
	}

	store, err := state.Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if store.PendingApproval == nil || store.PendingApproval.ID != "test_approval_123" {
		t.Errorf("Expected pending approval ID test_approval_123, got %+v", store.PendingApproval)
	}

	// Reply to approval
	if err := state.SetApprovalReply("test_approval_123", "allow", "User approved via mobile", 1); err != nil {
		t.Fatalf("Failed to set approval reply: %v", err)
	}

	storeAfter, _ := state.Load()
	if storeAfter.PendingApproval != nil {
		t.Errorf("Expected PendingApproval to be cleared after reply")
	}
	if storeAfter.LastApprovalReply == nil || storeAfter.LastApprovalReply.Decision != "allow" {
		t.Errorf("Expected LastApprovalReply to record allow, got %+v", storeAfter.LastApprovalReply)
	}
}

func TestMobilePromptState(t *testing.T) {
	prompt := "Mobile remote prompt verification"
	if err := state.SetLastMobilePrompt(prompt); err != nil {
		t.Fatalf("Failed to set mobile prompt: %v", err)
	}

	store, err := state.Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if store.LastMobilePrompt != prompt {
		t.Errorf("Expected prompt %q, got %q", prompt, store.LastMobilePrompt)
	}
}

func TestSetStoppedDoesNotResetMobileControl(t *testing.T) {
	// Enable mobile control
	if err := state.SetMobileControl(true); err != nil {
		t.Fatalf("Failed to set mobile control: %v", err)
	}

	// Pause then resume
	if err := state.SetStopped(true); err != nil {
		t.Fatalf("Failed to set stopped true: %v", err)
	}
	if err := state.SetStopped(false); err != nil {
		t.Fatalf("Failed to set stopped false: %v", err)
	}

	store, err := state.Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if !store.MobileControlActive {
		t.Errorf("Expected MobileControlActive to remain true after SetStopped(false)")
	}
	if store.Stopped {
		t.Errorf("Expected Stopped to be false")
	}
}

func TestConcurrentHelpersNoDeadlock(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			_ = state.SetCurrentStatus("running")
		}(i)
		go func(idx int) {
			defer wg.Done()
			_ = state.SetSelectedWork(uintptr(100+idx), "Work Window")
		}(i)
	}
	wg.Wait()
}

func TestProductivityAndFeatureToggles(t *testing.T) {
	tmpDir := t.TempDir()
	origLocal := os.Getenv("LOCALAPPDATA")
	os.Setenv("LOCALAPPDATA", tmpDir)
	defer os.Setenv("LOCALAPPDATA", origLocal)

	// Test ToggleAutoApproveSafe
	val1, err := state.ToggleAutoApproveSafe()
	if err != nil {
		t.Fatalf("ToggleAutoApproveSafe failed: %v", err)
	}
	if !val1 {
		t.Errorf("Expected val1 to be true, got %v", val1)
	}

	val2, err := state.ToggleAutoApproveSafe()
	if err != nil {
		t.Fatalf("ToggleAutoApproveSafe 2 failed: %v", err)
	}
	if val2 {
		t.Errorf("Expected val2 to be false, got %v", val2)
	}

	// Test ToggleKeepInTray
	tray1, err := state.ToggleKeepInTray()
	if err != nil {
		t.Fatalf("ToggleKeepInTray failed: %v", err)
	}
	if !tray1 {
		t.Errorf("Expected tray1 to be true, got %v", tray1)
	}

	// Test Task Metrics with simulated elapsed time
	_ = state.RecordTaskStart()
	
	// Manually backdate LastTaskStartTime to simulate 10 seconds of agent work
	store, _ := state.Load()
	store.LastTaskStartTime = time.Now().Add(-10 * time.Second)
	_ = store.Save()

	_ = state.RecordTaskEnd()
	_ = state.RecordApprovalHandled()

	store, err = state.Load()
	if err != nil {
		t.Fatalf("Load state failed: %v", err)
	}

	if store.TotalTasksCompleted != 1 {
		t.Errorf("Expected TotalTasksCompleted to be 1, got %d", store.TotalTasksCompleted)
	}
	if store.TotalApprovalsHandled != 1 {
		t.Errorf("Expected TotalApprovalsHandled to be 1, got %d", store.TotalApprovalsHandled)
	}
	if store.TotalTimeSavedSecs < 9 {
		t.Errorf("Expected TotalTimeSavedSecs >= 9, got %d", store.TotalTimeSavedSecs)
	}
}
