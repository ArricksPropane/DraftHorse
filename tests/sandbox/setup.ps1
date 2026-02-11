# setup.ps1 - Setup Windows Sandbox for DLL testing
# Installs WinAppDriver and registers the DLL
# Writes output to C:\output\setup.log

$ErrorActionPreference = "Stop"
$OutputFile = "C:\output\setup.log"
$WinAppDriverUrl = "https://github.com/microsoft/WinAppDriver/releases/download/v1.2.99/WindowsApplicationDriver-1.2.99-win-x64.exe"
$WinAppDriverInstaller = "$env:TEMP\WinAppDriver.exe"
$WinAppDriverPath = "C:\Program Files\Windows Application Driver\WinAppDriver.exe"

function Log($msg) {
    Write-Host $msg
    Add-Content -Path $OutputFile -Value $msg
}

"" | Set-Content $OutputFile
Log "=== Windows Sandbox Setup ==="
Log "Timestamp: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
Log ""

# Step 1: Enable Developer Mode (required for WinAppDriver)
Log "[1/2] Enabling Developer Mode..."
try {
    $devModePath = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock"
    if (-not (Test-Path $devModePath)) {
        New-Item -Path $devModePath -Force | Out-Null
    }
    Set-ItemProperty -Path $devModePath -Name "AllowDevelopmentWithoutDevLicense" -Value 1 -Type DWord -Force
    Log "OK: Developer Mode enabled"
} catch {
    Log "WARNING: Could not enable Developer Mode: $_"
}

# Step 2: Download and install WinAppDriver
Log ""
Log "[2/2] Installing WinAppDriver..."
if (Test-Path $WinAppDriverPath) {
    Log "OK: WinAppDriver already installed at $WinAppDriverPath"
} else {
    Log "Downloading from $WinAppDriverUrl..."
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Invoke-WebRequest -Uri $WinAppDriverUrl -OutFile $WinAppDriverInstaller -UseBasicParsing
        Log "Downloaded to $WinAppDriverInstaller"
        Log "Installing WinAppDriver (silent)..."
        Start-Process -FilePath $WinAppDriverInstaller -ArgumentList "/S" -Wait -NoNewWindow

        if (Test-Path $WinAppDriverPath) {
            Log "OK: WinAppDriver installed at $WinAppDriverPath"
        } else {
            Log "FAILED: WinAppDriver not found after install"
            exit 1
        }
    } catch {
        Log "FAILED: Could not install WinAppDriver: $_"
        exit 1
    }
}

Log ""
Log "=== SETUP COMPLETE ==="
exit 0
