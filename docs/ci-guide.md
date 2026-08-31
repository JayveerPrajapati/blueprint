# CI Guide

`blueprint ci` runs Blueprint validation against a base..head revision diff. It is designed for CI systems and protected-branch enforcement, and is the authoritative gate in Blueprint's enforcement chain (see [git-hook-guide](git-hook-guide.md)).

## Command

```sh
blueprint ci [--repo DIR] [--base main] [--head HEAD] \
             [--artifact-file blueprint-result.json] [--json] [--no-human]
```

| Flag | Default | Description |
|---|---|---|
| `--repo DIR` | current directory | Repository root. |
| `--base REF` | `main` | Base revision (branch, tag, or SHA). |
| `--head REF` | `HEAD` | Proposed revision (branch, tag, or SHA). |
| `--artifact-file FILE` | `blueprint-result.json` | Path to write the JSON artifact. |
| `--json` | `false` | Also emit the JSON artifact to stdout. |
| `--no-human` | `false` | Suppress the human-readable summary on stderr. |

## How it works

1. **Verify revisions** — both `--base` and `--head` must resolve; otherwise ERROR.
2. **Diff** — changed files between base and head are discovered.
3. **Validation root** — when `--head` is not `HEAD`, validation runs inside a throwaway **detached git worktree** at that ref; the source repo's working tree is **never checked out or mutated**, and the worktree is removed afterwards. When `--head` is `HEAD`, validation runs in the current checkout.
4. **Load config** — `.blueprint/config.yaml` is read from the validation root (or defaults apply).
5. **Build the kern index** — the symbol index is built **once** inside the validation worktree (CI must not depend on developer-local daemon state); when validating the current checkout (`--head HEAD`), the index is only refreshed if stale.
6. **Validate** — the same policy engine as local `check` runs against the base..head changes (source `ci`).
7. **Emit** — the JSON artifact is written to `--artifact-file`, and a human-readable summary goes to stderr (unless `--no-human`).

> **Safety note:** the source repository is never checked out — non-`HEAD` validation runs in a throwaway detached worktree that is removed when the run finishes.

## JSON artifact

`blueprint-result.json` is written on every run (even on setup errors). Schema:

```json
{
  "repo": "/path/to/repo",
  "base": "main",
  "head": "HEAD",
  "status": "PASS",
  "exit_code": 0,
  "files_changed": 3,
  "findings_count": 0,
  "duration_ms": 1200,
  "start_at": "2026-08-25T12:00:00Z",
  "findings": [
    {
      "rule_id": "secret:hardcoded-secret",
      "severity": "BLOCK",
      "category": "secret",
      "file": "config.go",
      "line": 42,
      "message": "hardcoded secret: AWS",
      "explanation": "Secret scanner detected a potential AWS secret access key. The secret value has been redacted and must not be committed.",
      "suggested_fix": "Move the credential to runtime secret storage (environment variable, vault, or secret manager).",
      "redacted": true
    }
  ],
  "error": ""
}
```

Notes:

- `findings` is omitted when empty; `error` is omitted when empty.
- Secret values are **redacted** — they never appear in the artifact.
- The artifact is always written to `--artifact-file`, regardless of `--json`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | PASS |
| 1 | BLOCK — policy violation in the diff. `blueprint fix` exits 1 while ANY finding — WARN or BLOCK — remains; the repair loop must iterate |
| 2 | ERROR — tool/runtime/config/usage failure (bad ref, kern missing, diff failure, checkout failure, index build failure, invalid flags) |
| 3 | Invalid Blueprint configuration |
| 4 | Unsupported operation or environment |

Use the exit code as the CI gate. The status in the JSON artifact mirrors it.

## Human-readable summary

Without `--no-human`, a summary is printed to stderr (status, finding count, files changed, duration, and per-finding details). Keep it separate from the JSON artifact: stdout stays clean for `--json` output.

## Example: GitHub Actions

```yaml
name: blueprint

on:
  pull_request:
    branches: [main]

jobs:
  blueprint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0        # required for base..head diffing

      - uses: actions/setup-go@v5
        with:
          go-version: '1.27'

      - name: Install kern
        run: |
          # Install the kern binary and ensure it is on $PATH
          # (or set KERN_BINARY to its absolute path)
          echo "$HOME/bin" >> "$GITHUB_PATH"

      - name: Build blueprint
        run: go build -o "$HOME/bin/blueprint" ./cmd/blueprint

      - name: Validate diff
        run: blueprint ci --repo . --base main --head HEAD --artifact-file blueprint-result.json
        # exit 1 (BLOCK) and exit 2 (ERROR) fail the job automatically

      - name: Upload artifact
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: blueprint-result
          path: blueprint-result.json
```

## Example: generic CI (script)

```sh
#!/usr/bin/env sh
set -e

# Fetch both refs so blueprint can diff base..head
git fetch origin main
git fetch origin "$HEAD_SHA"

blueprint ci --repo . --base origin/main --head "$HEAD_SHA" \
  --artifact-file blueprint-result.json

# The exit code is already the gate: 0 = PASS, 1 = BLOCK, 2 = ERROR (tool/runtime/config/usage), 3 = config error, 4 = unsupported operation or environment
```

Then consume `blueprint-result.json` for reporting or to attach findings to the check.

## Enforcement chain

The hook catches violations locally, CI catches anything committed with `--no-verify`, and the protected-branch policy refuses merges whose CI check failed. Configure the CI job as a required check on the protected branch to complete the chain.

## Related

- [Installation](installation.md) — prerequisites and building `blueprint`
- [Policy reference](policy-reference.md) — what the checks enforce and how findings aggregate