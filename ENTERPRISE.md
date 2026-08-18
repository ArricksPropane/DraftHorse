# DraftHorse for IT Administrators

Audience: Windows / IT admins deploying DraftHorse at scale — Intune,
managed desktops, RDS/Citrix, group policy.

> DraftHorse is a hardened fork of upstream go-mapi. **Installed binary
> and registry identifier names intentionally remain `go-mapi*`** so
> existing go-mapi 3.x installs upgrade in place; display surfaces —
> including the installer filename `DraftHorse-setup.exe`, Add/Remove
> Programs, Start Menu, Default Apps, and the app UI — say DraftHorse.
> Deployment tooling should key off the `go-mapi` identifiers throughout
> this document.

## At a glance

**Operational facts**

- Single-file NSIS installer (`DraftHorse-setup.exe`) — All Users only, no MSI
- Silent install via `/S`
- **No automatic updates.** This fork removed the upstream silent
  auto-updater (SYSTEM Scheduled Task) entirely. Updates are notify-only:
  the app tells the user; an admin or the user runs the new installer.
  The legacy `/AUTOUPDATE=` flag is accepted but **inert**, and
  `--update-check-silent` is a deliberate no-op, so old deployment
  scripts cannot re-arm updating.
- ~40–50 MB RAM per signed-in session (idle — see *RAM sizing* below)
- SHA-256 checksums published with every release (`SHA256SUMS.txt`)
- Per-user OAuth tokens in Windows Credential Manager (DPAPI-scoped)
- Outbound network: Google OAuth, Gmail API, GitHub Releases (update
  check + user-initiated download). Nothing else. See *Network egress*
  for the exact allowlist.

**Positioning**

- LGPL-3.0-or-later — FOSS, no per-seat licensing, source on GitHub
- No telemetry, no content retention, no analytics

## Code signing status

**Current prereleases are unsigned** (the release pipeline supports
SignPath.io signing but the signing secrets are not yet configured).

Practical consequences for managed deployments:

- **SmartScreen** will warn users on first interactive run
  (**More info → Run anyway**). Silent installs (`/S`) bypass the dialog.
- **WDAC** policies requiring signed binaries will block the app; allow by
  hash or path until signed releases ship.
