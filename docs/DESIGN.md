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
