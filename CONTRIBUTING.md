# Contributing to Sprout

Thanks for helping. A few things keep Sprout small and pleasant to work on.

## Setup

```bash
git clone https://github.com/Sprout-DevLabs/sprout && cd sprout
go test ./...
go run . --help
```

You need Go (see `go.mod`) and git. There are no other dependencies, and
we'd like to keep it that way.

## Ground rules

- **Standard library only.** Open an issue before adding a dependency.
- **One change per PR,** with a test that fails without it. Tests that need
  git create real throwaway repos (`setupGitRepo` in `tree_test.go`).
- **Every mode goes through `run()`.** CLI flags, `--json` and the MCP tools
  share one code path; don't fork logic for one of them.
- **Nothing hidden silently.** If a feature filters output, the summary must
  say so.
- **Untrusted input stays contained.** Anything reachable from MCP arguments
  must stay inside the root and must never reach git as an option.
- `gofmt` and `go vet` clean; CI runs tests with `-race` on Linux, macOS and
  Windows.

## Layout

| File | What |
|---|---|
| `main.go` | Flags, usage, mode dispatch |
| `tree.go` | Walk, `Node`, tree printer |
| `filter.go` | `.gitignore` via git, built-in ignores |
| `git.go` | `--git` status, `--diff` |
| `churn.go` | `--churn` |
| `ai.go` | `--ai` map and budget fitting |
| `output_json.go` | `--json` schema |
| `mcp.go` | `sprout mcp` server |
| `stats.go`, `project.go` | `--stats`, stack detection |

## Releasing (maintainers)

Tag and push; the Release workflow runs GoReleaser and updates the Homebrew
tap:

```bash
git tag v0.3.0 && git push origin v0.3.0
```
