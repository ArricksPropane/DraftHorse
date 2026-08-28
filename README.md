# DraftHorse

> DraftHorse is a hardened fork of [go-mapi](https://github.com/marcfargas/go-mapi)
> by [Marc Fargas](https://github.com/marcfargas), maintained by
> [Arrick's Propane](https://arrickspropane.com) for its scanner
> scan-to-email fleet.
>
> Right-click any file in Windows Explorer, click "Send to → Mail recipient",
> and the email appears as a **draft** in Gmail. Nothing is ever sent
> automatically — that's the whole point of the name.

[![Watch the upstream demo](https://img.youtube.com/vi/gxTpMXVdP40/maxresdefault.jpg)](https://youtu.be/gxTpMXVdP40)

## What it does

DraftHorse connects Windows' Simple MAPI plumbing — the machinery behind
"Send to → Mail recipient" and scanner "Scan to Email" buttons — to your
Gmail account. Any Windows app that sends mail that way routes to Gmail
instead of Outlook, as a draft you review before sending.

- Works with any Windows app that uses Simple MAPI: File Explorer, Office,
  legacy line-of-business software, and **document scanners** (ScanSnap,
  Epson, and other MFP "Scan to Email" software — including apps that call
  `MAPISendDocuments`, which many scanner utilities use)
- Both 32-bit and 64-bit callers are supported — 32-bit scanner software
  and 64-bit Explorer/Office each load a matching-bitness handler
- Drafts land in Gmail — review and send normally; **nothing is ever sent
  automatically**. The only Gmail API call the app makes is
  "create draft".
- Manual or automatic draft creation (your choice);   open directly in Gmail's compose window in your browser
- Handles `mailto:` links too — selectable in Windows Settings > Default
  apps, a click opens Gmail web compose prefilled from the link
- Keeps itself the default mail handler: if another install (say, Outlook)
  steals the Windows mail-client default, DraftHorse detects it within the
  hour and takes it back, so scan-to-email keeps working
- Your emails go directly to Gmail's servers — nowhere else. No
  third-party servers, no telemetry, no analytics.

## Before you install

You'll need a Gmail or Google Workspace account. The first time you launch
DraftHorse it opens a Google sign-in page in your browser so it can create
drafts on your behalf (`gmail.compose`) and read your Gmail signature so
drafts carry it (`gmail.settings.basic`). The app cannot send, delete, or
read your mail.

## Install

1. Download `DraftHorse-setup.exe` from the
   [latest release](https://github.com/ArricksPropane/DraftHorse/releases/latest)
2. Run it. Windows will ask for permission (UAC dialog) — click **Yes**.
   DraftHorse needs this to register itself as the Windows mail handler.
3. Sign in with your Gmail or Google Workspace account when prompted
4. Done

DraftHorse runs in the Windows notification area (the icons next to your
clock). Click its icon to open the window or change settings. To also make
it your `mailto:` link handler, pick DraftHorse in **Settings > Default
apps** — the app shows a one-click shortcut to that page when needed
(Windows does not allow apps to set this themselves).

> The installer file is named `DraftHorse-setup.exe`; the binaries it
> installs and every registry identifier intentionally remain `DraftHorse*`
> for upgrade compatibility — installs of earlier DraftHorse 3.x versions
> upgrade in place.

## How to use it

Trigger "Send to Mail recipient" (or your scanner's Scan-to-Email button)
as you normally would.

**Manual mode** (default): DraftHorse shows each email and waits for you
to click "Create draft". Nothing reaches Gmail until you say so.

**Auto mode**: DraftHorse creates the draft immediately and notifies you.
Failures that can't succeed on retry (for example an attachment over
Gmail's size limit — the app enforces an 18 MB budget so Gmail's encoded
25 MB cap is never hit) stop retrying after a few attempts and stay in
the queue for you to handle, instead of erroring forever.

## Updates

DraftHorse tells you when a new version is available. Updates are **never
installed automatically** — this fork removed the upstream silent
auto-updater entirely. Click the banner, download the installer, and run
it yourself.

## What this fork changes

The fork exists so a small business can own its scan-to-email path outright
(it replaces Affixa, which retires in January 2027). On top of upstream
DraftHorse v3.0.0:

- **Scanner-critical fixes**: `MAPISendDocuments` implemented (upstream
  stubbed it while reporting success — silent mail loss for most scanner
  software); attachment filenames sanitized; loader-lock safety in the
  in-process DLL
- **Correct dual-bitness registration**: one `REG_EXPAND_SZ` DLLPath that
  routes 32-bit and 64-bit MAPI callers to matching DLLs (the WOW64
  shared-key semantics this depends on are verified end-to-end in CI by
  real MAPI probe callers on every change)
- **Default-mail self-healing**: hourly guard re-claims the Windows mail
  client default if another installer takes it
- **mailto: handler** + Windows Default Apps registration, with an Intune
  associations profile for fleet-wide defaults
- **Open drafts in browser**: finished drafts pop open in Gmail compose
  (tray-toggleable)
- **Security hardening**: silent auto-updater removed; `gmail.send` scope
  dropped; queue-path containment; mail-header injection rejection;
  attachment size budgets; caller-input caps and crash guards in the DLL;
  release credentials can't be overridden by environment variables
- **A real test gate**: 33 installer smoke tests on every change,
  including probe executables of both bitnesses that exercise the actual
  Windows MAPI stub

Per-change rationale lives in [PATCHES.md](PATCHES.md); contributor and
architecture notes in [CLAUDE.md](CLAUDE.md).

## License

LGPL-3.0-or-later — free and open source, anyone can inspect the code.
See [LICENSE](LICENSE). Upstream copyright remains with Marc Fargas and
contributors.

---

For IT departments deploying at scale (silent install, Intune, group
policy), see [ENTERPRISE.md](ENTERPRISE.md) — note the fork deploys via
`DraftHorse-setup.exe /S` with detection on
`HKLM\SOFTWARE\Clients\Mail\DraftHorse`.

For contributors and maintainers, see [DEVELOPMENT.md](DEVELOPMENT.md).
