# TODO - Phase 1: Local Bridge MVP

## 1. Project Structure Setup

- [ ] Create `/src/interceptor/` directory for C++ DLL
- [ ] Create `/src/client/` directory for Electron app
- [ ] Create `/scripts/` directory for registry and build scripts
- [ ] Create `/docs/` directory for additional documentation
- [ ] Add `.gitignore` with entries for:
  - `node_modules/`
  - `build/`
  - `dist/`
  - `*.dll`
  - `*.obj`
  - `*.exe`
  - `.env`

## 2. C++ Interceptor DLL

### 2.1 CMake Configuration

- [ ] Create `src/interceptor/CMakeLists.txt`
  - Target: shared library (DLL)
  - Name: `go-mapi.dll`
  - C++17 standard
  - Link against: `kernel32.lib`, `user32.lib`
  - Export symbols for MAPI functions
  - Support both x86 and x64 builds

### 2.2 MAPI Exports

Create `src/interceptor/mapi_exports.def` with:
```
EXPORTS
    MAPISendMail
    MAPISendMailW
    MAPILogon
    MAPILogoff
    MAPIFreeBuffer
    MAPISendDocuments
```

### 2.3 Main Source Files

- [ ] Create `src/interceptor/main.cpp`
  - DllMain entry point
  - Initialize output directory on attach

- [ ] Create `src/interceptor/mapi_impl.cpp` and `mapi_impl.h`
  - Implement `MAPISendMail` (ANSI version)
    - Parse `lpMessage` parameter (MapiMessage struct)
    - Extract: subject, noteText, recipients, attachments
    - Call JSON serialization
    - Return SUCCESS_SUCCESS (0)
  - Implement `MAPISendMailW` (Unicode version)
    - Same as above but for wide strings
  - Implement stub functions for MAPILogon, MAPILogoff, MAPIFreeBuffer
    - These can return SUCCESS_SUCCESS immediately

### 2.4 JSON Serialization

- [ ] Create `src/interceptor/json_writer.cpp` and `json_writer.h`
  - Simple JSON writer (no external dependencies)
  - Function: `WriteMailToJson(const MailMessage& msg, const std::wstring& outputPath)`
  - Generate unique filename using timestamp + random suffix
  - Output path: `%TEMP%\go-mapi\{timestamp}_{random}.json`

### 2.5 JSON Schema

Output JSON format:
```json
{
  "version": 1,
  "timestamp": "2024-01-15T10:30:00Z",
  "subject": "Email subject",
  "body": "Email body text",
  "bodyFormat": "plain",
  "recipients": {
    "to": [
      { "name": "John Doe", "address": "john@example.com" }
    ],
    "cc": [],
    "bcc": []
  },
  "attachments": [
    {
      "filename": "document.pdf",
      "path": "C:\\Users\\...\\document.pdf",
      "size": 12345
    }
  ],
  "originApp": "explorer.exe"
}
```

- [ ] Create `docs/json-schema.json` with formal JSON Schema definition

### 2.6 File System Operations

