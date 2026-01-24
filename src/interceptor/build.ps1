# build.ps1
# Build script for go-mapi interceptor
# Usage: .\build.ps1 [-Config Release] [-Platform x64] [-Tests]

param(
    [string]$Config = "Debug",
    [string]$Platform = "x64",
    [switch]$Tests,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

# Navigate to the root of the project (two levels up from interceptor)
$interceptorRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent (Split-Path -Parent $interceptorRoot)
$buildDir = Join-Path $projectRoot "build"
$generatorName = "Visual Studio 17 2022"

Write-Host "================================"
Write-Host "  go-mapi Interceptor Build"
Write-Host "================================"
Write-Host ""

# Check for CMake
$cmake = Get-Command cmake -ErrorAction SilentlyContinue
if (-not $cmake) {
    Write-Error "CMake not found. Please install CMake or add it to PATH."
    exit 1
}

Write-Host "CMake: $($cmake.Source)"
Write-Host "Project Root: $projectRoot"
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

# Configure CMake from project root
Write-Host "Configuring CMake..."
Push-Location $projectRoot

$cmakeArgs = @(
    "-G", $generatorName,
    "-A", $Platform,
    "-DCMAKE_BUILD_TYPE=$Config",
    "-DBUILD_TESTS=$(if ($Tests) { 'ON' } else { 'OFF' })",
    "-S", ".",
    "-B", $buildDir
)

& cmake $cmakeArgs
if ($LASTEXITCODE -ne 0) {
    Write-Error "CMake configuration failed"
    exit 1
}

# Build
Write-Host ""
Write-Host "Building..."
& cmake --build $buildDir --config $Config
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
