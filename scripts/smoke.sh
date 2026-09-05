#!/usr/bin/env bash
# Self-contained smoke: start server, CSP PING, shut down.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SERVER="${ROOT}/build/server/server"
WAL="$(mktemp -t cinder-smoke-wal.XXXXXX)"
LOG="$(mktemp -t cinder-smoke-log.XXXXXX)"
PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"

cleanup() {
  if [[ -n "${PID:-}" ]] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -f "$WAL" "${WAL}.compact.tmp" "$LOG"
}
trap cleanup EXIT

export CINDER_ADDR="127.0.0.1:${PORT}"
export CINDER_WAL_PATH="$WAL"
export CINDER_LOG_LEVEL=error
export CINDER_FSYNC=no

"$SERVER" >"$LOG" 2>&1 &
PID=$!

# CSP: *1\r\n$4\r\nPING\r\n
REQ=$'*1\r\n$4\r\nPING\r\n'
ok=0
for _ in $(seq 1 50); do
  if printf '%s' "$REQ" | nc -w 1 127.0.0.1 "$PORT" 2>/dev/null | grep -q PONG; then
    ok=1
    break
  fi
  sleep 0.05
done

if [[ "$ok" -ne 1 ]]; then
  echo "smoke: server did not respond to CSP PING" >&2
  cat "$LOG" >&2 || true
  exit 1
fi

echo "smoke: CSP PING -> PONG ok (addr=${CINDER_ADDR})"
