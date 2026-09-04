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
internal/executor/          dispatch table, handlers
internal/storage/           dict, TTL, sampling
internal/pubsub/            channels, subscribe mode
internal/stream/            log, XREAD, groups
internal/wal/               segments, fsync, replay, compact
internal/info/              INFO counters

Makefile  go.mod  scripts/  tests beside each package
```

`server` owns sockets. `eventloop` only reports readiness. `executor` never sees an fd. `storage` never sees a socket.

Out of scope here (other majors / later): cluster, sharding, replication, auth, lists/sets/hashes, LSM.

## Sitting order

| # | Sitting | Issue | Lands in |
| --- | --- | --- | --- |
| 1 | Event loop | #21 #22 | `eventloop/`, `server/` I/O |
| 2 | CSP | #23 | `proto/` |
| 3 | Commands | #24 #25 | `executor/` + CLI |
| 4 | Keyspace / TTL | #26 | `storage/` |
| 5 | Pub/Sub | #27 | `pubsub/` + conn mode |
| 6 | Streams | #28 | `stream/` |
| 7 | AOF | #29 | `wal/` write path |
| 8 | Recovery | #30 | `wal/` replay + boot |
| 9 | Compaction | #31 | `wal/` segments |
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
- [ ] [#24](https://github.com/Rithvik89/memkv/issues/24) **Executor** — `internal/executor`
  - Dispatch table (verb → handler), not a growing `switch`
  - Arity / type errors as CSP error replies
  - PING, ECHO, GET, SET, DEL first; later sittings only register more verbs
  - Connection mode: normal vs subscribed
- [ ] [#26](https://github.com/Rithvik89/memkv/issues/26) **Storage** — `internal/storage`
  - One dict. Persistent path wraps that dict + WAL (do not copy the map twice)
  - Implement `Storage` fully (`MemoryStorage` is missing `Close`)
  - TTL, lazy expire on access, background sampling on the loop via `Post`
  - Values as bytes (CSP bulk strings)
- [ ] [#29](https://github.com/Rithvik89/memkv/issues/29) **WAL write path** — `internal/wal`
  - Keep `Write` / `Replay` / `Close` / `Truncate` as the seam
  - Checksummed records, segments, explicit fsync policy
- [ ] [#30](https://github.com/Rithvik89/memkv/issues/30) **WAL recovery** — `internal/wal`
  - Replay on boot
  - Detect a torn record at the tail and stop there
  - Acknowledged write survives a hard kill
- [ ] [#31](https://github.com/Rithvik89/memkv/issues/31) **WAL compaction** — `internal/wal`
  - New segment of live keys, then atomic swap
  - Not `Truncate(0)` on the same fd
  - Must not stall the loop long enough for clients to notice
- [ ] [#33](https://github.com/Rithvik89/memkv/issues/33) **cmd/server**
  - Config from env (`CINDER_ADDR`, WAL path, fsync, log level)
  - Graceful shutdown that waits for the loop (no `os.Exit` from the signal handler)
- [ ] [#25](https://github.com/Rithvik89/memkv/issues/25) **cmd/cli**
  - Drop cobra
  - Speak CSP
  - GET/SET/DEL, then SUBSCRIBE / XREAD
- [ ] [#34](https://github.com/Rithvik89/memkv/issues/34) **Logger + Makefile**
  - Inject the logger; tests can silence it
  - `run`, `test`, `race`, `smoke`, `bench`
  - `race` must stay green: command execution never leaves the loop thread

## Add

Packages and surfaces that are not in this tree yet.

- [ ] [#23](https://github.com/Rithvik89/memkv/issues/23) **`internal/proto` (CSP)**
  - Simple string, error, integer, bulk string, array
  - Incremental decode over a byte buffer
  - Encode replies the executor returns
  - Nothing else parses bytes
- [ ] [#27](https://github.com/Rithvik89/memkv/issues/27) **`internal/pubsub`**
  - `SUBSCRIBE` / `UNSUBSCRIBE` / `PUBLISH`
  - Fan-out from the loop thread
  - Slow-subscriber policy, tested
- [ ] [#28](https://github.com/Rithvik89/memkv/issues/28) **`internal/stream`**
  - Monotonic entry IDs, `XADD`, range read
  - Blocking `XREAD` that parks the client without parking the loop
  - Consumer groups + ack
- [ ] [#32](https://github.com/Rithvik89/memkv/issues/32) **`internal/info` + load generator**
  - Counters: ops, hits, expired, connected clients, aof bytes
  - `INFO` command
  - Load generator that prints p50/p99
- [ ] [#35](https://github.com/Rithvik89/memkv/issues/35) **Tests**
  - Loop: socketpair, single-thread assertion, write backpressure
  - Proto: split frames, leftover bytes, arrays
  - Commands: arity, unknown verb
  - Store: TTL lazy + sampling
  - WAL: kill after ACK, replay, torn tail, compaction
  - Pub/sub and streams: at least one blocking-client test each
