# MCP Integration Guide

## Why this MCP?

Most MCP servers are thin wrappers that execute with the user's full privileges
and leave no trace. Blueprint's MCP server is **governance-aware, auditable,
and sandboxed**:

- **Governance-aware** — `blueprint_validate_staged` returns structured findings
  (rule_id, severity, evidence, suggested_fix), never raw text. BLOCK/WARN
  policy is enforced at pre-commit, MCP, CI, and the watcher; the repair loop
  returns structured contracts so agents fix findings programmatically.
- **Auditable** — every validation writes a self-hashed record, best-effort
  linked into kern's tamper-evident hash chain; tampering with any record
  breaks the chain.
- **Sandboxed** — the pre-commit hook gates changes before they land and CI
  runs in an isolated git worktree; network isolation on Linux with WARN (not
  silent) when unavailable.

Blueprint ships an MCP (Model Context Protocol) server that exposes validation as tools for AI agents (e.g. code assistants). Agents call these tools voluntarily before writing changes; Blueprint never intercepts file writes.

## Tools

### `blueprint_validate_staged`

Validate the staged git changes in a repository against Blueprint policy (architecture boundaries, secrets).

**Arguments:**

| Name | Type | Description |
|---|---|---|
| `repo` | string | Absolute path to the repository root. Defaults to the server's working directory. |
| `source` | string | Change source: `agent`, `ide`, `human`, `refactor`, `dep-bot`, `ci`. Defaults to `agent`. |

**Returns:** a structured `ValidationResult` — status, exit code, and findings (rule ID, severity, category, file, line, message, explanation, suggested fix), plus a per-leg verdict section (`leg_verdicts` + `verdict_basis`) so the caller sees each check's verdict individually and which legs can block vs which are advisory-only. The result is machine-readable so the agent can act on it programmatically.

### `blueprint_explain_finding`

Get a human-readable explanation and suggested fix for a single finding.

**Arguments:**

| Name | Type | Description |
|---|---|---|
| `finding` | object (required) | A `Finding` object from a `ValidationResult`. |

**Returns:** text describing the finding, its severity, and a suggested fix. For advisory-only findings (e.g. `duplication:advisory`, `duplication:jscpd:clone`) the explanation explicitly notes that the leg cannot block, so the agent does not over-correct for a WARN that can never gate a change.

## Per-leg verdicts — blocking vs advisory

"Validate" means **run all checks** — the tool executes every registered check
and reports the aggregate verdict plus each leg's individual verdict. The
aggregate `status` alone does not tell you *which* checks produced it; a PASS
(or WARN) can hide advisory findings. Every validate response therefore
carries a per-leg verdict section:

| Field | Description |
|---|---|
| `leg_verdicts` | One entry per check family: `{leg, verdict, kind}`. |
| `verdict_basis` | One-line summary of which legs contributed to the verdict (blocking vs advisory). |

Each leg is classified honestly by `kind`:

| Leg | Kind | Meaning |
|---|---|---|
| `architecture` | `blocking` | Can produce `BLOCK` (boundary violations, incl. projected-import pre-write checks). |
| `secret` | `blocking` | Can produce `BLOCK` (hardcoded secrets). |
| `duplication` | `advisory` | WARN/INFO only — **cannot block** on its own; only `duplication:confirmed-block` is block-eligible. See [duplication-benchmark.md](duplication-benchmark.md) for the rationale. |
| `resilience` | `advisory` | Opt-in; `NOT_RUN` unless requested (recorded as `checks_skipped: ["resilience"]`). |

`blocking` legs can produce `BLOCK`; `advisory` legs only produce `WARN`/`INFO`
(or `NOT_RUN` when skipped). A leg that did not run appears as
`{"leg": "resilience", "verdict": "NOT_RUN", "kind": "advisory"}`.

Example response:

```json
{
  "status": "WARN",
  "exit_code": 0,
  "leg_verdicts": [
    {"leg": "architecture", "verdict": "PASS", "kind": "blocking"},
    {"leg": "secret", "verdict": "PASS", "kind": "blocking"},
    {"leg": "duplication", "verdict": "WARN", "kind": "advisory"},
    {"leg": "resilience", "verdict": "NOT_RUN", "kind": "advisory"}
  ],
  "verdict_basis": "WARN (advisory: duplication)"
}
```

## Advisory semantics — important

These tools are **advisory, not a pre-write firewall**:

- The agent opts in by calling `blueprint_validate_staged`. There is no OS-level file-write interception.
- Blueprint reports what is wrong and how to fix it; the agent decides whether to repair and retry.
- This is a critical safety property of Blueprint's design: validation is offered, never imposed, at the agent layer.

Architecture boundaries for new files are enforced once the file is on disk — see [limitations.md: No pre-write virtual-AST seam](limitations.md#no-pre-write-virtual-ast--projected-graph-seam). Call `blueprint_validate_staged` after staging to catch them pre-commit.

## Running the server

Build the MCP server binary (see [installation](installation.md)):

```sh
go build -o blueprint-mcp ./cmd/blueprint-mcp
```

The server speaks MCP over **stdio**: it reads JSON-RPC requests from stdin and writes responses to stdout. It implements `initialize`, `notifications/initialized`, `tools/list`, `tools/call`, and `shutdown`; unknown methods return a JSON-RPC `-32601` error.

Run it directly:

```sh
./blueprint-mcp
```

The server does not need a listening port — the MCP client (agent host) launches it as a subprocess and communicates over stdio.

## Connecting an MCP client

Configure your MCP client to launch `blueprint-mcp` as a stdio server. Example for a JSON-based MCP client config:

```json
{
  "mcpServers": {
    "blueprint": {
      "command": "/path/to/blueprint-mcp",
      "args": []
    }
  }
}
```

After the client connects, it can enumerate tools with `tools/list` and invoke them with `tools/call`:

```json
{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}
```

```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"blueprint_validate_staged","arguments":{"repo":"/path/to/repo","source":"agent"}}}
```

kern must be installed and on `$PATH` (or `KERN_BINARY` set) for the tools to execute; otherwise the tool returns an error result.

## Repair loop

The intended agent workflow is a validate → repair → validate loop:

```text
validate → BLOCK → explain → repair → validate → PASS
```

1. The agent stages changes and calls `blueprint_validate_staged`.
2. If the result is BLOCK, the agent calls `blueprint_explain_finding` on each finding to understand the violation and the suggested fix.
3. The agent repairs the code (e.g. removes the forbidden dependency or moves the secret to an env var) and stages the fix.
4. The agent calls `blueprint_validate_staged` again until the result is PASS.

Because the block response is machine-readable, this loop can be driven automatically by the agent without human intervention.

## Related

- [Git hook guide](git-hook-guide.md) — the human-side enforcement layer (pre-commit)
- [CI guide](ci-guide.md) — the server-side enforcement layer (protected branch)