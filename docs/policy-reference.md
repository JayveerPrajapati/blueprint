# Policy Reference

Blueprint validates changes against three built-in policies. Policies are enforced through kern, which Blueprint invokes as a subprocess.

## Architecture boundaries

**Rule ID:** `architecture:boundary-violation`

Boundary rules are declared in `.kern/boundaries.json` at the repository root. Blueprint enforces them by running `kern guard check --json` (scoped with `--file <staged>` for changed-file checks).

The schema uses **`{"action": "forbid"}`** for rules — not `{"forbid": true}` — wrapped in a top-level **`"rules"`** array (a `"boundaries"` wrapper is not honored):

```json
{
  "rules": [
    {
      "from": "web",
      "to": "db",
      "action": "forbid"
    }
  ]
}
```

A staged change that introduces or crosses a forbidden boundary produces a BLOCK finding.

### Scoping

- **Default (new-change principle):** only the staged files are checked. Pre-existing violations in unchanged files do not block a new, clean change.
- **Strict-baseline mode:** the whole tracked file tree (via `git ls-files`) is checked, so pre-existing violations are also reported.

The kern symbol index is refreshed (`kern index`) before the guard check so the index reflects current file content.

## Secret scanning

**Rule ID:** `secret:hardcoded-secret`

Secret detection is delegated to **gitleaks** (primary): Blueprint runs gitleaks
against the changed files and maps its structured findings (`secret:gitleaks`,
BLOCK, redacted). If the gitleaks binary is unavailable, Blueprint falls back to
the in-house `kern sec --json` (`secret:hardcoded-secret`) and emits a WARN
`secret:incumbent-unavailable` finding — never a silent pass. kern's secret
patterns cover hardcoded credentials such as API keys, AWS secrets, passwords,
private keys, and tokens in source-code and config extensions.

Key behavior:

- **Redaction:** secret snippets are never propagated into findings, output, or agent feedback. Findings report the rule, file, line, and category — never the secret value.
- **Allowlist:** findings in test fixtures are suppressed:
  - Files under a `testdata/` directory
  - Files with a `_test.go` suffix

This prevents false positives on intentional placeholder credentials in test fixtures.

A detected hardcoded secret is a BLOCK finding. The suggested fix is to move the credential to runtime secret storage (environment variable, vault, or secret manager).

## Duplication oracle

**Rule IDs:** `duplication:advisory` (in-house structural triage, WARN/INFO) and `duplication:confirmed-block` (two-pass escalation, BLOCK); primary detection is `duplication:jscpd`. Benchmark: `docs/duplication-benchmark.md`)

New functions are fingerprinted (signature shape, control flow, called symbols, size profile) and compared against existing functions. The structural similarity score is bucketed into the spec's tiers:

| Similarity | Tier | Behavior |
|---|---|---|
| `< 0.60` | ignore | No finding. |
| `0.60 – 0.85` | informational | Reported as INFO. |
| `0.85 – 0.95` | warning | Reported as WARN. |
| `> 0.95` | block-candidate | Inactive placeholder tier; never used for blocking. |

Detection uses a **two-pass model** (orchestrated by the jscpd adapter):

1. **Pass 1 — in-house structural triage** (`duplication:advisory`, always runs): every match ≥ 0.60 is an advisory candidate (WARN, or INFO for the informational bucket). Candidates with confidence **> 0.90** (`BlockCandidateThreshold`) are block-eligible.
2. **Pass 2 — jscpd confirmation** (when the jscpd binary resolves): jscpd independently scans the changed files (token-based clone detection). A block-eligible candidate whose file pair is **also** reported as a jscpd clone escalates to **`duplication:confirmed-block` (BLOCK)**. A candidate jscpd does not confirm stays advisory WARN.

**The in-house advisory check alone never blocks** — `duplication:advisory` findings are WARN/INFO. Duplication can only BLOCK through the two-pass escalation above, which requires both detectors to agree. When jscpd is unavailable the check is purely advisory and a `duplication:incumbent-unavailable` WARN is emitted. The default policy is `warn`; setting `duplication: block` in `.blueprint/config.yaml` is what turns a confirmed block into a hard failure. Thresholds and benchmark: `docs/duplication-benchmark.md`.

## Suppressing false positives

| Case | Mechanism |
|---|---|
| Secrets in test fixtures | Built-in allowlist suppresses `testdata/` and `_test.go` files. |
| Duplication | No override mechanism; `duplication:advisory` findings are WARN/INFO, and BLOCK comes only from `duplication:confirmed-block` (two-pass escalation). |
| Architecture | Fix the dependency or adjust the boundary rule in `.kern/boundaries.json`. |

False-positive overrides are tracked in local metrics (`blueprint metrics` reports the override count).

## How policy is enforced

Per-policy enforcement comes from the config (`block`, `warn`, or `skip`); see [configuration-reference](configuration-reference.md).

Findings are aggregated **monotonically**:

```text
ERROR > BLOCK > WARN > PASS
```

- Any BLOCK finding for a `block`-enforced policy makes the result BLOCK (exit code 1).
- `warn` mode downgrades BLOCK to WARN — but never to PASS.
- If any check fails at the tool level, the result is ERROR (exit code 2).
- WARN findings aggregate to WARN only when nothing worse is present.
- Otherwise the result is PASS (exit code 0).

## Configuring enforcement

```yaml
# .blueprint/config.yaml
version: 1
mode: enforce
policies:
  architecture: block   # boundary violations block
  secrets:      block   # hardcoded secrets block
  duplication:  warn    # advisory WARN; confirmed-block BLOCKs only if set to block
```

See [configuration-reference](configuration-reference.md) for all fields and defaults.