#Requires -RunAsAdministrator
#Requires -Version 5.1
<#
.SYNOPSIS
  Remove every pre-4.0 "go-mapi" registry trace so DraftHorse 4.0 can be the
  default everywhere. Run BEFORE installing DraftHorse-setup.exe 4.0 on a
  machine that ever had a 3.x install.

.DESCRIPTION
  The 4.0 installer and app migrate the core state (client key, data dirs,
  credentials, heal mirror, Applications keys). What they deliberately do NOT
  touch is per-user Default-Apps plumbing that may still point at the old
  name: UserChoice keys, Open-with lists, and association-toast markers.
  Dave's first migration retest showed those keep "go-mapi" rows alive in the
  Settings pickers until hand-deleted. This script deletes exactly the
  go-mapi-referencing entries — nothing else — and is safe to run twice, on a
  clean machine, or after 4.0 is already installed.

  Deleting a UserChoice key is allowed (Windows re-prompts on next use);
  SETTING one programmatically is hash-protected/UCPD-blocked, which is why
  the script never writes a replacement — pick DraftHorse once when asked.

  HKCU sections apply to the user running the script. On a machine with
  multiple profiles that used go-mapi, run once per user (the HKLM half is
  idempotent and just no-ops the second time).

.PARAMETER ReportOnly
  Show what would be deleted without deleting anything.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\cleanup-legacy-gomapi.ps1
#>
[CmdletBinding()]
param([switch]$ReportOnly)

Set-StrictMode -Version Latest
$removed = 0; $absent = 0

function Act {
    param([string]$What, [scriptblock]$Delete)
    if ($ReportOnly) { Write-Host "WOULD REMOVE: $What" -ForegroundColor Yellow; $script:removed++; return }
    try { & $Delete; Write-Host "removed: $What" -ForegroundColor Green; $script:removed++ }
    catch { Write-Host "FAILED:  $What — $($_.Exception.Message)" -ForegroundColor Red }
}
function Remove-KeyIfPresent([string]$Path, [string]$What) {
    if (Test-Path -LiteralPath $Path) { Act $What { Remove-Item -LiteralPath $Path -Recurse -Force } }
    else { Write-Host "absent:  $What"; $script:absent++ }
}
function Remove-ValueIfPresent([string]$Path, [string]$Name, [string]$What) {
    $p = Get-ItemProperty -LiteralPath $Path -Name $Name -ErrorAction SilentlyContinue
    if ($null -ne $p) { Act $What { Remove-ItemProperty -LiteralPath $Path -Name $Name -Force } }
    else { Write-Host "absent:  $What"; $script:absent++ }
}

Write-Host "== go-mapi legacy cleanup (pre-DraftHorse-4.0) ==" -ForegroundColor Cyan
if ($ReportOnly) { Write-Host "(report only — nothing will be deleted)" -ForegroundColor Yellow }

# Close the old app so nothing recreates keys mid-sweep.
Get-Process -Name 'go-mapi' -ErrorAction SilentlyContinue | ForEach-Object {
    Act "running go-mapi.exe (pid $($_.Id))" { $_ | Stop-Process -Force }
}

# ---- Machine scope (HKLM) --------------------------------------------------
Remove-KeyIfPresent 'HKLM:\SOFTWARE\Clients\Mail\go-mapi'                   'HKLM Clients\Mail\go-mapi (MAPI client key)'
Remove-KeyIfPresent 'HKLM:\SOFTWARE\Classes\go-mapi.mailto'                 'HKLM Classes\go-mapi.mailto (ProgID)'
Remove-KeyIfPresent 'HKLM:\SOFTWARE\Classes\Applications\go-mapi.exe'       'HKLM Classes\Applications\go-mapi.exe (browsed-app row)'
Remove-ValueIfPresent 'HKLM:\SOFTWARE\RegisteredApplications' 'go-mapi'     'HKLM RegisteredApplications\go-mapi'
Remove-KeyIfPresent 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\go-mapi'             'HKLM Uninstall\go-mapi (native view)'
Remove-KeyIfPresent 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\go-mapi' 'HKLM Uninstall\go-mapi (WOW6432 view)'
Remove-ValueIfPresent 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run' 'go-mapi'               'HKLM Run\go-mapi (autostart)'

