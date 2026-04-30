# go-mapi for IT Administrators

Audience: Windows / IT admins deploying go-mapi at scale — RDS, Citrix,
managed desktops, group policy.

## At a glance

- ~40–50 MB RAM per session — viable on RDS / Citrix multi-user hosts
- Machine-wide (All Users) install only — no per-user install path
- Unattended (silent) install supported via `/S` flag
- Optional automatic updates via a Windows Scheduled Task (SYSTEM context)
- SHA-256 checksums published with every release
- LGPL-3.0-or-later (FOSS — no per-seat licensing)
- No telemetry; the only outbound traffic is the Gmail API for the signed-in user

## Install modes

### All Users (the only mode)

`go-mapi-setup.exe` is **All Users only**. go-mapi registers itself as the
machine-wide MAPI handler under `HKLM\SOFTWARE\Clients\Mail\go-mapi`, which is
inherently machine-wide. There is no per-user install path.

The installer requires UAC elevation. Run as an administrator or via a
managed-deployment context that elevates.

Default install directory: `%ProgramFiles%\go-mapi` (64-bit).
The 32-bit MAPI DLL is placed separately at `%ProgramFiles(x86)%\go-mapi`
so that both 32-bit and 64-bit MAPI callers are routed correctly.

Registry footprint:
- `HKLM\SOFTWARE\Clients\Mail\go-mapi` (native/64-bit MAPI registration)
- `HKLM\SOFTWARE\WOW6432Node\Clients\Mail\go-mapi` (32-bit MAPI registration)
- `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\go-mapi` (Add/Remove Programs)

The installer backs up the previous default mail client name to
`%ProgramData%\go-mapi\uninst\previous-mail-client.json` and restores it on
uninstall.

## Silent install

Silent install with all defaults (no automatic updates):

```
go-mapi-setup.exe /S
```

Silent install to a custom path:

```
go-mapi-setup.exe /S /D=C:\Program Files\go-mapi
```

> Note: `/D` must be the last parameter and must not be quoted even if the
> path contains spaces (NSIS restriction).

Enable the automatic-update Scheduled Task at install time:

```
go-mapi-setup.exe /S /AUTOUPDATE=1
```

The installer is idempotent — running it over an existing install upgrades
in place (the previous mail client backup is preserved across upgrades).

Exit codes: NSIS silent installs exit with `0` on success and a non-zero
code on abort. Log output goes to the Windows installer log rather than a
file; add `/LOG=C:\path\to\install.log` if your deployment tooling needs a
log file.

## Automatic updates

When installed with `/AUTOUPDATE=1` (or with the "Enable automatic updates"
checkbox ticked during an interactive install), the installer registers a
Windows Scheduled Task:

| Property | Value |
|---|---|
| Task name | `go-mapi Auto Update` |
| Path | `\go-mapi Auto Update` (root of Task Scheduler) |
| Run as | `SYSTEM` (no per-user credential, no logon required) |
| Schedule | Daily 03:00 with ±30 minute random delay |
| Also runs | At system startup (5 minute delay) |
| Network | `RunOnlyIfNetworkAvailable=true` (skips offline runs) |
| Catch-up | `StartWhenAvailable=true` (runs after wake/reboot if missed) |
| Concurrency | `MultipleInstancesPolicy=IgnoreNew` (no overlapping runs) |
| Time limit | 12 hours per run (`ExecutionTimeLimit=PT12H`) |

The task fires `go-mapi.exe --update-check-silent`, which fetches the latest
release, verifies the SHA-256 digest before writing anything, and atomically
replaces the binary. The running interactive go-mapi instance keeps working
until the next launch; the task does **not** forcibly restart it.

Update logs are written to `%ProgramData%\go-mapi\updates\update.log`
(admin-readable; no PII, no message content).

### Managing the Scheduled Task

Disable (e.g. during maintenance windows):

```
schtasks /change /tn "go-mapi Auto Update" /disable
```

Re-enable:

```
schtasks /change /tn "go-mapi Auto Update" /enable
```

Force an immediate update check:

```
schtasks /run /tn "go-mapi Auto Update"
```

Query last run and status:

```
schtasks /query /tn "go-mapi Auto Update" /v /fo LIST
```

