# Checkpoints

## 1. Contract boundary

- Source labels exactly match the NPC v1 schema: shop, teleport, crafting, production, quest, arena, and administration. The administration label confers no permission.
- Catalog entries are metadata only; opaque references are never resolved or invoked.
- All included data is explicitly marked `DEMONSTRAÇÃO`.

## 2. Registry implementation

- Catalogs and descriptors are versioned.
- Full validation precedes atomic snapshot publication.
- Failed reloads retain the prior snapshot.
- Reads are deterministic and defensively copied.

## 3. Verification

- Unit tests cover allowed/unknown types, malformed and duplicate refs, strict decoding, failed reload preservation, sorted defensive reads, and concurrent access.
- CI runs gofmt, Go tests and race tests, vet, build, and JSON Schema/example validation.
- Local test outcomes are listed in `HANDOFF.md`.

## 4. Publication

- Branch: `codex/arko-services-20261105`, based on Pacote 3 HEAD `1870b89d7fa4a2546c32aac87be83b5f80e3da3b`.
- Implementation commit: `c3155721dc1cf36997474f42c18be08eb0574a99`.
- GitHub Actions run [42133133330](https://github.com/hernon2012-eng/wyd/actions/runs/42133133330) passed both `go` and `contracts` jobs.
