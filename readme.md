# Garden of the Azazil — DaemonProc

A peer-to-peer networking daemon providing NAT traversal, cryptographic handshakes, and secure UDP transport.

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.
See the [LICENSE](license.md) file for the full license text.

---

## Code Quality Rules

All contributions **must** comply with the following rules. Pull requests that violate these rules will be **automatically rejected** by the CI pipeline.

### 1. Linting — golangci-lint

Every file must pass `golangci-lint run ./...` with zero issues. The full configuration is defined in [`.golangci.yml`](.golangci.yml).

Below is a summary of the enforced rule categories:

#### 1.1 License Header

Every `.go` file must begin with the following header:

```go
// Copyright (c) <year> Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.
```

**Enforced by:** `goheader`

#### 1.2 Documentation Comments

| Rule | Description | Enforced By |
|------|-------------|-------------|
| All exported types, functions, methods, and variables must have doc comments | Matches Go convention and Effective Go guidelines | `revive (exported)`, `stylecheck (ST1020-ST1023)` |
| Every package must have a package-level comment | Typically placed in a `doc.go` file | `revive (package-comments)`, `stylecheck (ST1000)` |
| All comments must end with a period | Standard Go documentation convention | `godot` |

#### 1.3 Naming Conventions

| Rule | Description | Enforced By |
|------|-------------|-------------|
| Variable and function names must follow Go conventions | `camelCase` for unexported, `PascalCase` for exported | `revive (var-naming)`, `stylecheck (ST1003)` |
| Error types must end with `Error` suffix | e.g., `ConnectionError`, `TimeoutError` | `errname` |
| Error variables must be named `errXxx` | e.g., `errNotFound`, `errTimeout` | `stylecheck (ST1012)` |
| Error strings must start with lowercase | e.g., `"connection refused"` not `"Connection refused"` | `revive (error-strings)`, `stylecheck (ST1005)` |
| Receiver names must be consistent across methods | e.g., always `s` for `*Server`, not mixed | `revive (receiver-naming)`, `stylecheck (ST1016)` |
| Do not shadow predeclared identifiers | e.g., do not use `len`, `cap`, `error` as variable names | `predeclared` |

#### 1.4 Import Rules

Imports must be grouped in the following order, separated by blank lines:

```go
import (
    // Standard library
    "context"
    "fmt"

    // Third-party dependencies
    "golang.org/x/crypto/nacl/box"

    // Project-internal packages
    "github.com/gardenoftheazazil/daemonproc/interfaces"
)
```

**Enforced by:** `gci`, `goimports`

#### 1.5 Line Length

Maximum line length is **120 characters**. Exceptions:
- `package` and `import` statements
- Comments containing long URLs

**Enforced by:** `lll`

#### 1.6 Formatting

- All code must be formatted with `gofmt` (tabs, not spaces).
- Line endings must be **LF** (`\n`), not CRLF.
- No trailing whitespace.
- No consecutive blank lines inside functions.

**Enforced by:** `gofmt`, `whitespace`

#### 1.7 Forbidden Patterns

| Pattern | Reason | Alternative |
|---------|--------|-------------|
| `fmt.Print`, `fmt.Println`, `fmt.Printf` | Not suitable for production logging | Use a structured logger (`slog`, `zerolog`, `zap`) |
| `os.Stdout`, `os.Stderr` | Direct stream writes bypass logging | Use a structured logger |
| `panic()` | Uncontrolled crash | Return an `error` instead |

**Exception:** `main.go` and test files are exempt from `forbidigo`.

**Enforced by:** `forbidigo`

#### 1.8 Error Handling

| Rule | Description | Enforced By |
|------|-------------|-------------|
| All returned errors must be checked | No silently ignoring errors | `errcheck` |
| Use `fmt.Errorf("...: %w", err)` for wrapping | Enables `errors.Is` / `errors.As` chain | `errorlint` |
| Use `errors.Is()` instead of `==` for comparison | Supports wrapped error chains | `errorlint` |
| Do not return nil after checking `err != nil` | Likely a logic bug | `nilerr` |
| Close HTTP response bodies | Prevents resource leaks | `bodyclose` |

#### 1.9 Complexity Limits

| Metric | Threshold | Enforced By |
|--------|-----------|-------------|
| Cognitive complexity | ≤ 30 | `gocognit` |
| Cyclomatic complexity | ≤ 15 | `cyclop` |
| Nested if depth | ≤ 5 | `nestif` |

#### 1.10 Interface Design

Interfaces should follow the **small interface principle**. A single interface must not exceed **10 methods**.

**Enforced by:** `interfacebloat`

#### 1.11 Code Style

| Rule | Description | Enforced By |
|------|-------------|-------------|
| No empty blocks | All blocks must contain at least a comment | `revive (empty-block)` |
| Prefer early return | Avoid deep nesting with `else` | `revive (superfluous-else, early-return)` |
| `context.Context` must be the first parameter | Standard Go convention | `revive (context-as-argument)` |
| Switch on enums must be exhaustive | All cases must be handled | `exhaustive` |
| No unnecessary parentheses or statements | Keep code clean | `gocritic` |
| `nolint` directives require explanation | Must specify which linter and why | `nolintlint` |

### 2. Tests

All tests must pass:

```bash
go test ./... -race -count=1
```

- Tests should use `t.Helper()` in helper functions.
- Tests should use `t.Parallel()` where safe.

**Enforced by:** `thelper`, `tparallel`

### 3. Suppressing Rules

If a rule must be suppressed, use a `//nolint` directive with:
1. The **specific linter name** being suppressed
2. A **reason** explaining why

```go
//nolint:lll // This line contains a long regex pattern that cannot be split.
var pattern = regexp.MustCompile(`...very long pattern...`)
```

Blanket `//nolint` without a linter name or explanation will be rejected.

---

## CI / Merge Requirements

Pull requests to the `main` branch must pass **all** of the following checks before merging:

| Check | Command | Must Pass |
|-------|---------|-----------|
| **Lint** | `golangci-lint run ./...` | ✅ Required |
| **Tests** | `go test ./... -race -count=1` | ✅ Required |

> **Merging is blocked** if any check fails. There are no exceptions or override capabilities for non-admin users.

---

## Development Setup

### Prerequisites

- **Go** 1.26.5 or later
- **golangci-lint** v2.x ([installation guide](https://golangci-lint.run/welcome/install/))

### Running Locally

```bash
# Run linter
golangci-lint run ./...

# Run tests
go test ./... -race -count=1
```
