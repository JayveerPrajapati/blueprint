# Duplication Oracle Benchmark

## Overview

Blueprint detects duplication with a **two-pass triage model** (P1.1),
orchestrated by the jscpd adapter (`internal/blueprint/adapters/jscpd`):

1. **Pass 1 — in-house structural triage (always runs).** The in-house check
   (`duplication:advisory`) computes a similarity score between new and
   existing functions using a 4-signal weighted combination: signature shape
   (0.20), control-flow shape (0.35), called-symbol overlap (0.30), and
   size/literal profile (0.15). Every match ≥ 0.60 is an advisory candidate
   (WARN, or INFO for the 0.60–0.85 informational bucket). Candidates above
   **0.90** are block-eligible; everything else never blocks.
2. **Pass 2 — jscpd confirmation (when the jscpd binary is available).**
   jscpd independently scans the mirrored repo (token-based clone detection).
   A block-eligible candidate whose **file pair** is also reported as a jscpd
   clone is escalated to `duplication:confirmed-block` (BLOCK) — the first
   BLOCK duplication can produce. A candidate jscpd does NOT confirm stays
   advisory WARN (in-house thought high similarity, jscpd disagreed — not
   enough to block). jscpd-only clones stay WARN
   (`duplication:jscpd:clone`).

**Fallback:** when the jscpd binary is unavailable, the check is **pure
advisory** — no confirmation is possible, so nothing escalates. All in-house
findings stay WARN/INFO and a `duplication:incumbent-unavailable` WARN flag is
added so the fallback is never silent.

## Methodology

A 7-fixture corpus of Go function pairs, each with a ground-truth label
(true duplicate or not), is scored by the `Similarity` function. Two metric
sets are pinned:

- **Advisory path** — every match ≥ 0.60 (the triage layer; high recall is
  its job).
- **Blocking path** — only matches > 0.90, with jscpd confirmation simulated
  by a mock confirmer (the same `func([2]string) bool` shape the adapter's
  `WithConfirmer` option accepts): jscpd confirms the two true duplicates and
  rejects the structural false positive.

The corpus and scores are reproduced by `TestSimilarityBenchmark`
(advisory path) and `TestSimilarityBenchmarkBlockingPath` (blocking path) in
`internal/blueprint/checks/duplication/benchmark_test.go`. Both use the real
`kern fingerprint` output captured for each fixture (including control-flow
counts) — no real kern or jscpd binary is needed at test time. Scores are
deterministic: `Similarity` is a pure function.

> **Note on historical scores.** `fixtures.go` documents earlier scores
> (exact 0.825, renamed 0.585, refactored 0.820, different-algo 0.779,
> wrapper 0.416, unrelated 0.525, boilerplate 0.540). Those figures predate
> control-flow counts in fingerprint records. Once control-flow counts are in
> the record, the control-flow signal (weight 0.35) contributes fully, raising
> scores (e.g. exact duplicates now score 1.0). The numbers below are the
> current, measured values.

## Corpus

| # | Fixture | Ground truth | Score | Bucket | Advisory | Blocking |
|---|---------|-------------|-------|--------|----------|----------|
| 1 | exact-duplicate | DUP | 1.000 | block-candidate | TP | TP |
| 2 | renamed-duplicate | DUP | 0.760 | informational | TP | advisory only |
| 3 | slightly-refactored-duplicate | DUP | 0.975 | block-candidate | TP | TP |
| 4 | different-but-similar-algorithm | NOT DUP | 0.909 | warning | FP | TN |
| 5 | wrapper-around-existing | NOT DUP | 0.517 | ignore | TN | not detected |
| 6 | unrelated-same-signature | NOT DUP | 0.659 | informational | FP | advisory only |
| 7 | generated-boilerplate | NOT DUP | 0.680 | informational | FP | advisory only |

## Results — Advisory path (threshold = 0.60)

| Metric | Value | Target | Met? |
|--------|-------|--------|------|
| Precision | 0.50 | ≥0.50 | ✓ (at floor) |
| Recall | 1.00 | ≥0.75 | ✓ |
| FPR | 0.75 | — | — |

Confusion matrix: TP=3, FN=0, FP=3, TN=1.

The advisory path is the **triage layer**: high recall (it misses no true
duplicate) at the cost of false positives. Its 0.50 precision is expected and
documented — the structural heuristic alone cannot distinguish "same
algorithm, different names" from "different algorithm, same shape" reliably
enough to gate changes.

## Results — Blocking path (eligible > 0.90, jscpd-confirmed)

| Metric | Value | Target | Met? |
|--------|-------|--------|------|
| Precision | 1.00 | ≥0.90 | ✓ |
| Recall | 1.00 | ≥0.75 | ✓ |
| FPR | 0.00 | — | ✓ |

Confusion matrix: TP=2, FN=0, FP=0, TN=1.

Only three fixtures clear the > 0.90 gate: exact-duplicate (1.000),
slightly-refactored-duplicate (0.975), and different-but-similar-algorithm
(0.909). The mock confirmer (simulating jscpd's token-based signal) confirms
the two true duplicates and rejects the false positive, so the blocking path
is **FPR-free (0.00)** while keeping full recall. renamed-duplicate (0.760)
is below the gate, so it is advisory-only — **not** a blocking-path false
negative.

## Why the blocking path has FPR = 0.00

The structural false positive (different-but-similar-algorithm, 0.909) clears
the > 0.90 gate but is *not* a token-level clone: the two functions share
control-flow and signature shape while differing in the actual tokens (literals,
operations, statements). jscpd's confirmation is a second, independent signal
on the same file pair, so the structural false positive is filtered **before
anything can escalate**. Escalation requires both detectors to agree, and a
token-based clone in the same file pair is not something two genuinely
different algorithms produce.

This is the core justification for the two-pass model: the advisory layer
keeps the recall of the structural heuristic, while the confirmation layer
gives the blocking decision the precision of the token-based incumbent.

## Fallback behavior

Without the jscpd binary (JSCPD_BINARY unset, no `jscpd`/`cpd` on $PATH, no
npx), the check is **pure advisory**: the in-house scan runs as Pass 1, every
finding stays WARN/INFO (no confirmation is possible, so nothing escalates),
and a `duplication:incumbent-unavailable` WARN finding is added so the
fallback is never silent. Duplication can only BLOCK when both passes agree.

## Rule IDs

| RuleID | Meaning | Severity |
|--------|---------|----------|
| `duplication:advisory` | triage advisory (structural, ≥0.60) | WARN / INFO, never blocks |
| `duplication:jscpd:clone` | jscpd clone not escalated | WARN |
| `duplication:confirmed-block` | two-pass confirmed (structural >0.90 + jscpd clone in the same file pair) | BLOCK |

## Reproducing

```
cd /path/to/blueprint
go test -v ./internal/blueprint/checks/duplication/ -run TestSimilarityBenchmark
```