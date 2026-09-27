package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstallClaudeHooks configures hooks in ~/.claude/settings.json
func InstallClaudeHooks(focusmgrPath string) error {
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

	stopCmd := fmt.Sprintf("\"%s\" save-and-focus --agent claude-code --session $CLAUDE_SESSION_ID", focusmgrPath)
	promptCmd := fmt.Sprintf("\"%s\" restore --agent claude-code --session $CLAUDE_SESSION_ID", focusmgrPath)

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

// InstallAntigravityHooks configures hooks in ~/.gemini/config/hooks.json or local workspace
func InstallAntigravityHooks(focusmgrPath string, workspaceDir string) error {
	targetPaths := []string{}

	if workspaceDir != "" {
		targetPaths = append(targetPaths, filepath.Join(workspaceDir, ".agents", "hooks.json"))
	}

	home, err := os.UserHomeDir()
	if err == nil {
		targetPaths = append(targetPaths, filepath.Join(home, ".gemini", "config", "hooks.json"))
	}

	stopCmd := fmt.Sprintf("\"%s\" save-and-focus --agent antigravity --session %%conversationId%%", focusmgrPath)
	preCmd := fmt.Sprintf("\"%s\" restore --agent antigravity --session %%conversationId%%", focusmgrPath)

	type HookDef struct {
		Event   string `json:"event"`
		Command string `json:"command"`
	}

	type HooksFile struct {
		Hooks []HookDef `json:"hooks"`
	}

	for _, p := range targetPaths {
		_ = os.MkdirAll(filepath.Dir(p), 0755)

		var hf HooksFile
		data, err := os.ReadFile(p)
		if err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &hf)
		}

		// Filter out any existing focusmgr entries
		filtered := []HookDef{}
		for _, h := range hf.Hooks {
			if h.Event != "Stop" && h.Event != "PreInvocation" {
				filtered = append(filtered, h)
			}
		}

		filtered = append(filtered, HookDef{Event: "Stop", Command: stopCmd})
		filtered = append(filtered, HookDef{Event: "PreInvocation", Command: preCmd})

		hf.Hooks = filtered
		out, err := json.MarshalIndent(hf, "", "  ")
		if err == nil {
			_ = os.WriteFile(p, out, 0644)
		}
	}

	return nil
}
