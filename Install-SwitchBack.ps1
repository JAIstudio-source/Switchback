# ==============================================================================
# SwitchBack - Modern Windows Installer Script
# https://github.com/JAIstudio-source/Switchback
# ==============================================================================

$ErrorActionPreference = "Stop"
$AppName = "SwitchBack"
$AppVersion = "1.1.0"
$Publisher = "JAIstudio"
$RepoUrl = "https://github.com/JAIstudio-source/Switchback"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
if (-not $ScriptDir) { $ScriptDir = Get-Location }

$InstallDir = Join-Path $env:LOCALAPPDATA "SwitchBack"
$SourceExe = Join-Path $ScriptDir "switchback.exe"
$SourceIcon = Join-Path $ScriptDir "icon.png"
$SourceBatch = Join-Path $ScriptDir "Launch UI.bat"

Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "       SwitchBack Installer & Environment Setup        " -ForegroundColor Yellow
Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host " Target Directory: $InstallDir" -ForegroundColor Gray

# 1. Create Target Directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# 2. Stop running instance if any
$running = Get-Process -Name "switchback" -ErrorAction SilentlyContinue
if ($running) {
    Write-Host "[*] Stopping running SwitchBack process..." -ForegroundColor Yellow
    $running | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 800
}

# 3. Copy Application Files
Write-Host "[*] Copying binary and application assets..." -ForegroundColor Cyan
if (Test-Path $SourceExe) {
    Copy-Item -Path $SourceExe -Destination (Join-Path $InstallDir "switchback.exe") -Force
} else {
    Write-Host "[!] switchback.exe not found in $ScriptDir. Downloading latest release..." -ForegroundColor Yellow
    $downloadUrl = "$RepoUrl/releases/latest/download/switchback.exe"
    Invoke-WebRequest -Uri $downloadUrl -OutFile (Join-Path $InstallDir "switchback.exe") -UseBasicParsing
}

if (Test-Path (Join-Path $ScriptDir "icon.ico")) {
    Copy-Item -Path (Join-Path $ScriptDir "icon.ico") -Destination (Join-Path $InstallDir "icon.ico") -Force
}

if (Test-Path $SourceIcon) {
    Copy-Item -Path $SourceIcon -Destination (Join-Path $InstallDir "icon.png") -Force
}

if (Test-Path $SourceBatch) {
    Copy-Item -Path $SourceBatch -Destination (Join-Path $InstallDir "Launch UI.bat") -Force
}

# 4. Generate Uninstaller Script
$UninstallScript = Join-Path $InstallDir "uninstall.ps1"
$UninstallScriptContent = @"
`$ErrorActionPreference = "SilentlyContinue"
Write-Host "Uninstalling SwitchBack..." -ForegroundColor Yellow

# Stop process
Get-Process -Name "switchback" | Stop-Process -Force

# Remove hooks
& "$InstallDir\switchback.exe" uninstall

# Remove shortcuts
Remove-Item "$([Environment]::GetFolderPath('Desktop'))\SwitchBack.lnk" -Force
Remove-Item "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\SwitchBack.lnk" -Force

# Remove PATH
`$UserPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
if (`$UserPath -like "*$InstallDir*") {
    `$NewPath = (`$UserPath.Split(';') | Where-Object { `$_ -ne "$InstallDir" -and `$_ -ne "" }) -join ';'
    [Environment]::SetEnvironmentVariable("PATH", `$NewPath, [EnvironmentVariableTarget]::User)
}

# Remove Registry
Remove-Item "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack" -Recurse -Force

Write-Host "SwitchBack has been completely uninstalled." -ForegroundColor Green
"@
Set-Content -Path $UninstallScript -Value $UninstallScriptContent -Force

# 5. Create Desktop and Start Menu Shortcuts
Write-Host "[*] Creating Desktop and Start Menu shortcuts..." -ForegroundColor Cyan
$WshShell = New-Object -ComObject WScript.Shell
$IcoPath = Join-Path $InstallDir "icon.ico"

