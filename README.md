<div align="center">

A **local change governance engine** built on top of [kern](https://github.com/JayveerPrajapati/kern).

**Change-governance firewall for the kern ecosystem**

orchestration · verdicts · provenance · repair · audit · approval

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Language: Go](https://img.shields.io/badge/Language-Go_1.27+-blue.svg)](https://go.dev/)
[![Telemetry: None](https://img.shields.io/badge/Telemetry-None-brightgreen.svg)](#telemetry--privacy)
[![Network: none](https://img.shields.io/badge/Network-none-brightgreen.svg)](#telemetry--privacy)

**7 checks · 4 enforcement surfaces · 4 MCP tools · 12 CLI commands**

</div>

## Quick Start

```bash
# 1. Install
go install github.com/JayveerPrajapati/blueprint/cmd/blueprint@latest

# 2. Enforce on every commit (optional but recommended)
cd your-project && blueprint install hook

# 3. Validate your change
git add -A && blueprint check --staged
```

`0` = PASS, `1` = BLOCK, `2` = ERROR — see the [exit-code table](#exit-codes).
Full walkthrough in [Get Started](#get-started).

## Contents

- [Quick Start](#quick-start)
- [Get Started](#get-started)
- [Why Blueprint?](#why-blueprint)
- [What It Checks](#what-it-checks)
- [How It Works](#how-it-works)
- [CLI Reference](#cli-reference)
- [MCP Tools](#mcp-tools)
- [Enforcement Layers](#enforcement-layers)
- [Configuration](#configuration)
- [Telemetry & Privacy](#telemetry--privacy)
- [Supported Platforms](#supported-platforms)
- [Development](#development)
- [Documentation](#documentation)
- [Troubleshooting](#troubleshooting)
- [License](#license)

## Get Started

### Prerequisites

- **Go 1.27+** to build.
- **kern** — Blueprint auto-installs the latest kern when it is not found.
  If you already have kern, Blueprint requires **>= v0.9.0** (enforced
  automatically; you'll see a clear upgrade message if it's too old).
- **git** — a git repository is required (Blueprint validates changes).

### 1. Install

**Standard install** (Go 1.27+):

```bash
go install github.com/JayveerPrajapati/blueprint/cmd/blueprint@latest
go install github.com/JayveerPrajapati/blueprint/cmd/blueprint-mcp@latest
```

<details>
<summary><b>Other install methods</b></summary>

| Method | Command | Notes |
|---|---|---|
| **build locally** | `make build` → `bin/blueprint`, `bin/blueprint-mcp` | Version injection via ldflags |
| **go build** | `go build -o blueprint ./cmd/blueprint` | Plain build, prints `dev` version |

</details>

Blueprint resolves the kern binary in this order: `$KERN_BINARY` → `$PATH` →
`../kern/bin/kern` relative to the working directory.

### 2. Wire the MCP server (optional, for agents)

**Claude Code:**

```bash
claude mcp add blueprint -- blueprint-mcp
```

**Cursor / VS Code** — add to your project's `.mcp.json`:

```json
{
  "mcpServers": {
    "blueprint": { "command": "blueprint-mcp", "args": [] }
  }
}
```

### 3. Set architecture boundaries (optional, recommended)

```bash
mkdir -p .kern
cat > .kern/boundaries.json <<'EOF'
{
  "rules": [
    { "from": "frontend", "to": "db",      "action": "forbid" },
    { "from": "frontend", "to": "backend",  "action": "forbid" }
  ]
}
EOF
```

### 4. Validate

```bash
git add -A && blueprint check --staged
```

Exit code `0` means PASS. See the [exit-code table](#exit-codes) below.

---

## Why Blueprint?

Detection is a commodity — gitleaks (150+ secret types), jscpd (223 languages),
and incumbents win on coverage. Blueprint's value is the layer above:

- **Orchestration** — one pipeline, one set of structured findings, enforced at
  every surface (pre-commit, MCP, CI, watcher). Detection is delegated; the
  governance is Blueprint's.
- **Provenance** — every verdict carries rule_id, severity, file, line,
  evidence, and a suggested fix. Never raw text. Always redacted.
- **Repair** — the `blueprint_repair_guidance` MCP tool returns a structured
  repair contract so agents fix findings programmatically, not by guessing.
  See [docs/agent-repair-loop.md](docs/agent-repair-loop.md).
- **Audit** — every validation writes a self-hashed record, best-effort linked
  into kern's tamper-evident hash chain. Tampering with any record breaks the
  chain.
- **Approval** — high-risk agent changes gate behind a two-person rule:
  request → approve → re-run with `--approval-id`.

Blueprint does **not** reimplement code intelligence. It reuses kern's index,
call graph, `kern guard`, and `kern sec` and adds the governance layer on top.

Every change runs through the same policy pipeline. Beyond detection, two
governance gates are enforced at every surface: **agent authorization** (who
may change) and the **two-person approval** rule (high-risk agent changes need
a human approver).

---

## What It Checks

| Check | Category | Source of truth | Default | Posture |
|---|---|---|---|---|
| **Architecture boundaries** | `architecture` | `.kern/boundaries.json` via `kern guard` | **block** | Enforced; WARN if `.kern/` missing |
| **Agent authorization** | `authz` | kern `authz_verdict` (contract v2) | **block** | Unauthorized agent identities blocked |
| **Hardcoded secrets** | `secrets` | gitleaks (primary) · `kern sec` (fallback) | **block** | Delegated to incumbent; redacted |
| **Structural duplication** | `duplication` | jscpd (primary) · in-house advisory (fallback) | **warn** | Advisory-only |
| **Sandbox build/test** | `tests` | `go build ./...` + `go test ./...` in isolated worktree | **block** | Network-isolated (Linux) |
| **Resilience scenarios** | `resilience` | chaos scenarios (Go ecosystem) | **warn** | Opt-in (`--resilience`) |
| **Two-person approval** | `approval` | risk classifier + JSONL approval store | **block** | High-risk changes block until approved |

---

## How It Works

```text
        ┌─────────────────────────────────────┐
        │  AI Agent · IDE · Human             │
        └──────────────────┬──────────────────┘
                           │
                           ▼
        ┌─────────────────────────────────────┐
        │  MCP · CLI · Git hook · CI          │
        └──────────────────┬──────────────────┘
                           │
                           ▼
        ┌─────────────────────────────────────┐
        │          Blueprint Pipeline         │
        │                                     │
        │  architecture        kern guard     │
        │  secrets             gitleaks       │
        │  duplication         jscpd          │
        │  sandbox             go test        │
        │  resilience          chaos          │
        └──────────────────┬──────────────────┘
                           │
                           ▼
        ┌─────────────────────────────────────┐
        │         PASS / BLOCK / WARN         │
        │         structured · audited        │
        └─────────────────────────────────────┘
```

Blueprint orchestrates detection (delegated to incumbents) into a single
decision, provenance, repair, audit, and approval pipeline. After T2.1,
detection itself is delegated to incumbents (gitleaks, jscpd); Blueprint keeps
no second parser for detection — it consumes incumbents' structured output and
adds the decision/provenance/repair/audit layer.

---

## CLI Reference

```text
blueprint version                        Print the version
blueprint doctor   [--repo DIR] [--json] Preflight environment / configuration diagnostic

blueprint check   [--staged] [--format=terminal|json] [--json]
                  [--repo DIR] [--source agent|ide|human|refactor|dep-bot|ci]
                  [--tests] [--resilience] [--isolate-network]
                  [--allow-unisolated] [--require-kern]
                 → Validate staged changes against policy

blueprint install hook                   Install a pre-commit hook (thin adapter)

blueprint watch    [--strict] [--policy architecture,secrets]
                  [--interval DURATION] [--debounce DURATION]
                 → Continuous advisory file watcher

blueprint ci       [--repo DIR] [--base main] [--head HEAD]
                  [--artifact-file FILE] [--json] [--no-human] [--strict-latency]
                 → Validate base..head diff (CI / protected branch)

blueprint metrics  [--repo DIR] [--json] [--reset]
                 → Show or reset local metrics

blueprint fix      [--repo DIR] [--file FILE] [--content ...]
                 → Validate agent-proposed fixes in an isolated worktree

blueprint verify-receipt [--repo DIR]
                 → Verify a tamper-evident CI receipt (receipt + audit chain)

blueprint request-approval
                 → Request human approval for a high-risk change (two-person rule)

blueprint approve  <id> [--reason ...]   Approve a pending approval request
blueprint reject   <id> [--reason ...]   Reject a pending approval request
```

### Exit codes

| Code | Meaning |
|---|---|
| `0` | PASS — no BLOCK or WARN findings |
| `1` | BLOCK — policy violation. `blueprint fix` exits 1 while ANY finding — WARN or BLOCK — remains; the repair loop must iterate |
| `2` | ERROR — tool / runtime / configuration / usage error |
| `3` | Invalid Blueprint configuration |
| `4` | Unsupported operation or environment |

---

## MCP Tools

When running as an MCP server (`blueprint-mcp`), Blueprint exposes **4
governance-aware tools**:

| Tool | Description |
|---|---|
| `blueprint_validate_staged` | Validate staged git changes against policy. Returns structured `ValidationResult`. |
| `blueprint_validate_proposed` | Validate proposed file content (not yet on disk) against policy. |
| `blueprint_explain_finding` | Explain a finding in plain language with remediation guidance. |
| `blueprint_repair_guidance` | Return a structured repair contract so agents fix findings programmatically. |

Blueprint's MCP server is **governance-aware and auditable** — not just another
code MCP. Three properties set it apart:

- **Governance-aware** — returns structured findings (rule_id, severity,
  evidence, suggested_fix), never raw text. BLOCK/WARN policy is enforced at
  the tool level.
- **Auditable** — every validation writes a self-hashed record, best-effort
  linked into kern's tamper-evident hash chain.
- **Sandboxed** — CI runs in an isolated git worktree; the pre-commit hook
  runs locally with no network access to external services.

---

## Enforcement Layers

Blueprint enforces at four surfaces. Three are binding; the watcher is
advisory.

| Layer | Command / tool | Enforces |
|---|---|---|
| **Pre-commit hook** | `blueprint install hook` | runs `blueprint check --staged` on every commit |
| **MCP (agents)** | `blueprint_validate_staged`, `blueprint_validate_proposed`, `blueprint_explain_finding` | voluntary validation before writes |
| **CI / protected branch** | `blueprint ci --base main --head HEAD` | base..head diff validation, JSON artifact |
| **Watcher** | `blueprint watch` | continuous advisory feedback |

The enforcement chain is **hook + CI + protected branch**. The local hook can
be bypassed with `git commit --no-verify` — this is documented and expected, not
a bug. CI and branch protection are what make enforcement organizational.

---

## Configuration

Blueprint reads an **optional** `.blueprint/config.yaml`. If absent,
conservative defaults apply. If present but invalid, it fails with exit code `3`.

```yaml
# .blueprint/config.yaml
version: 1
mode: enforce                # enforce | warn | off

policies:
  architecture: block
  secrets:      block
  duplication:  warn
  tests:        block

sources:
  agent:
    duplication: warn        # override per-source (spec P0-3)

execution:
  timeout_seconds: 300
  max_output_bytes: 1048576

feedback:
  format: json
  include_suggestions: true

approval:
  enabled: true
  require_for_sources: [agent]
  require_for_risk_levels: [high]
  sensitive_paths: [".kern/**", "*.pem", "*.key"]
```

---

## Telemetry & Privacy

Blueprint collects **zero telemetry**. No network calls, no analytics, no
phoning home. All validation runs locally. The only network activity is
`go install` fetching modules (standard Go module proxy).

---

## Supported Platforms

Any platform with Go 1.27+: `go install` or build from source.

---

## GitHub Action

Protect your `main` branch without any local setup — the action installs
blueprint, kern, and gitleaks on the runner, then gates every pull request:

```yaml
- uses: JayveerPrajapati/blueprint/.github/actions/blueprint-ci@v0.1.0
  with:
    blueprint-ref: main    # or a pinned tag
    kern-ref: main         # or a pinned tag
```

The action runs `blueprint ci` and uploads the JSON artifact as a workflow
artifact. Combine with branch protection rules to block merges on BLOCK
findings.

---

## Development

### Prerequisites

- Go 1.27+
- git
- kern (for gate tests)

### Build

```bash
make build   # → bin/blueprint, bin/blueprint-mcp
```

### Test

```bash
go test ./...
```

### Version injection

```bash
go build -ldflags "-X github.com/JayveerPrajapati/blueprint/internal/blueprint/version.Version=$(git describe --tags --always --dirty)" \
  -o bin/blueprint ./cmd/blueprint
```

Without ldflags, the version prints `dev`.

### Project Layout

```text
cmd/blueprint/         CLI: check, doctor, fix, install hook, watch, ci, metrics, approval
cmd/blueprint-mcp/     MCP server binary
internal/blueprint/    Core logic: adapters, sandbox, version, config
internal/blueprint/adapters/kern/   Kern integration
internal/blueprint/sandbox/         Isolated worktree + network namespace
internal/blueprint/version/         Build version + kern version parsing
```

---

## Documentation

- [docs/blueprint-spec.md](docs/blueprint-spec.md) — full specification
- [docs/agent-repair-loop.md](docs/agent-repair-loop.md) — repair contract for agents
- [CHANGELOG.md](CHANGELOG.md) — release history

---

## Troubleshooting

**"kern not found"**
Blueprint auto-installs kern. If it fails, install manually:
```bash
go install github.com/JayveerPrajapati/kern/cmd/kern@latest
```

**"network isolation unavailable"**
On Linux, network namespace isolation requires CAP_SYS_ADMIN. If unavailable,
Blueprint runs without isolation and emits a WARN finding.

**Exit code 3**
Invalid `.blueprint/config.yaml`. Run `blueprint doctor` to diagnose.

---

## License

MIT — see [LICENSE](LICENSE).
