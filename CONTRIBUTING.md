# Contributing to go-mapi

Thank you for your interest in contributing to go-mapi! This document covers everything you need to know to contribute effectively.

> **Prerequisite**: Please read [README.md](README.md) first to understand the project architecture.

## Contributor License Agreement (CLA)

Before we can accept your contribution, you must sign our Contributor License Agreement (CLA). This is a one-time requirement that protects both you and the project.

When you submit your first pull request, a bot will automatically check if you've signed the CLA and guide you through the process if needed.

## Code of Conduct

Please be respectful and inclusive in all interactions. We expect professional conduct from all contributors.

## Development Environment Setup

### Prerequisites

- Windows 10/11
- MinGW toolchain (GCC 13+)
- CMake 3.20+
- Ninja build system
- Go 1.21+
- Node.js 18+
- Git

**Install all build tools via [Scoop](https://scoop.sh):**
```powershell
# Install Scoop if you haven't
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression

# Install tools
scoop install mingw-winlibs-ucrt cmake ninja go nodejs git
```

### Clone and Build

```powershell
git clone https://github.com/anthropics/go-mapi.git
cd go-mapi

# Install Node.js dependencies
npm install

# Build all components
npm run build
```

## Project Structure

```
go-mapi/
├── src/
│   ├── interceptor/       # C++ MAPI DLL
│   │   ├── main.cpp       # DLL entry point
│   │   ├── mapi_impl.cpp  # MAPISendMail implementation
│   │   ├── json_writer.cpp
│   │   └── test-harness/  # C++ tests
│   ├── native-host/       # Go native messaging host
│   │   ├── main.go        # Entry point
│   │   ├── watcher.go     # Filesystem watcher
│   │   ├── protocol.go    # Native messaging protocol
│   │   └── manifests/     # Chrome/Edge host manifests
│   └── extension/         # Browser extension (React)
│       ├── manifest.json  # Extension manifest v3
│       ├── src/
│       │   ├── background/    # Service worker
│       │   ├── popup/         # React UI
│       │   └── types/         # TypeScript types
│       └── package.json
├── scripts/               # Install & dev tools
│   ├── install.ps1              # Install/uninstall go-mapi
│   ├── test-drop-email.ps1      # Drop test emails for dev
│   └── generate-icons.js        # Generate extension icons
├── tests/
│   ├── e2e/               # Playwright E2E tests
│   └── sandbox/           # Windows Sandbox DLL tests
├── docs/                  # Additional documentation
│   └── json-schema.json   # IPC message schema
└── .github/workflows/     # CI/CD pipelines
```

## Build Commands

```powershell
# Build all components
npm run build

# Build individual components
npm run build:interceptor      # C++ DLL (Release)
npm run build:interceptor:debug # C++ DLL (Debug)
npm run build:native-host      # Go binary
npm run build:extension        # Browser extension

# Development mode (watch for changes)
npm run dev:extension

# Run tests
npm run test                   # Unit tests (Go + Extension)
npm run test:interceptor       # C++ test harness
npm run test:e2e               # Playwright E2E tests
npm run test:dll:sandbox       # Windows Sandbox DLL test (requires Win11 24H2+)

# Clean build artifacts
npm run clean
```

## Component-Specific Development

### Interceptor (C++ DLL)

Location: `src/interceptor/`

**Key constraints:**
- Zero external dependencies (no nlohmann/json - hand-rolled JSON)
- Target Simple MAPI spec only (not Extended MAPI/COM)
- Non-blocking - return immediately to caller
- Thread-safe operations

**Building and testing:**
```powershell
npm run build:interceptor:debug
npm run test:interceptor
```

**Output location:** `build/` directory

### Native Host (Go)

Location: `src/native-host/`

**Key files:**
- `main.go` - Entry point and message loop
- `protocol.go` - Native messaging protocol (length-prefixed JSON)
- `watcher.go` - Filesystem watcher using fsnotify

**Building:**
```powershell
npm run build:native-host
```

**Testing locally:**
```powershell
cd src/native-host
go test ./...
```

### Browser Extension (React)

Location: `src/extension/`

**Stack:** React 18, React Bootstrap, Vite, TypeScript

**Development:**
```powershell
cd src/extension
npm install
npm run dev    # Watch mode - rebuilds on changes
```

Then load the extension:
1. Open Chrome/Edge → Extensions
2. Enable Developer Mode
3. Click "Load unpacked" → Select `src/extension/dist/`
4. Reload extension after each change

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

### 2. Make Your Changes

Follow the code style guidelines below.

### 3. Write Tests

**For C++:**
- Add tests to `src/interceptor/test-harness/src/`
- Run: `npm run test:interceptor`

**For Go:**
- Add tests in `*_test.go` files
- Run: `cd src/native-host && go test ./...`

**For TypeScript/Extension:**
- Add tests alongside source files
- Run: `npm run --prefix src/extension test`

### 4. Commit Your Changes

We use [Conventional Commits](https://www.conventionalcommits.org/):

```bash
git commit -m "feat: add attachment upload support"
git commit -m "fix: handle empty recipient list"
git commit -m "docs: update installation instructions"
```

**Prefixes:** `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`

### 5. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then open a pull request against `main`.

## Code Style Guidelines

### C++ (Interceptor)

- **Standard:** C++17
- **Compiler:** MinGW GCC
- **Naming:** `PascalCase` for classes/functions, `camelCase` for variables
- **Indentation:** 4 spaces (no tabs)
- **Braces:** K&R style

### Go (Native Host)

- **Standard:** Go 1.21+
- **Format:** Always run `gofmt` before committing
- **Naming:** Follow Go conventions (`PascalCase` for exports, `camelCase` for private)

### TypeScript (Extension)

- **Naming:** `PascalCase` for components/types, `camelCase` for functions/variables
- **Indentation:** 2 spaces
- **Semicolons:** Required
- **Strict mode:** Always enabled

## Testing Your Changes

### Automated Tests

```powershell
# Unit tests
npm run test                   # All unit tests
npm run test:interceptor       # C++ test harness
npm run test:e2e               # Playwright browser tests

# Windows Sandbox tests (requires Windows 11 24H2+)
npm run test:dll:sandbox -- -RegistrationOnly  # Test DLL registration
npm run test:dll:sandbox -- -KeepRunning       # Keep sandbox open for inspection
```

### Manual Testing Workflow

1. Build the component you changed
2. If testing the full flow:
   - Register the DLL: `npm run register:mapi` (admin required)
   - Start the native host
   - Load the extension
3. Right-click a file → "Send to" → "Mail recipient"
4. Verify the email appears in the extension popup

### Debugging

**DLL debugging:**
- Build with debug flag: `npm run build:interceptor:debug`
- Attach debugger to the calling application

**Native host debugging:**
- Add logging statements
- Check `%TEMP%\go-mapi\` for JSON files

**Extension debugging:**
- Open DevTools for the extension popup
- Check the service worker console in `chrome://extensions`

## Areas for Contribution

### High Priority
- [ ] Attachment support (file upload to Gmail)
- [ ] Settings UI in extension
- [ ] MSI installer creation
- [ ] Comprehensive test coverage

### Medium Priority
- [ ] Additional MAPI function support
- [ ] UI/UX improvements
- [ ] Error handling improvements

### Lower Priority
- [ ] Performance optimization
- [ ] Accessibility improvements

## Getting Help

- **Questions:** Open a [GitHub Discussion](https://github.com/anthropics/go-mapi/discussions)
- **Bugs:** Open an [Issue](https://github.com/anthropics/go-mapi/issues) with reproduction steps
- **Ideas:** Start a Discussion or open an Issue

## License

By contributing, you agree that your contributions will be licensed under the project's license (TBD).
