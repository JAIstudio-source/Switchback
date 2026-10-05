package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"switchback/internal/installer"
	"switchback/internal/logger"
	"switchback/internal/notify"
	"switchback/internal/state"
	"switchback/internal/win32"
)

var (
	clientMu           sync.Mutex
	activeClients      = make(map[string]time.Time)
	hasHadClients      = false
	shutdownSignalChan = make(chan struct{}, 1)
	staleStatusMu      sync.Mutex
	staleAgentCount    int
	staleWorkCount     int
	pickWorkMu         sync.Mutex
)

func TriggerShutdown() {
	select {
	case shutdownSignalChan <- struct{}{}:
	default:
	}
}

func isRequestAuthorized(r *http.Request) bool {
	// Loopback / localhost is always authorized
	host := r.RemoteAddr
	if host == "" || strings.HasPrefix(host, "127.0.0.1:") || strings.HasPrefix(host, "[::1]:") || strings.HasPrefix(host, "localhost:") {
		return true
	}

	store, err := state.Load()
	if err != nil || store.SessionToken == "" {
		return false
	}

	token := r.Header.Get("X-Switchback-Token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	return token != "" && token == store.SessionToken
}

func isValidUUID(s string) bool {
	if len(s) < 32 || len(s) > 40 {
		return false
	}
	hyphens := 0
	for _, c := range s {
		if c == '-' {
			hyphens++
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return hyphens >= 3
}

func RegisterClientHeartbeat(id string) {
	if id == "" {
		return
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	activeClients[id] = time.Now()
	hasHadClients = true
}

func DeregisterClient(id string) {
	if id == "" {
		return
	}
	clientMu.Lock()
	defer clientMu.Unlock()
	delete(activeClients, id)
}

func runClientActivityMonitor() {
	// Give initial 20-second grace period for browser window to launch and register heartbeat
	time.Sleep(20 * time.Second)

	emptyCount := 0
	for {
		time.Sleep(2 * time.Second)

		clientMu.Lock()
		now := time.Now()
		// Evict stale clients (> 90 seconds without heartbeat, accounting for throttled background tabs)
		for id, lastSeen := range activeClients {
			if now.Sub(lastSeen) > 90*time.Second {
				delete(activeClients, id)
			}
		}

		numClients := len(activeClients)
		shouldCheck := hasHadClients
		clientMu.Unlock()

		if shouldCheck {
			if numClients == 0 {
				store, _ := state.Load()
				if store != nil && store.KeepInTray {
					emptyCount = 0
					continue
				}
				emptyCount++
				// If 0 clients connected for 30 seconds (15 consecutive ticks of 2s)
				if emptyCount >= 15 {
					logger.Info("[Server] All browser windows/tabs closed. Terminating SwitchBack background process.")
					fmt.Println("\n[INFO] All browser windows closed. SwitchBack stopped.")
					TriggerShutdown()
					return
				}
			} else {
				emptyCount = 0
			}
		}
	}
}

// StartServer launches the HTTP server and opens the browser.
func StartServer(port int, embeddedFS fs.FS) error {
	// 1. Check if an existing switchback instance is already running
	checkURL := fmt.Sprintf("http://127.0.0.1:%d/api/status", port)
	client := &http.Client{Timeout: 400 * time.Millisecond}
	if resp, err := client.Get(checkURL); err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Println("=======================================================")
			fmt.Printf(" [PIXEL UI] switchback is ALREADY running at: http://127.0.0.1:%d\n", port)
			fmt.Println(" Opening dashboard in your default browser...")
			fmt.Println("=======================================================")
			_ = win32.OpenURL(fmt.Sprintf("http://127.0.0.1:%d", port))
			time.Sleep(1200 * time.Millisecond)
			return nil
		}
	}

	mux := SetupRoutes(embeddedFS)

	// Bind to all interfaces (0.0.0.0) so mobile devices on LAN can connect
	var listener net.Listener
	var actualPort int = port
	for p := port; p < port+5; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p))
		if err == nil {
			listener = l
			actualPort = p
			break
		}
	}
	if listener == nil {
		return fmt.Errorf("unable to bind to ports %d-%d", port, port+4)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", actualPort)
	lanIP := GetPrimaryLANIP()
	mobileURL := fmt.Sprintf("http://%s:%d/mobile.html", lanIP, actualPort)

	fmt.Println("=======================================================")
	fmt.Printf(" [PIXEL UI] Dashboard running at:       %s\n", url)
	fmt.Printf(" [MOBILE]   Phone Controller URL:       %s\n", mobileURL)
	fmt.Println(" Keep this window open or minimize it.")
	fmt.Println(" Press Ctrl+C in this terminal window to stop.")
	fmt.Println("=======================================================")

	// Initialize Windows System Tray in background
	win32.StartTray(actualPort, func() {
		logger.Info("[Tray] User selected exit from system tray.")
		TriggerShutdown()
	})
	defer win32.StopTray()

	// Monitor client windows closing in background
	go runClientActivityMonitor()

	// Handle terminal shutdown signals (Ctrl+C, SIGTERM, SIGINT)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	server := &http.Server{Handler: mux}

	go func() {
		select {
		case <-sigChan:
			fmt.Println("\n[INFO] Termination signal received. Stopping SwitchBack...")
			logger.Info("[Server] Termination signal received. Exiting.")
		case <-shutdownSignalChan:
			logger.Info("[Server] Browser activity monitor requested graceful shutdown.")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		os.Exit(0)
	}()

	// Automatically open browser using Windows ShellExecuteW
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = win32.OpenURL(url)
	}()

	return server.Serve(listener)
}

