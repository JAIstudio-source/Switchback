package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"focusmgr/internal/config"
	"focusmgr/internal/installer"
	"focusmgr/internal/logger"
	"focusmgr/internal/state"
	"focusmgr/internal/win32"
)

// StartServer launches the HTTP server and opens the browser.
func StartServer(port int, embeddedFS fs.FS) error {
	mux := http.NewServeMux()

	// API Handlers
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/test-focus", handleTestFocus)
	mux.HandleFunc("/api/save-and-focus", handleSaveAndFocus)
	mux.HandleFunc("/api/restore", handleRestore)
	mux.HandleFunc("/api/install", handleInstall)
	mux.HandleFunc("/api/logs", handleLogs)
	mux.HandleFunc("/api/config", handleConfig)

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

	activeCount := 0
	if store != nil {
		activeCount = len(store.Sessions)
	}

	resp := map[string]interface{}{
		"current_hwnd":    fmt.Sprintf("%d", fg),
		"current_title":   title,
		"active_sessions": activeCount,
	}

	writeJSON(w, resp)
}

func handleTestFocus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fg := win32.GetForegroundWindow()
	title := win32.GetWindowTitle(fg)

	// Wait 1.5s, then switch back
	go func(target win32.HWND) {
		time.Sleep(1500 * time.Millisecond)
		if win32.IsWindowValid(target) {
			_ = win32.SetForegroundWindowWithBypass(target)
		}
	}(fg)

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Captured HWND %d (\"%s\"). Restoring in 1.5s!", fg, title),
	})
}

func handleSaveAndFocus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userHWND := win32.GetForegroundWindow()
	userTitle := win32.GetWindowTitle(userHWND)

	cfg := config.LoadConfig()
	agentHWND := win32.FindAncestorWindow()
	if agentHWND == 0 {
		agentHWND = win32.FindWindowByTitlePattern([]string{"Visual Studio Code", "Antigravity", "Claude", "Terminal"})
	}

	sess := state.SessionState{
		SavedHWND:  uintptr(userHWND),
		SavedTitle: userTitle,
		AgentHWND:  uintptr(agentHWND),
		AgentType:  "dashboard",
		State:      "waiting_user_prompt",
		UpdatedAt:  time.Now(),
	}
	_ = state.SaveSession("dashboard", sess)

	if agentHWND != 0 && agentHWND != userHWND {
		_ = win32.SetForegroundWindowWithBypass(agentHWND)
	}

	_ = cfg
	writeJSON(w, map[string]interface{}{
		"success":    true,
		"saved_hwnd": userHWND,
		"agent_hwnd": agentHWND,
	})
}

func handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess, found, err := state.GetSession("dashboard")
	if err == nil && found && win32.IsWindowValid(win32.HWND(sess.SavedHWND)) {
		_ = win32.SetForegroundWindowWithBypass(win32.HWND(sess.SavedHWND))
	}

	writeJSON(w, map[string]interface{}{"success": true})
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

	// Return last 20 lines
	if len(filtered) > 20 {
		filtered = filtered[len(filtered)-20:]
	}

	writeJSON(w, filtered)
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		cfg := config.LoadConfig()
		writeJSON(w, cfg)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	type ConfigReq struct {
		DebounceMS int      `json:"debounce_ms"`
		Patterns   []string `json:"patterns"`
	}

	var req ConfigReq
	if err := json.Unmarshal(body, &req); err == nil {
		cfg := config.LoadConfig()
		if ag, ok := cfg.Agents["antigravity"]; ok {
			ag.DebounceMS = req.DebounceMS
			cfg.Agents["antigravity"] = ag
		}
		if len(req.Patterns) > 0 {
			if ag, ok := cfg.Agents["claude-code"]; ok {
				ag.TitlePatterns = req.Patterns
				cfg.Agents["claude-code"] = ag
			}
		}
		_ = config.SaveConfig(cfg)
		logger.Info("[Config] Updated configuration via UI dashboard")
	}

	writeJSON(w, map[string]interface{}{"success": true})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
