@echo off
setlocal enabledelayedexpansion
title SwitchBack Installer
echo =======================================================
echo          SwitchBack - Windows Setup Installer          
echo =======================================================
echo.
echo Installing SwitchBack to your system...
echo.

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0Install-SwitchBack.ps1"
if %errorLevel% equ 0 (
    echo.
    echo =======================================================
    echo  [OK] Installation finished successfully!
    echo       You can now launch SwitchBack from your Start Menu,
    echo       Desktop, or by typing 'switchback' in any terminal.
    echo =======================================================
) else (
    echo.
    echo [!] Installation encountered an issue. See output above.
)

echo.
pause
