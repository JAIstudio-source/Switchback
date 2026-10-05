package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"switchback/internal/config"
	"switchback/internal/installer"
	"switchback/internal/logger"
	"switchback/internal/notify"
	"switchback/internal/server"
	"switchback/internal/state"
	"switchback/internal/win32"
)

//go:embed ui/*
var embeddedUI embed.FS

var version = "1.2.0"

func main() {
	if len(os.Args) < 2 {
		// Default when double-clicked from Windows Explorer: launch the Pixel UI dashboard!
		runUI()
		return
	}

	logger.Info("[Command] switchback invoked with args: %v", os.Args[1:])

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "ui", "gui", "dashboard":
		runUI()
	case "hook":
		handleHook(args)
	case "save-and-focus":
		// Backward compatible alias for Stop hook
		handleHook(append([]string{"--event", "Stop"}, args...))
	case "restore":
		// Backward compatible alias for UserPromptSubmit hook
		handleHook(append([]string{"--event", "UserPromptSubmit"}, args...))
	case "status":
		handleStatus()
	case "install":
		handleInstall()
	case "uninstall", "remove-hooks", "unhook":
		handleUninstall()
	case "stop", "pause", "disable":
		handleStop()
	case "resume", "enable", "start":
		handleResume()
	case "exit", "kill", "quit", "shutdown", "stop-app":
		handleKillRunning()
	case "test-focus":
		handleTestFocus()
	case "test-switch-work":
		handleTestSwitchWork()
	case "test-switch-agent":
		handleTestSwitchAgent()
	case "set-output":
		if len(args) < 1 {
			fmt.Println("Usage: switchback set-output \"<output text>\"")
			os.Exit(1)
		}
		outText := strings.Join(args, " ")
		_ = state.SetLatestAgentOutput(outText)
		fmt.Println("[+] Mobile output updated successfully.")
	case "version", "--version", "-v":
		fmt.Printf("switchback version %s (Windows x64)\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		printUsage()
	}
}

func handleKillRunning() {
	client := &http.Client{Timeout: 400 * time.Millisecond}
	for p := 48123; p <= 48127; p++ {
		_, _ = client.Post(fmt.Sprintf("http://127.0.0.1:%d/api/shutdown", p), "application/json", nil)
	}
	time.Sleep(300 * time.Millisecond)

	currentPid := os.Getpid()
	killScript := fmt.Sprintf(`Get-Process -Name "switchback" -ErrorAction SilentlyContinue | Where-Object { $_.Id -ne %d } | Stop-Process -Force -ErrorAction SilentlyContinue`, currentPid)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", killScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
	fmt.Println("[OK] All SwitchBack processes stopped.")
}

func runUI() {
	port := 48123
	if err := server.StartServer(port, embeddedUI); err != nil {
		fmt.Printf("\n[ERROR] Failed to launch UI server: %v\n", err)
		fmt.Println("\nPress Enter to close...")
		var dummy string
		fmt.Scanln(&dummy)
	}
}

func printUsage() {
	fmt.Println(`switchback - Auto-Focus Window Manager for AI Agents

Usage:
  switchback ui                                    (Launch Pixel UI dashboard in browser)
  switchback hook --event <Stop|UserPromptSubmit|PreInvocation|Notification>
  switchback status                                (Show active windows, hooks & configuration)
  switchback install                               (Configure hooks in Claude Code and Antigravity)
  switchback uninstall                             (Remove all hooks and restore system to normal)
  switchback stop                                  (Pause/disable auto-focusing immediately)
  switchback resume                                (Resume auto-focusing)
  switchback test-focus                            (Interactive focus test)
  switchback version`)
}

