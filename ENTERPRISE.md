# DraftHorse for IT Administrators

Audience: Windows / IT admins deploying DraftHorse at scale. This fleet
deploys via **ScreenConnect** (decided 2026-08-28 — no Intune/MDM); Intune
and GPO notes are kept where the mechanism is generic.

> DraftHorse is a hardened fork of upstream go-mapi. **As of 4.0, every
> installed identifier is `DraftHorse`** — binaries, registry keys, data
> paths, detection rules. (Through 3.x, identifiers were `go-mapi`; the 4.0
> installer migrates machine scope and the app migrates per-user state
> automatically.) Deployment tooling should key off the `DraftHorse`
> identifiers throughout this document; if a fleet report ever shows
> `go-mapi` keys or folders, that machine missed the 4.0 migration —
> run the diagnostics script's stale-leftovers section.

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
  anchored at `%ProgramFiles%\DraftHorse\` and `%ProgramFiles(x86)%\DraftHorse\`.

## Install modes

### All Users (the only mode)

The installer is **All Users only** and requires UAC elevation. DraftHorse
registers as the machine-wide MAPI handler, which is inherently
machine-wide.

File layout:

- `%ProgramFiles%\DraftHorse\` — app (`DraftHorse.exe`), x64 MAPI DLL,
  uninstaller, diagnostics scripts
- `%ProgramFiles(x86)%\DraftHorse\` — x86 MAPI DLL (32-bit scanner software
  and other 32-bit MAPI callers load this one)

Registry footprint (machine):

- `HKLM\SOFTWARE\Clients\Mail\DraftHorse` — MAPI registration. `DLLPath` is a
  single `REG_EXPAND_SZ` value, `%ProgramFiles%\DraftHorse\DraftHorse.dll`;
  Windows expands it per caller, so 32-bit and 64-bit MAPI callers each
  load the matching DLL from ONE registration. (`SOFTWARE\Clients` is a
  WOW64 *shared* key — there is no separate WOW6432Node registration, and
  tooling should not look for one.)
- `HKLM\SOFTWARE\Clients\Mail` `(Default)` = `DraftHorse` (default MAPI client
  pointer; previous value backed up and restored on uninstall)
- `HKLM\SOFTWARE\Classes\DraftHorse.mailto` — `mailto:` ProgID
- `HKLM\SOFTWARE\Clients\Mail\DraftHorse\Capabilities` +
  `HKLM\SOFTWARE\RegisteredApplications\DraftHorse` — Windows Default Apps
  registration (the app appears as **DraftHorse** in Settings > Default
  apps)
- `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\DraftHorse` —
  Add/Remove Programs (DisplayName: DraftHorse)
- `HKLM\Software\Microsoft\Windows\CurrentVersion\Run` value `DraftHorse` —
  logon autostart (all users). The queue watcher and the default-mail
  guard only work while the tray app runs; the installer also relaunches
  the app immediately after interactive installs (SYSTEM-context pushes
  rely on the next logon).

Registry footprint (per user, written by the app at runtime):

- `HKCU\Software\Clients\Mail\DraftHorse` (+ the HKCU `(Default)` pointer) —
  written **only** if the app's default-mail guard detects that another
  product has stolen the machine-wide MAPI default (see *Default-mail
  self-healing* below). The uninstaller removes the uninstalling user's
  copy.

The installer backs up the previous default mail client name to
`%ProgramData%\DraftHorse\uninst\previous-mail-client.json` and restores it on
uninstall. The uninstaller also removes the legacy upstream
`DraftHorse Auto Update` Scheduled Task if a pre-fork install left one behind.

## Default-mail self-healing (fleet-relevant behavior)

Every hour (and at startup), the running app checks that
`Clients\Mail (Default)` still resolves to `DraftHorse`. If another installer
(commonly Outlook) has claimed the default, the app re-claims it for the
**current user** by writing an unelevated per-user override at
`HKCU\Software\Clients\Mail`. This keeps scanner scan-to-email working
without IT intervention.

Plan for this on machines where users legitimately run another Simple MAPI
mail client: with DraftHorse installed and running, DraftHorse **will** win
the MAPI default for that user within the hour. If a host needs a different
default client, don't deploy DraftHorse to it (or stop the tray app).
Repairs are logged to `%APPDATA%\DraftHorse\app.log`.

## mailto: default (Settings > Default apps)

DraftHorse registers as a candidate `mailto:` handler. Windows does **not**
allow apps to set themselves as the mailto default programmatically
(`UserChoice` is hash-protected and enforced by the UCPD driver), so:

- **Per machine/user, interactively:** the user picks DraftHorse once in
  Settings > Default apps (the app shows a one-click deep link when it
  detects it isn't the default).
- **Fleet-wide, zero-touch:** deploy the default-associations profile —
  see `docs/mailto-default-associations.xml` — via domain GPO Device
  Configuration (or DISM `/Import-DefaultAppAssociations` for imaged
  hosts).

A mailto click runs `DraftHorse.exe --mailto "%1"`, which opens Gmail web
compose prefilled from the link and exits. No draft API call, no stored
credentials involved.

## Release signing (admin machine — the key never leaves it)

Releases are signed OFF-CI: CI builds, tests, and publishes unsigned assets
for a tag; the admin machine holding the Arrick's Propane certificate then
runs

    scripts\signing\sign-release.ps1 -Tag vX.Y.Z

which downloads the assets, verifies them against CI's SHA256SUMS.txt (you
sign exactly what CI tested), signs the binaries, repacks and signs the
installer, regenerates the manifest, and replaces the release assets (with a
confirmation prompt). One-time machine setup is in the script header: NSIS +
Windows SDK + repo clone + the PFX imported NON-exportable. If CI is ever
compromised, the worst case is an unsigned build that fails validation on
fleet machines — nothing fleet-trusted can be produced outside that one
machine. Archive the signed installer + SHA256SUMS with each release record.

## Per-machine install runbook (ScreenConnect)

Run in an elevated session on each machine, in this order:

1. **Pre-4.0 machines only**: `scripts\cleanup-legacy-gomapi.ps1` (sweeps
   go-mapi leftovers; `-ReportOnly` first if unsure).
2. **Trust the signing certificate** (once per machine):
   `scripts\signing\trust-signing-cert.ps1 -CertPath DraftHorse-signing.cer`
   — installs the Arrick's Propane self-signed cert into LocalMachine Root +
   TrustedPublisher so the signed binaries validate and UAC names the
   publisher. Self-signed = no SmartScreen reputation; if the installer was
   downloaded with mark-of-the-web, expect one "More info → Run anyway".
3. **Install**: `DraftHorse-setup.exe /S`.
4. **Sign in** (per the account model): app OAuth + dedicated browser
   profile(s) into the location account; on machines upgraded from pre-3.8,
   one sign-out/in grants the signature scope.
5. **mailto default**: no MDM policy push — set once via Settings > Default
   apps (the app deep-links there when it detects it is not the default),
   or domain GPO with `docs/mailto-default-associations.xml` if AD applies.
6. **Outlook check**: see the next section — exclude or policy-disable
   Outlook where present, or defaults revert at logon.

## Outlook coexistence — known conflict

Observed 2026-08-28 on a test machine: with Outlook installed (even unused),
a restart reverted mail defaults — mailto: switched back to Outlook and
ScanSnap stopped listing DraftHorse as an available client. Uninstalling
Outlook resolved both. Outlook re-asserts default-mail ownership at logon;
DraftHorse's guard re-heals the Simple MAPI default while running, but the
mailto UserChoice is hash-protected and cannot be healed programmatically.

Fleet guidance, pick one per machine:
- **Preferred**: exclude Outlook from the Microsoft 365 Apps deployment
  where it is unused (Office Deployment Tool configuration:
  `<ExcludeApp ID="Outlook" />`).
- Where Outlook must remain: disable its default-client check via the
  Office ADMX policy "Make Outlook the default program for E-mail,
  Contacts, and Calendar" (set Disabled), and re-apply the mailto
  associations XML.

If defaults still flip, Event Viewer > Applications and Services Logs >
Microsoft > Windows > Shell-Core > AppDefaults names which default was
reset and why; run the diagnostics script for the registry side.

## Two accounts per machine (4.0)

DraftHorse holds up to **two** signed-in Gmail accounts. Exactly one is
**active** — every scan drafts to it. The user switches from the tray
("Draft to: …" radio rows) or the in-app switcher; nothing prompts during a
scan. Tokens are stored as two Credential Manager entries under the
`DraftHorse` service (`oauth-tokens`, `oauth-tokens-2`).

Runbook: each signed-in account needs its OWN dedicated browser profile
signed in once by IT — `browser-profile` for account 1, `browser-profile-2`
for account 2 (see below for why they must never share one). Adding a second
account = "Sign in" on its slot in the app + one sign-in in the second
profile window when the first draft opens.

## Opening drafts: the dedicated browser profile

After a draft is created, DraftHorse opens it in **its own isolated browser
window** — Edge (or Chrome) launched with a private profile directory at
`%LOCALAPPDATA%\DraftHorse\browser-profile` — rather than the user's default
browser. This is deliberate for the delegated-mailbox fleet model: the app
is signed into the *location* account, while staff browsers are signed in
as the staff member (the location mailbox is only reachable there through
Gmail's delegation hop, which has no URL). A dedicated profile makes the
draft open in the right account every time, independent of the user's
daily browser.

**Setup (once per user per machine):** the first draft opened in the
dedicated window shows Google's sign-in prefilled with the location
account; IT completes it once. The session persists in the profile.
Nothing is bundled — Edge ships with Windows 11 and Microsoft patches it.

The tray checkbox **"Use DraftHorse's own browser window"** (default on)
falls back to the system default browser when unticked, or automatically
if no Edge/Chrome is found. The profile directory is removed by the
uninstaller for the uninstalling user (it holds the location account's
session cookies — see *Multi-user / RDS hosts* for other users).

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
DraftHorse-setup.exe /S /D=C:\Program Files\DraftHorse
```

