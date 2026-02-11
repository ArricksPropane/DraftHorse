#Requires -RunAsAdministrator
<#
.SYNOPSIS
    Uninstalls go-mapi: removes registry entries, restores previous mail client, deletes files.

.DESCRIPTION
    Reverses everything done by install.ps1. Can be run as a one-liner:

      irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/uninstall.ps1 | iex

.PARAMETER InstallDir
    Installation directory to remove. Defaults to "C:\Program Files\go-mapi".

.PARAMETER KeepFiles
    Only removes registry entries but keeps installed files.

.EXAMPLE
    irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/uninstall.ps1 | iex

.EXAMPLE
    .\uninstall.ps1 -KeepFiles
#>

param(
    [string]$InstallDir = "C:\Program Files\go-mapi",

    [switch]$KeepFiles
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# --- Helpers ---

function Write-Step { param([string]$Msg) Write-Host "  [+] $Msg" -ForegroundColor Green }
function Write-Info { param([string]$Msg) Write-Host "  [i] $Msg" -ForegroundColor Cyan }
function Write-Warn { param([string]$Msg) Write-Host "  [!] $Msg" -ForegroundColor Yellow }
function Write-Skip { param([string]$Msg) Write-Host "  [-] $Msg" -ForegroundColor DarkGray }

# --- Banner ---

Write-Host ""
Write-Host "  go-mapi uninstaller" -ForegroundColor White
Write-Host "  ========================================" -ForegroundColor DarkGray

# Read install metadata if available
$metadataPath = Join-Path $InstallDir ".install-metadata.json"
$metadata = $null
if (Test-Path $metadataPath) {
    $metadata = Get-Content $metadataPath -Raw | ConvertFrom-Json
    Write-Info "Found installation: $($metadata.version) (installed $($metadata.installedAt))"
} else {
    Write-Info "Install dir: $InstallDir"
}
Write-Host ""

# --- Step 1: Remove native messaging registrations ---

Write-Host "  Step 1: Remove browser registrations" -ForegroundColor White

$browsers = @(
    @{ Name = "Chrome";   Path = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.gomapi.host" },
    @{ Name = "Chromium"; Path = "HKCU:\Software\Chromium\NativeMessagingHosts\com.gomapi.host" },
    @{ Name = "Edge";     Path = "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.gomapi.host" }
)

foreach ($browser in $browsers) {
    if (Test-Path $browser.Path) {
        Remove-Item -Path $browser.Path -Force
        Write-Step "Removed $($browser.Name) registration"
    } else {
        Write-Skip "$($browser.Name) not registered"
    }
}

# --- Step 2: Remove MAPI registry entries ---

Write-Host ""
Write-Host "  Step 2: Remove MAPI registration" -ForegroundColor White

$mailClientsPath = "HKLM:\SOFTWARE\Clients\Mail"
$goMapiRegPath   = "$mailClientsPath\go-mapi"

$currentDefault = $null
try {
    $currentDefault = (Get-ItemProperty -Path $mailClientsPath -Name "(Default)" -ErrorAction SilentlyContinue).'(Default)'
} catch { }

if (Test-Path $goMapiRegPath) {
    Remove-Item -Path $goMapiRegPath -Recurse -Force
    Write-Step "Removed registry key: $goMapiRegPath"
} else {
    Write-Skip "MAPI registry key not found"
}

# Restore previous default mail client
if ($currentDefault -eq "go-mapi") {
    $previousDefault = $null

    # Try backup file first
    $backupFile = Join-Path $InstallDir ".previous-mail-client"
    if (Test-Path $backupFile) {
        $previousDefault = (Get-Content $backupFile -Raw).Trim()
    }
    # Try metadata
    elseif ($metadata -and $metadata.previousClient) {
        $previousDefault = $metadata.previousClient
    }

    if ($previousDefault -and (Test-Path "$mailClientsPath\$previousDefault")) {
        Set-ItemProperty -Path $mailClientsPath -Name "(Default)" -Value $previousDefault -Force
        Write-Step "Restored default mail client: $previousDefault"
    } else {
        # Auto-detect common clients
        $fallbacks = @("Microsoft Outlook", "Outlook", "Windows Mail")
        $restored = $false
        foreach ($fallback in $fallbacks) {
            if (Test-Path "$mailClientsPath\$fallback") {
                Set-ItemProperty -Path $mailClientsPath -Name "(Default)" -Value $fallback -Force
                Write-Step "Restored default mail client: $fallback (auto-detected)"
                $restored = $true
                break
            }
        }
        if (-not $restored) {
            Set-ItemProperty -Path $mailClientsPath -Name "(Default)" -Value "" -Force
            Write-Warn "No previous mail client found — cleared default"
        }
    }
} else {
    Write-Info "Default mail client is '$currentDefault' (not go-mapi, leaving unchanged)"
}

# --- Step 3: Remove files ---

Write-Host ""
Write-Host "  Step 3: Remove files" -ForegroundColor White

if ($KeepFiles) {
    Write-Info "KeepFiles specified — skipping file removal"
} elseif (Test-Path $InstallDir) {
    $fileCount = (Get-ChildItem -Path $InstallDir -Force).Count
    Remove-Item -Path $InstallDir -Recurse -Force
    Write-Step "Removed $InstallDir ($fileCount files)"
} else {
    Write-Skip "Install directory not found"
}

# --- Done ---

Write-Host ""
Write-Host "  ========================================" -ForegroundColor DarkGray
Write-Host "  go-mapi uninstalled" -ForegroundColor Green
Write-Host ""
