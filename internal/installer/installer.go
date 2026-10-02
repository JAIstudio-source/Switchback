package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"switchback/internal/win32"
)

// InstallClaudeHooks configures hooks in ~/.claude/settings.json
func InstallClaudeHooks(switchbackPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	claudeDir := filepath.Join(home, ".claude")
	_ = os.MkdirAll(claudeDir, 0755)
	settingsPath := filepath.Join(claudeDir, "settings.json")

	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]interface{})
	}

	hooksObj, ok := settings["hooks"].(map[string]interface{})
	if !ok || hooksObj == nil {
		hooksObj = make(map[string]interface{})
	}

	stopCmd := fmt.Sprintf("\"%s\" hook --event Stop --agent claude-code", switchbackPath)
	notifyCmd := fmt.Sprintf("\"%s\" hook --event Notification --agent claude-code", switchbackPath)
	promptCmd := fmt.Sprintf("\"%s\" hook --event UserPromptSubmit --agent claude-code", switchbackPath)

	hooksObj["Stop"] = []interface{}{
		map[string]interface{}{
			"matcher": "",
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": stopCmd,
				},
			},
		},
	}

	hooksObj["Notification"] = []interface{}{
		map[string]interface{}{
			"matcher": "",
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": notifyCmd,
				},
			},
		},
	}

	hooksObj["UserPromptSubmit"] = []interface{}{
		map[string]interface{}{
			"matcher": "",
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": promptCmd,
				},
			},
		},
	}

	settings["hooks"] = hooksObj

	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, updated, 0644)
}

// InstallAntigravityHooks configures hooks in ~/.gemini/config/hooks.json (global) or local workspace
func InstallAntigravityHooks(switchbackPath string, workspaceDir string) error {
	targetPaths := []string{}

	home, err := os.UserHomeDir()
	if err == nil {
		targetPaths = append(targetPaths, filepath.Join(home, ".gemini", "config", "hooks.json"))
	} else if workspaceDir != "" {
		targetPaths = append(targetPaths, filepath.Join(workspaceDir, ".agents", "hooks.json"))
	}

	// On Windows, Antigravity executes commands via cmd /c. If the command string contains
	// escaped quotes, cmd.exe fails with "'\"...\"' is not recognized as an internal or external command".
	// Therefore, we convert any path with spaces to an 8.3 short path and omit outer quotes.
	cleanPath := switchbackPath
	if strings.Contains(cleanPath, " ") {
		cleanPath = win32.GetShortPath(cleanPath)
	}
	stopCmd := fmt.Sprintf("%s hook --event Stop --agent antigravity", cleanPath)
	preCmd := fmt.Sprintf("%s hook --event PreInvocation --agent antigravity", cleanPath)
	notifyCmd := fmt.Sprintf("%s hook --event Notification --agent antigravity", cleanPath)

	type HookHandler struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout,omitempty"`
	}

	type MatcherGroup struct {
		Matcher string        `json:"matcher"`
		Hooks   []HookHandler `json:"hooks"`
	}

	type HookGroup struct {
		PreToolUse    []MatcherGroup `json:"PreToolUse,omitempty"`
		PreInvocation []HookHandler  `json:"PreInvocation,omitempty"`
		Stop          []HookHandler  `json:"Stop,omitempty"`
	}

	for _, p := range targetPaths {
		_ = os.MkdirAll(filepath.Dir(p), 0755)

		var rootMap map[string]interface{}
		data, err := os.ReadFile(p)
		if err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &rootMap)
		}
		if rootMap == nil {
			rootMap = make(map[string]interface{})
		}

		// Clean up any obsolete incorrect "hooks" array or old "focusmgr" group if present
		delete(rootMap, "hooks")
		delete(rootMap, "focusmgr")

		// Configure named switchback hook group
		rootMap["switchback"] = HookGroup{
			PreToolUse: []MatcherGroup{
				{
					Matcher: "ask_question",
					Hooks: []HookHandler{
						{
							Type:    "command",
							Command: notifyCmd,
							Timeout: 15,
						},
					},
				},
			},
			PreInvocation: []HookHandler{
				{
					Type:    "command",
					Command: preCmd,
					Timeout: 15,
				},
			},
			Stop: []HookHandler{
				{
					Type:    "command",
					Command: stopCmd,
					Timeout: 15,
				},
			},
		}

		out, err := json.MarshalIndent(rootMap, "", "  ")
		if err == nil {
			_ = os.WriteFile(p, out, 0644)
		}
	}

	return nil
}