func handleHook(args []string) {
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	event := fs.String("event", "Stop", "Hook event")
	agentType := fs.String("agent", "auto", "Agent type")
	sessionID := fs.String("session", "default", "Session ID")
	_ = fs.Parse(args)

	cleanSession := strings.TrimSpace(*sessionID)
	if cleanSession == "" || cleanSession == "$CLAUDE_SESSION_ID" || cleanSession == "%conversationId%" {
		cleanSession = "default"
	}

	var rawStdinBytes []byte
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		readDone := make(chan []byte, 1)
		go func() {
			b, _ := io.ReadAll(os.Stdin)
			readDone <- b
		}()
		select {
		case b := <-readDone:
			rawStdinBytes = b
		case <-time.After(150 * time.Millisecond):
			// stdin didn't close promptly, continue without hanging
		}
	}

	toolName, toolInput, responseText, parsedSession := parseHookStdin(rawStdinBytes)
	if parsedSession != "" && cleanSession == "default" {
		cleanSession = parsedSession
	}

	logger.Info("[Hook] Event: %s, Agent: %s, Session: %s, Tool: %s (RawBytes: %d)", *event, *agentType, cleanSession, toolName, len(rawStdinBytes))

	store, err := state.Load()
	if err != nil {
		fmt.Println("{}")
		os.Exit(0)
	}

	if store.Stopped {
		logger.Info("[Hook] Master stop active. Ignoring event '%s'.", *event)
		if *event == "Notification" || *event == "PreToolUse" {
			fmt.Println(`{"decision":"allow"}`)
		} else {
			fmt.Println("{}")
		}
		os.Exit(0)
	}

	// Debounce duplicate hook invocations within 600ms (e.g. workspace + global hooks fired at same millisecond)
	now := time.Now()
	if store.LastHookEvent == *event && now.Sub(store.LastHookTime) < 600*time.Millisecond {
		logger.Info("[Hook] Duplicate event '%s' debounced (ignored within %v)", *event, now.Sub(store.LastHookTime))
		if *event == "Notification" || *event == "PreToolUse" {
			fmt.Println(`{"decision":"allow"}`)
		} else {
			fmt.Println("{}")
		}
		os.Exit(0)
	}
	store.LastHookEvent = *event
	store.LastHookTime = now
	if cleanSession != "" && cleanSession != "default" {
		store.ActiveConversationID = cleanSession
	}
	_ = store.Save()

	// Helper to resolve the AI agent window
	resolveAgentHWND := func() win32.HWND {
		// 1. Highest priority: The actual IDE/terminal ancestor process that spawned this hook
		agentHWND := win32.FindAncestorWindow()
		if agentHWND != 0 && win32.IsWindowValid(agentHWND) {
			_ = state.SetSelectedAgent(uintptr(agentHWND), win32.GetWindowTitle(agentHWND), *agentType)
			return agentHWND
		}

		// 2. Second priority: Explicitly selected agent window from UI
		if store.SelectedAgentHWND != 0 && win32.IsWindowValid(win32.HWND(store.SelectedAgentHWND)) {
			return win32.HWND(store.SelectedAgentHWND)
		}

		// 3. Fallback: Search visible windows for known agent patterns (including user-configured patterns)
		patterns := []string{"Antigravity", "Claude", "Visual Studio Code", "Cursor", "Windsurf", "Codex", "Terminal"}
		cfg := config.LoadConfig()
		if agentCfg, ok := cfg.Agents[*agentType]; ok && len(agentCfg.TitlePatterns) > 0 {
			patterns = append(agentCfg.TitlePatterns, patterns...)
		}
		found := win32.FindWindowByTitlePattern(patterns)
		if found != 0 && win32.IsWindowValid(found) {
			_ = state.SetSelectedAgent(uintptr(found), win32.GetWindowTitle(found), *agentType)
			return found
		}

		return 0
	}

	// Helper to dynamically resolve the user's work window
	resolveWorkHWND := func(agentHWND win32.HWND) win32.HWND {
		workHWND := win32.HWND(0)
		// Check session saved HWND first
		if sess, ok := store.Sessions[cleanSession]; ok && sess.SavedHWND != 0 && win32.IsWindowValid(win32.HWND(sess.SavedHWND)) && win32.HWND(sess.SavedHWND) != agentHWND {
			workHWND = win32.HWND(sess.SavedHWND)
		}
		// Fallback to store.SelectedWorkHWND
		if (workHWND == 0 || !win32.IsWindowValid(workHWND) || workHWND == agentHWND) && store.SelectedWorkHWND != 0 {
			if win32.IsWindowValid(win32.HWND(store.SelectedWorkHWND)) && win32.HWND(store.SelectedWorkHWND) != agentHWND {
				workHWND = win32.HWND(store.SelectedWorkHWND)
			}
		}

		// Fallback to searching by title pattern if HWND became stale
		if (workHWND == 0 || !win32.IsWindowValid(workHWND) || workHWND == agentHWND) && store.SelectedWorkTitle != "" {
			parts := strings.Split(store.SelectedWorkTitle, " - ")
			searchPatterns := []string{store.SelectedWorkTitle}
			for _, part := range parts {
				p := strings.TrimSpace(part)
				if len(p) >= 3 {
					searchPatterns = append(searchPatterns, p)
				}
			}
			found := win32.FindWindowByTitlePattern(searchPatterns)
			if found != 0 && found != agentHWND && win32.IsWindowValid(found) {
				workHWND = found
				_ = state.SetSelectedWork(uintptr(found), win32.GetWindowTitle(found))
			}
		}

		// Fallback to open windows (prioritizing non-agent apps, but accepting any window other than agent)
		if workHWND == 0 || !win32.IsWindowValid(workHWND) || workHWND == agentHWND {
			openWindows := win32.GetOpenWindows()
			for _, win := range openWindows {
				if !win.IsAgent && win.HWND != uintptr(agentHWND) && win.Title != "PopupHost" && len(win.Title) >= 3 && win32.IsWindowVisible(win32.HWND(win.HWND)) {
					workHWND = win32.HWND(win.HWND)
					_ = state.SetSelectedWork(win.HWND, win.Title)
					break
				}
			}
			if workHWND == 0 || !win32.IsWindowValid(workHWND) || workHWND == agentHWND {
				for _, win := range openWindows {
					if win.HWND != uintptr(agentHWND) && win.Title != "PopupHost" && len(win.Title) >= 3 && win32.IsWindowVisible(win32.HWND(win.HWND)) {
						workHWND = win32.HWND(win.HWND)
						_ = state.SetSelectedWork(win.HWND, win.Title)
						break
					}
				}
			}
		}

		return workHWND
	}

	switch *event {
	case "UserPromptSubmit", "PreInvocation":
		// Record stats for task start
		_ = state.RecordTaskStart()

		// 1. User submitted task in agent -> Switch focus to the user's Work Window!
		agentHWND := resolveAgentHWND()
		workHWND := resolveWorkHWND(agentHWND)

		// Mobile Control Guard: If Mobile is primary, suppress PC window switching!
		if store.MobileControlActive {
			logger.Info("[MobileControl] Task started. Suppressing PC window focus switch (Mobile Primary Active).")
			store.CurrentStatus = "agent_working"
			_ = store.Save()
			if *event == "Notification" || *event == "PreToolUse" {
				fmt.Println(`{"decision":"allow"}`)
			} else {
				fmt.Println("{}")
			}
			os.Exit(0)
		}

		currentFG := win32.GetForegroundWindow()
		isUserAtAgent := (currentFG != 0 && (currentFG == agentHWND || agentHWND == 0))
		isInitialStart := (store.CurrentStatus != "agent_working") || isUserAtAgent

		if isInitialStart && workHWND != 0 && workHWND != agentHWND && win32.IsWindowValid(workHWND) {
			logger.Info("[AutoSwitch] Task submitted (%s). Auto-switching to work window: HWND %d (%s)",
				*event, workHWND, win32.GetWindowTitle(workHWND))
			_ = win32.SetForegroundWindowWithBypass(workHWND)

			// If media control is enabled, auto-resume playing media upon switching back to work window
			if store.MediaControl {
				logger.Info("[Media] Auto-resuming media playback upon user prompt submission")
				time.Sleep(200 * time.Millisecond)
				win32.ToggleMediaPlayback(workHWND)
				store.MediaPaused = false
			}
		}

		store.CurrentStatus = "agent_working"
		_ = store.Save()
		fmt.Println("{}")
		os.Exit(0)

	case "PostToolUse":
		logger.Info("[Hook] PostToolUse event. Clearing pending approval and resetting working state.")
		store.PendingApproval = nil
		if store.CurrentStatus == "permission_needed" {
			store.CurrentStatus = "agent_working"
		}
		_ = store.Save()
		fmt.Println("{}")
		os.Exit(0)

	case "Stop", "PostInvocation":
		// Record stats for task completion & time saved
		_ = state.RecordTaskEnd()
		win32.UpdateTrayStatus("Idle (Task Completed)")

		// Capture clean agent response into latest output for mobile controller
		respText := strings.TrimSpace(responseText)
		if respText != "" {
			store.LatestAgentOutput = respText
		}
		store.PendingApproval = nil

		// 2. Agent finished task -> Save user's current work window & switch to Agent window!
		agentHWND := resolveAgentHWND()
		currentFG := win32.GetForegroundWindow()

		if currentFG != 0 && currentFG != agentHWND && win32.IsWindowValid(currentFG) {
			fgTitle := win32.GetWindowTitle(currentFG)
			_ = state.SetSelectedWork(uintptr(currentFG), fgTitle)
			_ = state.SaveSession(cleanSession, state.SessionState{
				SavedHWND:  uintptr(currentFG),
				SavedTitle: fgTitle,
				AgentHWND:  uintptr(agentHWND),
				AgentType:  *agentType,
				State:      "waiting_user",
			})
		}
		workHWND := resolveWorkHWND(agentHWND)

		// Mobile Control Guard: If Mobile is primary, suppress PC window switching!
		if store.MobileControlActive {
			logger.Info("[MobileControl] Agent completed task. Suppressing PC window focus switch (Mobile Primary Active).")
			notify.SendToast("switchback // Mobile Alert", "AI Agent finished task! (Check mobile controller)")
			store.CurrentStatus = "completed"
			_ = store.Save()
			fmt.Println("{}")
			os.Exit(0)
		}

		if store.GamingMode {
			logger.Info("[GamingMode] Agent finished. Sent notification toast.")
			notify.SendToast("switchback // AI Agent Alert", "AI Agent finished its task!")
			store.CurrentStatus = "completed"
			_ = store.Save()
			fmt.Println("{}")
			os.Exit(0)
		}

		// Fullscreen Guard: If user is watching fullscreen video or playing game, do not steal focus!
		if store.FullscreenGuard && win32.IsFullscreenActive() {
			logger.Info("[FullscreenGuard] Fullscreen mode detected! Suppressing focus steal, sent desktop notification toast.")
			notify.SendToast("switchback // AI Agent Alert", "AI Agent finished its task! (Fullscreen mode detected)")
			store.CurrentStatus = "completed"
			_ = store.Save()
			fmt.Println("{}")
			os.Exit(0)
		}

		// Meeting Guard: If user is in a Zoom/Discord/Teams call, suppress focus steal!
		if store.MeetingGuard && win32.IsMeetingActive() {
			logger.Info("[MeetingGuard] Meeting/Voice Call detected! Suppressing focus steal, sent desktop notification toast.")
			notify.SendToast("switchback // AI Agent Alert", "AI Agent finished its task! (Meeting in progress)")
			store.CurrentStatus = "completed"
			_ = store.Save()
			fmt.Println("{}")
			os.Exit(0)
		}

		// If media control is enabled, pause media before switching to agent
		if store.MediaControl {
			logger.Info("[Media] Pausing media playback upon agent task completion")
			win32.ToggleMediaPlayback(workHWND)
			time.Sleep(120 * time.Millisecond)
			store.MediaPaused = true
		}

		// Switch to agent window
		if store.AutoSwitchEnabled && agentHWND != 0 && win32.IsWindowValid(agentHWND) {
			logger.Info("[AutoSwitch] Agent task complete. Auto-switching to agent window: HWND %d (%s)",
				agentHWND, win32.GetWindowTitle(agentHWND))
			_ = win32.SetForegroundWindowWithBypass(agentHWND)
		}
		store.CurrentStatus = "completed"
		_ = store.Save()
		fmt.Println("{}")
		os.Exit(0)

	case "PreToolUse":
		// Normal agent tool execution: update status, clear pending approval, and always allow
		if store.CurrentStatus != "agent_working" {
			store.CurrentStatus = "agent_working"
		}
		if store.PendingApproval != nil {
			store.PendingApproval = nil
		}
		_ = store.Save()
		fmt.Println(`{"decision":"allow"}`)
		os.Exit(0)

	case "Notification":
		// Agent explicitly asks a question or requires user input
		isQuestionTool := toolName == "ask_question" || toolName == "ask_user" || toolName == "ask"
		if !isQuestionTool {
			fmt.Println("{}")
			os.Exit(0)
		}

		agentHWND := resolveAgentHWND()
		currentFG := win32.GetForegroundWindow()
		workHWND := resolveWorkHWND(agentHWND)

		if currentFG != 0 && currentFG != agentHWND && win32.IsWindowValid(currentFG) {
			fgTitle := win32.GetWindowTitle(currentFG)
			workHWND = currentFG
			_ = state.SetSelectedWork(uintptr(currentFG), fgTitle)
			_ = state.SaveSession(cleanSession, state.SessionState{
				SavedHWND:  uintptr(currentFG),
				SavedTitle: fgTitle,
				AgentHWND:  uintptr(agentHWND),
				AgentType:  *agentType,
				State:      "waiting_user",
			})
		}

		// Check for Smart Auto-Approve of safe actions
		if store.AutoApproveSafe && isSafeAction(toolName, toolInput) {
			logger.Info("[AutoApprove] Safe action '%s' auto-approved.", toolName)
			_ = state.RecordApprovalHandled()
			fmt.Println(`{"decision":"deny","reason":"Auto-approved safe action: Proceed"}`)
			os.Exit(0)
		}

		win32.UpdateTrayStatus("Waiting for User Response")

		// Parse question data
		questionTitle, questionMsg, options, toolCleanName, snippet, dangerLevel := parseApprovalData(toolName, toolInput, responseText)
		reqID := fmt.Sprintf("req_%d", time.Now().UnixNano())
		store.PendingApproval = &state.PendingApprovalData{
			ID:             reqID,
			Title:          questionTitle,
			Message:        questionMsg,
			Type:           "confirm",
			Options:        options,
			ToolName:       toolCleanName,
			ContextSnippet: snippet,
			DangerLevel:    dangerLevel,
			CreatedAt:      time.Now(),
		}
		store.CurrentStatus = "permission_needed"
		_ = store.Save()

		// If Mobile Control is active, wait up to 30s for phone response
		if store.MobileControlActive {
			logger.Info("[MobileControl] Waiting for mobile approval response (Request ID: %s)...", reqID)
			startTime := time.Now()
			pollInterval := 100 * time.Millisecond
			for time.Since(startTime) < 30*time.Second {
				time.Sleep(pollInterval)
				if pollInterval < 500*time.Millisecond {
					pollInterval += 50 * time.Millisecond
				}
				s, err := state.Load()
				if err == nil && s.LastApprovalReply != nil && s.LastApprovalReply.ID == reqID {
					reply := s.LastApprovalReply
					_ = state.RecordApprovalHandled()
					if reply.Decision == "allow" {
						fmt.Printf("{\"decision\":\"allow\"}\n")
					} else if reply.Decision == "deny" {
						fmt.Printf("{\"decision\":\"deny\",\"reason\":\"User cancelled action from mobile remote controller.\"}\n")
					} else if reply.Index > 0 {
						fmt.Printf("{\"decision\":\"deny\",\"reason\":\"User selected from mobile remote controller: Option %d: %s\"}\n", reply.Index, reply.Answer)
					} else {
						fmt.Printf("{\"decision\":\"allow\"}\n")
					}
					os.Exit(0)
				}
			}
		}

		// Fallbacks for Gaming, Fullscreen, and Meeting modes
		if store.GamingMode {
			notify.SendToast("switchback // AI Agent Alert", fmt.Sprintf("Agent asks: %s", questionTitle))
			fmt.Println("{}")
			os.Exit(0)
		}
		if store.FullscreenGuard && win32.IsFullscreenActive() {
			notify.SendToast("switchback // AI Agent Alert", fmt.Sprintf("Agent asks: %s (Fullscreen detected)", questionTitle))
			fmt.Println("{}")
			os.Exit(0)
		}
		if store.MeetingGuard && win32.IsMeetingActive() {
			notify.SendToast("switchback // AI Agent Alert", fmt.Sprintf("Agent asks: %s (Meeting in progress)", questionTitle))
			fmt.Println("{}")
			os.Exit(0)
		}

		if store.MediaControl {
			logger.Info("[Media] Pausing media playback upon agent question prompt")
			win32.ToggleMediaPlayback(workHWND)
			time.Sleep(120 * time.Millisecond)
			store.MediaPaused = true
			_ = store.Save()
		}

		if agentHWND != 0 && win32.IsWindowValid(agentHWND) {
			logger.Info("[Notification] Agent requires user input. Auto-switching to agent window: HWND %d (%s)",
				agentHWND, win32.GetWindowTitle(agentHWND))
			_ = win32.SetForegroundWindowWithBypass(agentHWND)
		}

		fmt.Println("{}")
		os.Exit(0)
	}

	fmt.Println("{}")
	os.Exit(0)
}

