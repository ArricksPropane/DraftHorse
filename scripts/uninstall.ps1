#Requires -RunAsAdministrator
<#
.SYNOPSIS
    Uninstalls go-mapi: removes registry entries, restores previous mail client, deletes files.

.DESCRIPTION
    Reverses everything done by install.ps1:
    1. Removes native messaging registrations from Chrome and Edge
    2. Removes MAPI mail client registry entries
    3. Restores the previous default mail client (if backed up)
    4. Removes the installation directory and all files

.PARAMETER InstallDir
    Installation directory to remove. Defaults to "C:\Program Files\go-mapi".

.PARAMETER KeepFiles
    If specified, only removes registry entries but keeps installed files.

.EXAMPLE
    .\uninstall.ps1

.EXAMPLE
    .\uninstall.ps1 -InstallDir "D:\go-mapi"

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

function Write-Step { param([string]$Message) Write-Host "  [+] $Message" -ForegroundColor Green }
function Write-Info { param([string]$Message) Write-Host "  [i] $Message" -ForegroundColor Cyan }
function Write-Warn { param([string]$Message) Write-Host "  [!] $Message" -ForegroundColor Yellow }
function Write-Skip { param([string]$Message) Write-Host "  [-] $Message" -ForegroundColor DarkGray }

Write-Host ""
Write-Host "go-mapi Uninstaller" -ForegroundColor White -BackgroundColor DarkRed
Write-Host "========================================" -ForegroundColor DarkGray
Write-Info "Install dir: $InstallDir"
Write-Host ""

# --- Step 1: Remove native messaging registrations ---

Write-Host "Step 1: Remove browser registrations" -ForegroundColor White

$browsers = @(
    @{ Name = "Chrome"; Path = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.gomapi.host" },
    @{ Name = "Edge";   Path = "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.gomapi.host" }
)

foreach ($browser in $browsers) {
    if (Test-Path $browser.Path) {
        Remove-Item -Path $browser.Path -Force
        Write-Step "Removed $($browser.Name) registration"
    } else {
        Write-Skip "$($browser.Name) registration not found (already clean)"
    }
}

# --- Step 2: Remove MAPI registry entries ---

Write-Host ""
Write-Host "Step 2: Remove MAPI registration" -ForegroundColor White

$mailClientsPath = "HKLM:\SOFTWARE\Clients\Mail"
$goMapiRegPath = "$mailClientsPath\go-mapi"

# Check current default before removing
$currentDefault = $null
try {
    $currentDefault = (Get-ItemProperty -Path $mailClientsPath -Name "(Default)" -ErrorAction SilentlyContinue).'(Default)'
} catch { }

# Remove go-mapi mail client key
if (Test-Path $goMapiRegPath) {
    Remove-Item -Path $goMapiRegPath -Recurse -Force
    Write-Step "Removed registry key: $goMapiRegPath"
} else {
    Write-Skip "MAPI registry key not found (already clean)"
}

# Restore previous default mail client
if ($currentDefault -eq "go-mapi") {
    $backupFile = Join-Path $InstallDir ".previous-mail-client"
    $previousDefault = $null

    if (Test-Path $backupFile) {
        $previousDefault = (Get-Content $backupFile -Raw).Trim()
    }

    if ($previousDefault) {
        # Verify the previous client still exists in registry
        $previousPath = "$mailClientsPath\$previousDefault"
        if (Test-Path $previousPath) {
            Set-ItemProperty -Path $mailClientsPath -Name "(Default)" -Value $previousDefault -Force
            Write-Step "Restored default mail client: $previousDefault"
        } else {
            Write-Warn "Previous client '$previousDefault' no longer registered — clearing default"
            Set-ItemProperty -Path $mailClientsPath -Name "(Default)" -Value "" -Force
        }
    } else {
        # No backup — try common defaults
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
Write-Host "Step 3: Remove files" -ForegroundColor White

if ($KeepFiles) {
    Write-Info "KeepFiles specified — skipping file removal"
} elseif (Test-Path $InstallDir) {
    $files = Get-ChildItem -Path $InstallDir -Force
    $fileCount = $files.Count

    Remove-Item -Path $InstallDir -Recurse -Force
    Write-Step "Removed $InstallDir ($fileCount files)"
} else {
    Write-Skip "Install directory not found (already clean)"
}

# --- Summary ---

Write-Host ""
Write-Host "========================================" -ForegroundColor DarkGray
Write-Host "Uninstallation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "  Registry entries: removed"
Write-Host "  Browser hosts:    removed"
if (-not $KeepFiles) {
    Write-Host "  Files:            removed"
} else {
    Write-Host "  Files:            kept at $InstallDir"
}
Write-Host ""