// SetupRoutes constructs the HTTP ServeMux with all API endpoints and static UI assets.
func SetupRoutes(embeddedFS fs.FS) *http.ServeMux {
	mux := http.NewServeMux()

	// API Handlers
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/windows", handleWindows)
	mux.HandleFunc("/api/select-agent", handleSelectAgent)
	mux.HandleFunc("/api/select-work", handleSelectWork)
	mux.HandleFunc("/api/pick-work-switch", handlePickWorkSwitch)
	mux.HandleFunc("/api/pick-work-capture", handlePickWorkCapture)
	mux.HandleFunc("/api/toggle-mode", handleToggleMode)
	mux.HandleFunc("/api/toggle-auto-approve", handleToggleAutoApprove)
	mux.HandleFunc("/api/toggle-tray", handleToggleTray)
	mux.HandleFunc("/api/toggle-stopped", handleToggleStopped)
	mux.HandleFunc("/api/test-focus", handleTestFocus)
	mux.HandleFunc("/api/test-work", handleTestWork)
	mux.HandleFunc("/api/test-agent", handleTestAgent)
	mux.HandleFunc("/api/toggle-media", handleToggleMedia)
	mux.HandleFunc("/api/install", handleInstall)
	mux.HandleFunc("/api/uninstall-hooks", handleUninstallHooks)
	mux.HandleFunc("/api/logs", handleLogs)
	mux.HandleFunc("/api/shutdown", handleShutdown)
	mux.HandleFunc("/api/exit", handleShutdown)
	mux.HandleFunc("/api/heartbeat", handleHeartbeat)
	mux.HandleFunc("/api/disconnect", handleDisconnect)

	// Mobile Remote Controller Endpoints
	mux.HandleFunc("/api/mobile/info", handleMobileInfo)
	mux.HandleFunc("/api/mobile/toggle", handleMobileToggle)
	mux.HandleFunc("/api/mobile/status", handleMobileStatus)
	mux.HandleFunc("/api/mobile/reply", handleMobileReply)
	mux.HandleFunc("/api/mobile/prompt", handleMobilePrompt)
	mux.HandleFunc("/api/mobile/media", handleMobileMedia)
	mux.HandleFunc("/api/mobile/ask", handleMobileAsk)
	mux.HandleFunc("/api/mobile/output", handleMobileOutput)
	mux.HandleFunc("/api/mobile/execute", handleMobileExecute)

	// Static UI assets
	var fileServer http.Handler
	if _, err := os.Stat("ui/index.html"); err == nil {
		fileServer = http.FileServer(http.Dir("ui"))
	} else if sub, err := fs.Sub(embeddedFS, "ui"); err == nil {
		fileServer = http.FileServer(http.FS(sub))
	} else if embeddedFS != nil {
		fileServer = http.FileServer(http.FS(embeddedFS))
	}

	if fileServer != nil {
		mux.Handle("/", fileServer)
	}

	return mux
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("[Server] Shutdown requested from UI / API.")
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "SwitchBack server is shutting down...",
	})

	go func() {
		time.Sleep(200 * time.Millisecond)
		fmt.Println("\n[INFO] SwitchBack stopped by user.")
		os.Exit(0)
	}()
}

func handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client")
	if clientID != "" {
		RegisterClientHeartbeat(clientID)
	}
	writeJSON(w, map[string]interface{}{"status": "ok"})
}

func handleDisconnect(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client")
	if clientID != "" {
		DeregisterClient(clientID)
	}
	writeJSON(w, map[string]interface{}{"status": "ok"})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	fg := win32.GetForegroundWindow()
	title := win32.GetWindowTitle(fg)
	store, _ := state.Load()

	staleStatusMu.Lock()
	if store.SelectedAgentHWND != 0 {
		if !win32.IsWindowValid(win32.HWND(store.SelectedAgentHWND)) {
			staleAgentCount++
			if staleAgentCount >= 5 {
				store.SelectedAgentHWND = 0
				store.SelectedAgentTitle = ""
				_ = store.Save()
				staleAgentCount = 0
			}
		} else {
			staleAgentCount = 0
		}
	} else {
		staleAgentCount = 0
	}

	if store.SelectedWorkHWND != 0 {
		if !win32.IsWindowValid(win32.HWND(store.SelectedWorkHWND)) {
			staleWorkCount++
			if staleWorkCount >= 5 {
				store.SelectedWorkHWND = 0
				store.SelectedWorkTitle = ""
				_ = store.Save()
				staleWorkCount = 0
			}
		} else {
			staleWorkCount = 0
		}
	} else {
		staleWorkCount = 0
	}
	staleStatusMu.Unlock()

	cwd, _ := os.Getwd()
	agInstalled, claudeInstalled := installer.CheckHooksInstalled(cwd)

	resp := map[string]interface{}{
		"current_hwnd":            fmt.Sprintf("%d", fg),
		"current_title":           title,
		"selected_agent_hwnd":     store.SelectedAgentHWND,
		"selected_agent_title":    store.SelectedAgentTitle,
		"selected_agent_type":     store.SelectedAgentType,
		"selected_work_hwnd":      store.SelectedWorkHWND,
		"selected_work_title":     store.SelectedWorkTitle,
		"gaming_mode":             store.GamingMode,
		"auto_switch_enabled":     store.AutoSwitchEnabled,
		"fullscreen_guard":        store.FullscreenGuard,
		"meeting_guard":           store.MeetingGuard,
		"media_control":           store.MediaControl,
		"mobile_control_active":   store.MobileControlActive,
		"auto_approve_safe":       store.AutoApproveSafe,
		"keep_in_tray":            store.KeepInTray,
		"total_tasks_completed":   store.TotalTasksCompleted,
		"total_approvals_handled": store.TotalApprovalsHandled,
		"total_time_saved_secs":   store.TotalTimeSavedSecs,
		"pending_approval":        store.PendingApproval,
		"stopped":                 store.Stopped,
		"current_status":          store.CurrentStatus,
		"active_sessions":         len(store.Sessions),
		"antigravity_installed":   agInstalled,
		"claude_installed":        claudeInstalled,
	}

	writeJSON(w, resp)
}