func handleStatus() {
	fg := win32.GetForegroundWindow()
	title := win32.GetWindowTitle(fg)
	fmt.Printf("Current Foreground Window: HWND %d (\"%s\")\n", fg, title)

	store, err := state.Load()
	if err == nil {
		fmt.Printf("Selected Agent Window: HWND %d (\"%s\")\n", store.SelectedAgentHWND, store.SelectedAgentTitle)
		fmt.Printf("Selected Work Window:  HWND %d (\"%s\")\n", store.SelectedWorkHWND, store.SelectedWorkTitle)
		fmt.Printf("Gaming Mode (Notifications Only): %v\n", store.GamingMode)
		fmt.Printf("Auto-Switch Enabled:              %v\n", store.AutoSwitchEnabled)
		fmt.Printf("Fullscreen Guard (Video/Game):    %v\n", store.FullscreenGuard)
		fmt.Printf("Media Control (Auto-Pause/Play):  %v\n", store.MediaControl)
		stoppedText := "RUNNING / ACTIVE"
		if store.Stopped {
			stoppedText = "STOPPED / PAUSED"
		}
		fmt.Printf("Focus Manager Master Status:      %s\n", stoppedText)
		fmt.Printf("Current Agent Workflow Status:    %s\n", store.CurrentStatus)
	}

	cwd, _ := os.Getwd()
	agInstalled, claudeInstalled := installer.CheckHooksInstalled(cwd)
	fmt.Printf("Hooks Status (Antigravity):        %v\n", agInstalled)
	fmt.Printf("Hooks Status (Claude Code):        %v\n", claudeInstalled)

	statePath, _ := state.GetStateFilePath()
	fmt.Printf("State File: %s\n", statePath)
}

