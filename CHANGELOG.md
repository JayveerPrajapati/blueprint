# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-08-31

First release cut. This changelog consolidates everything merged since the
project's initial baseline commit; there are no prior releases to compare
against. Releases are cut by tagging (see `.github/workflows/release.yml`).

### Features

- Complete Blueprint phases 1-13: the validation engine, MCP server, git
  hooks, CI integration, and documentation.
- Graceful degradation, an intent gate, and tamper-evident receipts,
  formalized as the P0.4 governance contract.
- Post-review P0-P2 hardening: isolation, duplication detection, provenance
  tracking, honesty guarantees, and the change-governance gate.
- Change-governance strengthening from review findings P0-2 through P1-3,
  plus the P0-P3 gap fixes.
- G0 baseline enforcement with the exit-code 4 contract, a wired file
  watcher, and kern PATH fallback resolution.
- Machine-readable findings contract (E-3) with provenance fields on every
  finding, aligned with the Kern 2.0 Evidence model.
- Latency budget gate and CI artifact check breakdown.
- Declarative YAML resilience scenarios behind an opt-in check.
- Repair loop that validates agent-proposed fixes in an isolated worktree.
- Reusable `blueprint-ci` GitHub Action for repository change governance.

### Fixes

- Chunk guard files, batch git diffs, and cache security scans for a faster,
  more truthful summary.
- Worktree-safe CI, freshness-gated index, and atomic metrics writes.
- Preserve ERROR/SKIP statuses in policy results instead of silently
  reporting PASS.
- MCP `blueprint_validate_proposed` accepts not-yet-on-disk file paths: the
  pre-tool-use confinement gate now resolves the longest existing ancestor
  (symlink-safe) and allows the missing tail inside `files[].path`, while
  top-level path arguments stay strict.
- `blueprint fix` prints an explicit note when it exits 1 on WARN-only
  findings, documenting the repair-loop contract (ANY remaining finding
  means the loop must iterate).
- `blueprint verify-receipt` warns when the most recent CI run was
  BLOCK/ERROR: the newest receipt predates it and no receipt exists for the
  failed run.

### Docs

- Two-firewalls section explaining the division of labor between blueprint
  and kern.
- Full-surface audit cleanup: gate registry range G0-G29 everywhere,
  duplication two-pass confirmed-block policy (>0.90 threshold), exit-code 3
  tables, missing `check`/`ci` flags in the README, and the
  `BLUEPRINT_AGENT_ID` environment variable.

### CI & Tooling

- Pull the latest kern straight from its repository in CI instead of
  building it from scratch.

### Tests

- Sandbox cleanup test now polls for process-group reaping.

### Chores

- Release groundwork: license, Make targets, CI and GoReleaser config, a
  single version source, and doc-drift fixes.
- Initial baseline and safety harness (Phase 0).