// UninstallClaudeHooks removes switchback and legacy focusmgr hooks from ~/.claude/settings.json
func UninstallClaudeHooks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil // Nothing to uninstall
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}

	hooksObj, ok := settings["hooks"].(map[string]interface{})
	if !ok || hooksObj == nil {
		return nil
	}

	// Filter out switchback and focusmgr hooks from Stop, Notification, UserPromptSubmit
	events := []string{"Stop", "Notification", "UserPromptSubmit"}
	for _, evt := range events {
		if arr, ok := hooksObj[evt].([]interface{}); ok {
			filtered := []interface{}{}
			for _, item := range arr {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				innerHooks, ok := m["hooks"].([]interface{})
				if !ok {
					continue
				}
				keepInner := []interface{}{}
				for _, h := range innerHooks {
					hm, ok := h.(map[string]interface{})
					if ok {
						cmd, _ := hm["command"].(string)
						if strings.Contains(cmd, "switchback") || strings.Contains(cmd, "focusmgr") {
							continue
						}
					}
					keepInner = append(keepInner, h)
				}
				if len(keepInner) > 0 {
					m["hooks"] = keepInner
					filtered = append(filtered, m)
				}
			}
			if len(filtered) > 0 {
				hooksObj[evt] = filtered
			} else {
				delete(hooksObj, evt)
			}
		}
	}

	if len(hooksObj) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooksObj
	}

	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, updated, 0644)
}

// UninstallAntigravityHooks removes switchback and focusmgr hooks from ~/.gemini/config/hooks.json and workspace .agents/hooks.json
func UninstallAntigravityHooks(workspaceDir string) error {
	targetPaths := []string{}

	home, err := os.UserHomeDir()
	if err == nil {
		targetPaths = append(targetPaths, filepath.Join(home, ".gemini", "config", "hooks.json"))
	}
	if workspaceDir != "" {
		targetPaths = append(targetPaths, filepath.Join(workspaceDir, ".agents", "hooks.json"))
	}

	for _, p := range targetPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var rootMap map[string]interface{}
		if err := json.Unmarshal(data, &rootMap); err != nil {
			continue
		}

		delete(rootMap, "switchback")
		delete(rootMap, "focusmgr")
		delete(rootMap, "hooks")

		out, err := json.MarshalIndent(rootMap, "", "  ")
		if err == nil {
			_ = os.WriteFile(p, out, 0644)
		}
	}

	return nil
}

// CheckHooksInstalled checks if switchback (or legacy focusmgr) is registered in Antigravity or Claude
func CheckHooksInstalled(workspaceDir string) (bool, bool) {
	antigravityInstalled := false
	claudeInstalled := false

	home, err := os.UserHomeDir()
	if err == nil {
		agPath := filepath.Join(home, ".gemini", "config", "hooks.json")
		if data, err := os.ReadFile(agPath); err == nil {
			if strings.Contains(string(data), "switchback") || strings.Contains(string(data), "focusmgr") {
				antigravityInstalled = true
			}
		}
	}
	if workspaceDir != "" && !antigravityInstalled {
		agPath := filepath.Join(workspaceDir, ".agents", "hooks.json")
		if data, err := os.ReadFile(agPath); err == nil {
			if strings.Contains(string(data), "switchback") || strings.Contains(string(data), "focusmgr") {
				antigravityInstalled = true
			}
		}
	}

	if home != "" {
		claudePath := filepath.Join(home, ".claude", "settings.json")
		if data, err := os.ReadFile(claudePath); err == nil {
			if strings.Contains(string(data), "switchback") || strings.Contains(string(data), "focusmgr") {
				claudeInstalled = true
			}
		}
	}

	return antigravityInstalled, claudeInstalled
}

// EnsureIDEKeybindings configures keybindings for Antigravity IDE and VS Code to ensure chat focus works seamlessly
func EnsureIDEKeybindings() error {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		appdata = filepath.Join(home, "AppData", "Roaming")
	}

	targets := []string{
		filepath.Join(appdata, "Antigravity IDE", "User", "keybindings.json"),
		filepath.Join(appdata, "Antigravity", "User", "keybindings.json"),
		filepath.Join(appdata, "Code", "User", "keybindings.json"),
	}

	for _, target := range targets {
		dir := filepath.Dir(target)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			var entries []map[string]interface{}
			data, err := os.ReadFile(target)
			if err == nil && len(data) > 0 {
				_ = json.Unmarshal(data, &entries)
			}
			if entries == nil {
				entries = make([]map[string]interface{}, 0)
			}

			hasBinding := false
			for _, e := range entries {
				if cmd, ok := e["command"].(string); ok && (cmd == "workbench.action.chat.open" || cmd == "workbench.action.chat.focusInput") {
					hasBinding = true
					break
				}
			}

			if !hasBinding {
				entries = append(entries, map[string]interface{}{
					"key":     "ctrl+alt+i",
					"command": "workbench.action.chat.open",
				})
				entries = append(entries, map[string]interface{}{
					"key":     "ctrl+alt+i",
					"command": "workbench.action.chat.focusInput",
				})
				out, err := json.MarshalIndent(entries, "", "  ")
				if err == nil {
					_ = os.WriteFile(target, out, 0644)
				}
			}
		}
	}
	return nil
}


