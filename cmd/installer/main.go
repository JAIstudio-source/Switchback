package main

import (
	"bytes"
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

//go:embed README.md
var readmeBytes []byte

const (
	AppName    = "SwitchBack"
	AppVersion = "1.2.0"
	Publisher  = "JAIstudio"
	RepoURL    = "https://github.com/JAIstudio-source/Switchback"
)

type InstallPayload struct {
	InstallPath string `json:"installPath"`
}

var (
	shell32DLL         = syscall.NewLazyDLL("shell32.dll")
	procSHChangeNotify = shell32DLL.NewProc("SHChangeNotify")
)

func refreshShellIcons() {
	// SHCNE_ASSOCCHANGED = 0x08000000, SHCNF_IDLIST = 0x0000
	_, _, _ = procSHChangeNotify.Call(0x08000000, 0, 0, 0)
}

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

	mux.HandleFunc("/icon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write(iconICOBytes)
	})

	mux.HandleFunc("/api/default-path", handleDefaultPathRequest)
	mux.HandleFunc("/api/browse", handleBrowseRequest)
	mux.HandleFunc("/api/install", handleInstallRequest)
	mux.HandleFunc("/api/launch", handleLaunchRequest)
	mux.HandleFunc("/api/close", handleCloseRequest)

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

func getDefaultInstallDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(localAppData, AppName)
}

func openAppWindow(url string) {
	// Try Edge App Mode (default on Windows 10/11)
	edgePaths := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	}
	for _, p := range edgePaths {
		if _, err := os.Stat(p); err == nil {
			cmd := exec.Command(p, fmt.Sprintf("--app=%s", url), "--window-size=540,680", "--disable-features=Translate")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if err := cmd.Start(); err == nil {
				return
			}
		}
	}

	// Try Chrome App Mode
	chromePath := `C:\Program Files\Google\Chrome\Application\chrome.exe`
	if _, err := os.Stat(chromePath); err == nil {
		cmd := exec.Command(chromePath, fmt.Sprintf("--app=%s", url), "--window-size=540,680")
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

func handleDefaultPathRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"path": getDefaultInstallDir(),
	})
}