func handleInstall() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path: %v\n", err)
		return
	}
	exePath, _ = filepath.Abs(exePath)

	fmt.Printf("Installing switchback hooks using binary: %s\n", exePath)

	if err := installer.InstallClaudeHooks(exePath); err != nil {
		fmt.Printf("[-] Failed to configure Claude Code hooks: %v\n", err)
	} else {
		fmt.Println("[+] Claude Code hooks configured successfully (~/.claude/settings.json)")
	}

	cwd, _ := os.Getwd()
	if err := installer.InstallAntigravityHooks(exePath, cwd); err != nil {
		fmt.Printf("[-] Failed to configure Antigravity hooks: %v\n", err)
	} else {
		fmt.Println("[+] Antigravity hooks configured successfully (hooks.json)")
	}

	_ = state.SetStopped(false)
	fmt.Println("[+] switchback is now ACTIVE.")
}

func handleUninstall() {
	fmt.Println("Removing switchback hooks from all agent configuration files...")
	if err := installer.UninstallClaudeHooks(); err != nil {
		fmt.Printf("[-] Failed to remove Claude Code hooks: %v\n", err)
	} else {
		fmt.Println("[+] Claude Code hooks removed (~/.claude/settings.json)")
	}

	cwd, _ := os.Getwd()
	if err := installer.UninstallAntigravityHooks(cwd); err != nil {
		fmt.Printf("[-] Failed to remove Antigravity hooks: %v\n", err)
	} else {
		fmt.Println("[+] Antigravity hooks removed (hooks.json)")
	}

	_ = state.SetStopped(true)
	fmt.Println("[+] switchback is STOPPED and all hooks have been removed. System returned to default behavior.")
}

