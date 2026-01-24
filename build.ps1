# build.ps1
# Build script for go-mapi project
# Usage: .\build.ps1 [-Config Release] [-Platform x64] [-Tests]

param(
    [string]$Config = "Debug",
    [string]$Platform = "x64",
    [switch]$Tests,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$buildDir = Join-Path $projectRoot "build"
$generatorName = "Visual Studio 17 2022"

Write-Host "================================"
Write-Host "  go-mapi Build Script"
Write-Host "================================"
Write-Host ""

# Check for CMake
$cmake = Get-Command cmake -ErrorAction SilentlyContinue
if (-not $cmake) {
    Write-Error "CMake not found. Please install CMake or add it to PATH."
    exit 1
}

Write-Host "CMake: $($cmake.Source)"
Write-Host ""

# Clean build directory if requested
if ($Clean) {
    Write-Host "Cleaning build directory..."
    if (Test-Path $buildDir) {
        Remove-Item $buildDir -Recurse -Force
    }
}

# Create build directory
if (-not (Test-Path $buildDir)) {
    New-Item $buildDir -ItemType Directory | Out-Null
}

Write-Host "Configuration: $Config"
Write-Host "Platform: $Platform"
Write-Host "Build Directory: $buildDir"
Write-Host ""

# Configure CMake
Write-Host "Configuring CMake..."
Push-Location $buildDir

$cmakeArgs = @(
    "-G", $generatorName,
    "-A", $Platform,
    "-DCMAKE_BUILD_TYPE=$Config",
    "-DBUILD_TESTS=$(if ($Tests) { 'ON' } else { 'OFF' })",
    ".."
)

& cmake $cmakeArgs
if ($LASTEXITCODE -ne 0) {
    Write-Error "CMake configuration failed"
    exit 1
}

# Build
Write-Host ""
Write-Host "Building..."
& cmake --build . --config $Config
if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed"
    exit 1
}

Pop-Location

Write-Host ""
Write-Host "================================"
Write-Host "  Build successful!"
Write-Host "================================"
Write-Host ""
Write-Host "Output directory: $buildDir\bin"
Write-Host ""
