# 🌱 Sprout

**Map your codebase, for you and your AI agent.**

[![CI](https://github.com/Sprout-DevLabs/sprout/actions/workflows/go.yml/badge.svg)](https://github.com/Sprout-DevLabs/sprout/actions/workflows/go.yml)
[![Release](https://img.shields.io/github/v/release/Sprout-DevLabs/sprout?include_prereleases)](https://github.com/Sprout-DevLabs/sprout/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Sprout is a fast, single-binary directory explorer built around how developers
actually read projects. It respects `.gitignore`, shows git changes and
hotspots in the tree, renders a PR as a tree, and produces a compact project
map for LLMs. It also runs as an MCP server, so coding agents can use it
directly.

```bash
sprout --ai | pbcopy                # project context for any chat, ~2k tokens
sprout --diff main...HEAD -L 2      # what this branch touched, as a tree
sprout --churn --since '90 days ago'  # where development is concentrated
```

📖 **Docs:** https://sprout-devlabs.github.io/sprout-web/

## Install

```bash
brew install sprout-devlabs/tap/sprout                 # macOS, Linux
go install github.com/Sprout-DevLabs/sprout@latest     # Go 1.22+
```

Or grab a binary for Linux, macOS or Windows (amd64/arm64) from
[Releases](https://github.com/Sprout-DevLabs/sprout/releases).

## A tree that knows what matters

```
$ sprout -L 1          # in a clone of charmbracelet/bubbletea
.
├── LICENSE
├── README.md
├── commands.go
├── examples/
├── go.mod
├── tea.go
…
└── xterm.go

3 directories, 46 files (6 hidden or ignored, --all to show)
```

Inside a git repo, **`.gitignore` decides** what's shown. Sprout asks git
itself, so nested ignores, negations and global excludes all work. Outside
git, common build and dependency directories (`node_modules`, `dist`, `.venv`,
`target`, …) are hidden instead. Filtering is never silent: the last line says
how much was left out.

| Flag | |
|---|---|
| `-L, --depth N` | Limit depth |
| `-a, --all` | Show hidden and ignored entries |
| `--hidden` | Show dotfiles |
| `--no-ignore` | Skip `.gitignore` and the built-in list |
| `--ignore LIST` | Extra names/globs, e.g. `'*.log,fixtures'` |

Flags go before or after the path.

## `--ai`: context for LLMs

A structure-first map of the project, sized to a token budget. It never
includes file contents.

```
$ sprout --ai --budget 500
# bubbletea
The fun, functional and stateful way to build terminal apps. A Go framework

stack: Go (go.mod)
languages: Go 66%, Markdown 26%, YAML 8%
size: 228 files, 74 directories
git: on main
most changed (90d, commits): tea.go (5), color.go (2), cursed_renderer.go (2), examples/go.mod (2), go.mod (2)
entry points: tutorials/basics/main.go, tutorials/commands/main.go
config: .github/workflows/build.yml, .github/workflows/lint.yml, .github/workflows/release.yml, .goreleaser.yml, …

## structure
./ LICENSE README.md Taskfile.yaml clipboard.go color.go commands.go … tea.go termcap.go +10 more
.github/ dependabot.yml
  ISSUE_TEMPLATE/ bug.yml bug_report.md config.yml feature_request.md
  workflows/ build.yml coverage.yml dependabot-sync.yml examples.yml lint.yml release.yml
examples/ (147 files)
testdata/
  TestClearMsg/ bg_fg_cur_color.golden clear_screen.golden read_set_clipboard.golden
  TestViewModel/ altscreen.golden altscreen_autoexit.golden bg_set_color.golden … +2 more
tutorials/ go.mod go.sum
  basics/ README.md main.go
  commands/ README.md main.go
```

The budget (default 2000 tokens) is spent breadth-first. Every directory
starts as a one-line summary and opens while the map still fits. Your own
code is laid out before `examples/`, `tests/` and `vendor/` get any of it.

## `--diff`: a PR as a tree

```
$ sprout --diff main...HEAD -L 1
. main...HEAD
├── .github/  (2 changed)  +13 -28
├── cursed_renderer.go  M  +67 -17
├── cursed_renderer_test.go  M  +123 -0
├── examples/  (3 changed)  +5 -5
├── tea.go  M  +45 -20
└── testdata/  (13 changed)  +13 -13

34 files changed, +469 -98
```

Takes any git revision: `main...HEAD` (since the branch point), `HEAD~3`,
`v1.0..v1.1`. Line counts roll up into directories, and `--depth` collapses
deep subtrees into their totals.

## `--git` and `--churn`

```
$ sprout --git                          $ sprout --churn -L 1
. on feature/login                      .
├── api/  (2 changed)                   ├── README.md  ▂ 23
│   ├── auth.go  M                      ├── cursed_renderer.go  ▇ 86
│   └── session.go  A                   ├── tea.go  █ 97
└── old_auth.go  D                      └── examples/  ▅ 61
```

`--git` marks `M` modified, `A` added, `D` deleted (shown where the file used
to be), `R` renamed, `?` untracked, `U` conflicted. `--churn` counts the
commits touching each path. `--since '90 days ago'` narrows the window.
Both work together and with `--depth`.

## For coding agents: `sprout mcp`

Sprout is also an [MCP](https://modelcontextprotocol.io) server. Agents get a
map of the repo in one call instead of burning context on `ls -R` and `find`.

```bash
claude mcp add sprout -- sprout mcp
```

```json
{ "mcpServers": { "sprout": { "command": "sprout", "args": ["mcp"] } } }
```

| Tool | Does |
|---|---|
| `project_map` | The `--ai` map. Agents are told to call it first in an unfamiliar repo |
| `tree` | Tree of a subdirectory, optionally with `git` status or `churn` (depth 3 by default) |
| `diff_tree` | `--diff` for a revision like `main...HEAD` |

Paths are confined to the directory the server was started in, symlinks
included, and git arguments can't carry options.

## Scripting: `--json` and `--stats`

`--json` emits the tree, detected stack and stats as one document with a
`schemaVersion`. Adding fields isn't a breaking change. Every other flag
applies, so `sprout --diff main...HEAD --json` feeds a PR bot directly.

```bash
sprout --json | jq '.summary.languages'
sprout --stats
```

## Design principles

- **Useful by default.** `sprout` with no flags should be the right answer most of the time.
- **Transparent.** Anything hidden automatically is counted and can be shown.
- **Humans and machines.** Color on terminals only (`NO_COLOR` respected), plain text in pipes, JSON when asked.
- **Small.** One ~1 MB static binary, standard library only, git is the only runtime dependency (and only for git features).

## Roadmap

- `--entry`: suggested reading order, ranked by how central each file is in the import graph
- Editor integrations built on `--json`
- Monorepo awareness: one map section per workspace package

Ideas and bugs: [open an issue](https://github.com/Sprout-DevLabs/sprout/issues).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). In short: `go test ./...` passes, one
feature per PR, no new dependencies without an issue first.

## License

[MIT](LICENSE)
