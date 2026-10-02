# 🛡️ focusmgr Antivirus & Publishing Guide: Resolving False Positives Once and for All

This guide explains why Windows Defender flagged `focusmgr.exe` as `Behavior:Win32/DefenseEvasion.A!ml`, the architectural changes already implemented in the code to eliminate those heuristic triggers, and the exact steps to publish and distribute your app without security warnings.

---

## 1. What Triggered the Alert & What Was Fixed in Code

### The Detection: `Behavior:Win32/DefenseEvasion.A!ml`
The `!ml` tag stands for **Machine Learning Heuristic Detection**. It was triggered because the original binary performed several suspicious low-level actions simultaneously:

1. **`AttachToInputDesktop()` in `init()`**:
   - Calling `OpenInputDesktop` with `0x0100` (`DESKTOP_SWITCHDESKTOP`) immediately at entrypoint.
   - **Fix Applied**: Removed from `init()`. Switched to standard `0x0081` (`DESKTOP_READOBJECTS | DESKTOP_WRITEOBJECTS`) called on-demand only.
2. **Global System Lock Alteration**:
   - Calling `SystemParametersInfoW` with `SPI_SETFOREGROUNDLOCKTIMEOUT` set to `0` changed system-wide focus settings.
   - **Fix Applied**: Completely removed. No system-wide parameters are touched.
3. **Simulated Synthetic Keystrokes**:
   - Injecting fake `VK_MENU` (Alt key) down/up events via `keybd_event` to trick Windows focus filters.
   - **Fix Applied**: Completely removed. Replaced with the official Win32 `SwitchToThisWindow` and clean thread-input queue coupling.
4. **Invalid / Stale HWNDs and Hook Prefix**:
   - Stale HWNDs from closed browser tabs caused focus switches to fail silently.
   - Antigravity hooks had an erroneous `call` prefix which broke non-cmd runners.
   - **Fix Applied**: Added dynamic window title & process matching fallback and clean executable hook paths.

---

## 2. Local Development: Immediate 1-Click Fix

For local testing while compiling and modifying the source code:

1. Right-click **`setup_dev_env.bat`** in the project folder and select **Run as Administrator** (or double click and grant UAC elevation).
2. It executes:
   ```powershell
   Add-MpPreference -ExclusionPath "D:\Download\focusmgr"
   ```
3. This prevents Windows Defender from interfering with intermediate builds in your workspace.

---

## 3. Permanent Fix Before Publishing Your App

When releasing software to public users, Windows Defender and SmartScreen use **reputation and digital identity** to determine whether an executable is safe. Follow these four industry standards to guarantee clean downloads:

### Step 1: Add Windows PE Version Information & Manifest
Antivirus engines scrutinize raw Go binaries because they lack standard Windows version resource metadata (Company Name, File Version, Copyright, Product Name).

1. Install `go-winres`:
   ```bash
   go install github.com/tc-hib/go-winres@latest
   ```
2. Initialize and configure a `winres.json` with your app details (App name, version, icon, publisher).
3. Generate the Windows resource object file:
   ```bash
   go-winres make
   ```
4. Build `focusmgr.exe`:
   ```bash
   go build -ldflags "-s -w" -o focusmgr.exe ./cmd/focusmgr
   ```

### Step 2: Code Signing with an Authenticode Certificate
An unsigned `.exe` downloaded from the internet will almost always trigger Windows SmartScreen warnings ("Windows protected your PC").

* **For Open Source projects**: Apply for free code signing via [SignPath Foundation](https://about.signpath.io/open-source).
* **For Commercial projects**: Purchase a Standard OV or EV Code Signing Certificate (from DigiCert, Sectigo, or SSL.com).
* **Sign the binary using Microsoft SignTool**:
  ```powershell
  signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /a "D:\Download\focusmgr\focusmgr.exe"
  ```

### Step 3: Submit to Microsoft Security Intelligence (White-Listing)
Before announcing a release, submit your compiled executable directly to Microsoft's Defender research team:

1. Visit the official portal:  
   🔗 **[Microsoft Security Intelligence Sample Submission](https://www.microsoft.com/en-us/wdsi/filesubmission)**
2. Select **"Software Developer"** as your user type.
3. Upload `focusmgr.exe`.
4. Enter:
   - **Product Name**: `focusmgr`
   - **Company Name / Author**: Your GitHub / Developer name.
   - **Comments**: *"Clean open-source utility for managing AI agent window focus using standard Win32 APIs (SetForegroundWindow, SwitchToThisWindow). False positive heuristic detection under Behavior:Win32/DefenseEvasion.A!ml."*
5. Within 2 to 24 hours, Microsoft updates their cloud definitions to automatically whitelist your binary.

### Step 4: Packaging and Distribution
Do not distribute loose `.exe` files in zip archives without an installer if you want to avoid SmartScreen prompts. Recommended methods:

* **Inno Setup / NSIS**: Package `focusmgr` with an installer that registers uninstall entries in Windows Settings.
* **Winget (Windows Package Manager)**: Submit your release to the [Microsoft Winget Community Repository](https://github.com/microsoft/winget-pkgs). Winget automatically scans releases through Microsoft's validation pipelines.
* **GitHub Releases**: Always include the SHA256 checksum and link to the source code repository in release notes.
