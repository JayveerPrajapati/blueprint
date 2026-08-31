# Git Hook Guide

Blueprint can install a git pre-commit hook that validates staged changes before every commit. This is the developer-local enforcement layer.

## Install

```sh
blueprint install hook
```

This writes a pre-commit hook to `.git/hooks/pre-commit` in the current repository. The hook is a thin adapter — it only executes:

```sh
blueprint check --staged --format=terminal
```

and propagates the exit code. No command logic is duplicated in the shell script.

The command prints:

```text
Installed pre-commit hook at .git/hooks/pre-commit
The hook runs `blueprint check --staged` on every commit.
To bypass: git commit --no-verify (documented, not a bug)
```

## What happens on commit

When you run `git commit`, the hook runs `blueprint check --staged`:

- **PASS (exit 0)** — the commit proceeds.
- **BLOCK (exit 1)** — the commit is aborted and findings are printed to the terminal.
- **ERROR (exit 2)** — the commit is aborted; e.g. kern binary missing, not a git repo, or staged changes could not be discovered.

`check --staged` is the default behavior of `blueprint check`; the `--staged` flag exists for hook clarity.

## Bypass is documented, not hidden

```sh
git commit --no-verify
```

This skips the pre-commit hook entirely. Blueprint treats bypass as a documented behavior, not a bug — which is why the enforcement chain continues past the hook (see below).

## Idempotent, refuses foreign hooks

- Re-running `blueprint install hook` overwrites Blueprint's own hook safely (it detects the Blueprint marker comment).
- If a **foreign** pre-commit hook already exists, install refuses and tells you to remove it first:

```text
blueprint: a pre-commit hook already exists at .git/hooks/pre-commit
  To overwrite, remove it first: rm .git/hooks/pre-commit
  Then re-run: blueprint install hook
```

- Worktrees are handled: the hook is installed in the common git directory, where git actually reads it.

## The enforcement chain

The hook is one layer of a three-layer chain:

```text
pre-commit hook (local) → CI (server) → protected branch (merge policy)
```

1. **Hook** — catches violations before they are committed locally.
2. **CI** — re-validates the base..head diff in a clean environment; catches anything the hook missed or that was committed with `--no-verify`.
3. **Protected branch** — the branch policy refuses merges whose CI check failed.

Because the hook is bypassable, CI (not the hook) is the authoritative gate. See [ci-guide.md](ci-guide.md) for the CI side.

## Related

- [Installation](installation.md) — build `blueprint` and install kern
- [Policy reference](policy-reference.md) — what the checks enforce