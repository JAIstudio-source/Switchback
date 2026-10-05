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
	store, _ := state.Load()

	// 1. Test /api/mobile/status
	req := httptest.NewRequest(http.MethodGet, "/api/mobile/status", nil)
	req.RemoteAddr = "127.0.0.1:50000"
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
	reqInfo.RemoteAddr = "127.0.0.1:50000"
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
	if _, ok := infoResp["session_token"]; !ok {
		t.Errorf("Expected session_token in mobile info response")
	}

	// 2b. Test Remote Unauthenticated /api/mobile/info (Must return 401 to prevent token leak)
	reqRemoteInfoUnauth := httptest.NewRequest(http.MethodGet, "/api/mobile/info", nil)
	reqRemoteInfoUnauth.RemoteAddr = "192.168.1.105:54321"
	rrRemoteInfoUnauth := httptest.NewRecorder()
	mux.ServeHTTP(rrRemoteInfoUnauth, reqRemoteInfoUnauth)
	if rrRemoteInfoUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for remote unauthenticated /api/mobile/info, got %d", rrRemoteInfoUnauth.Code)
	}

	// 2c. Test Remote Authenticated /api/mobile/info (Returns 200 OK with valid token)
	reqRemoteInfoAuth := httptest.NewRequest(http.MethodGet, "/api/mobile/info", nil)
	reqRemoteInfoAuth.RemoteAddr = "192.168.1.105:54321"
	reqRemoteInfoAuth.Header.Set("X-Switchback-Token", store.SessionToken)
	rrRemoteInfoAuth := httptest.NewRecorder()
	mux.ServeHTTP(rrRemoteInfoAuth, reqRemoteInfoAuth)
	if rrRemoteInfoAuth.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for remote /api/mobile/info with valid token, got %d", rrRemoteInfoAuth.Code)
	}

	// 3. Test /api/mobile/execute (local workspace command)
	execBody := []byte(`{"command":"echo 'testing-switchback'"}`)
	reqExec := httptest.NewRequest(http.MethodPost, "/api/mobile/execute", bytes.NewBuffer(execBody))
	reqExec.RemoteAddr = "127.0.0.1:50000"
	reqExec.Header.Set("Content-Type", "application/json")
	rrExec := httptest.NewRecorder()
	mux.ServeHTTP(rrExec, reqExec)

	if rrExec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/mobile/execute, got %d", rrExec.Code)
	}

	// 4. Test /api/mobile/reply
	replyBody := []byte(`{"id":"test_id","decision":"allow","answer":"Unit test allow","index":1}`)
	reqReply := httptest.NewRequest(http.MethodPost, "/api/mobile/reply", bytes.NewBuffer(replyBody))
	reqReply.RemoteAddr = "127.0.0.1:50000"
	reqReply.Header.Set("Content-Type", "application/json")
	rrReply := httptest.NewRecorder()
	mux.ServeHTTP(rrReply, reqReply)

	if rrReply.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/mobile/reply, got %d", rrReply.Code)
	}

	// 5. Test Remote Unauthenticated Access Rejected (SRV-01)
	reqRemoteUnauth := httptest.NewRequest(http.MethodPost, "/api/mobile/execute", bytes.NewBuffer(execBody))
	reqRemoteUnauth.RemoteAddr = "192.168.1.105:54321"
	reqRemoteUnauth.Header.Set("Content-Type", "application/json")
	rrRemoteUnauth := httptest.NewRecorder()
	mux.ServeHTTP(rrRemoteUnauth, reqRemoteUnauth)

	if rrRemoteUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for remote unauthenticated execute request, got %d", rrRemoteUnauth.Code)
	}

	// 6. Test Remote Authenticated Access Succeeded with X-Switchback-Token (SRV-01)
	reqRemoteAuth := httptest.NewRequest(http.MethodPost, "/api/mobile/execute", bytes.NewBuffer(execBody))
	reqRemoteAuth.RemoteAddr = "192.168.1.105:54321"
	reqRemoteAuth.Header.Set("Content-Type", "application/json")
	reqRemoteAuth.Header.Set("X-Switchback-Token", store.SessionToken)
	rrRemoteAuth := httptest.NewRecorder()
	mux.ServeHTTP(rrRemoteAuth, reqRemoteAuth)

	if rrRemoteAuth.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for remote request with valid X-Switchback-Token, got %d", rrRemoteAuth.Code)
	}
}

