#Requires -RunAsAdministrator
<#
.SYNOPSIS
    Installs or uninstalls go-mapi.

.DESCRIPTION
    One-liner installer/uninstaller for go-mapi. Downloads the latest release
    (or a specific version) from GitHub, installs binaries, registers the MAPI
    handler, and sets up native messaging for Chrome, Chromium, and Edge.

    Install (interactive):
      irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/install.ps1 | iex

    Uninstall:
      .\install.ps1 -Uninstall

.PARAMETER Uninstall
    Remove go-mapi: deletes registry entries, restores previous mail client,
    removes installed files.

.PARAMETER ExtensionId
    Chrome/Edge extension ID (32 lowercase letters). If not provided, the script
    will prompt interactively.

.PARAMETER Version
    Specific version to install (e.g., "v0.1.0"). Defaults to "latest".

.PARAMETER InstallDir
    Target directory. Defaults to "C:\Program Files\go-mapi".

.PARAMETER Local
    Install from local build artifacts instead of downloading from GitHub.
    Searches the repo build output directories automatically.

.PARAMETER BuildDir
    When used with -Local, specifies the directory containing built artifacts.

.PARAMETER KeepFiles
    When used with -Uninstall, only removes registry entries but keeps files.

.EXAMPLE
    # Interactive install (prompts for extension ID)
    irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/install.ps1 | iex

.EXAMPLE
    # Non-interactive install
    .\install.ps1 -ExtensionId "abcdefghijklmnopqrstuvwxyz123456"

.EXAMPLE
    # Developer: install from local build
    .\install.ps1 -ExtensionId "abc..." -Local

.EXAMPLE
    # Uninstall
    .\install.ps1 -Uninstall
#>