func handleStop() {
	if err := state.SetStopped(true); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to persist state: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[+] switchback is now PAUSED / STOPPED. All auto-switching is disabled.")
}

func handleResume() {
	if err := state.SetStopped(false); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to persist state: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[+] switchback is now RESUMED / ACTIVE. Auto-switching is enabled.")
}

func handleTestFocus() {
	fmt.Println("--- switchback Focus Switch Test ---")
	store, _ := state.Load()

	agentHWND := win32.HWND(store.SelectedAgentHWND)
	if agentHWND == 0 || !win32.IsWindowValid(agentHWND) {
		agentHWND = win32.FindAncestorWindow()
		if agentHWND == 0 {
			agentHWND = win32.FindWindowByTitlePattern([]string{"Antigravity", "Claude", "Visual Studio Code", "Cursor", "Codex", "Terminal"})
		}
	}

	workHWND := win32.HWND(store.SelectedWorkHWND)
	if workHWND == 0 || !win32.IsWindowValid(workHWND) {
		if store.SelectedWorkTitle != "" {
			parts := strings.Split(store.SelectedWorkTitle, " - ")
			patterns := []string{store.SelectedWorkTitle}
			for _, part := range parts {
				p := strings.TrimSpace(part)
				if len(p) >= 3 {
					patterns = append(patterns, p)
				}
			}
			workHWND = win32.FindWindowByTitlePattern(patterns)
		}
		if workHWND == 0 || !win32.IsWindowValid(workHWND) {
			for _, win := range win32.GetOpenWindows() {
				if !win.IsAgent && win.HWND != uintptr(agentHWND) {
					workHWND = win32.HWND(win.HWND)
					break
				}
			}
			if workHWND == 0 || !win32.IsWindowValid(workHWND) {
				for _, win := range win32.GetOpenWindows() {
					if win.HWND != uintptr(agentHWND) {
						workHWND = win32.HWND(win.HWND)
						break
					}
				}
			}
		}
		if workHWND != 0 {
			_ = state.SetSelectedWork(uintptr(workHWND), win32.GetWindowTitle(workHWND))
			store, _ = state.Load()
		}
	}

	fmt.Printf("Selected Agent: HWND %d (\"%s\")\n", agentHWND, win32.GetWindowTitle(agentHWND))
	fmt.Printf("Selected Work:  HWND %d (\"%s\")\n", workHWND, win32.GetWindowTitle(workHWND))
	fmt.Printf("Media Control:  %v\n", store.MediaControl)
	fmt.Printf("Gaming Mode:    %v\n\n", store.GamingMode)

	if workHWND != 0 && win32.IsWindowValid(workHWND) {
		fmt.Printf("1. Switching to Work/Video window (HWND %d: \"%s\")...\n", workHWND, win32.GetWindowTitle(workHWND))
		time.Sleep(1 * time.Second)
		_ = win32.SetForegroundWindowWithBypass(workHWND)

		if store.MediaControl {
			fmt.Println("   [Media] Auto-resuming video/music playback...")
			time.Sleep(250 * time.Millisecond)
			win32.ToggleMediaPlayback(workHWND)
		}
	}

	fmt.Println("2. Simulating AI working in background for 4 seconds (watch video play)...")
	time.Sleep(4 * time.Second)

	if store.MediaControl && workHWND != 0 && win32.IsWindowValid(workHWND) {
		fmt.Println("3. [Media] Pausing video/music playback before switching to agent...")
		win32.ToggleMediaPlayback(workHWND)
		time.Sleep(200 * time.Millisecond)
	}

	if store.GamingMode {
		fmt.Println("4. Gaming Mode is ON: Triggering Toast Notification (No focus steal)...")
		notify.SendToast("switchback // AI Agent Alert", "AI Agent finished its task!")
	} else if agentHWND != 0 && win32.IsWindowValid(agentHWND) {
		fmt.Printf("4. Auto-switching back to Agent window (HWND %d: \"%s\")...\n", agentHWND, win32.GetWindowTitle(agentHWND))
		_ = win32.SetForegroundWindowWithBypass(agentHWND)
	}
	fmt.Println("\n[OK] Eye test complete! Everything executed as expected.")
}

