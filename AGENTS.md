# Agent Instructions: go-mapi

You are working on a Windows MAPI-to-Gmail bridge with three components.

## Architecture Overview

```
Windows App → [C++ DLL] → JSON files → [Go Native Host] → [Browser Extension] → Gmail API
```

## Components

### 1. Interceptor (C++ DLL) - `src/interceptor/`

**Purpose**: Capture MAPI calls and write JSON files

**Key files**:
- `main.cpp` - DLL entry point, exports MAPI functions
- `mapi_impl.cpp` - MAPISendMail implementation
- `json_writer.cpp` - JSON serialization
- `fs_utils.cpp` - File system operations

**Constraints**:
- Zero external dependencies (no nlohmann/json - hand-rolled JSON)
- Target Simple MAPI spec only (not Extended MAPI/COM)
- Non-blocking - return immediately to caller
- Thread-safe

**Output**: JSON files in `%TEMP%\go-mapi\`

### 2. Native Messaging Host (Go) - `src/native-host/`

**Purpose**: Bridge filesystem to browser extension via Chrome Native Messaging

**Key files**:
- `main.go` - Entry point, message loop
- `protocol.go` - Native messaging protocol (length-prefixed JSON)
- `watcher.go` - Filesystem watcher using fsnotify

**Protocol**:
```json
// Host → Extension
{"type": "email", "id": "...", "data": {...}}
{"type": "removed", "id": "..."}
{"type": "ready", "version": "1.0.0"}

// Extension → Host
{"type": "process", "id": "..."}
{"type": "delete", "id": "..."}
{"type": "list"}
```

### 3. Browser Extension (React) - `src/extension/`

**Purpose**: UI and Gmail API integration

**Key files**:
- `manifest.json` - Extension manifest v3
- `src/background/service-worker.ts` - Native messaging + Gmail API
- `src/popup/App.tsx` - Main UI component
- `src/types/messages.ts` - TypeScript interfaces

**Stack**: React 18, React Bootstrap, Vite, TypeScript

## JSON Schema (IPC)

Files in `%TEMP%\go-mapi\*.json`:

```json
{
  "version": 1,
  "timestamp": "2024-01-15T10:30:00.000Z",
  "subject": "Email Subject",
  "body": "Email body content",
  "bodyFormat": "plain|html",
  "recipients": {
    "to": [{"name": "John", "address": "john@example.com"}],
    "cc": [],
    "bcc": []
  },
  "attachments": [
    {"filename": "doc.pdf", "path": "C:\\path\\to\\file.pdf", "size": 102400}
  ],
  "originApp": "explorer.exe"
}
```

## Build Commands

```powershell
npm run build              # Build all
npm run build:interceptor  # C++ DLL
npm run build:native-host  # Go binary
npm run build:extension    # Browser extension
npm run test               # Run tests
```

## Persona Tasks

- **C++ Specialist**: Focus on `src/interceptor/`. Keep DLL minimal and non-blocking.
- **Go Developer**: Focus on `src/native-host/`. Handle filesystem watching and native messaging.
- **Frontend Developer**: Focus on `src/extension/`. Build clean React UI with Gmail integration.
- **DevOps/CI**: Focus on `.github/workflows/`. Windows builds for all three components.
