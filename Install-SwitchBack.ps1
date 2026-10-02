# ==============================================================================
# SwitchBack - Modern Windows Installer Script
# https://github.com/JAIstudio-source/Switchback
# ==============================================================================

param(
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"
$AppName = "SwitchBack"
$AppVersion = "1.2.0"
$Publisher = "JAIstudio"
$RepoUrl = "https://github.com/JAIstudio-source/Switchback"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
if (-not $ScriptDir) { $ScriptDir = Get-Location }

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "SwitchBack"
}

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
$SourceExe = Join-Path $ScriptDir "switchback.exe"
if (Test-Path $SourceExe) {
    Copy-Item -Path $SourceExe -Destination (Join-Path $InstallDir "switchback.exe") -Force
} else {
    Write-Host "[!] switchback.exe not found in $ScriptDir. Downloading latest release..." -ForegroundColor Yellow
    $downloadUrl = "$RepoUrl/releases/latest/download/switchback.exe"
    Invoke-WebRequest -Uri $downloadUrl -OutFile (Join-Path $InstallDir "switchback.exe") -UseBasicParsing
}

$SourceIco = Join-Path $ScriptDir "icon.ico"
if (Test-Path $SourceIco) {
    Copy-Item -Path $SourceIco -Destination (Join-Path $InstallDir "icon.ico") -Force
}

$SourceIcon = Join-Path $ScriptDir "icon.png"
if (Test-Path $SourceIcon) {
    Copy-Item -Path $SourceIcon -Destination (Join-Path $InstallDir "icon.png") -Force
}

$SourceBatch = Join-Path $ScriptDir "Launch UI.bat"
if (Test-Path $SourceBatch) {
    Copy-Item -Path $SourceBatch -Destination (Join-Path $InstallDir "Launch UI.bat") -Force
}

$SourceReadme = Join-Path $ScriptDir "README.md"
if (Test-Path $SourceReadme) {
    Copy-Item -Path $SourceReadme -Destination (Join-Path $InstallDir "README.md") -Force
}

$SourceUninstallBat = Join-Path $ScriptDir "Uninstall.bat"
if (Test-Path $SourceUninstallBat) {
    Copy-Item -Path $SourceUninstallBat -Destination (Join-Path $InstallDir "Uninstall.bat") -Force
}

$SourceUninstallPs1 = Join-Path $ScriptDir "Uninstall-SwitchBack.ps1"
if (Test-Path $SourceUninstallPs1) {
    Copy-Item -Path $SourceUninstallPs1 -Destination (Join-Path $InstallDir "Uninstall-SwitchBack.ps1") -Force
    Copy-Item -Path $SourceUninstallPs1 -Destination (Join-Path $InstallDir "uninstall.ps1") -Force
}

# 4. Create Desktop and Start Menu Shortcuts
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

# Start Menu Uninstall Shortcut
$StartUninstallShortcut = $WshShell.CreateShortcut((Join-Path $StartMenuPath "Uninstall SwitchBack.lnk"))
$StartUninstallShortcut.TargetPath = (Join-Path $InstallDir "Uninstall.bat")
$StartUninstallShortcut.WorkingDirectory = $InstallDir
$StartUninstallShortcut.Description = "Uninstall SwitchBack from your computer"
if (Test-Path $IcoPath) {
    $StartUninstallShortcut.IconLocation = "$IcoPath,0"
}
$StartUninstallShortcut.Save()

# 5. Add to User PATH
Write-Host "[*] Adding SwitchBack to user PATH environment variable..." -ForegroundColor Cyan
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
if ($CurrentPath -notlike "*$InstallDir*") {
    $NewPath = if ($CurrentPath) { "$CurrentPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, [EnvironmentVariableTarget]::User)
    $env:PATH = "$env:PATH;$InstallDir"
    Write-Host "[OK] Added to PATH. 'switchback' command is now available everywhere!" -ForegroundColor Green
}

# 6. Register in Windows Add/Remove Programs (Control Panel & Settings)
Write-Host "[*] Registering in Windows Add/Remove Programs..." -ForegroundColor Cyan
$UninstallRegKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack"
if (-not (Test-Path $UninstallRegKey)) {
    New-Item -Path $UninstallRegKey -Force | Out-Null
}
$UninstallExe = Join-Path $InstallDir "Uninstall.bat"
Set-ItemProperty -Path $UninstallRegKey -Name "DisplayName" -Value "SwitchBack"
Set-ItemProperty -Path $UninstallRegKey -Name "DisplayVersion" -Value $AppVersion
Set-ItemProperty -Path $UninstallRegKey -Name "Publisher" -Value $Publisher
Set-ItemProperty -Path $UninstallRegKey -Name "InstallLocation" -Value $InstallDir
Set-ItemProperty -Path $UninstallRegKey -Name "UninstallString" -Value "`"$UninstallExe`""
Set-ItemProperty -Path $UninstallRegKey -Name "DisplayIcon" -Value (Join-Path $InstallDir "icon.ico")
Set-ItemProperty -Path $UninstallRegKey -Name "HelpLink" -Value $RepoUrl
Set-ItemProperty -Path $UninstallRegKey -Name "NoModify" -Value 1 -Type DWord
Set-ItemProperty -Path $UninstallRegKey -Name "NoRepair" -Value 1 -Type DWord

# 7. Refresh Windows Shell & Icon Cache
& ie4uinit.exe -show 2>$null

# 8. Configure AI Agent Hooks
Write-Host "[*] Configuring agent hooks for Antigravity IDE and Claude Code..." -ForegroundColor Cyan
& (Join-Path $InstallDir "switchback.exe") install | Out-Null

Write-Host "`n=======================================================" -ForegroundColor Green
Write-Host "       SwitchBack Installed Successfully (v$AppVersion)!       " -ForegroundColor Yellow
Write-Host "=======================================================" -ForegroundColor Green
Write-Host " 🚀 Desktop Shortcut: Created on Desktop" -ForegroundColor White
Write-Host " 💻 Start Menu:       Available in Windows Start Menu" -ForegroundColor White
Write-Host " ⚡ Terminal Command: Type 'switchback' or 'switchback ui'" -ForegroundColor White
Write-Host " 🗑️  Uninstaller:      Uninstall.bat or Start Menu" -ForegroundColor White
Write-Host "=======================================================" -ForegroundColor Green
