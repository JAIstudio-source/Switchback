package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"focusmgr/internal/installer"
	"focusmgr/internal/logger"
	"focusmgr/internal/notify"
	"focusmgr/internal/server"
	"focusmgr/internal/state"
	"focusmgr/internal/win32"
)

//go:embed ui/*
var embeddedUI embed.FS

var version = "1.1.0"

func main() {
	if len(os.Args) < 2 {
		// Default when double-clicked from Windows Explorer: launch the Pixel UI dashboard!
		runUI()
		return
	}

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
		handleInstall(args)
	case "test-focus":
		handleTestFocus()
	case "version", "--version", "-v":
		fmt.Printf("focusmgr version %s (Windows x64)\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		printUsage()
	}
}

func runUI() {
	port := 48123
	if err := server.StartServer(port, embeddedUI); err != nil {
		fmt.Printf("Failed to launch UI server: %v\n", err)
	}
}

func printUsage() {
	fmt.Println(`focusmgr - Auto-Focus Window Manager for AI Agents

Usage:
  focusmgr ui                                    (Launch Pixel UI dashboard in browser)
  focusmgr hook --event <Stop|UserPromptSubmit|PreInvocation|Notification>
  focusmgr status                                (Show active windows and configuration)
  focusmgr install                               (Configure hooks in Claude Code and Antigravity)
  focusmgr test-focus                            (Interactive test)
  focusmgr version`)
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

	logger.Info("[Hook] Event: %s, Agent: %s, Session: %s", *event, *agentType, cleanSession)

	store, err := state.Load()
	if err != nil {
		os.Exit(0)
	}

	switch *event {
	case "UserPromptSubmit", "PreInvocation":
		// 1. User submitted task OR granted permission!
		// Switch focus to the Selected Work Window
		if !store.AutoSwitchEnabled {
			os.Exit(0)
		}

		workHWND := win32.HWND(store.SelectedWorkHWND)
		if workHWND == 0 {
			// Fallback: check session saved HWND
			if sess, ok := store.Sessions[cleanSession]; ok && sess.SavedHWND != 0 {
				workHWND = win32.HWND(sess.SavedHWND)
			}
		}

		if workHWND != 0 && win32.IsWindowValid(workHWND) {
			logger.Info("[AutoSwitch] Task submitted / Permission granted. Auto-switching to work window: HWND %d (%s)",
				workHWND, win32.GetWindowTitle(workHWND))
			_ = win32.SetForegroundWindowWithBypass(workHWND)
			store.CurrentStatus = "agent_working"
			_ = store.Save()
		}

	case "Stop", "Notification":
		// 2. Agent finished task OR requires user permission!
		currentFG := win32.GetForegroundWindow()

		// Save current foreground window as work window if not already set
		if store.SelectedWorkHWND == 0 && currentFG != 0 {
			_ = state.SetSelectedWork(uintptr(currentFG), win32.GetWindowTitle(currentFG))
		}

		if store.GamingMode {
			// Gaming / DND mode: do not steal focus, send toast notification!
			logger.Info("[GamingMode] Agent requires permission or attention. Sent notification toast only.")
			notify.SendToast("focusmgr // AI Agent Alert", "Your AI Agent requires permission or attention!")
			store.CurrentStatus = "permission_needed"
			_ = store.Save()
			os.Exit(0)
		}

		if !store.AutoSwitchEnabled {
			os.Exit(0)
		}

		// Full auto switch mode: switch to agent window
		agentHWND := win32.HWND(store.SelectedAgentHWND)
		if agentHWND == 0 {
			// Auto detect
			agentHWND = win32.FindAncestorWindow()
			if agentHWND == 0 {
				agentHWND = win32.FindWindowByTitlePattern([]string{"Antigravity", "Claude", "Visual Studio Code", "Cursor", "Codex", "Terminal"})
			}
		}

		if agentHWND != 0 && win32.IsWindowValid(agentHWND) {
			logger.Info("[AutoSwitch] Agent needs permission. Auto-switching to agent window: HWND %d (%s)",
				agentHWND, win32.GetWindowTitle(agentHWND))
			_ = win32.SetForegroundWindowWithBypass(agentHWND)
			store.CurrentStatus = "permission_needed"
			_ = store.Save()
		}
	}

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
		fmt.Printf("Current Agent Workflow Status:    %s\n", store.CurrentStatus)
	}

	statePath, _ := state.GetStateFilePath()
	fmt.Printf("State File: %s\n", statePath)
}

func handleInstall(args []string) {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Failed to get executable path: %v\n", err)
		return
	}
	exePath, _ = filepath.Abs(exePath)

	fmt.Printf("Installing focusmgr hooks using binary: %s\n", exePath)

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
}

func handleTestFocus() {
	fmt.Println("--- focusmgr Focus Switch Test ---")
	store, _ := state.Load()

	fmt.Printf("Selected Agent: HWND %d (\"%s\")\n", store.SelectedAgentHWND, store.SelectedAgentTitle)
	fmt.Printf("Selected Work:  HWND %d (\"%s\")\n", store.SelectedWorkHWND, store.SelectedWorkTitle)
	fmt.Printf("Gaming Mode:    %v\n\n", store.GamingMode)

	if store.SelectedWorkHWND != 0 && win32.IsWindowValid(win32.HWND(store.SelectedWorkHWND)) {
		fmt.Println("1. Switching to Work window in 2 seconds...")
		time.Sleep(2 * time.Second)
		_ = win32.SetForegroundWindowWithBypass(win32.HWND(store.SelectedWorkHWND))
	}

	time.Sleep(3 * time.Second)

	if store.GamingMode {
		fmt.Println("2. Gaming Mode is ON: Triggering Toast Notification (No focus steal)...")
		notify.SendToast("focusmgr // AI Agent Alert", "Agent needs permission: Review tool call!")
	} else if store.SelectedAgentHWND != 0 && win32.IsWindowValid(win32.HWND(store.SelectedAgentHWND)) {
		fmt.Println("2. Auto-switching back to Agent window...")
		_ = win32.SetForegroundWindowWithBypass(win32.HWND(store.SelectedAgentHWND))
	}
	fmt.Println("Test complete!")
}
