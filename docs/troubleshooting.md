# Troubleshooting

Common issues you may hit while running Blueprint, and how to resolve them.

---

## "kern binary not found"

Blueprint consumes kern's index and governance system, so a working `kern`
binary is required.

**Fix:** Either install kern and make sure it is on your `PATH`, or point
Blueprint at a specific binary:

```sh
export KERN_BINARY=/path/to/kern
```

---

## "boundaries.json rules not working"

Blueprint's `boundaries.json` schema uses a string action field:

```json
{
  "action": "forbid"
}
```

It does **not** use a boolean form. This shape is silently ignored — no error
is raised, the rule just never matches:

```json
{
  "forbid": true
}
```

**Fix:** Replace `{"forbid": true}` with `{"action": "forbid"}`. The boolean
form is a silent no-op.

---

## "secret scan returns nothing"

Two known causes, both related to how `kern sec` behaves:

1. **`kern sec` requires a directory argument.** Running `kern sec --json <file>`
   on a single file returns `null`. You must scan a directory.
2. **`kern sec` only scans source extensions** (`.go`, `.js`, `.ts`, `.py`,
   `.rb`, `.java`). It does **not** scan `.pem` files or other formats. Private
   keys used in fixtures must be embedded inside source files.

**Fix:** Point the scan at a directory, and keep secrets-to-be-detected inside
source files with a recognized extension.

---

## "pre-commit hook not blocking"

The pre-commit hook only runs if Blueprint is on your `PATH` at commit time,
and it can always be bypassed:

```sh
git commit --no-verify
```

**Fix:**

- Verify Blueprint is installed and reachable as `blueprint` from your shell.
- Do not rely on the hook alone. The intended enforcement chain is
  **hook → CI → protected branch**. `--no-verify` defeats the local hook, so
  CI and branch protection must exist to catch bypasses. No single layer is
  sufficient.

---

## "CI shows PASS but local shows BLOCK"

CI checks out the head ref, so the base/head comparison can differ from your
local state.

**Fix:** Verify the base and head refs Blueprint is comparing in CI. Make sure
the CI checkout actually contains the changes being validated (checkout the
correct head ref rather than a default/detached commit).

---

## "kern sandbox fails with file name too long"

This is a known upstream bug: `kern sandbox --json` has a recursion bug that
produces infinite directory nesting, which eventually fails with "file name
too long".

**Fix:** Nothing to fix on Blueprint's side. Blueprint does not use `kern
sandbox`; it uses git worktrees for isolation instead (an allowed
"sandbox/worktree mechanism" per the spec).

---

## "duplication check has too many false positives"

This is expected at the current stage. The duplication oracle is **warning
only** and has a documented baseline that has **not** been promoted to BLOCK:

| Metric    | Baseline |
|-----------|----------|
| Precision | 0.50     |
| Recall    | 0.75     |
| FPR       | 1.00     |

**Fix:** Treat duplication findings as advisory. The check must not be promoted
to BLOCK until it is benchmarked against these metrics.

---

## "watcher doesn't detect changes"

The continuous watcher is a post-write polling/event mechanism. If changes
aren't detected:

- Check the ignore paths — the change may be under an ignored path.
- The polling interval may be too long; shorten it if you need faster feedback.

Also remember: the watcher is advisory and post-write. It cannot intercept
writes before they happen, and file events are not transactional.

---

## "metrics file missing"

The metrics file is created on first run. If you need to re-initialize it:

```sh
blueprint metrics --reset
```

---

## Related

- [Security model](security-model.md) — enforcement chain, redaction, sandbox
  isolation properties.
- [Limitations](limitations.md) — full list of known limitations and anti-goals.