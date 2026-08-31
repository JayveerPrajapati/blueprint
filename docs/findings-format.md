# Findings Format — the shared contract

Blueprint's `Finding` is the shared findings format across blueprint and kern.
This document is the contract; the machine-readable version lives at
`schema/findings-schema.json` (JSON Schema draft-07).

## Shape

A finding is a JSON object. Four fields are always present; everything else is
optional (`omitempty`) and carries `additionalProperties: true` at the top
level — see **Additive evolution** below.

| Field | Type | Semantics |
| --- | --- | --- |
| `rule_id` | string | Stable rule identifier, e.g. `duplication:advisory`, `performance:latency-budget`. |
| `severity` | enum | `block` > `error` > `warn` > `info`. Only deterministic checks may produce `block`. |
| `category` | enum | `architecture`, `secret`, `duplication`, `tests`, `build`, `resilience`, `policy`, `performance`. |
| `file` / `line` / `column` | string / int | Location (repo-relative file). Absent for repo-scoped findings. |
| `message` | string | One-line, human-readable, always safe to display. |
| `explanation` | string | Deeper context; safe to display. |
| `suggested_fix` | string | Actionable guidance; in v1 always prose (no rule has a machine-applicable fix). |
| `evidence[]` | array | `{kind, description, location}` — the deterministic evidence behind the finding (e.g. `import-edge`, `pattern-match`, `structural-fingerprint`, `latency`). |
| `redacted` | bool | True when the original content contained secret material. |
| `suppressed` / `suppression_reason` / `owner` | bool / string / string | Suppression maturity (P1-2): reviewed, expiring suppressions; owner routing from `owners.yaml`. |
| `rule_version` | string | Version of the rule family that produced the finding (`"1"` in v1). |
| `kern_version` | string | Version of the kern binary behind the check (`"dev"` for source builds). Best-effort; empty on probe failure. |
| `index_freshness` | enum | `fresh` / `stale` — whether the index-backed check ran against a freshly rebuilt index. Only set by index-backed findings. |
| `confidence` | number 0..1 | Deterministic checks 1.0; secrets 0.95; duplication = the structural similarity score; resilience 0.7. |
| `scope` | enum | `file` (located finding) or `repo` (repo-scoped, e.g. resilience, latency). |

## Invariants

1. **Redaction.** A finding with `redacted: true` never contains the secret
   material anywhere in its text fields — `message`, `explanation`,
   `suggested_fix`, or evidence descriptions. The invariant is enforced at
   the check boundary, before anything is rendered or persisted.
2. **Never-blocking categories.** Duplication, resilience, and performance
   findings are WARN at most, regardless of policy configuration. The policy
   engine never sees the performance finding at all (constructed post-policy).
3. **Additive evolution.** Producers only add fields; they never rename or
   remove existing ones. Consumers must ignore unknown fields. The JSON
   Schema deliberately keeps `additionalProperties: true`.
4. **Determinism.** Evidence is derived from deterministic computation
   (AST, index, structural fingerprints), never from an LLM.

## Consumption extension points

- **kern web console / explain**: can render blueprint findings by accepting
  the shape above and honoring the redaction invariant (a `redacted: true`
  finding must be displayed without its original text fields if they were
  stripped upstream).
- **Repair loop (`blueprint fix`)**: suggested_fix prose can be enriched by
  calling kern's context tools (`kern context <symbol>`, `kern why`) — the
  finding's `evidence[].location` and `file`/`line` identify the symbols to
  fetch context for.

## Validation

`schema/findings-schema.json` is validated by
`internal/blueprint/domain/findings_schema_test.go` (schema parses; a
representative finding conforms). External consumers may validate any
findings payload against the schema.