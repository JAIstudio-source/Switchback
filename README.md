<div align="center">

# ⚡ SwitchBack
### Autonomous Focus Switcher, Media Controller & Mobile Remote for AI Coding Agents

[![Release](https://img.shields.io/github/v/release/JAIstudio-source/Switchback?color=blue&style=flat-square)](https://github.com/JAIstudio-source/Switchback/releases)
[![Platform](https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078d7.svg?style=flat-square)](https://github.com/JAIstudio-source/Switchback)
[![License](https://img.shields.io/badge/license-MIT-green.svg?style=flat-square)](LICENSE)
[![Go Report](https://img.shields.io/badge/go-1.24-00ADD8.svg?style=flat-square)](https://golang.org)

**Watch videos, browse, or game while your AI coding agent works.**  
SwitchBack automatically switches windows and pauses media when your agent needs input, and lets you control everything from your phone over local Wi-Fi.

[📥 Download Latest Setup.exe](https://github.com/JAIstudio-source/Switchback/releases/latest) • [📱 Mobile Remote Guide](#-mobile-remote-controller) • [🤖 Supported Agents](#-supported-ai-agents)

---

</div>

## 💡 What is SwitchBack?

When working with autonomous AI coding agents (**Antigravity**, **Claude Code**, **Cursor**, **Windsurf**), you often wait 1 to 5 minutes while the agent analyzes files, writes code, and runs tests. 

Instead of staring at a terminal, **SwitchBack lets you switch to YouTube, VLC, or a game**:

1. **You submit a prompt** ➔ SwitchBack automatically switches to your video/game and **resumes playback**.
2. **Agent works in background** ➔ You enjoy your movie or game undisturbed.
3. **Agent finishes or asks a question** ➔ SwitchBack automatically **pauses your media** and brings the agent window back to your screen.

---

## 📱 Mobile Remote Controller

Control your PC's AI agent from your couch, kitchen, or phone over local Wi-Fi:

* **🚀 Send Prompts**: Type instructions or tap quick prompt chips from your phone.
* **✔ 1-Tap Approvals**: When your agent asks `[y/n]` or multiple-choice questions, interactive buttons pop up on your phone.
* **⚡ Live Output Stream**: See real-time progress and command output directly on your phone screen.
* **⏯️ PC Media Toggle**: Play/pause your PC's media (YouTube, Spotify, VLC) from your phone.

> **Access URL**: Simply open `http://<YOUR-PC-IP>:48123/mobile.html` on your mobile browser (displayed in the desktop launcher).

---

## 🤖 Supported AI Agents

| Agent / IDE | Focus Switch | Media Pause/Resume | Mobile Prompts & Approvals |
| :--- | :---: | :---: | :---: |
| **Google Antigravity IDE** | ✅ | ✅ | ⚡ Native Language Server IPC |
| **Anthropic Claude Code** | ✅ | ✅ | ⌨️ CLI Terminal & Headless |
| **Cursor** | ✅ | ✅ | ⌨️ Auto Composer / Chat Input |
| **Windsurf (Codeium)** | ✅ | ✅ | ⌨️ Cascade Chat Input |
| **VS Code (Cline / Roo / Copilot)** | ✅ | ✅ | ⌨️ Chat View Automation |
| **Aider / Goose / OpenHands** | ✅ | ✅ | ⌨️ CLI Stdin & `[y/n]` Choices |
| **JetBrains (IntelliJ, PyCharm)** | ✅ | ✅ | ⌨️ Window Focus & Notifications |

---

## 🛡️ Smart Interruption Guards

* 🎮 **Gaming & Fullscreen Guard**: If you are playing a game or watching a fullscreen video, SwitchBack **suppresses window switching** and sends a subtle notification toast instead.
* 📞 **Meeting Guard**: Protects active **Zoom, Google Meet, Microsoft Teams, Discord, and Slack Huddles** from being interrupted by window focus changes.
* 📱 **Mobile Primary Mode**: Keep working on your PC without window switching while managing the agent exclusively from your phone.

---

## 🚀 Quick Start (Installation)

### Option 1: Standalone Setup Installer (Recommended)
1. Download **[`Setup.exe`](https://github.com/JAIstudio-source/Switchback/releases/latest/download/Setup.exe)** from the latest release.
2. Run `Setup.exe` — it installs SwitchBack to your system, creates Start Menu & Desktop shortcuts, and sets up agent hooks automatically.

### Option 2: Portable Zip
1. Download **`SwitchBack-Windows-x64.zip`**.
2. Extract anywhere and double-click **`Launch UI.bat`** (or run `Install.bat`).

---

## 🖥️ Using the Pixel Dashboard

Double-click the **SwitchBack** desktop shortcut or run `switchback ui` in any terminal:

1. **Select Agent**: Pick your active AI agent window (Antigravity, Claude, Cursor, etc.).
2. **Select Work / Media**: Pick your video player, browser, or game.
3. **Toggle Modes**: Enable Media Control, Fullscreen Guard, or Gaming Mode.
4. **Scan QR / Mobile Link**: Open the mobile remote on your phone.

---

## ⌨️ CLI Commands

For terminal power-users:

```powershell
# Launch the Web & Mobile Dashboard
switchback ui

# Show status, detected windows & active sessions
switchback status

# Install hooks into Antigravity and Claude Code
switchback install

# Pause or resume auto-switching
switchback stop
switchback resume

# Remove hooks and restore default behavior
switchback uninstall
```

---

## 📄 License

Distributed under the **MIT License**. Free and open-source for personal and commercial use.

---

## 🔏 Code Signing

Free code signing provided by [SignPath.io](https://signpath.io), certificate by [SignPath Foundation](https://signpath.org).