# Desktop Shortcut
$DesktopPath = [Environment]::GetFolderPath("Desktop")
$DesktopShortcut = $WshShell.CreateShortcut((Join-Path $DesktopPath "SwitchBack.lnk"))
$DesktopShortcut.TargetPath = (Join-Path $InstallDir "switchback.exe")
$DesktopShortcut.Arguments = "ui"
$DesktopShortcut.WorkingDirectory = $InstallDir
$DesktopShortcut.Description = "SwitchBack - AI Agent Focus & Mobile Remote"
if (Test-Path $IcoPath) {
    $DesktopShortcut.IconLocation = "$IcoPath,0"
}
$DesktopShortcut.Save()

# Start Menu Shortcut (Indexed by Windows Search)
$StartMenuPath = [Environment]::GetFolderPath("Programs")
$StartShortcut = $WshShell.CreateShortcut((Join-Path $StartMenuPath "SwitchBack.lnk"))
$StartShortcut.TargetPath = (Join-Path $InstallDir "switchback.exe")
$StartShortcut.Arguments = "ui"
$StartShortcut.WorkingDirectory = $InstallDir
$StartShortcut.Description = "SwitchBack - AI Agent Focus & Mobile Remote"
if (Test-Path $IcoPath) {
    $StartShortcut.IconLocation = "$IcoPath,0"
}
$StartShortcut.Save()

# 6. Add to User PATH
Write-Host "[*] Adding SwitchBack to user PATH environment variable..." -ForegroundColor Cyan
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
if ($CurrentPath -notlike "*$InstallDir*") {
    $NewPath = if ($CurrentPath) { "$CurrentPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, [EnvironmentVariableTarget]::User)
    $env:PATH = "$env:PATH;$InstallDir"
    Write-Host "[OK] Added to PATH. 'switchback' command is now available everywhere!" -ForegroundColor Green
}

# 7. Register in Windows Add/Remove Programs (Control Panel & Settings)
Write-Host "[*] Registering in Windows Add/Remove Programs..." -ForegroundColor Cyan
$UninstallRegKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack"
if (-not (Test-Path $UninstallRegKey)) {
    New-Item -Path $UninstallRegKey -Force | Out-Null
}
Set-ItemProperty -Path $UninstallRegKey -Name "DisplayName" -Value "SwitchBack"
Set-ItemProperty -Path $UninstallRegKey -Name "DisplayVersion" -Value $AppVersion
Set-ItemProperty -Path $UninstallRegKey -Name "Publisher" -Value $Publisher
Set-ItemProperty -Path $UninstallRegKey -Name "InstallLocation" -Value $InstallDir
Set-ItemProperty -Path $UninstallRegKey -Name "UninstallString" -Value "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `"$UninstallScript`""
Set-ItemProperty -Path $UninstallRegKey -Name "DisplayIcon" -Value (Join-Path $InstallDir "icon.ico")
Set-ItemProperty -Path $UninstallRegKey -Name "HelpLink" -Value $RepoUrl
Set-ItemProperty -Path $UninstallRegKey -Name "NoModify" -Value 1 -Type DWord
Set-ItemProperty -Path $UninstallRegKey -Name "NoRepair" -Value 1 -Type DWord

# 8. Refresh Windows Shell & Icon Cache
Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public class ShellHelper {
    [DllImport("shell32.dll", CharSet = CharSet.Auto, SetLastError = true)]
    public static extern void SHChangeNotify(uint wEventId, uint uFlags, IntPtr dwItem1, IntPtr dwItem2);
}
"@
[ShellHelper]::SHChangeNotify(0x08000000, 0x0000, [IntPtr]::Zero, [IntPtr]::Zero)

# 9. Configure AI Agent Hooks
Write-Host "[*] Configuring agent hooks for Antigravity IDE and Claude Code..." -ForegroundColor Cyan
& (Join-Path $InstallDir "switchback.exe") install | Out-Null

Write-Host "`n=======================================================" -ForegroundColor Green
Write-Host "       SwitchBack Installed Successfully (v$AppVersion)!       " -ForegroundColor Yellow
Write-Host "=======================================================" -ForegroundColor Green
Write-Host " 🚀 Desktop Shortcut: Created on Desktop" -ForegroundColor White
Write-Host " 💻 Start Menu:       Available in Windows Start Menu" -ForegroundColor White
Write-Host " ⚡ Terminal Command: Type 'switchback' or 'switchback ui'" -ForegroundColor White
Write-Host "=======================================================" -ForegroundColor Green
