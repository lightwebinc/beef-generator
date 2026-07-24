# beef-generator

Traffic generator for the BRC-148 BEEF object plane: `beef-gen` submits BEEF
submission records (topic list + BEEF object) to a shard-proxy ingress port —
the open tx port (8725) or the dedicated BEEF lane (8728).

Objects are synthetic (valid BEEF-family leading marker + seeded bytes; the
fabric never parses past the marker) or the real BRC-62 specification example
(`-encoding real`) for verbatim-carriage proofs. Every record is self-verified
before writing, and every synthetic emission carries a unique ContentID so
ingress dedup never suppresses generator traffic.

```sh
go build ./cmd/beef-gen
./beef-gen -addr '[::1]:8725' -topics tm_demo,tm_other -count 100
```

See [docs/architecture.md](docs/architecture.md) and
[docs/configuration.md](docs/configuration.md). Canonical spec:
`bsv-multicast/docs/brc-148-shard-domain-beef-plane.md`.
