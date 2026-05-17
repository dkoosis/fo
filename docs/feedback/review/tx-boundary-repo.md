# tx-boundary review — repo

**Run:** `bd775e303d86-tx-boundary`
**Date:** 2026-05-17
**Target:** whole repo (`/Users/vcto/Projects/fo`)
**Linter:** tx-boundary (multi-write atomicity, check-then-write, parent+children, tx leaks)

## Summary

**0 findings.**

fo has no database layer. `go.mod` declares no SQL/ORM/KV dependency (`database/sql`, `sqlx`, `gorm`, `sqlite`, `pgx`, `mongo`, `bolt`, `badger` — all absent). `rg -l 'database/sql|BeginTx|sqlx|gorm|sqlite|pgx|mongo'` returns zero Go files. The tx-boundary rule set (multi-write, check-then-write, parent-children, invariant-spans-statements, tx-no-defer-rollback, tx-commit-without-error-check, tx-across-goroutines, tx-spans-network-call) applies to transactional DB code; none of those primitives exist here.

The closest analog is sidecar file persistence in `pkg/state/` and the suppress-ignore writer in `cmd/fo/`. All three writers already use the canonical atomic-rename idiom:

- `pkg/state/state.go:131-157` — `MkdirAll` → `CreateTemp` → write → `Rename`
- `pkg/state/metrics_history.go:109-132` — same pattern
- `cmd/fo/suppress_cmd.go:215-238` — same pattern

That is the filesystem equivalent of `BeginTx`/`Commit`: a reader either sees the old file or the new file, never a torn write. No partial-state risk.

## Scope notes

- fo is a streaming presentation filter (stdin → IR → render). No persistent store, no concurrent writers, no parent/child rows.
- The `.fo/last-run.json` sidecar is single-process, single-writer per `fo` invocation. The atomic-rename idiom is sufficient; no locking or fsync coordination needed for this contract.
- If fo ever grows a shared cache or multi-process state (e.g., a watch daemon writing concurrent runs), revisit: rename is atomic within a directory, but two racing writers can still clobber each other's payload. Today that is out of scope per the north-star (no daemon, no server).

## Recommendation

Skip tx-boundary on this repo until a transactional store is introduced. The bundled snipe metrics (cycles, instability, LCOM4) are more useful signals for fo's actual risk surface.

---

## Run telemetry

`trixi log-skill` invocation failed (command not registered in this trixi build). Findings count of 0 logged here for the audit trail under run-id `bd775e303d86-tx-boundary`.