func handleWindows(w http.ResponseWriter, r *http.Request) {
	windowsList := win32.GetOpenWindows()
	store, _ := state.Load()

	agents := []win32.WindowInfo{}
	workApps := []win32.WindowInfo{}

	for _, win := range windowsList {
		if win.IsAgent {
			agents = append(agents, win)
		}
		// Any window other than the current agent is an eligible work window
		if win.HWND != store.SelectedAgentHWND {
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

func handlePickWorkCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fg := win32.GetForegroundWindow()
	title := win32.GetWindowTitle(fg)

	if fg != 0 && win32.IsWindowValid(fg) {
		_ = state.SetSelectedWork(uintptr(fg), title)
		logger.Info("[Config] Captured Work Window via immediate capture: HWND %d (\"%s\")", fg, title)
		writeJSON(w, map[string]interface{}{
			"success": true,
			"hwnd":    fg,
			"title":   title,
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": false,
		"message": "Failed to capture window. Please make sure the window is focused.",
	})
}

func handlePickWorkSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Query().Get("immediate") == "true" {
		handlePickWorkCapture(w, r)
		return
	}

	if !pickWorkMu.TryLock() {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"message": "A window capture is already in progress. Please switch to your target window now.",
		})
		return
	}
	defer pickWorkMu.Unlock()

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
		GamingMode          bool `json:"gaming_mode"`
		AutoSwitchEnabled   bool `json:"auto_switch_enabled"`
		FullscreenGuard     bool `json:"fullscreen_guard"`
		MeetingGuard        bool `json:"meeting_guard"`
		MediaControl        bool `json:"media_control"`
		MobileControlActive bool `json:"mobile_control_active"`
		AutoApproveSafe     bool `json:"auto_approve_safe"`
		KeepInTray          bool `json:"keep_in_tray"`
		Stopped             bool `json:"stopped"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	_ = state.SetModes(req.GamingMode, req.AutoSwitchEnabled, req.FullscreenGuard, req.MeetingGuard, req.MediaControl, req.MobileControlActive, req.Stopped)
	_ = state.SetSafeAutoApprove(req.AutoApproveSafe)
	_ = state.SetKeepInTray(req.KeepInTray)
	logger.Info("[Config] Updated modes - GamingMode: %v, AutoSwitch: %v, FullscreenGuard: %v, MeetingGuard: %v, MediaControl: %v, MobileControlActive: %v, AutoApproveSafe: %v, KeepInTray: %v, Stopped: %v",
		req.GamingMode, req.AutoSwitchEnabled, req.FullscreenGuard, req.MeetingGuard, req.MediaControl, req.MobileControlActive, req.AutoApproveSafe, req.KeepInTray, req.Stopped)
	writeJSON(w, map[string]interface{}{"success": true})
}

func handleToggleAutoApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	val, err := state.ToggleAutoApproveSafe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logger.Info("[Config] AutoApproveSafe toggled: %v", val)
	writeJSON(w, map[string]interface{}{"success": true, "auto_approve_safe": val})
}

func handleToggleTray(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	val, err := state.ToggleKeepInTray()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logger.Info("[Config] KeepInTray toggled: %v", val)
	writeJSON(w, map[string]interface{}{"success": true, "keep_in_tray": val})
}

func handleToggleStopped(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stopped, err := state.ToggleStopped()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logger.Info("[Config] Master Stop toggled. Stopped: %v", stopped)
	writeJSON(w, map[string]interface{}{"success": true, "stopped": stopped})
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
			logger.Info("[Test] 1/3 Switched to Work/Video Window: HWND %d", store.SelectedWorkHWND)

			if store.MediaControl {
				time.Sleep(250 * time.Millisecond)
				win32.ToggleMediaPlayback(win32.HWND(store.SelectedWorkHWND))
				logger.Info("[Test] 2/3 Auto-resumed video/music playback")
			}
		}

		// Let video play for 4 seconds during simulation
		time.Sleep(4000 * time.Millisecond)

		// 2. AI Agent completes task
		if store.MediaControl && store.SelectedWorkHWND != 0 {
			win32.ToggleMediaPlayback(win32.HWND(store.SelectedWorkHWND))
			logger.Info("[Test] 3/3 Paused video/music playback before focus switch")
		}

		if store.GamingMode {
			logger.Info("[Test] Gaming Mode: Sending Toast Notification!")
			notify.SendToast("switchback // AI Agent Alert", "AI Agent finished its task!")
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

func handleTestWork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store, _ := state.Load()
	if store.SelectedWorkHWND == 0 || !win32.IsWindowValid(win32.HWND(store.SelectedWorkHWND)) {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"message": "No valid Work window selected. Please pick or select a target window first.",
		})
		return
	}

	_ = win32.SetForegroundWindowWithBypass(win32.HWND(store.SelectedWorkHWND))
	logger.Info("[Test] Step 1: Switched to Work Window HWND %d (\"%s\")", store.SelectedWorkHWND, store.SelectedWorkTitle)

	if store.MediaControl {
		time.Sleep(200 * time.Millisecond)
		win32.ToggleMediaPlayback(win32.HWND(store.SelectedWorkHWND))
		logger.Info("[Test] Step 1: Auto-resumed media playback on target window")
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Switched to Work Target (\"%s\") & resumed media!", store.SelectedWorkTitle),
	})
}

func handleTestAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store, _ := state.Load()
	if store.SelectedAgentHWND == 0 || !win32.IsWindowValid(win32.HWND(store.SelectedAgentHWND)) {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"message": "No valid Agent window selected. Please scan or select an AI agent window.",
		})
		return
	}

	if store.MediaControl && store.SelectedWorkHWND != 0 && win32.IsWindowValid(win32.HWND(store.SelectedWorkHWND)) {
		win32.ToggleMediaPlayback(win32.HWND(store.SelectedWorkHWND))
		logger.Info("[Test] Step 2: Auto-paused media playback before switching to Agent")
	}

	_ = win32.SetForegroundWindowWithBypass(win32.HWND(store.SelectedAgentHWND))
	logger.Info("[Test] Step 2: Switched back to Agent Window HWND %d (\"%s\")", store.SelectedAgentHWND, store.SelectedAgentTitle)

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Paused media & switched to Agent (\"%s\")!", store.SelectedAgentTitle),
	})
}

func handleToggleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store, _ := state.Load()
	targetHwnd := win32.HWND(store.SelectedWorkHWND)
	win32.ToggleMediaPlayback(targetHwnd)
	logger.Info("[Test] Triggered direct Media Play/Pause toggle on HWND %d", targetHwnd)
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Sent Play/Pause signal to media player!",
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

func handleUninstallHooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_ = installer.UninstallClaudeHooks()
	cwd, _ := os.Getwd()
	_ = installer.UninstallAntigravityHooks(cwd)

	// Also make sure stopped is set to true
	_ = state.SetStopped(true)

	logger.Info("[Hooks] All agent hooks removed and switchback paused.")
	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "All hooks removed from Claude Code and Antigravity! System restored to standard behavior.",
	})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	lines := logger.GetRecentLogs(25)
	writeJSON(w, lines)
}

// Mobile Control Handlers

func handleMobileInfo(w http.ResponseWriter, r *http.Request) {
	if !isRequestAuthorized(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	lanIP := GetPrimaryLANIP()
	allIPs := GetLocalIPs()
	store, _ := state.Load()

	// Parse port from host
	host := r.Host
	parts := strings.Split(host, ":")
	port := "48123"
	if len(parts) == 2 {
		port = parts[1]
	}

	mobileURL := fmt.Sprintf("http://%s:%s/mobile.html?token=%s", lanIP, port, store.SessionToken)

	writeJSON(w, map[string]interface{}{
		"lan_ip":                lanIP,
		"all_ips":               allIPs,
		"port":                  port,
		"mobile_url":            mobileURL,
		"session_token":         store.SessionToken,
		"mobile_control_active": store.MobileControlActive,
		"current_status":        store.CurrentStatus,
		"agent_title":           store.SelectedAgentTitle,
		"work_title":            store.SelectedWorkTitle,
	})
}

func handleMobileToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !isRequestAuthorized(r) {
		http.Error(w, "Unauthorized: missing or invalid session token", http.StatusUnauthorized)
		return
	}

	// Body can optionally specify {"active": bool}. If omitted/empty, it acts as a toggle.
	var req struct {
		Active *bool `json:"active"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	store, _ := state.Load()
	newState := !store.MobileControlActive
	if req.Active != nil {
		newState = *req.Active
	}

	_ = state.SetMobileControl(newState)
	logger.Info("[MobileControl] Mode toggled from mobile/web: %v (PC focus switching suppressed: %v)", newState, newState)

	writeJSON(w, map[string]interface{}{
		"success":               true,
		"mobile_control_active": newState,
		"message":               fmt.Sprintf("Mobile Control is now %v", newState),
	})
}

