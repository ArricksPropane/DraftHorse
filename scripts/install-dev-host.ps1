# install-dev-host.ps1
# Installs native messaging host for development (points to build directory)
# Usage: .\install-dev-host.ps1 -ExtensionId "your-extension-id"

param(
    [Parameter(Mandatory=$true)]
    [string]$ExtensionId
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
$hostExe = Join-Path $repoRoot "src\native-host\build\go-mapi-host.exe"

if (-not (Test-Path $hostExe)) {
    Write-Error "Native host not built. Run: npm run build:native-host"
    exit 1
}

Write-Host "Installing go-mapi Native Messaging Host (dev mode)..."
Write-Host "Extension ID: $ExtensionId"
Write-Host "Host executable: $hostExe"

# Create manifest pointing to build directory
$manifest = @{
    name = "com.gomapi.host"
    description = "go-mapi Native Messaging Host (dev)"
    path = $hostExe
    type = "stdio"
    allowed_origins = @("chrome-extension://$ExtensionId/")
} | ConvertTo-Json -Depth 10

# Write manifest to repo
$manifestPath = Join-Path $repoRoot "src\native-host\build\com.gomapi.host.json"
$manifest | Out-File -FilePath $manifestPath -Encoding UTF8
Write-Host "Created manifest: $manifestPath"

# Register with Chrome
$chromeRegPath = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.gomapi.host"
if (-not (Test-Path (Split-Path $chromeRegPath))) {
    New-Item -Path (Split-Path $chromeRegPath) -Force | Out-Null
}
New-Item -Path $chromeRegPath -Force | Out-Null
Set-ItemProperty -Path $chromeRegPath -Name "(Default)" -Value $manifestPath
Write-Host "Registered Chrome: $chromeRegPath"

# Register with Edge
$edgeRegPath = "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.gomapi.host"
if (-not (Test-Path (Split-Path $edgeRegPath))) {
    New-Item -Path (Split-Path $edgeRegPath) -Force | Out-Null
}
New-Item -Path $edgeRegPath -Force | Out-Null
Set-ItemProperty -Path $edgeRegPath -Name "(Default)" -Value $manifestPath
Write-Host "Registered Edge: $edgeRegPath"

Write-Host ""
Write-Host "Done! Native host registered for development." -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:"
Write-Host "1. Reload the extension in chrome://extensions"
Write-Host "2. Click the extension icon to see the popup"
