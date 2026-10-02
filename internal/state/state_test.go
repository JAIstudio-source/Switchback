package state_test

import (
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
