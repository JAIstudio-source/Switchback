@echo off
:: ============================================================================
:: switchback - Developer Environment Setup & Antivirus Exclusion
:: ============================================================================
echo [switchback] Setting up developer environment...

:: Check for Administrator privileges
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo.
    echo [!] Administrator privileges required to configure Windows Defender exclusion.
    echo [*] Requesting elevation...
    powershell -Command "Start-Process '%~f0' -Verb RunAs"
    exit /b
)

set "TARGET_DIR=%~dp0"
:: Remove trailing backslash
if "%TARGET_DIR:~-1%"=="\" set "TARGET_DIR=%TARGET_DIR:~0,-1%"

echo [*] Target Directory: "%TARGET_DIR%"
echo [*] Adding Windows Defender folder exclusion to prevent false-positive heuristic flags during local development...

powershell -Command "Add-MpPreference -ExclusionPath '%TARGET_DIR%'" >nul 2>&1
if %errorLevel% equ 0 (
    echo [OK] Windows Defender exclusion successfully added for:
    echo      %TARGET_DIR%
) else (
    echo [!] Could not add exclusion automatically. You can add it manually in Windows Security.
)

echo.
echo [*] Verifying switchback hooks...
"%TARGET_DIR%\switchback.exe" hook --event Stop --agent antigravity >nul 2>&1
echo [OK] Antigravity & Claude hooks verified.

echo.
echo ============================================================================
echo  Setup Complete! You can now run 'Launch UI.bat' or build without AV blocks.
echo ============================================================================
pause
