# Security Policy

## Supported Versions

| Version | Supported |
|---------|:---------:|
| Latest release | ✅ |
| Previous release | ✅ |
| Older versions | ❌ |

## Reporting a Vulnerability

**Please do NOT open a public issue for security vulnerabilities.**

Instead, report security issues privately via one of the following:

- **Email:** security@starclaw.me
- **GitHub Security Advisories:** [Report a vulnerability](https://github.com/yinhe/starclaw/security/advisories/new)

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response timeline

- **Acknowledgment:** within 48 hours
- **Initial assessment:** within 1 week
- **Fix release:** within 2 weeks for critical issues

## Security Features

StarClaw includes built-in security features:

- **AES-256-GCM encryption** for sensitive data at rest
- **Ed25519 signature authentication** for inter-node communication
- **Merkle-linked audit chain** for tamper-evident logging
- **JWT authentication** with configurable secret rotation
- **RBAC** role-based access control
- **Sandbox execution** for user code with resource limits
- **Input validation** on all API endpoints

## Best Practices for Deployment

1. **Always change default secrets** — Set unique `JWT_SECRET`, database passwords, and `STARCLAW_MASTER_KEY`
2. **Use HTTPS** — Never expose the API over plain HTTP in production
3. **Restrict network access** — Bind ports to `127.0.0.1` and use a reverse proxy
4. **Keep updated** — Enable Molt auto-update checks or watch GitHub releases
5. **Review API keys** — Use BYOK (Bring Your Own Key) and rotate regularly
