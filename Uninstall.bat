@echo off
setlocal enabledelayedexpansion
title SwitchBack Uninstaller
echo =======================================================
echo          SwitchBack - Uninstall Assistant              
echo =======================================================
echo.
echo Removing SwitchBack and its shortcuts from your system...
echo.

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Uninstall-SwitchBack.ps1"
if %errorLevel% equ 0 (
    echo.
    echo =======================================================
    echo  [OK] Uninstallation complete!
    echo       SwitchBack has been successfully removed.
    echo =======================================================
) else (
    echo.
    echo [!] Uninstallation encountered an issue.
)

echo.
pause
