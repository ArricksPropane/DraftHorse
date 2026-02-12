# Package extension for Chrome Web Store
param(
    [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"

# Paths
$ExtensionDist = Join-Path $PSScriptRoot "..\src\extension\dist"
$OutputPath = Join-Path $PSScriptRoot "..\$OutputDir"
$Version = (Get-Content (Join-Path $PSScriptRoot "..\package.json") -Raw | ConvertFrom-Json).version

# Create output directory
New-Item -ItemType Directory -Force -Path $OutputPath | Out-Null

# Output file
$ZipFile = Join-Path $OutputPath "go-mapi-extension-$Version.zip"

# Check if extension is built
if (-not (Test-Path $ExtensionDist)) {
    Write-Error "Extension not built. Run 'npm run build:extension' first."
    exit 1
}

# Remove old zip if exists
if (Test-Path $ZipFile) {
    Remove-Item $ZipFile -Force
    Write-Host "Removed existing package: $ZipFile"
}

# Create zip archive
Write-Host "Packaging extension from: $ExtensionDist"
Compress-Archive -Path "$ExtensionDist\*" -DestinationPath $ZipFile -CompressionLevel Optimal

Write-Host ""
Write-Host "Extension packaged successfully!" -ForegroundColor Green
Write-Host "  File: $ZipFile" -ForegroundColor Cyan
Write-Host "  Size: $([math]::Round((Get-Item $ZipFile).Length / 1KB, 2)) KB" -ForegroundColor Cyan
Write-Host ""
Write-Host "Ready for Chrome Web Store upload." -ForegroundColor Green
