# naswarden

Real-time TrueNAS SCALE monitoring that fills a specific gap: **ZFS
dataset-level quota utilization**. Stock tools (`df`, node-exporter) only
see per-mountpoint usage — they have no concept of a ZFS dataset's own
`quota`/`refquota` property, which TrueNAS users commonly set on individual
app-data/user datasets.

Talks directly to TrueNAS's own middleware API (the same one `midclt` and
the Terraform provider use) over its WebSocket endpoint — no SSH, no `zfs`
CLI dependency.

## Status

Early — Phase 1 (pool health) working against a live TrueNAS SCALE 25.10.7
box. Dataset quota utilization (the actual point of this tool) is next.

## Configuration

| Env var | Required | Description |
|---|---|---|
| `TRUENAS_HOST` | yes | Host and port, e.g. `192.168.8.100:8443` |
| `TRUENAS_API_KEY` | yes | An API key scoped to a TrueNAS user (Credentials → API Keys) |
| `TRUENAS_TLS` | no | `true` to use `wss://` (default `false`) |
| `TRUENAS_INSECURE_TLS` | no | `true` to skip TLS verification, needed for TrueNAS's default self-signed certificate |
| `PORT` | no | HTTP port to listen on (default `8080`) |

```
go run ./cmd/naswarden
```
