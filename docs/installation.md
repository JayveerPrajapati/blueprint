# Installation

## Prerequisites

- **Go 1.27+** for building from source.
- **kern binary** installed and available on `$PATH`. Blueprint invokes kern as a subprocess and needs these commands:
  - `kern guard check` — architecture boundary checks
  - `kern sec` — secret scanning
  - `kern index` — symbol index build
  - `kern index --status` — index freshness checks
  - `kern version` — contract version negotiation
  - `kern audit append` — tamper-evident audit chain linking
  - `kern fingerprint` — repository fingerprinting

If kern is not on `$PATH`, set the `KERN_BINARY` environment variable to its absolute path.

## Build from source

Build the Blueprint CLI:

```sh
go build -o blueprint ./cmd/blueprint
```

Build the MCP server binary:

```sh
go build -o blueprint-mcp ./cmd/blueprint-mcp
```

Optionally install both to a directory on your `$PATH`:

```sh
go build -o "$(go env GOPATH)/bin/blueprint" ./cmd/blueprint
go build -o "$(go env GOPATH)/bin/blueprint-mcp" ./cmd/blueprint-mcp
```

## Point Blueprint at kern

Blueprint resolves the kern binary in this order:

1. `KERN_BINARY` environment variable
2. `kern` on `$PATH`
3. `../kern/bin/kern` relative to the current working directory

If kern is installed somewhere unusual:

```sh
export KERN_BINARY=/path/to/kern
```

## Environment variables

| Variable | Description |
|---|---|
| `KERN_BINARY` | Absolute path to the kern binary when it is not on `$PATH`. |
| `BLUEPRINT_AGENT_ID` | Agent identity recorded on audit records and used for source-aware policy/approval gating (`source=agent`). Defaults to a stable host-derived id. |

## Keep blueprint and kern in sync

Blueprint shells out to the kern binary (resolved via `KERN_BINARY` → `$PATH` →
`../kern/bin/kern`). After upgrading kern, **rebuild blueprint from source** —
the two must speak the same contract. If they drift, `blueprint doctor` reports
an ERROR (schema_version mismatch). Contract-version skew in the authz probe
degrades to a WARN (`authz:verdict-error`): the architecture check proceeds,
and the authz verdict is recorded as unavailable — it does not fail closed:

```sh
blueprint doctor   # kern-contract check -> ERROR (schema_version mismatch) when out of sync
```

## Verify

```sh
blueprint version
```

You should see output like:

```text
blueprint <version>
```

The version is injected at build time via ldflags (`-X .../version.Version=...`); a plain `go build` prints `dev`. See the [Makefile](../Makefile) for the canonical build with version injection.

Then run a full check against a repository with staged changes:

```sh
cd /path/to/your/repo
git add .
blueprint check
```

Exit code `0` means PASS. See the [README](README.md) for the full exit-code table.

## First run
On a repository without a `.kern/` directory (no index and no boundaries declared), `blueprint check` emits a WARN (`architecture:not-enforced`) for non-empty changes — no guardrails declared means the architecture policy is not evaluated, and failure is never a silent pass. It SKIPs only when there are no staged files. To enable enforcement, declare boundaries in `.kern/boundaries.json` (see [policy-reference](policy-reference.md)); Blueprint builds the kern index on demand.

While running, Blueprint writes `.blueprint/` (metrics + scan cache) under the repository. This directory is gitignored.

## Next steps

- [Configuration reference](configuration-reference.md) — optional `.blueprint/config.yaml`
- [Git hook guide](git-hook-guide.md) — install the pre-commit hook with `blueprint install hook`