- **AppLocker**: use Hash rules against `SHA256SUMS.txt`, or Path rules
  anchored at `%ProgramFiles%\go-mapi\` and `%ProgramFiles(x86)%\go-mapi\`.

## Install modes

### All Users (the only mode)

The installer is **All Users only** and requires UAC elevation. DraftHorse
registers as the machine-wide MAPI handler, which is inherently
machine-wide.

File layout:

- `%ProgramFiles%\go-mapi\` — app (`go-mapi.exe`), x64 MAPI DLL,
  uninstaller, diagnostics scripts
- `%ProgramFiles(x86)%\go-mapi\` — x86 MAPI DLL (32-bit scanner software
  and other 32-bit MAPI callers load this one)

Registry footprint (machine):

- `HKLM\SOFTWARE\Clients\Mail\go-mapi` — MAPI registration. `DLLPath` is a
  single `REG_EXPAND_SZ` value, `%ProgramFiles%\go-mapi\go-mapi.dll`;
  Windows expands it per caller, so 32-bit and 64-bit MAPI callers each
  load the matching DLL from ONE registration. (`SOFTWARE\Clients` is a
  WOW64 *shared* key — there is no separate WOW6432Node registration, and
  tooling should not look for one.)
- `HKLM\SOFTWARE\Clients\Mail` `(Default)` = `go-mapi` (default MAPI client
  pointer; previous value backed up and restored on uninstall)
- `HKLM\SOFTWARE\Classes\go-mapi.mailto` — `mailto:` ProgID
- `HKLM\SOFTWARE\Clients\Mail\go-mapi\Capabilities` +
  `HKLM\SOFTWARE\RegisteredApplications\go-mapi` — Windows Default Apps
  registration (the app appears as **DraftHorse** in Settings > Default
  apps)
- `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\go-mapi` —
  Add/Remove Programs (DisplayName: DraftHorse)
- `HKLM\Software\Microsoft\Windows\CurrentVersion\Run` value `go-mapi` —
  logon autostart (all users). The queue watcher and the default-mail
  guard only work while the tray app runs; the installer also relaunches
  the app immediately after interactive installs (SYSTEM-context pushes
  rely on the next logon).

Registry footprint (per user, written by the app at runtime):

- `HKCU\Software\Clients\Mail\go-mapi` (+ the HKCU `(Default)` pointer) —
  written **only** if the app's default-mail guard detects that another
  product has stolen the machine-wide MAPI default (see *Default-mail
  self-healing* below). The uninstaller removes the uninstalling user's
  copy.

The installer backs up the previous default mail client name to
`%ProgramData%\go-mapi\uninst\previous-mail-client.json` and restores it on
uninstall. The uninstaller also removes the legacy upstream
`go-mapi Auto Update` Scheduled Task if a pre-fork install left one behind.

## Default-mail self-healing (fleet-relevant behavior)

Every hour (and at startup), the running app checks that
`Clients\Mail (Default)` still resolves to `go-mapi`. If another installer
(commonly Outlook) has claimed the default, the app re-claims it for the
**current user** by writing an unelevated per-user override at
`HKCU\Software\Clients\Mail`. This keeps scanner scan-to-email working
without IT intervention.

Plan for this on machines where users legitimately run another Simple MAPI
mail client: with DraftHorse installed and running, DraftHorse **will** win
the MAPI default for that user within the hour. If a host needs a different
default client, don't deploy DraftHorse to it (or stop the tray app).
Repairs are logged to `%APPDATA%\go-mapi\app.log`.

## mailto: default (Settings > Default apps)

DraftHorse registers as a candidate `mailto:` handler. Windows does **not**
allow apps to set themselves as the mailto default programmatically
(`UserChoice` is hash-protected and enforced by the UCPD driver), so:

- **Per machine/user, interactively:** the user picks DraftHorse once in
  Settings > Default apps (the app shows a one-click deep link when it
  detects it isn't the default).
- **Fleet-wide, zero-touch:** deploy the default-associations profile —
  see `docs/mailto-default-associations.xml` — via Intune Device
  Configuration (or DISM `/Import-DefaultAppAssociations` for imaged
  hosts).

A mailto click runs `go-mapi.exe --mailto "%1"`, which opens Gmail web
compose prefilled from the link and exits. No draft API call, no stored
credentials involved.

## Silent install

Silent install with all defaults:

```
DraftHorse-setup.exe /S
```

Silent install to a custom path (`/D` last, unquoted — NSIS restriction;
the MAPI DLLs are additionally pinned at the fixed `%ProgramFiles%`
locations regardless of `/D`, because the MAPI registration resolves
there):

```
DraftHorse-setup.exe /S /D=C:\Program Files\go-mapi
```

The installer is idempotent — running it over an existing install upgrades
in place (the previous-mail-client backup is preserved across upgrades).
The legacy `/AUTOUPDATE=1` flag is accepted and ignored.

### Exit codes

NSIS conventions: `0` success, `1` user/interactive cancel, `2` runtime
failure (including `go-mapi.exe` still running after the 10-second
graceful-close poll on silent installs). Treat any non-zero exit as
failure.

### Detection rule (Intune / SCCM)

Key: `HKLM\SOFTWARE\Clients\Mail\go-mapi`, value `DLLPath` exists.
(The Uninstall key also works; the Clients\Mail key is the one that
actually makes the product functional, so it is the better health signal.)

## Updates (notify-only)

The app checks GitHub Releases daily and shows a banner when a newer
version exists. Nothing is downloaded or installed automatically — the
user (or your deployment pipeline) runs the new `DraftHorse-setup.exe`.

For managed fleets, the recommended pattern is to treat updates like any
other Win32 app revision: push the new installer via Intune with `/S`.
In-place upgrade over a running app is supported (the installer closes
the tray app gracefully, with a silent-mode retry window).

To disable even the update *check* per user, untick "Check for updates"
in the tray menu (persisted per user in `%APPDATA%\go-mapi\settings.json`).

## Verify download integrity

Every release publishes `SHA256SUMS.txt`:

```
https://github.com/egkrateia247/DraftHorse/releases/latest/download/SHA256SUMS.txt
```

```powershell
$base = "https://github.com/egkrateia247/DraftHorse/releases/download/vX.Y.Z"
$sums = (Invoke-WebRequest "$base/SHA256SUMS.txt").Content
$expected = ($sums -split "`n" |
    Where-Object { $_ -match 'DraftHorse-setup\.exe' } |
    ForEach-Object { ($_ -split '\s+')[0] })
