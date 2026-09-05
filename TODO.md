# Cinder curriculum — finish the tree

Build the complete KV store on this repo first. After it runs end-to-end, reverse-engineer sittings into `TODO(n.x)` placeholders for DevLabs.

Tracker: [#36](https://github.com/Rithvik89/memkv/issues/36). Issues are labeled [`cinder`](https://github.com/Rithvik89/memkv/issues?q=is%3Aissue+label%3Acinder). Work them in sitting order.

Package names stay as they are. Add only `proto`, `pubsub`, `stream`, and `info`.

```
cmd/server/                 process: config, signals, New / Start / Close
cmd/cli/                    CSP client

internal/logger/
internal/eventloop/         poller + loop only
internal/server/            listener, conns, read / frame / flush
internal/proto/             CSP
internal/command/           dispatch table, handlers
internal/store/              dict, TTL, sampling (IStore + Store)
internal/pubsub/            channels, subscribe mode
internal/stream/            log, XREAD, groups
internal/wal/               segments, fsync, replay, compact
internal/info/              INFO counters

Makefile  go.mod  scripts/  tests beside each package
```

`server` owns sockets. `eventloop` only reports readiness. `command` never sees an fd. `store` never sees a socket.

Out of scope here (other majors / later): cluster, sharding, replication, auth, lists/sets/hashes, LSM.

## Sitting order

| # | Sitting | Issue | Lands in |
| --- | --- | --- | --- |
| 1 | Event loop | #21 #22 | `eventloop/`, `server/` I/O |
| 2 | CSP | #23 | `proto/` |
| 3 | Commands | #24 #25 | `command/` + CLI |
| 4 | Keyspace / TTL | #26 | `store/` |
| 5 | Pub/Sub | #27 | `pubsub/` + conn mode — **backlog** |
| 6 | Streams | #28 | `stream/` — **backlog** |
| 7 | AOF | #29 | `wal/` write path — done |
| 8 | Recovery | #30 | `wal/` replay — done |
| 9 | Compaction | #31 | `wal/` compact — done |
| 10 | Benchmarks | #32 | `info/` + load gen |
| — | Process wiring | #33 | `cmd/server` |
| — | Logger + Makefile | #34 | `logger/`, `Makefile` |
| — | Tests (land with each sitting) | #35 | beside each package |

First green slice: `printf 'PING\r\n' | nc 127.0.0.1 7379` returns a CSP `+PONG`.

## Refurbish

Existing packages. Rewrite the body; keep the name.

- [ ] [#21](https://github.com/Rithvik89/memkv/issues/21) **Event loop** — `internal/eventloop`
  - Split kqueue vs epoll behind one `Poller`
  - Level-triggered read **and** write interest
  - `Register` / `Modify` / `Deregister` / `Run` / `Post` / `Stop`
  - Pin the loop thread; `EINTR` is normal; handler errors do not kill the loop
  - Drop accept, parse, and `Write` from this package
- [ ] [#22](https://github.com/Rithvik89/memkv/issues/22) **Server I/O** — `internal/server`
  - Open the listener with socket/bind/listen (loop needs the raw fd)
  - Per-conn in/out buffers, read-until-`EAGAIN`, write-interest arming
  - Partial frames, pipelining, half-close still owed replies
  - Cap request size so a client with no delimiter cannot grow RAM forever
- [x] [#24](https://github.com/Rithvik89/memkv/issues/24) **Command dispatch** — `internal/command`
  - Dispatch table (verb → handler), not a growing `switch`
  - Arity / type errors as CSP error replies
  - PING, ECHO, GET, SET, DEL first; later sittings only register more verbs
  - Connection mode deferred to #27 (**backlog**)
- [x] [#26](https://github.com/Rithvik89/memkv/issues/26) **Storage** — `internal/store`
  - One `Store` (dict + optional WAL). Lazy expire; EXPIRE / TTL / PERSIST
  - TTL not in WAL yet; values still `string`
- [x] [#29](https://github.com/Rithvik89/memkv/issues/29) **WAL write path** — `internal/wal`
  - Keep `Write` / `Replay` / `Close` / `Rewrite` / `Size` as the seam
  - Checksummed framed records; fsync `always` / `everysec` / `no`
  - One file (multi-segment = #31)
- [x] [#30](https://github.com/Rithvik89/memkv/issues/30) **WAL recovery** — `internal/wal`
  - Truncate torn tail on open; append-after-tear recovers
  - Fsync'd write survives unclean reopen; mid-file CRC → error
- [x] [#31](https://github.com/Rithvik89/memkv/issues/31) **WAL compaction** — `internal/wal`
  - `Rewrite` live SETs via temp + atomic rename (not `Truncate(0)`)
  - `Store.Compact` from dict; sync/admin only; multi-segment deferred
- [x] [#32](https://github.com/Rithvik89/memkv/issues/32) **`internal/info` + bench**
  - Counters: ops, hits/misses, expired, clients, wal_bytes, keys
  - `INFO` command; `cmd/bench` with p50/p99; `make bench`
- [x] [#33](https://github.com/Rithvik89/memkv/issues/33) **cmd/server**
  - Env: `CINDER_ADDR`, `CINDER_WAL_PATH`, `CINDER_FSYNC`, `CINDER_LOG_LEVEL`
  - Graceful shutdown: signal → Shutdown → Start returns → Close WAL (no `os.Exit`)
- [x] [#34](https://github.com/Rithvik89/memkv/issues/34) **Logger + Makefile**
  - Injectable logger; tests use `Discard` / `TestMain`
  - `make test`, `race`, `run`, `smoke`, `bench`
- [ ] [#25](https://github.com/Rithvik89/memkv/issues/25) **cmd/cli**
  - Drop cobra
  - Speak CSP
  - GET/SET/DEL (SUBSCRIBE / XREAD when #27/#28 leave backlog)

## Add

Packages and surfaces that are not in this tree yet.

- [ ] [#23](https://github.com/Rithvik89/memkv/issues/23) **`internal/proto` (CSP)**
  - Simple string, error, integer, bulk string, array
  - Incremental decode over a byte buffer
  - Encode replies the command layer returns
  - Nothing else parses bytes
- [ ] [#27](https://github.com/Rithvik89/memkv/issues/27) **`internal/pubsub`** — **backlog**
  - `SUBSCRIBE` / `UNSUBSCRIBE` / `PUBLISH`
  - Fan-out from the loop thread
  - Slow-subscriber policy, tested
- [ ] [#28](https://github.com/Rithvik89/memkv/issues/28) **`internal/stream`** — **backlog**
  - Monotonic entry IDs, `XADD`, range read
  - Blocking `XREAD` that parks the client without parking the loop
  - Consumer groups + ack
- [ ] [#35](https://github.com/Rithvik89/memkv/issues/35) **Tests**
  - Loop: socketpair, single-thread assertion, write backpressure
  - Proto: split frames, leftover bytes, arrays
  - Commands: arity, unknown verb
  - Store: TTL lazy + sampling
  - WAL: kill after ACK, replay, torn tail, compaction
  - Pub/sub and streams: at least one blocking-client test each
