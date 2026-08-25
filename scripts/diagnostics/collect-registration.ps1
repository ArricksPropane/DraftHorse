<#
.SYNOPSIS
  Collect DraftHorse MAPI registration + DLL diagnostics into a timestamped text file.

.DESCRIPTION
  Produces a single text report at $OutputDir\DraftHorse-registration-<yyyyMMdd-HHmmss>.txt
  covering:
    1. Header (host, user, OS bitness, process bitness)
    2. HKLM mail clients (native view)
    3. HKLM mail clients (WOW6432 view)
    4. HKLM DraftHorse registration (both views)
    5. DLL presence, size, SHA256, PE bitness (32 vs 64)
    6. DLL export probe via LoadLibraryEx + GetProcAddress
    7. Footer

  Intended for inclusion in bug reports. Only the final "Report written to: …" line
  is echoed to the console.

.NOTES
  Added by quick/260423-msq.
  Window-PowerShell 5.1 compatible (do not require PowerShell 7).
  The export probe runs in the CURRENT process's architecture — re-run this script
  from C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe to probe the 32-bit DLL.
#>
#Requires -Version 5.1

[CmdletBinding()]
param(
    [string]$OutputDir = "$env:USERPROFILE\Desktop"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Continue'

$scriptVersion = '2.0'   # V4: all identifiers renamed go-mapi -> DraftHorse; report now also flags stale go-mapi keys (pre-4.0 leftovers)
$timestamp = Get-Date -Format 'yyyyMMdd-HHmmss'

if (-not (Test-Path -LiteralPath $OutputDir)) {
    try {
        New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
    } catch {
        Write-Error "Failed to create OutputDir '$OutputDir': $($_.Exception.Message)"
        exit 1
    }
}

$out = Join-Path $OutputDir "DraftHorse-registration-$timestamp.txt"

function Append-Banner {
    param([string]$Title)
    Add-Content -LiteralPath $out -Value ''
    Add-Content -LiteralPath $out -Value ('=' * 72)
    Add-Content -LiteralPath $out -Value "=== Section: $Title ==="
    Add-Content -LiteralPath $out -Value ('=' * 72)
}

function Append-Line {
    param([string]$Line = '')
    Add-Content -LiteralPath $out -Value $Line
}

function Append-Block {
    param([Parameter(ValueFromPipeline = $true)]$Input)
    process {
        if ($null -eq $Input) { return }
        $text = $Input | Out-String -Width 240
        $text = $text.TrimEnd("`r", "`n")
        if ($text.Length -gt 0) {
            Add-Content -LiteralPath $out -Value $text
        }
    }
}

function Safe-Invoke {
    param(
        [string]$Description,
        [scriptblock]$Action
    )
    try {
        & $Action
    } catch {
        Append-Line "[$Description] ERROR: $($_.Exception.Message)"
    }
}

# -----------------------------------------------------------------------------
# Section 1: Header
# -----------------------------------------------------------------------------
Append-Banner 'Header'
Append-Line "DraftHorse registration report (script v$scriptVersion)"
Append-Line "Timestamp     : $(Get-Date -Format 'yyyy-MM-ddTHH:mm:sszzz')"
Append-Line "Computer      : $env:COMPUTERNAME"
Append-Line "User          : $env:USERNAME"
Append-Line "OS            : $([Environment]::OSVersion.VersionString)"
Append-Line "Is64BitOS     : $([Environment]::Is64BitOperatingSystem)"
Append-Line "Is64BitProcess: $([Environment]::Is64BitProcess)"
Append-Line "PSVersion     : $($PSVersionTable.PSVersion)"

# -----------------------------------------------------------------------------
# Section 2: HKLM mail clients (native view)
# -----------------------------------------------------------------------------
Append-Banner 'HKLM mail clients (native view)'
Safe-Invoke 'HKLM Mail clients' {
    Get-ChildItem -LiteralPath 'HKLM:\SOFTWARE\Clients\Mail' -ErrorAction Stop |
        Select-Object Name, PSChildName |
        Append-Block
    Append-Line ''
    Append-Line 'Default value at HKLM:\SOFTWARE\Clients\Mail :'
    # Guarded (Default) read — under StrictMode, referencing a non-existent
    # property throws NullReference. A fresh install may have no (Default)
    # value under HKLM\SOFTWARE\Clients\Mail (see tests/live/*-165325.txt).
    # Also, piping the registry PSObject through `Format-List *` tickles a
    # StrictMode NullReference inside the formatter on some Windows locales
    # (observed on es-ES Windows 11) — iterate user-facing properties
    # ourselves to stay within the StrictMode envelope.
    $props = Get-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\Clients\Mail' -ErrorAction SilentlyContinue
    if ($props -and ($props.PSObject.Properties.Name -contains '(default)')) {
        Append-Line "  (default) = $($props.'(default)')"
    } else {
        Append-Line "  (default) = <not set>"
    }
    if ($props) {
        foreach ($p in $props.PSObject.Properties) {
            if ($p.Name -like 'PS*') { continue }
            if ($p.Name -eq '(default)') { continue }
            Append-Line "  $($p.Name) = $($p.Value)"
        }
    }
}

# -----------------------------------------------------------------------------
# Section 3: HKLM mail clients (WOW6432 view)
# -----------------------------------------------------------------------------
Append-Banner 'HKLM mail clients (WOW6432 view)'
Safe-Invoke 'HKLM WOW6432 Mail clients' {
    if (Test-Path -LiteralPath 'HKLM:\SOFTWARE\WOW6432Node\Clients\Mail') {
        Get-ChildItem -LiteralPath 'HKLM:\SOFTWARE\WOW6432Node\Clients\Mail' -ErrorAction Stop |
            Select-Object Name, PSChildName |
            Append-Block
        Append-Line ''
        Append-Line 'Default value at HKLM:\SOFTWARE\WOW6432Node\Clients\Mail :'
        # Guarded (Default) read + manual property enumeration — see
        # native-view comment above for the StrictMode + Format-List rationale.
        $props = Get-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\WOW6432Node\Clients\Mail' -ErrorAction SilentlyContinue
        if ($props -and ($props.PSObject.Properties.Name -contains '(default)')) {
            Append-Line "  (default) = $($props.'(default)')"
        } else {
            Append-Line "  (default) = <not set>"
        }
        if ($props) {
            foreach ($p in $props.PSObject.Properties) {
                if ($p.Name -like 'PS*') { continue }
                if ($p.Name -eq '(default)') { continue }
                Append-Line "  $($p.Name) = $($p.Value)"
            }
        }
    } else {
        Append-Line '(No HKLM:\SOFTWARE\WOW6432Node\Clients\Mail key — 32-bit view not present)'
    }
}

# -----------------------------------------------------------------------------
# Section 4: HKLM DraftHorse registration (native + WOW6432)
# -----------------------------------------------------------------------------
Append-Banner 'HKLM DraftHorse registration'
foreach ($root in @('HKLM:\SOFTWARE\Clients\Mail\DraftHorse',
                    'HKLM:\SOFTWARE\WOW6432Node\Clients\Mail\DraftHorse')) {
    Append-Line ''
    Append-Line "Key: $root"
    Safe-Invoke "Dump $root" {
        if (Test-Path -LiteralPath $root) {
            # Guarded property dump — under StrictMode, piping the registry
            # PSObject through `Format-List *` has been observed to throw
            # NullReference on some Windows locales (es-ES Windows 11),
            # presumably from an internal formatter property access.
            # Enumerate user-facing properties ourselves instead.
            $props = Get-ItemProperty -LiteralPath $root -ErrorAction SilentlyContinue
            if ($props) {
                foreach ($p in $props.PSObject.Properties) {
                    if ($p.Name -like 'PS*') { continue }
                    Append-Line "  $($p.Name) = $($p.Value)"
                }
            } else {
                Append-Line '  (no values on this key)'
            }
            # Dump all subkeys recursively
            Get-ChildItem -LiteralPath $root -Recurse -ErrorAction SilentlyContinue |
                ForEach-Object {
                    Append-Line ''
                    Append-Line "Subkey: $($_.Name)"
                    $subProps = Get-ItemProperty -LiteralPath $_.PSPath -ErrorAction SilentlyContinue
                    if ($subProps) {
                        foreach ($p in $subProps.PSObject.Properties) {
                            if ($p.Name -like 'PS*') { continue }
                            Append-Line "  $($p.Name) = $($p.Value)"
                        }
                    } else {
                        Append-Line '  (no values on this subkey)'
                    }
                }
        } else {
            Append-Line '(key not present)'
        }
    }
}

# -----------------------------------------------------------------------------
# Section 4a (v2.0 / V4): stale pre-4.0 go-mapi leftovers. All four locations
# are scrubbed by the 4.0 installer (machine scope) or migrate.go (per-user);
# anything listed here survived migration and explains "worked before upgrade".
Append-Banner 'STALE PRE-4.0 (go-mapi) LEFTOVERS — all should say ABSENT'
foreach ($stale in @(
    'HKLM:\SOFTWARE\Clients\Mail\go-mapi',
    'HKCU:\Software\Clients\Mail\go-mapi',
    'HKLM:\SOFTWARE\Classes\go-mapi.mailto',
    'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\go-mapi'
)) {
    if (Test-Path -LiteralPath $stale) {
        Append-Line "PRESENT (stale — migration missed it): $stale"
    } else {
        Append-Line "absent: $stale"
    }
}
foreach ($staleDir in @(
    (Join-Path $env:ProgramFiles 'go-mapi'),
    (Join-Path $env:LOCALAPPDATA 'go-mapi'),
    (Join-Path $env:APPDATA 'go-mapi')
)) {
    if (Test-Path -LiteralPath $staleDir) {
        Append-Line "PRESENT (stale dir): $staleDir"
    } else {
        Append-Line "absent: $staleDir"
    }
}

# Section 4b (v1.1 / ARRICKS-19): HKCU self-heal layer + mailto UserChoice +
# autostart + running-process state — the surfaces the ARRICKS-13 guard and
# the ARRICKS-19 autostart operate on.
# -----------------------------------------------------------------------------
Append-Banner 'HKCU mail-client layer (ARRICKS-13 self-heal)'
Safe-Invoke 'HKCU Clients\Mail' {
    if (Test-Path -LiteralPath 'HKCU:\Software\Clients\Mail') {
        $props = Get-ItemProperty -LiteralPath 'HKCU:\Software\Clients\Mail' -ErrorAction SilentlyContinue
        if ($props -and ($props.PSObject.Properties.Name -contains '(default)')) {
            Append-Line "  HKCU (default) = $($props.'(default)')"
        } else {
            Append-Line '  HKCU (default) = <not set>'
        }
        if (Test-Path -LiteralPath 'HKCU:\Software\Clients\Mail\DraftHorse') {
            $sub = Get-ItemProperty -LiteralPath 'HKCU:\Software\Clients\Mail\DraftHorse' -ErrorAction SilentlyContinue
            if ($sub) {
                foreach ($p in $sub.PSObject.Properties) {
                    if ($p.Name -like 'PS*') { continue }
                    Append-Line "  DraftHorse mirror: $($p.Name) = $($p.Value)"
                }
            }
        } else {
            Append-Line '  (no HKCU DraftHorse mirror — guard has not needed to self-heal)'
        }
    } else {
        Append-Line '(no HKCU:\Software\Clients\Mail key)'
    }
}

Append-Banner 'mailto UserChoice (Settings > Default apps > Email link handler)'
Safe-Invoke 'mailto UserChoice' {
    $uc = 'HKCU:\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\mailto\UserChoice'
    if (Test-Path -LiteralPath $uc) {
        $p = Get-ItemProperty -LiteralPath $uc -ErrorAction SilentlyContinue
        if ($p -and ($p.PSObject.Properties.Name -contains 'ProgId')) {
            Append-Line "  ProgId = $($p.ProgId)   (DraftHorse.mailto = DraftHorse)"
        } else {
            Append-Line '  ProgId = <not set>'
        }
    } else {
        Append-Line '(no mailto UserChoice key — no explicit choice made yet)'
    }
}

Append-Banner 'Autostart + running processes (ARRICKS-19)'
Safe-Invoke 'Run key' {
    $run = Get-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run' -ErrorAction SilentlyContinue
    if ($run -and ($run.PSObject.Properties.Name -contains 'DraftHorse')) {
        Append-Line "  HKLM Run\DraftHorse = $($run.'DraftHorse')"
    } else {
        Append-Line '  HKLM Run\DraftHorse = <not registered> (pre-ARRICKS-19 install?)'
    }
}
Safe-Invoke 'Processes' {
    $procs = Get-Process -Name 'DraftHorse' -ErrorAction SilentlyContinue
    if ($procs) {
        foreach ($p in $procs) {
            Append-Line "  DraftHorse.exe running: PID $($p.Id), started $($p.StartTime)"
        }
    } else {
        Append-Line '  DraftHorse.exe NOT RUNNING — queue watcher and default-mail guard are inactive'
    }
    foreach ($name in @('OUTLOOK', 'olk')) {
        $o = Get-Process -Name $name -ErrorAction SilentlyContinue
        if ($o) { Append-Line "  $name.exe running: $((@($o)).Count) instance(s)" }
    }
}

# -----------------------------------------------------------------------------
# Section 4c (v1.2 / ARRICKS-20): "Send to -> Mail recipient" plumbing.
# -----------------------------------------------------------------------------
Append-Banner 'SendTo plumbing (Mail Recipient entry)'
Safe-Invoke 'SendTo file' {
    $sendTo = Join-Path $env:APPDATA 'Microsoft\Windows\SendTo'
    $entry = Join-Path $sendTo 'Mail Recipient.MAPIMail'
    Append-Line "  SendTo dir            : $sendTo"
    Append-Line "  Mail Recipient.MAPIMail present: $(Test-Path -LiteralPath $entry)"
    if (Test-Path -LiteralPath $sendTo) {
        Append-Line '  SendTo dir contents:'
        Get-ChildItem -LiteralPath $sendTo -ErrorAction SilentlyContinue |
            ForEach-Object { Append-Line "    $($_.Name)" }
    }
}
Safe-Invoke '.MAPIMail association' {
    $hkcr = 'Registry::HKEY_CLASSES_ROOT\.MAPIMail'
    if (Test-Path -LiteralPath $hkcr) {
        $p = Get-ItemProperty -LiteralPath $hkcr -ErrorAction SilentlyContinue
        if ($p -and ($p.PSObject.Properties.Name -contains '(default)')) {
            Append-Line "  HKCR\.MAPIMail (default) = $($p.'(default)')"
        } else {
            Append-Line '  HKCR\.MAPIMail (default) = <not set>'
        }
    } else {
        Append-Line '  HKCR\.MAPIMail = <KEY MISSING> (extension association stripped)'
    }
    $uc = 'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.MAPIMail\UserChoice'
    if (Test-Path -LiteralPath $uc) {
        $p = Get-ItemProperty -LiteralPath $uc -ErrorAction SilentlyContinue
        $progId = if ($p -and ($p.PSObject.Properties.Name -contains 'ProgId')) { $p.ProgId } else { '<none>' }
        Append-Line "  FileExts\.MAPIMail\UserChoice ProgId = $progId  (stale entries here break the menu)"
    } else {
        Append-Line '  FileExts\.MAPIMail\UserChoice = <not present> (good)'
    }
}

# -----------------------------------------------------------------------------
# Section 5: DLL presence + PE bitness + hash
# -----------------------------------------------------------------------------
Append-Banner 'DLL presence / PE bitness / SHA256'

$dllCandidates = @(
    "$env:ProgramFiles\DraftHorse\DraftHorse.dll",
    "${env:ProgramFiles(x86)}\DraftHorse\DraftHorse.dll"
)

function Get-PEBitness {
    param([string]$Path)
    try {
        $bytes = [System.IO.File]::ReadAllBytes($Path)
        if ($bytes.Length -lt 0x40) { return 'unknown (file too small)' }
        # e_lfanew is a 4-byte little-endian value at offset 0x3C.
        $e_lfanew = [BitConverter]::ToInt32($bytes, 0x3C)
        if ($bytes.Length -lt ($e_lfanew + 0x18 + 2)) { return 'unknown (truncated PE)' }
        # "PE\0\0" + FileHeader(20 bytes) + OptionalHeader magic(2 bytes).
        $magic = [BitConverter]::ToUInt16($bytes, $e_lfanew + 4 + 20)
        switch ($magic) {
            0x10B { return 'PE32 (32-bit / x86)' }
            0x20B { return 'PE32+ (64-bit / x64 or arm64)' }
            default { return ('unknown (magic=0x{0:X4})' -f $magic) }
        }
    } catch {
        return "error: $($_.Exception.Message)"
    }
}

foreach ($dll in $dllCandidates) {
    Append-Line ''
    Append-Line "Candidate path: $dll"
    if (Test-Path -LiteralPath $dll) {
        try {
            $item = Get-Item -LiteralPath $dll
            Append-Line "  Present        : YES"
            Append-Line "  Size (bytes)   : $($item.Length)"
            Append-Line "  LastWriteTime  : $($item.LastWriteTime)"
            $bitness = Get-PEBitness -Path $dll
            Append-Line "  PE bitness     : $bitness"
            $hash = Get-FileHash -LiteralPath $dll -Algorithm SHA256
            Append-Line "  SHA256         : $($hash.Hash)"
        } catch {
            Append-Line "  ERROR inspecting DLL: $($_.Exception.Message)"
        }
    } else {
        Append-Line "  Present        : NO"
    }
}

# -----------------------------------------------------------------------------
# Section 6: DLL export probe (LoadLibraryEx + GetProcAddress)
# -----------------------------------------------------------------------------
Append-Banner 'DLL export probe (in-process)'
Append-Line 'Probes the CURRENT PowerShell process architecture.'
Append-Line "CurrentProcessIs64Bit: $([Environment]::Is64BitProcess)"
Append-Line 'Re-run via C:\Windows\SysWOW64\WindowsPowerShell\v1.0\powershell.exe to probe the 32-bit DLL.'

$csharp = @'
using System;
using System.Runtime.InteropServices;

public static class GoMapiProbe
{
    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    public static extern IntPtr LoadLibraryExW(string lpFileName, IntPtr hFile, uint dwFlags);

    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern bool FreeLibrary(IntPtr hModule);

    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Ansi)]
    public static extern IntPtr GetProcAddress(IntPtr hModule, string lpProcName);

    [DllImport("kernel32.dll")]
    public static extern uint GetLastError();

    public const uint DONT_RESOLVE_DLL_REFERENCES = 0x00000001;
    public const uint LOAD_LIBRARY_AS_DATAFILE   = 0x00000002;
}
'@

