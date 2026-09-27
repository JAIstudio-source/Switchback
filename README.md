# focusmgr - Auto-Focus Window Manager for AI Agents

`focusmgr` is a lightweight, sub-50ms Windows window management CLI designed specifically to automate the **"Focus Loop"** for AI coding agents such as **Claude Code**, **OpenAI Codex**, and **Antigravity**.

---

## ⚡ The Focus Loop Workflow

```
       ┌────────────────────────────────────────────────────────┐
       │                  User Working Context                  │
       │            (Browser / Slack / Editor / etc.)           │
       └───────────────────────────┬────────────────────────────┘
                                   │
                Agent Task Completes (`Stop` Hook)
                  1. Save current foreground HWND
                  2. Bring Agent window to front
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │                   AI Agent Window                      │
       │       (Terminal / VS Code / Antigravity Session)       │
       └───────────────────────────┬────────────────────────────┘
                                   │
              User Prompts Agent (`UserPromptSubmit` Hook)
                  1. Validate saved user HWND
                  2. Restore user's previous window
                                   │
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │              Context Restored Seamlessly!              │
       │          (Resume previous task with zero friction)     │
       └────────────────────────────────────────────────────────┘
```

---

## 🚀 Key Features

* **Sub-50ms Hook Latency:** Starts and executes window switches almost instantaneously without lagging or stalling agent sessions.
* **Fail-Safe Operation:** Always exits with code `0`. Any failure is recorded in `%LOCALAPPDATA%\focusmgr\focusmgr.log` without interfering with agent execution.
* **Windows Focus-Stealing Bypass:** Utilizes `AttachThreadInput`, temporary foreground lock timeout override (`SPI_SETFOREGROUNDLOCKTIMEOUT`), and simulated `VK_MENU` pulse to reliably bypass Windows 10/11 foreground restrictions.
* **Parent Process Auto-Detection:** Automatically inspects ancestor process trees to find the terminal emulator or IDE window hosting the agent.
* **Safe Concurrent Multi-Session Tracking:** State is stored atomically in `%LOCALAPPDATA%\focusmgr\state.json`.

---

## 🛠️ Commands

```powershell
# Save foreground window and switch focus to agent
focusmgr save-and-focus --session <session-id> [--agent claude-code|codex|antigravity]

# Restore focus to previous window when user prompts agent
focusmgr restore --session <session-id> [--agent claude-code|codex|antigravity]

# Check active window, tracked sessions, and paths
focusmgr status

# Automatically configure hooks in installed agents
focusmgr install

# Interactive 3-second focus switch test
focusmgr test-focus
```

---

## ⚙️ Configuration (`%LOCALAPPDATA%\focusmgr\config.json`)

```json
{
  "log_to_file": true,
  "log_level": "info",
  "agents": {
    "claude-code": {
      "title_patterns": ["Claude", "Windows Terminal", "Visual Studio Code"],
      "auto_detect_parent": true,
      "debounce_ms": 0
    },
    "codex": {
      "title_patterns": ["Codex", "Windows Terminal"],
      "auto_detect_parent": true,
      "debounce_ms": 0
    },
    "antigravity": {
      "title_patterns": ["Antigravity", "Visual Studio Code"],
      "auto_detect_parent": true,
      "debounce_ms": 2000
    }
  }
}
```
