# Security Policy

## Supported versions

The latest released version receives security fixes. Please upgrade to the most
recent release before reporting an issue.

## Reporting a vulnerability

**Please do not open a public issue for security vulnerabilities.**

Instead, report privately using GitHub's
[private vulnerability reporting](https://github.com/yash5155/apic/security/advisories/new)
("Report a vulnerability" under the repository's **Security** tab).

Please include:

- A description of the vulnerability and its impact
- Steps to reproduce (a spec file or command line, if relevant)
- The version of `apic` and your OS/architecture

We aim to acknowledge reports within a few days and to release a fix as quickly
as is practical. We'll credit you in the release notes unless you prefer to
remain anonymous.

## Scope and notes

`apic` is a client that sends requests you construct. Some security-relevant
behaviour is documented in
[docs/DOCUMENTATION.md §11](docs/DOCUMENTATION.md#11-security-model), including:

- TLS verification is on by default (`--insecure` opts out).
- `Authorization`/`Cookie` headers are stripped on cross-host redirects.
- External `$ref`s in remote specs are not fetched over the network (SSRF-safe).
- Spec and response sizes are bounded.

Tokens and secrets you pass via `-H` or type into the form are sent to the host
you target and may appear in saved response files — handle them accordingly.