The installer is idempotent — running it over an existing install upgrades
in place (the previous-mail-client backup is preserved across upgrades).
The legacy `/AUTOUPDATE=1` flag is accepted and ignored.

### Exit codes

NSIS conventions: `0` success, `1` user/interactive cancel, `2` runtime
failure (including `DraftHorse.exe` still running after the 10-second
graceful-close poll on silent installs). Treat any non-zero exit as
failure.

### Detection rule (any RMM / inventory tooling)

Key: `HKLM\SOFTWARE\Clients\Mail\DraftHorse`, value `DLLPath` exists.
(The Uninstall key also works; the Clients\Mail key is the one that
actually makes the product functional, so it is the better health signal.)

## Updates (notify-only)

The app checks GitHub Releases daily and shows a banner when a newer
version exists. Nothing is downloaded or installed automatically — the
user (or your deployment pipeline) runs the new `DraftHorse-setup.exe`.

For managed fleets, the recommended pattern is to treat updates like any
other Win32 app revision: run the new installer with `/S` (ScreenConnect
command or session).
In-place upgrade over a running app is supported (the installer closes
the tray app gracefully, with a silent-mode retry window).

To disable even the update *check* per user, untick "Check for updates"
in the tray menu (persisted per user in `%APPDATA%\DraftHorse\settings.json`).

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
Gmail's own API; queued messages transit `%LOCALAPPDATA%\DraftHorse\queue\`
on the local machine only, and attachments are constrained to that
directory by validation.

