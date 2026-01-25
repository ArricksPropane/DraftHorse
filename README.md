# go-mapi

**The MAPI-to-Gmail bridge you always wanted.**

## Overview

[MAPI](https://en.wikipedia.org/wiki/MAPI) (Messaging Application Programming Interface) is a Microsoft Windows API that allows programs to become email-aware. It enables the "Send by Email" feature found in many Windows applications - from right-clicking a file in Explorer to printing to PDF and emailing it.

**Gmail and Google Workspace have no native MAPI support.** Despite Google Workspace being a major enterprise email solution, there has never been an official MAPI-to-Gmail bridge.

A few third-party tools have filled this gap over the years. The most notable, [Affixa](https://help.affixa.com/article/100-sunsetting-and-retirement-of-affixa), recently announced its shutdown - leaving Google Workspace users without a way to use "Send by Email" functionality.

**go-mapi** is the open-source replacement.

## Status

**Alpha** - Core functionality works, but not yet production-ready.

- MAPI interception: Working
- Native messaging bridge: Working
- Browser extension UI: Basic implementation
- Gmail integration: In progress
- Attachment support: Planned
- MSI installer: Planned

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

| Component | Technology | Deployment | Purpose |
|-----------|------------|------------|---------|
| Interceptor DLL | C++ (MinGW) | MSI/Admin | Captures MAPI calls, writes JSON |
| Native Host | Go | MSI/Admin | Bridges filesystem to browser |
| Browser Extension | React + TypeScript | Chrome Web Store / Edge Add-ons | UI + Gmail API |

### Why This Architecture?

- **Enterprise-friendly**: DLL and native host are stable, deployed via MSI with admin rights
- **Easy updates**: Extension updates arrive via browser extension management - no admin needed
- **Simple OAuth**: Extension uses Chrome Identity API - no separate auth flow
- **Cross-browser**: Same extension works in Chrome and Edge

## Quick Start

### Prerequisites

- Windows 10/11
- Chrome or Edge browser
- Gmail or Google Workspace account

### Installation

> Note: MSI installer coming soon. For now, manual installation is required.

1. Download the latest release from [Releases](https://github.com/telenieko/go-mapi/releases)

2. **Register the DLL** (run as admin):
   ```powershell
   regsvr32 "C:\Program Files\go-mapi\go-mapi.dll"
   ```

3. **Install native messaging host** (run as admin):
   ```powershell
   go-mapi-host.exe -install -extension-id YOUR_EXTENSION_ID
   ```

4. **Install the extension**:
   - Chrome Web Store: [Link coming soon]
   - Edge Add-ons: [Link coming soon]

### Usage

1. Right-click any file in Windows Explorer → "Send to" → "Mail recipient"
2. The email appears in the go-mapi extension popup
3. Click "Save as Draft" or "Send Now"

## Enterprise Deployment

For enterprise deployment, create an MSI that includes:
- `go-mapi.dll` → `C:\Program Files\go-mapi\`
- `go-mapi-host.exe` → `C:\Program Files\go-mapi\`
- Registry entries for MAPI client and native messaging host

The extension can be deployed via:
- Chrome Web Store (public or unlisted)
- Edge Add-ons
- Enterprise policy (`ExtensionInstallForcelist`)

### OAuth Setup

1. Create a project in [Google Cloud Console](https://console.cloud.google.com/)
2. Enable Gmail API
3. Create OAuth 2.0 credentials (Chrome Extension type)
4. Configure the extension with your client ID

## Key Technologies

- **C++ (MinGW)**: MAPI interceptor DLL - zero dependencies, minimal footprint
- **Go**: Native messaging host - efficient filesystem watching
- **TypeScript + React**: Browser extension - modern UI with type safety
- **Chrome Native Messaging**: Secure communication between extension and host
- **Gmail API**: Draft creation and email sending

## Why "go-mapi"?

The name is a nod to "Go(ogle)" and "let's go". The project started as a pragmatic solution to the Affixa shutdown.

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
