# Limitations

Honest, current limitations of Blueprint. This includes explicit anti-goals
from the spec (Section 23, "What Must NOT Be Built"), known upstream kern
bugs, and design constraints. These are not sugarcoated — read them before
relying on Blueprint for enforcement.

---

## Explicit anti-goals (spec Section 23)

### No second code index

Blueprint consumes kern's index. It does **not** build its own. If you need
indexing behavior beyond what kern provides, Blueprint will not provide it.

### No second architecture parser

Blueprint extends kern's governance system. It does **not** implement a second
architecture parser.

### No pre-write virtual-AST / projected-graph seam

Blueprint does **not** offer a "validate my plan before I write" oracle that
runs architecture-boundary checks against a *projected* symbol graph (current
index + agent-declared virtual symbols). This is a deliberate choice, not an
oversight:

- **No second parser.** Accepting the agent's *claimed* symbols/imports on
  trust would make the agent an unverified parser — gameable and drift-prone,
  violating the "no second architecture parser" anti-goal above. The only
  sound alternative is to parse the proposed content with kern itself, which
  requires a full projected-tree reindex on every pre-write call (architecture
  guard needs cross-file edge resolution, so the temp-mirror trick the
  duplication check uses — mirroring only the proposed files — does not work
  here). That is an L-sized latency cost for feedback that arrives only one
  write+stage cycle earlier than `blueprint_validate_staged`, which already
  rebuilds the kern index when stale and catches new-file boundary violations
  after write, before commit.

- **Pre-write is still advisory.** A pre-write MCP tool is opt-in; `git commit
  --no-verify` bypasses the hook regardless. Pre-write interception is
  *earlier advisory feedback*, not *stronger enforcement*. The enforcement
  chain (MCP → hook → CI → protected branch) is unchanged by moving one
  advisory signal earlier.

**What Blueprint does provide pre-write:** `blueprint_validate_proposed`
accepts `[{path, content, op}]` and validates **secrets** and **duplication**
against proposed content before it is written. Architecture boundaries for
*new* files are enforced once the file is on disk — call
`blueprint_validate_staged` after writing/staging to catch them pre-commit.
This is the intended workflow; do not expect `blueprint_validate_proposed` to
block architecture violations on not-yet-written files.

**Why not zero-config modularity-ΔQ gating:** kern's community detection is
deterministic label propagation, not Louvain, and computes no modularity Q.
Modularity ΔQ on single changes is a coarse, high-variance signal; building a
Louvain + projected-graph variant for a WARN-only check is disproportionate.
Blueprint's architecture oracle remains the explicit, auditable
`.kern/boundaries.json` rules evaluated by `kern guard`.

### Watcher is not a pre-write firewall

The continuous watcher is **post-write advisory feedback**. It cannot intercept
writes before they happen. File events are **not transactional** — there is no
atomic validate-before-write path.

### Local hooks are bypassable

`git commit --no-verify` bypasses the pre-commit hook. There is no way for a
local hook to prevent this. The enforcement chain is **hook + CI + protected
branch** — no single layer is sufficient on its own.

### Duplication is WARN only

The duplication oracle starts as **warning** and must **not** be promoted to
BLOCK until it is benchmarked. Current documented baseline:

| Metric    | Baseline |
|-----------|----------|
| Precision | 0.50     |
| Recall    | 1.00     |
| FPR       | 0.75     |

The false-positive rate is high by design at this stage. Treat duplication
findings as advisory.

### No generic cross-language chaos injection

Resilience scenarios use explicit adapters per ecosystem. Currently **only Go
is supported**, with these scenarios:

- `HTTPTimeout`
- `HTTP500`
- `MalformedJSON`

Other ecosystems have no resilience scenarios until explicit adapters are
written.

### Secrets are never sent to agents

All feedback is redacted. The `Redacted: true` flag is set on secret findings.
The secret snippet never propagates into Finding fields. This applies to
Finding fields, JSON output, and log/audit paths.

### Offline-first / local-only

Blueprint is local and offline-first:

- No cloud telemetry.
- Metrics are stored locally.
- No network access is implicit.
- Any external dependency is explicit and documented.

---

## Known kern bugs and limitations that affect Blueprint

### kern sandbox is broken

`kern sandbox --json` has a recursion bug (infinite directory nesting) that
fails with "file name too long". Blueprint does not use `kern sandbox`; it uses
**git worktrees** instead. The spec allows "sandbox/worktree mechanisms", so
this is a workaround, not a feature loss.

