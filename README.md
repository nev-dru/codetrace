# codetrace

Code-graph navigation for LLM agents. One CLI call answers: who calls X,
what does X reach, is X dead, what breaks if I change this — across
JS → HTTP → Go boundaries (UI→API→DB paths).

Two tools, one plugin:

- **codetrace** (this repo): gopls/VTA-exact Go analysis + syntactic JS
  (ast-grep) + cross-layer stitching. Precise; Go/JS only.
- **codebase-memory** (bundled companion skill + hooks): tree-sitter code
  graph for 159 languages via the third-party MIT
  `codebase-memory-mcp` binary. Broad; syntactic.

## Install as a Claude Code plugin (recommended)

```
/plugin marketplace add nev-dru/codetrace
/plugin install codetrace@codetrace
```

First use builds the binary from source (needs Go 1.25+); then run
`codetrace doctor` in your project and follow install hints
(gopls required; ast-grep needed for JS; codebase-memory-mcp optional).

## Install as a plain CLI

```bash
go install github.com/nev-dru/codetrace/cmd/codetrace@latest
# or: git clone https://github.com/nev-dru/codetrace && cd codetrace && make install
```

## Usage

Run from the target repo's root:

```bash
codetrace callers <symbol>      # who calls this (gopls-exact)
codetrace paths <from> <to>     # full UI→API→DB chain
codetrace impact main           # blast radius of your branch
codetrace dead                  # unreachable functions
codetrace doctor                # tool + cache health
```

Layout assumptions (v1): Go module at `./go.mod` or `backend/go.mod`
(`-C <dir>` to override); JS root `src/` or `.`; JS→HTTP edges detected at
`apiJson`/`apiRequest` call sites; chi-style Go routes. A per-project
config file (`.codetrace.yml`) for custom API wrappers, routers, and
custom stitch layers is the next milestone.

## Development

```bash
make build   # bin/codetrace
make test
make lint
```
