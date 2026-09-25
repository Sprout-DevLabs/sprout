# Sprout (under development)

- Understand your project at a glance.

Sprout is a fast, developer-first directory explorer written in Go.

It takes the familiar idea behind `tree` and focuses it around a modern developer workflow: simple commands, project-aware filtering, useful statistics, Git awareness, and machine-readable output.

Instead of asking, "How do I list every file?", Sprout asks, "What parts of this project actually matter?"

## Features

**Currently planned**

- Clean directory tree visualization
- Hidden file support
- Configurable directory depth
- Simple ignore patterns
- Project-aware filtering
- Project statistics
- Git status integration
- JSON / Markdown output
- AI-friendly project mapping
- Fast native executable
- Cross-platform support

Note: Sprout is currently under development. Some features listed above are planned and may not yet be available.

## Quick Start

### Install with Go

```
go install github.com/Sprout-DevLabs/sprout@latest
```

Then:

```
sprout
```

### Build from source

```
git clone https://github.com/Sprout-DevLabs/sprout.git
cd sprout
go build -o sprout .
```

Run it:

```
./sprout
```

## Basic Usage

Run Sprout in the current directory:

```
sprout
```

Explore a specific directory:

```
sprout src
```

Limit the directory depth:

```
sprout --depth 2
```

Show hidden files:

```
sprout --hidden
```

Ignore specific directories:

```
sprout --ignore node_modules,.git,venv
```

Combine options:

```
sprout --hidden --depth 3 --ignore node_modules,.git
```

## Project Mode

Modern repositories contain a lot of files that aren't particularly useful when you're trying to understand the project.

For example:

```
node_modules/
.git/
dist/
.cache/
__pycache__/
.venv/
```

Instead of manually specifying every directory, Sprout will provide a project-aware mode:

```
sprout --project
```

The goal is to automatically identify common development artifacts while keeping important project files visible.

For example:

```
my-project/
├── src/
│   ├── components/
│   │   ├── Navbar.jsx
│   │   └── Footer.jsx
│   ├── App.jsx
│   └── main.jsx
├── tests/
├── public/
├── package.json
└── README.md
```

Rather than:

```
my-project/
├── node_modules/
│   ├── ...
│   └── ...
├── .git/
│   ├── ...
│   └── ...
├── src/
├── dist/
├── .cache/
└── package.json
```

## Smart Mode

Sprout will eventually provide a smarter project inspection mode:

```
sprout --smart
```

This mode can detect common project characteristics and make sensible decisions about what to display.

For example:

```
Detected project:
  Language:       Python
  Framework:      FastAPI
  Package manager: pip
  Version control: Git
Ignored:
  .venv/          Python virtual environment
  __pycache__/    Python bytecode cache
  .git/           Git metadata
```

Sprout should never hide files without making it possible to understand why.

Use:

```
sprout --smart --explain
```

to see the reasoning behind automatic filtering.

## Project Statistics

Sprout will provide an optional project overview:

```
sprout --stats
```

Example:

```
my-project/
Files:          87
Directories:    19
Total size:     4.8 MB
Languages:
  Python        48 files
  JavaScript    21 files
  CSS            9 files
  Markdown      6 files
  JSON           3 files
```

This turns Sprout from a simple tree renderer into a lightweight project inspection tool.

## Git Integration

Future versions will provide Git-aware output:

```
sprout --git
```

Example:

```
project/
├── src/
│   ├── api.py          M
│   ├── models.py
│   └── utils.py
├── tests/
│   └── test_api.py     ?
└── README.md
```

Possible status indicators:

```
M   Modified
A   Added
D   Deleted
?   Untracked
```

The goal is to see both project structure and current changes in one place.

## Machine-Readable Output

Sprout isn't intended to be useful only to humans.

Future versions will support structured output:

```
sprout --json
```

Example:

```json
{
  "name": "my-project",
  "type": "directory",
  "children": [
    {
      "name": "src",
      "type": "directory"
    },
    {
      "name": "main.py",
      "type": "file"
    }
  ]
}
```

Additional formats may include:

```
sprout --markdown
sprout --yaml
```

This makes Sprout useful for:

- Shell scripts
- CI/CD
- Documentation generators
- Developer tooling
- IDE integrations
- AI-assisted development

## AI Project Mapping

One of Sprout's longer-term goals is to generate compact project context for AI tools.

For example:

```
sprout --ai
```

could produce:

```
PROJECT: flow-study
LANGUAGES:
  Python 62%
  JavaScript 31%
  CSS 7%
STRUCTURE:
src/
├── api/
│   ├── routes.py
│   └── models.py
├── components/
│   ├── Dashboard.jsx
│   └── Login.jsx
└── main.py
tests/
├── test_api.py
└── test_auth.py
CONFIG:
  package.json
  requirements.txt
  .env.example
```

The purpose isn't to send an entire repository into an AI context window.

