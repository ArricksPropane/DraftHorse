; go-mapi.nsi — NSIS installer for DraftHorse (v3.0)
;
; Plan 10-01 scaffold: ModernUI2 layout, admin-elevation, machine-wide install,
; MAPI handler registration, previous-mail-client backup, Add/Remove Programs
; metadata, and stub Call sites for plans 10-02 / 10-03 / 10-04.
;
; Compile with:
;     makensis /DGOMAPI_VERSION=0.0.0-dev src\installer\go-mapi.nsi
;
; Requires: pre-built src\app\build\bin\DraftHorse.exe and
;           src\interceptor\build\bin\DraftHorse.dll (staged by CI in plan 10-05 /
;           release pipeline in plan 10-06).
;
; References:
;   D-01, D-02, D-03, D-04 — NSIS + admin + machine-wide + output filename
;   D-09 — HKLM\SOFTWARE\Clients\Mail\DraftHorse registration layout
;   D-10 — BackupPreviousMailClient BEFORE overwriting HKLM Mail (Default)
;   D-12 — %ProgramData%\DraftHorse\uninst\ directory for backup JSON
;   T-10-01-01 — ordering invariant enforced below (Call BackupPreviousMailClient
;                precedes WriteRegStr HKLM "SOFTWARE\Clients\Mail" "" "DraftHorse")
;   QUICK-260423-msq — DLL queue relocated from %TEMP%\DraftHorse\ to
;                %LOCALAPPDATA%\DraftHorse\queue\ (DLL creates it at DllMain; installer does not
;                pre-create it — no install-time action required for the path itself).

Unicode True

;------------------------------------------------------------------------------
; Product defines (consumed by plans 10-02 / 10-03 / 10-04)
;------------------------------------------------------------------------------

!ifndef GOMAPI_VERSION
  !define GOMAPI_VERSION "0.0.0-dev"
!endif

!define PRODUCT_NAME      "DraftHorse"
; V4 (2026-08): identifier and display are now BOTH DraftHorse. The
; ARRICKS-11 split (display DraftHorse, identifiers go-mapi) ended with 4.0,
; while the installed base was 4 test machines — see docs/V4-PLAN.md Phase 1
; and the V4 MIGRATION block in the Install section, which scrubs the
; old-name artifacts a 3.x install left behind. PRODUCT_NAME still feeds the
; Clients\Mail subkey + resolver, ProgID, uninstall key, firewall rule and
; Intune detection — keep it in lockstep with the Go-side constants
; (defaultmail.go, auth.go, paths.go).
!define PRODUCT_DISPLAY   "DraftHorse"
!define PRODUCT_VERSION   "${GOMAPI_VERSION}"
!define PRODUCT_PUBLISHER "Arrick's Propane"
!define PRODUCT_WEB_SITE  "https://github.com/egkrateia247/DraftHorse"
!define AUMID             "com.arrickspropane.drafthorse"

;------------------------------------------------------------------------------
; Compiler / installer-wide settings
;------------------------------------------------------------------------------

SetCompressor /SOLID lzma
RequestExecutionLevel admin
InstallDir   "$PROGRAMFILES64\DraftHorse"
OutFile      "DraftHorse-setup.exe"
Name         "${PRODUCT_DISPLAY} ${PRODUCT_VERSION}"
BrandingText "${PRODUCT_DISPLAY} ${PRODUCT_VERSION} — LGPL-3.0"

; Repo-local plugin directory for vendored NSIS plugins (ApplicationID.dll).
; `${__FILEDIR__}` resolves to src\installer\ at makensis time.
; IMPORTANT: !addplugindir for user-added directories searches the directory
; root directly — NOT an x86-unicode/ subdirectory (that convention applies
; only to the NSIS built-in Plugins/ tree). Point directly at x86-unicode/.
!addplugindir "${__FILEDIR__}\plugins\x86-unicode"

;------------------------------------------------------------------------------
; ModernUI2 pages
;------------------------------------------------------------------------------

!include "MUI2.nsh"
; Phase 11.1 Plan 05 — D-07 / D-08 silent auto-update wiring.
;   FileFunc.nsh : ${GetParameters} + ${GetOptions} for /AUTOUPDATE=N parsing.
;   nsDialogs.nsh: nsDialogs::Create + ${NSD_*} for the AutoUpdate page checkbox.
;   LogicLib.nsh : ${If}/${Else}/${EndIf} + ${DoUntil}/${LoopUntil}/${Errors}
;                  used by AutoUpdatePageLeave and un.ScrubOldOrphans.
!include "FileFunc.nsh"
!include "nsDialogs.nsh"
!include "LogicLib.nsh"

; ARRICKS-06: $AutoUpdateFlag / $AutoUpdateCheckboxState removed along with the
; auto-update opt-in page and RegisterScheduledTask. This installer never
; creates the "DraftHorse Auto Update" Scheduled Task. See main.go for why.

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${__FILEDIR__}\..\..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
; ARRICKS-06: the AutoUpdate opt-in page has been removed.
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; ARRICKS-06: .onInit (the /AUTOUPDATE= parser), AutoUpdatePage and
; AutoUpdatePageLeave removed. /AUTOUPDATE=1 is now silently ignored rather
; than honoured, so a deployment script carrying the old flag cannot re-arm
; the updater.

;------------------------------------------------------------------------------
; Install section
;------------------------------------------------------------------------------

