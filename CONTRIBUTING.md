# Contributing to NasWarden

Thanks for helping. NasWarden is a small, **read-only** dashboard for TrueNAS (see [Zero-Mutation Read-Only Architecture](README.md#zero-mutation-read-only-architecture)). Contributions that keep it that way are welcome: bug reports, fixes, new read-only panels and metrics.

## Before you start

- Open an issue first for anything bigger than a small fix, so we can agree on the scope.
- **Read-only is a hard rule.** NasWarden may only send read queries to TrueNAS, Docker and Incus. A change that writes, starts, stops or deletes anything will not be merged.
- **What belongs at `/metrics`:** values only the TrueNAS API provides (ZFS errors, scrub state, snapshot space, task status). Host and container CPU/memory are left to node-exporter and cAdvisor.

## Set up

You need Go (the version in `go.mod`), and Node 20 or newer for `web/`.

```bash
# Backend, against your own TrueNAS
export TRUENAS_HOST="truenas.local:8443" TRUENAS_API_KEY="..." TRUENAS_INSECURE_TLS=true
go run ./cmd/naswarden

# Or build, run and check it end to end (keys come from the environment only)
scripts/local-test.sh

# Frontend
cd web && npm install && npm run build
```

Use the API key of a user with the **Readonly Admin** role, not an admin key. Never put a key in a file, a command line you share, a commit or an issue.

## Checks to run before opening a PR

```bash
gofmt -l .                # must print nothing
go vet ./...
go test -race ./...
golangci-lint run ./...   # config in .golangci.yml; CI runs it too

# only if you touched web/
cd web && npm run lint && npm test && npm run build
```

`internal/web/dist` is **committed**. If you change `web/`, run `npm run build` and commit the result; CI fails when it is out of date. If you change the UI visibly, `npm run screenshots` in `web/` refreshes the README screenshots (demo data, no real system).

## Working on the UI

The UI has a demo mode: a made-up TrueNAS ([`web/scripts/demo-state.mjs`](web/scripts/demo-state.mjs)), so you need no real system and no Go backend. Run these in `web/`:

| Command | What it does |
| --- | --- |
| `npm run demo` | Starts a local server at <http://localhost:5199> with demo data and keeps running (Ctrl+C to stop). Open it in any browser to look at your change, in both themes and at phone and tablet widths. |
| `npm run screenshots` | Refreshes the README screenshots from the same demo server. |

Chromium is needed once for the screenshots: `npx playwright install --only-shell chromium`. Both commands share one fake backend, [`web/scripts/demo-server.mjs`](web/scripts/demo-server.mjs); extend that rather than adding mocks to a script.

## Tests come from real responses

TrueNAS replies differ from the documentation more often than you would expect (fields that are `null`, numbers sent as strings, values above 100%). So:

1. Record the real reply with a **read-only** call, for example `midclt call pool.query` on the NAS.
2. Build the test fixture from it. **Redact** hostnames, serial numbers, IPs and any other identifying values.
3. Never record or commit calls that return secrets inline (for example Cloud Sync credentials). Do not paste them into issues either.
4. Say in the PR what you checked on a real box and what you could not.

Cover three cases: value present, value missing, and the thing disappearing.

## Adding a metric

Follow the pattern in `internal/metrics/export.go`:

- A `GaugeVec`, a `tracked` series, registration in `init()`, an `Update*` function called from `refresh()` in `cmd/naswarden/main.go`. Metrics come from the same refresh as the UI; do not add extra TrueNAS calls unless you must.
- Names are `naswarden_*` with base units (`_bytes`, `_seconds`, `_percent`).
- **Leave the series out** when the value is missing or stale. Never export 0 for "unknown".
- Labels must be low-cardinality: no IP addresses, container IDs, command lines, task names or paths.
- Add a test that also checks the metric is exposed on the default gatherer (see `TestPoolUsableMetricsAreExposedOnTheRegistry`).
- Add the metric to the table in the README. Add a panel to `deploy/grafana/naswarden.dashboard.json` and, where it makes sense, a rule to `deploy/prometheus/naswarden.rules.yml`.
- The ZFS error counts are gauges: alert with `delta()`, not `increase()`.
- Check rule changes with `promtool check rules` and say in the PR whether you did.

## Pull requests

- One topic per PR, from a branch off `main`. Write "Part of #N" or "Closes #N" in the description.
- Commit messages: a short imperative subject, then why. Keep generated trailers out of them.
- Use `git add -A cmd internal deploy README.md docs` rather than `git commit -a`, so a stray file is not committed.
- CI must be green (tests, golangci-lint, the `dist` check). PRs are squash-merged.

## Releases and deployment

Maintainers tag releases (`vX.Y.Z`); the image is built from `main` by CI. The Grafana dashboard and Prometheus rules in `deploy/` are not part of the image: users copy them from the matching commit.

Release notes live in the body of the GitHub Release for each tag (there is no changelog file), so each release has one place to read. Write them for people who run NasWarden, in the style of the earlier releases: a bold heading per change, what it does for them and anything that may surprise them (numbers that move, new metrics, what was and was not checked), then the image line, e.g. `ghcr.io/ahmd-soliman/naswarden:1.4.0` (also `:1.4`). Say whether `deploy/` changed, since users copy the dashboard and rules by hand.

## Reporting bugs and security issues

For a bug, include the NasWarden version (`/version`), your TrueNAS version, and what the browser console or the NasWarden log says. Redact hostnames and keys. For a suspected security problem, do not open a public issue; follow the [security policy](SECURITY.md).

## License

By contributing you agree your work is released under the [MIT license](LICENSE).