func handleTestSwitchWork() {
	store, _ := state.Load()
	workHWND := win32.HWND(store.SelectedWorkHWND)
	if workHWND == 0 || !win32.IsWindowValid(workHWND) {
		openWindows := win32.GetOpenWindows()
		for _, win := range openWindows {
			if !win.IsAgent {
				workHWND = win32.HWND(win.HWND)
				break
			}
		}
	}

	if workHWND != 0 && win32.IsWindowValid(workHWND) {
		fmt.Printf("[Step 1] Switching to Work/Video window: HWND %d (\"%s\")...\n", workHWND, win32.GetWindowTitle(workHWND))
		_ = win32.SetForegroundWindowWithBypass(workHWND)

		if store.MediaControl {
			fmt.Println("[Step 1] Auto-resuming video/music playback...")
			time.Sleep(250 * time.Millisecond)
			win32.ToggleMediaPlayback(workHWND)
		}
		fmt.Println("[OK] Step 1 complete. Window is focused and playback resumed.")
	} else {
		fmt.Println("[ERROR] No valid work window found to switch to.")
	}
}

func handleTestSwitchAgent() {
	store, _ := state.Load()
	agentHWND := win32.HWND(store.SelectedAgentHWND)
	workHWND := win32.HWND(store.SelectedWorkHWND)

	if agentHWND == 0 || !win32.IsWindowValid(agentHWND) {
		agentHWND = win32.FindWindowByTitlePattern([]string{"Antigravity", "Claude", "Visual Studio Code", "Cursor", "Codex", "Terminal"})
	}

	if store.MediaControl && workHWND != 0 && win32.IsWindowValid(workHWND) {
		fmt.Println("[Step 2] Pausing video/music playback...")
		win32.ToggleMediaPlayback(workHWND)
		time.Sleep(200 * time.Millisecond)
	}

	if agentHWND != 0 && win32.IsWindowValid(agentHWND) {
		fmt.Printf("[Step 2] Auto-switching back to Agent window: HWND %d (\"%s\")...\n", agentHWND, win32.GetWindowTitle(agentHWND))
		_ = win32.SetForegroundWindowWithBypass(agentHWND)
		fmt.Println("[OK] Step 2 complete. Media paused and Agent window focused.")
	} else {
		fmt.Println("[ERROR] No valid agent window found to switch to.")
	}
}

