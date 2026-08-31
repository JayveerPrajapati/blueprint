# Blueprint CI — GitHub Action

Reusable composite action that runs the blueprint change-governance gate
(`blueprint ci`) against your repository and uploads the JSON result artifact
when the gate fails. The gate covers architecture boundaries, hardcoded
secrets, and structural duplication, with policy enforcement (mode, per-source
overrides, suppressions) applied exactly as the CLI would.

## Usage

```yaml
name: blueprint-gate
on:
  pull_request:
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0        # required: blueprint ci diffs base..head
      - uses: JayveerPrajapati/blueprint/.github/actions/blueprint-ci@v0.1.0
        with:
          repo: .              # target repository (relative to workspace)
```

That is the whole gate: five lines of YAML. On failure, `blueprint-ci-result`
(JSON) is uploaded as an artifact and the job exits with blueprint's exit-code
contract (0 pass, 1 findings remain, 2 tool error, 3 config).

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `repo` | `.` | Path to the target repository, relative to the workspace root. |
| `blueprint-ref` | `main` | Git ref of `JayveerPrajapati/blueprint` to build blueprint from. Pin a tag for reproducibility. |
| `kern-ref` | `main` | Git ref of `JayveerPrajapati/kern` to build kern from. Pin a tag for reproducibility. |
| `go-version` | `1.27` | Go toolchain used to build blueprint and kern. |
| `base` | `main` | Base revision passed to `blueprint ci`. |
| `head` | `HEAD` | Proposed revision passed to `blueprint ci`. |
| `extra-args` | _(empty)_ | Extra `blueprint ci` flags, e.g. `--resilience`. |
| `artifact-file` | `blueprint-result.json` | Where blueprint writes its JSON result. |
| `artifact-name` | `blueprint-ci-result` | Uploaded artifact name on failure. |

## Notes

- **Pin your refs.** `blueprint-ref`/`kern-ref` default to `main`, which moves
  under you. A tag pin is the responsible choice for a protected-branch gate.
- **The gate is only as good as its checkout.** Use `fetch-depth: 0` so
  `base..head` covers the full proposed diff.
- **Where it fits.** The pre-commit hook guards each commit locally; this
  action is the organizational backstop — it runs the same pipeline on the
  full diff and produces an artifact the reviewer can inspect.
- **Source build for now.** blueprint and kern are built from source at the
  given refs. Once kern v0.9.0 ships prebuilt release assets, the install
  steps switch to downloading them (see `.github/workflows/ci.yml` for the
  same NOTE).