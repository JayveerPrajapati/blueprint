# Blueprint User Guide

This guide walks through using Blueprint end to end: installing it, validating a
change, wiring it into your commit flow, CI, and AI agent, and tuning policy.

Blueprint is a **local change governance engine**. It does not replace your
editor, your tests, or your CI runner — it sits in front of them and answers one
question for every proposed change:

> Is this change allowed by repository policy, structurally consistent, safe,
> and sufficiently validated?

It reuses [kern](https://github.com/JayveerPrajapati/kern) for all code
intelligence (index, call graph, architecture guard, secret scanner) and adds
the governance layer: the change contract, policy orchestration, structured
findings, and enforcement adapters.

---

## Contents

1. [Concepts](#1-concepts)
2. [Install](#2-install)
3. [Your first validation](#3-your-first-validation)
4. [Reading the output](#4-reading-the-output)
5. [Git pre-commit hook](#5-git-pre-commit-hook)
6. [CI / protected-branch enforcement](#6-ci--protected-branch-enforcement)
7. [AI agent integration (MCP)](#7-ai-agent-integration-mcp)
8. [Continuous watcher](#8-continuous-watcher)
9. [Configuration](#9-configuration)
10. [Architecture boundaries](#10-architecture-boundaries)
11. [Secret scanning](#11-secret-scanning)
12. [Duplication oracle](#12-duplication-oracle)
13. [Sandbox validation](#13-sandbox-validation)
14. [Metrics](#14-metrics)
15. [Troubleshooting](#15-troubleshooting)

---

## 1. Concepts

**Change request.** Blueprint normalizes any change into a `ChangeRequest`: a
list of file changes plus metadata (source = agent / ide / human / refactor /
dep-bot / ci). The same request flows through every adapter.

**Validation result.** Every validation returns a `ValidationResult` with
`Findings` and per-check `CheckResult`s plus an overall `Status`:

| Status | Meaning | Exit code |
|---|---|---|
| `PASS` | No blocking findings | `0` |
| `BLOCK` | At least one block-level finding | `1` |
| `ERROR` | A check or the runtime failed | `2` |
| `SKIP` | All checks were skipped | `0` |

**One canonical engine.** The CLI, the git hook, CI, and the MCP server all call
the same `BlueprintService.Validate`. There is no second validation pipeline.

**Enforcement chain.** Blueprint enforces at four layers:

```
pre-commit hook  →  CI  →  protected branch policy  →  (advisory) watcher
   local gate        org gate      hard gate              feedback only
```

The hook is a local gate and **can be bypassed** with `git commit --no-verify`
(this is documented, not a bug). CI and branch protection provide the
organizational guarantee.

---

## 2. Install

### Prerequisites

- **Go 1.27+** to build.
- The **`kern` binary** on `$PATH`. Blueprint invokes kern as a subprocess for:
  - `kern guard check` — architecture boundary checks
  - `kern sec` — secret scanning fallback (gitleaks is the primary secret detector)
  - sandbox build/test — `go build ./...` and `go test ./...` directly in an isolated git worktree (no kern subprocess)
- **gitleaks** and **jscpd** binaries (optional but recommended) — the primary
  secret and duplication detectors. When absent, Blueprint falls back to the
  in-house checks and reports a WARN `*-incumbent-unavailable` finding.

### Build

```sh
git clone <this-repo> blueprint && cd blueprint

go build -o blueprint       ./cmd/blueprint
go build -o blueprint-mcp   ./cmd/blueprint-mcp
```

Optionally install both to `$PATH`:

```sh
go build -o "$(go env GOPATH)/bin/blueprint"     ./cmd/blueprint
go build -o "$(go env GOPATH)/bin/blueprint-mcp" ./cmd/blueprint-mcp
```

### Point Blueprint at kern

Blueprint resolves the kern binary in this order:

1. `$KERN_BINARY` (absolute path)
2. `$PATH`
3. `../kern/bin/kern` relative to the working directory

```sh
export KERN_BINARY=/path/to/kern   # only if kern is not on $PATH
blueprint version                  # sanity check
```

If you see an error like `kern binary not found`, set `KERN_BINARY` or put kern
on `$PATH`.

---

## 3. Your first validation

```sh
cd /path/to/your/repo

# make a change
echo "package main" > hello.go
git add hello.go

# validate the staged change (terminal output by default)
blueprint check --staged
```

`check` validates **staged** changes (`git diff --cached`). `--staged` is the
default behavior; the flag exists for hook clarity.

Useful flags:

| Flag | Purpose |
|---|---|
| `--staged` | Check staged changes (default) |
| `--format=terminal\|json` | Output format (default: `terminal`) |
| `--json` | Shorthand for `--format=json` |
| `--repo DIR` | Repository root (default: current directory) |
| `--source agent\|ide\|human\|refactor\|dep-bot\|ci` | Who/what is making the change |
| `--agent-id ID` | Agent identity for the change; enables the authz gate (defaults to `$BLUEPRINT_AGENT_ID`, then a stable host-derived id) |

Exit code `0` = PASS. `1` = BLOCK. `2` = ERROR. `3` = bad config. `4` =
unsupported environment. See [Exit codes](README.md#exit-codes).

---

## 4. Reading the output

A finding looks like:

```
Rule: frontend-no-db
File: frontend/components/Header.tsx:21
Reason: frontend layer cannot depend on database infrastructure
Suggested fix: call the approved application/API boundary
Evidence: import edge -> backend/database
```

Every actionable `BLOCK` includes: file and line/column, what failed, why it
failed, and a suggested fix (when `feedback.include_suggestions: true`). Blueprint
deliberately avoids vague messages like "Architecture error".

For machine consumption, use `--json`:

```sh
blueprint check --staged --json
```

The JSON shape is the same `ValidationResult` the MCP server and CI emit, so
tooling written against one works against all.

**Secrets are always redacted.** A secret finding reports the category (e.g.
`AWS credential`) and `[REDACTED]` for the value — never the secret itself, in
terminal, JSON, or log output.

---

## 5. Git pre-commit hook

Install a pre-commit hook that runs `blueprint check --staged` on every commit:

```sh
blueprint install hook
```

The hook is a **thin adapter** — it contains no validation logic, just:

```sh
#!/bin/sh
# Blueprint pre-commit hook — thin adapter to `blueprint check --staged`
exec blueprint check --staged --format=terminal
```

Behavior:

- **Refuses to overwrite a foreign hook.** If `.git/hooks/pre-commit` already
  exists and is not a Blueprint hook, install aborts with exit code `2` and
  tells you to remove it first. Re-running `blueprint install hook` is idempotent.
- **Bypass is documented.** `git commit --no-verify` skips the hook. This is
  intentional — the real guarantee comes from CI + branch protection.
- **Worktree-aware.** `install hook` finds the git dir even inside linked
  worktrees.

To remove the hook, just delete the file:

```sh
rm .git/hooks/pre-commit
```

See [git-hook-guide](git-hook-guide.md) for more.

---

## 6. CI / protected-branch enforcement

CI validates the full base..head diff in a clean environment with **no local
daemon state**:

```sh
blueprint ci --base main --head HEAD
```

Flags:

| Flag | Default | Purpose |
|---|---|---|
| `--repo DIR` | current dir | repository root |
| `--base main` | `main` | base revision (branch/tag/sha) |
| `--head HEAD` | `HEAD` | proposed revision |
| `--artifact-file FILE` | `blueprint-result.json` | JSON artifact path |
| `--json` | off | also emit JSON to stdout |
| `--no-human` | off | suppress human-readable summary on stderr |

The JSON artifact is always written to `--artifact-file`, so CI systems can
attach or upload it. The run is deterministic across clean runners — it
reconstructs the validation context from the repository checkout alone.

Example GitHub Actions step:

```yaml
- name: Blueprint CI check
  run: |
    blueprint ci --base origin/main --head HEAD --artifact-file blueprint-result.json
- name: Upload Blueprint artifact
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: blueprint-result
    path: blueprint-result.json
```

See [ci-guide](ci-guide.md) for full CI configurations.

---

## 7. AI agent integration (MCP)

Blueprint ships an MCP server (`blueprint-mcp`) that exposes four tools agents
can call **before** writing:
| Tool | Purpose |
|---|---|
| `blueprint_validate_staged` | Validate staged changes; returns the same `ValidationResult` as the CLI |
| `blueprint_validate_proposed` | Validate proposed (not-yet-written) file content against policy before staging |
| `blueprint_explain_finding` | Return a structured explanation + suggested fix for one finding |
| `blueprint_repair_guidance` | Return a structured, machine-readable repair contract for a finding (rule_id, what failed, why, suggested fix, evidence, re-validation hint) |

The server wires a **PreToolUse gate** (`WithPreToolHook`) that runs before every
tool handler. The default gate confines tool targets to workspace roots from
`BLUEPRINT_ROOTS` (default: the process working directory) — the same model as
kern's `KERN_ROOTS`. A tool call whose target escapes the configured roots is
denied before any side effect.

```sh
# run the MCP server (stdio transport)
blueprint-mcp

# optionally restrict the roots the server may touch
export BLUEPRINT_ROOTS=/path/to/repo:/path/to/other-repo
```

The repair loop works like this:

1. Agent stages a change and calls `blueprint_validate_staged`.
2. If `BLOCK`, the agent reads the structured findings, calls
   `blueprint_explain_finding` for any unclear ones, and produces a repair patch.
3. Agent re-stages and calls `blueprint_validate_staged` again.
4. Repeat until `PASS`.

If the MCP integration cannot guarantee safe pre-write interception for a given
agent, Blueprint treats it as **post-write validation** and documents that — it
never falsely represents an advisory hook as a hard boundary.

See [mcp-integration-guide](mcp-integration-guide.md) for connection details.

---

## 8. Continuous watcher

`blueprint watch` runs an advisory file-change watcher in the foreground:

```sh
blueprint watch
```

Flags:

| Flag | Default | Purpose |
|---|---|---|
| `--repo DIR` | current dir | repository root |
| `--interval DURATION` | `500ms` | polling interval |
| `--debounce DURATION` | `1s` | quiet period before emitting events |
| `--strict` | off | exit non-zero on any violation |
| `--policy architecture,secrets` | all | comma-separated policies to run |

The watcher is **advisory only**. It debounces bursty edits, ignores editor temp
files and generated files, and emits warnings/errors as you work. It is not a
pre-write filesystem firewall — filesystem events are not transactional write
interception, and Blueprint does not pretend otherwise.

See [demo-and-examples](demo-and-examples.md) for watcher scenarios.

---

## 9. Configuration

Blueprint reads an **optional** `.blueprint/config.yaml` in the repository root.
If absent, conservative defaults apply. If present but invalid, Blueprint fails
with exit code `3` rather than silently proceeding.

```yaml
# .blueprint/config.yaml
version: 1
mode: enforce                # enforce | warn | off

policies:
  architecture: block        # block | warn | skip
  secrets:      block
  duplication:  warn
  tests:        block
  resilience:   warn

execution:
  timeout_seconds: 300       # max 3600
  max_output_bytes: 1048576  # max 10485760 (10 MiB)

feedback:
  format: json
  include_suggestions: true
```

| Mode | Meaning |
|---|---|
| `enforce` | Policies enforce normally; `block` findings block. |
| `warn` | Global downgrade: BLOCK findings become WARN. Never downgrades to PASS. |
| `off` | Validation is effectively disabled. |

| Policy value | Meaning |
|---|---|
| `block` | Findings of this category can block the change (exit code 1). |
| `warn` | Findings are reported; the change still passes. |
| `skip` | The policy is not run. |

A partial config is overlaid on the defaults — specify only what you want to
change. Default policies: `architecture: block`, `secrets: block`,
`duplication: warn`, `tests: block`, `resilience: warn`.

See [configuration-reference](configuration-reference.md) for the full schema.

---

## 10. Architecture boundaries

Architecture enforcement uses kern's boundaries, declared in
`.kern/boundaries.json`. **The schema uses an `"action"` field**, not a boolean:

```json
{
  "rules": [
    { "from": "frontend", "to": "db",      "action": "forbid" },
    { "from": "frontend", "to": "backend", "action": "forbid" }
  ]
}
```

> A rule written as `{ "from": "frontend", "to": "db", "forbid": true }` (no
> `"action"` field) is **silently ignored**. Always use `"action": "forbid"`.

By default Blueprint reports **new violations introduced by the change** — a
pre-existing violation in an untouched file will not make an unrelated commit
suddenly fail. Note: strict mode (`NewArchitectureCheckStrict`) exists in the
API but is not yet exposed via CLI flags. It may be enabled in a future release.

Blueprint delegates the actual scanning to `kern guard check --file <staged>`
and reports only the violations kern finds. It does not maintain a second
import scanner or architecture parser.

---

## 11. Secret scanning

Secret detection is delegated to **gitleaks** (primary); when gitleaks is
unavailable, Blueprint falls back to the in-house `kern sec` scanner (WARN
`secret:incumbent-unavailable`, never a silent pass). Blueprint:

- detects secrets before the final enforcement point;
- **never echoes the secret** into agent feedback or logs — values are replaced
  with `[REDACTED]`;
- identifies the secret category (AWS credential, private key, token, …) where
  possible;
- supports an allowlist for test fixtures and known placeholders so test
  fixtures named without the `_test.go` suffix are not flagged.

A secret finding:

```
Rule: secret-detected
File: config/aws.go:42
Type: AWS credential
Value: [REDACTED]
Suggested fix: move credential to runtime secret storage
```

Redaction applies in terminal output, JSON output, and audit/log entries.

---

## 12. Duplication oracle

Primary duplication detection is delegated to **jscpd** (`duplication:jscpd`).
When jscpd is unavailable, the in-house advisory fallback compares new
functions against existing ones using **structural fingerprints** (signature
shape + control-flow vector + called symbols + size), not raw text equality.

```
WARN  duplicate-candidate
new:      payments/retry.go::retryRequest
existing: shared/retry.go::RetryRequest
similarity: 0.92
suggestion: reuse shared/retry.go::RetryRequest
```

This check is **advisory only** — `duplication:advisory` emits `WARN` (or `INFO`
for low similarity) regardless of the score, and never blocks on its own. It can
only escalate to `duplication:confirmed-block` (BLOCK) when an in-house candidate
above 0.90 is also confirmed as a jscpd clone in the same file pair. Tuning that
threshold is a deliberate policy decision you make after benchmarking
false-positive rates on your own codebase; Blueprint does not do it for you.

---

## 13. Sandbox validation

For checks that need to execute the change (build, test), Blueprint runs them in
an **isolated git worktree** with:

- a separate worktree (the main working tree is never mutated);
- a sanitized environment (secrets stripped);
- a hard timeout;
- capped stdout/stderr capture;
- process-group cancellation so child processes are cleaned up.

If the sandboxed command fails, the worktree is cleaned up and the repository is
left untouched. You can confirm this: after a failing sandbox run, `git status`
in your real working tree shows no changes introduced by Blueprint.

Do not assume "worktree" automatically means "security sandbox" — filesystem
isolation, process isolation, and network isolation are separate properties.
Blueprint is explicit about which it provides.

---

## 14. Metrics

Blueprint records local-only metrics to `.blueprint/metrics.json`:

```sh
blueprint metrics                 # human-readable
blueprint metrics --json          # machine-readable
blueprint metrics --reset         # zero all counters
```

Tracked: validation counts (pass / block / warn / error), per-check latency
(p50 / p95), sandbox latency, repair attempts, and overrides. There is **no
cloud telemetry** — metrics stay on the local machine.

---

## 15. Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `kern binary not found` | Set `KERN_BINARY` or put kern on `$PATH`. |
| Exit code `3` | `.blueprint/config.yaml` is invalid. Check `version`, `mode`, policy keys/values. |
| Pre-existing violation blocks an unrelated commit | Use non-strict mode (default) which reports only new violations; or fix the pre-existing violation. |
| Hook did not run | Confirm `.git/hooks/pre-commit` is executable and is a Blueprint hook. `blueprint install hook` is idempotent. |
| `git commit` bypassed the hook | `--no-verify` skips it by design. Enforce in CI instead. |
| Test fixture flagged as a secret | Use the `_test.go` suffix for fixture helpers, or add to the secret allowlist. |
| boundaries rule ignored | Use `"action": "forbid"`, not `"forbid": true`. See [§10](#10-architecture-boundaries). |
| MCP tool denied | The target path is outside `BLUEPRINT_ROOTS` (default: cwd). Set `BLUEPRINT_ROOTS` to include it. |

More in [troubleshooting](troubleshooting.md) and [limitations](limitations.md).

---

## Where to go next

- [Policy reference](policy-reference.md) — every policy, threshold, and suppression
- [Security model](security-model.md) — trust boundaries and the enforcement chain
- [Demo and examples](demo-and-examples.md) — end-to-end worked examples