Section "Install" SecInstall
  ; QUICK-260423-ntu T2 — if a previous install's DraftHorse.exe is running in
  ; $INSTDIR, give it a chance to close cleanly (WM_CLOSE via taskkill
  ; without /F triggers the intentionalQuit path in src/app/main.go) before
  ; we overwrite the binary. Silent mode auto-retries; interactive mode
  ; prompts the user. MUST be the first statement in the section.
  Call EnsureAppNotRunning

  ;----------------------------------------------------------------------------
  ; V4 MIGRATION — scrub the machine-scope artifacts of a pre-4.0 install.
  ;
  ; 4.0 renamed every installed identifier from go-mapi to DraftHorse (see
  ; docs/V4-PLAN.md Phase 1). This block removes what a 3.x install left
  ; under the old names; the app migrates PER-USER state (data dirs,
  ; credential target, HKCU heal mirror) at first run, because this
  ; installer runs elevated and cannot see each user's profile.
  ;
  ; Every step is idempotent and tolerant of "not found" — a fresh machine
  ; runs straight through. DELETE THIS BLOCK only when no 3.x install can
  ; exist anywhere (post-rollout fleet reimage, realistically never).

  ; Old app may be autostarted and running under the old name — close it the
  ; same way EnsureAppNotRunning closes the new one (WM_CLOSE, no /F).
  ExecWait 'taskkill /im "go-mapi.exe"' $0
  Sleep 2000
  ExecWait 'taskkill /f /im "go-mapi.exe"' $0

  ; Old MAPI client key (WOW64-shared — one delete covers both views).
  DeleteRegKey HKLM "SOFTWARE\Clients\Mail\go-mapi"

  ; Old autostart (written under SetRegView 64, matching ARRICKS-19).
  SetRegView 64
  DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "go-mapi"
  SetRegView default

  ; Old mailto ProgID + Default Apps listing.
  DeleteRegKey HKLM "SOFTWARE\Classes\go-mapi.mailto"
  DeleteRegValue HKLM "SOFTWARE\RegisteredApplications" "go-mapi"

  ; Windows' browsed-app auto-registration for the old exe ("look for
  ; another app on this PC" creates Classes\Applications\<exe>). Left in
  ; place it keeps a dead "go-mapi" row in every Default Apps picker
  ; (found on the first 4.0 migration retest). Machine hive here; each
  ; user's HKCU twin is scrubbed by the app (migrate.go).
  DeleteRegKey HKLM "SOFTWARE\Classes\Applications\go-mapi.exe"

  ; Old ARP entry (the new write below uses Uninstall\DraftHorse — without
  ; this, Add/Remove Programs shows two rows and the old uninstaller would
  ; scrub the NEW Clients\Mail resolver if ever run). Uninstall is a WOW64
  ; REDIRECTED key (unlike Clients\Mail / Classes / RegisteredApplications,
  ; which are shared) — smoke test 36 proved a single delete from this
  ; 32-bit installer only reaches the WOW6432Node view, so delete BOTH.
  SetRegView 64
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\go-mapi"
  SetRegView 32
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\go-mapi"
  SetRegView default

  ; Old firewall rule (new rule name is added later this section).
  ExecWait 'netsh advfirewall firewall delete rule name="go-mapi OAuth loopback"' $0

  ; Old install trees, both bitnesses. Safe after the taskkill above; queue
  ; and settings never lived here (they are per-user under %LOCALAPPDATA%).
  RMDir /r "$PROGRAMFILES64\go-mapi"
  RMDir /r "$PROGRAMFILES32\go-mapi"

  ; Old Start Menu shortcut.
  SetShellVarContext all
  Delete "$SMPROGRAMS\go-mapi.lnk"
  SetShellVarContext current
  ;---------------------------------------------------------------- end V4 ----

  SetOutPath "$INSTDIR"

  ; Staged binary paths — produced by:
  ;   npm run build:interceptor         (clang + CMake → build-x64/ + build-x86/)
  ;   wails build -platform windows/amd64 (→ DraftHorse.exe with go:embed frontend)
  ;
  ; QUICK-260423-ntu T3c — dual-bitness layout: x64 DLL lands in $INSTDIR
  ; (= $PROGRAMFILES64\DraftHorse) for native MAPI callers; x86 DLL lands in
  ; $PROGRAMFILES32\DraftHorse for legacy 32-bit MAPI callers. The single
  ; REG_EXPAND_SZ DLLPath write below (ARRICKS-10) routes each caller to the
  ; matching-bitness DLL via its own %ProgramFiles% expansion.
  ; PHASE 11.1 T4 (D-04): explicit Delete + SetOverwrite try collapses transient
  ; AV/filter holds into a no-op rather than aborting the installer. RESEARCH
  ; §Pattern 1 + §Pitfall 1. NSIS default SetOverwrite is `on`, which makes
  ; reinstall fail hard on any transient lock; `try` skips silently if write
  ; fails (the explicit Delete clears the prior version first so it does not).
  ClearErrors
  Delete "$INSTDIR\DraftHorse.exe"
  Delete "$INSTDIR\DraftHorse.dll"
  Delete "$INSTDIR\DraftHorse.ico"
  SetOverwrite try
  File "${__FILEDIR__}\..\app\build\bin\DraftHorse.exe"
  File "${__FILEDIR__}\..\interceptor\build-x64\bin\DraftHorse.dll"
  ; ARRICKS-16 — toast icon. toast_windows.go (toastIconPath) has always
  ; looked for $INSTDIR\DraftHorse.ico next to the exe, but nothing ever
  ; shipped it — toasts rendered without an app image. Same multi-res icon
  ; Wails embeds into the exe (src/app/build/windows/icon.ico), deposited
  ; under the name the Go code expects (toastIconPath: DraftHorse.ico).
  File "/oname=DraftHorse.ico" "${__FILEDIR__}\..\app\build\windows\icon.ico"
  SetOverwrite on

  ; x86 DLL — same T4 treatment in $PROGRAMFILES32 view.
  CreateDirectory "$PROGRAMFILES32\DraftHorse"
  SetOutPath "$PROGRAMFILES32\DraftHorse"
  ClearErrors
  Delete "$PROGRAMFILES32\DraftHorse\DraftHorse.dll"
  SetOverwrite try
  File "${__FILEDIR__}\..\interceptor\build-x86\bin\DraftHorse.dll"
  SetOverwrite on

  ; ARRICKS-10 — the REG_EXPAND_SZ DLLPath (see D-09 below) resolves 64-bit
  ; callers to %ProgramFiles%\DraftHorse\DraftHorse.dll unconditionally, so a
  ; custom /D= install dir must still deposit the x64 DLL at that fixed
  ; path (mirroring how the x86 DLL is always at $PROGRAMFILES32\DraftHorse).
  ; NSIS StrCmp is case-insensitive, matching NTFS path semantics. Skipped
  ; entirely for the default dir, where $INSTDIR already IS that path.
  StrCmp "$INSTDIR" "$PROGRAMFILES64\DraftHorse" MapiDllPinned
  CreateDirectory "$PROGRAMFILES64\DraftHorse"
  SetOutPath "$PROGRAMFILES64\DraftHorse"
  ClearErrors
  Delete "$PROGRAMFILES64\DraftHorse\DraftHorse.dll"
  SetOverwrite try
  File "${__FILEDIR__}\..\interceptor\build-x64\bin\DraftHorse.dll"
  SetOverwrite on
  DetailPrint "Non-default InstallDir: x64 MAPI DLL also pinned to $PROGRAMFILES64\DraftHorse"