func handleBrowseRequest(w http.ResponseWriter, r *http.Request) {
	// Call Windows Folder Browser Dialog via PowerShell STA
	psScript := "Add-Type -AssemblyName System.Windows.Forms; $f = New-Object System.Windows.Forms.FolderBrowserDialog; $f.Description = 'Select SwitchBack Installation Folder'; $f.ShowNewFolderButton = $true; if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $f.SelectedPath }"
	cmd := exec.Command("powershell", "-NoProfile", "-Sta", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()

	selectedPath := strings.TrimSpace(out.String())
	if selectedPath != "" {
		// If user selected a directory not ending with SwitchBack, append SwitchBack for convenience
		if !strings.EqualFold(filepath.Base(selectedPath), AppName) {
			selectedPath = filepath.Join(selectedPath, AppName)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"path": selectedPath,
	})
}

var lastInstallDir string

func handleInstallRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload InstallPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)

	installDir := strings.TrimSpace(payload.InstallPath)
	if installDir == "" {
		installDir = getDefaultInstallDir()
	}
	installDir, _ = filepath.Abs(installDir)
	lastInstallDir = installDir

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

	// Save README.md guide
	targetReadme := filepath.Join(installDir, "README.md")
	if len(readmeBytes) > 0 {
		_ = os.WriteFile(targetReadme, readmeBytes, 0644)
	}

	// Save Launch UI.bat
	targetBat := filepath.Join(installDir, "Launch UI.bat")
	_ = os.WriteFile(targetBat, []byte("@echo off\r\ntitle SwitchBack - Launcher\r\nstart \"\" \"%~dp0switchback.exe\" ui\r\n"), 0644)

	// Save uninstaller script and uninstaller batch file
	uninstallPs1 := fmt.Sprintf(`$ErrorActionPreference = "SilentlyContinue"
Write-Host "Uninstalling SwitchBack..." -ForegroundColor Yellow
$InstallDir = "%s"

# Stop running process
Get-Process -Name "switchback" | Stop-Process -Force

# Disconnect hooks
if (Test-Path "$InstallDir\switchback.exe") {
    & "$InstallDir\switchback.exe" uninstall
}

# Remove shortcuts
Remove-Item "$([Environment]::GetFolderPath('Desktop'))\SwitchBack.lnk" -Force
Remove-Item "$([Environment]::GetFolderPath('Programs'))\SwitchBack.lnk" -Force
Remove-Item "$([Environment]::GetFolderPath('Programs'))\Uninstall SwitchBack.lnk" -Force

# Remove PATH
$UserPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
if ($UserPath -like "*$InstallDir*") {
    $NewPath = ($UserPath.Split(';') | Where-Object { $_ -ne "$InstallDir" -and $_ -ne "" }) -join ';'
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, [EnvironmentVariableTarget]::User)
}

# Remove Registry
Remove-Item "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack" -Recurse -Force

# Schedule folder cleanup
Start-Process -FilePath "cmd.exe" -ArgumentList ("/c timeout /t 1 /nobreak >nul & rd /s /q """ + $InstallDir + """") -WindowStyle Hidden
Write-Host "SwitchBack has been completely uninstalled." -ForegroundColor Green
`, strings.ReplaceAll(installDir, `\`, `\\`))

	_ = os.WriteFile(filepath.Join(installDir, "uninstall.ps1"), []byte(uninstallPs1), 0644)
	_ = os.WriteFile(filepath.Join(installDir, "Uninstall.bat"), []byte("@echo off\r\npowershell.exe -NoProfile -ExecutionPolicy Bypass -File \"%~dp0uninstall.ps1\"\r\npause\r\n"), 0644)

	// Run PowerShell script for shortcuts with real icon.ico and registry registration
	psScript := fmt.Sprintf(`
$InstallDir = "%s"
$WshShell = New-Object -ComObject WScript.Shell
$IcoPath = Join-Path $InstallDir "icon.ico"

# 1. Desktop Shortcut
$DesktopPath = [Environment]::GetFolderPath("Desktop")
$Shortcut = $WshShell.CreateShortcut((Join-Path $DesktopPath "SwitchBack.lnk"))
$Shortcut.TargetPath = (Join-Path $InstallDir "switchback.exe")
$Shortcut.Arguments = "ui"
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "SwitchBack - AI Agent Focus & Mobile Remote"
if (Test-Path $IcoPath) { $Shortcut.IconLocation = "$IcoPath,0" }
$Shortcut.Save()

# 2. Start Menu Shortcut (Indexed in Windows Search)
$StartMenuPath = [Environment]::GetFolderPath("Programs")
$StartShortcut = $WshShell.CreateShortcut((Join-Path $StartMenuPath "SwitchBack.lnk"))
$StartShortcut.TargetPath = (Join-Path $InstallDir "switchback.exe")
$StartShortcut.Arguments = "ui"
$StartShortcut.WorkingDirectory = $InstallDir
$StartShortcut.Description = "SwitchBack - AI Agent Focus & Mobile Remote"
if (Test-Path $IcoPath) { $StartShortcut.IconLocation = "$IcoPath,0" }
$StartShortcut.Save()

# 3. Start Menu Uninstall Shortcut
$StartUninstallShortcut = $WshShell.CreateShortcut((Join-Path $StartMenuPath "Uninstall SwitchBack.lnk"))
$StartUninstallShortcut.TargetPath = (Join-Path $InstallDir "Uninstall.bat")
$StartUninstallShortcut.WorkingDirectory = $InstallDir
$StartUninstallShortcut.Description = "Uninstall SwitchBack"
if (Test-Path $IcoPath) { $StartUninstallShortcut.IconLocation = "$IcoPath,0" }
$StartUninstallShortcut.Save()

# 4. Add to User PATH
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
if ($CurrentPath -notlike "*$InstallDir*") {
    $NewPath = if ($CurrentPath) { "$CurrentPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, [EnvironmentVariableTarget]::User)
}

# 5. Register in Windows Add/Remove Programs
$RegKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack"
if (-not (Test-Path $RegKey)) { New-Item -Path $RegKey -Force | Out-Null }
$UninstallExe = Join-Path $InstallDir "Uninstall.bat"
Set-ItemProperty -Path $RegKey -Name "DisplayName" -Value "SwitchBack"
Set-ItemProperty -Path $RegKey -Name "DisplayVersion" -Value "%s"
Set-ItemProperty -Path $RegKey -Name "Publisher" -Value "%s"
Set-ItemProperty -Path $RegKey -Name "InstallLocation" -Value $InstallDir
Set-ItemProperty -Path $RegKey -Name 'UninstallString' -Value ('"' + $UninstallExe + '"')
Set-ItemProperty -Path $RegKey -Name "DisplayIcon" -Value (Join-Path $InstallDir "icon.ico")
Set-ItemProperty -Path $RegKey -Name "HelpLink" -Value "%s"
Set-ItemProperty -Path $RegKey -Name "NoModify" -Value 1 -Type DWord
Set-ItemProperty -Path $RegKey -Name "NoRepair" -Value 1 -Type DWord
`, strings.ReplaceAll(installDir, `\`, `\\`), AppVersion, Publisher, RepoURL)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()

	// Flush Windows Shell Icon Cache via native Win32 DLL
	refreshShellIcons()

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
	installDir := lastInstallDir
	if installDir == "" {
		installDir = getDefaultInstallDir()
	}
	targetExe := filepath.Join(installDir, "switchback.exe")

	cmd := exec.Command(targetExe, "ui")
	_ = cmd.Start()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

func handleCloseRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
}
