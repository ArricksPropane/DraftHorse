# go-mapi

**The MAPI to Gmail bridge you always wanted.**

## The Problem

[MAPI](https://en.wikipedia.org/wiki/MAPI) (Messaging Application Programming Interface) is a Microsoft Windows API that allows programs to become email-aware. It enables the "Send by Email" feature found in many Windows applications - from right-clicking a file in Explorer to printing to PDF and emailing it.

**Gmail and Google Workspace have no native MAPI support.** Despite Google Workspace being a major enterprise email solution, there has never been an official MAPI-to-Gmail bridge.

A few third-party tools have filled this gap over the years. The most notable, [Affixa](https://help.affixa.com/article/100-sunsetting-and-retirement-of-affixa), recently announced its shutdown - leaving Google Workspace users without a way to use "Send by Email" functionality.

**go-mapi** aims to be the open-source replacement.

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

| Component | Technology | Deployment | Update Frequency |
|-----------|------------|------------|------------------|
| Interceptor DLL | C++ (MinGW) | MSI/Admin | Rare |
| Native Host | Go | MSI/Admin | Rare |
| Browser Extension | React + TypeScript | Chrome Web Store / Edge Add-ons | Frequent |

### Why This Architecture?

- **Enterprise-friendly**: DLL and native host are stable, deployed via MSI with admin rights
- **Easy updates**: Extension updates arrive via browser extension management - no admin needed
- **Simple OAuth**: Extension uses Chrome Identity API - no separate auth flow
- **Cross-browser**: Same extension works in Chrome and Edge

## Quick Start

### Prerequisites

- Windows 10/11
- Chrome or Edge browser
- Node.js 18+ and Go 1.21+ (for building)

**Install build tools via [Scoop](https://scoop.sh):**
```powershell
scoop install mingw-winlibs-ucrt cmake ninja go nodejs
```

### Building

```powershell
# Build everything
npm run build

# Or build components individually
npm run build:interceptor      # C++ DLL
npm run build:native-host      # Go binary
npm run build:extension        # Browser extension
```

### Installation

1. **Register the DLL** (run as admin):
   ```powershell
   npm run register:mapi
   ```

2. **Install native messaging host**:
   ```powershell
   npm run register:native-host -- -ExtensionId YOUR_EXTENSION_ID
   ```

3. **Load the extension**:
   - Open Chrome/Edge → Extensions → Enable Developer Mode
   - Click "Load unpacked" → Select `src/extension/dist/`

### Usage

1. Right-click any file in Windows Explorer → "Send to" → "Mail recipient"
2. The email appears in the go-mapi extension popup
3. Click "Save as Draft" or "Send Now"

## Project Structure

```
go-mapi/
├── src/
│   ├── interceptor/       # C++ MAPI DLL
│   │   ├── build.ps1      # Build script
│   │   ├── main.cpp       # DLL entry point
│   │   └── test-harness/  # C++ tests
│   ├── native-host/       # Go native messaging host
│   │   ├── main.go        # Entry point
│   │   ├── watcher.go     # File system watcher
│   │   ├── protocol.go    # Native messaging protocol
│   │   └── manifests/     # Chrome/Edge host manifests
│   └── extension/         # Browser extension (React)
│       ├── manifest.json  # Extension manifest v3
│       ├── src/
│       │   ├── background/    # Service worker
│       │   ├── popup/         # React UI
│       │   └── types/         # TypeScript types
│       └── package.json
├── scripts/               # Registry scripts
├── package.json           # Root build scripts
└── .github/workflows/     # CI/CD
```

## Development

### Build Scripts

```powershell
npm run build              # Build all components
npm run build:interceptor  # Build C++ DLL (Release)
npm run build:native-host  # Build Go native host
npm run build:extension    # Build browser extension

npm run dev:extension      # Watch mode for extension

npm run test               # Run tests
npm run clean              # Clean all build artifacts
```

### Extension Development

```powershell
cd src/extension
npm install
npm run dev    # Watch mode - rebuilds on changes
```

Then reload the extension in Chrome/Edge after each change.

### Testing the DLL

```powershell
npm run build:interceptor:debug
npm run test:interceptor
```

## Configuration

### OAuth Setup

1. Create a project in [Google Cloud Console](https://console.cloud.google.com/)
2. Enable Gmail API
3. Create OAuth 2.0 credentials (Chrome Extension type)
4. Update `src/extension/manifest.json` with your client ID

### Enterprise Deployment

For enterprise deployment, create an MSI that includes:
- `go-mapi.dll` → `C:\Program Files\go-mapi\`
- `go-mapi-host.exe` → `C:\Program Files\go-mapi\`
- Registry entries for MAPI client and native messaging host

The extension can be deployed via:
- Chrome Web Store (public or unlisted)
- Edge Add-ons
- Enterprise policy (`ExtensionInstallForcelist`)

## Why "go-mapi"?

The name is a nod to "Go(ogle)" and "let's go". The project started as a pragmatic solution to the Affixa shutdown.

## License

MIT

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## References

- [Simple MAPI Documentation](https://learn.microsoft.com/en-us/previous-versions/dd296734(v=vs.85))
- [Chrome Native Messaging](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging)
- [Gmail API](https://developers.google.com/gmail/api)
- [Affixa Sunset Announcement](https://help.affixa.com/article/100-sunsetting-and-retirement-of-affixa)
