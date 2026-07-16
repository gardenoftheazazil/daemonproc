# Security Policy

## Reporting a Vulnerability

The Garden of the Azazil team takes security vulnerabilities seriously — especially
given that DaemonProc handles **end-to-end encrypted P2P communication** and
**cryptographic handshakes**.

**⚠️ Please do NOT report security vulnerabilities through public GitHub issues.**

### How to Report

1. **GitHub Private Vulnerability Reporting (Preferred):**
   Go to [Security Advisories](https://github.com/gardenoftheazazil/daemonproc/security/advisories/new)
   and create a new private security advisory.

2. **Email:**
   Send a detailed report to **security@gardenoftheazazil.dev**.

### What to Include

- **Description** of the vulnerability
- **Steps to reproduce** or a proof-of-concept
- **Impact assessment** — what an attacker could achieve
- **Affected component** (e.g., handshake, NAT punch, IPC, encryption)
- **Suggested fix** (if any)

### What to Expect

| Timeline | Action |
|----------|--------|
| **24 hours** | Acknowledgment of your report |
| **72 hours** | Initial assessment and severity classification |
| **7 days** | Remediation plan communicated to you |
| **30 days** | Fix released (for critical/high severity) |

### Severity Classification

We use the [CVSS v3.1](https://www.first.org/cvss/v3.1/specification-document) scoring
system:

| Severity | CVSS Score | Example |
|----------|-----------|---------|
| **Critical** | 9.0–10.0 | Remote code execution, key extraction |
| **High** | 7.0–8.9 | Authentication bypass, E2EE downgrade |
| **Medium** | 4.0–6.9 | Information disclosure, DoS |
| **Low** | 0.1–3.9 | Minor information leak, timing side-channel |

## Scope

The following components are **in scope** for security reports:

- Cryptographic handshake implementation (Noise protocol)
- End-to-end encryption (key exchange, symmetric encryption)
- NAT traversal and UDP transport
- Unix socket IPC between applications and the daemon
- Authentication and peer identity verification
- Memory safety issues in Go code

The following are **out of scope**:

- Vulnerabilities in third-party dependencies (report upstream instead)
- Social engineering attacks
- Denial of service via network flooding (expected limitation of P2P)
- Issues in the `libgota` client library (separate project)

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest `main` | ✅ Yes |
| Older releases | ❌ No (upgrade to latest) |

## Recognition

We believe in recognizing security researchers who help keep our users safe.
With your permission, we will:

- Credit you in the security advisory
- Credit you in the CHANGELOG
- Add you to a `SECURITY_HALL_OF_FAME.md` (when applicable)

## Disclosure Policy

We follow **coordinated disclosure**:

1. Reporter submits the vulnerability privately
2. We confirm, assess, and develop a fix
3. We release the fix and publish a security advisory
4. Reporter is free to publish their findings after the advisory is public
