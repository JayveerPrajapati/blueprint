# Blueprint reconciliation campaign — consolidated deliverable (2026-10-18)

Campaign outcome for the GO on "blueprint reconciliation, then closure".
Every claim below was verified live against both repos (git logs, dispatch
tables, real test runs with the restored binary) — no assumptions.

## What the campaign did

The pivot had removed the blueprint family from kern (P4) while the standalone
blueprint repo's 7 port commits (03:09-12:28 today) moved the engines across.
What looked like "the reconciliation is the remaining work" turned out, under
recon, to be **one contract break** + **one unfinished worktree** — both now
fixed.

### P7 — kern bridge restored (commit b6175028)

Blueprint's `adapters/kern` layer consumes kern via binary commands BY DESIGN
(`NewKernClient()`: KERN_BINARY -> PATH -> ../kern/bin/kern; the G2/G3/G7/G17/
G27 gates are integration tests against the real binary, `KERN_REQUIRE_BINARY=1`
in CI). The pivot CLI slim (194->52) deleted the dispatch rows while kern kept
the engines — an over-deletion (guard/audit were on the pivot keep-list).

Restored as thin shells over the still-present engines:

| command | implementation | engine |
|---|---|---|
| `kern guard` | runGuard -> contextcli.RunGuard | internal/guard + index |
| `kern sec` | runSec (body restored; cmd_security.go had only runMask left) | internal/sec + secscan + suppression registry |
| `kern fingerprint` | runFingerprint (file was intact) | internal/intel.ComputeFingerprint |
| `kern audit` | approvalcli.RunAudit (intact) | tamper-evident chain |

Live probes after restore: sec --json schema 2 (0 findings + 3 suppressed),
guard check --json 0 violations, fingerprint --json 11,363 fingerprints,
audit append appended audit-42608.

### P8 — blueprint worktree finished & committed (commit 9fe6231)

The mid-flight port layer, verified coherent and committed:
- **Native secscan fallback**: gitleaks fallback has two tiers — kern sec when
  the binary is present, else the built-in secscan core (internal/scanners/
  secscan + internal/pii, ported, no external binary). The secrets gate NEVER
  goes quiet: a hardcoded secret BLOCKS even with neither gitleaks nor kern
  available (TestGitleaksFallbackNoClientScans pins block + WARN together).
- **Branding**: `blueprint check` / `blueprint approve` / `blueprint install
  hook`; hooks exec `${BLUEPRINT_BINARY:-blueprint}`.
- **Concurrent Validate** with panic recovery (wall time = slowest leg; a
  panicking check becomes a visible ERROR, never a deadlock).
- **Version-path fixes** (internal/blueprint/version -> bpreceipt/version in
  Makefile/.goreleaser/docs), CI gate-test path fix, boundaries.json re-aimed.

## Verification (the bar)

- kern: `go test -short ./...` 177 packages ok, 0 FAIL; architecture parity
  green; `TestCLIReferenceDocCoversCommandTable` green (doc already covered
  all four restored commands).
- blueprint: `KERN_BINARY=<bridge build> go test ./...` — **29 packages ok,
  0 FAIL** (was 3 failing packages: adapters/kern G2/G3, bpcli/cli G17,
  bpcli/mcp G5/G18/G28/P12). `go vet ./...` clean. Both repos' trees clean
  after commit.

## Remaining (not done, by design)

1. **`make install`** — the installed kern binary (~/.local/bin/kern,
   324b965b) predates the bridge and P5. The hook still works (`kern verify
   --staged` exists there) and commits are green, but the installed CLI lacks
   guard/sec/fingerprint/audit and the P5 surface. Governed install is blocked
   by the stale REJECTED approval record (appr-fb3002cc, memory #736) — the
   operator runs `make install` directly (trusted principal, ungated), then
   `kern setup` to refresh the plugin/AGENTS copies.
2. **Blueprint release** — the standalone is now a shippable product (54k LOC
   incl. the ported engines + 29-package green suite). Releasing (tag +
   goreleaser + CI) is the operator's call; the CI action + goreleaser config
   are already updated for the new layout.
3. **docs/benchmarks** — already published in-repo (152K, 9 files, README +
   fixtures, linked from the README's measured/synthetic claims). No work
   needed; the "publish" recommendation was already satisfied.

## Tracked artifacts

- kern: `.slim/deepwork/pivot-progress-2026-10-18.md` (11 rows, all green)
- blueprint: `.slim/deepwork/blueprint-reconciliation-state-2026-10-18.md`
- this file: `.slim/deepwork/blueprint-reconciliation-campaign-2026-10-18.md`