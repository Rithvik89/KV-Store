# Changelog

All notable changes to this project are documented here.
Format inspired by [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added

- Checksummed WAL records (`CNDR` header, length-prefixed payload, CRC32) with
  fsync policies `always` / `everysec` / `no` (`wal.WithFsync`).
- WAL recovery (#30): truncate torn tails on open so later appends are not
  hidden; fsync'd ACK survives unclean reopen (tested).
- WAL compaction (#31): `Rewrite` live SETs via temp file + atomic rename;
  `Store.Compact` snapshots the dict (maintenance path, not a CSP command).
- One-dict keyspace with lazy TTL (`internal/store`): `entry{value,expire}`;
  EXPIRE / TTL / PERSIST on the command table. TTL is memory-only this sitting.
- Cinder curriculum tracker and labeled issues; `TODO.md` on `main`.
- Event loop refactor (branch `cinder/21-eventloop`): `Poller` + `Loop` with
  Darwin kqueue and Linux epoll; readiness-only package; socketpair tests.
- `docs/DESIGN.md` for standing design decisions.
- CSP (`internal/proto`): RESP-shaped encode/decode with incremental framing;
  server speaks CSP; command layer returns `proto.Value`.
- Command dispatch table (`internal/command`): verb → handler registry; arity
  checked in one place; `NewWithStore` for tests.
- Renamed package `executor` → `command`.
- CSP CLI (`cmd/cli`): no cobra; `cinder>` REPL; default port 9573; `-raw`.

### Changed

- Collapsed `MemoryStorage` / `PersistentStorage` into `store.Store`
  (`Open` / `OpenMemory`); WAL is optional on the same type.
- Renamed package `internal/storage` → `internal/store`; interface file
  `istore.go` (`IStore`); `NewWithStorage` → `NewWithStore`.
- WAL `Truncate(0)` compaction stub replaced by `Rewrite` + rename.
- Accept / read / write moved out of `eventloop` into `server` (naive
  read→process→write path).
- Server live path no longer uses `strings.Fields` line protocol.

### Design decisions

- **Readable-only I/O for now:** client fds registered `Readable` only; replies
  written inside `OnReadable`. `OnWritable` kept on the API but unused until
  out-buffer / write-interest arming is needed. See [docs/DESIGN.md](docs/DESIGN.md).
- **#22 server I/O polish:** backlog — functional structure first.
- **Map + WAL, one Store:** WAL is durability/replay; the dict is the live
  keyspace. `OpenMemory` skips the WAL for tests. Package is `store`.
- **Lazy-only TTL:** expired keys are removed on access; they may linger until
  then. Sampling deferred.
- **WAL framing (#29):** one file, checksummed records, explicit fsync policy;
  multi-segment compaction later (#31).
- **WAL recovery (#30):** repair torn tail on open; mid-file CRC still errors.
- **WAL compact (#31):** single-file live-key rewrite + atomic rename; no
  multi-segment yet; Compact is sync/admin only.

## [0.1.0] — prior

Initial MemKV layout: kqueue event loop, executor, memory + WAL storage, CLI.