- [ ] Create `src/interceptor/fs_utils.cpp` and `fs_utils.h`
  - Function: `EnsureOutputDirectory()` - create `%TEMP%\go-mapi\` if not exists
  - Function: `GetTempPath()` - wrapper for Windows API
  - Function: `GenerateUniqueFilename()` - timestamp + 6 random chars
  - Function: `WriteFile()` - UTF-8 encoded JSON output

### 2.7 MAPI Structures

- [ ] Create `src/interceptor/mapi_types.h`
  - Define MapiMessage, MapiRecipDesc, MapiFileDesc structs
  - Match Windows MAPI definitions exactly
  - Reference: https://learn.microsoft.com/en-us/windows/win32/api/mapi/ns-mapi-mapimessage

## 3. Registry Scripts

- [ ] Create `scripts/register-mapi.reg`
  ```reg
  Windows Registry Editor Version 5.00

  [HKEY_LOCAL_MACHINE\SOFTWARE\Clients\Mail\go-mapi]
  @="go-mapi"

  [HKEY_LOCAL_MACHINE\SOFTWARE\Clients\Mail\go-mapi\DLLPath]
  @="C:\\Program Files\\go-mapi\\go-mapi.dll"
  ```

- [ ] Create `scripts/unregister-mapi.reg` to remove entries

- [ ] Create `scripts/register-dev.ps1` PowerShell script
  - Registers DLL from build output directory
  - For development/testing use

## 4. Electron Client

### 4.1 Project Initialization

- [ ] Initialize Electron project in `src/client/`
  ```bash
  npm init
  npm install electron typescript @types/node
  npm install --save-dev electron-builder
  ```

- [ ] Create `src/client/tsconfig.json`
  - Target: ES2020
  - Module: CommonJS
  - Strict mode enabled

- [ ] Create `src/client/package.json` scripts:
  - `dev`: run Electron with Vite dev server
  - `build`: vite build && electron-builder
  - `test`: vitest
  - `test:coverage`: vitest --coverage

### 4.2 Main Process

- [ ] Create `src/client/src/main.ts`
  - Create BrowserWindow (hidden by default)
  - Create system tray icon
  - Tray menu: Show/Hide, Settings, Quit
  - Start folder watcher on app ready
  - Handle single instance lock

### 4.3 Folder Watcher

- [ ] Create `src/client/src/watcher.ts`
  - Use `chokidar` or native `fs.watch`
  - Watch directory: `%TEMP%\go-mapi\`
  - On new `.json` file:
    - Read and parse JSON
    - Validate against schema
    - Emit event to renderer/main
    - Move file to `processed/` subdirectory

### 4.4 Mail Queue

- [ ] Create `src/client/src/mail-queue.ts`
  - In-memory queue of pending emails
  - Interface: `MailMessage` matching JSON schema
  - Methods: `add()`, `remove()`, `getAll()`, `getById()`

### 4.5 Renderer (UI)

- [ ] Create `src/client/src/renderer/index.html`
  - Simple UI showing pending emails
  - List view with subject, recipients, timestamp
  - Actions: Send, Edit, Delete

- [ ] Create `src/client/src/renderer/renderer.ts`
  - IPC communication with main process
  - Display mail queue
  - Handle user actions

- [ ] Create `src/client/src/renderer/styles.css`
  - Clean, minimal styling
  - System tray popup-friendly dimensions

### 4.6 Preload Script

- [ ] Create `src/client/src/preload.ts`
  - Expose safe IPC methods to renderer
  - `getQueue()`, `sendMail()`, `deleteMail()`

### 4.7 Tray Icon

- [ ] Create `src/client/assets/tray-icon.png` (16x16, 32x32)
- [ ] Create `src/client/assets/tray-icon-pending.png` (with notification dot)

## 5. GitHub Actions CI/CD

- [ ] Update `.github/workflows/build.yml`
  - Matrix build: x86, x64
  - Steps:
    1. Checkout
    2. Setup MSVC
    3. Build C++ interceptor with CMake
    4. Setup Node.js
    5. Install client dependencies
    6. Build Electron app
    7. Upload artifacts

- [ ] Create `.github/workflows/release.yml`
  - Trigger on tag push (v*)
  - Build all artifacts
  - Create GitHub Release
  - Upload installers

## 6. Testing

### 6.1 C++ Interceptor Unit Tests

- [ ] Create `src/interceptor/tests/` directory
- [ ] Set up Google Test or Catch2 framework in CMake
- [ ] Create `test_json_writer.cpp`
  - Test JSON escaping (quotes, backslashes, unicode)
  - Test empty fields handling
  - Test large body text
  - Test special characters in filenames
- [ ] Create `test_fs_utils.cpp`
  - Test directory creation
  - Test unique filename generation (no collisions)
  - Test UTF-8 file writing
  - Test temp path resolution

### 6.2 MAPI Test Harness (Critical)

- [ ] Create `src/test-harness/` - standalone C++ test application
- [ ] Create `src/test-harness/CMakeLists.txt`
  - Build as executable
  - Links against our go-mapi.dll dynamically
- [ ] Create `src/test-harness/main.cpp`
  - Load go-mapi.dll via LoadLibrary
  - Get MAPISendMail function pointer via GetProcAddress
  - Call MAPISendMail with test data
  - Verify JSON file was created
  - Parse and validate JSON output
- [ ] Create test scenarios:
  - [ ] `test_simple_send.cpp` - basic email, no attachments
  - [ ] `test_with_attachment.cpp` - single file attachment
  - [ ] `test_multiple_attachments.cpp` - multiple files
  - [ ] `test_unicode.cpp` - unicode subject, body, recipient names
  - [ ] `test_empty_fields.cpp` - missing subject, empty body
  - [ ] `test_multiple_recipients.cpp` - TO, CC, BCC
  - [ ] `test_large_body.cpp` - very long email body
  - [ ] `test_special_paths.cpp` - spaces, unicode in file paths

### 6.3 Windows Integration Tests

- [ ] Create `scripts/test-sendto.ps1`
  - Programmatically trigger "Send to > Mail recipient"
  - Use Shell.Application COM object
  - Verify JSON output
- [ ] Create `scripts/test-mapi-apps.ps1`
  - Test with common MAPI-using apps if available
  - Document which apps were tested
- [ ] Create `src/test-harness/test_real_mapi_flow.cpp`
  - Register our DLL as default mail client
  - Trigger MAPISendDocuments (what Explorer uses)
  - Verify the full chain works

### 6.4 Electron Client Unit Tests

- [ ] Set up Vite + Vitest in `src/client/`
  - Use Vite as build tool for faster dev experience
  - Configure Vitest for unit testing (Jest-compatible API)
- [ ] Create `src/client/tests/` directory
- [ ] Create `mail-queue.test.ts`
  - Test add/remove/getAll/getById
  - Test queue persistence
  - Test duplicate handling
- [ ] Create `json-parser.test.ts`
  - Test valid JSON parsing
  - Test schema validation
  - Test malformed JSON handling
  - Test missing required fields
  - Test version compatibility
- [ ] Create `watcher.test.ts`
  - Test file detection
  - Test file processing
  - Test error handling (locked files, permissions)
  - Test cleanup of processed files

### 6.5 Electron Client Integration Tests

- [ ] Create `src/client/tests/integration/` directory
- [ ] Create `watcher-integration.test.ts`
  - Drop real JSON files into watch directory
  - Verify they're picked up and parsed
  - Verify they're moved to processed folder
- [ ] Create `ipc-integration.test.ts`
  - Test main <-> renderer communication
  - Test queue updates propagate to UI

### 6.6 End-to-End Tests

- [ ] Create `tests/e2e/` directory at project root
- [ ] Create `tests/e2e/full-flow.test.ts`
  - Start Electron client
  - Run MAPI test harness
  - Verify email appears in client UI
  - Verify all fields match
- [ ] Create `tests/e2e/stress.test.ts`
  - Rapid-fire multiple MAPI calls
  - Verify all messages are captured
  - Verify no race conditions

### 6.7 Test Fixtures

- [ ] Create `tests/fixtures/` directory
- [ ] Create sample JSON files:
  - `simple-email.json`
  - `with-attachment.json`
  - `unicode-content.json`
  - `multiple-recipients.json`
  - `malformed.json` (for error handling tests)
- [ ] Create sample attachment files for tests

### 6.8 CI Testing

- [ ] Add test step to `.github/workflows/build.yml`
  - Run C++ unit tests
  - Run MAPI test harness
  - Run Electron unit tests
- [ ] Create `.github/workflows/test.yml` for PR checks
  - Run full test suite
  - Report coverage
- [ ] Set up Windows runner for integration tests
  - Needs real Windows environment for MAPI testing

## 7. Documentation

- [ ] Update README.md with build instructions once structure is complete
- [ ] Add inline code comments for MAPI struct handling
- [ ] Document JSON schema in `docs/json-schema.md`

---

## Definition of Done (Phase 1)

### Functional
- [ ] Right-click any file in Windows Explorer → "Send to" → "Mail recipient"
- [ ] go-mapi.dll intercepts the call
- [ ] JSON file appears in `%TEMP%\go-mapi\`
- [ ] Electron client detects the file and displays it in UI
- [ ] User can see pending email with subject, body, and attachment list

### Quality Gates
- [ ] All C++ unit tests pass
- [ ] MAPI test harness passes all scenarios
- [ ] All Electron unit tests pass
- [ ] E2E test completes successfully
- [ ] CI pipeline green on main branch
- [ ] No memory leaks in DLL (test with multiple calls)
- [ ] Unicode content works correctly throughout