func parseHookStdin(rawBytes []byte) (toolName string, toolInput map[string]interface{}, responseText string, conversationID string) {
	if len(rawBytes) == 0 {
		return "", nil, "", ""
	}

	var root map[string]interface{}
	if err := json.Unmarshal(rawBytes, &root); err != nil {
		return "", nil, string(rawBytes), ""
	}

	// 1. Extract conversation / session ID
	for _, key := range []string{"conversationId", "conversation_id", "session_id", "sessionId"} {
		if v, ok := root[key].(string); ok && v != "" {
			conversationID = v
			break
		}
	}

	// 2. Extract response text (for Stop events)
	for _, key := range []string{"last_assistant_response", "lastAssistantResponse", "response", "message", "output", "content"} {
		if v, ok := root[key].(string); ok && v != "" {
			responseText = v
			break
		}
	}

	// 3. Extract tool call info
	var toolObj map[string]interface{}
	for _, key := range []string{"toolCall", "tool_call", "tool", "tool_input", "toolInput"} {
		if m, ok := root[key].(map[string]interface{}); ok {
			toolObj = m
			break
		}
	}

	for _, key := range []string{"tool_name", "toolName", "name", "tool"} {
		if v, ok := root[key].(string); ok && v != "" {
			toolName = v
			break
		}
	}
	if toolName == "" && toolObj != nil {
		for _, key := range []string{"name", "tool_name", "toolName", "tool"} {
			if v, ok := toolObj[key].(string); ok && v != "" {
				toolName = v
				break
			}
		}
	}

	extractMap := func(val interface{}) map[string]interface{} {
		if val == nil {
			return nil
		}
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
		if s, ok := val.(string); ok && len(strings.TrimSpace(s)) > 0 {
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(s), &parsed); err == nil {
				return parsed
			}
		}
		return nil
	}

	inputKeys := []string{"tool_input", "toolInput", "input", "args", "arguments", "argumentsJson", "parameters"}
	for _, key := range inputKeys {
		if m := extractMap(root[key]); m != nil {
			toolInput = m
			break
		}
	}
	if toolInput == nil && toolObj != nil {
		for _, key := range inputKeys {
			if m := extractMap(toolObj[key]); m != nil {
				toolInput = m
				break
			}
		}
	}

	if toolInput == nil {
		if _, hasQ := root["questions"]; hasQ {
			toolInput = root
		} else if _, hasCmd := root["CommandLine"]; hasCmd {
			toolInput = root
		} else if _, hasTarget := root["TargetFile"]; hasTarget {
			toolInput = root
		}
	}

	return toolName, toolInput, responseText, conversationID
}

// isSafeAction determines if a tool execution is safe for silent auto-approval.
func isSafeAction(toolName string, toolInput map[string]interface{}) bool {
	if toolName == "read_file" || toolName == "view_file" || toolName == "list_dir" || toolName == "grep_search" || toolName == "read_url_content" {
		return true
	}
	if toolName == "ask_question" || toolName == "ask_user" || toolName == "ask" {
		qText := ""
		if toolInput != nil {
			if qRaw, ok := toolInput["questions"]; ok {
				if qList, ok := qRaw.([]interface{}); ok && len(qList) > 0 {
					if qMap, ok := qList[0].(map[string]interface{}); ok {
						if qStr, ok := qMap["question"].(string); ok {
							qText = qStr
						}
					}
				}
			}
			if qText == "" {
				if qStr, ok := toolInput["question"].(string); ok {
					qText = qStr
				}
			}
		}
		cleanQ := strings.ToLower(qText)
		dangerTokens := []string{"rm ", "rmdir", "del ", "delete", "destroy", "drop", "truncate", "erase", "remove", "overwrite", "kill", "format"}
		for _, token := range dangerTokens {
			if strings.Contains(cleanQ, token) {
				return false
			}
		}
		return true
	}
	if toolName == "run_command" && toolInput != nil {
		if cmd, ok := toolInput["CommandLine"].(string); ok {
			cleanCmd := strings.TrimSpace(strings.ToLower(cmd))
			// Reject any dangerous or mutating commands
			dangerTokens := []string{"rm ", "rmdir", "del ", "delete", "erase", "push", "drop", "truncate", "format", "chmod", "chown", ">", ">>", "kill", "taskkill", "shutdown", "reboot"}
			for _, token := range dangerTokens {
				if strings.Contains(cleanCmd, token) {
					return false
				}
			}
			// Allow safe read-only queries
			safePrefixes := []string{"git status", "git diff", "git log", "git branch", "git show", "git tag", "ls", "dir", "cat ", "echo ", "pwd", "type ", "whoami", "hostname", "cargo check", "go test", "npm test"}
			for _, prefix := range safePrefixes {
				if strings.HasPrefix(cleanCmd, prefix) {
					return true
				}
			}
		}
	}
	return false
}

