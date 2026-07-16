# Contributing to Garden of the Azazil — DaemonProc

Thank you for your interest in contributing to DaemonProc! This document provides
guidelines and instructions for contributing to this project.

## Table of Contents

- [Project Overview](#project-overview)
- [Development Setup](#development-setup)
- [Development Workflow](#development-workflow)
- [Commit Convention](#commit-convention)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Reporting Bugs](#reporting-bugs)
- [Suggesting Features](#suggesting-features)

---

## Project Overview

DaemonProc is the background daemon component of the **Garden of the Azazil (GOTA)**
ecosystem. It manages peer-to-peer, end-to-end encrypted (E2EE) communication between
applications on different machines.

### Architecture

```
┌─────────────┐    Unix Socket    ┌─────────────┐    UDP/P2P    ┌─────────────┐
│  Your App   │◄────────────────►│ DaemonProc  │◄────────────►│ DaemonProc  │
│  (uses      │   IPC via        │ (this repo) │  NAT Punch   │ (remote     │
│  libgota)   │   libgota.dll    │             │  + E2EE      │  peer)      │
└─────────────┘                  └─────────────┘              └─────────────┘
```

**Related projects:**
- **libgota** — Client library (`.dll` / `.so`) that applications link against
- **daemonproc** — This project; the always-running daemon that handles networking

---

## Development Setup

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://go.dev/dl/) | 1.26.5+ | Language runtime |
| [golangci-lint](https://golangci-lint.run/welcome/install/) | v2.x | Linter suite |
| [GNU Make](https://www.gnu.org/software/make/) | Any | Build automation |
| [Git](https://git-scm.com/) | 2.x+ | Version control |

### Getting Started

```bash
# 1. Fork the repository on GitHub

# 2. Clone your fork
git clone https://github.com/<your-username>/daemonproc.git
cd daemonproc

# 3. Add upstream remote
git remote add upstream https://github.com/gardenoftheazazil/daemonproc.git

# 4. Verify your setup
make check
```

---

## Development Workflow

1. **Sync with upstream:**
   ```bash
   git fetch upstream
   git checkout main
   git merge upstream/main
   ```

2. **Create a feature branch:**
   ```bash
   git checkout -b feat/my-feature
   ```

3. **Make your changes** and ensure they pass all checks:
   ```bash
   make lint    # Run linter
   make test    # Run tests
   make fmt     # Format code
   ```

4. **Commit your changes** following the [commit convention](#commit-convention).

5. **Push and create a PR:**
   ```bash
   git push origin feat/my-feature
   ```

---

## Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation only changes |
| `style` | Formatting, missing semicolons, etc. (no code change) |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `perf` | Performance improvement |
| `test` | Adding or correcting tests |
| `build` | Changes to build system or external dependencies |
| `ci` | Changes to CI configuration |
| `chore` | Other changes that don't modify src or test files |

### Examples

```
feat(handshake): add Noise protocol IKpsk2 handshake implementation
fix(natpunch): resolve STUN response timeout on symmetric NAT
docs(readme): add architecture diagram
test(udp): add packet fragmentation edge case tests
```

---

## Pull Request Process

1. **Before submitting:**
   - [ ] Code compiles without errors (`make build`)
   - [ ] All tests pass (`make test`)
   - [ ] Linter reports zero issues (`make lint`)
   - [ ] New code has appropriate doc comments
   - [ ] Commit messages follow the convention

2. **PR title** must follow the commit convention format.

3. **PR description** must explain:
   - What the change does
   - Why it's needed
   - How it was tested

4. **Review process:**
   - At least 1 approving review is required
   - CI checks (Lint + Test) must pass
   - Stale reviews are dismissed on new pushes

5. **Merging:**
   - Squash merge is preferred for single-purpose PRs
   - Rebase merge for multi-commit PRs where history matters

---

## Code Style

All code must comply with the project's [`.golangci.yml`](.golangci.yml) configuration.
See the [Code Quality Rules](readme.md#code-quality-rules) section in the README for
a detailed breakdown.

Key highlights:
- **Line length:** 120 characters max
- **Formatting:** `gofmt` (tabs, not spaces)
- **Line endings:** LF only (enforced by `.gitattributes`)
- **Comments:** All exported symbols must have doc comments ending with a period
- **Errors:** Always check, always wrap with `%w`, use `errors.Is()` for comparison
- **Logging:** No `fmt.Print*` in production code; use a structured logger
- **Imports:** Grouped as stdlib → third-party → project-internal

---

## Reporting Bugs

Use the [Bug Report](https://github.com/gardenoftheazazil/daemonproc/issues/new?template=bug_report.yml)
issue template. Include:

- Steps to reproduce
- Expected vs. actual behavior
- Go version and OS
- Relevant logs or error messages

---

## Suggesting Features

Use the [Feature Request](https://github.com/gardenoftheazazil/daemonproc/issues/new?template=feature_request.yml)
issue template. Please describe:

- The problem you're trying to solve
- Your proposed solution
- Any alternatives you've considered

---

## License

By contributing to this project, you agree that your contributions will be licensed
under the [GNU Affero General Public License v3.0](license.md).
