# go-mapi Quick Start Guide

Get go-mapi up and running in 5 minutes!

## Prerequisites

- Windows 10/11
- Visual Studio 2022 (with C++)
- Node.js 18+
- CMake 3.20+

## Installation

### 1. Clone Repository

```bash
git clone https://github.com/yourusername/go-mapi.git
cd go-mapi
```

### 2. Build Everything

```powershell
# Build DLL and tests (2-3 minutes)
.\build.ps1 -Config Debug -Platform x64 -Tests

# Verify DLL was built
ls build\bin\go-mapi.dll
```

### 3. Test the Build

```bash
# Run C++ test harness
build\bin\go-mapi-test-harness.exe

# Output: ✓ [PASS] Simple Send, ✓ [PASS] With Attachments, etc.
```

### 4. Set Up Electron Client

```bash
cd src/client
npm install
npm test

# Run in development mode
npm run dev
```

### 5. Register DLL (Optional)

To use go-mapi as your default mail handler:

```powershell
# Administrative PowerShell
.\scripts\register-dev.ps1 -BuildPath "c:\dev\go-mapi\build\bin"
```

## Verify Everything Works

### Check JSON File Creation

The DLL writes JSON files to `%TEMP%\go-mapi\`:

```powershell
# Open the directory
explorer $env:TEMP\go-mapi

# After running test harness, you'll see files like:
# msg_20240115_102345_abc123.json
```

### Inspect a JSON File

```powershell
# View the captured email as JSON
Get-Content $env:TEMP\go-mapi\msg_*.json | ConvertFrom-Json | Format-List
```

### Test File Watcher

```bash
cd src/client

# In one terminal: start the client
npm run dev

# In another terminal: run the test harness
# (from project root)
build\bin\go-mapi-test-harness.exe

# Watch: New emails appear in the client UI!
```

## Project Structure Quick View

```
go-mapi/
├── src/interceptor/    → C++ DLL (intercepts MAPI)
├── src/client/         → Electron app (shows queue)
├── src/test-harness/   → Test suite
├── build.ps1           → Build script
└── README.md           → Full documentation
```

## Common Tasks

### Build Just the DLL

```powershell
.\build.ps1 -Config Release -Platform x64
```

### Run Client Tests

```bash
cd src/client
npm test
```

### Clean Build

```powershell
.\build.ps1 -Config Debug -Platform x64 -Clean
```

### Check for Errors

```bash
# C++ compilation errors
cmake --build build --verbose

# TypeScript errors
cd src/client && npm run build
```

## Troubleshooting

### "CMake not found"
→ Install CMake or add to PATH: https://cmake.org/download/

### "Visual Studio not found"
→ Install Visual Studio 2022 with C++: https://visualstudio.microsoft.com/

### "npm not found"
→ Install Node.js: https://nodejs.org/

### DLL not building
→ Try clean build: `.\build.ps1 -Config Debug -Platform x64 -Clean`

### Tests fail
→ Check `%TEMP%\go-mapi\errors\` for error logs

## Next Steps

1. **Understand the architecture**: Read [ARCHITECTURE.md](ARCHITECTURE.md)
2. **See the roadmap**: Read [ROADMAP.md](ROADMAP.md)
3. **Contribute**: Read [CONTRIBUTING.md](CONTRIBUTING.md)
4. **Test more**: Read [TESTING.md](TESTING.md)

## Key Commands Reference

| Task | Command |
|------|---------|
| Build (Debug) | `.\build.ps1 -Config Debug -Platform x64` |
| Build (Release) | `.\build.ps1 -Config Release -Platform x64` |
| Run C++ Tests | `build\bin\go-mapi-test-harness.exe` |
| Run TS Tests | `cd src/client && npm test` |
| Start Client | `cd src/client && npm run dev` |
| Register DLL | `.\scripts\register-dev.ps1 -BuildPath "c:\dev\go-mapi\build\bin"` |

## Where to Go Next

- **Want to build Gmail integration?** → Start [Phase 2](ROADMAP.md#phase-2-gmail-integration)
- **Found a bug?** → Create an issue on GitHub
- **Want to contribute?** → Read [CONTRIBUTING.md](CONTRIBUTING.md)
- **Need help?** → Check [TESTING.md](TESTING.md) or [ARCHITECTURE.md](ARCHITECTURE.md)

---

**You're all set! Happy coding! 🚀**