Instead, Sprout aims to provide a compact representation of a project's structure and important metadata.

This feature is part of the long-term roadmap and is not currently implemented.

## Coming From tree?

If you're already familiar with `tree`, the basic concepts should feel familiar.

| tree | Sprout |
|---|---|
| `tree` | `sprout` |
| `tree -a` | `sprout --hidden` |
| `tree -L 2` | `sprout --depth 2` |
| `tree -I venv` | `sprout --ignore venv` |
| Complex filtering | `sprout --project` |
| Manual project filtering | `sprout --smart` |
| Basic tree output | `sprout --stats` |
| Filesystem visualization | `sprout --git` |
| Text output | `sprout --json` |

Sprout isn't intended to simply duplicate every feature of `tree`.

The goal is to provide a developer-oriented interface with sensible defaults.

## Design Philosophy

Sprout follows a few simple principles.

1. **Simple by default.** Basic usage should require almost no documentation:

   ```
   sprout
   ```

2. **Useful defaults.** Developers shouldn't need to remember a long list of directories that should usually be ignored.

3. **Transparent automation.** Smart features should explain their decisions.

4. **Human and machine friendly.** Terminal output should be pleasant to read while structured output should be easy for programs to consume.

5. **Fast.** Sprout should remain lightweight and fast enough to use repeatedly during development.

6. **Cross-platform.** Sprout should work across:

   - macOS
   - Linux
   - Windows

   and support common architectures such as:

   - amd64
   - arm64

## Built With

Sprout is written in Go.

The project intentionally relies heavily on Go's standard library where practical.

Potential areas include:

- os
- path/filepath
- flag
- encoding/json
- runtime
- filesystem APIs

External dependencies should be kept minimal.

## Roadmap

**v0.1**
- Basic directory traversal
- Tree rendering
- Directory argument
- `--hidden`
- `--depth`
- `--ignore`
- Cross-platform support
- Unit tests
- CI

**v0.2**
- `--project`
- Project type detection
- Smart filtering
- `--stats`

**v0.3**
- `--git`
- Git status indicators
- Improved project detection
- `--orphans` — cross-reference the import graph against the filesystem and flag files nothing imports; a lightweight dead-code finder that isn't scoped to one language or bolted onto a heavyweight linter
- `--churn` — parse `git log` change frequency per file and render it inline in the tree as an intensity marker, so hotspots are visible at a glance instead of living in a separate report

**v0.4**
- `--json`
- `--markdown`
- Structured output API
- `--diff <ref>..<ref>` — render a tree with added, removed, and changed subtrees nested in place, instead of a flat file-stat list, so structural impact of a PR is visible at a glance
- `--config-map` — group every config-ish file across the repo (`.env*`, `*.config.*`, `Dockerfile`, `docker-compose.yml`, CI YAML, `tsconfig`, etc.) into one flat "control plane" view instead of leaving them scattered through the tree

**v1.0**
- Stable CLI interface
- Performance benchmarks
- GoReleaser
- GitHub Releases
- Homebrew distribution
- Comprehensive documentation
- Contribution guide

**Future**
- `--ai`
- AI-oriented project context
- `--entry` — build a lightweight import/dependency graph per language and rank files by centrality (how many files import them, how deep from an entry point like `main.go` or `index.js`), producing a suggested reading order for unfamiliar codebases
- Additional output formats
- Editor integrations
- Plugin/extensibility system

## Project Structure

Once v0.1 is stable, the plan is to move Sprout under an organization and split it into focused repositories:

- **sprout-src** — the core CLI and Go source
- **sprout-docs** — documentation site and guides
- **homebrew-tap** — Homebrew tap for `brew install`
- Additional repos as needed (e.g. editor integrations, plugin registry)

This keeps the core binary lean while letting docs, distribution, and integrations evolve independently.

## Distribution

The goal is to make Sprout installable through several methods.

**Go**

```
go install github.com/Sprout-DevLabs/sprout@latest
```

**Homebrew**

Eventually:

```
brew install ManasDasri/tap/sprout
```

**GitHub Releases**

Precompiled binaries will eventually be provided for:

- macOS ARM64
- macOS AMD64
- Linux ARM64
- Linux AMD64
- Windows AMD64

Automated releases will be handled through GoReleaser.

## Contributing

Contributions are welcome.

Before submitting a pull request:

```
go test ./...
```

Please keep contributions focused and maintain the project's emphasis on:

- Simple UX
- Clear APIs
- Cross-platform compatibility
- Good documentation
- Minimal unnecessary dependencies

For larger changes, open an issue first so the design can be discussed before implementation.

## License

Sprout will be released under the MIT License.

See LICENSE for details.

## Why "Sprout"?

A directory tree shows what's growing in a project.

Sprout is meant to help you see the useful parts without having to dig through the entire forest.

Understand your project at a glance.
