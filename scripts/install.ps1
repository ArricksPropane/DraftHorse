#Requires -RunAsAdministrator
<#
.SYNOPSIS
    Installs go-mapi: copies binaries, registers MAPI DLL, and sets up native messaging.

.DESCRIPTION
    Single installer that:
    1. Copies go-mapi.dll and go-mapi-host.exe to the install directory
    2. Registers the DLL as the default Windows MAPI mail client
    3. Creates and registers native messaging manifests for Chrome and Edge
    4. Backs up the previous default mail client for clean uninstall

.PARAMETER ExtensionId
    Chrome/Edge extension ID (from chrome://extensions after loading unpacked).
    Required for native messaging to work.

.PARAMETER InstallDir
    Target installation directory. Defaults to "C:\Program Files\go-mapi".

.PARAMETER BuildDir
    Directory containing built artifacts. If not specified, searches the repo
    build output directories automatically.

.EXAMPLE
    .\install.ps1 -ExtensionId "abcdefghijklmnopqrstuvwxyz123456"

.EXAMPLE
    .\install.ps1 -ExtensionId "abc123..." -InstallDir "D:\go-mapi" -BuildDir "C:\dev\go-mapi\out"
#>

param(
    [Parameter(Mandatory = $true, HelpMessage = "Extension ID from chrome://extensions")]
    [ValidatePattern('^[a-z]{32}$')]
    [string]$ExtensionId,

    [string]$InstallDir = "C:\Program Files\go-mapi",

    [string]$BuildDir
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# --- Helpers ---

function Write-Step { param([string]$Message) Write-Host "  [+] $Message" -ForegroundColor Green }
function Write-Info { param([string]$Message) Write-Host "  [i] $Message" -ForegroundColor Cyan }
function Write-Warn { param([string]$Message) Write-Host "  [!] $Message" -ForegroundColor Yellow }

function Find-Artifact {
    param([string]$FileName, [string[]]$SearchPaths)
    foreach ($dir in $SearchPaths) {
        $path = Join-Path $dir $FileName
        if (Test-Path $path) { return $path }
    }
    return $null
}

# --- Locate build artifacts ---

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir

if ($BuildDir) {
    $searchPaths = @($BuildDir)
} else {
    $searchPaths = @(
        (Join-Path $repoRoot "src\interceptor\build\bin"),
        (Join-Path $repoRoot "src\native-host\build"),
        (Join-Path $repoRoot "build"),
        (Join-Path $repoRoot "dist")
    )
}

$dllSource = Find-Artifact "go-mapi.dll" $searchPaths
$hostSource = Find-Artifact "go-mapi-host.exe" $searchPaths

if (-not $dllSource) {
    Write-Error "go-mapi.dll not found. Run 'npm run build:interceptor' first.`nSearched: $($searchPaths -join ', ')"
}
if (-not $hostSource) {
    Write-Error "go-mapi-host.exe not found. Run 'npm run build:native-host' first.`nSearched: $($searchPaths -join ', ')"
}

Write-Host ""
Write-Host "go-mapi Installer" -ForegroundColor White -BackgroundColor DarkBlue
Write-Host "========================================" -ForegroundColor DarkGray
Write-Info "DLL source:  $dllSource"
Write-Info "Host source: $hostSource"
Write-Info "Install to:  $InstallDir"
Write-Info "Extension:   $ExtensionId"
Write-Host ""

# --- Step 1: Create install directory and copy files ---

Write-Host "Step 1: Copy files" -ForegroundColor White
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Write-Step "Created directory: $InstallDir"
} else {
    Write-Info "Directory exists: $InstallDir"
}

Copy-Item -Path $dllSource -Destination (Join-Path $InstallDir "go-mapi.dll") -Force
Write-Step "Copied go-mapi.dll"

Copy-Item -Path $hostSource -Destination (Join-Path $InstallDir "go-mapi-host.exe") -Force
Write-Step "Copied go-mapi-host.exe"

# --- Step 2: Register MAPI DLL as default mail client ---

Write-Host ""
Write-Host "Step 2: Register MAPI handler" -ForegroundColor White

$mailClientsPath = "HKLM:\SOFTWARE\Clients\Mail"
$goMapiRegPath = "$mailClientsPath\go-mapi"
$dllInstallPath = Join-Path $InstallDir "go-mapi.dll"

# Back up the current default mail client (for uninstall)
$previousDefault = $null
try {
    $previousDefault = (Get-ItemProperty -Path $mailClientsPath -Name "(Default)" -ErrorAction SilentlyContinue).'(Default)'
} catch { }

if ($previousDefault -and $previousDefault -ne "go-mapi") {
    $backupFile = Join-Path $InstallDir ".previous-mail-client"
    $previousDefault | Out-File -FilePath $backupFile -Encoding UTF8 -NoNewline
    Write-Info "Backed up previous default mail client: $previousDefault"
}

# Create go-mapi mail client key
New-Item -Path $goMapiRegPath -Force | Out-Null
New-ItemProperty -Path $goMapiRegPath -Name "(Default)" -Value "go-mapi" -PropertyType String -Force | Out-Null

# DLLPath subkey with path to the DLL
$dllRegPath = "$goMapiRegPath\DLLPath"
New-Item -Path $dllRegPath -Force | Out-Null
New-ItemProperty -Path $dllRegPath -Name "(Default)" -Value $dllInstallPath -PropertyType String -Force | Out-Null

# Set as default mail client
Set-ItemProperty -Path $mailClientsPath -Name "(Default)" -Value "go-mapi" -Force

Write-Step "Registered at: $goMapiRegPath"
Write-Step "DLL path: $dllInstallPath"
Write-Step "Set as default mail client"

# --- Step 3: Create native messaging manifest ---

Write-Host ""
Write-Host "Step 3: Native messaging host" -ForegroundColor White

$hostExePath = (Join-Path $InstallDir "go-mapi-host.exe") -replace '\\', '\\'
$manifest = @{
    name             = "com.gomapi.host"
    description      = "go-mapi Native Messaging Host — bridges MAPI interceptor with browser extension"
    path             = (Join-Path $InstallDir "go-mapi-host.exe")
    type             = "stdio"
    allowed_origins  = @("chrome-extension://$ExtensionId/")
} | ConvertTo-Json -Depth 10

$manifestPath = Join-Path $InstallDir "com.gomapi.host.json"
$manifest | Out-File -FilePath $manifestPath -Encoding UTF8
Write-Step "Created manifest: $manifestPath"

# --- Step 4: Register with Chrome and Edge ---

Write-Host ""
Write-Host "Step 4: Register with browsers" -ForegroundColor White

$browsers = @(
    @{ Name = "Chrome"; Path = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.gomapi.host" },
    @{ Name = "Edge";   Path = "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.gomapi.host" }
)

foreach ($browser in $browsers) {
    $parentPath = Split-Path $browser.Path
    if (-not (Test-Path $parentPath)) {
        New-Item -Path $parentPath -Force | Out-Null
    }
    New-Item -Path $browser.Path -Force | Out-Null
    Set-ItemProperty -Path $browser.Path -Name "(Default)" -Value $manifestPath
    Write-Step "Registered $($browser.Name): $($browser.Path)"
}

# --- Step 5: Save install metadata (for uninstall) ---

$metadata = @{
    version      = "1.0.0"
    installDir   = $InstallDir
    extensionId  = $ExtensionId
    installedAt  = (Get-Date -Format "o")
    dllSource    = $dllSource
    hostSource   = $hostSource
} | ConvertTo-Json -Depth 10

$metadataPath = Join-Path $InstallDir ".install-metadata.json"
$metadata | Out-File -FilePath $metadataPath -Encoding UTF8

# --- Summary ---

Write-Host ""
Write-Host "========================================" -ForegroundColor DarkGray
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "  Install dir:     $InstallDir"
Write-Host "  MAPI DLL:        $dllInstallPath"
Write-Host "  Native host:     $(Join-Path $InstallDir 'go-mapi-host.exe')"
Write-Host "  Manifest:        $manifestPath"
Write-Host "  Default client:  go-mapi"
if ($previousDefault -and $previousDefault -ne "go-mapi") {
    Write-Host "  Previous client: $previousDefault (backed up)"
}
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  1. Load the extension in chrome://extensions (Developer mode → Load unpacked)"
Write-Host "  2. Verify the extension ID matches: $ExtensionId"
Write-Host "  3. Test with: right-click a file → Send to → Mail recipient"
Write-Host ""