MapiDllPinned:

  ; Reset $OUTDIR for the rest of the install section
  SetOutPath "$INSTDIR"

  ; QUICK-260423-msq — diagnostic scripts shipped alongside the app for the
  ; future in-app "Report bug" flow. PS 5.1-compatible, non-admin, read-only.
  ; Installed to $INSTDIR\diagnostics\ so the Wails app can invoke them at a
  ; known relative path when the user triggers a bug report.
  SetOutPath "$INSTDIR\diagnostics"
  File "${__FILEDIR__}\..\..\scripts\diagnostics\collect-registration.ps1"
  File "${__FILEDIR__}\..\..\scripts\diagnostics\collect-runtime.ps1"
  SetOutPath "$INSTDIR"

  ; D-10 + T-10-01-01 — MUST run BEFORE the HKLM Mail (Default) overwrite below
  ; so the pre-install mail client name is captured correctly.
  ; (ARRICKS fix: the function now resolves %PROGRAMDATA% via ReadEnvStr
  ; into $9 — the old $APPDATA-relative walk landed in
  ; <userprofile>\ProgramData under the default `current` shell context.)
  Call BackupPreviousMailClient

  ; D-09 — MAPI handler registration (machine-wide).
  ; Subkey + DLLPath are set first; the HKLM\SOFTWARE\Clients\Mail\(Default)
  ; overwrite happens AFTER the backup call above.
  ;
  ; ARRICKS-10 — dual-bitness registration via a single REG_EXPAND_SZ value.
  ; HKLM\SOFTWARE\Clients is on the WOW64 SHARED-key list since Windows 7
  ; (learn.microsoft.com "Registry Keys Affected by WOW64"), so the previous
  ; native + SetRegView 32 write pair landed in ONE physical key and the last
  ; write (x86 path) won for ALL callers — 64-bit MAPI callers then failed to
  ; load the x86 DLL (proved by installer-smoke test 21's first real run; the
  ; per-view pattern dates from XP/Vista where Clients really was redirected).
  ; The mapi32.dll stub expands a REG_EXPAND_SZ DLLPath with the CALLER's own
  ; environment (RegQueryWszExpand -> ExpandEnvironmentStringsW in Microsoft's
  ; published MAPIStubLibrary), and WOW64 gives 32-bit processes
  ; %ProgramFiles% = "Program Files (x86)", so one value routes each caller
  ; to the matching-bitness DLL.
  ;
  ; SetRegView 64 is LOAD-BEARING, not stylistic: this installer is a 32-bit
  ; process, and WOW64 rewrites a 32-bit writer's leading "%ProgramFiles%"
  ; into "%ProgramFiles(x86)%" unless the key is opened with KEY_WOW64_64KEY
  ; (learn.microsoft.com "Registry Redirector"). Without it the stored value
  ; silently becomes the x86 path for everyone again.
  ; ARRICKS-11: the client SUBKEY's (Default) is its human-facing display
  ; name — rebranded. The Clients\Mail (Default) below is the RESOLVER: the
  ; mapi32 stub opens the subkey named by that exact string, so it MUST stay
  ; "DraftHorse" (the subkey name), never the display name.
  SetRegView 64
  WriteRegStr HKLM "SOFTWARE\Clients\Mail\DraftHorse" "" "${PRODUCT_DISPLAY}"
  WriteRegExpandStr HKLM "SOFTWARE\Clients\Mail\DraftHorse" "DLLPath" "%ProgramFiles%\DraftHorse\DraftHorse.dll"
  WriteRegStr HKLM "SOFTWARE\Clients\Mail" "" "DraftHorse"
  SetRegView default

  ; ARRICKS-09 — mailto: protocol handler + Default Apps registration.
  ; Three pieces, all machine-wide, native (64-bit) view only — Default Apps
  ; and UserChoice resolve ProgIDs from the native Classes view:
  ;
  ;  1. The DraftHorse.mailto ProgID: shell open command Windows invokes when
  ;     DraftHorse is the chosen mailto handler. --mailto opens Gmail web
  ;     compose prefilled from the URL (see src/app/mailto.go) and exits.
  ;  2. Capabilities under the existing Clients\Mail\DraftHorse key (canonical
  ;     location for mail clients), advertising the mailto association.
  ;  3. The RegisteredApplications pointer that makes DraftHorse appear in
  ;     Settings > Default apps.
  ;
  ; Deliberately NOT written: HKCU\...\UserChoice. Windows 11 hash-protects
  ; it; the user picks DraftHorse once in Settings > Default apps, or Intune
  ; sets it fleet-wide (docs\mailto-default-associations.xml).
  WriteRegStr HKLM "SOFTWARE\Classes\DraftHorse.mailto" "" "${PRODUCT_DISPLAY} mailto handler"
  WriteRegStr HKLM "SOFTWARE\Classes\DraftHorse.mailto\DefaultIcon" "" "$INSTDIR\DraftHorse.exe,0"
  WriteRegStr HKLM "SOFTWARE\Classes\DraftHorse.mailto\shell\open\command" "" '"$INSTDIR\DraftHorse.exe" --mailto "%1"'
  WriteRegStr HKLM "SOFTWARE\Clients\Mail\DraftHorse\Capabilities" "ApplicationName" "${PRODUCT_DISPLAY}"
  WriteRegStr HKLM "SOFTWARE\Clients\Mail\DraftHorse\Capabilities" "ApplicationDescription" "Creates Gmail drafts from Simple MAPI calls and opens Gmail compose for mailto: links"
  WriteRegStr HKLM "SOFTWARE\Clients\Mail\DraftHorse\Capabilities\URLAssociations" "mailto" "DraftHorse.mailto"
  WriteRegStr HKLM "SOFTWARE\RegisteredApplications" "DraftHorse" "SOFTWARE\Clients\Mail\DraftHorse\Capabilities"

  ; Uninstaller binary
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Add/Remove Programs metadata
  ; V4: the key path follows PRODUCT_NAME (now DraftHorse). Pre-4.0 installs
  ; used Uninstall\go-mapi — deleted by the V4 MIGRATION block above, so ARP
  ; shows exactly one row across the upgrade.
  WriteRegStr   HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "DisplayName"     "${PRODUCT_DISPLAY}"
  WriteRegStr   HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "DisplayVersion"  "${PRODUCT_VERSION}"
  WriteRegStr   HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "Publisher"       "${PRODUCT_PUBLISHER}"
  WriteRegStr   HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "URLInfoAbout"    "${PRODUCT_WEB_SITE}"
  WriteRegStr   HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "NoModify"        1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}" "NoRepair"        1

  ; D-03: best-effort cleanup of stale per-user shortcut from pre-11.1 builds.
  SetShellVarContext current
  Delete "$SMPROGRAMS\DraftHorse.lnk"
  ; (next call to CreateShortcutAndAUMID below already wrapped to all-users)

  ; Stub calls — bodies are filled in by later plans. Each stub emits a
  ; DetailPrint so the installer log documents which milestone owns the work.
  Call InstallWebView2           ; plan 10-02
  Call CreateShortcutAndAUMID    ; plan 10-03 (D-03)
  Call AddFirewallRule           ; plan 10-03

  ; ARRICKS-19 — autostart at logon (all users). The queue watcher and the
  ; ARRICKS-13 default-mail guard only work while DraftHorse.exe runs, and
  ; upstream's planned Phase-10 autostart was never implemented — a reboot
  ; left scans queueing with no drafts created, and let a competing client
  ; (Outlook, on the validation PC) keep a stolen MAPI default with nothing
  ; alive to heal it. The app starts hidden to the tray (StartHidden) and
  ; is single-instance, so logon start is silent. Native view: HKLM Run is
  ; WOW64-redirected and admins expect the entry in the 64-bit key.
  SetRegView 64
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "DraftHorse" '"$INSTDIR\DraftHorse.exe"'
  SetRegView default

  ; ARRICKS-19 — relaunch after install. EnsureAppNotRunning closed any
  ; running instance at install start; without this, every silent upgrade
  ; left the watcher down until the next logon. explorer.exe indirection
  ; de-elevates the child so the tray app runs as the logged-on user, not
  ; the elevated installer token. Best-effort: under a SYSTEM-context
  ; Intune push there is no interactive session and this is a no-op — the
  ; Run key covers the next logon.
  Exec 'explorer.exe "$INSTDIR\DraftHorse.exe"'

  ; ARRICKS-17 — flush the shell icon cache after upgrades. Explorer and the
  ; Start Menu cache icons keyed on the exe PATH, which is identical across
  ; in-place upgrades — so a changed embedded icon (ARRICKS-16) keeps
  ; rendering as the old art until the cache refreshes. SHChangeNotify
  ; broadcasts the association/icon change; ie4uinit -show is the Win10/11
  ; icon-cache rebuild trigger. Both best-effort: a failure must never fail
  ; the install (already-correct machines are a no-op).
  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'
  ClearErrors
  ExecWait '"$SYSDIR\ie4uinit.exe" -show' $0
  IfErrors 0 +2
  StrCpy $0 "launch-failed"
  DetailPrint "icon-cache refresh (ie4uinit -show) rc=$0"
SectionEnd

;------------------------------------------------------------------------------
; BackupPreviousMailClient — D-10 / T-10-01-01
;
; Writes %ProgramData%\DraftHorse\uninst\previous-mail-client.json with the shape
;     {"previousClient": "<name>"|null, "backedUpAt": "<ISO-8601>"}
; so the uninstaller (plan 10-04) can restore the pre-install Mail client.
;
; Upgrade case (current (Default) is already "DraftHorse") intentionally preserves
; the existing backup — overwriting would lose the original previous-client
; name across reinstalls.
;
; Timestamp primitive: nsExec::ExecToStack invokes powershell.exe once to emit
; an ISO-8601 UTC date. nsExec ships with core NSIS, so no additional plugin
; is required. The newline returned by PowerShell is trimmed via StrCpy -2
; (strips the trailing CRLF).
;------------------------------------------------------------------------------