### kern sec only scans source extensions

`kern sec` scans only `.go`, `.js`, `.ts`, `.py`, `.rb`, `.java`. It does
**not** scan `.pem` files. Private-key fixtures must embed PEM content inside
source files for the scanner to see them.

### kern sec natively skips `_test.go` files

`kern sec` skips `_test.go` files by default. Blueprint's allowlist
additionally suppresses `testdata/` directories.

### kern sec requires a directory argument

`kern sec --json <file>` returns `null` for an individual file. Scans must
target directories.

### kern sec scans whole directories; Blueprint caches per-file

`kern sec` scans a whole directory in one invocation. Blueprint runs it once
per validation and caches **per-file findings keyed by file size + mtime**, so
repeat validations against unchanged files skip the kern scan entirely.

---

## Design and integration limitations

### MCP is advisory, not OS-level interception

Blueprint's MCP tools are called **voluntarily** by agents. They cannot
intercept writes the agent did not volunteer for. True pre-write interception
would require kern upstream's `PreToolUse` hook, which is **not yet
implemented**. The MCP layer is exactly as strong as the agent's willingness
to call it.

### Index is non-deterministic at byte level

kern's `index.json` contains `updated_at`, `root` (an absolute path), and
`max_mtime`, which break byte-level comparison in CI. The content is
deterministic. Blueprint's CI uses **normalized comparison** to account for
this.

### Index freshness is a heuristic

The index freshness heuristic (file mtime vs HEAD commit time) can miss
changes that preserve mtimes — for example `git apply` of a patch. When the
index may be stale, refresh it manually with `kern index`.

### Sandbox network isolation is opt-in and Linux-only

The sandbox (git worktree) isolates the filesystem and the process (process
group + timeout + output cap). Network isolation is **opt-in** via
`--isolate-network` and is only truly enforced on **Linux**, where the
sandboxed process runs in a new network namespace (CLONE_NEWNET) with only
loopback. On macOS and Windows the flag warns and continues **without**
isolation, so resilience scenarios there can still reach the host network.

### Sandbox build/test is Go-only

The sandbox build/test check runs hardcoded `go build ./...` and `go test
./...` inside the worktree. Non-Go repositories are not validated by this
check yet.

### Go 1.27 build constraint

The repo builds with Go 1.27, which rejects spreading variadic parameters
after fixed arguments. Code must respect this constraint to compile.

### boundaries.json uses string actions

`boundaries.json` uses `{"action": "forbid"}` (string field), **not**
`{"forbid": true}` (boolean). The boolean form is silently ignored — no error,
the rule just never matches.

### summary counts findings

`summary.blocks` and `summary.errors` count **findings** (the number of BLOCK
/ ERROR findings reported), not a flag. Compare them as counts, not booleans.

---

## Summary table

| Area | Limitation |
|------|-----------|
| Code index | No second index; consumes kern's |
| Architecture parsing | No second parser; extends kern's |
| Watcher | Post-write advisory only; events not transactional |
| Local hooks | Bypassable via `--no-verify`; rely on hook + CI + protected branch |
| Duplication | WARN only; FPR 0.75 baseline, not promoted to BLOCK |
| Chaos injection | Go only (`HTTPTimeout`, `HTTP500`, `MalformedJSON`) |
| Secrets | Always redacted (`Redacted: true`); snippet never in Findings |
| Telemetry | None; local-only, offline-first |
| kern sandbox | Broken upstream; Blueprint uses git worktrees |
| kern sec extensions | Source extensions only; no `.pem` |
| kern sec test files | Skips `_test.go`; Blueprint also suppresses `testdata/` |
| kern sec argument | Requires a directory; file argument returns `null` |
| MCP interception | Advisory; no `PreToolUse` hook yet |
| Index determinism | Byte-level non-deterministic; CI uses normalized comparison |
| Index freshness | Heuristic (mtime vs HEAD time); can miss mtime-preserving changes, refresh with `kern index` |
| sec scan scope | Whole-directory scans; Blueprint caches per-file findings (size+mtime keyed) |
| Summary fields | `blocks`/`errors` count findings, not a flag |
| Sandbox network | Opt-in via --isolate-network; Linux-only (CLONE_NEWNET), macOS/Windows warn |
| Go version | Requires Go 1.27 build behavior |

## Related

- [Security model](security-model.md) — enforcement chain, redaction, sandbox
  isolation properties.
- [Troubleshooting](troubleshooting.md) — common issues and fixes.