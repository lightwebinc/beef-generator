# beef-generator architecture

One binary, `cmd/beef-gen`, mirroring the subtx-generator push-emitter
skeleton (dial with exponential backoff, ticker-paced emissions, reconnect on
write error, `sent=N` summary).

Per emission it builds one BRC-148 **submission record** via
`shard-common/objfmt`: `EncodeBEEFRecord(topics, object)` → self-verify with
`BEEFRecordSize` → single TCP write. The proxy expands the record into one
FrameVer 0x09 frame per topic; the generator never builds frames and never
stamps — HashKey/SeqNum are the ingress's to assign from the observed source.

Objects: a valid BEEF-family leading marker (`0100BEEF`, `0200BEEF`, or the
Atomic prefix) followed by seeded deterministic bytes and a trailing
emission counter (unique ContentIDs ⇒ ingress pair-dedup never suppresses
the generator), or the embedded BRC-62 specification example for
verbatim-carriage proofs. The fabric reads only the marker; consumers of
synthetic objects must not parse them as real BEEF.
