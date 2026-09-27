package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"focusmgr/internal/config"
	"focusmgr/internal/installer"
	"focusmgr/internal/logger"
	"focusmgr/internal/state"
	"focusmgr/internal/win32"
)

var version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "save-and-focus":
		handleSaveAndFocus(args)
	case "restore":
		handleRestore(args)
	case "status":
		handleStatus()
	case "install":
		handleInstall(args)
	case "test-focus":
		handleTestFocus()
	case "version", "--version", "-v":
		fmt.Printf("focusmgr version %s (Windows x64)\n", version)
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`focusmgr - Auto-Focus Window Manager for AI Agents

Usage:
  focusmgr save-and-focus --session <id> [--agent <type>] [--title <pattern>]
  focusmgr restore        --session <id> [--agent <type>]
  focusmgr status
  focusmgr install        [--agent <claude|antigravity|all>]
  focusmgr test-focus     (Test window switching with a 3s countdown)
  focusmgr version`)
}

func handleSaveAndFocus(args []string) {
	fs := flag.NewFlagSet("save-and-focus", flag.ContinueOnError)
	sessionID := fs.String("session", "default", "Session identifier")
	agentType := fs.String("agent", "claude-code", "Agent type: claude-code, codex, antigravity")
	titlePattern := fs.String("title", "", "Optional window title override pattern")
	_ = fs.Parse(args)

	// Clean environment variable expansion if passed raw (e.g. $CLAUDE_SESSION_ID or %conversationId%)
	cleanSession := strings.TrimSpace(*sessionID)
	if cleanSession == "" || cleanSession == "$CLAUDE_SESSION_ID" || cleanSession == "%conversationId%" {
		cleanSession = "default"
	}

	logger.Info("[save-and-focus] Triggered for session: %s, agent: %s", cleanSession, *agentType)

	// 1. Capture current foreground window (the user's working window)
	userHWND := win32.GetForegroundWindow()
	userTitle := win32.GetWindowTitle(userHWND)
	logger.Info("[save-and-focus] Current foreground window HWND: %d (%s)", userHWND, userTitle)

	// 2. Identify agent window
	cfg := config.LoadConfig()
	var agentHWND win32.HWND

	// Strategy A: Ancestor process tree resolution
	if agentCfg, ok := cfg.Agents[*agentType]; ok && agentCfg.AutoDetectParent {
		agentHWND = win32.FindAncestorWindow()
		if agentHWND != 0 {
			logger.Info("[save-and-focus] Auto-detected ancestor agent window HWND: %d (%s)", agentHWND, win32.GetWindowTitle(agentHWND))
		}
	}

	// Strategy B: Title match pattern from config
	if agentHWND == 0 {
		var patterns []string
		if *titlePattern != "" {
			patterns = []string{*titlePattern}
		} else if agentCfg, ok := cfg.Agents[*agentType]; ok {
			patterns = agentCfg.TitlePatterns
		}
		if len(patterns) > 0 {
			agentHWND = win32.FindWindowByTitlePattern(patterns)
			if agentHWND != 0 {
				logger.Info("[save-and-focus] Found agent window by title pattern HWND: %d (%s)", agentHWND, win32.GetWindowTitle(agentHWND))
			}
		}
	}

	// 3. Save state
	sessState := state.SessionState{
		SavedHWND:  uintptr(userHWND),
		SavedTitle: userTitle,
		AgentHWND:  uintptr(agentHWND),
		AgentType:  *agentType,
		State:      "waiting_user_prompt",
		UpdatedAt:  time.Now(),
	}
	if err := state.SaveSession(cleanSession, sessState); err != nil {
		logger.Error("[save-and-focus] Failed to save session state: %v", err)
	}

	// 4. Activate agent window
	if agentHWND != 0 && agentHWND != userHWND {
		if err := win32.SetForegroundWindowWithBypass(agentHWND); err != nil {
			logger.Error("[save-and-focus] SetForegroundWindow failed: %v", err)
		} else {
			logger.Info("[save-and-focus] Successfully activated agent window HWND: %d", agentHWND)
		}
	}

	// Always exit 0 so agent hooks never fail
	os.Exit(0)
}

func handleRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	sessionID := fs.String("session", "default", "Session identifier")
	agentType := fs.String("agent", "claude-code", "Agent type")
	_ = fs.Parse(args)

	cleanSession := strings.TrimSpace(*sessionID)
	if cleanSession == "" || cleanSession == "$CLAUDE_SESSION_ID" || cleanSession == "%conversationId%" {
		cleanSession = "default"
	}

	logger.Info("[restore] Triggered for session: %s, agent: %s", cleanSession, *agentType)

	sess, found, err := state.GetSession(cleanSession)
	if err != nil || !found {
		logger.Info("[restore] No saved state found for session %s, exiting gracefully", cleanSession)
		os.Exit(0)
	}

	// Check Antigravity debounce if applicable
	cfg := config.LoadConfig()
	if *agentType == "antigravity" {
		if agentCfg, ok := cfg.Agents["antigravity"]; ok && agentCfg.DebounceMS > 0 {
			elapsed := time.Since(sess.UpdatedAt)
			if elapsed < time.Duration(agentCfg.DebounceMS)*time.Millisecond && sess.State == "agent_running" {
				logger.Info("[restore] Debouncing Antigravity PreInvocation (%v elapsed)", elapsed)
				os.Exit(0)
			}
		}
	}

	targetHWND := win32.HWND(sess.SavedHWND)
	if !win32.IsWindowValid(targetHWND) {
		logger.Info("[restore] Saved window HWND %d no longer exists, skipping restore", targetHWND)
		os.Exit(0)
	}

	logger.Info("[restore] Restoring focus to HWND %d (%s)", targetHWND, sess.SavedTitle)
	if err := win32.SetForegroundWindowWithBypass(targetHWND); err != nil {
		logger.Error("[restore] Failed to restore foreground window: %v", err)
	} else {
		logger.Info("[restore] Focus successfully restored to HWND %d", targetHWND)
	}

	// Update state
	sess.State = "agent_running"
	_ = state.SaveSession(cleanSession, sess)

	os.Exit(0)
}

func handleStatus() {
	fg := win32.GetForegroundWindow()
	title := win32.GetWindowTitle(fg)
	fmt.Printf("Current Foreground Window: HWND %d (\"%s\")\n", fg, title)

	statePath, _ := state.GetStateFilePath()
	fmt.Printf("State File: %s\n", statePath)

	configPath, _ := config.GetConfigFilePath()
	fmt.Printf("Config File: %s\n", configPath)

	store, err := state.Load()
	if err == nil {
		fmt.Printf("Active Tracked Sessions: %d\n", len(store.Sessions))
		for id, s := range store.Sessions {
			fmt.Printf("  • [%s] Agent: %s, User HWND: %d (\"%s\"), State: %s\n",
				id, s.AgentType, s.SavedHWND, s.SavedTitle, s.State)
		}
	}
}

func handleInstall(args []string) {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path: %v\n", err)
		return
	}
	exePath, _ = filepath.Abs(exePath)

	fmt.Printf("Installing focusmgr hooks using binary: %s\n", exePath)

	// Claude Code
	if err := installer.InstallClaudeHooks(exePath); err != nil {
		fmt.Printf("[-] Failed to configure Claude Code hooks: %v\n", err)
	} else {
		fmt.Println("[+] Claude Code hooks configured successfully (~/.claude/settings.json)")
	}

	// Antigravity
	cwd, _ := os.Getwd()
	if err := installer.InstallAntigravityHooks(exePath, cwd); err != nil {
		fmt.Printf("[-] Failed to configure Antigravity hooks: %v\n", err)
	} else {
		fmt.Println("[+] Antigravity hooks configured successfully (hooks.json)")
	}
}

func handleTestFocus() {
	fmt.Println("--- focusmgr Focus Switch Test ---")
	fmt.Println("Step 1: In 3 seconds, focusmgr will capture your currently active window.")
	for i := 3; i > 0; i-- {
		fmt.Printf("%d...\n", i)
		time.Sleep(1 * time.Second)
	}

	userHWND := win32.GetForegroundWindow()
	userTitle := win32.GetWindowTitle(userHWND)
	fmt.Printf("[Captured] HWND: %d (\"%s\")\n\n", userHWND, userTitle)

	fmt.Println("Step 2: Please click on another window (e.g. Chrome, Notepad) NOW!")
	for i := 5; i > 0; i-- {
		fmt.Printf("Switching back in %d seconds...\n", i)
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("Step 3: Restoring focus to HWND %d (\"%s\")...\n", userHWND, userTitle)
	if err := win32.SetForegroundWindowWithBypass(userHWND); err != nil {
		fmt.Printf("[-] Focus restoration failed: %v\n", err)
	} else {
		fmt.Println("[+] Focus restoration succeeded!")
	}
}
