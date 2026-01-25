# Contributing to go-mapi

Thank you for your interest in contributing to go-mapi!

## Code of Conduct

Please be respectful and inclusive in all interactions.

## Getting Started

### Prerequisites

- Windows 10/11
- MinGW toolchain (GCC 13+), CMake, Ninja
- Go 1.21+
- Node.js 18+
- Git

**Install build tools via [Scoop](https://scoop.sh):**
```powershell
scoop install mingw-winlibs-ucrt cmake ninja go nodejs
```

### Setup Development Environment

```powershell
# Clone the repository
git clone https://github.com/anthropics/go-mapi.git
cd go-mapi

# Build all components
npm run build

# Or build individually:
npm run build:interceptor      # C++ DLL
npm run build:native-host      # Go binary
npm run build:extension        # Browser extension
```

### Project Components

| Component | Language | Location | Build Command |
|-----------|----------|----------|---------------|
| Interceptor DLL | C++ (MinGW) | `src/interceptor/` | `npm run build:interceptor` |
| Native Host | Go | `src/native-host/` | `npm run build:native-host` |
| Browser Extension | TypeScript/React | `src/extension/` | `npm run build:extension` |

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

**For C++**:
- Add tests to `src/interceptor/test-harness/src/`
- Run: `npm run test:interceptor`

**For TypeScript/Extension**:
- Add tests alongside source files
- Run: `npm run --prefix src/extension test`

### 4. Commit Your Changes

```bash
git add .
git commit -m "feat: add feature description"
```

**Commit prefixes**: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`

### 5. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

## Code Style Guide

### C++ (Interceptor)

- **Standard**: C++17
- **Compiler**: MinGW GCC
- **Naming**: `PascalCase` for classes/functions, `camelCase` for variables
- **Spacing**: 4 spaces (no tabs)

### Go (Native Host)

- **Standard**: Go 1.21+
- **Naming**: Follow Go conventions (`PascalCase` for exports, `camelCase` for private)
- **Format**: Use `gofmt`

### TypeScript (Extension)

- **Naming**: `PascalCase` for components/types, `camelCase` for functions/variables
- **Spacing**: 2 spaces
- **Semicolons**: Required
- **Strict Mode**: Always enabled

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
- [ ] Documentation improvements
- [ ] Performance optimization

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Bugs**: Open an Issue with reproduction steps
- **Ideas**: Start a Discussion or open an Issue

## License

By contributing, you agree that your contributions will be licensed under the MIT license.
