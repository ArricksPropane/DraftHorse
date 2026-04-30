# go-mapi

> Right-click any file in Windows Explorer, click "Send to → Mail recipient",
> and the email appears ready to send in Gmail.
>
> No configuration. No subscription fees. Completely open source.

## What it does

Any Windows app that has a "Send to Mail recipient" option — File Explorer,
Word, Excel, legacy line-of-business software — will route that action to Gmail
instead of Outlook. go-mapi sits quietly in the background and creates a draft
in your Gmail inbox, ready for you to review and send.

- Works with any Windows app that uses "Send to Mail recipient"
- Drafts land in Gmail — review and send normally, nothing is sent automatically
- Manual or automatic draft creation (your choice)
- Privacy-first: no telemetry, email content never leaves the Gmail API

## Install

1. Download `go-mapi-setup.exe` from the [latest release](https://github.com/marcfargas/go-mapi/releases/latest)
2. Run it (administrator prompt required — go-mapi registers itself as the Windows mail handler)
3. Sign in with your Gmail or Google Workspace account when prompted
4. Done

go-mapi runs quietly in the system tray. Nothing else changes — keep using
your apps as normal.

> **Upgrading from go-mapi v2.x?**
> Uninstall v2.x first via **Settings → Apps → Installed apps**, then install
> the new version. Running both side-by-side is not supported.

## How to use it

Once installed, trigger "Send to Mail recipient" in any Windows app as you
normally would. go-mapi intercepts the request and shows you the draft in its
tray window before anything goes to Gmail.

**Manual mode** (default): go-mapi shows you each email and waits for you to
click "Create draft". Nothing reaches Gmail until you say so.

**Auto mode**: go-mapi creates the draft immediately and tells you it's ready
in your inbox. Switch between modes in the tray window.

## Updates

go-mapi checks for updates automatically and lets you know when one is ready.
You stay in control — updates are never installed without your say-so. When a
new version is available, click the banner to open the download page and run
the new installer yourself.

## Known issues

- **Saving preferences**: some settings beyond the Manual/Auto mode toggle may
  not persist between sessions. The Mode toggle works correctly; other
  settings are not yet user-exposed. This will be fixed in an upcoming release.

## License

LGPL-3.0-or-later. See [LICENSE](LICENSE).

---

For IT departments and admins deploying go-mapi at scale (RDS, Citrix, silent
install, group policy), see [ENTERPRISE.md](ENTERPRISE.md).

For contributors and maintainers, see [DEVELOPMENT.md](DEVELOPMENT.md).
