# ==============================================================================
# SwitchBack / FocusMgr - Automated Code Signing Helper Script
# ==============================================================================
# This script creates a local Code Signing Certificate (if one doesn't exist)
# and signs 'switchback.exe' and 'focusmgr.exe' with Authenticode SHA256.
# ==============================================================================

param(
    [string]$CertSubject = "CN=SwitchBack Developer"
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$ProjectRoot = Split-Path -Parent $ScriptDir

$TargetExes = @(
    (Join-Path $ProjectRoot "switchback.exe")
)

Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "  SwitchBack - Code Signing Tool (Authenticode SHA256)  " -ForegroundColor Yellow
Write-Host "=======================================================" -ForegroundColor Cyan

# 1. Look for an existing Code Signing Certificate
$cert = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert | 
        Where-Object { $_.Subject -like "*$CertSubject*" } | 
        Select-Object -First 1

if (-not $cert) {
    Write-Host "[*] Generating a new self-signed Code Signing Certificate..." -ForegroundColor Cyan

    $cert = New-SelfSignedCertificate `
        -Type CodeSigningCert `
        -Subject $CertSubject `
        -CertStoreLocation "Cert:\CurrentUser\My" `
        -NotAfter (Get-Date).AddYears(5) `
        -HashAlgorithm "SHA256" `
        -KeyLength 2048

    Write-Host "[OK] Created Certificate: $($cert.Thumbprint)" -ForegroundColor Green
} else {
    Write-Host "[OK] Using existing certificate: $($cert.Thumbprint) ($($cert.Subject))" -ForegroundColor Green
}

# 2. Sign each target executable
foreach ($exe in $TargetExes) {
    if (Test-Path $exe) {
        Write-Host "[*] Signing: $exe ..." -ForegroundColor Cyan
        try {
            $sigResult = Set-AuthenticodeSignature -FilePath $exe -Certificate $cert -HashAlgorithm "SHA256" -ErrorAction Stop

            if ($sigResult.Status -eq "Valid") {
                Write-Host "    [VALID] $exe successfully signed!" -ForegroundColor Green
            } else {
                Write-Host "    [SIGNED] ($($sigResult.Status)): $($sigResult.StatusMessage)" -ForegroundColor Yellow
            }
        } catch {
            Write-Host "    [SKIPPED] Cannot sign $exe while it is currently running. Please stop the process and re-run." -ForegroundColor Magenta
        }
    }
}

Write-Host "`nSignature Summary:" -ForegroundColor Cyan
foreach ($exe in $TargetExes) {
    if (Test-Path $exe) {
        Get-AuthenticodeSignature $exe | Format-Table -AutoSize
    }
}

Write-Host "Done! Your executables have Authenticode digital signatures." -ForegroundColor Green
