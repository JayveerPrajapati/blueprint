# Security Model

Blueprint is a local change governance engine built on top of kern. This
document describes how enforcement works, what the boundaries are, and what
Blueprint does and does not protect against.

---

## Enforcement chain

Blueprint provides multiple enforcement layers. Each layer is stronger than
the last, but also fires later in the change lifecycle. **No single layer is
sufficient** — the layers compose:

```
agent
  │
  ▼
MCP (advisory)          agent opts in; can be ignored
  │
  ▼
pre-commit hook         fires at commit time; bypassable with --no-verify
  │
  ▼
CI                      runs on push; sees the head ref
  │
  ▼
protected branch        enforces CI as a merge requirement
```

- **agent → MCP**: advisory only. The agent voluntarily calls Blueprint's MCP
  tools.
- **MCP → pre-commit hook**: the hook is stronger (it fires regardless of
  agent cooperation) but runs later — at commit time, after files are already
  on disk.
- **pre-commit hook → CI**: CI cannot be bypassed with `--no-verify`. Branch
  protection makes CI a hard merge gate.
- **CI → protected branch**: the final gate. A commit cannot land without
  passing the required checks.

Because each layer can be bypassed or ignored on its own (e.g.
`git commit --no-verify` defeats the local hook), enforcement depends on the
chain as a whole. **Hook + CI + protected branch is the enforcement chain.**

Pre-write MCP feedback (e.g. from `blueprint_validate_proposed`) is advisory
and does not change this chain — it fires earlier, not stronger, so
enforcement still rests on hook + CI + protected branch.

---

## Pre-write vs post-write semantics

Only the MCP layer could in principle intercept a change *before* it is
written, and even then:

- **MCP is advisory, not OS-level interception.** Blueprint's MCP tools are
  called voluntarily by agents. They cannot intercept writes the agent did not
  volunteer for. True pre-write interception would require kern upstream's
  `PreToolUse` hook, which is **not yet implemented**.
- **The watcher is explicitly post-write.** The continuous watcher is advisory
  feedback that reacts to file events after a write has already happened. File
  events are **not transactional** — there is no atomic "validate before
  write" path.
- **Git hooks fire at commit time.** By the time a pre-commit hook runs, the
  file is already on disk.

In short: Blueprint's enforcement is a **post-write advisory + commit-time +
CI** model. It is **not** a hard pre-write firewall. Do not rely on it to
prevent a write from happening; rely on it to surface problems before they
merge.

---

## Secret redaction guarantee

Secret values never reach agents, logs, or audit paths.

- Every secret finding carries `Redacted: true`.
- The secret snippet never propagates into Finding fields.
- The secret value, snippet, and any identifying details are stripped from:
  - Finding fields,
  - JSON output,
  - log and audit paths.

All feedback sent to agents is redacted.

---

## Sandbox isolation properties

Resilience scenarios run in a sandbox built from a git worktree. Its isolation
properties:

| Aspect        | Isolated? | Notes                                        |
|---------------|-----------|----------------------------------------------|
| Filesystem    | Yes       | Git worktree provides a separate working tree |
| Process       | Yes       | Runs in its own process group, with timeout and output cap |
| Network       | Opt-in    | Isolated when `--isolate-network` is set (Linux: new network namespace via CLONE_NEWNET; macOS: not supported, warns) |

Network isolation is opt-in via `--isolate-network` (or
`Config.NetworkIsolated` in the sandbox). On Linux the sandboxed process runs
in a new network namespace (CLONE_NEWNET) containing only loopback — no
external egress. On macOS and Windows isolation is not supported: Blueprint
prints a warning and continues without isolation.

---

## Local-only / offline-first

Blueprint is local and offline-first.

- **No cloud telemetry.** All metrics, logs, and artifacts are stored locally.
- **No implicit network access.** Nothing in Blueprint assumes or requires the
  network.
- **Any external dependency is explicit and documented.** No data leaves the
  machine unless you explicitly configure an external CI to consume results.

---

## What an agent can and cannot do via MCP

**Can:**
- Voluntarily ask Blueprint to validate a change against policies
  (architecture boundaries, secrets, duplication).
- Receive advisory findings, including redacted secret findings.

**Cannot:**
- Be forcibly intercepted at write time. Blueprint has no OS-level interception;
  it only acts when the agent opts in.
