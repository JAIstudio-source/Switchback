package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"switchback/internal/server"
	"switchback/internal/state"
)

func TestMobileEndpoints(t *testing.T) {
	mux := server.SetupRoutes(nil)

	// 1. Test /api/mobile/status
	req := httptest.NewRequest(http.MethodGet, "/api/mobile/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/mobile/status, got %d", rr.Code)
	}

	var statusResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("Failed to parse /api/mobile/status JSON: %v", err)
	}
	if _, ok := statusResp["current_status"]; !ok {
		t.Errorf("Expected current_status in mobile status response")
	}

	// 2. Test /api/mobile/info
	reqInfo := httptest.NewRequest(http.MethodGet, "/api/mobile/info", nil)
	rrInfo := httptest.NewRecorder()
	mux.ServeHTTP(rrInfo, reqInfo)

	if rrInfo.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/mobile/info, got %d", rrInfo.Code)
	}

	var infoResp map[string]interface{}
	if err := json.Unmarshal(rrInfo.Body.Bytes(), &infoResp); err != nil {
		t.Fatalf("Failed to parse /api/mobile/info JSON: %v", err)
	}
	if _, ok := infoResp["mobile_url"]; !ok {
		t.Errorf("Expected mobile_url in mobile info response")
	}

	// 3. Test /api/mobile/execute (local workspace command)
	execBody := []byte(`{"command":"echo 'testing-switchback'"}`)
	reqExec := httptest.NewRequest(http.MethodPost, "/api/mobile/execute", bytes.NewBuffer(execBody))
	reqExec.Header.Set("Content-Type", "application/json")
	rrExec := httptest.NewRecorder()
	mux.ServeHTTP(rrExec, reqExec)

	if rrExec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/mobile/execute, got %d", rrExec.Code)
	}

	// 4. Test /api/mobile/reply
	replyBody := []byte(`{"id":"test_id","decision":"allow","answer":"Unit test allow","index":1}`)
	reqReply := httptest.NewRequest(http.MethodPost, "/api/mobile/reply", bytes.NewBuffer(replyBody))
	reqReply.Header.Set("Content-Type", "application/json")
	rrReply := httptest.NewRecorder()
	mux.ServeHTTP(rrReply, reqReply)

	if rrReply.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/mobile/reply, got %d", rrReply.Code)
	}
}

func TestMobilePromptEndpointValidation(t *testing.T) {
	mux := server.SetupRoutes(nil)

	// Empty prompt should be rejected with 400 Bad Request
	emptyBody := []byte(`{"prompt":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/mobile/prompt", bytes.NewBuffer(emptyBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for empty prompt, got %d", rr.Code)
	}

	// Valid prompt should succeed
	validBody := []byte(`{"prompt":"Run tests and verify."}`)
	reqValid := httptest.NewRequest(http.MethodPost, "/api/mobile/prompt", bytes.NewBuffer(validBody))
	reqValid.Header.Set("Content-Type", "application/json")
	rrValid := httptest.NewRecorder()
	mux.ServeHTTP(rrValid, reqValid)

	if rrValid.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for valid prompt, got %d", rrValid.Code)
	}

	store, _ := state.Load()
	if store.LastMobilePrompt != "Run tests and verify." {
		t.Errorf("Expected LastMobilePrompt to be updated, got %q", store.LastMobilePrompt)
	}
}
