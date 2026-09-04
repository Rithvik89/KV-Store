# Changelog

All notable changes to this project are documented here.
Format inspired by [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added

- Cinder curriculum tracker and labeled issues; `TODO.md` on `main`.
- Event loop refactor (branch `cinder/21-eventloop`): `Poller` + `Loop` with
  Darwin kqueue and Linux epoll; readiness-only package; socketpair tests.
- `docs/DESIGN.md` for standing design decisions.

### Changed

- Accept / read / write moved out of `eventloop` into `server` (naive
  read→process→write path).

### Design decisions

- **Readable-only I/O for now:** client fds registered `Readable` only; replies
  written inside `OnReadable`. `OnWritable` kept on the API but unused until
  out-buffer / write-interest arming is needed. See [docs/DESIGN.md](docs/DESIGN.md).

## [0.1.0] — prior

Initial MemKV layout: kqueue event loop, executor, memory + WAL storage, CLI.