func parseApprovalData(toolName string, toolInput map[string]interface{}, rawMsg string) (string, string, []string, string, string, string) {
	title := "Agent Action Approval"
	message := "The AI Agent requires permission for an action."
	options := []string{"Allow Action", "Deny Action"}
	toolClean := toolName
	snippet := ""
	dangerLevel := "medium"

	if toolInput == nil {
		if rawMsg != "" {
			message = rawMsg
			snippet = rawMsg
		}
		return title, message, options, toolClean, snippet, dangerLevel
	}

	isAskQuestion := (toolName == "ask_question" || toolName == "ask_user" || toolName == "ask")
	if !isAskQuestion {
		if _, ok := toolInput["questions"]; ok {
			isAskQuestion = true
		} else if _, ok := toolInput["question"]; ok {
			isAskQuestion = true
		}
	}

	if isAskQuestion {
		title = "AI Agent Question"
		toolClean = "ask_question"
		dangerLevel = "medium"

		// 1. Check questions array
		if qRaw, ok := toolInput["questions"]; ok {
			var qList []interface{}
			if list, ok := qRaw.([]interface{}); ok {
				qList = list
			} else if s, ok := qRaw.(string); ok {
				var parsed []interface{}
				_ = json.Unmarshal([]byte(s), &parsed)
				qList = parsed
			}

			if len(qList) > 0 {
				if qMap, ok := qList[0].(map[string]interface{}); ok {
					if qText, ok := qMap["question"].(string); ok && qText != "" {
						message = qText
						snippet = qText
					}
					if optRaw, ok := qMap["options"]; ok {
						options = extractOptions(optRaw)
					}
				}
			}
		}

		// 2. Direct question field
		if qText, ok := toolInput["question"].(string); ok && qText != "" {
			message = qText
			snippet = qText
		}
		if optRaw, ok := toolInput["options"]; ok {
			if opts := extractOptions(optRaw); len(opts) > 0 {
				options = opts
			}
		}

		if len(options) == 0 {
			options = []string{"Yes, proceed with action", "No, cancel"}
		}
		return title, message, options, toolClean, snippet, dangerLevel
	}

	// Tool execution approval
	if toolName != "" {
		title = fmt.Sprintf("Tool: %s", toolName)
		if cmd, ok := toolInput["CommandLine"].(string); ok && cmd != "" {
			message = fmt.Sprintf("Execute command:\n%s", cmd)
			snippet = cmd
			lower := strings.ToLower(cmd)
			if strings.Contains(lower, "rm ") || strings.Contains(lower, "del ") || strings.Contains(lower, "push") || strings.Contains(lower, "drop") {
				dangerLevel = "danger"
			} else if strings.HasPrefix(lower, "git status") || strings.HasPrefix(lower, "ls") || strings.HasPrefix(lower, "dir") {
				dangerLevel = "safe"
			} else {
				dangerLevel = "warning"
			}
		} else if target, ok := toolInput["TargetFile"].(string); ok && target != "" {
			message = fmt.Sprintf("Modify file:\n%s", target)
			snippet = target
			dangerLevel = "warning"
		} else if url, ok := toolInput["Url"].(string); ok && url != "" {
			message = fmt.Sprintf("Open URL:\n%s", url)
			snippet = url
			dangerLevel = "medium"
		} else if path, ok := toolInput["AbsolutePath"].(string); ok && path != "" {
			message = fmt.Sprintf("Access file:\n%s", path)
			snippet = path
			dangerLevel = "safe"
		} else {
			message = fmt.Sprintf("Agent requested to execute tool '%s'", toolName)
			snippet = toolName
			dangerLevel = "medium"
		}
	} else if rawMsg != "" {
		message = rawMsg
		snippet = rawMsg
	}

	return title, message, options, toolClean, snippet, dangerLevel
}

func extractOptions(optRaw interface{}) []string {
	var options []string
	if optList, ok := optRaw.([]interface{}); ok {
		for _, o := range optList {
			if s, ok := o.(string); ok && s != "" {
				options = append(options, s)
			} else if om, ok := o.(map[string]interface{}); ok {
				for _, k := range []string{"label", "text", "option", "value", "title"} {
					if s, ok := om[k].(string); ok && s != "" {
						options = append(options, s)
						break
					}
				}
			}
		}
	}
	return options
}
