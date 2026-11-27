<!-- Read CONTRIBUTING.md first. For anything bigger than a small fix, open an issue before the PR. -->

## What and why

<!-- One topic per PR. Write "Closes #N" or "Part of #N". -->

## Read-only

- [ ] This only sends read queries to TrueNAS, Docker and Incus.

## Checks

- [ ] `gofmt -l .` prints nothing, and `go vet ./...` and `go test -race ./...` pass
- [ ] `golangci-lint run ./...` passes
- [ ] If I touched `web/`: `npm run lint && npm test && npm run build`, and the rebuilt `internal/web/dist` is committed
- [ ] Tests cover the value present, the value missing, and the thing disappearing
- [ ] Test fixtures come from real replies and are redacted (no hostnames, serials, IPs, keys or credentials)

## If this adds or changes a metric

- [ ] `naswarden_*` name with base units, low-cardinality labels, series left out (not 0) when the value is unknown
- [ ] README table, Grafana panel and Prometheus rule updated where it makes sense
- [ ] `promtool check rules` run on rule changes

## Tested on

<!-- What you checked on a real TrueNAS (version), and what you could not. -->
