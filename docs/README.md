# Blueprint

Blueprint is a **local change governance engine** built on top of [kern](https://github.com/JayveerPrajapati/kern). It orchestrates detection (delegated to incumbents like gitleaks and jscpd), emits structured provenance-backed verdicts, drives a structured repair loop for AI agents, and writes a tamper-evident audit trail — enforced at every surface where changes enter a repository: pre-commit hooks, MCP tools for AI agents, CI / protected-branch checks, and a continuous advisory watcher.

Everything runs locally. Blueprint never sends code, findings, or telemetry to the cloud.

## What Blueprint does

- **Policy orchestration** — one validation pipeline (architecture, secrets, duplication, sandbox, resilience), one set of structured findings, enforced identically at every surface (pre-commit, MCP, CI, watcher) through the single `service.Validate()` engine. Detection is delegated; the decision is centralized.
- **Provenance-emitting verdicts** — every finding is structured (`rule_id`, `severity`, `category`, `file`, `line`, `evidence`, `suggested_fix`) and redacted — never raw text. See [findings-format](findings-format.md).
- **Structured repair loop** — the `blueprint_repair_guidance` MCP tool returns a machine-readable repair contract so agents fix BLOCKs programmatically. See [agent-repair-loop](agent-repair-loop.md).
- **Tamper-evident audit trail** — every validation appends a self-hashed JSONL record to `.blueprint/audit/audit.jsonl`, best-effort chained into kern's tamper-evident hash chain via `kern audit append`.
- **Architecture guard** — enforces dependency boundaries declared in `.kern/boundaries.json` (`architecture:boundary-violation`). New changes are scoped to the staged files (strict mode exists in the API but is not yet exposed via CLI flags).
- **Secret scanning** — delegated to gitleaks (primary, `secret:gitleaks`); in-house `kern sec` fallback (`secret:hardcoded-secret`) when gitleaks is unavailable. Secret material is redacted from all output and never echoed back to agents.
- **Duplication oracle** — primary detection delegated to jscpd (`duplication:jscpd`); the in-house structural-fingerprint triage (`duplication:advisory`) is advisory-only WARN/INFO and escalates to `duplication:confirmed-block` (BLOCK) only when a candidate above 0.90 is also confirmed as a jscpd clone. Benchmark: `docs/duplication-benchmark.md`.
- **MCP integration** — exposes `blueprint_validate_staged`, `blueprint_validate_proposed`, `blueprint_explain_finding`, and `blueprint_repair_guidance` so AI agents can validate changes before writing them and repair BLOCKs programmatically (advisory, opt-in).
- **CI enforcement** — `blueprint ci` validates a base..head diff in clean environments with no local daemon state, emitting a JSON artifact for CI systems.
- **Sandbox** — runs untrusted validation commands in isolated git worktrees with timeouts, output caps, and process-group cancellation.
- **Watcher** — continuous advisory file-change watcher that emits debounced events as you work.
- **Metrics** — local-only validation metrics (pass/block/warn/error counts, latency percentiles, repair-success rate) stored in `.blueprint/metrics.json`.

## Quick start

1. Install the `kern` binary and make sure it is on your `$PATH` (or set `KERN_BINARY`). See [installation](installation.md).
2. Build Blueprint from source: `go build -o blueprint ./cmd/blueprint`
3. Stage some changes and validate them:

```sh
git add .
blueprint check
```

The config file is optional — `.blueprint/config.yaml` with conservative defaults applies when none exists. See [configuration-reference](configuration-reference.md).

## Enforcement layers

| Layer | Command / tool | Enforces |
|---|---|---|
| Pre-commit hook | `blueprint install hook` | `blueprint check --staged` on every commit |
| MCP (agents) | `blueprint_validate_staged` | Voluntary validation before writes |
| CI / protected branch | `blueprint ci --base main --head HEAD` | Base..head diff validation |
| Watcher | `blueprint watch` | Continuous advisory feedback |

The enforcement chain is **hook + CI + protected branch**: the hook catches violations locally, and CI catches anything that bypasses it. See [git-hook-guide](git-hook-guide.md) and [ci-guide](ci-guide.md).

## Exit codes

| Code | Meaning |
|---|---|
| 0 | PASS |
| 1 | Policy violation / BLOCK. `blueprint fix` exits 1 while ANY finding — WARN or BLOCK — remains; the repair loop must iterate |
| 2 | Tool / runtime / configuration / usage ERROR |
| 3 | Invalid Blueprint configuration |
| 4 | Unsupported operation or environment |

## Command overview

```text
blueprint version                        Print the version
blueprint check [--staged] [--format=terminal|json] [--json] [--tests] [--resilience] [--isolate-network] [--allow-unisolated] [--require-kern]
blueprint install hook                   Install a pre-commit hook
blueprint watch [--strict] [--policy architecture,secrets] [--interval] [--debounce]
blueprint ci [--repo DIR] [--base main] [--head HEAD] [--artifact-file FILE] [--json] [--no-human] [--strict-latency]
blueprint metrics [--repo DIR] [--json] [--reset]
blueprint fix [--repo DIR] [--file FILE] [--content ...]
               Validate agent-proposed fixes in an isolated worktree
blueprint verify-receipt [--repo DIR]
               Verify a tamper-evident CI receipt (receipt + audit chain)
blueprint request-approval
               Request human approval for a high-risk change (two-person rule)
blueprint approve <id> [--reason ...]    Approve a pending approval request
blueprint reject <id> [--reason ...]     Reject a pending approval request
```

## Documentation

- [Installation](installation.md) — prerequisites and building from source
- [Configuration reference](configuration-reference.md) — `.blueprint/config.yaml`
- [Policy reference](policy-reference.md) — boundaries, secrets, duplication, suppression
- [Phase gates](gates.md) — the authoritative G0–G29 gate registry and how to query it
- [MCP integration guide](mcp-integration-guide.md) — tools for AI agents
- [Agent repair loop](agent-repair-loop.md) — propose→validate→repair→re-validate cycle for AI agents
- [Git hook guide](git-hook-guide.md) — pre-commit enforcement
- [CI guide](ci-guide.md) — protected-branch enforcement and CI config