# Blueprint reconciliation — verified state & remaining work (2026-10-18)

Deliverable for the "GO" on Recommendation 1 (blueprint reconciliation). All
claims verified against both repos live (git logs, dispatch tables, test runs)
— no assumptions.

## What is DONE (verified)

**kern side (commits):**
- `a90feb51` P4 phase A — four blueprint knots extracted into core.
- `a05c91c7` P4 — blueprint family deleted from kern (19.7k LOC).
- `8a37162b` P5b — CLI command table 194 -> 52 (core surface only).
- `5e2c85cd`/`10af215e` P5 + tracker — dead CLI/MCP surface removed.
- Global pre-commit hook re-pointed: now runs `kern verify --staged
  security,architecture` (verified in ~/.kern/git-hooks/pre-commit) — the
  dead `kern check` knot is gone.

**blueprint standalone (7 port commits, all TODAY 03:09-12:28):**
- `31bfe87` scanners (gitleaks+jscpd+duplication), `84f25e5` resilience,
  `051e172` bpreceipt (+fsutil), `9c60f73` bppolicy (policy+risk),
  `46c4ab3` gates (approval store + approval check), `6bbf48a` bpcli
  (CLI/MCP surface), `cbf724e` blueprint-core (adapters/kern + audit + service).
- Production tree now 53,925 LOC (was 16,346) — the full change-governance
  engine, as a standalone binary.
- `go build ./...` green. `internal/architecture` parity green (kern).
- Full suite: 177 packages ok, 0 FAIL (kern, post-P5).

## What is BROKEN — the real remaining work

`go test ./...` in blueprint fails in 3 packages, all one root cause:

| package | failing gates | cause |
|---|---|---|
| internal/blueprint/adapters/kern | G2 (architecture), G3 (secrets) | execs `kern guard check --json` / `kern sec --json` — **commands deleted from kern by P5b CLI slim** |
| internal/bpcli/cli | G17 doctor "kern contract" | same: `kern sec` probe fails |
| internal/bpcli/mcp | G5, G18, G28, P12 | same + `kern audit append` missing |

Blueprint's `adapters/kern` layer is the binary-consumer boundary BY DESIGN:
`NewKernClient()` resolves the kern binary (KERN_BINARY -> PATH ->
../kern/bin/kern) and the G2/G3/G7/G17/G27 gates are integration tests against
the REAL kern binary (requireKernBinary; CI sets KERN_REQUIRE_BINARY=1 so a
missing binary FAILs instead of skipping). kern deleted the commands those
gates and the adapters depend on.

**kern still has the engines** (verified on disk):
- `internal/guard` (LoadBoundaries/Verdict/DirMatch) — used by `kern verify`
- `internal/sec` + `internal/secscan` — used by `kern verify security`
- fingerprint computation lives in `internal/index` (snapcache/snapshot)
- audit chain + evidence engines intact (`kern evidence` still dispatched)

## The fork, decided

- **Option A — blueprint goes fully native** (rewire adapters to internal
  engines): REJECTED. `duplication.NewCheck(client *kern.KernClient)` is
  hard-wired to the kern binary; the fingerprint computation lives in kern's
  index. Porting it means forking kern's index — exactly what the pivot
  forbids. The G2/G3/G7 integration harness (21k test LOC, KERN_REQUIRE_BINARY)
  exists precisely to test the real-binary contract.
- **Option B — restore the 4 thin bridge commands in kern** (guard check,
  sec, fingerprint, audit append) as thin shells over the still-present
  engines: ACCEPTED. Matches the pivot's own keep-list ("keep kern guard,
  authorize-context, audit, evidence"); the CLI slim over-deleted them (they
  were on the keep list, not the move list). Cost: ~4 thin commands, well
  under the 19,600 cmd/kern cap; blueprint's 21k harness goes green as-is.

Also pending: blueprint worktree is mid-flight (21 uncommitted files: `blueprint
check` branding, concurrent Validate, untracked internal/pii + secscan being
pulled in). Finish it coherently and commit.

## Remaining phases (corrected)

1. **P7 kern bridge**: restore `kern guard check`, `kern sec`,
   `kern fingerprint`, `kern audit append` (thin shells over internal engines,
   same argv contracts the adapters call) + dispatch entries + verify.
2. **P8 blueprint**: finish the mid-flight worktree, commit it; blueprint full
   suite green.
3. **P9 closure**: campaign write-up + publish docs/benchmarks + make install
   (kern binary must match the P5 tree).