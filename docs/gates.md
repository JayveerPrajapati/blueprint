# Phase Gates (G0–G29)

Blueprint's roadmap is organized as a sequence of **phase gates**. A gate is a
capability milestone — a feature area that must exist, work, and stay working.
Each gate is **backed by at least one test** that proves the capability, so a
gate is never "done" on faith: the test suite is the evidence.

## The authoritative registry

The single source of truth for the gate inventory is the Go package
[`internal/blueprint/gates/`](../internal/blueprint/gates/registry.go):

- `Registry` is a compiled-in `[]Gate` — **not** a JSON/YAML manifest file —
  so `blueprint doctor` reads it with no file I/O and it can never drift from
  the code that ships in the same binary.
- Every gate entry records its `ID` (`G0`–`G29`), `Name`, what it `Verifies`,
  the repo-relative `TestFile` that proves it, the `TestFuncs` in that file,
  and the Go `Package` they live in. Each gate also carries an `Enforcement`
  level — `block`, `warn`, `skip`, or `info` — describing how hard the gate
  fails when violated.

### Bidirectional orphan enforcement

`internal/blueprint/gates/registry_test.go` keeps the mapping honest in both
directions:

- **Forward** (registry → tests): every registry entry's `TestFile` must exist
  on disk and every listed `TestFuncs` entry must be declared somewhere as a
  `func <Name>(...)` test. A gate listed with no test is an orphan.
- **Reverse** (tests → registry): every gate-named test in the repo —
  `func TestG<NN>_...` anywhere, plus G14's special `TestContract*` family in
  `adapters/kern/contract_test.go` and the primary-adapter families
  `TestGitleaks*` (→ G3) and `TestJSCPD*` (→ G6) — must map to a registered
  gate number. A test for a gate with no registry entry is an orphan.

Adding a gate means: add the capability, add the tests, add the registry entry.
The orphan test fails until all three exist.

## Querying the registry

`blueprint doctor --json` emits the full registry alongside its preflight
checks:

```sh
blueprint doctor --json | jq '.gate_count, .gates[0]'
# 30
# {
#   "id": "G0",
#   "name": "Baseline build & vets",
#   "verifies": "Baseline builds+vets, ownership docs exist, exit-code contract",
#   "enforcement": "block",
#   "test_file": "cmd/blueprint/g0_test.go",
#   "package": "main",
#   "test_count": 3
# }
```

The terminal form prints a compact summary:

```sh
blueprint doctor
# ...
# Gates: 30 registered (G0–G29). Run --json for details.
```

## Gate table

`Tests` is the number of test functions listed in the registry entry; `Test location` is the gate's **primary** test file (some `TestFuncs` live in other files of the same package — see note below). `Enforcement` is the gate's severity: `block` hard-fails the build, `warn` is advisory, `info` is informational.

