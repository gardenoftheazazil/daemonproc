# Garden of the Azazil — DaemonProc

[![CI](https://github.com/gardenoftheazazil/daemonproc/actions/workflows/ci.yml/badge.svg)](https://github.com/gardenoftheazazil/daemonproc/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/gardenoftheazazil/daemonproc)](https://go.dev/)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/gardenoftheazazil/daemonproc)](https://goreportcard.com/report/github.com/gardenoftheazazil/daemonproc)

**Serverless, decentralized, end-to-end encrypted peer-to-peer communication daemon.**

DaemonProc is the core networking daemon of the **Garden of the Azazil (GOTA)** ecosystem. It enables applications to communicate directly with each other across the internet — without any central server — using NAT traversal, cryptographic handshakes, and end-to-end encryption.

---

## Vision

We're building a standard for **app-to-app P2P communication** that is:

- 🔒 **End-to-End Encrypted** — No one can read your data, not even us
- 🌐 **Serverless** — No central server, no single point of failure
- ⚡ **Fast** — Direct peer connections via UDP with NAT traversal
- 🧩 **App-Agnostic** — Any application can use GOTA through a standard library

### Future Roadmap

The long-term mission extends beyond simple P2P:

```
Phase 1 (Current)     Phase 2              Phase 3
─────────────────     ─────────────        ─────────────
P2P E2EE              Mesh Network         Distributed
App-to-App    ───►    TURN/Relay    ───►   Infrastructure
Communication         Fallback             & Protocol Std
```

---

## Architecture

DaemonProc is one component in the GOTA ecosystem. Applications never touch the network directly — they communicate through a local daemon via Unix sockets.

```
    Machine A                                        Machine B
┌──────────────────┐                          ┌──────────────────┐
│                  │                          │                  │
│  ┌────────────┐  │                          │  ┌────────────┐  │
│  │  Your App  │  │                          │  │  Peer App  │  │
│  │            │  │                          │  │            │  │
│  └─────┬──────┘  │                          │  └─────┬──────┘  │
│        │         │                          │        │         │
│   Unix Socket    │                          │   Unix Socket    │
│   (libgota)      │                          │   (libgota)      │
│        │         │                          │        │         │
│  ┌─────▼──────┐  │    UDP / P2P / E2EE     │  ┌─────▼──────┐  │
│  │ DaemonProc │◄─┼────────────────────────►─┤  │ DaemonProc │  │
│  │            │  │  NAT Punch + Handshake   │  │            │  │
│  └────────────┘  │  Noise Protocol IKpsk2   │  └────────────┘  │
│                  │                          │                  │
└──────────────────┘                          └──────────────────┘
```

### Components

| Component | Repository | Description |
|-----------|-----------|-------------|
| **DaemonProc** | This repo | Background daemon handling P2P networking, NAT traversal, and E2EE |
| **libgota** | *(coming soon)* | Client library (`.dll` / `.so`) that applications link against for IPC |

### How It Works

1. **Your app** links against `libgota` and writes data to a Unix socket
2. **DaemonProc** reads from the socket, encrypts the data, and sends it over UDP
3. **NAT Punching** establishes a direct connection between peers
4. **Noise Protocol (IKpsk2)** performs the cryptographic handshake
5. **ChaCha20-Poly1305** encrypts all traffic end-to-end
6. **Remote DaemonProc** decrypts and delivers to the peer app via its Unix socket

---

## Getting Started

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26.5+ | [go.dev/dl](https://go.dev/dl/) |
| golangci-lint | v2.x | [golangci-lint.run](https://golangci-lint.run/welcome/install/) |
| GNU Make | Any | Pre-installed on Linux/macOS; [Windows](https://gnuwin32.sourceforge.net/packages/make.htm) |

### Quick Start

```bash
# Clone
git clone https://github.com/gardenoftheazazil/daemonproc.git
cd daemonproc

# Install dependencies
make deps

# Run all checks
make check

# See all available commands
make help
```

### Available Make Targets

```
  fmt             Format all Go source files.
  lint            Run golangci-lint on all packages.
  vet             Run go vet on all packages.
  test            Run all tests with race detector.
  test-short      Run tests in short mode.
  cover           Run tests with coverage report.
  build           Build the daemon binary.
  deps            Download and tidy Go module dependencies.
  deps-upgrade    Upgrade all dependencies.
  check           Run all quality checks (CI equivalent).
  clean           Remove build artifacts and coverage files.
```

---

## Code Quality Rules

All contributions **must** comply with the following rules. Pull requests that violate these rules will be **automatically rejected** by the CI pipeline.

### Linting — golangci-lint

Every file must pass `golangci-lint run ./...` with zero issues. The full configuration is defined in [`.golangci.yml`](.golangci.yml).

<details>
<summary><strong>📋 Click to expand: Full rule breakdown</strong></summary>

#### License Header

Every `.go` file must begin with:

```go
// Copyright (c) <year> Garden of the Azazil. All rights reserved.
// Licensed under the AGPL-3.0 License.
// See LICENSE file in the project root for full license information.
```

#### Documentation Comments

| Rule | Enforced By |
|------|-------------|
| All exported symbols must have doc comments | `revive`, `stylecheck` |
| Every package must have a package comment | `revive`, `stylecheck` |
| All comments must end with a period | `godot` |

#### Naming Conventions

| Rule | Enforced By |
|------|-------------|
| Go naming conventions (`camelCase` / `PascalCase`) | `revive`, `stylecheck` |
| Error types must end with `Error` suffix | `errname` |
| Error variables must be named `errXxx` | `stylecheck` |
| Error strings must start with lowercase | `revive`, `stylecheck` |
| Consistent receiver names | `revive`, `stylecheck` |
| No shadowing predeclared identifiers | `predeclared` |

#### Import Ordering

```go
import (
    "context"                                          // stdlib
    "golang.org/x/crypto/nacl/box"                     // third-party
    "github.com/gardenoftheazazil/daemonproc/interfaces" // project
)
```

#### Formatting & Style

| Rule | Details |
|------|---------|
| Max line length | 120 characters |
| Formatting | `gofmt` (tabs) |
| Line endings | LF only |
| No `fmt.Print*` in production | Use structured logger |
| No `panic()` | Return errors instead |

#### Error Handling

| Rule | Enforced By |
|------|-------------|
| All errors must be checked | `errcheck` |
| Wrap errors with `%w` | `errorlint` |
| Use `errors.Is()` for comparison | `errorlint` |
| Close HTTP response bodies | `bodyclose` |

#### Complexity Limits

| Metric | Threshold |
|--------|-----------|
| Cognitive complexity | ≤ 30 |
| Cyclomatic complexity | ≤ 15 |
| Nested if depth | ≤ 5 |

</details>

### Tests

```bash
go test ./... -race -count=1
```

### Suppressing Rules

```go
//nolint:lll // This line contains a long regex pattern that cannot be split.
var pattern = regexp.MustCompile(`...very long pattern...`)
```

---

## CI / Merge Requirements

| Check | Command | Required |
|-------|---------|----------|
| **Lint** | `golangci-lint run ./...` | ✅ |
| **Tests** | `go test ./... -race -count=1` | ✅ |
| **Review** | 1 approving review | ✅ |

> **Merging is blocked** if any check fails.

---

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) before submitting a pull request.

- [Bug Report](https://github.com/gardenoftheazazil/daemonproc/issues/new?template=bug_report.yml)
- [Feature Request](https://github.com/gardenoftheazazil/daemonproc/issues/new?template=feature_request.yml)

## Security

For security vulnerabilities, please see our [Security Policy](SECURITY.md).
**Do NOT report security issues through public GitHub issues.**

## Code of Conduct

This project follows the [Contributor Covenant v2.1](CODE_OF_CONDUCT.md).

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.
See [LICENSE](license.md) for the full text.
