# ==============================================================================
# SwitchBack - Uninstaller Script
# https://github.com/JAIstudio-source/Switchback
# ==============================================================================

param(
    [string]$InstallDir = ""
)

$ErrorActionPreference = "SilentlyContinue"

Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "           SwitchBack - Uninstall Assistant            " -ForegroundColor Yellow
Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host ""

if (-not $InstallDir) {
    # Check registry or default location
    $RegKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack"
    if (Test-Path $RegKey) {
        $InstallDir = (Get-ItemProperty -Path $RegKey -Name "InstallLocation" -ErrorAction SilentlyContinue).InstallLocation
    }
    if (-not $InstallDir) {
        $InstallDir = Join-Path $env:LOCALAPPDATA "SwitchBack"
    }
}

Write-Host "[1/6] Stopping running SwitchBack instances..." -ForegroundColor Cyan
$running = Get-Process -Name "switchback" -ErrorAction SilentlyContinue
if ($running) {
    $running | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Milliseconds 600
}

# 2. Remove Agent Hooks
Write-Host "[2/6] Disconnecting AI agent hooks..." -ForegroundColor Cyan
$ExePath = Join-Path $InstallDir "switchback.exe"
if (Test-Path $ExePath) {
    & $ExePath uninstall | Out-Null
}

# 3. Remove Desktop & Start Menu Shortcuts
Write-Host "[3/6] Removing shortcuts..." -ForegroundColor Cyan
$DesktopShortcut = Join-Path ([Environment]::GetFolderPath("Desktop")) "SwitchBack.lnk"
if (Test-Path $DesktopShortcut) {
    Remove-Item $DesktopShortcut -Force -ErrorAction SilentlyContinue
}

$StartPrograms = [Environment]::GetFolderPath("Programs")
$StartShortcut = Join-Path $StartPrograms "SwitchBack.lnk"
if (Test-Path $StartShortcut) {
    Remove-Item $StartShortcut -Force -ErrorAction SilentlyContinue
}
$StartUninstallShortcut = Join-Path $StartPrograms "Uninstall SwitchBack.lnk"
if (Test-Path $StartUninstallShortcut) {
    Remove-Item $StartUninstallShortcut -Force -ErrorAction SilentlyContinue
}

# 4. Remove User PATH
Write-Host "[4/6] Cleaning environment PATH variable..." -ForegroundColor Cyan
$UserPath = [Environment]::GetEnvironmentVariable("PATH", [EnvironmentVariableTarget]::User)
if ($UserPath -like "*$InstallDir*") {
    $NewPath = ($UserPath.Split(';') | Where-Object { $_ -ne "$InstallDir" -and $_ -ne "" }) -join ';'
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, [EnvironmentVariableTarget]::User)
}

# 5. Remove Registry Entry
Write-Host "[5/6] Removing Windows registry entries..." -ForegroundColor Cyan
$RegKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SwitchBack"
if (Test-Path $RegKey) {
    Remove-Item $RegKey -Recurse -Force -ErrorAction SilentlyContinue
}

# Clean up Windows Firewall Inbound Rule
& netsh.exe advfirewall firewall delete rule name="SwitchBack Mobile Remote" 2>$null | Out-Null

# 6. Delete Installed Directory
Write-Host "[6/6] Removing application files..." -ForegroundColor Cyan
if (Test-Path $InstallDir) {
    # If the script is running from inside the install dir, schedule self-delete or delete remaining items
    Get-ChildItem -Path $InstallDir -Exclude "uninstall.ps1", "Uninstall.bat" -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
    
    # Schedule cleanup of folder after exit
    Start-Process -FilePath "cmd.exe" -ArgumentList "/c timeout /t 1 /nobreak >nul & rd /s /q `"$InstallDir`"" -WindowStyle Hidden
}

# Refresh Windows Shell
& ie4uinit.exe -show 2>$null

Write-Host ""
Write-Host "=======================================================" -ForegroundColor Green
Write-Host " [OK] SwitchBack has been completely removed from your PC." -ForegroundColor Green
Write-Host "=======================================================" -ForegroundColor Green
