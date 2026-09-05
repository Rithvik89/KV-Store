# Cinder V0.1 — complete

Tagged **`v0.1.0`**. Working single-threaded KV store on this tree.

Next: reverse-engineer sittings into DevLabs `TODO(n.x)` placeholders
(`devlabs` → `frontend/src/fixtures/projectModules/cinder/`).

## V0.1 shipped

| Sitting | Issue | Package |
| --- | --- | --- |
| Event loop | #21 | `internal/eventloop` |
| CSP | #23 | `internal/proto` |
| Commands + CLI | #24 #25 | `internal/command`, `cmd/cli` |
| Keyspace / lazy TTL | #26 | `internal/store` |
| WAL write | #29 | `internal/wal` |
| WAL recovery | #30 | `internal/wal` |
| WAL compaction | #31 | `internal/wal` |
| INFO + bench | #32 | `internal/info`, `cmd/bench` |
| Process wiring | #33 | `cmd/server` |
| Logger + Makefile | #34 | `internal/logger`, `Makefile` |
| Tests | #35 | beside each package |

## Explicitly not V0.1 (backlog)

| Issue | Topic |
| --- | --- |
| #22 | Server write-interest / framing hardening |
| #27 | Pub/Sub |
| #28 | Streams |

## Frozen tree

```
cmd/server/                 process: config, signals, New / Start / Close
cmd/cli/                    CSP client
cmd/bench/                  load gen (p50/p99)

internal/logger/
internal/eventloop/         poller + loop only (Readable-first; Writable API reserved)
internal/server/            listener, conns, read / frame / flush
internal/proto/             CSP
internal/command/           dispatch table, handlers
internal/store/             dict + lazy TTL (IStore + Store)
internal/wal/               framed records, fsync, replay, Rewrite compact
internal/info/              INFO counters
```

`server` owns sockets. `eventloop` only reports readiness. `command` never sees an fd. `store` never sees a socket.

Out of scope for Cinder: cluster, sharding, replication, auth, lists/sets/hashes, LSM (→ Ember).