Credential storage: per-user OAuth tokens in Windows Credential Manager
(target `DraftHorse:oauth-tokens`, DPAPI-scoped). OAuth scopes:
`gmail.compose` (create drafts), `gmail.settings.basic` (reads the
account's signature so drafts carry it — the only settings call made),
and basic profile. The app cannot send, delete, or read mail.

> Upgrading from 3.7.x or earlier: tokens granted before the signature
> scope produce a one-time "signature fetch forbidden" log line and
> unsigned drafts until the user signs out and back in once (Sign out in
> the app window, then sign in). Draft creation itself is unaffected.

## OAuth loopback firewall rule

Sign-in opens a short-lived loopback listener on `127.0.0.1` (ephemeral
port). The installer creates a program-scoped inbound allow rule named
`DraftHorse OAuth loopback` for `%ProgramFiles%\DraftHorse\DraftHorse.exe` to
suppress the first-bind consent prompt. If GPO blocks the installer's
`netsh` write, pre-create an identical rule via policy:

```powershell
New-NetFirewallRule `
  -DisplayName "DraftHorse OAuth loopback" `
  -Direction Inbound `
  -Action Allow `
  -Program "$env:ProgramFiles\DraftHorse\DraftHorse.exe" `
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

- `%APPDATA%\DraftHorse\` (settings, log)
- `%LOCALAPPDATA%\DraftHorse\browser-profile\` (dedicated browser profile —
  holds the location account's Google session; clear it for non-uninstalling
  users if policy requires)
- Credential Manager target `DraftHorse:oauth-tokens` (DPAPI-scoped)
- `HKCU\Software\Clients\Mail\DraftHorse` for users other than the
  uninstalling one, if the default-mail guard ever self-healed in their
  session

The Credential Manager residue is a DPAPI-encrypted refresh token with no
privileged caller once the binary is gone; the HKCU registry residue is a
dangling pointer Windows ignores. If policy requires cleanup anyway, run
per user at logon:

```powershell
Remove-Item -Recurse -Force "$env:APPDATA\DraftHorse" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\DraftHorse\browser-profile" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "HKCU:\Software\Clients\Mail\DraftHorse" -ErrorAction SilentlyContinue
cmdkey /list:DraftHorse:oauth-tokens 2>$null | Out-Null
if ($LASTEXITCODE -eq 0) { cmdkey /delete:DraftHorse:oauth-tokens | Out-Null }
```

### No MSI

The installer is an NSIS EXE. There is no MSI wrapper. Use Win32
app deployment, or a GP startup script for GPO-only environments.

## Support

[GitHub Issues](https://github.com/egkrateia247/DraftHorse/issues)

---

For end-user installation and usage, see [README.md](README.md).
For contributors and maintainers, see [DEVELOPMENT.md](DEVELOPMENT.md)
and [CLAUDE.md](CLAUDE.md); per-change rationale is in
[PATCHES.md](PATCHES.md).