| ID  | Name | Verifies | Enforcement | Test location | Tests |
|-----|------|----------|-------------|---------------|-------|
| G0 | Baseline build & vets | Baseline builds+vets, ownership docs exist, exit-code contract | block | `cmd/blueprint/g0_test.go` | 3 |
| G1 | Validation engine | PASS/BLOCK aggregation, precedence ERROR>BLOCK, JSON, SKIP | block | `internal/blueprint/service/validate_test.go` | 9 |
| G2 | Architecture/boundary enforcement | Architecture/boundary enforcement via kern guard | block | `internal/blueprint/adapters/kern/g2_test.go` | 10 |
| G3 | Secret detection & redaction | Secret detection + redaction (gitleaks adapter primary, kern sec fallback) | block | `internal/blueprint/adapters/kern/g3_test.go` | 22 |
| G4 | Pre-commit hook via CLI | Pre-commit hook: clean/block/JSON/bypass/idempotent/foreign-hook | block | `cmd/blueprint/g4_test.go` | 15 |
| G5 | MCP validate_staged tool | MCP server validate_staged tool | block | `internal/blueprint/mcp/g5_test.go` | 13 |
| G6 | Duplication check | Duplication: precision/recall on 7 fixtures, never-blocks, format | warn | `internal/blueprint/checks/duplication/g6_test.go` | 20 |
| G7 | Agent repair loop & feedback | Agent repair loop + feedback contract (BLOCK carries evidence) | block | `internal/blueprint/adapters/kern/g7_test.go` | 5 |
| G8 | Sandboxed build/test isolation | Sandboxed build/test isolation | block | `internal/blueprint/sandbox/g8_test.go` | 14 |
| G9 | Resilience scenarios | Resilience: injected timeouts, network leakage, cleanup, shell scenarios | warn | `internal/blueprint/resilience/g9_test.go` | 13 |
| G10 | Watcher robustness | Watcher: bursty edits, rename, delete, temp files, CPU idle, shutdown | warn | `internal/blueprint/watcher/g10_test.go` | 14 |
| G11 | CI command | CI command: clean/block PR, determinism, JSON artifact, detached-head no-mutation | block | `cmd/blueprint/g11_test.go` | 10 |
| G12 | Metrics | Metrics: latency benchmarks, persistence, cap, atomic save | info | `internal/blueprint/metrics/g12_test.go` | 7 |
| G13 | Fresh-machine end-to-end | Fresh-machine end-to-end (build+install+MCP+check) | block | `cmd/blueprint/g13_test.go` | 1 |
| G14 | Versioned kern contract | Versioned kern contract, fail-closed | block | `internal/blueprint/adapters/kern/contract_test.go` | 8 |
| G15 | Pre-write validation (validate_proposed) | Pre-write validation for agents via validate_proposed | block | `internal/blueprint/mcp/g15_test.go` | 5 |
| G16 | Source-aware policy | Source-aware policy (source override changes status, never passes block, warn cap) | warn | `internal/blueprint/policy/engine_test.go` | 6 |
| G17 | Doctor preflight | blueprint doctor preflight (env/config/git) | info | `cmd/blueprint/doctor_test.go` | 6 |
| G18 | Policy in MCP handlers | Policy evaluator wired into MCP handlers | block | `internal/blueprint/mcp/g18_test.go` | 3 |
| G19 | Audit trail on validation | Audit trail written on validation | block | `cmd/blueprint/g19_test.go` | 4 |
| G20 | Suppressions & owners | Suppressions + owners policy | info | `internal/blueprint/policy/engine_test.go` | 8 |
| G21 | Duplication on-disk/content-path | Duplication on-disk / content-path warn, confidence=similarity | warn | `internal/blueprint/checks/duplication/g6_test.go` | 5 |
| G22 | blueprint fix command | blueprint fix: proposed fix, confinement, worktree cleanup, JSON | block | `cmd/blueprint/fix_test.go` | 7 |
| G23 | Resilience check wiring | Resilience check wiring / scenarios (YAML) | warn | `cmd/blueprint/g23_test.go` | 2 |
| G24 | Latency budget gate | Latency budget gate, strict-latency hard-fail | block | `cmd/blueprint/g24_test.go` | 4 |
| G25 | Evidence provenance fields | Kern 2.0 evidence provenance fields | info | `cmd/blueprint/g25_test.go` | 3 |
| G26 | Sandbox tests opt-in | Sandbox build/test check behind --tests opt-in | block | `cmd/blueprint/g26_test.go` | 1 |
| G27 | Audit chain linked to kern | audit chain linked to kern's tamper-evident chain via kern audit append | block | `cmd/blueprint/g27_test.go` | 1 |
| G28 | Repair loop via MCP | Repair-loop end-to-end via MCP repair_guidance tool | block | `internal/blueprint/mcp/g28_test.go` | 2 |
| G29 | Approval gate (two-person rule) | high-risk agent changes require explicit human approval before proceeding | block | `cmd/blueprint/approval_cmd_test.go` | 12 |

`TestFile` names each gate's **primary** test file, but a gate's `TestFuncs` are
not required to all live in it: some gates are proven by test functions spread
across more than one file (for example G16 also has tests in
`policy/loader_test.go`, G19 in `service/validate_test.go`, and G3/G6 list the
primary gitleaks/jscpd adapter tests in `adapters/gitleaks/` and
`adapters/jscpd/`). The forward-mapping test resolves every listed function
against the whole repo, so the table's test count is exact even when the file
column is not. The `TestFuncs` list in the registry names every function
exactly, and the orphan test verifies each one against the real test files.