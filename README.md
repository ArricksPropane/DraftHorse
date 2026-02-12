# go-mapi

**The MAPI-to-Gmail bridge you always wanted.**

## Overview

[MAPI](https://en.wikipedia.org/wiki/MAPI) (Messaging Application Programming Interface) is a Microsoft Windows API that allows programs to become email-aware. It enables the "Send by Email" feature found in many Windows applications - from right-clicking a file in Explorer to printing to PDF and emailing it.

**Gmail and Google Workspace have no native MAPI support.** Despite Google Workspace being a major enterprise email solution, there has never been an official MAPI-to-Gmail bridge.

A few third-party tools have filled this gap over the years. The most notable, [Affixa](https://help.affixa.com/article/100-sunsetting-and-retirement-of-affixa), recently announced its shutdown - leaving Google Workspace users without a way to use "Send by Email" functionality.

**go-mapi** is the open-source replacement.

## Status

**Beta** — Core flow works end-to-end. Usable for early adopters.

| Component | Status |
|-----------|--------|
| MAPI interception (ANSI + Unicode) | ✅ Working |
| Native messaging bridge | ✅ Working |
| Browser extension (popup + notifications) | ✅ Working |
| Gmail draft creation (with attachments) | ✅ Working |
| UTF-8 / codepage encoding | ✅ Working |
| Version tracking across components | ✅ Working |
| PowerShell installer | ✅ Working |
| OAuth consent (external users) | 📋 Planned |
| MSI installer | 📋 Planned |

See [ROADMAP.md](ROADMAP.md) for planned work.

## Architecture

go-mapi uses a three-component architecture optimized for enterprise deployment:

```
┌─────────────────────┐                      ┌─────────────────────┐
│   Windows App       │                      │  Browser Extension  │
│   (Explorer, etc.)  │                      │  (Chrome / Edge)    │
│         │           │                      │         │           │
│    MAPISendMail()   │                      │   React Popup UI    │
│         ▼           │                      │         │           │
│  ┌─────────────┐    │   %TEMP%\go-mapi\    │   ┌───────────┐     │
│  │ go-mapi.dll │────┼──────────────────────┤   │ Gmail API │     │
│  └─────────────┘    │         ▲            │   └───────────┘     │
└─────────────────────┘         │            └─────────────────────┘
                                │
                     ┌──────────┴──────────┐
                     │  Native Messaging   │
                     │  Host (Go binary)   │
                     │  Watches folder     │
                     └─────────────────────┘
```

### Components

| Component | Technology | Purpose |
|-----------|------------|---------|
| Interceptor DLL | C++ (MinGW) | Captures MAPI calls, writes JSON to `%TEMP%\go-mapi\` |
| Native Host | Go | Watches folder, bridges to browser via Native Messaging |
| Browser Extension | React + TypeScript | UI + Gmail API (Chrome & Edge) |

### Why This Architecture?

- **Enterprise-friendly**: DLL and host installed once with admin rights; extension updates independently
- **Simple OAuth**: Extension uses Chrome Identity API — no separate auth flow needed
- **Cross-browser**: Same extension works in Chrome and Edge
- **Debuggable**: JSON files on disk make the IPC trivially inspectable

## Quick Start

### Prerequisites

- Windows 10/11
- Chrome or Edge browser
- Gmail or Google Workspace account

### Installation

1. **Install the extension** in Chrome or Edge:
   - Chrome Web Store: [Link coming soon]
   - Edge Add-ons: [Link coming soon]
   - Or load unpacked from a [release zip](https://github.com/marcfargas/go-mapi/releases)

2. **Copy your extension ID** from `chrome://extensions` (enable Developer mode)

3. **Run the installer** (admin PowerShell):
   ```powershell
   irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/install.ps1 | iex
   ```
   Downloads the latest release, installs binaries to `C:\Program Files\go-mapi\`,
   registers the MAPI handler, and sets up native messaging for Chrome and Edge.
   It will prompt for your extension ID.

### Usage

1. Right-click any file in Windows Explorer → "Send to" → "Mail recipient"
2. The email appears in the go-mapi extension popup
3. Click "Save as Draft" or "Send Now"

### Advanced Install Options

```powershell
# Pin a specific version
.\install.ps1 -ExtensionId "abc..." -Version "v0.1.0"

# Custom install directory
.\install.ps1 -ExtensionId "abc..." -InstallDir "D:\go-mapi"

# Developer: install from local build instead of GitHub
.\install.ps1 -ExtensionId "abc..." -Local
```

### Uninstall

```powershell
# One-liner (admin PowerShell)
irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/uninstall.ps1 | iex

# Registry-only (keep files)
.\uninstall.ps1 -KeepFiles
```

The uninstaller removes all registry entries and restores your previous default mail client.

## Enterprise Deployment

For managed environments:

- **Binaries**: Deploy `go-mapi.dll` and `go-mapi-host.exe` to `C:\Program Files\go-mapi\` via MSI or SCCM
- **Registry**: The installer script creates the required entries; export them for GPO deployment
- **Extension**: Force-install via Chrome/Edge enterprise policy ([`ExtensionInstallForcelist`](https://chromeenterprise.google/policies/#ExtensionInstallForcelist))
- **OAuth**: Create a GCP project, enable Gmail API, and configure an OAuth 2.0 client ID (Chrome Extension type)

## Why "go-mapi"?

The name is a nod to "Go(ogle)" and "let's go". The project started as a pragmatic solution to the [Affixa shutdown](https://help.affixa.com/article/100-sunsetting-and-retirement-of-affixa).

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:
- Setting up the development environment
- Building components
- Submitting pull requests

## License

License TBD.

> **Note:** Until a license is explicitly added to this project, it shall not be considered license-free, copyleft, or public domain. The absence of a license means all rights are reserved - the code is provided for viewing only.

## References

- [Simple MAPI Documentation](https://learn.microsoft.com/en-us/previous-versions/dd296734(v=vs.85))
- [Chrome Native Messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging)
- [Gmail API](https://developers.google.com/gmail/api)
- [Affixa Sunset Announcement](https://help.affixa.com/article/100-sunsetting-and-retirement-of-affixa)
