# Blueprint — usage demo & examples

Verified live against a disposable clone of the **kern repo** (`/tmp/blueprint-demo/kern`,
real commits, real index). All output below is actual command output.

## 1. Install & configure

```bash
# Build (or `go install`)
go build -o blueprint ./cmd/blueprint

# Repo setup: two config files
mkdir -p .blueprint .kern

# Policy — .blueprint/config.yaml (optional; conservative defaults otherwise)
cat > .blueprint/config.yaml <<'EOF'
version: 1
mode: enforce
policies:
  architecture: block
  secrets:      block
  duplication:  warn
  tests:        block
  resilience:   warn
execution:
  timeout_seconds: 120
EOF

# Architecture boundaries — .kern/boundaries.json
# NOTE: dirMatch is exact/prefix/suffix — NO glob ("internal/*" does NOT match).
cat > .kern/boundaries.json <<'EOF'
{"rules": [
  {"from": "internal/demo", "to": "cmd/kern",     "action": "forbid"},
  {"from": "internal/demo", "to": "cmd/kern-mcp", "action": "forbid"}
]}
EOF

# kern index must be fresh (guard resolves import edges from it)
kern index .
```

## 2. Check staged changes

```bash
# Clean change → PASS (exit 0)
blueprint check --staged
#   blueprint: PASS (exit 0)
#   [PASS] architecture:guard (2019ms, 0 findings)
#   [PASS] secret:scan (2475ms, 0 findings)

# Violating change (internal/demo importing cmd/kern) → BLOCK (exit 1)
blueprint check --staged --format=json
#   status: BLOCK | exit: 1
#   finding: architecture:boundary-violation
#     "internal/demo/demo.go () calls into cmd/kern/: forbidden by
#      boundary rule internal/demo -> cmd/kern"
#     evidence: import-edge internal/demo/demo.go -> cmd/kern/
#     suggested_fix: Remove the dependency, or add an allow rule.

# Hardcoded secret → BLOCK with redaction
blueprint check --staged --format=json
#   status: BLOCK | exit: 1
#   finding: secret:hardcoded-secret | hardcoded secret: AWS
#     redacted: True        ← the secret value never propagates

# Fix the file, re-stage, re-check → PASS again.
```

**Important:** re-run `kern index .` after adding/renaming files — the guard
check resolves imports from the index. A file that doesn't compile cleanly may
not get its import edges indexed.

## 3. CI enforcement (protected branches)

```bash
# Any range: base..head, checks the diff, writes a JSON artifact
blueprint ci --repo . --base main --head HEAD \
  --artifact-file blueprint-result.json --no-human
# exit 0 = PASS, 1 = BLOCK, 2 = ERROR (tool/runtime/config/usage), 3 = config error, 4 = unsupported operation or environment

# Verified live on real kern commits:
blueprint ci --repo . --base main~2 --head main --artifact-file /tmp/ci.json --no-human
#   status: PASS | base: main~2 | head: main | findings: 0
```

Artifact schema (always written, even on error):
```json
{
  "repo": "/path", "base": "main", "head": "HEAD",
  "status": "PASS", "exit_code": 0, "files_changed": 3,
  "findings_count": 0, "duration_ms": 1200,
  "findings": [ { "rule_id": "...", "severity": "BLOCK",
                  "file": "...", "line": 42, "message": "...",
                  "redacted": true } ],
  "error": ""
}
```

## 4. Pre-commit hook

```bash
blueprint install hook
#   Installed pre-commit hook at <repo>/.git/hooks/pre-commit
#   The hook runs `blueprint check --staged` on every commit.
#   To bypass: git commit --no-verify (documented, not a bug)

# The hook:
#   exec blueprint check --staged --format=terminal
```

Blueprint **enforces itself**: it blocked its own commit once (7 secret
findings in test fixtures) until the fixtures were renamed `_test.go` and
`docs/ci-guide.md` was scoped via `.kernignore`.

## 5. MCP (agent pre-write control plane)

```bash
# Start the server (roots confinement mirrors kern's KERN_ROOTS)
BLUEPRINT_ROOTS=<repo> blueprint-mcp

# Tools: blueprint_validate_staged, blueprint_explain_finding
```

Agent workflow (JSON-RPC over stdio):

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/call",
 "params":{"name":"blueprint_validate_staged",
           "arguments":{"repo":"/path/to/repo","source":"agent"}}}
```

Results (verified live):
- Violating staged change → `status: BLOCK` + machine-readable finding
- Repo outside `BLUEPRINT_ROOTS` → `isError: true, "pre-tool-use denied:
  repository ... is outside the allowed workspace roots"`
- `blueprint_explain_finding` → human-readable explanation + suggested fix
  (the repair-loop tool)

## 6. Watcher & metrics

```bash
blueprint watch          # continuous advisory file-change validation
blueprint metrics        # local stats, no telemetry
#   Validations: 0 (pass=0 block=0 warn=0 error=0)
#   Latency p50: 0.00ms | p95: 0.00ms
#   Metrics file: <repo>/.blueprint/metrics.json
```

## Exit codes (spec Section 6)

| Code | Meaning |
|---|---|
| 0 | PASS |
| 1 | policy violation / BLOCK. `blueprint fix` exits 1 while ANY finding — WARN or BLOCK — remains; the repair loop must iterate |
| 2 | tool/runtime/configuration/usage ERROR |
| 3 | invalid Blueprint configuration |
| 4 | unsupported operation or environment |

## Using it for the kern repo — the 4 steps

1. `mkdir -p .blueprint .kern` and write `config.yaml` + `boundaries.json`
   (real rules you want; exact dirs, no globs).
2. `kern index .` — keep fresh (CI should run it before `blueprint ci`).
3. `blueprint ci --repo . --base main --head HEAD` in CI; gate on exit code.
4. `blueprint install hook` for devs; `blueprint-mcp` for agents.