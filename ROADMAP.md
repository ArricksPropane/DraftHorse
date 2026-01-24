# go-mapi Development Roadmap

## Phase 1: Foundation (MVP - "Local Bridge")

### Project Setup
- [ ] Set up monorepo structure (`/src/interceptor`, `/src/client`)
- [ ] Configure CMake for C++ interceptor build
- [ ] Configure Electron/TypeScript client scaffolding
- [ ] Set up GitHub Actions CI/CD pipeline
- [ ] Define JSON schema for MAPI message interchange

### C++ Interceptor
- [ ] Implement `MAPISendMail` export (ANSI)
- [ ] Implement `MAPISendMailW` export (Unicode)
- [ ] Serialize message data (subject, body, recipients, attachments) to JSON
- [ ] Write JSON to `%TEMP%\go-mapi\` directory
- [ ] Create `.reg` scripts to register DLL as default Windows mail handler
- [ ] Minimal footprint, zero external dependencies

### Electron Client Shell
- [ ] Basic system tray application
- [ ] Folder watcher service for `%TEMP%\go-mapi\`
- [ ] Parse incoming JSON mail requests
- [ ] Display queued emails in UI
- [ ] Basic logging and error display

## Phase 2: Gmail Integration

- [ ] Implement OAuth2 flow for Google account authentication
- [ ] Securely store OAuth tokens
- [ ] Use Gmail API to upload attachments
- [ ] Create Gmail draft from intercepted message
- [ ] "The Pop" - automatically open browser to the specific Gmail draft
- [ ] Support for "Direct Send" mode (bypass draft, send immediately)
- [ ] Handle multiple recipients (To, CC, BCC)

## Phase 3: Polish & Release

### Quality
- [ ] Comprehensive error handling and user-friendly messages
- [ ] Logging system for debugging
- [ ] Security hardening (token storage, IPC)
- [ ] Testing suite (Vite + Vitest for Electron client)

### Distribution
- [ ] Professional `.msi` installer for Windows
- [ ] Auto-update mechanism
- [ ] User documentation / Quick Start guide
- [ ] Release build process and versioning

## Phase 4: Pro & Enterprise (Monetization)

- [ ] Support for multiple Google Workspace accounts
- [ ] Centralized web-based "go-mapi Console" for:
  - License management
  - Remote configuration
  - Usage analytics
- [ ] GPO/Registry support for IT admin pre-configuration
- [ ] Corporate deployment features (silent install, managed settings)
- [ ] Google Workspace Admin integration

## To consider

- Add telemetry, 
  - opt-out on free/personal, opt-in on pro/enterprise?

---

## Architecture Overview

```
┌─────────────────────┐     JSON file      ┌─────────────────────┐
│   Windows App       │                    │   Electron Client   │
│   (e.g., Explorer)  │                    │   (System Tray)     │
│         │           │                    │         │           │
│    MAPISendMail()   │                    │   Folder Watcher    │
│         │           │                    │         │           │
│         ▼           │                    │         ▼           │
│  ┌─────────────┐    │  %TEMP%\go-mapi\   │   ┌───────────┐     │
│  │ go-mapi.dll │────┼───────────────────►│   │ Gmail API │     │
│  └─────────────┘    │                    │   └─────┬─────┘     │
└─────────────────────┘                    │         │           │
                                           │         ▼           │
                                           │   Gmail Draft /     │
                                           │   Direct Send       │
                                           └─────────────────────┘
```

## References

- [Simple MAPI (Microsoft Docs)](https://learn.microsoft.com/en-us/previous-versions/dd296734(v=vs.85))
- [MAPI Stub Library](https://github.com/microsoft/MAPIStubLibrary)
- [MFCMAPI](https://github.com/microsoft/mfcmapi)
- [Gmail API Documentation](https://developers.google.com/gmail/api)
