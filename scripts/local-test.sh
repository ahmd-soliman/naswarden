#!/usr/bin/env bash
# Build NasWarden, run it against a real TrueNAS and check that it works.
#
#   TRUENAS_HOST=192.168.1.10:8443 TRUENAS_API_KEY=... scripts/local-test.sh
#
# Everything comes from environment variables, so no key is ever written to a
# file, a command line or the output. Use the key of a user that only has the
# Readonly Admin role: that is what NasWarden is meant to run with, and the
# test then proves it works with nothing more.
#
# Required:
#   TRUENAS_HOST      host[:port] of the TrueNAS web UI, e.g. 192.168.1.10:8443
#   TRUENAS_API_KEY   the API key
# Optional (same meaning as for NasWarden itself):
#   TRUENAS_TLS            default true
#   TRUENAS_INSECURE_TLS   default true (a LAN TrueNAS usually has a self-signed certificate)
#   DOCKER_PROXY_URL, DOCKER_HOST, INCUS_URL, INCUS_CLIENT_CERT, INCUS_CLIENT_KEY
# Script settings:
#   PORT       port to listen on, default 18080
#   KEEP=1     leave NasWarden running afterwards, to look at it in a browser
#              (stop it with Ctrl-C)
#   WAIT       seconds to wait for the first refresh, default 90
#
# The update check (the "Update available" pill) runs in the background and
# lands on the second refresh, about a minute after start; this script waits
# for it. The browser part of the check needs Node 22 or newer; without it the
# script still checks the metrics.
set -euo pipefail

: "${TRUENAS_HOST:?set TRUENAS_HOST, e.g. 192.168.1.10:8443}"
: "${TRUENAS_API_KEY:?set TRUENAS_API_KEY (the key of a Readonly Admin user)}"
export TRUENAS_TLS="${TRUENAS_TLS:-true}"
export TRUENAS_INSECURE_TLS="${TRUENAS_INSECURE_TLS:-true}"
PORT="${PORT:-18080}"
WAIT="${WAIT:-90}"
export PORT

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
bin="$work/naswarden"
log="$work/naswarden.log"
pid=""

cleanup() {
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then kill "$pid" 2>/dev/null || true; fi
  rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

if lsof -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "port $PORT is already in use; set PORT to another one" >&2
  exit 2
fi

echo "==> building"
(cd "$root" && go build -o "$bin" ./cmd/naswarden)

echo "==> starting on http://localhost:$PORT (TrueNAS at $TRUENAS_HOST)"
"$bin" >"$log" 2>&1 &
pid=$!

base="http://127.0.0.1:$PORT"
metric() { curl -fsS "$base/metrics" 2>/dev/null | awk -v n="$1" '$1==n {print $2; exit}'; }

echo "==> waiting up to ${WAIT}s for the first refresh from TrueNAS"
ok=""
for _ in $(seq 1 "$WAIT"); do
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "NasWarden exited:" >&2
    sed 's/^/    /' "$log" >&2
    exit 1
  fi
  ts="$(metric naswarden_last_refresh_success_timestamp_seconds || true)"
  if [ -n "$ts" ] && [ "${ts%%.*}" != "0" ]; then ok=1; break; fi
  sleep 1
done
if [ -z "$ok" ]; then
  echo "FAIL: no successful refresh. The log says:" >&2
  grep -iE "error|warn" "$log" | sed 's/^/    /' >&2 || true
  echo "    (the dashboard shows the reason too: open $base/)" >&2
  exit 1
fi
echo "ok: refreshed from TrueNAS"

echo "==> version"
curl -fsS "$base/version"; echo

echo "==> pools (from /metrics)"
curl -fsS "$base/metrics" | grep -E '^naswarden_pool_(healthy|size_bytes|usable_bytes|available_bytes)\{' | sort | sed 's/^/    /'

if command -v node >/dev/null 2>&1 && node -e 'process.exit(typeof WebSocket==="undefined"?1:0)' 2>/dev/null; then
  echo "==> what the browser receives (waiting for the update check, up to ${WAIT}s)"
  BASE_WS="ws://127.0.0.1:$PORT/ws" WAIT="$WAIT" node -e '
    const ws = new WebSocket(process.env.BASE_WS);
    const limit = Number(process.env.WAIT) * 1000;
    let last = null;
    const done = (code) => {
      if (!last) { console.log("    no state received"); process.exit(1); }
      const s = last.server || {};
      console.log("    host:", s.hostname, "| TrueNAS", s.version);
      console.log("    pools:", (last.pools || []).map((p) => p.name + (p.usable_size ? " (usable)" : " (raw size)")).join(", ") || "none");
      console.log("    update:", s.update ? JSON.stringify(s.update) : "none yet");
      console.log("    docker:", JSON.stringify(last.docker || null));
      console.log("    stale sources:", JSON.stringify(last.stale_sources || []));
      process.exit(code);
    };
    setTimeout(() => done(0), limit);
    ws.onmessage = (e) => {
      const m = JSON.parse(e.data);
      if (m.type === "error") { console.log("    error:", m.message); return; }
      if (m.type === "state") { last = m; if (m.server && m.server.update) done(0); }
    };
    ws.onerror = () => { console.log("    websocket error"); process.exit(1); };
  '
else
  echo "==> skipping the browser-side check (needs Node 22 or newer)"
fi

echo "==> warnings and errors in the NasWarden log"
grep -iE "warn|error" "$log" | sed 's/^/    /' || echo "    none"

if [ "${KEEP:-}" = "1" ]; then
  echo
  echo "Running at http://localhost:$PORT/ -- Ctrl-C to stop."
  wait "$pid"
fi
echo "==> done"
