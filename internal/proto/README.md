# CSP — Cinder Serialization Protocol

RESP-shaped wire format for MemKV/Cinder. Not a Redis compatibility promise.

## Client → server

An **array** of bulk strings: verb + args.

```
*2\r\n$4\r\nPING\r\n$4\r\nPONG\r\n   → PING with arg PONG (echo style)
*1\r\n$4\r\nPING\r\n                 → PING
*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n
```

## Server → client

| Type | Form | Example |
| --- | --- | --- |
| Simple | `+…\r\n` | `+PONG\r\n`, `+OK\r\n` |
| Error | `-…\r\n` | `-ERR unknown command\r\n` |
| Integer | `:n\r\n` | `:1\r\n` |
| Bulk | `$n\r\n` + n bytes + `\r\n` | `$5\r\nhello\r\n` |
| Null bulk | `$-1\r\n` | GET miss |

## Incremental decode

`Decode(buf)` returns `ok=false` when more bytes are needed. The caller keeps
`buf` and appends the next read. Corrupt input returns an error.
