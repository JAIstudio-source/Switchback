package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"focusmgr/internal/installer"
	"focusmgr/internal/logger"
	"focusmgr/internal/notify"
	"focusmgr/internal/state"
	"focusmgr/internal/win32"
)

// StartServer launches the HTTP server and opens the browser.
func StartServer(port int, embeddedFS fs.FS) error {
	mux := http.NewServeMux()

	// API Handlers
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/windows", handleWindows)
	mux.HandleFunc("/api/select-agent", handleSelectAgent)
	mux.HandleFunc("/api/select-work", handleSelectWork)
	mux.HandleFunc("/api/pick-work-switch", handlePickWorkSwitch)
	mux.HandleFunc("/api/toggle-mode", handleToggleMode)
	mux.HandleFunc("/api/test-focus", handleTestFocus)
	mux.HandleFunc("/api/install", handleInstall)
	mux.HandleFunc("/api/logs", handleLogs)

	// Static UI assets
	var fileServer http.Handler
	if _, err := os.Stat("ui/index.html"); err == nil {
		fileServer = http.FileServer(http.Dir("ui"))
	} else if sub, err := fs.Sub(embeddedFS, "ui"); err == nil {
		fileServer = http.FileServer(http.FS(sub))
	} else {
		fileServer = http.FileServer(http.FS(embeddedFS))
	}

	mux.Handle("/", fileServer)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := fmt.Sprintf("http://%s", addr)

	fmt.Println("=======================================================")
	fmt.Printf(" [PIXEL UI] focusmgr dashboard running at: %s\n", url)
	fmt.Println(" Opening dashboard in your default browser...")
	fmt.Println(" Press Ctrl+C in this terminal window to stop.")
	fmt.Println("=======================================================")

	// Automatically open browser
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}()

	return http.ListenAndServe(addr, mux)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	fg := win32.GetForegroundWindow()
	title := win32.GetWindowTitle(fg)
	store, _ := state.Load()

	if store.SelectedAgentHWND != 0 && !win32.IsWindowValid(win32.HWND(store.SelectedAgentHWND)) {
		store.SelectedAgentHWND = 0
		store.SelectedAgentTitle = ""
		_ = store.Save()
	}
	if store.SelectedWorkHWND != 0 && !win32.IsWindowValid(win32.HWND(store.SelectedWorkHWND)) {
		store.SelectedWorkHWND = 0
		store.SelectedWorkTitle = ""
		_ = store.Save()
	}

	resp := map[string]interface{}{
		"current_hwnd":         fmt.Sprintf("%d", fg),
		"current_title":        title,
		"selected_agent_hwnd":  store.SelectedAgentHWND,
		"selected_agent_title": store.SelectedAgentTitle,
		"selected_agent_type":  store.SelectedAgentType,
		"selected_work_hwnd":   store.SelectedWorkHWND,
		"selected_work_title":  store.SelectedWorkTitle,
		"gaming_mode":          store.GamingMode,
		"auto_switch_enabled":  store.AutoSwitchEnabled,
		"current_status":       store.CurrentStatus,
		"active_sessions":      len(store.Sessions),
	}

	writeJSON(w, resp)
}

func handleWindows(w http.ResponseWriter, r *http.Request) {
	windowsList := win32.GetOpenWindows()

	agents := []win32.WindowInfo{}
	workApps := []win32.WindowInfo{}

	for _, win := range windowsList {
		if win.IsAgent {
			agents = append(agents, win)
		} else {
			workApps = append(workApps, win)
		}
	}

	writeJSON(w, map[string]interface{}{
		"agents":    agents,
		"work_apps": workApps,
	})
}

func handleSelectAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		HWND      uintptr `json:"hwnd"`
		Title     string  `json:"title"`
		AgentType string  `json:"agent_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	_ = state.SetSelectedAgent(req.HWND, req.Title, req.AgentType)
	logger.Info("[Config] Selected Agent Window: HWND %d (\"%s\")", req.HWND, req.Title)
	writeJSON(w, map[string]interface{}{"success": true})
}

func handleSelectWork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		HWND  uintptr `json:"hwnd"`
		Title string  `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	_ = state.SetSelectedWork(req.HWND, req.Title)
	logger.Info("[Config] Selected Work/Gaming Window: HWND %d (\"%s\")", req.HWND, req.Title)
	writeJSON(w, map[string]interface{}{"success": true})
}

func handlePickWorkSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Capture target window after 3.2 seconds countdown
	time.Sleep(3200 * time.Millisecond)

	fg := win32.GetForegroundWindow()
	title := win32.GetWindowTitle(fg)

	if fg != 0 && win32.IsWindowValid(fg) {
		_ = state.SetSelectedWork(uintptr(fg), title)
		logger.Info("[Config] Captured Work Window via switch: HWND %d (\"%s\")", fg, title)
		writeJSON(w, map[string]interface{}{
			"success": true,
			"hwnd":    fg,
			"title":   title,
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": false,
		"message": "Failed to capture window. Try again.",
	})
}

func handleToggleMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GamingMode        bool `json:"gaming_mode"`
		AutoSwitchEnabled bool `json:"auto_switch_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	_ = state.SetModes(req.GamingMode, req.AutoSwitchEnabled)
	logger.Info("[Config] Updated modes - GamingMode: %v, AutoSwitch: %v", req.GamingMode, req.AutoSwitchEnabled)
	writeJSON(w, map[string]interface{}{"success": true})
}

func handleTestFocus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store, _ := state.Load()

	// Simulated test sequence
	go func() {
		// 1. Task start: switch to work window if set
		if store.SelectedWorkHWND != 0 && win32.IsWindowValid(win32.HWND(store.SelectedWorkHWND)) {
			_ = win32.SetForegroundWindowWithBypass(win32.HWND(store.SelectedWorkHWND))
			logger.Info("[Test] Switched to Work Window: HWND %d", store.SelectedWorkHWND)
		}

		time.Sleep(2500 * time.Millisecond)

		// 2. Permission requested
		if store.GamingMode {
			logger.Info("[Test] Gaming Mode: Sending Toast Notification for permission!")
			notify.SendToast("focusmgr // AI Agent Alert", "Agent needs permission: Review tool call!")
		} else if store.SelectedAgentHWND != 0 && win32.IsWindowValid(win32.HWND(store.SelectedAgentHWND)) {
			_ = win32.SetForegroundWindowWithBypass(win32.HWND(store.SelectedAgentHWND))
			logger.Info("[Test] Switched back to Agent Window: HWND %d", store.SelectedAgentHWND)
		}
	}()

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Simulated task flow triggered! Check window focus & notification.",
	})
}

func handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	exePath, _ := os.Executable()
	exePath, _ = filepath.Abs(exePath)

	_ = installer.InstallClaudeHooks(exePath)
	cwd, _ := os.Getwd()
	_ = installer.InstallAntigravityHooks(exePath, cwd)

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Hooks successfully configured for Claude Code and Antigravity!",
	})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	localAppData := os.Getenv("LOCALAPPDATA")
	logPath := filepath.Join(localAppData, "focusmgr", "focusmgr.log")

	data, err := os.ReadFile(logPath)
	if err != nil {
		writeJSON(w, []string{"[INFO] Log file created."})
		return
	}

	lines := strings.Split(string(data), "\n")
	filtered := []string{}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			filtered = append(filtered, l)
		}
	}

	if len(filtered) > 25 {
		filtered = filtered[len(filtered)-25:]
	}

	writeJSON(w, filtered)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
