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
- CMake 3.20+
- MSVC or MinGW-w64

### Building

```bash
# Client
cd src/client
npm install
npm run dev

# Interceptor
cd src/interceptor
cmake -B build
cmake --build build
```

### Registering the DLL

Run `scripts/register-mapi.reg` to set go-mapi.dll as the default Windows mail handler.

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