func handleMobileStatus(w http.ResponseWriter, r *http.Request) {
	store, _ := state.Load()

	writeJSON(w, map[string]interface{}{
		"mobile_control_active":   store.MobileControlActive,
		"current_status":          store.CurrentStatus,
		"agent_title":             store.SelectedAgentTitle,
		"work_title":              store.SelectedWorkTitle,
		"media_control":           store.MediaControl,
		"media_paused":            store.MediaPaused,
		"pending_approval":        store.PendingApproval,
		"auto_approve_safe":       store.AutoApproveSafe,
		"keep_in_tray":            store.KeepInTray,
		"total_tasks_completed":   store.TotalTasksCompleted,
		"total_approvals_handled": store.TotalApprovalsHandled,
		"total_time_saved_secs":   store.TotalTimeSavedSecs,
		"last_prompt":             store.LastMobilePrompt,
		"gaming_mode":             store.GamingMode,
		"fullscreen_guard":        store.FullscreenGuard,
	})
}

func handleMobileReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !isRequestAuthorized(r) {
		http.Error(w, "Unauthorized: missing or invalid session token", http.StatusUnauthorized)
		return
	}

	var req struct {
		ID       string `json:"id"`
		Decision string `json:"decision"` // "allow", "deny", "option"
		Answer   string `json:"answer"`
		Index    int    `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	store, _ := state.Load()
	isPending := store.PendingApproval != nil && (store.PendingApproval.ID == req.ID || req.ID == "" || req.ID == "test_clear")

	_ = state.SetApprovalReply(req.ID, req.Decision, req.Answer, req.Index)
	_ = state.RecordApprovalHandled()
	logger.Info("[MobileControl] Approval replied from phone: ID=%s, Decision=%s, OptionIndex=%d, Answer=\"%s\"", req.ID, req.Decision, req.Index, req.Answer)

	// Inject via Win32 keystrokes only if an approval prompt is active
	if isPending {
		targetHWND := win32.HWND(store.SelectedAgentHWND)
		if targetHWND == 0 || !win32.IsWindowValid(targetHWND) {
			found := win32.FindWindowByTitlePattern([]string{
				"Antigravity", "Claude", "Cursor", "Visual Studio Code", "Code -", "Windsurf",
			})
			if found != 0 {
				targetHWND = found
			}
		}

		if targetHWND != 0 && win32.IsWindowValid(targetHWND) {
			go func(hwnd win32.HWND, dec, ans string, idx int, mobPrim bool) {
				_ = win32.InjectApprovalToAgent(hwnd, dec, ans, idx, mobPrim)
			}(targetHWND, req.Decision, req.Answer, req.Index, store.MobileControlActive)
		}
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Response '%s' submitted to agent!", req.Decision),
	})
}

func handleMobilePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !isRequestAuthorized(r) {
		http.Error(w, "Unauthorized: missing or invalid session token", http.StatusUnauthorized)
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "Prompt cannot be empty", http.StatusBadRequest)
		return
	}

	cleanPrompt := strings.TrimSpace(req.Prompt)
	_ = state.SetLastMobilePrompt(cleanPrompt)
	logger.Info("[MobileControl] Received new prompt from phone: \"%s\"", cleanPrompt)

	_ = state.SetLatestAgentOutput(fmt.Sprintf("> User Prompt: %s\n\n⚡ Status: Prompt sent to Agent\nAgent is executing...", cleanPrompt))
	_ = state.SetCurrentStatus("agent_working")

	go func(prompt string) {
		if err := DispatchAgentPrompt(prompt); err != nil {
			logger.Error("[MobileControl] Failed to dispatch prompt: %v", err)
		}
	}(cleanPrompt)

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Prompt submitted directly to AI Agent!",
		"prompt":  cleanPrompt,
	})
}

func findAgentAPI() (string, []string) {
	home, _ := os.UserHomeDir()

	// 1. agentapi.bat in ~/.gemini/antigravity-ide/bin/
	batPath := filepath.Join(home, ".gemini", "antigravity-ide", "bin", "agentapi.bat")
	if _, err := os.Stat(batPath); err == nil {
		return batPath, nil
	}

	// 2. language_server_windows_x64.exe directly
	exeCandidates := []string{
		`d:\Apps\Antigravity\Antigravity IDE\resources\app\extensions\antigravity\bin\language_server_windows_x64.exe`,
		`c:\Apps\Antigravity\Antigravity IDE\resources\app\extensions\antigravity\bin\language_server_windows_x64.exe`,
		filepath.Join(home, `AppData\Local\Programs\Antigravity\resources\app\extensions\antigravity\bin\language_server_windows_x64.exe`),
		filepath.Join(home, `AppData\Local\Programs\Antigravity IDE\resources\app\extensions\antigravity\bin\language_server_windows_x64.exe`),
	}
	for _, c := range exeCandidates {
		if _, err := os.Stat(c); err == nil {
			return c, []string{"agentapi"}
		}
	}

	return "", nil
}

func resolveConversationID() string {
	// 1. Check state store
	if store, err := state.Load(); err == nil && store.ActiveConversationID != "" && store.ActiveConversationID != "default" {
		return store.ActiveConversationID
	}

	// 2. Check the active SQLite conversation DB in ~/.gemini/antigravity-ide/conversations/
	home, err := os.UserHomeDir()
	if err == nil {
		convDir := filepath.Join(home, ".gemini", "antigravity-ide", "conversations")
		if entries, err := os.ReadDir(convDir); err == nil {
			var newestID string
			var newestTime time.Time
			for _, entry := range entries {
				name := entry.Name()
				if strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".db") {
					info, err := entry.Info()
					if err == nil && info.ModTime().After(newestTime) {
						newestTime = info.ModTime()
						id := strings.TrimSuffix(name, ".db-wal")
						id = strings.TrimSuffix(id, ".db")
						if isValidUUID(id) {
							newestID = id
						}
					}
				}
			}
			if newestID != "" {
				return newestID
			}
		}
	}

	return ""
}

func DispatchAgentPrompt(prompt string) error {
	convID := resolveConversationID()
	prog, prefixArgs := findAgentAPI()

	store, _ := state.Load()
	targetHWND := win32.HWND(store.SelectedAgentHWND)
	if targetHWND == 0 || !win32.IsWindowValid(targetHWND) {
		targetHWND = win32.FindWindowByTitlePattern([]string{"Antigravity", "Claude", "Cursor", "Windsurf", "Visual Studio Code", "Terminal"})
		if targetHWND != 0 {
			_ = state.SetSelectedAgent(uintptr(targetHWND), win32.GetWindowTitle(targetHWND), "auto-detected")
		}
	}
	if targetHWND != 0 && win32.IsWindowValid(targetHWND) {
		_ = win32.SetForegroundWindowWithBypass(targetHWND)
	}

	agentType := strings.ToLower(store.SelectedAgentType)
	agentTitle := strings.ToLower(store.SelectedAgentTitle)
	isAntigravity := agentType == "antigravity" || strings.Contains(agentTitle, "antigravity") || (agentType == "" && strings.Contains(agentTitle, "antigravity"))

	// 1. Antigravity IDE: Native Language Server IPC (agentapi)
	if isAntigravity && convID != "" && prog != "" {
		args := append([]string{}, prefixArgs...)
		args = append(args, "send-message", "--title=📱 Mobile Remote Command", convID, prompt)

		cmd := exec.Command(prog, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf

		err := cmd.Run()
		if err == nil {
			logger.Info("[MobileControl] Successfully delivered prompt via native AgentAPI to conversation %s", convID)
			return nil
		}
		logger.Warn("[MobileControl] AgentAPI send-message returned %v: %s", err, outBuf.String())
	}

	// 2. Claude Code: terminal window injection or headless CLI
	if agentType == "claude-code" || strings.Contains(agentTitle, "claude") {
		if targetHWND != 0 && win32.IsWindowValid(targetHWND) {
			logger.Info("[MobileControl] Dispatching prompt to Claude Code window HWND %d...", targetHWND)
			return win32.InjectTextToAgent(targetHWND, prompt, store.MobileControlActive)
		}
		if claudePath, err := exec.LookPath("claude"); err == nil {
			logger.Info("[MobileControl] Dispatching to headless Claude Code CLI: %s", claudePath)
			cmd := exec.Command(claudePath, "-p", prompt)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			cwd, _ := os.Getwd()
			cmd.Dir = cwd
			return cmd.Start()
		}
	}

	// 3. Cursor, Windsurf, VS Code (Cline/Roo/Copilot), JetBrains, Aider: Direct window injection
	if targetHWND != 0 && win32.IsWindowValid(targetHWND) {
		logger.Info("[MobileControl] Dispatching prompt to %s window HWND %d...", store.SelectedAgentType, targetHWND)
		return win32.InjectTextToAgent(targetHWND, prompt, store.MobileControlActive)
	}

	// 4. Fallback: Native AgentAPI if available
	if convID != "" && prog != "" {
		args := append([]string{}, prefixArgs...)
		args = append(args, "send-message", "--title=📱 Mobile Remote Command", convID, prompt)

		cmd := exec.Command(prog, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("no agent dispatch mechanism available (AgentAPI, Claude CLI, or valid HWND)")
}

func handleMobileOutput(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if !isRequestAuthorized(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			Output string `json:"output"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Output != "" {
			_ = state.SetLatestAgentOutput(req.Output)
		}
		writeJSON(w, map[string]interface{}{"success": true})
		return
	}

	store, _ := state.Load()
	output, _ := state.GetLatestAgentOutput()
	output = strings.TrimSpace(output)
	if output == "" {
		output = "Ready. The agent's latest response will appear here."
	}

	writeJSON(w, map[string]interface{}{
		"output":         output,
		"current_status": store.CurrentStatus,
		"agent_title":    store.SelectedAgentTitle,
	})
}

func handleMobileMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !isRequestAuthorized(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	store, _ := state.Load()
	targetHwnd := win32.HWND(store.SelectedWorkHWND)
	win32.ToggleMediaPlayback(targetHwnd)
	logger.Info("[MobileControl] Toggled PC media playback from mobile on HWND %d", targetHwnd)

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Toggled PC Media Playback",
	})
}

func handleMobileAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !isRequestAuthorized(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		Title   string   `json:"title"`
		Message string   `json:"message"`
		Type    string   `json:"type"`
		Options []string `json:"options"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.Title == "" {
		req.Title = "Agent Approval / Question"
	}
	if req.Message == "" {
		req.Message = "The AI Agent is requesting your input on this action."
	}
	if len(req.Options) == 0 {
		req.Options = []string{"Yes, proceed with action", "No, cancel"}
	}

	approval := &state.PendingApprovalData{
		ID:        fmt.Sprintf("q_%d", time.Now().UnixNano()),
		Title:     req.Title,
		Message:   req.Message,
		Type:      req.Type,
		Options:   req.Options,
		CreatedAt: time.Now(),
	}

	_ = state.SetPendingApproval(approval)
	logger.Info("[MobileControl] Pushed question/approval to mobile: %s (\"%s\")", approval.Title, approval.Message)

	writeJSON(w, map[string]interface{}{
		"success":  true,
		"approval": approval,
	})
}

func handleMobileExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !isRequestAuthorized(r) {
		http.Error(w, "Unauthorized: missing or invalid session token", http.StatusUnauthorized)
		return
	}

	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Command) == "" {
		http.Error(w, "Command cannot be empty", http.StatusBadRequest)
		return
	}

	cmdStr := strings.TrimSpace(req.Command)
	logger.Info("[TaskRunner] Running command from mobile: %s", cmdStr)
	_ = state.SetCurrentStatus("agent_working")
	_ = state.SetLatestAgentOutput(fmt.Sprintf("⚡ Running: %s\nExecuting in workspace...\n", cmdStr))

	go func(command string) {
		cwd, _ := os.Getwd()
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", command)
		cmd.Dir = cwd

		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf

		err := cmd.Run()
		output := strings.TrimSpace(outBuf.String())
		if output == "" {
			if err != nil {
				output = fmt.Sprintf("Command failed: %v", err)
			} else {
				output = "Command executed successfully (no output returned)."
			}
		}

		statusHeader := "✔ Success"
		if err != nil {
			statusHeader = fmt.Sprintf("✖ Failed (exit code: %v)", err)
		}

		formatted := fmt.Sprintf("⚡ Command: %s\nStatus: %s\n\n--- Output ---\n%s", command, statusHeader, output)
		_ = state.SetLatestAgentOutput(formatted)
		_ = state.SetCurrentStatus("completed")
		logger.Info("[TaskRunner] Finished execution of: %s", command)
	}(cmdStr)

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Execution started: %s", cmdStr),
		"command": cmdStr,
	})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