$actual = (Get-FileHash .\DraftHorse-setup.exe -Algorithm SHA256).Hash.ToLower()
if ($actual -eq $expected) {
    Write-Output "OK ($actual)"
} else {
    throw "Checksum mismatch: expected $expected, got $actual"
}
```

## Network egress

If you filter outbound traffic, allow exactly these:

| Host | Purpose |
|---|---|
| `accounts.google.com` | Google sign-in (OAuth consent) |
| `oauth2.googleapis.com` | OAuth token exchange / refresh |
| `www.googleapis.com` | Gmail API (`/gmail/v1/users/me/drafts` — draft creation is the only Gmail call) |
| `github.com`, `objects.githubusercontent.com` | Update check + user-initiated installer download |

> Note: the app does **not** use `gmail.googleapis.com` (an allowlist
> built from upstream's older docs breaks sign-in). The Gmail API is
> reached via `www.googleapis.com`.

No telemetry. No content retention. Email content is never stored outside
Gmail's own API; queued messages transit `%LOCALAPPDATA%\go-mapi\queue\`
on the local machine only, and attachments are constrained to that
directory by validation.

Credential storage: per-user OAuth tokens in Windows Credential Manager
(target `go-mapi:oauth-tokens`, DPAPI-scoped). OAuth scope is
`gmail.compose` + basic profile only — the app cannot send, delete, or
read mail.

## OAuth loopback firewall rule

Sign-in opens a short-lived loopback listener on `127.0.0.1` (ephemeral
port). The installer creates a program-scoped inbound allow rule named
`go-mapi OAuth loopback` for `%ProgramFiles%\go-mapi\go-mapi.exe` to
suppress the first-bind consent prompt. If GPO blocks the installer's
`netsh` write, pre-create an identical rule via policy:

```powershell
New-NetFirewallRule `
  -DisplayName "go-mapi OAuth loopback" `
  -Direction Inbound `
  -Action Allow `
  -Program "$env:ProgramFiles\go-mapi\go-mapi.exe" `
  -Profile Any
```

## RAM sizing

~40–50 MB working set per signed-in idle session; modest growth during
draft creation, returning to baseline afterwards. For RDS / Citrix
capacity planning, treat 50 MB per concurrent signed-in session as a
working ceiling.

## Limitations

### Multi-user / RDS hosts

The uninstaller scrubs the uninstalling user's profile and all
machine-wide locations. It does **not** enumerate other user profiles.
Per-user residue after uninstall (harmless — see below):

- `%APPDATA%\go-mapi\` (settings, log)
- Credential Manager target `go-mapi:oauth-tokens` (DPAPI-scoped)
- `HKCU\Software\Clients\Mail\go-mapi` for users other than the
  uninstalling one, if the default-mail guard ever self-healed in their
  session

The Credential Manager residue is a DPAPI-encrypted refresh token with no
privileged caller once the binary is gone; the HKCU registry residue is a
dangling pointer Windows ignores. If policy requires cleanup anyway, run
per user at logon:

```powershell
Remove-Item -Recurse -Force "$env:APPDATA\go-mapi" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "HKCU:\Software\Clients\Mail\go-mapi" -ErrorAction SilentlyContinue
cmdkey /list:go-mapi:oauth-tokens 2>$null | Out-Null
if ($LASTEXITCODE -eq 0) { cmdkey /delete:go-mapi:oauth-tokens | Out-Null }
```

### No MSI

The installer is an NSIS EXE. There is no MSI wrapper. Use Intune Win32
app deployment, or a GP startup script for GPO-only environments.

## Support

[GitHub Issues](https://github.com/egkrateia247/DraftHorse/issues)

---

For end-user installation and usage, see [README.md](README.md).
For contributors and maintainers, see [DEVELOPMENT.md](DEVELOPMENT.md)
and [CLAUDE.md](CLAUDE.md); per-change rationale is in
[PATCHES.md](PATCHES.md).
