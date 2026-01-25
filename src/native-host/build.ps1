# build.ps1 - Build the native messaging host

param(
    [ValidateSet("Debug", "Release")]
    [string]$Configuration = "Release"
)

$ErrorActionPreference = "Stop"

Write-Host "Building go-mapi Native Messaging Host ($Configuration)..." -ForegroundColor Cyan

# Get script directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Push-Location $scriptDir

try {
    # Create build directory
    $buildDir = Join-Path $scriptDir "build"
    if (-not (Test-Path $buildDir)) {
        New-Item -ItemType Directory -Path $buildDir | Out-Null
    }

    # Set build flags
    $ldflags = "-s -w"  # Strip debug info for smaller binary
    if ($Configuration -eq "Debug") {
        $ldflags = ""
    }

    # Build
    $env:CGO_ENABLED = "0"  # Pure Go, no C dependencies
    $outputPath = Join-Path $buildDir "go-mapi-host.exe"

    Write-Host "Building to $outputPath..."

    if ($ldflags) {
        go build -ldflags $ldflags -o $outputPath .
    } else {
        go build -o $outputPath .
    }

    if ($LASTEXITCODE -ne 0) {
        throw "Build failed"
    }

    # Show result
    $fileInfo = Get-Item $outputPath
    Write-Host ""
    Write-Host "Build successful!" -ForegroundColor Green
    Write-Host "Output: $outputPath"
    Write-Host "Size: $([math]::Round($fileInfo.Length / 1KB, 1)) KB"

} finally {
    Pop-Location
}
