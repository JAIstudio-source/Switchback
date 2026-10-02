package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed installer.html
var installerHTML []byte

//go:embed icon.png
var iconPNGBytes []byte

//go:embed icon.ico
var iconICOBytes []byte

const (
	AppName    = "SwitchBack"
	AppVersion = "1.1.0"
	Publisher  = "JAIstudio"
	RepoURL    = "https://github.com/JAIstudio-source/Switchback"
)

func main() {
	// 1. Setup local HTTP server on random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Failed to bind port: %v\n", err)
		return
	}
	port := listener.Addr().(*net.TCPAddr).Port
	installerURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(installerHTML)
	})

	mux.HandleFunc("/icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(iconPNGBytes)
	})

	mux.HandleFunc("/api/install", handleInstallRequest)
	mux.HandleFunc("/api/launch", handleLaunchRequest)

	server := &http.Server{Handler: mux}

	go func() {
		_ = server.Serve(listener)
	}()

	// 2. Open in sleek borderless app mode if Chrome/Edge is available, otherwise default browser
	go func() {
		time.Sleep(150 * time.Millisecond)
		openAppWindow(installerURL)
	}()

	// Keep alive until closed or install finishes
	select {}
}

func openAppWindow(url string) {
	// Try Edge App Mode (default on all Windows 10/11)
	edgePaths := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, p := range edgePaths {
		if _, err := os.Stat(p); err == nil {
			cmd := exec.Command(p, fmt.Sprintf("--app=%s", url), "--window-size=520,640", "--disable-features=Translate")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if err := cmd.Start(); err == nil {
				return
			}
		}
	}

	// Try Chrome App Mode
	chromePath := `C:\Program Files\Google\Chrome\Application\chrome.exe`
	if _, err := os.Stat(chromePath); err == nil {
		cmd := exec.Command(chromePath, fmt.Sprintf("--app=%s", url), "--window-size=520,640")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Start(); err == nil {
			return
		}
	}

	// Fallback to default browser
	cmd := exec.Command("cmd", "/c", "start", "", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}

func handleInstallRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	installDir := filepath.Join(localAppData, AppName)
	_ = os.MkdirAll(installDir, 0755)

	// Stop running instance if any
	_ = exec.Command("taskkill", "/F", "/IM", "switchback.exe").Run()
	time.Sleep(300 * time.Millisecond)

	// Copy or download switchback.exe
	targetExe := filepath.Join(installDir, "switchback.exe")
	exeDir, _ := os.Executable()
	localSiblingExe := filepath.Join(filepath.Dir(exeDir), "switchback.exe")

	if _, err := os.Stat(localSiblingExe); err == nil {
		data, err := os.ReadFile(localSiblingExe)
		if err == nil {
			_ = os.WriteFile(targetExe, data, 0755)
		}
	} else {
		downloadURL := fmt.Sprintf("%s/releases/latest/download/switchback.exe", RepoURL)
		resp, err := http.Get(downloadURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			out, err := os.Create(targetExe)
			if err == nil {
				_, _ = io.Copy(out, resp.Body)
				out.Close()
			}
		}
	}

	// Save icon.ico and icon.png
	targetICO := filepath.Join(installDir, "icon.ico")
	if len(iconICOBytes) > 0 {
		_ = os.WriteFile(targetICO, iconICOBytes, 0644)
	}
	targetPNG := filepath.Join(installDir, "icon.png")
	if len(iconPNGBytes) > 0 {
		_ = os.WriteFile(targetPNG, iconPNGBytes, 0644)
	}

	// Save Launch UI.bat
	targetBat := filepath.Join(installDir, "Launch UI.bat")
	_ = os.WriteFile(targetBat, []byte("@echo off\r\ntitle SwitchBack - Pixel UI Launcher\r\nstart \"\" \"%~dp0switchback.exe\" ui\r\n"), 0644)

	// Run PowerShell script for shortcuts with real icon.ico and Shell Refresh
	psScript := fmt.Sprintf(`
$InstallDir = "%s"
$WshShell = New-Object -ComObject WScript.Shell

# 1. Desktop Shortcut
$DesktopPath = [Environment]::GetFolderPath("Desktop")
$Shortcut = $WshShell.CreateShortcut((Join-Path $DesktopPath "SwitchBack.lnk"))
$Shortcut.TargetPath = (Join-Path $InstallDir "switchback.exe")
$Shortcut.Arguments = "ui"
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "SwitchBack - AI Agent Focus & Mobile Remote"
$Shortcut.IconLocation = "$InstallDir\icon.ico,0"
$Shortcut.Save()

# 2. Start Menu Shortcut (Registered and Indexed in Windows Search)
$StartMenuPath = [Environment]::GetFolderPath("Programs")
$StartShortcut = $WshShell.CreateShortcut((Join-Path $StartMenuPath "SwitchBack.lnk"))
$StartShortcut.TargetPath = (Join-Path $InstallDir "switchback.exe")
$StartShortcut.Arguments = "ui"
$StartShortcut.WorkingDirectory = $InstallDir
$StartShortcut.Description = "SwitchBack - AI Agent Focus & Mobile Remote"
$StartShortcut.IconLocation = "$InstallDir\icon.ico,0"
$StartShortcut.Save()

# 3. Add to User PATH
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
if ($CurrentPath -notlike "*$InstallDir*") {
    $NewPath = if ($CurrentPath) { "$CurrentPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, [EnvironmentVariableTarget]::User)
}

# 4. Register Uninstaller in Windows Registry
$RegKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack"
if (-not (Test-Path $RegKey)) { New-Item -Path $RegKey -Force | Out-Null }
Set-ItemProperty -Path $RegKey -Name "DisplayName" -Value "SwitchBack"
Set-ItemProperty -Path $RegKey -Name "DisplayVersion" -Value "%s"
Set-ItemProperty -Path $RegKey -Name "Publisher" -Value "%s"
Set-ItemProperty -Path $RegKey -Name "InstallLocation" -Value $InstallDir
Set-ItemProperty -Path $RegKey -Name "UninstallString" -Value "powershell.exe -NoProfile -Command \"& '$InstallDir\switchback.exe' uninstall; Remove-Item '$([Environment]::GetFolderPath('Desktop'))\SwitchBack.lnk' -Force; Remove-Item '$env:APPDATA\Microsoft\Windows\Start Menu\Programs\SwitchBack.lnk' -Force; Remove-Item 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack' -Recurse -Force; Write-Host 'Uninstalled.'\""
Set-ItemProperty -Path $RegKey -Name "DisplayIcon" -Value (Join-Path $InstallDir "icon.ico")
Set-ItemProperty -Path $RegKey -Name "HelpLink" -Value "%s"

# 5. Flush Windows Shell Icon Cache & Windows Search Notification
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public class ShellHelper {
    [DllImport("shell32.dll", CharSet = CharSet.Auto, SetLastError = true)]
    public static extern void SHChangeNotify(uint wEventId, uint uFlags, IntPtr dwItem1, IntPtr dwItem2);
}
"@
[ShellHelper]::SHChangeNotify(0x08000000, 0x0000, [IntPtr]::Zero, [IntPtr]::Zero)
`, strings.ReplaceAll(installDir, `\`, `\\`), AppVersion, Publisher, RepoURL)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()

	// Configure Agent Hooks
	hookCmd := exec.Command(targetExe, "install")
	hookCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = hookCmd.Run()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "SwitchBack successfully installed!",
	})
}

func handleLaunchRequest(w http.ResponseWriter, r *http.Request) {
	localAppData := os.Getenv("LOCALAPPDATA")
	targetExe := filepath.Join(localAppData, AppName, "switchback.exe")

	cmd := exec.Command(targetExe, "ui")
	_ = cmd.Start()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}
