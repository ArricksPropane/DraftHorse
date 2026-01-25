# AI Agent Instructions: go-mapi

This document provides guidance for AI agents working on the go-mapi project.

> **Prerequisites**: Read [README.md](README.md) for architecture overview and [CONTRIBUTING.md](CONTRIBUTING.md) for development setup.

## Quick Reference

**Stack:** C++ (MinGW), Go, TypeScript + React

**Build:** `npm run build` (or individual `npm run build:interceptor|native-host|extension`)

**Test:** `npm run test`

## Architecture Summary

```
Windows App → [C++ DLL] → JSON files → [Go Native Host] → [Browser Extension] → Gmail API
```

| Component | Location | Language | Key Constraint |
|-----------|----------|----------|----------------|
| Interceptor DLL | `src/interceptor/` | C++ | Zero dependencies, non-blocking |
| Native Host | `src/native-host/` | Go | Filesystem watching, native messaging |
| Extension | `src/extension/` | TypeScript/React | Chrome Identity API for OAuth |

## Project Philosophy

### Technical Principles

- **Privacy-focused**: Data stays local (filesystem, browser storage). Minimize cloud exposure.
- **Local-first**: The app works offline. External API calls only when necessary (Gmail API for sending).
- **Cost-conscious**: Prefer solutions that don't require paid services.
- **Pragmatic**: Ship working software. Iterate based on real usage.
- **Type-safe**: Use TypeScript strict mode. Go provides compile-time safety.

### Code Organization

- **Keep related things together**: Avoid excessive file splitting
- **Minimize bloat**: Single file for related components when reasonable
- **Colocation preferred**: Related code lives near each other, not in separate layer directories

### UI Philosophy

- **Component-first libraries**: Use React Bootstrap, Radix UI, or similar
- **Avoid utility-first CSS**: No Tailwind. Prefer semantic, maintainable markup.
- **Rationale**: Cleaner JSX, easier to maintain, less verbose

### Internationalization

- The project should be ready for translations from the start
- Use message catalogs or i18n libraries for user-facing strings
- Technical documentation remains in English

## Component Guidelines

### Interceptor (C++ DLL)

**Purpose:** Capture MAPI calls and write JSON files to `%TEMP%\go-mapi\`

**Critical constraints:**
- Zero external dependencies (no nlohmann/json - hand-rolled JSON)
- Target Simple MAPI spec only (not Extended MAPI/COM)
- Non-blocking - return immediately to caller
- Thread-safe - multiple apps may call simultaneously

**Key files:**
- `main.cpp` - DLL entry point, exports MAPI functions
- `mapi_impl.cpp` - MAPISendMail implementation
- `json_writer.cpp` - JSON serialization
- `fs_utils.cpp` - File system operations

**When modifying:**
- Always test with debug build: `npm run build:interceptor:debug`
- Test with multiple calling applications (Explorer, PDF viewers, etc.)
- Check for memory leaks and thread safety

### Native Host (Go)

**Purpose:** Bridge filesystem to browser extension via Chrome Native Messaging

**Protocol (length-prefixed JSON):**
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

**Key files:**
- `main.go` - Entry point, message loop
- `protocol.go` - Native messaging protocol
- `watcher.go` - Filesystem watcher using fsnotify

**When modifying:**
- Run `gofmt` before committing
- Test native messaging with the extension
- Handle filesystem edge cases (deleted files, permission errors)

### Browser Extension (React)

**Purpose:** UI and Gmail API integration

**Stack:** React 18, React Bootstrap, Vite, TypeScript

**Key files:**
- `manifest.json` - Extension manifest v3
- `src/background/service-worker.ts` - Native messaging + Gmail API
- `src/popup/App.tsx` - Main UI component
- `src/types/messages.ts` - TypeScript interfaces

**When modifying:**
- Use React Bootstrap components for UI
- Keep service worker minimal - offload to popup when possible
- TypeScript strict mode is required
- Test in both Chrome and Edge

## JSON Schema (IPC)

Files written to `%TEMP%\go-mapi\*.json`:

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

Full schema: [docs/json-schema.json](docs/json-schema.json)

## Documentation Standards

### When Creating/Updating Docs

- **README.md**: User-focused. How to install and use.
- **CONTRIBUTING.md**: Developer-focused. How to build, test, contribute.
- **AGENTS.md**: AI agent-focused. Technical context and constraints.
- **ROADMAP.md**: Strategic planning. Phases, not timelines.

### Code Comments

- Comment the "why", not the "what"
- Document non-obvious constraints
- Keep comments concise

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):
- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `test:` Test additions/changes
- `refactor:` Code restructuring
- `chore:` Build, CI, dependencies

## Task Approach

### Before Starting Work

1. Understand the component affected (interceptor, native-host, extension)
2. Read relevant source files to understand existing patterns
3. Check if tests exist and run them
4. Identify related files that may need changes

### When Implementing

1. Follow existing code patterns in the component
2. Keep changes minimal and focused
3. Add tests for new functionality
4. Update documentation if behavior changes

### Before Submitting

1. Run the full build: `npm run build`
2. Run tests: `npm run test`
3. Test manually if changing user-facing behavior
4. Write clear commit messages

## Persona-Based Work

When working on specific components, adopt these personas:

- **C++ Specialist** (`src/interceptor/`): Focus on minimal, non-blocking code. Avoid dependencies.
- **Go Developer** (`src/native-host/`): Handle filesystem watching and native messaging protocol.
- **Frontend Developer** (`src/extension/`): Build clean React UI with Gmail integration.
- **DevOps/CI** (`.github/workflows/`): Windows builds for all three components.

## Common Tasks

### Adding a new MAPI field

1. Update JSON schema in `docs/json-schema.json`
2. Add field to C++ `json_writer.cpp`
3. Update TypeScript types in `src/extension/src/types/`
4. Update Go structs in `src/native-host/`

### Adding extension settings

1. Add setting to Chrome storage API
2. Create UI in settings panel
3. Read setting in service worker
4. Persist with `chrome.storage.sync`

### Debugging the full flow

1. Build all components: `npm run build`
2. Register DLL (admin): `npm run register:mapi`
3. Load extension in Chrome/Edge developer mode
4. Check `%TEMP%\go-mapi\` for JSON files
5. Check extension DevTools for errors

## Key Decisions (ADRs)

### MinGW over MSVC

**Context:** Need to compile C++ DLL on Windows
**Decision:** Use MinGW (GCC) instead of MSVC
**Rationale:** Simpler build setup, no Visual Studio dependency, works with Scoop

### React Bootstrap over Tailwind

**Context:** Need UI component library for extension
**Decision:** Use React Bootstrap
**Rationale:** Component-first, semantic markup, project philosophy avoids utility-first CSS

### Filesystem IPC over Named Pipes

**Context:** Need communication between DLL and native host
**Decision:** Use JSON files in `%TEMP%`
**Rationale:** Simple, debuggable, works across processes, no Windows API complexity
