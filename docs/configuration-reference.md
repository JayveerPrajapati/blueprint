# Configuration Reference

Blueprint reads an optional configuration file at `.blueprint/config.yaml` in the repository root.

**The config file is optional.** If it does not exist, Blueprint uses conservative defaults (see below). If it exists but is invalid, Blueprint fails with exit code `3` rather than silently proceeding.

## Schema

| Field | Type | Default | Description |
|---|---|---|---|
| `version` | int | `1` | Config schema version. Must be `1`. |
| `mode` | string | `enforce` | Global mode: `enforce`, `warn`, or `off`. `warn` downgrades BLOCK to WARN but never to PASS. |
| `policies` | map[string]string | see defaults | Per-policy enforcement: `block`, `warn`, or `skip`. Keys: `architecture`, `secrets`, `duplication`, `tests`, `resilience`. |
| `thresholds` | map[string]float64 | — | Policy thresholds (e.g. duplication similarity cutoffs). Reserved for per-policy tuning. |
| `execution.timeout_seconds` | int | `120` | Per-validation timeout in seconds. Max `3600`. |
| `execution.max_output_bytes` | int | `200000` | Cap on captured command output (stdout+stderr) per validation. Max `10485760` (10 MiB). |
| `feedback.format` | string | `json` | Feedback output format. |
| `feedback.include_suggestions` | bool | `false` | Whether findings include suggested fixes in feedback. |
| `approval.enabled` | bool | `true` | Enable the two-person approval gate. `false` turns the gate off entirely. |
| `approval.require_for_sources` | []string | `["agent"]` | Change sources that require approval. |
| `approval.require_for_risk_levels` | []string | `["high"]` | Risk levels that require approval. |
| `approval.max_diff_lines` | int | `500` | Diff line threshold above which a change is classified as at least medium risk. |

### Enforcement values

| Value | Meaning |
|---|---|
| `block` | Findings of this category can block the change (exit code 1). |
| `warn` | Findings are reported as warnings; the change still passes. |
| `skip` | The policy is not run. |

### Modes

| Mode | Meaning |
|---|---|
| `enforce` | Policies enforce normally; `block` findings block. |
| `warn` | Global downgrade: BLOCK findings become WARN. Never downgrades to PASS. |
| `off` | Validation is effectively disabled. |

## Defaults (no config file)

With no `.blueprint/config.yaml`, Blueprint applies these conservative defaults:

```text
mode:        enforce
architecture: block
secrets:      block
duplication:  warn
tests:        block
resilience:   warn
approval:     block
timeout_seconds:     120
max_output_bytes:    200000
feedback.format:     json
```

A partial config file is overlaid on these defaults — you only need to specify what you want to change.

### approval

The approval gate enforces the **two-person rule** for high-risk agent changes:
a change that requires approval stays blocked until a human records an approved
request in `.blueprint/approvals/requests.jsonl`. Absent (or empty) `approval:`
falls back to conservative defaults: the gate is **enabled**, **agent** changes
at **high** risk require approval, and a diff larger than **500** lines is
classified as at least medium risk.

```yaml
approval:
  enabled: false            # set to true to enable the gate (default: enabled)
  require_for_sources:      # which change sources require approval (default: [agent])
    - agent
  require_for_risk_levels:  # which risk levels require approval (default: [high])
    - high
  max_diff_lines: 500       # diff line threshold above which a change is classified as at least medium risk
```

- `enabled` — defaults to `true`; set `false` to disable the gate entirely (the
  approval check is then not wired into the pipeline).
- `require_for_sources` — sources the gate applies to; default `["agent"]`.
  Humans are the approvers, so they never approve their own change by default.
- `require_for_risk_levels` — risk levels that require approval; default
  `["high"]`.
- `max_diff_lines` — added+removed line threshold above which a diff counts as
  large; `0` falls back to the default `500`.
- `sensitive_paths` — glob patterns (`path.Match` plus `**`) matched against
  change file paths that mark a change sensitive; empty falls back to the
  built-in list (`.kern/`, `*.pem`, `*.key`, `auth/`, `credentials*`,
  `secrets*`).

**Default enforcement: `block`** — when the gate is enabled and a change
requires approval, a missing (or pending/rejected) approval blocks the change
with an `approval:required` / `approval:rejected` finding.

## Example

```yaml
# .blueprint/config.yaml
version: 1
mode: enforce

policies:
  architecture: block
  secrets:      block
  duplication:  warn
  tests:        warn

execution:
  timeout_seconds: 300
  max_output_bytes: 1048576

feedback:
  format: json
  include_suggestions: true
```

## Validation rules

- `version` must be `1`; anything else is an error (exit code 3).
- `mode` must be one of `enforce | warn | off`.
- Policy keys must be one of `architecture | secrets | duplication | tests | resilience | approval`.
- Enforcement values must be one of `block | warn | skip`.
- `approval.require_for_sources` values must be known sources (`agent | ide | human | refactor | dep-bot | ci`).
- `approval.require_for_risk_levels` values must be `low | medium | high`.
- `approval.max_diff_lines` must be `>= 0` (defaults to `500`).
- `timeout_seconds` defaults to `120` if unset or `<= 0`; errors if `> 3600`.
- `max_output_bytes` defaults to `200000` if unset or `<= 0`; errors if `> 10485760`.

## Invalid config behavior

An invalid config file causes `blueprint check` and `blueprint ci` to fail with exit code `3` (invalid Blueprint configuration). When `--json` is used, the error is emitted as JSON with the same exit code.

## Related

- [Policy reference](policy-reference.md) — what the policies actually check and how findings are aggregated
- [Sandbox limits](installation.md) — timeouts and output caps also bound sandboxed command execution