try {
    Add-Type -TypeDefinition $csharp -ErrorAction Stop
} catch {
    Append-Line "Could not compile probe shim: $($_.Exception.Message)"
    Append-Line 'Skipping export probe.'
}

$expectedExports = @(
    'MAPISendMail',
    'MAPISendMailW',
    'MAPILogon',
    'MAPILogoff',
    'MAPIFreeBuffer',
    'MAPISendDocuments'
)

foreach ($dll in $dllCandidates) {
    Append-Line ''
    Append-Line "Probing: $dll"
    if (-not (Test-Path -LiteralPath $dll)) {
        Append-Line '  (not present — skipped)'
        continue
    }
    # DONT_RESOLVE_DLL_REFERENCES lets us inspect exports without running DllMain.
    $h = [GoMapiProbe]::LoadLibraryExW($dll, [IntPtr]::Zero, [GoMapiProbe]::DONT_RESOLVE_DLL_REFERENCES)
    if ($h -eq [IntPtr]::Zero) {
        $err = [GoMapiProbe]::GetLastError()
        Append-Line ('  LoadLibraryEx failed (GetLastError=0x{0:X8} / {1})' -f $err, $err)
        continue
    }
    try {
        foreach ($exp in $expectedExports) {
            $addr = [GoMapiProbe]::GetProcAddress($h, $exp)
            if ($addr -eq [IntPtr]::Zero) {
                $err = [GoMapiProbe]::GetLastError()
                Append-Line ('  {0,-20} NOT FOUND (GetLastError=0x{1:X8})' -f $exp, $err)
            } else {
                Append-Line ('  {0,-20} found  at 0x{1:X}' -f $exp, $addr.ToInt64())
            }
        }
    } finally {
        [void][GoMapiProbe]::FreeLibrary($h)
    }
}

# -----------------------------------------------------------------------------
# Footer
# -----------------------------------------------------------------------------
Append-Banner 'Footer'
Append-Line "End of report ($([DateTime]::Now.ToString('o')))"

Write-Host "Report written to: $out"
