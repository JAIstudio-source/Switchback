package main

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

//go:embed icon.png
var iconBytes []byte

const (
	AppName    = "SwitchBack"
	AppVersion = "1.1.0"
	Publisher  = "JAIstudio"
	RepoURL    = "https://github.com/JAIstudio-source/Switchback"

	MB_OK               = 0x00000000
	MB_YESNO            = 0x00000004
	MB_ICONINFORMATION  = 0x00000040
	MB_ICONQUESTION     = 0x00000020
	MB_ICONERROR        = 0x00000010
	IDYES               = 6
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW  = user32.NewProc("MessageBoxW")
)

func messageBox(title, message string, flags uint32) int {
	pTitle, _ := syscall.UTF16PtrFromString(title)
	pMsg, _ := syscall.UTF16PtrFromString(message)
	ret, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(pMsg)), uintptr(unsafe.Pointer(pTitle)), uintptr(flags))
	return int(ret)
}

func main() {
	// 1. Welcome prompt
	welcomeMsg := fmt.Sprintf("Welcome to %s Setup (v%s)!\n\nThis will install %s on your computer:\n • Location: %%LOCALAPPDATA%%\\SwitchBack\n • Desktop & Start Menu Shortcuts\n • Global 'switchback' terminal command\n • Auto-configuration for Antigravity & Claude Code\n\nDo you want to continue?", AppName, AppVersion, AppName)
	if messageBox(AppName+" Setup", welcomeMsg, MB_YESNO|MB_ICONQUESTION) != IDYES {
		os.Exit(0)
	}

	// 2. Resolve Installation Directory
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	installDir := filepath.Join(localAppData, AppName)
	_ = os.MkdirAll(installDir, 0755)

	// 3. Stop running instance if any
	_ = exec.Command("taskkill", "/F", "/IM", "switchback.exe").Run()
	time.Sleep(500 * time.Millisecond)

	// 4. Copy or download switchback.exe
	targetExe := filepath.Join(installDir, "switchback.exe")
	exeDir, _ := os.Executable()
	localSiblingExe := filepath.Join(filepath.Dir(exeDir), "switchback.exe")

	if _, err := os.Stat(localSiblingExe); err == nil {
		data, err := os.ReadFile(localSiblingExe)
		if err == nil {
			_ = os.WriteFile(targetExe, data, 0755)
		}
	} else {
		// Download latest release binary from GitHub
		downloadURL := fmt.Sprintf("%s/releases/latest/download/switchback.exe", RepoURL)
		resp, err := http.Get(downloadURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			messageBox(AppName+" Setup Error", fmt.Sprintf("Could not locate or download switchback.exe.\nPlease ensure an internet connection is available or switchback.exe is in the same folder.\n\nError: %v", err), MB_OK|MB_ICONERROR)
			os.Exit(1)
		}
		defer resp.Body.Close()
		out, err := os.Create(targetExe)
		if err == nil {
			_, _ = io.Copy(out, resp.Body)
			out.Close()
		}
	}

	// 5. Write icon and Launch script
	targetIcon := filepath.Join(installDir, "icon.png")
	if len(iconBytes) > 0 {
		_ = os.WriteFile(targetIcon, iconBytes, 0644)
	}
	targetBat := filepath.Join(installDir, "Launch UI.bat")
	_ = os.WriteFile(targetBat, []byte("@echo off\r\ntitle SwitchBack - Pixel UI Launcher\r\nstart \"\" \"%~dp0switchback.exe\" ui\r\n"), 0644)

	// 6. Create Shortcuts & Configure Registry via PowerShell helper
	psScript := fmt.Sprintf(`
$InstallDir = "%s"
$WshShell = New-Object -ComObject WScript.Shell

# Desktop Shortcut
$DesktopPath = [Environment]::GetFolderPath("Desktop")
$Shortcut = $WshShell.CreateShortcut((Join-Path $DesktopPath "SwitchBack.lnk"))
$Shortcut.TargetPath = (Join-Path $InstallDir "switchback.exe")
$Shortcut.Arguments = "ui"
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.Description = "SwitchBack - AI Agent Focus & Mobile Remote"
$Shortcut.IconLocation = (Join-Path $InstallDir "icon.png")
$Shortcut.Save()

# Start Menu Shortcut
$StartMenuPath = [Environment]::GetFolderPath("Programs")
$StartShortcut = $WshShell.CreateShortcut((Join-Path $StartMenuPath "SwitchBack.lnk"))
$StartShortcut.TargetPath = (Join-Path $InstallDir "switchback.exe")
$StartShortcut.Arguments = "ui"
$StartShortcut.WorkingDirectory = $InstallDir
$StartShortcut.Description = "SwitchBack - AI Agent Focus & Mobile Remote"
$StartShortcut.IconLocation = (Join-Path $InstallDir "icon.png")
$StartShortcut.Save()

# Add to User PATH
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
if ($CurrentPath -notlike "*$InstallDir*") {
    $NewPath = if ($CurrentPath) { "$CurrentPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, [EnvironmentVariableTarget]::User)
}

# Register Uninstaller in Windows Registry
$RegKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack"
if (-not (Test-Path $RegKey)) { New-Item -Path $RegKey -Force | Out-Null }
Set-ItemProperty -Path $RegKey -Name "DisplayName" -Value "SwitchBack"
Set-ItemProperty -Path $RegKey -Name "DisplayVersion" -Value "%s"
Set-ItemProperty -Path $RegKey -Name "Publisher" -Value "%s"
Set-ItemProperty -Path $RegKey -Name "InstallLocation" -Value $InstallDir
Set-ItemProperty -Path $RegKey -Name "UninstallString" -Value "powershell.exe -NoProfile -Command \"& '$InstallDir\switchback.exe' uninstall; Remove-Item '$([Environment]::GetFolderPath('Desktop'))\SwitchBack.lnk' -Force; Remove-Item '$env:APPDATA\Microsoft\Windows\Start Menu\Programs\SwitchBack.lnk' -Force; Remove-Item 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack' -Recurse -Force; Write-Host 'Uninstalled.'\""
Set-ItemProperty -Path $RegKey -Name "DisplayIcon" -Value (Join-Path $InstallDir "switchback.exe")
Set-ItemProperty -Path $RegKey -Name "HelpLink" -Value "%s"
`, strings.ReplaceAll(installDir, `\`, `\\`), AppVersion, Publisher, RepoURL)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()

	// 7. Run agent hooks configuration
	hookCmd := exec.Command(targetExe, "install")
	hookCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = hookCmd.Run()

	// 8. Finish prompt & Option to launch
	finishMsg := fmt.Sprintf("Installation completed successfully!\n\n%s is now ready to use.\n • Desktop shortcut created\n • Start Menu shortcut created\n • Global command 'switchback' enabled\n\nWould you like to launch SwitchBack now?", AppName)
	if messageBox(AppName+" Setup Complete", finishMsg, MB_YESNO|MB_ICONINFORMATION) == IDYES {
		launchCmd := exec.Command(targetExe, "ui")
		_ = launchCmd.Start()
	}
}
