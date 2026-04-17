# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |

## Reporting a Vulnerability

If you discover a security vulnerability in krit, please report it responsibly:

1. **Do NOT open a public GitHub issue**
2. Email security concerns to jason.d.pearson@gmail.com
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
4. You will receive a response within 48 hours

## Scope

Security issues in these areas are in scope:
- **Binary distribution**: compromised releases, missing checksums
- **Code execution**: rules that could execute arbitrary code
- **Path traversal**: file access outside the intended scope
- **Denial of service**: inputs that cause excessive resource consumption

## Out of Scope

- False positives/negatives in lint rules (use bug reports)
- Performance issues (use feature requests)
- Issues in third-party dependencies (report upstream)