param(
    [switch]$Uninstall,

    [string]$ExtensionId,

    [string]$Version = "latest",

    [string]$InstallDir = "C:\Program Files\go-mapi",

    [switch]$Local,

    [string]$BuildDir,

    [switch]$KeepFiles
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$GH_REPO = "marcfargas/go-mapi"

# --- Helpers ---

function Write-Step { param([string]$Msg) Write-Host "  [+] $Msg" -ForegroundColor Green }
function Write-Info { param([string]$Msg) Write-Host "  [i] $Msg" -ForegroundColor Cyan }
function Write-Warn { param([string]$Msg) Write-Host "  [!] $Msg" -ForegroundColor Yellow }
function Write-Skip { param([string]$Msg) Write-Host "  [-] $Msg" -ForegroundColor DarkGray }

$browsers = @(
    @{ Name = "Chrome";   Path = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.gomapi.host" },
    @{ Name = "Chromium"; Path = "HKCU:\Software\Chromium\NativeMessagingHosts\com.gomapi.host" },
    @{ Name = "Edge";     Path = "HKCU:\Software\Microsoft\Edge\NativeMessagingHosts\com.gomapi.host" }
)

# ============================================================
#  UNINSTALL
# ============================================================

if ($Uninstall) {
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

    exit 0
}

# ============================================================
#  INSTALL
# ============================================================

function Get-LatestRelease {
    $url = "https://api.github.com/repos/$GH_REPO/releases/latest"
    try {
        $release = Invoke-RestMethod -Uri $url -Headers @{ Accept = "application/vnd.github.v3+json" }
        return $release
    } catch {
        if ($_.Exception.Response.StatusCode -eq 404) {
            Write-Error "No releases found at $url. Is the repository public?"
        }
        throw
    }
}

function Get-SpecificRelease {
    param([string]$Tag)
    $url = "https://api.github.com/repos/$GH_REPO/releases/tags/$Tag"
    try {
        return Invoke-RestMethod -Uri $url -Headers @{ Accept = "application/vnd.github.v3+json" }
    } catch {
        Write-Error "Release '$Tag' not found at $url"
        throw
    }
}

function Download-Asset {
    param([string]$Url, [string]$OutPath, [string]$Name)
    Write-Host "      Downloading $Name..." -NoNewline
    Invoke-WebRequest -Uri $Url -OutFile $OutPath -UseBasicParsing
    $size = [math]::Round((Get-Item $OutPath).Length / 1KB)
    Write-Host " ${size}KB" -ForegroundColor DarkGray
}

function Find-LocalArtifact {
    param([string]$FileName, [string[]]$SearchPaths)
    foreach ($dir in $SearchPaths) {
        $path = Join-Path $dir $FileName
        if (Test-Path $path) { return $path }
    }
    return $null
}

# --- Banner ---

Write-Host ""
Write-Host "  go-mapi installer" -ForegroundColor White
Write-Host "  ========================================" -ForegroundColor DarkGray

# --- Prompt for Extension ID if missing ---

if (-not $ExtensionId) {
    Write-Host ""
    Write-Info "The extension ID is required for native messaging."
    Write-Info "Find it at chrome://extensions (Developer mode ON)."
    Write-Host ""
    $ExtensionId = Read-Host "  Extension ID (32 chars)"
}

if ($ExtensionId -notmatch '^[a-z]{32}$') {
    Write-Error "Invalid extension ID: '$ExtensionId'. Must be 32 lowercase letters (from chrome://extensions)."
}

Write-Host ""
Write-Info "Install dir:  $InstallDir"
Write-Info "Extension ID: $ExtensionId"

# --- Acquire artifacts ---

$tempDir = Join-Path $env:TEMP "go-mapi-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    if ($Local) {
        # --- Local mode: find build artifacts ---
        Write-Host ""
        Write-Host "  Step 1: Locate build artifacts" -ForegroundColor White

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

        $dllSource = Find-LocalArtifact "go-mapi.dll" $searchPaths
        $hostSource = Find-LocalArtifact "go-mapi-host.exe" $searchPaths

        if (-not $dllSource) {
            Write-Error "go-mapi.dll not found. Run 'npm run build:interceptor' first.`nSearched: $($searchPaths -join ', ')"
        }
        if (-not $hostSource) {
            Write-Error "go-mapi-host.exe not found. Run 'npm run build:native-host' first.`nSearched: $($searchPaths -join ', ')"
        }

        Write-Step "Found DLL:  $dllSource"
        Write-Step "Found Host: $hostSource"

        $dllArtifact = $dllSource
        $hostArtifact = $hostSource
        $installedVersion = "local"

    } else {
        # --- Download mode: fetch from GitHub releases ---
        Write-Host ""
        Write-Host "  Step 1: Fetch release from GitHub" -ForegroundColor White

        if ($Version -eq "latest") {
            $release = Get-LatestRelease
        } else {
            $release = Get-SpecificRelease -Tag $Version
        }

        $tag = $release.tag_name
        Write-Step "Release: $tag ($($release.published_at.ToString('yyyy-MM-dd')))"

        # Find assets
        $dllAsset  = $release.assets | Where-Object { $_.name -eq "go-mapi.dll" }
        $hostAsset = $release.assets | Where-Object { $_.name -eq "go-mapi-host.exe" }

        if (-not $dllAsset) { Write-Error "Release $tag is missing go-mapi.dll asset" }
        if (-not $hostAsset) { Write-Error "Release $tag is missing go-mapi-host.exe asset" }

        # Download
        $dllArtifact = Join-Path $tempDir "go-mapi.dll"
        $hostArtifact = Join-Path $tempDir "go-mapi-host.exe"

        Download-Asset -Url $dllAsset.browser_download_url  -OutPath $dllArtifact  -Name "go-mapi.dll"
        Download-Asset -Url $hostAsset.browser_download_url -OutPath $hostArtifact -Name "go-mapi-host.exe"

        Write-Step "Downloaded $([math]::Round(($dllAsset.size + $hostAsset.size) / 1KB))KB total"

        $installedVersion = $tag
    }

    # --- Step 2: Install files ---

    Write-Host ""
    Write-Host "  Step 2: Install files" -ForegroundColor White

    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        Write-Step "Created: $InstallDir"
    }

    Copy-Item -Path $dllArtifact  -Destination (Join-Path $InstallDir "go-mapi.dll")      -Force
    Copy-Item -Path $hostArtifact -Destination (Join-Path $InstallDir "go-mapi-host.exe")  -Force
    Write-Step "Installed go-mapi.dll"
    Write-Step "Installed go-mapi-host.exe"

    # --- Step 3: Register MAPI handler ---

    Write-Host ""
    Write-Host "  Step 3: Register MAPI handler" -ForegroundColor White

    $mailClientsPath = "HKLM:\SOFTWARE\Clients\Mail"
    $goMapiRegPath   = "$mailClientsPath\go-mapi"
    $dllInstallPath  = Join-Path $InstallDir "go-mapi.dll"

    # Back up current default mail client
    $previousDefault = $null
    try {
        $previousDefault = (Get-ItemProperty -Path $mailClientsPath -Name "(Default)" -ErrorAction SilentlyContinue).'(Default)'
    } catch { }

    if ($previousDefault -and $previousDefault -ne "go-mapi") {
        $previousDefault | Out-File -FilePath (Join-Path $InstallDir ".previous-mail-client") -Encoding UTF8 -NoNewline
        Write-Info "Backed up previous default: $previousDefault"
    }

    # Register — DLLPath must be a VALUE on the key (not a subkey).
    # This is how mapi32.dll stub finds the actual MAPI DLL to load.
    New-Item -Path $goMapiRegPath -Force | Out-Null
    New-ItemProperty -Path $goMapiRegPath -Name "(Default)" -Value "go-mapi" -PropertyType String -Force | Out-Null
    New-ItemProperty -Path $goMapiRegPath -Name "DLLPath" -Value $dllInstallPath -PropertyType String -Force | Out-Null

    Set-ItemProperty -Path $mailClientsPath -Name "(Default)" -Value "go-mapi" -Force

    Write-Step "Registered MAPI handler"
    Write-Step "Set as default mail client"

    # --- Step 4: Native messaging ---

    Write-Host ""
    Write-Host "  Step 4: Register native messaging" -ForegroundColor White

    $manifest = @{
        name            = "com.gomapi.host"
        description     = "go-mapi Native Messaging Host"
        path            = (Join-Path $InstallDir "go-mapi-host.exe")
        type            = "stdio"
        allowed_origins = @("chrome-extension://$ExtensionId/")
    } | ConvertTo-Json -Depth 10

    $manifestPath = Join-Path $InstallDir "com.gomapi.host.json"
    $manifest | Out-File -FilePath $manifestPath -Encoding UTF8
    Write-Step "Created manifest: $manifestPath"

    foreach ($browser in $browsers) {
        $parentPath = Split-Path $browser.Path
        if (-not (Test-Path $parentPath)) {
            New-Item -Path $parentPath -Force | Out-Null
        }
        New-Item -Path $browser.Path -Force | Out-Null
        Set-ItemProperty -Path $browser.Path -Name "(Default)" -Value $manifestPath
        Write-Step "Registered $($browser.Name)"
    }

    # --- Save install metadata ---

    @{
        version         = $installedVersion
        installDir      = $InstallDir
        extensionId     = $ExtensionId
        installedAt     = (Get-Date -Format "o")
        previousClient  = $previousDefault
    } | ConvertTo-Json -Depth 10 | Out-File -FilePath (Join-Path $InstallDir ".install-metadata.json") -Encoding UTF8

    # --- Done ---

    Write-Host ""
    Write-Host "  ========================================" -ForegroundColor DarkGray
    Write-Host "  Installed go-mapi $installedVersion" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Install dir:      $InstallDir"
    Write-Host "  MAPI DLL:         $dllInstallPath"
    Write-Host "  Native host:      $(Join-Path $InstallDir 'go-mapi-host.exe')"
    Write-Host "  Default client:   go-mapi"
    if ($previousDefault -and $previousDefault -ne "go-mapi") {
        Write-Host "  Previous client:  $previousDefault (backed up)"
    }
    Write-Host ""
    Write-Host "  Next steps:" -ForegroundColor Cyan
    Write-Host "    1. Install the extension from Chrome Web Store (or load unpacked)"
    Write-Host "    2. Sign in with your Google account in the extension"
    Write-Host "    3. Right-click a file -> Send to -> Mail recipient"
    Write-Host ""
    Write-Host "  To uninstall:" -ForegroundColor DarkGray
    Write-Host "    .\install.ps1 -Uninstall" -ForegroundColor DarkGray
    Write-Host ""

} finally {
    # Clean up temp dir
    if (Test-Path $tempDir) {
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