func TestMobilePromptEndpointValidation(t *testing.T) {
	mux := server.SetupRoutes(nil)

	// Empty prompt should be rejected with 400 Bad Request
	emptyBody := []byte(`{"prompt":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/mobile/prompt", bytes.NewBuffer(emptyBody))
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for empty prompt, got %d", rr.Code)
	}

	// Valid prompt should succeed
	validBody := []byte(`{"prompt":"Run tests and verify."}`)
	reqValid := httptest.NewRequest(http.MethodPost, "/api/mobile/prompt", bytes.NewBuffer(validBody))
	reqValid.RemoteAddr = "127.0.0.1:50000"
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

func TestHeartbeatAndDisconnectEndpoints(t *testing.T) {
	mux := server.SetupRoutes(nil)

	// Test /api/heartbeat
	reqHb := httptest.NewRequest(http.MethodPost, "/api/heartbeat?client=test_tab_1", nil)
	rrHb := httptest.NewRecorder()
	mux.ServeHTTP(rrHb, reqHb)

	if rrHb.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/heartbeat, got %d", rrHb.Code)
	}

	// Test /api/disconnect
	reqDc := httptest.NewRequest(http.MethodPost, "/api/disconnect?client=test_tab_1", nil)
	rrDc := httptest.NewRecorder()
	mux.ServeHTTP(rrDc, reqDc)

	if rrDc.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/disconnect, got %d", rrDc.Code)
	}
}

func TestLogsEndpoint(t *testing.T) {
	mux := server.SetupRoutes(nil)

	reqLogs := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	reqLogs.RemoteAddr = "127.0.0.1:50000"
	rrLogs := httptest.NewRecorder()
	mux.ServeHTTP(rrLogs, reqLogs)

	if rrLogs.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/logs, got %d", rrLogs.Code)
	}

	var logs []string
	if err := json.Unmarshal(rrLogs.Body.Bytes(), &logs); err != nil {
		t.Fatalf("Failed to parse /api/logs JSON array: %v", err)
	}
}

func TestToggleEndpoints(t *testing.T) {
	mux := server.SetupRoutes(nil)

	// Test /api/toggle-auto-approve
	reqAuto := httptest.NewRequest(http.MethodPost, "/api/toggle-auto-approve", nil)
	reqAuto.RemoteAddr = "127.0.0.1:50000"
	rrAuto := httptest.NewRecorder()
	mux.ServeHTTP(rrAuto, reqAuto)

	if rrAuto.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/toggle-auto-approve, got %d", rrAuto.Code)
	}

	var autoResp map[string]interface{}
	if err := json.Unmarshal(rrAuto.Body.Bytes(), &autoResp); err != nil {
		t.Fatalf("Failed to parse toggle-auto-approve JSON: %v", err)
	}
	if _, ok := autoResp["auto_approve_safe"]; !ok {
		t.Errorf("Expected auto_approve_safe in response")
	}

	// Test /api/toggle-tray
	reqTray := httptest.NewRequest(http.MethodPost, "/api/toggle-tray", nil)
	reqTray.RemoteAddr = "127.0.0.1:50000"
	rrTray := httptest.NewRecorder()
	mux.ServeHTTP(rrTray, reqTray)

	if rrTray.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/toggle-tray, got %d", rrTray.Code)
	}

	var trayResp map[string]interface{}
	if err := json.Unmarshal(rrTray.Body.Bytes(), &trayResp); err != nil {
		t.Fatalf("Failed to parse toggle-tray JSON: %v", err)
	}
	if _, ok := trayResp["keep_in_tray"]; !ok {
		t.Errorf("Expected keep_in_tray in response")
	}
}