- Receive unredacted secret material. All secret feedback is redacted
  (`Redacted: true`), and the secret snippet never enters Finding fields.

The MCP layer is exactly as strong as the agent's willingness to call it.

---

## Audit chain threat model

The P1.4 audit chain (`.blueprint/audit/audit.jsonl`) and the CI receipts
(`.blueprint/receipts/<id>.json`) are **tamper-evident, not tamper-proof**.
This section states honestly what they do and do not protect against.

**What the chain and receipt detect:**
- **Accidental corruption.** Any record whose fields are edited without
  recomputing its hash breaks that record's self-hash and every subsequent
  link. `blueprint verify-receipt` and `blueprint doctor` walk the chain and
  name the first broken record.
- **Partial tampering.** An attacker who edits a record but recomputes only
  that record's hash still breaks the `previous_hash` link of the next record
  (each hash covers the previous hash), so the chain stays unforgeable without
  recomputing every record after the edit.
- **Head-prepend / mid-chain anchors.** Only the first record (and the legacy
  pre-P1.4 prefix) may carry an empty `previous_hash`; an empty anchor
  appearing after the chain has started fails verification.
- **Concurrent-write forks.** Cross-process `flock` serializes appends, so two
  simultaneous `blueprint` processes cannot each read genesis and append a
  second chain head (which would fork the chain into two valid-looking arms).

**What the receipt signature is — and is not:**
- The receipt signature is a **keyless SHA-256 integrity seal**: it detects
  modification of the receipt since it was generated, but it is **not**
  cryptographic authenticity. Anyone who can write to `.blueprint/` can
  regenerate a receipt (and recompute its signature) from scratch.

**What the chain does NOT protect against:**
- **A motivated actor with filesystem write access to `.blueprint/`.** The
  chain, the receipt, and their hashes all live under the same directory. An
  attacker who can write there can rewrite the entire chain, recompute every
  hash in order, and forge a matching receipt. The local chain detects
  accidental corruption and casual tampering; it does not stop a determined
  local attacker.
- **Deletion.** Removing the whole `.blueprint/` directory destroys the
  evidence (receipts then report "not found", which fails the merge gate
  closed, but the historical record is gone).

**The kern cross-link is the independent anchor:**
- Each audit write best-effort appends a mapped entry to kern's tamper-evident
  chain (`kern audit append`). The receipt cites kern's returned chain hash,
  and `blueprint verify-receipt` checks it against kern's chain. Because kern's
  chain lives outside `.blueprint/` (in kern's own store), an attacker with
  write access to `.blueprint/` alone cannot rewrite it — the two chains must
  agree.
- This anchor is **best-effort**: if kern is not installed, the cross-link is
  skipped and the receipt verifies on the local chain only (with a warning).
  It is a hardening layer, not a hard requirement.

**Recommendation for branch-protection-grade evidence:**
- Store `.blueprint/` on a separate **append-only** store (read-only bind
  mount, WORM volume, or a CI-managed workspace that is never writable by
  contributors), so the local chain itself becomes append-only.
- Or use kern's evidence export to produce **signed bundles** (a signature
  rooted in a key held outside the repo) for the receipts that must survive
  an adversarial `.blueprint/`.
- The protected-branch CI gate remains the organizational boundary: `git
  commit --no-verify` and local forgery both stop at a branch policy that
  requires a passing `blueprint verify-receipt` run by CI on the server.

---

## Threat model

**What Blueprint protects against:**
- Code changes that violate declared architecture boundaries.
- Secrets accidentally committed to source-controlled files, when detected by
  `kern sec` scans of supported source extensions.
- Duplicated code, at **warning** level only (see the baseline metrics in
  [Limitations](limitations.md)) — not a hard BLOCK.
- Changes that would fail governance checks slipping through unnoticed, when
  the full enforcement chain (hook + CI + protected branch) is configured.

**What Blueprint does NOT protect against:**
- Writes the agent never volunteered to check (MCP is advisory).
- `git commit --no-verify` bypassing the local hook.
- Secrets in file types `kern sec` does not scan (`.pem`, etc.).
- Network egress from the sandbox when `--isolate-network` is NOT set (isolation is opt-in).
- Anything that requires a second code index or second architecture parser —
  Blueprint intentionally builds neither and consumes kern's instead.

## Related

- [Troubleshooting](troubleshooting.md) — common issues and fixes.
- [Limitations](limitations.md) — full list of known limitations and anti-goals.