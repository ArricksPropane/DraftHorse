# setup.ps1 - Setup Windows Sandbox for DLL testing
# Installs WinAppDriver and registers the DLL

$ErrorActionPreference = "Stop"
$ProjectRoot = "C:\go-mapi"
$WinAppDriverUrl = "https://github.com/microsoft/WinAppDriver/releases/download/v1.2.99/WindowsApplicationDriver-1.2.99-win-x64.exe"
$WinAppDriverInstaller = "$env:TEMP\WinAppDriver.exe"
$WinAppDriverPath = "C:\Program Files\Windows Application Driver\WinAppDriver.exe"

Write-Host "=== Windows Sandbox Setup ===" -ForegroundColor Cyan

# Step 1: Register DLL
Write-Host "`n[1/3] Registering MAPI DLL..."
try {
    & "$ProjectRoot\tests\sandbox\test-dll-registration.ps1"
} catch {
    Write-Host "DLL registration failed: $_" -ForegroundColor Red
    exit 1
}

# Step 2: Enable Developer Mode (required for WinAppDriver)
Write-Host "`n[2/3] Enabling Developer Mode..."
try {
    $devModePath = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock"
    if (-not (Test-Path $devModePath)) {
        New-Item -Path $devModePath -Force | Out-Null
    }
    Set-ItemProperty -Path $devModePath -Name "AllowDevelopmentWithoutDevLicense" -Value 1 -Type DWord -Force
    Write-Host "OK: Developer Mode enabled" -ForegroundColor Green
} catch {
    Write-Host "WARNING: Could not enable Developer Mode: $_" -ForegroundColor Yellow
}

# Step 3: Download and install WinAppDriver
Write-Host "`n[3/3] Installing WinAppDriver..."
if (Test-Path $WinAppDriverPath) {
    Write-Host "OK: WinAppDriver already installed" -ForegroundColor Green
} else {
    Write-Host "Downloading WinAppDriver..."
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri $WinAppDriverUrl -OutFile $WinAppDriverInstaller -UseBasicParsing
        Write-Host "Installing WinAppDriver (silent)..."
        Start-Process -FilePath $WinAppDriverInstaller -ArgumentList "/S" -Wait -NoNewWindow

        if (Test-Path $WinAppDriverPath) {
            Write-Host "OK: WinAppDriver installed" -ForegroundColor Green
        } else {
            Write-Host "FAILED: WinAppDriver not found after install" -ForegroundColor Red
            exit 1
        }
    } catch {
        Write-Host "FAILED: Could not install WinAppDriver: $_" -ForegroundColor Red
        exit 1
    }
}

Write-Host "`n=== SETUP COMPLETE ===" -ForegroundColor Green
Write-Host ""
Write-Host "To start WinAppDriver server:"
Write-Host "  & '$WinAppDriverPath'"
Write-Host ""
Write-Host "To run the full DLL test:"
Write-Host "  & '$ProjectRoot\tests\sandbox\test-dll.ps1'"
