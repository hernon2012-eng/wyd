# Protocol 12000 compatibility status

**Status: UNVERIFIED. Do not infer or invent wire behavior from the number “12000.”**

No real WYD assets or protocol-12000 definitions were available in the audited worktrees. No verified framing, transport, handshake, message/opcode registry, encoding, cipher, checksum, authentication exchange, version negotiation, or compatibility test was found. Consequently, compatibility cannot be claimed, rejected, or safely implemented from this evidence.

## Keep distinct

- P01 documents an isolated **HTTP/JSON v1** gateway contract, including its own demo handshake. It is not evidence for protocol 12000, CPSock, or any production WYD listener.
- The foundation handoff itself directs the integrator to identify the actual server/client versions and compare the live protocol; it cautions against assuming equivalence with a public CPSock/W2PP reference.
- Synthetic JSON examples and schemas are proposal contracts, not captured WYD packets or protocol definitions.

## Evidence needed to establish compatibility

Obtain authorized, version-pinned server and client artifacts and document:

1. Exact server/client product, branch, commit/build, platform, configuration, and protocol selector.
2. Transport and connection lifecycle; framing, byte order, lengths, checksums, compression, encryption, key exchange, and sequence/replay behavior.
3. Handshake/version negotiation and the precise login/session exchanges, including error and disconnect semantics.
4. Message/opcode definitions with field types, bounds, optionality, encoding, and state-transition rules.
5. Server-side authority and mutation rules for every operation proposed for adaptation.
6. Sanitized, authorized fixtures and a compatibility matrix for supported versions; no credentials or protected production captures.

Unknowns must remain marked unknown until verified against authoritative, authorized sources. Do not reverse engineer against production, modify a live listener, synthesize opcodes/handshakes, or send unverified messages to a real server.

## Acceptance gate

Protocol work remains blocked until source/version evidence, authorization, a reviewed written specification, independent parser/serializer tests, and interoperability results against a disposable authorized environment are attached to a specific commit. Until then, keep adapters disabled and preserve the original protocol unchanged.
