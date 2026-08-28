#Requires -Version 5.1
<#
.SYNOPSIS
  Sign a DraftHorse release ON THE ADMIN MACHINE and replace the published
  assets with signed ones. The signing key never leaves this machine.

.DESCRIPTION
  The release model (decided 2026-08-28): CI builds, tests, and publishes
  UNSIGNED assets for a tag; this script — run on the one machine that holds
  the Arrick's Propane certificate — turns that into the signed release:

    1. Downloads the tag's three binaries + CI's SHA256SUMS.txt
    2. VERIFIES the hashes — you sign exactly what CI built and tested
    3. Signs the binaries with the cert from this machine's store
       (imported non-exportable; signtool signs by thumbprint)
    4. Rebuilds the installer from the SIGNED binaries (makensis; every
       other packed file is in the repo checkout), then signs the installer
    5. Regenerates SHA256SUMS.txt (signing changes every hash)
    6. Replaces the release's assets on GitHub (asks first)

  One-time machine setup:
    - Install NSIS and the Windows SDK (for signtool); clone the repo
    - Import the certificate WITHOUT the exportable flag (the default):
        Import-PfxCertificate -FilePath DraftHorse-signing.pfx `
          -CertStoreLocation Cert:\CurrentUser\My -Password (Read-Host -AsSecureString)
    - Optionally run trust-signing-cert.ps1 here too, so the final
      Get-AuthenticodeSignature check reports Valid instead of merely Signed.

.PARAMETER Tag
  The release tag to sign, e.g. v4.0.0.

.PARAMETER Token
  GitHub token with repo scope (for replacing assets). Falls back to
  $env:GITHUB_TOKEN, then prompts.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File scripts\signing\sign-release.ps1 -Tag v4.0.0
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$Tag,
    [string]$Repo = 'egkrateia247/DraftHorse',
    [string]$Subject = "CN=Arrick's Propane",
    [string]$Token,
    [switch]$Force
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')

# ---- tools -----------------------------------------------------------------
$signtool = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\*\x64\signtool.exe" -ErrorAction SilentlyContinue |
    Sort-Object FullName | Select-Object -Last 1
if (-not $signtool) { throw 'signtool.exe not found — install the Windows SDK.' }
$makensis = Get-Command makensis -ErrorAction SilentlyContinue
if (-not $makensis) { throw 'makensis not found on PATH — install NSIS.' }

# ---- certificate (private key stays in this machine's store) ---------------
$cert = Get-ChildItem Cert:\CurrentUser\My |
    Where-Object { $_.Subject -eq $Subject -and $_.HasPrivateKey } |
    Sort-Object NotAfter | Select-Object -Last 1
if (-not $cert) {
    throw "No certificate '$Subject' with a private key in Cert:\CurrentUser\My. Import the PFX (see .DESCRIPTION) first."
}
Write-Host "Signing with $($cert.Subject) thumbprint $($cert.Thumbprint) (expires $($cert.NotAfter))" -ForegroundColor Cyan

function Invoke-Sign([string[]]$Files) {
    & $signtool.FullName sign /sha1 $cert.Thumbprint /s My /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 @Files
    if ($LASTEXITCODE -ne 0) { throw "signtool sign failed (exit $LASTEXITCODE)" }
}

# ---- download + verify CI's build ------------------------------------------
$work = Join-Path $env:TEMP "drafthorse-sign-$($Tag -replace '[^\w\.-]','_')"
Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $work | Out-Null
$base = "https://github.com/$Repo/releases/download/$Tag"
$binaries = 'DraftHorse.exe', 'DraftHorse-x64.dll', 'DraftHorse-x86.dll'
foreach ($f in $binaries + @('SHA256SUMS.txt')) {
    Write-Host "downloading $f"
    Invoke-WebRequest -Uri "$base/$f" -OutFile (Join-Path $work $f) -UseBasicParsing
}
$sums = @{}
Get-Content (Join-Path $work 'SHA256SUMS.txt') | ForEach-Object {
    if ($_ -match '^([0-9a-f]{64})\s+(\S+)$') { $sums[$Matches[2]] = $Matches[1] }
}
foreach ($f in $binaries) {
    $h = (Get-FileHash (Join-Path $work $f) -Algorithm SHA256).Hash.ToLower()
    if (-not $sums.ContainsKey($f)) { throw "SHA256SUMS.txt has no entry for $f" }
    if ($h -ne $sums[$f]) { throw "HASH MISMATCH for ${f}: got $h, CI published $($sums[$f]). Refusing to sign." }
    Write-Host "verified $f" -ForegroundColor Green
}

# ---- sign binaries, rebuild installer from them, sign it -------------------
Invoke-Sign @($binaries | ForEach-Object { Join-Path $work $_ })

# Stage into the layout go-mapi.nsi packs from (all other inputs are committed).
$stage = @{
    'DraftHorse.exe'     = 'src\app\build\bin\DraftHorse.exe'
    'DraftHorse-x64.dll' = 'src\interceptor\build-x64\bin\DraftHorse.dll'
    'DraftHorse-x86.dll' = 'src\interceptor\build-x86\bin\DraftHorse.dll'
}
foreach ($k in $stage.Keys) {
    $dst = Join-Path $repoRoot $stage[$k]
    New-Item -ItemType Directory -Force (Split-Path $dst) | Out-Null
    Copy-Item (Join-Path $work $k) $dst -Force
}
$version = $Tag -replace '^v', ''
Push-Location (Join-Path $repoRoot 'src\installer')
try {
    & $makensis.Source "/DGOMAPI_VERSION=$version" go-mapi.nsi
    if ($LASTEXITCODE -ne 0) { throw "makensis failed (exit $LASTEXITCODE)" }
    Move-Item 'DraftHorse-setup.exe' (Join-Path $work 'DraftHorse-setup.exe') -Force
} finally { Pop-Location }
Invoke-Sign @(Join-Path $work 'DraftHorse-setup.exe')

# ---- new manifest (every hash changed) -------------------------------------
$assets = @('DraftHorse-setup.exe') + $binaries
$lines = foreach ($f in $assets) {
    "{0}  {1}" -f (Get-FileHash (Join-Path $work $f) -Algorithm SHA256).Hash.ToLower(), $f
}
Set-Content -Path (Join-Path $work 'SHA256SUMS.txt') -Value ($lines -join "`n") -NoNewline -Encoding ascii

