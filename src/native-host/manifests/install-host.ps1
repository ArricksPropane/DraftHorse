# install-host.ps1
# Installs the native messaging host manifests for Chrome and Edge
# Run as Administrator

param(
    [Parameter(Mandatory=$true)]
    [string]$ExtensionId,

    [string]$InstallDir = "C:\Program Files\go-mapi"
)

$ErrorActionPreference = "Stop"

Write-Host "Installing go-mapi Native Messaging Host..."
Write-Host "Extension ID: $ExtensionId"
Write-Host "Install Directory: $InstallDir"

# Create install directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Write-Host "Created directory: $InstallDir"
}

# Copy executable
$exePath = Join-Path $PSScriptRoot "..\build\go-mapi-host.exe"
if (Test-Path $exePath) {
    Copy-Item $exePath -Destination $InstallDir -Force
    Write-Host "Copied executable to $InstallDir"
} else {
    Write-Warning "Executable not found at $exePath - skipping copy"
}

# Create manifest content
$manifest = @{
    name = "com.gomapi.host"
    description = "go-mapi Native Messaging Host - bridges MAPI interceptor with browser extension"
    path = "$InstallDir\go-mapi-host.exe"
    type = "stdio"
    allowed_origins = @("chrome-extension://$ExtensionId/")
} | ConvertTo-Json -Depth 10

# Write Chrome manifest
$chromeManifestPath = Join-Path $InstallDir "com.gomapi.host.json"
$manifest | Out-File -FilePath $chromeManifestPath -Encoding UTF8
Write-Host "Created manifest: $chromeManifestPath"

# Register with Chrome (HKCU for current user, HKLM for all users)
$chromeRegPath = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.gomapi.host"
if (-not (Test-Path (Split-Path $chromeRegPath))) {
    New-Item -Path (Split-Path $chromeRegPath) -Force | Out-Null
}
New-Item -Path $chromeRegPath -Force | Out-Null
Set-ItemProperty -Path $chromeRegPath -Name "(Default)" -Value $chromeManifestPath
Write-Host "Registered Chrome native host at: $chromeRegPath"

# Register with Edge
$edgeRegPath = "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.gomapi.host"
if (-not (Test-Path (Split-Path $edgeRegPath))) {
    New-Item -Path (Split-Path $edgeRegPath) -Force | Out-Null
}
New-Item -Path $edgeRegPath -Force | Out-Null
Set-ItemProperty -Path $edgeRegPath -Name "(Default)" -Value $chromeManifestPath
Write-Host "Registered Edge native host at: $edgeRegPath"

Write-Host ""
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "The native messaging host is now registered for both Chrome and Edge."
