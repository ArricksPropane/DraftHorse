# go-mapi

**The MAPI to Google Workspace bridge you always wanted.**

## The Problem

[MAPI](https://en.wikipedia.org/wiki/MAPI) (Messaging Application Programming Interface) is a Microsoft Windows API that allows programs to become email-aware. In practical terms, it enables the "Send by Email" feature found in many Windows applications - from right-clicking a file in Explorer to printing to PDF and emailing it.

**Gmail and Google Workspace have no native MAPI support.** Despite Google Workspace being a major enterprise email solution, there has never been an official MAPI-to-Gmail bridge.

A few third-party tools have filled this gap over the years. The most notable, [Affixa](https://help.affixa.com/article/100-sunsetting-and-retirement-of-affixa), recently announced its shutdown - leaving Google Workspace users without a way to use "Send by Email" functionality.

**go-mapi** aims to be the open-source replacement.

## How It Works

go-mapi uses a two-component architecture to keep things simple and maintainable:

### 1. The Interceptor (C++ DLL)

A minimal, zero-dependency Windows DLL that registers as the system's default mail client. When any application calls `MAPISendMail`, the DLL:

- Captures the message data (subject, body, recipients, attachments)
- Serializes it to a JSON file in `%TEMP%\go-mapi\`
- Returns immediately (non-blocking)

This keeps the native component as simple as possible - it just bridges MAPI calls to the filesystem.

### 2. The Client (Electron/TypeScript)

A background system tray application that:

- Watches the `%TEMP%\go-mapi\` directory for new messages
- Handles OAuth2 authentication with Google
- Uses the Gmail API to create drafts or send emails
- Opens the user's browser to the composed draft ("The Pop")

```
┌─────────────────────┐     JSON file      ┌─────────────────────┐
│   Windows App       │                    │   Electron Client   │
│   (Explorer, etc.)  │                    │   (System Tray)     │
│         │           │                    │         │           │
│    MAPISendMail()   │                    │   Folder Watcher    │
│         ▼           │                    │         ▼           │
│  ┌─────────────┐    │  %TEMP%\go-mapi\   │   ┌───────────┐     │
│  │ go-mapi.dll │────┼───────────────────►│   │ Gmail API │     │
│  └─────────────┘    │                    │   └───────────┘     │
└─────────────────────┘                    └─────────────────────┘
```

## Tech Stack

| Component     | Technology                        |
|---------------|-----------------------------------|
| Interceptor   | C++ (MinGW/MSVC), CMake           |
| Client        | Electron, TypeScript, Node.js     |
| API           | Google Workspace Gmail API, OAuth2|
| Build/CI      | GitHub Actions                    |

## Project Structure

```
go-mapi/
├── src/
│   ├── interceptor/    # C++ MAPI DLL
│   └── client/         # Electron/TypeScript app
├── scripts/            # Registry scripts, build helpers
└── .github/workflows/  # CI/CD pipelines
```

## Development Setup

### Prerequisites

- Windows 10/11
- Node.js 18+
- MinGW toolchain (GCC), CMake, Ninja

**Using [Scoop](https://scoop.sh) (recommended):**
```powershell
scoop install mingw-winlibs-ucrt cmake ninja
```

### Building the Interceptor

```powershell
cd src/interceptor

# Build debug version with tests
.\build.ps1 -Tests

# Build release version
.\build.ps1 -Config Release

# Clean and rebuild
.\build.ps1 -Config Release -Tests -Clean
```

The build script auto-detects MinGW from common scoop/system paths.

### Building the Electron Client

```bash
cd src/client

# Install dependencies
npm install

# Build TypeScript
npm run build

# Run in development mode
npm run dev

# Package for distribution
npm run pack
```

### Project Layout

```
go-mapi/
├── src/
│   ├── interceptor/           # C++ MAPI DLL + build system
│   │   ├── CMakeLists.txt     # Main CMake config
│   │   ├── build.ps1          # Build script (MinGW + Ninja)
│   │   ├── main.cpp           # DLL entry point
│   │   ├── mapi_impl.*        # MAPI function implementations
│   │   ├── json_writer.*      # JSON serialization
│   │   ├── fs_utils.*         # File system operations
│   │   ├── mapi_types.h       # MAPI structure definitions
│   │   └── test-harness/      # C++ test harness
│   │       ├── CMakeLists.txt
│   │       ├── test_utils.*
│   │       └── src/*.cpp
│   └── client/                # Electron/TypeScript app
│       ├── src/
│       │   ├── main.ts        # Main process
│       │   ├── mail-queue.ts  # Email queue management
│       │   ├── watcher.ts     # File watcher
│       │   └── renderer/      # UI code
│       └── package.json
├── scripts/                   # Registry scripts
├── .github/workflows/         # CI/CD (MinGW builds)
├── README.md
├── ROADMAP.md
└── TODO.md
```

### Output Artifacts

After building, output files are located in:
- **DLL**: `src/interceptor/build/bin/go-mapi.dll`
- **Test Harness**: `src/interceptor/build/bin/go-mapi-test-harness.exe`
- **Electron App**: `src/client/dist/go-mapi Client Setup *.exe`

### Testing

```bash
# Run C++ test harness (from interceptor directory)
cd src/interceptor
.\build\bin\go-mapi-test-harness.exe .\build\bin\go-mapi.dll

# Run Electron unit tests
cd src/client
npm test
```

### Registering the DLL (Development)

To register the DLL as your default MAPI handler:

```powershell
# For development builds (from build output)
.\scripts\register-dev.ps1 -BuildPath "C:\dev\go-mapi\src\interceptor\build\bin"

# Or manually import the registry file
regedit /s .\scripts\register-mapi.reg
```

To unregister:

```
regedit /s .\scripts\unregister-mapi.reg
```

### Environment Variables

For development, you may want to set:

```powershell
$env:GO_MAPI_DEBUG=1           # Enable debug logging in DLL
$env:NODE_ENV=development       # Electron development mode
```

## Roadmap

See [ROADMAP.md](ROADMAP.md) for detailed development phases:

1. **Phase 1** - Local Bridge MVP (interceptor + basic client)
2. **Phase 2** - Gmail Integration (OAuth, drafts, send)
3. **Phase 3** - Polish & Release (installer, docs, testing)
4. **Phase 4** - Pro & Enterprise (multi-account, centralized management)

## Why "go-mapi"?

The name is a nod to "Go(ogle)" and get going / let's go. The project started as a pragmatic solution to the Affixa shutdown.

## License

[TBD]

## Contributing

Contributions welcome! This project exists because the community needs an open-source MAPI-to-Gmail bridge.

## References

- [Simple MAPI Documentation](https://learn.microsoft.com/en-us/previous-versions/dd296734(v=vs.85))
- [MAPI Stub Library](https://github.com/microsoft/MAPIStubLibrary)
- [MFCMAPI](https://github.com/microsoft/mfcmapi)
- [Gmail API](https://developers.google.com/gmail/api)
- [Affixa Sunset Announcement](https://help.affixa.com/article/100-sunsetting-and-retirement-of-affixa)