# Resolver default: only if it still names go-mapi (never clobber another client).
$mailDefault = (Get-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\Clients\Mail' -ErrorAction SilentlyContinue).'(default)'
if ($mailDefault -eq 'go-mapi') {
    Act 'HKLM Clients\Mail (Default) = go-mapi (resolver)' {
        Remove-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\Clients\Mail' -Name '(default)' -Force }
} else { Write-Host "absent:  HKLM Clients\Mail (Default) does not name go-mapi (is: '$mailDefault')"; $absent++ }

# Old firewall rule (registry-backed; netsh is the supported delete path).
$fw = netsh advfirewall firewall show rule name="go-mapi OAuth loopback" 2>&1 | Out-String
if ($fw -notmatch 'No rules match') {
    Act 'firewall rule "go-mapi OAuth loopback"' {
        netsh advfirewall firewall delete rule name="go-mapi OAuth loopback" | Out-Null }
} else { Write-Host 'absent:  firewall rule "go-mapi OAuth loopback"'; $absent++ }

# ---- Per-user scope (HKCU — the current user) ------------------------------
Remove-KeyIfPresent 'HKCU:\Software\Clients\Mail\go-mapi'             'HKCU Clients\Mail\go-mapi (heal mirror)'
Remove-KeyIfPresent 'HKCU:\Software\Classes\go-mapi.mailto'           'HKCU Classes\go-mapi.mailto'
Remove-KeyIfPresent 'HKCU:\Software\Classes\Applications\go-mapi.exe' 'HKCU Classes\Applications\go-mapi.exe (browsed-app row)'
Remove-ValueIfPresent 'HKCU:\Software\RegisteredApplications' 'go-mapi' 'HKCU RegisteredApplications\go-mapi'

$cuDefault = (Get-ItemProperty -LiteralPath 'HKCU:\Software\Clients\Mail' -ErrorAction SilentlyContinue).'(default)'
if ($cuDefault -eq 'go-mapi') {
    Act 'HKCU Clients\Mail (Default) = go-mapi' {
        Remove-ItemProperty -LiteralPath 'HKCU:\Software\Clients\Mail' -Name '(default)' -Force }
}

# mailto UserChoice → go-mapi.mailto: delete so Windows re-prompts (pick
# DraftHorse once). Deleting is allowed; writing a replacement is not.
$mailtoUC = 'HKCU:\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\mailto\UserChoice'
$ucProg = (Get-ItemProperty -LiteralPath $mailtoUC -ErrorAction SilentlyContinue).ProgId
if ($ucProg -like '*go-mapi*') {
    Act "mailto UserChoice (ProgId=$ucProg)" { Remove-Item -LiteralPath $mailtoUC -Recurse -Force }
} else { Write-Host "absent:  mailto UserChoice does not reference go-mapi (is: '$ucProg')"; $absent++ }

# File-extension plumbing for every extension go-mapi ever appeared under.
foreach ($ext in @('.eml', '.MAPIMail')) {
    $fx = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\$ext"
    if (-not (Test-Path -LiteralPath $fx)) { Write-Host "absent:  FileExts\$ext"; $absent++; continue }

    # UserChoice pointing at the old exe/ProgID → delete key (re-prompts).
    $uc = "$fx\UserChoice"
    $prog = (Get-ItemProperty -LiteralPath $uc -ErrorAction SilentlyContinue).ProgId
    if ($prog -like '*go-mapi*') {
        Act "FileExts\$ext UserChoice (ProgId=$prog)" { Remove-Item -LiteralPath $uc -Recurse -Force }
    }

    # OpenWithProgids values naming a go-mapi ProgID.
    $owp = "$fx\OpenWithProgids"
    if (Test-Path -LiteralPath $owp) {
        (Get-Item -LiteralPath $owp).Property | Where-Object { $_ -like '*go-mapi*' } | ForEach-Object {
            $name = $_
            Act "FileExts\$ext OpenWithProgids\$name" { Remove-ItemProperty -LiteralPath $owp -Name $name -Force }
        }
    }

    # OpenWithList letter slots whose data is go-mapi.exe (+ prune MRUList).
    $owl = "$fx\OpenWithList"
    if (Test-Path -LiteralPath $owl) {
        $item = Get-Item -LiteralPath $owl
        $mru  = (Get-ItemProperty -LiteralPath $owl -ErrorAction SilentlyContinue).MRUList
        foreach ($name in @($item.Property | Where-Object { $_ -ne 'MRUList' })) {
            if (($item.GetValue($name)) -like '*go-mapi*') {
                Act "FileExts\$ext OpenWithList\$name (go-mapi.exe)" {
                    Remove-ItemProperty -LiteralPath $owl -Name $name -Force
                    if ($mru) { Set-ItemProperty -LiteralPath $owl -Name MRUList -Value ($mru -replace [regex]::Escape($name), '') }
                }
            }
        }
    }
}

# Association-toast markers ("new app installed" bookkeeping).
$aat = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\ApplicationAssociationToasts'
if (Test-Path -LiteralPath $aat) {
    (Get-Item -LiteralPath $aat).Property | Where-Object { $_ -like '*go-mapi*' } | ForEach-Object {
        $name = $_
        Act "ApplicationAssociationToasts\$name" { Remove-ItemProperty -LiteralPath $aat -Name $name -Force }
    }
}

# ---- Tell the shell associations changed (pickers re-read) -----------------
if (-not $ReportOnly) {
    Add-Type -Namespace Win32 -Name Shell -MemberDefinition '[DllImport("shell32.dll")] public static extern void SHChangeNotify(int eventId, int flags, IntPtr i1, IntPtr i2);'
    [Win32.Shell]::SHChangeNotify(0x08000000, 0, [IntPtr]::Zero, [IntPtr]::Zero)  # SHCNE_ASSOCCHANGED
}

Write-Host ""
Write-Host ("== done: {0} removed, {1} already absent ==" -f $removed, $absent) -ForegroundColor Cyan
Write-Host "Next: install DraftHorse-setup.exe. If a picker still shows a stale row,"
Write-Host "sign out and back in to Windows (Explorer caches the app list per session)."
