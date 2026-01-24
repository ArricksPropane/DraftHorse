# Contributing to go-mapi

Thank you for your interest in contributing to go-mapi! This guide will help you get started.

## Code of Conduct

Please be respectful and inclusive in all interactions.

## Getting Started

### Prerequisites

- Windows 10/11
- Visual Studio 2022 with C++17 support
- Node.js 18+
- CMake 3.20+
- Git

### Setup Development Environment

```bash
# Clone the repository
git clone https://github.com/yourusername/go-mapi.git
cd go-mapi

# Build C++ components
.\build.ps1 -Config Debug -Platform x64 -Tests

# Install and test Electron client
cd src/client
npm install
npm test
```

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

### 2. Make Your Changes

Follow the guidelines below for code style and structure.

### 3. Write Tests

Always include tests for new functionality:

**For C++**:
- Add test to `src/test-harness/src/test_*.cpp`
- Update `test_utils.h` if adding new test utilities
- Ensure tests pass: `build\bin\go-mapi-test-harness.exe`

**For TypeScript**:
- Add test to `src/client/src/__tests__/*.test.ts`
- Ensure tests pass: `npm test`
- Aim for > 80% code coverage

### 4. Run Full Test Suite

```powershell
# Build and run all tests
.\build.ps1 -Config Debug -Platform x64 -Tests

# TypeScript tests
cd src/client
npm test
```

### 5. Commit Your Changes

```bash
git add .
git commit -m "feat: add feature description

- Detailed explanation of what was added
- Why this change was necessary
- Any breaking changes or gotchas"
```

**Commit Message Format**:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation
- `test:` - Tests
- `refactor:` - Code refactoring
- `perf:` - Performance improvement
- `chore:` - Build/tooling changes

### 6. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub. Include:
- Description of changes
- Why this change is needed
- How to test the changes
- Any related issues

## Code Style Guide

### C++

- **Standard**: C++17
- **Naming**: 
  - Classes: `PascalCase` (e.g., `MapiImpl`)
  - Functions: `PascalCase` (e.g., `WriteMailToFile`)
  - Constants: `UPPER_SNAKE_CASE` (e.g., `MAPI_TO`)
  - Variables: `camelCase` (e.g., `recipCount`)
- **Spacing**: 4 spaces (no tabs)
- **Comments**: Use `//` for single-line, `/* */` for multi-line

**Example**:
```cpp
class FileWriter {
public:
    // Write data to file with UTF-8 encoding
    static bool WriteFile(const std::wstring& filePath, const std::string& content);

private:
    // Helper to ensure directory exists
    static bool EnsureDirectory(const std::wstring& dirPath);
};
```

### TypeScript

- **Lint**: ESLint (configured via tsconfig)
- **Naming**:
  - Classes: `PascalCase` (e.g., `MailQueue`)
  - Functions: `camelCase` (e.g., `getQueue`)
  - Constants: `UPPER_SNAKE_CASE` (e.g., `MAX_MESSAGE_SIZE`)
  - Variables: `camelCase` (e.g., `messageCount`)
  - Interfaces: `PascalCase` with `I` prefix optional (e.g., `MailMessage`)
- **Spacing**: 2 spaces
- **Semicolons**: Required
- **Strict Mode**: Always enabled

**Example**:
```typescript
interface MailMessage {
  id: string;
  subject: string;
  // ...
}

class MailQueue {
  private queue: Map<string, MailMessage> = new Map();

  /**
   * Add a message to the queue
   */
  add(message: MailMessage): void {
    this.queue.set(message.id, message);
  }
}
```

### Comments & Documentation

**C++**:
```cpp
// Brief description
unsigned long MAPISendMail(...) {
    // Implementation
}
```

**TypeScript**:
```typescript
/**
 * Brief description with more detail
 * @param message The message to send
 * @returns True if successful
 */
function sendMessage(message: MailMessage): boolean {
  // Implementation
}
```

## Testing Requirements

### For C++ Changes
- New code must have unit tests
- All tests must pass locally
- `test_harness` should exit with code 0

### For TypeScript Changes
- New code must have unit tests
- Minimum 80% code coverage
- All tests must pass: `npm test`

### Test Writing Tips
- Use descriptive test names: `test("should add message to queue")`
- Test both success and failure cases
- Use fixtures for test data
- Clean up after tests (delete temp files)

## Documentation

### When to Update Docs

- Adding a new feature → Update README.md
- Changing architecture → Update ARCHITECTURE.md
- Adding tests → Update TESTING.md
- Adding build steps → Update README.md or build.ps1

### Documentation Standards

- Keep explanations clear and concise
- Include code examples where helpful
- Link to relevant references
- Update table of contents if adding sections

## Commit Best Practices

1. **Atomic commits**: Each commit should be a single logical change
2. **Test before commit**: Ensure all tests pass
3. **Meaningful messages**: Describe WHAT and WHY, not HOW
4. **Reference issues**: Use "Fixes #123" in commit body

**Good commit message**:
```
feat: add Unicode support to JSON serialization

Properly escape Unicode characters in JSON output to support
international character sets in email subjects and bodies.

Fixes #42
```

**Bad commit message**:
```
fix stuff
```

## Pull Request Process

1. Ensure all tests pass
2. Update documentation if needed
3. Add tests for new code
4. Request review from maintainers
5. Address feedback promptly
6. Ensure clean commit history (rebase if needed)

## Areas for Contribution

### High Priority
- [ ] Gmail API integration (Phase 2)
- [ ] OAuth2 flow implementation
- [ ] Error handling improvements
- [ ] Performance optimization

### Medium Priority
- [ ] Additional MAPI function support
- [ ] UI/UX improvements
- [ ] Configuration management
- [ ] Logging system

### Lower Priority
- [ ] Documentation improvements
- [ ] Code refactoring
- [ ] Build system optimization
- [ ] Test coverage expansion

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Bugs**: Open an Issue with reproduction steps
- **Ideas**: Start a Discussion or open an Issue

## License

By contributing, you agree that your contributions will be licensed under the project's license (TBD).

## Recognition

Contributors will be recognized in:
- CONTRIBUTORS.md
- GitHub contributors page
- Release notes

Thank you for helping make go-mapi better! 🚀

