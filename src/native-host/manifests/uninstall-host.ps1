# uninstall-host.ps1
# Removes the native messaging host registration
# Run as Administrator

param(
    [string]$InstallDir = "C:\Program Files\go-mapi"
)

$ErrorActionPreference = "Stop"

Write-Host "Uninstalling go-mapi Native Messaging Host..."

# Remove Chrome registration
$chromeRegPath = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.gomapi.host"
if (Test-Path $chromeRegPath) {
    Remove-Item -Path $chromeRegPath -Force
    Write-Host "Removed Chrome registration"
}

# Remove Edge registration
$edgeRegPath = "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.gomapi.host"
if (Test-Path $edgeRegPath) {
    Remove-Item -Path $edgeRegPath -Force
    Write-Host "Removed Edge registration"
}

# Remove files
if (Test-Path $InstallDir) {
    $hostExe = Join-Path $InstallDir "go-mapi-host.exe"
    $manifest = Join-Path $InstallDir "com.gomapi.host.json"

    if (Test-Path $hostExe) { Remove-Item $hostExe -Force }
    if (Test-Path $manifest) { Remove-Item $manifest -Force }

    Write-Host "Removed files from $InstallDir"
}

Write-Host ""
Write-Host "Uninstallation complete!" -ForegroundColor Green
