## DraftHorse

A hardened fork of go-mapi: routes Windows "Send to → Mail recipient" and
scanner Scan-to-Email (Simple MAPI) calls to Gmail as **drafts**. Nothing is
ever sent automatically. Wails v2 + Svelte 5 + WebView2 + C++17 MAPI DLL,
both bitnesses.

### Install

1. Download `DraftHorse-setup.exe` from the assets below, or use the stable URL:
   `https://github.com/ArricksPropane/DraftHorse/releases/latest/download/DraftHorse-setup.exe`
2. Run the installer as administrator. Elevation is required because it
   registers the machine-wide MAPI handler under `HKLM\SOFTWARE\Clients\Mail`.
3. First launch: sign in with your Google account — the app opens your
   default browser for OAuth consent (scope: create drafts only).

Upgrading from any DraftHorse/DraftHorse 3.x install: just run the new
installer — it upgrades in place.

### Updates are manual

DraftHorse shows an in-app banner when a newer release is published but
never replaces its own binary. Download and run the new installer yourself
(or push it via Intune). This is an explicit design decision — the upstream
silent auto-updater was removed in this fork.

### System requirements

- Windows 10 (22H2) or Windows 11
- Microsoft Edge WebView2 Evergreen Runtime — auto-bootstrapped by the installer if missing
- Gmail or Google Workspace account

### Release artifacts

- `DraftHorse-setup.exe` — single-file installer (bundles WebView2 bootstrapper, both MAPI DLLs, and the app)
- `DraftHorse.exe`, `DraftHorse-x64.dll`, `DraftHorse-x86.dll` — individual binaries (for verification; the installer is the supported install path)
- `SHA256SUMS.txt` — checksum manifest for everything above

### License

LGPL-3.0 — see [LICENSE](https://github.com/ArricksPropane/DraftHorse/blob/main/LICENSE).
Fork of [marcfargas/go-mapi](https://github.com/marcfargas/go-mapi).

---

Full docs: [README](https://github.com/ArricksPropane/DraftHorse#readme) ·
[IT/Enterprise deployment](https://github.com/ArricksPropane/DraftHorse/blob/main/ENTERPRISE.md) ·
[fork rationale](https://github.com/ArricksPropane/DraftHorse/blob/main/PATCHES.md)
