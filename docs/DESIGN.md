# Design decisions

Living notes for MemKV / Cinder. Prefer short entries with date and rationale.
Product changelog: [CHANGELOG.md](../CHANGELOG.md).

## Event loop: Readable-only I/O for now

**Date:** 2026-09-04  
**Status:** Accepted (revisit when needed)

### Decision

Drive all connection I/O through `OnReadable`. After accept, register client
fds with `Readable` only. Read the request, process it, and `Write` the reply
inside that path.

Keep `OnWritable`, `Modify`, and Writable interest in the eventloop API, but do
**not** arm Writable interest in the server yet. `Server.OnWritable` is a no-op.

### Why

- Small request/response replies usually fit in one `Write`; write-interest
  arming adds complexity before we need it.
- Idle sockets are almost always writable. Watching Writable by default busy-
  spins the loop. Correct write interest needs an out buffer and arm/disarm
  rules we are not implementing yet.
- Pub/sub and large payloads may force the issue later; that is when we revisit,
  not a reason to wire it on day one.

### Consequences

- Partial `Write` / `EAGAIN` on a slow peer is not handled carefully yet.
- Slow pub/sub subscribers are not handled yet (no out buffer, no drop policy).
- Loop still dispatches Writable before Readable so a later change does not
  reshuffle callback order.

### When to revisit

Arm Writable (out buffer + `Modify`) if we see:

- incomplete replies under load,
- pub/sub fans that block the loop on slow subscribers,
- or large bulk values that do not fit a single `Write`.

Related curriculum ticket: [#22](https://github.com/Rithvik89/memkv/issues/22)
(deferred relative to this decision; reopen when we choose to implement it).

## Backlog: pub/sub (#27) and streams (#28)

**Date:** 2026-09-05  
**Status:** Backlog

### Decision

Do not implement `internal/pubsub` or `internal/stream` on the current path.
Both issues stay open with label `backlog` and written requirements for later.
Focus remains on the KV store, WAL, and process/tooling sittings (#32–#35, #33, #34).

## INFO metrics and load generator

**Date:** 2026-09-05  
**Status:** Accepted (#32)

### Decision

- `internal/info.Metrics` (atomics) shared by store/command/server.
- `INFO [section]` returns a Redis-ish bulk string (stats/clients/persistence/keyspace).
- `cmd/bench` dials CSP and prints p50/p99 (not averages-only).
- No pub/sub or stream counters while those sittings are backlog.

## cmd/server: env config and graceful shutdown

**Date:** 2026-09-05  
**Status:** Accepted (#33)

### Decision

- Config from env: `CINDER_ADDR`, `CINDER_WAL_PATH`, `CINDER_FSYNC`, `CINDER_LOG_LEVEL`.
- Listen via `Config.Addr` (not a bare port field).
- Signal handler calls `Shutdown()` only (loop.Stop). `main` waits for `Start`
  to return, then `Close()` for the WAL. No `os.Exit` in the signal path.

## Storage: one Store (dict + optional WAL); lazy-only TTL

**Date:** 2026-09-05  
**Status:** Accepted

### Decision

- Package `internal/store` (not `storage`): single type `store.Store` with
  interface `IStore` in `istore.go`.
- In-memory dict always; WAL optional (`Open` / `OpenWithWAL` vs `OpenMemory`).
- Write path when durable: WAL first, then dict. Boot: replay WAL → dict.
- TTL is **lazy on access only**. Expired keys may linger until touched.
  Sampling / `Post` sweeps deferred. TTL not written to WAL yet.

### Why

One type beats Memory + Persistent wrappers. Package name `store` matches the
type; `IStore` is the interface file/name so it is not confused with the old
`storage` package. WAL-only would make every read scan the log. Lazy-only is
enough to learn deadlines; linger is documented.

## WAL: checksummed records + fsync policy

**Date:** 2026-09-05  
**Status:** Accepted (#29)

### Decision

- Replace text-line WAL with framed records: `CNDR`+version header;
  `len | payload | crc32`. Payload is length-prefixed key/value (binary-safe).
- Fsync policies: `always` (default), `everysec`, `no` via `wal.WithFsync`.
- Still **one file**. Torn-tail / kill-9 = #30. Compaction = live rewrite + rename (#31);
  multi-segment directories still deferred.
- Incomplete trailing record stops replay without error; CRC mismatch at EOF
  is treated as torn. Mid-file CRC mismatch is `ErrCorrupt`.

## WAL recovery: truncate torn tail on open

**Date:** 2026-09-05  
**Status:** Accepted (#30)

### Decision

On `NewFileWAL`, scan for the last good record offset and **truncate** any
torn suffix before SeekEnd. Otherwise appends land past garbage and Replay
never sees them. Fsync'd writes are proven to survive unclean reopen in tests.

## WAL compaction: rewrite live keys + atomic rename

**Date:** 2026-09-05  
**Status:** Accepted (#31)

### Decision

- `WAL.Rewrite(entries)` writes live SET records to `*.compact.tmp`, fsyncs,
  closes the old fd, `Rename`s onto the WAL path, reopens for append.
- `Store.Compact` snapshots the in-memory dict (skipping expired) and calls
  Rewrite. Not a CSP command; sync on the caller — keep off the hot path.
- Leftover `.compact.tmp` from a crash before rename is deleted on open.
- Multi-segment directories deferred; this is the one-file stand-in for “atomic swap”.