Write-Host "`nSignature check:" -ForegroundColor Cyan
foreach ($f in $assets) {
    $sig = Get-AuthenticodeSignature (Join-Path $work $f)
    Write-Host ("  {0,-22} {1}" -f $f, $sig.Status)
}
Write-Host '(Valid needs this machine to trust the cert; UnknownError still means signed.)'

# ---- replace release assets -------------------------------------------------
if (-not $Token) { $Token = $env:GITHUB_TOKEN }
if (-not $Token) {
    $sec = Read-Host -AsSecureString -Prompt 'GitHub token (repo scope)'
    $Token = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec))
}
$hdr = @{ Authorization = "token $Token"; Accept = 'application/vnd.github+json' }
$rel = Invoke-RestMethod -Headers $hdr -Uri "https://api.github.com/repos/$Repo/releases/tags/$Tag"

if (-not $Force) {
    $answer = Read-Host "Replace the $($rel.assets.Count) assets on release $Tag with the signed set? (yes/no)"
    if ($answer -ne 'yes') { Write-Host "Aborted. Signed files remain in $work"; return }
}
$replace = $assets + @('SHA256SUMS.txt')
foreach ($a in $rel.assets) {
    if ($replace -contains $a.name) {
        Write-Host "deleting old asset $($a.name)"
        Invoke-RestMethod -Method Delete -Headers $hdr -Uri "https://api.github.com/repos/$Repo/releases/assets/$($a.id)" | Out-Null
    }
}
foreach ($f in $replace) {
    Write-Host "uploading signed $f"
    $uploadUri = "https://uploads.github.com/repos/$Repo/releases/$($rel.id)/assets?name=$f"
    Invoke-RestMethod -Method Post -Headers ($hdr + @{ 'Content-Type' = 'application/octet-stream' }) `
        -Uri $uploadUri -InFile (Join-Path $work $f) | Out-Null
}
Write-Host "`nDone: $Tag now serves signed assets. Working copies kept in $work" -ForegroundColor Green
Write-Host 'Archive DraftHorse-setup.exe + SHA256SUMS.txt from there with the release record.'
