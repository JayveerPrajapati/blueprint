# Agent Repair Loop

Blueprint provides a structured repair-and-retry loop for AI coding agents. When an agent's proposed change is blocked, Blueprint returns a structured feedback contract the agent can act on deterministically — then re-validate after the fix.

## The loop

1. **Propose**: the agent stages or proposes a change.
2. **Validate**: call `blueprint_validate_staged` (for staged changes) or `blueprint_validate_proposed` (for pre-write content checks). The result is a `ValidationResult` with `Findings`.
3. **If BLOCK**: for each BLOCK finding, call `blueprint_repair_guidance` with the finding to get a structured repair contract (what failed, why, suggested fix, evidence, re-validation hint). The agent applies the `suggested_fix`.
4. **Re-validate**: call `blueprint_validate_staged`/`blueprint_validate_proposed` again. Repeat until the result is `PASS` with no BLOCK findings.

## The feedback contract

Every BLOCK finding carries (enforced by G7):
- `rule_id` — which rule fired
- `category` — architecture, secret, duplication, tests, resilience
- `file` / `line` — where the violation is
- `message` — what failed (specific, never vague)
- `explanation` — why it's a violation
- `suggested_fix` — actionable guidance
- `evidence` — supporting detail (truncated, redacted for secrets)

## Vague-response rejection

Blueprint rejects vague messages ("architecture error", "validation failed", "check failed", bare "error"). The `blueprint_repair_guidance` tool's `agent_contract.is_actionable` field is `false` for any finding with a vague message — agents should treat a non-actionable finding as a Blueprint bug, not a repair target.

## Determinism

The loop is deterministic: the same proposed change + policy + kern version always produces the same findings and the same repair guidance. CI verdicts are cached by content hash, so a re-validation of an identical change is instant.