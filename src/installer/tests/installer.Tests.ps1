# src/installer/tests/installer.Tests.ps1
# Pester 5 smoke test — D-21 13-item coverage.
#
# Pester 5 idioms only: New-PesterConfiguration, Describe/Context/It, Should -BeTrue/-BeFalse.
# Pester 4 EnableExit switch is forbidden (D-30).
#
# MUST run on an ephemeral CI runner (windows-latest) — this suite invokes
# go-mapi-setup.exe /S /D=... which actually writes to HKLM, ProgramFiles,
# Start Menu, and Windows Firewall. Running on a developer workstation will
# modify the system.
#
# D-21 item coverage (13 items, split across two Context blocks):
#   Silent install:   1 (exit code), 2 (binaries), 3 (MAPI key + DLLPath),
#                     4 (backup JSON shape), 5 (shortcut + AUMID),
#                     6 (firewall rule present)
#   Silent uninstall: 7 (exit code), 8 (install dir gone), 9 (MAPI key gone),
#                     10 (firewall rule gone), 11 (%APPDATA% gone),
#                     12 (Credential Manager scrubbed), 13 (shortcut gone)
#
# Cross-plan literal contract (byte-for-byte match with 10-03 + 10-04):
#   AUMID         = com.marcfargas.gomapi    (NOT com.marcfargas.gomapi.dev)
#   Firewall rule = go-mapi OAuth loopback   (match 10-03 AddFirewallRule + 10-04 RemoveFirewallRule)
#   Cred target   = go-mapi:oauth-tokens     (COLON separator — zalando/go-keyring Windows backend)

BeforeAll {
    # Dot-source the AUMID reader helper (defines Get-ShortcutAumid + .NET types).
    . "$PSScriptRoot\AumidReader.ps1"

    # The installer binary is produced by the CI workflow (installer-smoke.yml)
    # via `makensis src\installer\go-mapi.nsi` at the repo root.
    # Path resolution:
    #   From src/installer/tests/installer.Tests.ps1 ..\..\..\ = repo root
    $script:SetupExe     = Join-Path $PSScriptRoot '..\..\..\go-mapi-setup.exe' | Resolve-Path -ErrorAction Stop | ForEach-Object Path
    $script:InstallDir   = "$env:ProgramFiles\go-mapi"
    $script:ProgramData  = "$env:ProgramData\go-mapi"
    $script:BackupJson   = "$script:ProgramData\uninst\previous-mail-client.json"
    $script:MapiKey      = 'HKLM:\SOFTWARE\Clients\Mail\go-mapi'
    $script:MailKey      = 'HKLM:\SOFTWARE\Clients\Mail'
    # ARRICKS-11: display-name rebrand — the shortcut carries the display
    # name (it is also what toasts show for the stamped AUMID).
    $script:Shortcut     = "$env:ProgramData\Microsoft\Windows\Start Menu\Programs\DraftHorse.lnk"
    $script:FirewallRule = 'go-mapi OAuth loopback'
    $script:ExpectedAumid = 'com.marcfargas.gomapi'
    $script:CredTarget   = 'go-mapi:oauth-tokens'
    # QUICK-260423-ntu T3d — dual-bitness install surfaces. (The old
    # $script:MapiKey32 WOW6432Node path is gone with ARRICKS-10: Clients is
    # a shared key, so the 32-bit view is read via reg.exe /reg:32 instead.)
    $script:InstallDir32 = "${env:ProgramFiles(x86)}\go-mapi"

    # Phase 11.1 D-03 / D-18 case 4: %APPDATA% path is the negative-assertion target.
    # The %ProgramData% path is already $script:Shortcut (set by Phase 10).
    $script:AppDataLnk = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\DraftHorse.lnk'

    # Phase 11.1 Plan 11.1-05 — Scheduled Task assertions (D-08 / D-16 / D-18 cases 1, 2, 5, 6)
    $script:TaskName    = 'go-mapi Auto Update'
    $script:UpdatesDir  = Join-Path $env:ProgramData 'go-mapi\updates'

    Write-Host ("[Setup] SetupExe    = {0}" -f $script:SetupExe)
    Write-Host ("[Setup] InstallDir  = {0}" -f $script:InstallDir)
    Write-Host ("[Setup] ProgramData = {0}" -f $script:ProgramData)
    Write-Host ("[Setup] CredTarget  = {0}" -f $script:CredTarget)
}

