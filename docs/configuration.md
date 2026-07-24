# beef-gen configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `[::1]:8725` | Proxy ingress TCP address. The open tx port accepts records via the `0xBEEF` tag grammar; point at the dedicated lane (8728) to exercise flow separation |
| `-topics` | `tm_demo` | Comma-separated overlay topic names carried in every submission (1–15, each ≤ 64 bytes UTF-8) |
| `-encoding` | `beef` | `beef` \| `beefv2` \| `atomic` synthetic markers, or `real` — the BRC-62 specification example, sent verbatim (ignores `-object-bytes`; identical across emissions, so ingress dedup admits it once per topic) |
| `-object-bytes` | `64` | Synthetic object size (≥ 16; the trailing 8 bytes are the emission counter that keeps ContentIDs unique) |
| `-count` | `10` | Submissions to send (0 = unlimited, bound with `-duration`) |
| `-interval` | `50ms` | Delay between submissions |
| `-duration` | `0` | Stop after this long (0 = count-driven) |
| `-seed` | `1` | Deterministic synthetic-object seed |
| `-log-hashes` | `false` | Log each emission's ContentID prefix |

On exit the generator prints `sent=N` for harness assertions. A submission
failure reconnects with exponential backoff and retries the same emission; a
malformed record is a fatal self-verify error, never sent.

Deferred: a proxy-bypassing `-mode direct-multicast` (self-stamped FrameVer
0x09 emission for listener-only tests) — the harness drives everything
through the proxy today.
