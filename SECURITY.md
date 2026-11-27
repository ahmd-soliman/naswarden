# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for a security problem.

Report it privately through GitHub: [Report a vulnerability](https://github.com/ahmd-soliman/naswarden/security/advisories/new) (the **Security** tab, then **Report a vulnerability**).

Include:

- the NasWarden version (`/version`) and how you run it (Docker, reverse proxy, which environment variables are set),
- what you found, how to reproduce it, and what an attacker gains,
- a proof of concept if you have one.

**Redact** API keys, hostnames, IP addresses and serial numbers from anything you send. If a real key was exposed while you were testing, revoke it in TrueNAS first.

This is a one-person hobby project. I will try to acknowledge a report within a week and to fix a confirmed problem as soon as I can. Please give me reasonable time before you publish. You will be credited in the release notes unless you prefer not to be.

## Scope

In scope:

- A way to make NasWarden **write** to TrueNAS, Docker or Incus. NasWarden is read-only by design (see [Zero-Mutation Read-Only Architecture](README.md#zero-mutation-read-only-architecture)).
- A leak of the TrueNAS API key, the Incus client certificate, or secrets returned by TrueNAS (for example Cloud Sync credentials) through the UI, the API, `/metrics` or the logs.
- Bypassing the `NASWARDEN_ALLOWED_HOSTS` check, cross-site access to the dashboard or its WebSocket, or injection into the web UI (XSS).
- Vulnerabilities in the published container image that NasWarden's own code or configuration causes.

Not in scope:

- **The dashboard has no login (as of now).** This is documented in the [Security notes](README.md#security-notes): keep it on your LAN or behind an authenticating reverse proxy. Until a login exists, reports that "an unauthenticated user can see the dashboard" will be closed. Once authentication is added, bypassing it will be in scope and this line will be removed.
- A key, role or Docker socket that you gave NasWarden more rights than the README recommends.
- `TRUENAS_INSECURE_TLS=true` accepting a self-signed certificate. That is what the option does.
- Issues in a dependency, or scanner output, without a demonstration that NasWarden is affected.
- Denial of service by someone who already has network access to the dashboard.

## Supported versions

Only the latest release receives fixes. Upgrade to the newest `vX.Y.Z` tag (or `latest`) before reporting.