Describe "go-mapi installer round-trip" {

    Context "Silent install" {
        # D-21 item 1
        It "1. silent install exits 0 with /S /D=<InstallDir>" {
            # NSIS /D= must be the LAST argument and NOT quoted (per RESEARCH Pitfall 5).
            # PowerShell's -ArgumentList array form preserves the token correctly.
            $proc = Start-Process -FilePath $script:SetupExe -ArgumentList '/S',"/D=$($script:InstallDir)" -Wait -PassThru
            $proc.ExitCode | Should -Be 0
        }

        # D-21 item 2
        It "2. go-mapi.exe and go-mapi.dll are deposited in InstallDir" {
            Test-Path (Join-Path $script:InstallDir 'go-mapi.exe') | Should -BeTrue
            Test-Path (Join-Path $script:InstallDir 'go-mapi.dll') | Should -BeTrue
        }

        # D-21 item 3 — tightened by ARRICKS-10. The old assertion matched only
        # the 'go-mapi.dll' suffix, which is exactly what let the shared-key
        # last-write-wins bug (x86 path served to ALL callers) go unnoticed.
        # Assert the full raw value, its REG_EXPAND_SZ kind, and that this
        # 64-bit pwsh's expansion lands on the deposited x64 DLL.
        It "3. HKLM MAPI handler key is registered with expandable DLLPath" {
            Test-Path $script:MapiKey | Should -BeTrue
            $key = Get-Item $script:MapiKey
            $key.GetValueKind('DLLPath') | Should -Be 'ExpandString'
            $key.GetValue('DLLPath', $null, 'DoNotExpandEnvironmentNames') |
                Should -Be '%ProgramFiles%\go-mapi\go-mapi.dll' -Because 'a 32-bit installer writing outside SetRegView 64 gets %ProgramFiles% rewritten to %ProgramFiles(x86)% by WOW64'
            $expanded = (Get-ItemProperty -Path $script:MapiKey).DLLPath
            $expanded | Should -Be (Join-Path $env:ProgramFiles 'go-mapi\go-mapi.dll')
            Test-Path $expanded | Should -BeTrue
            # ARRICKS-11: the client subkey's (Default) is its DISPLAY name;
            # the resolver (Default) on Clients\Mail must stay the subkey
            # name 'go-mapi' — both asserted so a future edit can't swap them.
            (Get-ItemProperty -Path $script:MapiKey -Name '(default)').'(default)' | Should -Be 'DraftHorse'
            (Get-ItemProperty -Path $script:MailKey -Name '(default)').'(default)' | Should -Be 'go-mapi'
        }

        # ARRICKS-09: mailto handler + Default Apps (Capabilities/RegisteredApplications)
        It "26. mailto ProgID and Default Apps registration exist" {
            $cmdKey = 'HKLM:\SOFTWARE\Classes\go-mapi.mailto\shell\open\command'
            Test-Path $cmdKey | Should -BeTrue
            $cmd = (Get-ItemProperty -Path $cmdKey -Name '(default)').'(default)'
            $cmd | Should -Match 'go-mapi\.exe'
            $cmd | Should -Match '--mailto'
            (Get-ItemProperty -Path "$script:MapiKey\Capabilities\URLAssociations" -Name 'mailto').'mailto' |
                Should -Be 'go-mapi.mailto'
            (Get-ItemProperty -Path 'HKLM:\SOFTWARE\RegisteredApplications' -Name 'go-mapi').'go-mapi' |
                Should -Be 'SOFTWARE\Clients\Mail\go-mapi\Capabilities'
        }

        # D-21 item 4
        It "4. previous-mail-client.json backup exists and parses with required fields" {
            Test-Path $script:BackupJson | Should -BeTrue
            $json = Get-Content $script:BackupJson -Raw | ConvertFrom-Json
            $json.PSObject.Properties.Name | Should -Contain 'previousClient'
            $json.PSObject.Properties.Name | Should -Contain 'backedUpAt'
            # backedUpAt should look like an ISO-8601 timestamp. ARRICKS fix:
            # assert against the raw JSON text — pwsh 7's ConvertFrom-Json
            # coerces ISO-8601 strings into [datetime], which -Match then
            # stringifies in culture format ('08/14/2026 ...'), failing the
            # ISO regex even though the file content is correct.
            Get-Content $script:BackupJson -Raw | Should -Match '"backedUpAt":"\d{4}-\d{2}-\d{2}T'
        }

        # D-21 item 5 — AUMID stamped on shortcut
        It "5. Start Menu shortcut exists with AUMID == com.marcfargas.gomapi" {
            Test-Path $script:Shortcut | Should -BeTrue
            $actual = Get-ShortcutAumid -Path $script:Shortcut
            $actual | Should -Be $script:ExpectedAumid
        }

        # D-21 item 6
        It "6. Windows Firewall inbound rule 'go-mapi OAuth loopback' exists" {
            $rule = Get-NetFirewallRule -DisplayName $script:FirewallRule -ErrorAction SilentlyContinue
            $rule | Should -Not -BeNullOrEmpty
            $rule.Direction | Should -Be 'Inbound'
            $rule.Action    | Should -Be 'Allow'
        }

        # QUICK-260423-ntu item 14 — install-time running-process guard (silent)
        It "14. silent install succeeds when go-mapi.exe is already running in InstallDir" {
            # Pre-condition: install completed in item 1. Launch a decoy process
            # from the installed path, then re-run the installer in /S mode and
            # assert the exe is still runnable post-install (i.e. the installer
            # closed the old instance cleanly, overwrote it, and did NOT abort).
            $exe = Join-Path $script:InstallDir 'go-mapi.exe'
            $decoy = Start-Process -FilePath $exe -PassThru -WindowStyle Hidden
            try {
                Start-Sleep -Seconds 1
                $proc = Start-Process -FilePath $script:SetupExe -ArgumentList '/S',"/D=$($script:InstallDir)" -Wait -PassThru
                $proc.ExitCode | Should -Be 0
                Test-Path $exe | Should -BeTrue
            } finally {
                # Belt-and-braces cleanup in case the installer did not close it
                if (-not $decoy.HasExited) { $decoy.Kill() }
            }
        }

        # QUICK-260423-ntu item 16 — x86 DLL deposited alongside x64 DLL
        It "16. go-mapi.dll is deposited in both ProgramFiles and ProgramFiles(x86)" {
            Test-Path (Join-Path $script:InstallDir   'go-mapi.dll') | Should -BeTrue
            Test-Path (Join-Path $script:InstallDir32 'go-mapi.dll') | Should -BeTrue
        }

        # QUICK-260423-ntu item 17 — each DLL has the matching PE bitness
        It "17. x64 DLL is PE32+ and x86 DLL is PE32" {
            function Get-PeMagic($p) {
                $b = [IO.File]::ReadAllBytes($p)
                $e = [BitConverter]::ToInt32($b, 0x3C)
                return [BitConverter]::ToUInt16($b, $e + 4 + 20)
            }
            Get-PeMagic (Join-Path $script:InstallDir   'go-mapi.dll') | Should -Be 0x20B
            Get-PeMagic (Join-Path $script:InstallDir32 'go-mapi.dll') | Should -Be 0x10B
        }

        # QUICK-260423-ntu item 18, reworked by ARRICKS-10. Clients is a WOW64
        # SHARED key (Win7+), so there is no separate 32-bit copy to assert
        # on; what matters is that the 32-bit VIEW (KEY_WOW64_32KEY — the view
        # the SysWOW64 mapi32.dll stub reads) sees the same unexpanded
        # %ProgramFiles% value. reg.exe /reg:32 opens exactly that view. The
        # actual x86 DLL routing is proven end-to-end by item 29.
        It "18. 32-bit registry view sees the same unexpanded DLLPath (shared key)" {
            $out = (& reg.exe query 'HKLM\SOFTWARE\Clients\Mail\go-mapi' /v DLLPath /reg:32) -join "`n"
            $LASTEXITCODE | Should -Be 0
            $out | Should -Match 'REG_EXPAND_SZ\s+%ProgramFiles%\\go-mapi\\go-mapi\.dll'
        }

        # Phase 11.1 D-05 / D-18 case 3 — silent reinstall overwrites both DLLs (T4 regression)
        It "21. silent reinstall over existing install overwrites both x64 and x86 DLLs" {
            # Pre-condition: prior items already installed once into $script:InstallDir.
            # Capture both DLLs' hashes before reinstall to detect "no overwrite happened".
            $x64Path = Join-Path $script:InstallDir   'go-mapi.dll'
            $x86Path = Join-Path $script:InstallDir32 'go-mapi.dll'
            $x64Before = (Get-FileHash -Algorithm SHA256 -Path $x64Path).Hash
            $x86Before = (Get-FileHash -Algorithm SHA256 -Path $x86Path).Hash

            # Touch both files to a known sentinel mtime so a silent skip leaves them stale.
            $sentinel = (Get-Date).AddDays(-1)
            (Get-Item $x64Path).LastWriteTime = $sentinel
            (Get-Item $x86Path).LastWriteTime = $sentinel

            # Reinstall silently WITHOUT prior uninstall — this is the T4 repro case.
            $proc = Start-Process -FilePath $script:SetupExe -ArgumentList '/S',"/D=$($script:InstallDir)" -Wait -PassThru
            $proc.ExitCode | Should -Be 0

            # Both DLLs MUST have moved off the sentinel (overwrite happened).
            # NOT "fresh vs now": NSIS SetDateSave (default on) restores each
            # file's stored BUILD-time mtime on extraction, so a freshness
            # window fails whenever build-to-assert exceeds it (first seen at
            # 4.4 min on a cache-cold runner — ARRICKS-12). A T4 silent skip
            # leaves exactly the sentinel; an overwrite replaces it with the
            # stored build time, which is always far newer than yesterday.
            (Get-Item $x64Path).LastWriteTime | Should -BeGreaterThan $sentinel.AddHours(1)
            (Get-Item $x86Path).LastWriteTime | Should -BeGreaterThan $sentinel.AddHours(1)

            # Hashes should match the prior install (same binaries shipped — confirms the
            # overwrite happened with a real File write rather than NSIS skipping).
            (Get-FileHash -Algorithm SHA256 -Path $x64Path).Hash | Should -Be $x64Before
            (Get-FileHash -Algorithm SHA256 -Path $x86Path).Hash | Should -Be $x86Before

            # Registry DLLPath after reinstall — strict again (ARRICKS-10).
            # The first real run of this suite proved HKLM\SOFTWARE\Clients is
            # a WOW64 SHARED key (the old per-view assertion was
            # unimplementable: both installer writes hit one physical key and
            # the x86 path won for all callers). The redesign registers a
            # single REG_EXPAND_SZ value; reinstall must leave its raw text,
            # kind, and both views' reads intact.
            $key = Get-Item $script:MapiKey
            $key.GetValueKind('DLLPath') | Should -Be 'ExpandString'
            $key.GetValue('DLLPath', $null, 'DoNotExpandEnvironmentNames') |
                Should -Be '%ProgramFiles%\go-mapi\go-mapi.dll'
            $expanded = (Get-ItemProperty -Path $script:MapiKey).DLLPath
            $expanded | Should -Be (Join-Path $env:ProgramFiles 'go-mapi\go-mapi.dll')
            Test-Path $expanded | Should -BeTrue -Because "DLLPath must point at a deposited DLL"
            $out32 = (& reg.exe query 'HKLM\SOFTWARE\Clients\Mail\go-mapi' /v DLLPath /reg:32) -join "`n"
            $LASTEXITCODE | Should -Be 0
            $out32 | Should -Match 'REG_EXPAND_SZ\s+%ProgramFiles%\\go-mapi\\go-mapi\.dll'
        }

        # Phase 11.1 D-03 / D-18 case 4 — Start Menu shortcut location regression
        It "25. Start Menu shortcut lands at %ProgramData%\Microsoft\Windows\Start Menu\Programs (D-03 regression)" {
            # The reinstall above ensures the shortcut is in place — no extra setup needed.
            Test-Path $script:Shortcut    | Should -BeTrue  -Because "D-03: shortcut MUST be all-users (%ProgramData%)"
            Test-Path $script:AppDataLnk  | Should -BeFalse -Because "D-03: per-user shortcut MUST NOT be created (%APPDATA%)"
        }

        # ARRICKS-10 items 28/29 — end-to-end stub resolution, per bitness.
        # mapiprobe{64,32}.exe (built by installer-smoke.yml from
        # tests/probe/mapiprobe.c) call MAPISendMail through the in-box
        # mapi32.dll stub exactly like 64-bit Explorer "Send to" and 32-bit
        # scanner software do, then report which go-mapi.dll actually loaded
        # in-process. This is the on-real-Windows proof that the stub reads
        # Clients\Mail\<client>\DLLPath and expands REG_EXPAND_SZ with the
        # CALLER's environment — the fact the whole ARRICKS-10 design rests on.
        It "28. 64-bit Simple MAPI caller loads the x64 DLL through the stub" {
            $probe = Join-Path $PSScriptRoot '..\..\..\mapiprobe64.exe' | Resolve-Path -ErrorAction Stop | ForEach-Object Path
            $out = & $probe 2>&1 | Out-String
            Write-Host $out
            $LASTEXITCODE | Should -Be 0 -Because "the stub must resolve and load go-mapi.dll for a 64-bit caller"
            $out | Should -Match ([regex]::Escape("PROGRAMFILES=$env:ProgramFiles") + '\s')
            $out | Should -Match ([regex]::Escape("RESOLVED=$env:ProgramFiles\go-mapi\go-mapi.dll") + '\s')
            $out | Should -Match 'MAPIRC=0\s'
        }

        It "29. 32-bit Simple MAPI caller loads the x86 DLL through the stub" {
            $probe = Join-Path $PSScriptRoot '..\..\..\mapiprobe32.exe' | Resolve-Path -ErrorAction Stop | ForEach-Object Path
            $out = & $probe 2>&1 | Out-String
            Write-Host $out
            $LASTEXITCODE | Should -Be 0 -Because "the stub must resolve and load go-mapi.dll for a 32-bit caller"
            $out | Should -Match ([regex]::Escape("PROGRAMFILES=${env:ProgramFiles(x86)}") + '\s')
            $out | Should -Match ([regex]::Escape("RESOLVED=${env:ProgramFiles(x86)}\go-mapi\go-mapi.dll") + '\s')
            $out | Should -Match 'MAPIRC=0\s'
        }

        # ARRICKS-06 — /AUTOUPDATE=1 must now be INERT.
        #
        # Replaces the upstream test that asserted the flag registers a
        # SYSTEM Scheduled Task. RegisterScheduledTask and the /AUTOUPDATE=
        # parser have been removed, so a deployment script still carrying the
        # old flag must not be able to re-arm the updater.
        It "22. /AUTOUPDATE=1 install does NOT register a Scheduled Task (flag is inert)" {
            $uninst = Join-Path $script:InstallDir 'uninstall.exe'
            if (Test-Path $uninst) {
                Start-Process -FilePath $uninst -ArgumentList '/S' -Wait | Out-Null
                Start-Sleep -Seconds 2
            }
            $proc = Start-Process -FilePath $script:SetupExe -ArgumentList '/S','/AUTOUPDATE=1',"/D=$($script:InstallDir)" -Wait -PassThru
            $proc.ExitCode | Should -Be 0
            Start-Sleep -Seconds 1   # Pitfall 5: let Task Scheduler cache settle.

            Get-ScheduledTask -TaskName $script:TaskName -ErrorAction SilentlyContinue |
                Should -BeNullOrEmpty -Because "ARRICKS-06: the installer must never create the auto-update task, even when the legacy flag is passed"
        }

        # ARRICKS-06 — the binary must ignore the silent-update flag entirely.
        It "22b. go-mapi.exe --update-check-silent is a no-op" {
            $exe = Join-Path $script:InstallDir 'go-mapi.exe'
            $proc = Start-Process -FilePath $exe -ArgumentList '--update-check-silent' -Wait -PassThru
            $proc.ExitCode | Should -Be 0
            Test-Path (Join-Path $env:ProgramData 'go-mapi\updates\update.log') |
                Should -BeFalse -Because "ARRICKS-06: no silent update routine should run, so no update log should be produced"
        }

        # Phase 11.1 D-07 / D-18 case 2 — /AUTOUPDATE absent: no Scheduled Task
        It "23. /AUTOUPDATE=0 install does NOT register the Scheduled Task" {
            $uninst = Join-Path $script:InstallDir 'uninstall.exe'
            if (Test-Path $uninst) {
                Start-Process -FilePath $uninst -ArgumentList '/S' -Wait | Out-Null
                Start-Sleep -Seconds 2
            }
            Start-Process -FilePath $script:SetupExe -ArgumentList '/S',"/D=$($script:InstallDir)" -Wait | Out-Null
            Start-Sleep -Seconds 1
            Get-ScheduledTask -TaskName $script:TaskName -ErrorAction SilentlyContinue | Should -BeNullOrEmpty
        }

        # Phase 11.1 D-16 / D-18 case 5 — uninstaller idempotently removes the task
        # AND scrubs %ProgramData%\go-mapi\updates (D-18 case 6)
        It "24. uninstall removes the Scheduled Task even when /AUTOUPDATE=0 was used" {
            # /AUTOUPDATE=0 install — uninstall must still run schtasks /delete /f and exit 0.
            Start-Process -FilePath $script:SetupExe -ArgumentList '/S',"/D=$($script:InstallDir)" -Wait | Out-Null
            $uninst = Join-Path $script:InstallDir 'uninstall.exe'
            $proc = Start-Process -FilePath $uninst -ArgumentList '/S' -Wait -PassThru
            $proc.ExitCode | Should -Be 0   # D-16: idempotent removal — 'task not found' is OK.
            Start-Sleep -Seconds 2

            # D-16 belt: even though /AUTOUPDATE=0 means no task was registered,
            # the uninstaller's schtasks /delete /f ran (rc=1 swallowed). Confirm
            # nothing is left behind in Task Scheduler post-uninstall.
            Get-ScheduledTask -TaskName $script:TaskName -ErrorAction SilentlyContinue | Should -BeNullOrEmpty

            # Uninstaller also scrubs %ProgramData%\go-mapi\updates (D-18 case 6).
            Test-Path $script:UpdatesDir | Should -BeFalse -Because "uninstaller scrubs %ProgramData%\go-mapi\updates per D-18 case 6"
        }

        # Phase 11.1 W7 — uninstaller scrubs *.old.<pid> orphan files left by silent updater
        It "24b. uninstaller scrubs *.old.<pid> orphan files left by silent updater (W7 regression)" {
            # Reinstall fresh so $script:InstallDir exists with the binary.
            $uninst = Join-Path $script:InstallDir 'uninstall.exe'
            if (Test-Path $uninst) {
                Start-Process -FilePath $uninst -ArgumentList '/S' -Wait | Out-Null
                Start-Sleep -Seconds 2
            }
            Start-Process -FilePath $script:SetupExe -ArgumentList '/S',"/D=$($script:InstallDir)" -Wait | Out-Null

            # Plant orphan files mimicking what swapInPlace would leave behind.
            $orphan64  = Join-Path $script:InstallDir   'go-mapi.exe.old.123'
            $orphanDll = Join-Path $script:InstallDir   'go-mapi.dll.old.456'
            $orphan32  = Join-Path $script:InstallDir32 'go-mapi.dll.old.789'
            New-Item -ItemType File -Path $orphan64  -Force | Out-Null
            New-Item -ItemType File -Path $orphanDll -Force | Out-Null
            New-Item -ItemType File -Path $orphan32  -Force | Out-Null

            Test-Path $orphan64  | Should -BeTrue  # sanity
            Test-Path $orphanDll | Should -BeTrue
            Test-Path $orphan32  | Should -BeTrue

            # Uninstall — orphans MUST be gone.
            $uninstAfter = Join-Path $script:InstallDir 'uninstall.exe'
            $proc = Start-Process -FilePath $uninstAfter -ArgumentList '/S' -Wait -PassThru
            $proc.ExitCode | Should -Be 0
            Start-Sleep -Seconds 2

            Test-Path $orphan64  | Should -BeFalse -Because "uninstaller MUST scrub *.old.<pid> orphans in `$INSTDIR (W7)"
            Test-Path $orphanDll | Should -BeFalse -Because "uninstaller MUST scrub *.old.<pid> orphans in `$INSTDIR (W7)"
            Test-Path $orphan32  | Should -BeFalse -Because "uninstaller MUST scrub *.old.<pid> orphans in `$PROGRAMFILES32\go-mapi (W7)"
        }
    }

    Context "Silent uninstall" {
        BeforeAll {
            # ARRICKS fix: test 24b ends by uninstalling and never reinstalls,
            # so this context previously started against an uninstalled
            # machine and test 7 found no uninstaller. Reinstall here iff
            # needed so the uninstall assertions exercise a real install,
            # regardless of what order the install-phase tests ran in.
            $uninst = Join-Path $script:InstallDir 'uninstall.exe'
            if (-not (Test-Path $uninst)) {
                Start-Process -FilePath $script:SetupExe -ArgumentList '/S',"/D=$($script:InstallDir)" -Wait | Out-Null
            }
        }

        # D-21 item 7
        It "7. silent uninstall exits 0 with /S" {
            $uninst = Join-Path $script:InstallDir 'uninstall.exe'
            Test-Path $uninst | Should -BeTrue -Because "uninstaller must be in place after install"
            $proc = Start-Process -FilePath $uninst -ArgumentList '/S' -Wait -PassThru
            $proc.ExitCode | Should -Be 0
            # NSIS uninstaller self-deletes via a batch wrapper; sleep briefly so the
            # batch can complete before subsequent Test-Path probes.
            Start-Sleep -Seconds 2
        }

        # D-21 item 8
        It "8. install dir is gone (or empty)" {
            $exists = Test-Path $script:InstallDir
            if ($exists) {
                # Acceptable if empty — NSIS RMDir (non-recursive) leaves dir when files remain
                (Get-ChildItem $script:InstallDir -Force -ErrorAction SilentlyContinue).Count | Should -Be 0
            }
        }

        # D-21 item 9
        It "9. MAPI handler key HKLM\SOFTWARE\Clients\Mail\go-mapi is gone" {
            Test-Path $script:MapiKey | Should -BeFalse
        }

        # ARRICKS-09: mailto registration removed with the app
        It "27. mailto ProgID and RegisteredApplications entry are gone" {
            Test-Path 'HKLM:\SOFTWARE\Classes\go-mapi.mailto' | Should -BeFalse
            (Get-ItemProperty -Path 'HKLM:\SOFTWARE\RegisteredApplications' -ErrorAction SilentlyContinue).'go-mapi' |
                Should -BeNullOrEmpty
        }

        # D-21 item 10
        It "10. firewall rule 'go-mapi OAuth loopback' is gone" {
            Get-NetFirewallRule -DisplayName $script:FirewallRule -ErrorAction SilentlyContinue | Should -BeNullOrEmpty
        }

        # D-21 item 11
        It "11. %APPDATA%\go-mapi\ is gone for the runner user" {
            Test-Path "$env:APPDATA\go-mapi" | Should -BeFalse
        }

        # D-21 item 12 — Credential Manager scrub (colon target per PATTERNS.md Shared Pattern 3)
        It "12. cmdkey /list:go-mapi:oauth-tokens returns no matching entries" {
            # cmdkey prints to stdout + may use stderr depending on locale; merge streams.
            $out = & cmdkey /list:$script:CredTarget 2>&1 | Out-String
            # cmdkey output contains 'Target:' lines when an entry matches, or a
            # "NONE" / locale-dependent "no credentials" message when nothing matches.
            # Safe assertion: no line containing the literal target string.
            # ARRICKS fix: cmdkey /list:<target> echoes the target name in its
            # header even when reporting "* NONE *", so matching the bare
            # target string always false-positived. A real stored credential
            # is listed as "Target: ...target=<name>"; match that instead.
            $out | Should -Not -Match "target=$([regex]::Escape($script:CredTarget))" -Because "cmdkey should find no credentials under target '$($script:CredTarget)' after uninstall"
        }

        # D-21 item 13
        It "13. Start Menu shortcut is gone" {
            Test-Path $script:Shortcut | Should -BeFalse
        }

        # QUICK-260423-ntu item 15 — uninstall-time running-process guard (silent)
        It "15. silent uninstall closes a running go-mapi.exe in InstallDir and removes the binary" {
            # Re-install first because item 7 already uninstalled.
            Start-Process -FilePath $script:SetupExe -ArgumentList '/S',"/D=$($script:InstallDir)" -Wait
            $exe = Join-Path $script:InstallDir 'go-mapi.exe'
            $decoy = Start-Process -FilePath $exe -PassThru -WindowStyle Hidden
            try {
                Start-Sleep -Seconds 1
                $uninst = Join-Path $script:InstallDir 'uninstall.exe'
                $proc = Start-Process -FilePath $uninst -ArgumentList '/S' -Wait -PassThru
                $proc.ExitCode | Should -Be 0
                Start-Sleep -Seconds 2   # NSIS batch-wrapper self-delete
                Test-Path $exe | Should -BeFalse -Because "uninstaller should have closed the running instance and deleted the binary"
                $decoy.HasExited | Should -BeTrue
            } finally {
                if (-not $decoy.HasExited) { $decoy.Kill() }
            }
        }

        # QUICK-260423-ntu item 19 — x86 DLL + install dir removed by uninstall
        It "19. ProgramFiles(x86)\go-mapi is gone after uninstall" {
            $exists = Test-Path $script:InstallDir32
            if ($exists) {
                (Get-ChildItem $script:InstallDir32 -Force -ErrorAction SilentlyContinue).Count | Should -Be 0
            }
            Test-Path (Join-Path $script:InstallDir32 'go-mapi.dll') | Should -BeFalse
        }

        # QUICK-260423-ntu item 20, reworked by ARRICKS-10: Clients is a WOW64
        # shared key, so assert the 32-bit VIEW no longer resolves the client
        # key after uninstall (reg.exe /reg:32 = KEY_WOW64_32KEY).
        It "20. 32-bit registry view MAPI handler key is gone after uninstall" {
            & reg.exe query 'HKLM\SOFTWARE\Clients\Mail\go-mapi' /reg:32 2>$null | Out-Null
            $LASTEXITCODE | Should -Not -Be 0
        }
    }
}
