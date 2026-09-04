# Changelog

All notable changes to this project are documented here.
Format inspired by [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added

- One-dict keyspace with lazy TTL (`internal/storage`): `entry{value,expire}`;
  EXPIRE / TTL / PERSIST on the command table. Persistent path wraps the same
  dict + WAL (no second map). TTL is memory-only this sitting.
- Cinder curriculum tracker and labeled issues; `TODO.md` on `main`.
- Event loop refactor (branch `cinder/21-eventloop`): `Poller` + `Loop` with
  Darwin kqueue and Linux epoll; readiness-only package; socketpair tests.
- `docs/DESIGN.md` for standing design decisions.
- CSP (`internal/proto`): RESP-shaped encode/decode with incremental framing;
  server speaks CSP; command layer returns `proto.Value`.
- Command dispatch table (`internal/command`): verb → handler registry; arity
  checked in one place; `NewWithStorage` for tests.
- Renamed package `executor` → `command`.
- CSP CLI (`cmd/cli`): no cobra; `cinder>` REPL; default port 9573; `-raw`.

### Changed

- Accept / read / write moved out of `eventloop` into `server` (naive
  read→process→write path).
- Server live path no longer uses `strings.Fields` line protocol.

### Design decisions

- **Readable-only I/O for now:** client fds registered `Readable` only; replies
  written inside `OnReadable`. `OnWritable` kept on the API but unused until
  out-buffer / write-interest arming is needed. See [docs/DESIGN.md](docs/DESIGN.md).
- **#22 server I/O polish:** backlog — functional structure first.
- **Map + WAL, one dict:** WAL is durability/replay; the dict is the live
  keyspace. PersistentStorage wraps dict + WAL (do not duplicate maps).
- **Lazy-only TTL:** expired keys are removed on access; they may linger until
  then. Sampling deferred.

## [0.1.0] — prior

Initial MemKV layout: kqueue event loop, executor, memory + WAL storage, CLI.