Function BackupPreviousMailClient
  ; ARRICKS fix: resolve %PROGRAMDATA% via ReadEnvStr (the same primitive
  ; the uninstaller's D-18 scrub already uses — see its comment). The old
  ; "$APPDATA\..\..\ProgramData" walk resolved to <userprofile>\ProgramData
  ; under the default `current` shell context, so the backup landed where
  ; the documented %ProgramData% location (D-12) never saw it. $9 carries
  ; the resolved path for the rest of this function.
  ReadEnvStr $9 PROGRAMDATA
  CreateDirectory "$9\DraftHorse\uninst"

  ReadRegStr $0 HKLM "SOFTWARE\Clients\Mail" ""

  ; QUICK-260423-ntu T3c — also capture the WOW6432 view's (Default)
  ; Mail client so the uninstaller can restore both views symmetrically.
  SetRegView 32
  ReadRegStr $4 HKLM "SOFTWARE\Clients\Mail" ""
  SetRegView default

  ; Upgrade case: existing install. Preserve original backup, skip write.
  StrCmp $0 "DraftHorse" AlreadyUs
  ; Clean install with no prior default Mail client.
  StrCmp $0 "" BackupNull

  ; WR-02: escape $0 (and $4) for JSON string context before interpolation.
  ; A mail client display name may legally contain `"` or `\` (e.g. locale-
  ; specific or custom enterprise names) which would otherwise produce
  ; invalid JSON and break the uninstaller's restore path.
  Push $0
  Call EscapeJsonString
  Pop $0

  Push $4
  Call EscapeJsonString
  Pop $4

  ; Get ISO-8601 UTC timestamp via Windows PowerShell (not pwsh — end-user
  ; machines may only have PS 5.1 per §Anti-Patterns in 10-RESEARCH.md).
  nsExec::ExecToStack 'powershell.exe -NoProfile -Command "[DateTime]::UtcNow.ToString(\"yyyy-MM-ddTHH:mm:ssZ\")"'
  Pop $2   ; exit code (discard)
  Pop $3   ; stdout (timestamp + trailing CRLF)
  StrCpy $3 $3 -2   ; strip trailing \r\n

  FileOpen  $1 "$9\DraftHorse\uninst\previous-mail-client.json" w
  StrCmp $4 "" BackupWriteNative32
  FileWrite $1 '{"previousClient":"$0","previousClient32":"$4","backedUpAt":"$3"}'
  Goto BackupWriteDone
BackupWriteNative32:
  FileWrite $1 '{"previousClient":"$0","previousClient32":null,"backedUpAt":"$3"}'
BackupWriteDone:
  FileClose $1
  DetailPrint "Previous Mail client backed up: native='$0' wow6432='$4'"
  Return

BackupNull:
  nsExec::ExecToStack 'powershell.exe -NoProfile -Command "[DateTime]::UtcNow.ToString(\"yyyy-MM-ddTHH:mm:ssZ\")"'
  Pop $2
  Pop $3
  StrCpy $3 $3 -2

  ; Also escape $4 for the WOW6432 side of the null-backup path (it may
  ; still have a non-empty value even when the native view is empty).
  Push $4
  Call EscapeJsonString
  Pop $4

  FileOpen  $1 "$9\DraftHorse\uninst\previous-mail-client.json" w
  StrCmp $4 "" BackupNullNoWow
  FileWrite $1 '{"previousClient":null,"previousClient32":"$4","backedUpAt":"$3"}'
  Goto BackupNullDone
BackupNullNoWow:
  FileWrite $1 '{"previousClient":null,"previousClient32":null,"backedUpAt":"$3"}'
BackupNullDone:
  FileClose $1
  DetailPrint "No previous native Mail client (wow6432='$4' backed up)"
  Return

AlreadyUs:
  DetailPrint "Upgrade detected — preserving existing previous-mail-client.json"
  Return
FunctionEnd

;------------------------------------------------------------------------------
; EnsureAppNotRunning — QUICK-260423-ntu T2 (installer scope)
;
; If a DraftHorse.exe process is running, offer clean-close-and-retry. Uses
; `tasklist` (core Windows tool, no plugin) for detection and `taskkill`
; WITHOUT /F for graceful shutdown — WM_CLOSE maps to the same
; intentionalQuit path in src/app/main.go that the tray "Quit" menu item
; triggers. Polls every 500ms up to 20 iterations (10s budget) for the
; process to exit; aborts on timeout.
;
; Image-name match only (no WMIC path-narrowing) — DraftHorse.exe is unique
; enough in practice that a duplicate unrelated process is an acceptable
; v3.0 risk, and WMIC has been removed on recent Windows 11 builds.
;
; Silent mode (`/S` — used by CI Pester harness) auto-selects "close and
; retry" so the test harness does not hang on a MessageBox.
;
; The un.EnsureAppNotRunning copy below is a byte-for-byte duplicate with
; the `un.` prefix — NSIS requires it for uninstaller-scope functions.
;------------------------------------------------------------------------------

Function EnsureAppNotRunning
  Push $0
  Push $1

  ; Quick probe — is any DraftHorse.exe running at all?
  nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq DraftHorse.exe" /NH /FO CSV'
  Pop $0   ; exit code
  Pop $1   ; stdout

  Push $1
  Push "DraftHorse.exe"
  Call StrContains
  Pop $0   ; "1" = found, "0" = not found
  StrCmp $0 "1" EANR_Found EANR_NotFound

EANR_Found:
  DetailPrint "DraftHorse.exe is running — attempting graceful close"
  IfSilent EANR_SilentRetry EANR_AskUser

EANR_AskUser:
  MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "DraftHorse is currently running. Click OK to close it and continue, or Cancel to abort the installer." IDOK EANR_SilentRetry IDCANCEL EANR_Cancel

EANR_Cancel:
  DetailPrint "User cancelled — aborting installer"
  Pop $1
  Pop $0
  Abort "Installer aborted by user (DraftHorse was running)."

EANR_SilentRetry:
  ; Send WM_CLOSE to every DraftHorse.exe instance (no /F — honours
  ; intentionalQuit path). /IM matches by image name; /T includes children.
  nsExec::ExecToStack 'taskkill /IM DraftHorse.exe'
  Pop $0
  Pop $1
  DetailPrint "taskkill /IM DraftHorse.exe rc=$0"

  ; Poll loop — 20 iterations * 500ms = 10s budget
  StrCpy $0 0
EANR_PollLoop:
  Sleep 500
  nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq DraftHorse.exe" /NH /FO CSV'
  Pop $1   ; exit code (discard)
  Pop $1   ; stdout
  Push $1
  Push "DraftHorse.exe"
  Call StrContains
  Pop $1
  StrCmp $1 "0" EANR_Exited
  IntOp $0 $0 + 1
  IntCmp $0 20 EANR_Timeout
  Goto EANR_PollLoop

EANR_Timeout:
  DetailPrint "ERROR: DraftHorse.exe did not exit within 10s"
  Pop $1
  Pop $0
  IfSilent EANR_SilentAbort
  MessageBox MB_OK|MB_ICONSTOP "DraftHorse did not close within 10 seconds. Please close it manually and re-run the installer."
EANR_SilentAbort:
  Abort "DraftHorse.exe still running after 10s close poll."

EANR_Exited:
  DetailPrint "DraftHorse.exe exited after $0 poll iterations"
  Pop $1
  Pop $0
  Return

EANR_NotFound:
  Pop $1
  Pop $0
FunctionEnd

;------------------------------------------------------------------------------
; StrContains (installer scope) — shared by EnsureAppNotRunning.
;
; Mirror of un.StrContains (lives in the uninstall section because the
; uninstaller already needed it for backup-JSON parsing). We keep a separate
; installer-scope copy rather than un.-prefixing both to avoid NSIS function
; scope restrictions.
;
; Push haystack, push needle. Pops "1" (found) or "0". Case-sensitive.
;------------------------------------------------------------------------------

Function StrContains
  Exch $R1   ; needle
  Exch
  Exch $R2   ; haystack
  Push $R3   ; needle-length
  Push $R4   ; haystack cursor
  Push $R5   ; needle cursor
  StrLen $R3 $R1
  StrCpy $R4 0
SC_Loop:
  StrCpy $R5 $R2 $R3 $R4
  StrCmp $R5 $R1 SC_Found
  StrCmp $R5 "" SC_NotFound
  IntOp $R4 $R4 + 1
  Goto SC_Loop
SC_Found:
  StrCpy $R1 "1"
  Goto SC_Done
SC_NotFound:
  StrCpy $R1 "0"
SC_Done:
  Pop $R5
  Pop $R4
  Pop $R3
  Pop $R2
  Exch $R1
FunctionEnd

;------------------------------------------------------------------------------
; EscapeJsonString — WR-02
;
; Pops one string off the stack, pushes a JSON-string-safe version. NSIS has
; no native JSON escaper; for the narrow context of a previous-mail-client
; display name written into a JSON string literal, the two characters that
; matter are `\` and `"`. Order matters: backslash MUST be escaped before
; quote (escaping quote first would then double-escape our new backslash).
;
; Register usage (local only — all $R* values restored before return):
;   $R0 = input/output string         $R3 = output buffer
;   $R1 = length                      $R4 = single char at cursor
;   $R2 = cursor (0-indexed)
;------------------------------------------------------------------------------

Function EscapeJsonString
  Exch $R0     ; input string
  Push $R1
  Push $R2
  Push $R3
  Push $R4

  StrCpy $R3 ""
  StrLen $R1 $R0
  StrCpy $R2 0

EscLoop:
  IntCmp $R2 $R1 EscDone
  StrCpy $R4 $R0 1 $R2
  StrCmp $R4 "\" EscBackslash
  StrCmp $R4 '"' EscQuote
  StrCpy $R3 "$R3$R4"
  Goto EscNext
EscBackslash:
  StrCpy $R3 "$R3\\"
  Goto EscNext
EscQuote:
  StrCpy $R3 '$R3\"'
  Goto EscNext
EscNext:
  IntOp $R2 $R2 + 1
  Goto EscLoop

EscDone:
  StrCpy $R0 $R3
  Pop $R4
  Pop $R3
  Pop $R2
  Pop $R1
  Exch $R0     ; push escaped string; restore caller's $R0
FunctionEnd

;------------------------------------------------------------------------------
; Stub Functions — bodies filled in by later plans. They exist in this plan
; so the NSIS script remains compilable and the Install-section Call sites
; resolve cleanly.
;------------------------------------------------------------------------------

;------------------------------------------------------------------------------
; DetectWebView2 — D-06 / INST-02
;
; Pushes "1" (runtime present) or "0" (absent) onto the stack. Probes three
; registry locations per Microsoft's WebView2 distribution guidance:
;   1. HKLM\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{GUID}  (64-bit view)
;   2. HKLM\SOFTWARE\Microsoft\EdgeUpdate\Clients\{GUID}              (direct HKLM)
;   3. HKCU\Software\Microsoft\EdgeUpdate\Clients\{GUID}              (per-user)
; Each probe rejects pv="" OR pv="0.0.0.0" (Microsoft's broken-install sentinel
; per WebView2 distribution docs) — matches the Go-side check in
; webview2_check.go (`pv != "" && pv != "0.0.0.0"`). The two layers MUST stay
; in sync or the installer skips the bootstrapper while the app shows the
; "WebView2 required" dialog.
;------------------------------------------------------------------------------

Function DetectWebView2
  Push $0
  Push $1

  SetRegView 64
  ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  StrCmp $0 "" TryDirectHKLM
  StrCmp $0 "0.0.0.0" TryDirectHKLM
  Goto WebView2Found

TryDirectHKLM:
  ReadRegStr $0 HKLM "SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  StrCmp $0 "" TryHKCU
  StrCmp $0 "0.0.0.0" TryHKCU
  Goto WebView2Found

TryHKCU:
  SetRegView 32
  ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  StrCmp $0 "" WebView2NotFound
  StrCmp $0 "0.0.0.0" WebView2NotFound
  Goto WebView2Found

WebView2NotFound:
  ; IN-04: reset registry view before returning so subsequent registry writes
  ; in the install section (AddFirewallRule, future growth) are not silently
  ; redirected through WOW6432Node or forced to the 32-bit view.
  SetRegView default
  Pop $1
  Pop $0
  Push "0"
  Return

WebView2Found:
  ; IN-04: see WebView2NotFound above — reset view before returning.
  SetRegView default
  DetailPrint "WebView2 runtime detected: $0"
  Pop $1
  Pop $0
  Push "1"
  Return
FunctionEnd

;------------------------------------------------------------------------------
; InstallWebView2 — D-05 / D-06 / D-07 / INST-02
;
; If runtime absent, extract the vendored bootstrapper to $INSTDIR, invoke with
; /silent /install, then poll the registry every 2 seconds for up to 30 iterations
; (60-second budget per D-06). The bootstrapper is known to exit before install
; completes (GH WebView2Feedback#1349, still unfixed) — registry poll is the
; only reliable completion signal.
;
; D-07: On poll timeout, continue (do NOT abort). The Wails app has its own
; runtime-missing recovery (D-08, see webview2_check.go). Log the timeout to
; $INSTDIR\install.log (append mode so prior log lines survive).
;
; Cleanup: the bootstrapper is deleted from $INSTDIR on both success and timeout
; branches — no leftover bootstrapper in the install dir (D-05 cleanup intent).
;------------------------------------------------------------------------------

Function InstallWebView2
  Call DetectWebView2
  Pop $0
  StrCmp $0 "1" WebView2Ready

  DetailPrint "WebView2 runtime not present — invoking bootstrapper"
  SetOutPath "$INSTDIR"
  File "${__FILEDIR__}\MicrosoftEdgeWebview2Setup.exe"

  ; D-06: bootstrapper exits before install completes (GH WebView2Feedback#1349)
  ExecWait '"$INSTDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install' $1
  DetailPrint "WebView2 bootstrapper exit=$1 — polling registry for completion"

  ; Poll every 2s for up to 30 iterations (60s budget) — D-06
  StrCpy $2 "0"
PollLoop:
  IntOp $2 $2 + 1
  IntCmp $2 30 PollTimeout
  Sleep 2000
  Call DetectWebView2
  Pop $0
  StrCmp $0 "1" WebView2Installed
  Goto PollLoop

PollTimeout:
  ; D-07: continue, do NOT abort. Wails app has runtime-recovery path (D-08).
  DetailPrint "WARNING: WebView2 bootstrap did not complete within 60s"
  FileOpen $3 "$INSTDIR\install.log" a
  FileWrite $3 "WebView2 bootstrap timed out after 60s; user will be prompted on app launch.$\r$\n"
  FileClose $3
  Delete "$INSTDIR\MicrosoftEdgeWebview2Setup.exe"
  Return

WebView2Installed:
  DetailPrint "WebView2 runtime install completed after $2 polls"
  Delete "$INSTDIR\MicrosoftEdgeWebview2Setup.exe"
  Return

WebView2Ready:
  DetailPrint "WebView2 runtime already present; skipping bootstrap"
  Return
FunctionEnd

;------------------------------------------------------------------------------
; CreateShortcutAndAUMID — D-13 / D-14 / D-15 / INST-01
;
; Creates the all-users Start Menu shortcut at $SMPROGRAMS\DraftHorse.lnk and
; stamps PKEY_AppUserModel_ID on it via the ApplicationID NSIS plugin. The
; stamped AUMID is what makes Phase 9's toast notifications persist in Action
; Center — the shortcut AUMID MUST match the Wails app's runtime AUMID
; (com.arrickspropane.drafthorse per D-15), which the plan 10-06 release pipeline
; injects into the .exe via ldflags.
;
; Plugin ABI (from NSIS ApplicationID v1.1):
;     ApplicationID::Set "<shortcut-path>" "<aumid-string>"
;     Pop $0     ; "0" = success, non-zero = error
; RESEARCH §Pitfall 2 — Pop is required; without it the rc is swallowed.
;------------------------------------------------------------------------------

Function CreateShortcutAndAUMID
  ; D-03 (Phase 11.1): tight SetShellVarContext all wrap around the All Users
  ; shortcut create. Pitfall 2: this also redirects $APPDATA, $LOCALAPPDATA,
  ; $DESKTOP — keep the wrap tight so the existing %ProgramData% walk at
  ; lines 666-676 stays in default `current` context.
  SetShellVarContext all
  ; ARRICKS-11: shortcut renamed to the display name. The .lnk filename is
  ; ALSO the app name Windows shows on toast notifications (the AUMID is
  ; resolved to the stamped shortcut's display name). Clean up the old-name
  ; shortcut first so upgrades don't leave both in the Start Menu.
  Delete "$SMPROGRAMS\DraftHorse.lnk"
  CreateShortcut "$SMPROGRAMS\DraftHorse.lnk" \
      "$INSTDIR\DraftHorse.exe" \
      "" \
      "$INSTDIR\DraftHorse.exe" 0 \
      SW_SHOWNORMAL "" \
      "DraftHorse — creates Gmail drafts from Simple MAPI (never sends)"

  ; D-14: stamp PKEY_AppUserModel_ID via ApplicationID plugin. Plugin loaded
  ; from src/installer/plugins/x86-unicode/ApplicationID.dll (vendored in plan 10-01).
  ; ApplicationID::Set pushes "0" on success, "-1" on error.
  ; D-15: production AUMID is com.arrickspropane.drafthorse (matches the ${AUMID} define).
  ApplicationID::Set "$SMPROGRAMS\DraftHorse.lnk" "${AUMID}"
  Pop $0
  SetShellVarContext current
  StrCmp $0 "0" AumidOk
  DetailPrint "WARNING: AUMID stamp rc=$0 — Action Center persistence may break"
  ; Do NOT halt the installer — continue install; Pester test (plan 10-05) will surface this in CI.
  Goto AumidDone
AumidOk:
  DetailPrint "AUMID stamped: ${AUMID}"
AumidDone:
FunctionEnd

;------------------------------------------------------------------------------
; AddFirewallRule — D-16 / INST-06
;
; Creates an inbound Windows Firewall rule named "DraftHorse OAuth loopback" bound
; to program=$INSTDIR\DraftHorse.exe. Pre-creating the rule at install time avoids
; the first-bind firewall prompt that Windows otherwise raises when DraftHorse
; binds its OAuth loopback listener — on RDS that prompt appears on the server
; console, invisible to the user in the RDP session (RESEARCH §Pitfall 4).
;
; Why netsh over `powershell.exe -Command "New-NetFirewallRule ..."`:
;   - single-line ExecWait with no PowerShell quote escaping
;   - works on all Windows 10+ SKUs without the NetSecurity PS module
;   - shorter NSIS script (RESEARCH §Pitfall 4 recommendation)
;
; Why program= (not localport=):
;   - DraftHorse binds 127.0.0.1:0 (ephemeral port) for the OAuth loopback server
;   - a program-scoped rule is both narrower (only this .exe) and port-stable
;   - broad port exposure is avoided; tampering with $INSTDIR requires admin
;
; Rule name "DraftHorse OAuth loopback" MUST match byte-for-byte the uninstall
; counterpart in plan 10-04 — a typo here breaks uninstall.
;------------------------------------------------------------------------------

Function AddFirewallRule
  ExecWait 'netsh advfirewall firewall add rule name="DraftHorse OAuth loopback" dir=in program="$INSTDIR\DraftHorse.exe" action=allow profile=any' $0
  DetailPrint "firewall add rule rc=$0"
  ; Do NOT halt on non-zero rc — group policy may block netsh writes, in which
  ; case OAuth on RDS will still hang but desktop Windows works (Windows
  ; auto-classifies loopback without the prompt on non-RDS sessions).
FunctionEnd

; ARRICKS-06: RegisterScheduledTask removed.
;
; It generated a Task Scheduler XML running "$INSTDIR\DraftHorse.exe
; --update-check-silent" as S-1-5-18 (SYSTEM) with RunLevel HighestAvailable,
; daily at 03:00 plus five minutes after every boot, and registered it with
; schtasks /RU SYSTEM. That task downloaded binaries from GitHub and
; MoveFileEx'd them over %ProgramFiles%, verified only against a SHA-256
; manifest served from the same URL as the binaries.
;
; un.RemoveScheduledTask is deliberately KEPT below: uninstalling over a
; machine that previously had an upstream build must still remove that task.

;------------------------------------------------------------------------------
; Uninstall section
;
; Full 10-step scrub (D-18) lives in plan 10-04. This stub keeps the
; uninstaller compilable so `makensis` does not fail on the scaffold plan.
;------------------------------------------------------------------------------

Section "Uninstall"
  ; Phase 11.1 D-16: remove the Scheduled Task FIRST so it cannot fire
  ; mid-uninstall (the task launches DraftHorse.exe --update-check-silent which
  ; would write to $INSTDIR while we are scrubbing it). schtasks /delete /f
  ; is idempotent — rc=0 (removed) and rc=1 (not found) are both swallowed
  ; by un.RemoveScheduledTask, so /AUTOUPDATE=0 installs uninstall cleanly too.
  Call un.RemoveScheduledTask

  ; QUICK-260423-ntu T2 — runs SECOND now (was first): if DraftHorse.exe is
  ; still running when the uninstaller starts, WM_CLOSE it and wait up to
  ; 10s for the intentionalQuit path to fire before any Delete runs.
  Call un.EnsureAppNotRunning

  ; D-18: 10-step full scrub. Steps execute in order; failures log but do
  ; not abort — we want to get as close to a clean state as possible even
  ; when some steps fail (e.g. firewall rule GPO-locked, AV-locked file).

  ; 1. Firewall rule — name MUST match plan 10-03 AddFirewallRule byte-for-byte
  ExecWait 'netsh advfirewall firewall delete rule name="DraftHorse OAuth loopback"' $0
  DetailPrint "firewall delete rule rc=$0"

  ; 2. Start Menu shortcut (plan 10-03 stamped the AUMID on this .lnk).
  ; ARRICKS-11: current name is DraftHorse.lnk; the go-mapi.lnk delete stays
  ; as belt-and-braces for installs that predate the rebrand.
  SetShellVarContext all
  Delete "$SMPROGRAMS\DraftHorse.lnk"
  Delete "$SMPROGRAMS\go-mapi.lnk"
  SetShellVarContext current

  ; 3. MAPI handler key. Also removes the ARRICKS-09 Capabilities subtree
  ; living under it. ARRICKS-10: HKLM\SOFTWARE\Clients is a WOW64 SHARED
  ; key — one physical key serves both views, so a single delete suffices
  ; (the old follow-up SetRegView 32 delete hit the same key twice).
  DeleteRegKey HKLM "SOFTWARE\Clients\Mail\DraftHorse"

  ; 3y. ARRICKS-19 — logon autostart entry (native view, matching the write).
  SetRegView 64
  DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "DraftHorse"
  SetRegView default

  ; 3z. ARRICKS-13 — per-user self-heal mirror written by the app's
  ; default-mail guard (defaultmail.go). HKCU\Software is not
  ; WOW64-redirected, so one delete suffices. The (Default) pointer is
  ; cleared only if it still names us — another client's later claim is
  ; not ours to clobber. D-19 multi-user caveat applies as with %APPDATA%:
  ; only the uninstalling user's hive is cleaned.
  DeleteRegKey HKCU "Software\Clients\Mail\DraftHorse"
  ReadRegStr $0 HKCU "Software\Clients\Mail" ""
  StrCmp $0 "DraftHorse" 0 +2
  DeleteRegValue HKCU "Software\Clients\Mail" ""

  ; 3a. ARRICKS-09 — mailto ProgID + RegisteredApplications pointer.
  ; Windows tolerates a dangling UserChoice (it falls back to asking the
  ; user), but the ProgID and the Default Apps listing must go.
  DeleteRegKey HKLM "SOFTWARE\Classes\DraftHorse.mailto"
  DeleteRegValue HKLM "SOFTWARE\RegisteredApplications" "DraftHorse"

  ; 4. Restore (Default) Mail client from backup (D-11)
  Call un.RestorePreviousMailClient

  ; Phase 11.1 D-18 case 6: scrub silent-update staging dir (Plan 11.1-04 writes
  ; here under SYSTEM context; Plan 11.1-05 owns the cleanup).
  ; Use ReadEnvStr to read %PROGRAMDATA% directly. The old
  ; `$APPDATA\..\..\ProgramData` pattern this file previously used
  ; resolves to `<userprofile>\ProgramData` under default `current` context — a
  ; non-existent path. Verified by Plan 11.1-05 sandbox UAT (Test B updates_dir_after=true
  ; while planted file remained at C:\ProgramData\DraftHorse\updates). ReadEnvStr is
  ; reliable across user/SYSTEM contexts.
  ReadEnvStr $0 PROGRAMDATA
  RMDir /r "$0\DraftHorse\updates"

  ; Phase 11.1 W7: belt-and-braces cleanup of *.old.<pid> orphans left by
  ; silent-updater swaps (Plan 11.1-04 swapInPlace renames the old binary
  ; aside via MoveFileEx before placing the new one). Plan 11.1-04 also
  ; cleans these proactively at silent-update start; this catches any orphans
  ; that survive past the last cycle. Runs before the binary scrub at step 9.
  Push "$INSTDIR"
  Call un.ScrubOldOrphans
  Push "$PROGRAMFILES32\DraftHorse"
  Call un.ScrubOldOrphans

  ; 5. %ProgramData%\DraftHorse\uninst\ — remove AFTER the restore (step 4) since
  ; the restore reads from this directory. ARRICKS fix: resolve %PROGRAMDATA%
  ; via ReadEnvStr (see the D-18 comment above); re-read here because the
  ; ScrubOldOrphans calls above may have clobbered registers.
  ReadEnvStr $9 PROGRAMDATA
  RMDir /r "$9\DraftHorse\uninst"
  RMDir    "$9\DraftHorse"   ; only if empty (non-recursive)

  ; 6. %TEMP%\DraftHorse\ — best-effort. Under elevated uninstall this is the
  ; SYSTEM user's TEMP, not the real user's. Real users' temp already
  ; auto-cleans via the delete-on-process privacy model in src/app/watcher_bridge.go.
  RMDir /r "$TEMP\DraftHorse"

  ; 7. %APPDATA%\DraftHorse\ — uninstalling user only (D-19 multi-user caveat:
  ; other users on the machine retain their own copies; documented in README)
  RMDir /r "$APPDATA\DraftHorse"

  ; 7b. ARRICKS-21 — the dedicated browser profile holds the location
  ; account's Google session cookies; it must not outlive the app. Same
  ; uninstalling-user-only caveat as step 7. Best-effort: a running Edge
  ; window on this profile may pin some files until it closes.
  RMDir /r "$LOCALAPPDATA\DraftHorse\browser-profile"
  RMDir /r "$LOCALAPPDATA\DraftHorse\browser-profile-2"  ; V4 slot-2 account profile

  ; 8. Windows Credential Manager — target is "<service>:<username>" per
  ; zalando/go-keyring Windows backend (PATTERNS.md §Shared Pattern 3).
  ; CONTEXT specifics line 199 wrote the slash-separated form — WRONG.
  ; Verified target: "DraftHorse:oauth-tokens" (colon). This is the byte-for-byte
  ; value returned by zalando/go-keyring's credName() method for
  ; service="DraftHorse" + username="oauth-tokens" (see src/app/auth.go:27-28).
  ExecWait 'cmdkey /delete:DraftHorse:oauth-tokens' $0
  DetailPrint "cmdkey /delete:DraftHorse:oauth-tokens rc=$0"

  ; 9. Binaries (x64 side) — the un.EnsureAppNotRunning call at the start
  ; of this section has already closed any running DraftHorse.exe so these
  ; Deletes succeed.
  Delete "$INSTDIR\DraftHorse.exe"
  Delete "$INSTDIR\DraftHorse.dll"
  Delete "$INSTDIR\DraftHorse.ico"
  Delete "$INSTDIR\uninstall.exe"
  Delete "$INSTDIR\install.log"

  ; 9b. Diagnostic scripts (QUICK-260423-msq)
  Delete "$INSTDIR\diagnostics\collect-registration.ps1"
  Delete "$INSTDIR\diagnostics\collect-runtime.ps1"
  RMDir  "$INSTDIR\diagnostics"

  ; 9c. QUICK-260423-ntu T3c — x86 DLL + its parallel install dir
  Delete "$PROGRAMFILES32\DraftHorse\DraftHorse.dll"
  RMDir  "$PROGRAMFILES32\DraftHorse"

  ; 9d. ARRICKS-10 — x64 DLL pinned at the fixed %ProgramFiles% path for
  ; non-default /D= installs. No-ops for default installs (step 9 already
  ; deleted it as $INSTDIR\DraftHorse.dll; RMDir skips non-empty dirs).
  Delete "$PROGRAMFILES64\DraftHorse\DraftHorse.dll"
  RMDir  "$PROGRAMFILES64\DraftHorse"

  ; 10. Install dir (RMDir non-recursive — only removes if empty)
  RMDir "$INSTDIR"

  ; Add/Remove Programs entry
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

  DetailPrint "Uninstall complete"
SectionEnd

; D-11: on uninstall, restore HKLM\SOFTWARE\Clients\Mail\(Default) from the backup JSON.
; Only restores if:
;   1. backup JSON exists AND
;   2. current (Default) still points at "DraftHorse" (don't clobber another installer) AND
;   3. the restoration target's subkey still exists under HKLM\SOFTWARE\Clients\Mail\
; Otherwise: try fallbacks (Microsoft Outlook -> Outlook -> Windows Mail) or clear to "".
Function un.RestorePreviousMailClient
  ; ARRICKS fix: resolve %PROGRAMDATA% via ReadEnvStr into $9 (used by the
  ; backup-JSON reads below). The old $APPDATA-relative walk pointed at
  ; <userprofile>\ProgramData and never matched where the backup lives.
  ReadEnvStr $9 PROGRAMDATA

  ; Guard 1: only restore if current (Default) is still our claim
  ReadRegStr $0 HKLM "SOFTWARE\Clients\Mail" ""
  StrCmp $0 "DraftHorse" 0 DoneRestore
  DetailPrint "Mail (Default) is still 'DraftHorse' — proceeding with restore"

  ; IN-05: parse the backup JSON via PowerShell's ConvertFrom-Json instead of a
  ; naive substring search. The previous substring-based detection of
  ; `"previousClient":null` would false-match on a display-name containing that
  ; exact literal (contrived, but brittle); ConvertFrom-Json unambiguously
  ; distinguishes `null` from a string value, and also correctly unescapes
  ; JSON-escaped `"` / `\` characters written by EscapeJsonString (WR-02).
  ;
  ; Shape (from BackupPreviousMailClient, single line):
  ;   {"previousClient":null,"backedUpAt":"..."}     OR
  ;   {"previousClient":"<name>","backedUpAt":"..."}
  ;
  ; PowerShell output:
  ;   - missing file / parse error: non-zero exit code -> fall through to fallbacks
  ;   - previousClient=null:        exit 0, stdout = "" (just trailing CRLF)
  ;   - previousClient="<name>":    exit 0, stdout = "<name>" + trailing CRLF
  StrCpy $1 ""  ; candidate name
  IfFileExists "$9\DraftHorse\uninst\previous-mail-client.json" 0 NoBackup
  nsExec::ExecToStack 'powershell.exe -NoProfile -Command "try { $$j = Get-Content -LiteralPath ''$9\DraftHorse\uninst\previous-mail-client.json'' -Raw | ConvertFrom-Json; if ($$null -ne $$j.previousClient) { Write-Output $$j.previousClient } exit 0 } catch { exit 1 }"'
  Pop $4    ; exit code
  Pop $1    ; stdout (empty if null or parse error)
  StrCmp $4 "0" 0 TryFallbacks
  ; Strip trailing CRLF if present (same pattern as BackupPreviousMailClient's timestamp)
  ; IntCmp len 2 <equal> <less> <greater>: trim when len >= 2 (equal or greater);
  ; skip when len < 2 (no CRLF could fit).
  StrLen $4 $1
  IntCmp $4 2 0 SkipTrim 0
  StrCpy $1 $1 -2
SkipTrim:
  StrCmp $1 "" TryFallbacks
  DetailPrint "Backup contains previousClient='$1'"
  Goto VerifyAndRestore

NoBackup:
  DetailPrint "No backup JSON at %ProgramData%\DraftHorse\uninst\previous-mail-client.json — trying fallbacks"
  Goto TryFallbacks

VerifyAndRestore:
  ; Confirm the target subkey still exists under HKLM\SOFTWARE\Clients\Mail\<name>
  ; (some other installer may have removed it since backup).
  ReadRegStr $5 HKLM "SOFTWARE\Clients\Mail\$1" ""
  StrCmp $5 "" TryFallbacks     ; subkey gone; fall through
  WriteRegStr HKLM "SOFTWARE\Clients\Mail" "" "$1"
  DetailPrint "Restored Mail (Default) to: $1"
  Goto DoneRestore

TryFallbacks:
  ; Try "Microsoft Outlook" -> "Outlook" -> "Windows Mail" -> clear
  ReadRegStr $5 HKLM "SOFTWARE\Clients\Mail\Microsoft Outlook" ""
  StrCmp $5 "" TryOutlook
  WriteRegStr HKLM "SOFTWARE\Clients\Mail" "" "Microsoft Outlook"
  DetailPrint "Fallback: restored Mail (Default) to 'Microsoft Outlook'"
  Goto DoneRestore
TryOutlook:
  ReadRegStr $5 HKLM "SOFTWARE\Clients\Mail\Outlook" ""
  StrCmp $5 "" TryWinMail
  WriteRegStr HKLM "SOFTWARE\Clients\Mail" "" "Outlook"
  DetailPrint "Fallback: restored Mail (Default) to 'Outlook'"
  Goto DoneRestore
TryWinMail:
  ReadRegStr $5 HKLM "SOFTWARE\Clients\Mail\Windows Mail" ""
  StrCmp $5 "" ClearDefault
  WriteRegStr HKLM "SOFTWARE\Clients\Mail" "" "Windows Mail"
  DetailPrint "Fallback: restored Mail (Default) to 'Windows Mail'"
  Goto DoneRestore
ClearDefault:
  WriteRegStr HKLM "SOFTWARE\Clients\Mail" "" ""
  DetailPrint "No fallback Mail client available — cleared (Default)"
DoneRestore:
  ; QUICK-260423-ntu T3c — symmetric WOW6432 restore. If the backup JSON
  ; is present and contains a non-null previousClient32 value, write it
  ; back to the 32-bit view's (Default). Parse via PowerShell's
  ; ConvertFrom-Json — same pattern as the native-view restore above.
  IfFileExists "$9\DraftHorse\uninst\previous-mail-client.json" 0 NoWow6432
  nsExec::ExecToStack 'powershell.exe -NoProfile -Command "try { $$j = Get-Content -LiteralPath ''$9\DraftHorse\uninst\previous-mail-client.json'' -Raw | ConvertFrom-Json; if ($$null -ne $$j.previousClient32) { Write-Output $$j.previousClient32 } exit 0 } catch { exit 1 }"'
  Pop $4    ; exit code
  Pop $1    ; stdout
  StrCmp $4 "0" 0 NoWow6432
  StrLen $4 $1
  IntCmp $4 2 0 WowSkipTrim 0
  StrCpy $1 $1 -2
WowSkipTrim:
  StrCmp $1 "" NoWow6432
  SetRegView 32
  ReadRegStr $5 HKLM "SOFTWARE\Clients\Mail\$1" ""
  StrCmp $5 "" WowKeyGone
  WriteRegStr HKLM "SOFTWARE\Clients\Mail" "" "$1"
  DetailPrint "Restored WOW6432 Mail (Default) to: $1"
  Goto WowDone
WowKeyGone:
  DetailPrint "WOW6432 previous client subkey missing — skipping restore"
WowDone:
  SetRegView default
  Goto Wow6432End
NoWow6432:
Wow6432End:
FunctionEnd

; Helper: case-sensitive substring check. Push haystack, push needle. Pops "1" (found) or "0".
Function un.StrContains
  Exch $R1   ; needle
  Exch
  Exch $R2   ; haystack
  Push $R3   ; needle-length
  Push $R4   ; haystack cursor
  Push $R5   ; needle cursor
  StrLen $R3 $R1
  StrCpy $R4 0
un.SC_Loop:
  StrCpy $R5 $R2 $R3 $R4
  StrCmp $R5 $R1 un.SC_Found
  StrCmp $R5 "" un.SC_NotFound   ; cursor past end of haystack
  IntOp $R4 $R4 + 1
  Goto un.SC_Loop
un.SC_Found:
  StrCpy $R1 "1"
  Goto un.SC_Done
un.SC_NotFound:
  StrCpy $R1 "0"
un.SC_Done:
  ; IN-02: correct save/restore sequence. Prelude saved prev$R1, prev$R2 via
  ; Exch $R1; Exch; Exch $R2 (2 saves, stack = [prev$R1, prev$R2]), then
  ; Push $R3..$R5 (stack = [prev$R1, prev$R2, prev$R3, prev$R4, prev$R5]).
  ; Result lives in $R1. Exit: pop $R5..$R3, pop prev$R2 into $R2, then Exch $R1
  ; swaps the remaining prev$R1 on stack with the result in $R1 — caller sees
  ; the result on top of stack, $R1 is restored, $R2 is restored.
  Pop $R5
  Pop $R4
  Pop $R3
  Pop $R2     ; restore prev$R2
  Exch $R1    ; swap prev$R1 on stack with result in $R1: stack top = result, $R1 = prev$R1
FunctionEnd

; Helper: extract substring between two delimiters. Push haystack, push startDelim, push endDelim.
; Returns the substring on the stack, or "" if not found.
Function un.StrExtract
  Exch $R1   ; endDelim
  Exch
  Exch $R2   ; startDelim
  Exch 2
  Exch $R3   ; haystack
  Push $R4   ; startDelim-length
  Push $R5   ; startIndex
  Push $R6   ; cursor/endIndex
  Push $R7   ; temp
  StrLen $R4 $R2
  StrCpy $R5 0
un.SE_FindStart:
  StrCpy $R7 $R3 $R4 $R5
  StrCmp $R7 $R2 un.SE_FoundStart
  StrCmp $R7 "" un.SE_NotFound
  IntOp $R5 $R5 + 1
  Goto un.SE_FindStart
un.SE_FoundStart:
  IntOp $R5 $R5 + $R4     ; cursor past startDelim
  StrCpy $R6 $R5
un.SE_FindEnd:
  StrCpy $R7 $R3 1 $R6
  StrCmp $R7 $R1 un.SE_FoundEnd
  StrCmp $R7 "" un.SE_NotFound
  IntOp $R6 $R6 + 1
  Goto un.SE_FindEnd
un.SE_FoundEnd:
  IntOp $R7 $R6 - $R5
  StrCpy $R1 $R3 $R7 $R5
  Goto un.SE_Done
un.SE_NotFound:
  StrCpy $R1 ""
un.SE_Done:
  ; IN-03: correct save/restore sequence. Prelude:
  ;   Exch $R1 (save prev$R1), Exch, Exch $R2 (save prev$R2), Exch 2, Exch $R3
  ;   (save prev$R3). Post-prelude stack (bottom->top):
  ;     [prev$R2, prev$R1, prev$R3]
  ;   Then Push $R4..$R7 adds 4 items. Full stack:
  ;     [prev$R2, prev$R1, prev$R3, prev$R4, prev$R5, prev$R6, prev$R7]
  ; Result lives in $R1. Cleanup: pop R7..R4 (restores R4..R7), Pop R3 (restores
  ; prev$R3), then the remaining stack is [prev$R2, prev$R1]. We need to restore
  ; $R2 = prev$R2 and $R1 = prev$R1, and push result. Swap the top two so
  ; prev$R2 is on top, Pop into $R2, then Exch $R1 swaps prev$R1 with result.
  Pop $R7
  Pop $R6
  Pop $R5
  Pop $R4
  Pop $R3     ; restore prev$R3
  Exch        ; swap top two: stack was [prev$R2, prev$R1] -> [prev$R1, prev$R2]
  Pop $R2     ; restore prev$R2
  Exch $R1    ; swap prev$R1 on stack with result in $R1: stack top = result, $R1 = prev$R1
FunctionEnd

;------------------------------------------------------------------------------
; un.EnsureAppNotRunning — QUICK-260423-ntu T2 (uninstaller scope)
;
; Byte-for-byte duplicate of EnsureAppNotRunning above with the un. prefix
; required by NSIS for uninstaller-scope functions. NSIS macros would avoid
; the duplication but the body is small enough that inline is clearer.
; Uses un.StrContains (already defined above).
;------------------------------------------------------------------------------

Function un.EnsureAppNotRunning
  Push $0
  Push $1

  nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq DraftHorse.exe" /NH /FO CSV'
  Pop $0
  Pop $1

  Push $1
  Push "DraftHorse.exe"
  Call un.StrContains
  Pop $0
  StrCmp $0 "1" unEANR_Found unEANR_NotFound

unEANR_Found:
  DetailPrint "DraftHorse.exe is running — attempting graceful close"
  IfSilent unEANR_SilentRetry unEANR_AskUser

unEANR_AskUser:
  MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "DraftHorse is currently running. Click OK to close it and continue, or Cancel to abort the uninstaller." IDOK unEANR_SilentRetry IDCANCEL unEANR_Cancel

unEANR_Cancel:
  DetailPrint "User cancelled — aborting uninstaller"
  Pop $1
  Pop $0
  Abort "Uninstaller aborted by user (DraftHorse was running)."

unEANR_SilentRetry:
  nsExec::ExecToStack 'taskkill /IM DraftHorse.exe'
  Pop $0
  Pop $1
  DetailPrint "taskkill /IM DraftHorse.exe rc=$0"

  StrCpy $0 0
unEANR_PollLoop:
  Sleep 500
  nsExec::ExecToStack 'tasklist /FI "IMAGENAME eq DraftHorse.exe" /NH /FO CSV'
  Pop $1
  Pop $1
  Push $1
  Push "DraftHorse.exe"
  Call un.StrContains
  Pop $1
  StrCmp $1 "0" unEANR_Exited
  IntOp $0 $0 + 1
  IntCmp $0 20 unEANR_Timeout
  Goto unEANR_PollLoop

unEANR_Timeout:
  DetailPrint "ERROR: DraftHorse.exe did not exit within 10s"
  Pop $1
  Pop $0
  IfSilent unEANR_SilentAbort
  MessageBox MB_OK|MB_ICONSTOP "DraftHorse did not close within 10 seconds. Please close it manually and re-run the uninstaller."
unEANR_SilentAbort:
  Abort "DraftHorse.exe still running after 10s close poll."

unEANR_Exited:
  DetailPrint "DraftHorse.exe exited after $0 poll iterations"
  Pop $1
  Pop $0
  Return

unEANR_NotFound:
  Pop $1
  Pop $0
FunctionEnd

;------------------------------------------------------------------------------
; un.RemoveScheduledTask — Phase 11.1 D-16
;
; Idempotent removal of the silent-update Scheduled Task. Runs unconditionally
; — installs that did NOT register the task (e.g. /AUTOUPDATE=0) still call
; this; rc=1 ("task not found") is swallowed. /F suppresses the confirmation
; prompt. Logged via DetailPrint for installer-log forensic trail.
;------------------------------------------------------------------------------

Function un.RemoveScheduledTask
  ; The task name is UPSTREAM's, from before the fork — never renamed, because
  ; this function's whole job is scrubbing what an old install left behind.
  ExecWait 'schtasks /delete /tn "go-mapi Auto Update" /f' $0
  DetailPrint "schtasks /delete rc=$0 (0=removed, 1=not found — both ok)"
FunctionEnd

;------------------------------------------------------------------------------
; un.ScrubOldOrphans — Phase 11.1 W7
;
; Belt-and-braces cleanup of *.old.<pid> orphan files left behind by the
; silent updater's MoveFileEx rename-while-running pattern (Plan 11.1-04
; swapInPlace). Plan 11.1-04 cleans these proactively at silent-update start;
; this uninstaller helper catches any orphans that survive past the last
; update cycle. Pattern matches both DraftHorse.exe.old.<pid> and
; DraftHorse.dll.old.<pid> via NSIS FindFirst/FindNext.
;
; Stack contract: caller pushes the directory path (e.g. "$INSTDIR"), function
; pops it, scrubs all "*.old.*" matches in that directory, returns nothing.
;------------------------------------------------------------------------------

Function un.ScrubOldOrphans
  Pop $R0   ; directory path (e.g. "$INSTDIR")
  Push $R1
  Push $R2

  ClearErrors
  FindFirst $R1 $R2 "$R0\*.old.*"
  IfErrors un.SOO_Done
un.SOO_Loop:
  StrCmp $R2 "" un.SOO_Done
  Delete "$R0\$R2"
  DetailPrint "scrubbed orphan: $R0\$R2"
  ClearErrors
  FindNext $R1 $R2
  IfErrors un.SOO_Done
  Goto un.SOO_Loop
un.SOO_Done:
  FindClose $R1
  Pop $R2
  Pop $R1
FunctionEnd