The task is removed automatically by the go-mapi uninstaller.

To add automatic updates to an existing notify-only install, re-run the
installer over the existing install with `/AUTOUPDATE=1` — idempotent.

To disable automatic updates on a managed host, either install with
`/AUTOUPDATE=0` (the default) or delete the task after install:

```
schtasks /delete /tn "go-mapi Auto Update" /f
```

## Verify download integrity

Every release publishes `SHA256SUMS.txt` alongside the installer:

```
https://github.com/marcfargas/go-mapi/releases/latest/download/SHA256SUMS.txt
```

Format follows the `sha256sum` convention (`<lowercase-hex>  <filename>`).
The automatic updater verifies downloads before applying them. For manual
verification before deployment:

```powershell
$sums    = (Invoke-WebRequest 'https://github.com/marcfargas/go-mapi/releases/latest/download/SHA256SUMS.txt').Content
$actual  = (Get-FileHash -Algorithm SHA256 .\go-mapi-setup.exe).Hash.ToLower()
$sums   # inspect expected hash for go-mapi-setup.exe
$actual  # compare
```

If the release carries an Authenticode signature (SignPath.io for OSS), verify
both the SHA-256 digest and the signature for defense-in-depth.

## Mass deployment

go-mapi uses a standard NSIS installer with a silent mode and a documented
registry footprint, which makes it compatible with most Windows software
distribution tooling:

- **Intune / SCCM**: deploy `go-mapi-setup.exe /S` as a Win32 app. Detection
  rule: key `HKLM\Software\Microsoft\Windows\CurrentVersion\Uninstall\go-mapi`
  exists.
- **Group Policy (Software Installation)**: NSIS produces an EXE, not an MSI.
  Use a GP startup script or a third-party EXE-to-MSI wrapper if your policy
  requires MSI format.
- **Chocolatey / Scoop**: no official package yet. Use the GitHub Releases URL
  directly in your internal feed.

## Privacy posture

go-mapi makes network calls only to:

- `https://github.com/marcfargas/go-mapi/releases/latest/download/...`
  (update check and asset download — only when automatic updates are enabled
  or when the user clicks "download update")
- Google OAuth endpoints (sign-in and token refresh)
- Gmail API (`https://gmail.googleapis.com/`) — only when the user is signed in

No telemetry. No content retention. Email content is never stored outside of
Gmail's own API. The silent-update log at `%ProgramData%\go-mapi\updates\update.log`
records version transitions and download success/failure only — no message
bodies, no recipient data, no credential material.

Credential storage: each user's OAuth tokens are stored in the Windows
Credential Manager (DPAPI-scoped, per user). go-mapi never stores tokens in
shared locations.

## Limitations

### Multi-user / RDS hosts

The uninstaller scrubs the uninstalling admin's profile and all machine-wide
locations (`HKLM\SOFTWARE\Clients\Mail\go-mapi`, `%ProgramFiles%\go-mapi\`,
`%ProgramData%\go-mapi\`, and the Scheduled Task). It does **not** enumerate
every user profile on the host.

Residue that persists per user after uninstall:
- `%APPDATA%\go-mapi\` (settings, log) — per user, not touched by the uninstaller
- Windows Credential Manager target `go-mapi:oauth-tokens` — per user (DPAPI-scoped)

To clean up a specific user's residue, that user (or an admin impersonating
their session) must:

```
rmdir /s /q "%APPDATA%\go-mapi"
cmdkey /delete:go-mapi:oauth-tokens
```

### RDS firewall loopback

go-mapi's OAuth sign-in opens a short-lived loopback listener on `127.0.0.1`
with an ephemeral port. The installer creates a Windows Firewall inbound rule
(`go-mapi OAuth loopback`, scoped to the binary) to suppress the first-bind
consent prompt. On RDS hosts, that prompt appears on the server console (not in
the user's RDP session) if the rule is absent or blocked by group policy. If
your GPO blocks `netsh advfirewall` writes, pre-create the rule via policy
before deploying go-mapi.

### No MSI

The installer is an NSIS EXE. There is no MSI wrapper. This is a known
limitation for GPO-based Software Installation policies.

## Support

[GitHub Issues](https://github.com/marcfargas/go-mapi/issues)
