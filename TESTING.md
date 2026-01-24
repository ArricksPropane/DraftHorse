# go-mapi Testing Guide

This document describes the testing strategy for the go-mapi project, covering C++ unit tests, integration tests, and end-to-end tests.

## Test Overview

```
Testing Pyramid
───────────────
        ▲
       / \
      /E2E\           End-to-End Tests (full flow)
     /─────\
    / Integ \         Integration Tests
   /─────────\
  / Unit/Syst \       Unit Tests + System Tests
 /─────────────\
```

## C++ Unit Tests

### Building Tests

```bash
# Build with tests enabled
.\build.ps1 -Config Debug -Platform x64 -Tests

# Or manually
cmake -S . -B build -DBUILD_TESTS=ON
cmake --build build
```

### Test Structure

- **Location**: `src/test-harness/`
- **Framework**: Custom test harness (no external dependencies)
- **Tests**:
  - `test_simple_send.cpp` - Basic email sending
  - `test_with_attachments.cpp` - File attachment handling
  - `test_unicode.cpp` - Unicode character support
  - `test_multiple_recipients.cpp` - TO/CC/BCC recipients

### Running C++ Tests

```bash
# Run all tests
build\bin\go-mapi-test-harness.exe

# Tests verify:
# 1. DLL loads successfully
# 2. MAPISendMail function is exported
# 3. JSON files are created in %TEMP%\go-mapi\
# 4. JSON content is valid and contains required fields
# 5. Unicode and special characters are preserved
```

### Test Utilities

The `test_utils.h/cpp` library provides:

- `LoadMAPISendMail()` - Dynamic DLL loading
- `VerifyJsonFileCreated()` - Check for output files
- `ValidateJsonFile()` - Parse and validate JSON structure
- `CleanupTestFiles()` - Test cleanup
- `GetGoMapiTempDir()` - Platform-independent temp path

## Electron TypeScript Tests

### Running Tests

```bash
cd src/client

# Install dependencies
npm install

# Run tests once
npm test

# Run tests in watch mode
npm test:watch

# Run with coverage
npm test -- --coverage
```

### Test Files

Located in `src/client/src/__tests__/`:

#### `mail-queue.test.ts`
Tests the in-memory email queue:
- Adding messages
- Removing messages
- Retrieving messages by ID
- Getting all messages
- Clearing the queue
- Listener notifications
- Unsubscribe functionality

#### `json-parser.test.ts`
Tests JSON parsing and validation:
- Parsing valid JSON messages
- Rejecting invalid JSON
- Schema validation
- Required field checking
- Type checking
- Multiple recipients handling
- Attachment handling
- Unique ID generation

### Test Fixtures

Located in `tests/fixtures/`:

- `simple-email.json` - Basic email
- `with-attachment.json` - Single attachment
- `unicode-content.json` - Unicode characters
- `multiple-recipients.json` - TO/CC/BCC recipients
- `malformed.json` - Invalid JSON (for error testing)

## Integration Tests

### File Watcher Integration

Tests the file watching and message processing pipeline:

```bash
cd src/client
npm test -- watcher-integration
```

**Test Scenarios**:
1. Drop JSON file into watch directory
2. Verify file is processed
3. Verify message appears in queue
4. Verify file is moved to `processed/` directory
5. Verify error handling for malformed files

### IPC Integration

Tests main-to-renderer communication:

```bash
npm test -- ipc-integration
```

**Test Scenarios**:
1. Queue updates sent from main to renderer
2. IPC handlers return correct data
3. Error handling in IPC calls

## End-to-End Tests

### Full Flow Testing

```bash
npm test -- e2e
```

**Full Flow Test**:
1. Start Electron client
2. Verify file watcher is active
3. Run C++ test harness (MAPISendMail call)
4. Verify JSON file created
5. Wait for file watcher to detect it
6. Verify message appears in client UI
7. Verify all fields match source data

**Requirements**:
- Electron app must start and exit cleanly
- DLL must be in PATH or built in build directory
- Temp directory must be writable

### Stress Testing

```bash
npm test -- e2e --stress
```

**Stress Test**:
1. Send 100 rapid MAPISendMail calls
2. Verify all messages captured
3. Verify no race conditions
4. Verify memory usage remains stable
5. Verify all attachments handled correctly

## CI/CD Testing

### GitHub Actions

Automated testing runs on every push and pull request:

```yaml
# .github/workflows/build.yml
- Build C++ with tests
- Run test harness
- Build Electron client
- Run TypeScript unit tests
```

### Test Coverage

Target coverage:
- **C++ Interceptor**: 80%+ (MAPI functions, JSON serialization, file I/O)
- **TypeScript Client**: 85%+ (queue, parser, watcher, API integration)

## Testing Best Practices

1. **Isolation**: Tests don't depend on each other
2. **Cleanup**: Always clean up files in `%TEMP%\go-mapi\`
3. **Mocking**: Mock file system and IPC in unit tests
4. **Determinism**: Tests produce same results every run
5. **Speed**: Unit tests complete in < 1 second total

## Debugging Tests

### C++ Tests

Enable debug output:

```powershell
$env:GO_MAPI_DEBUG=1
build\bin\go-mapi-test-harness.exe
```

### TypeScript Tests

Run with verbose output:

```bash
npm test -- --verbose
```

Debug specific test file:

```bash
npm test -- mail-queue.test.ts
```

## Performance Testing

Monitor performance metrics:

```bash
# Memory usage during stress test
npm test -- e2e --stress --memory-profile

# File watcher latency
npm test -- watcher-integration --timing
```

## Known Limitations

1. **Unicode in ANSI MAPI**: Limited to UTF-8 approximations in test (real MAPI clients may use different encodings)
2. **Large Attachments**: Tests use dummy paths; don't test actual file transfer
3. **Gmail API**: OAuth tokens mocked in unit tests; use integration tests for real API calls

## Future Testing Plans

- [ ] Performance benchmarking suite
- [ ] Automated installer testing
- [ ] Real Gmail API integration tests
- [ ] Multi-instance testing (multiple Electron windows)
- [ ] Network interruption simulation
- [ ] Memory leak detection

## Contributing Tests

When adding new features:

1. Write tests first (TDD approach)
2. Ensure tests pass locally
3. Update this document if adding new test categories
4. Aim for > 80% code coverage
5. Include both positive and negative test cases

