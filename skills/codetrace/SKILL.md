---
name: codetrace
description: Use for ANY code-navigation question — who calls X, where is X used/defined, what does X reach, what can reach X, recursion cycles, dead code — in Go or JS. One CLI call replaces many Grep/Read round-trips. Read files only to understand logic AFTER locating it with codetrace.
---

# codetrace — code tracing for agents

Binary: `<plugin root>/bin/codetrace` — the plugin root is two directories up
from this skill file (this file lives at `<plugin root>/skills/codetrace/`).
Always invoke it with the TARGET repo's root as cwd.

**Bootstrap (first use on a machine):** if the binary is missing, build it —
`cd <plugin root> && go build -o bin/codetrace ./cmd/codetrace` (requires a Go
toolchain). Then run `codetrace doctor` from the target repo and follow
install hints.

| Question | Command |
|---|---|
| Who calls X? / What does X call? | `callers <sym>` / `callees <sym>` |
| Where is X used / defined? (Go or JS) | `refs <sym>` (`--js` forces JS) / `def <sym>` |
| Show me X's source | `def <sym> --body` (Go; prints the function in the same call) |
| What does X reach / what can reach X? | `reachable <sym>` / `reaches <sym>` |
| Recursion cycles / unreachable functions | `cycles` / `dead` |
| Full UI→API→DB chain between two points | `paths <from> <to>` |
| Endpoint's JS callers + handler blast radius | `endpoint <METHOD> <path>` |
| What can my uncommitted/branch changes break? | `impact` / `impact main` |
| Diagram of a flow (for PRs/docs) | `paths <from> <to> --mermaid` |

**Cross-layer questions: run `paths` FIRST, and trust it.** "Trace X from
frontend to DB" is ONE `paths` call — its output is the complete chain plus
`@` legend lines giving each hop's file:line. That is self-sufficient: cite
it directly, never reconstruct or confirm it with other commands or reads.
Need the logic inside one hop (e.g. the SQL)? `def <that sym> --body`.
JS endpoints are addressed as `js:<file>:<funcName>` (e.g.
`js:src/lib/api.js:getUser`). `endpoint`'s "reaches" section is a
blast-radius dump — impact analysis only, never flow tracing.
Graph results (`reachable`/`reaches`/`dead`) also carry file:line inline.

**Trust precise results — never re-verify.** `callers`/`callees`/`refs`/`def`
are gopls-backed: exact, complete, no false positives; a grep cross-check
doubles cost and adds nothing. Only graph commands ever warrant
spot-verification, at the call sites that matter.

Symbols: bare (`handleLogin`), qualified (`Handler.List`,
`db.Client.GetUser` — receiver and/or package dir, anchored so `Client`
never matches `MockDBClient`), or position (`file.go:73:6`). If you know the
file, use position/qualified form — bare common names cost an ambiguity
round-trip. Exit codes: 0 results · 1 none (try broader name, then grep) ·
2 ambiguous (candidates listed; re-run with one) · 3 error.
Flags: `--json` · `--all` (no 200-line cap) · `-C <dir>` module dir · `--js`
· `--body` (def: print source) · `--mermaid` (paths: flowchart for PRs/docs).
Results in _test.go and generated files (gomock) are hidden by default with
a stderr count — `--tests` / `--generated` restore them; if EVERY result is
scaffolding they're shown anyway. Symbol resolution prefers production
definitions, so mock shadows never cause ambiguity.

## Repo layout assumptions (v1 — config file comes later)

- Go module auto-detected at `./go.mod`, then `backend/go.mod`; override with `-C <dir>`.
- JS root: `src/` if present, else the repo root.
- JS→HTTP boundary edges are detected at `apiJson(path, ...)` / `apiRequest(path, ...)`
  call sites; Go routes via chi-style registrations. Projects using other
  wrappers/routers get Go-only + JS-only graphs (still useful; stitching
  becomes configurable in a later version).

## Caveats

1. Graph commands (`reachable`/`reaches`/`cycles`/`dead`) over-approximate
   dynamic dispatch: edges are POSSIBLE, not actual (an interface call links
   every concrete type that could flow there). gopls commands are precise.
2. All output is call-flow, not data-flow — for value logic, read the body.
3. JS answers are syntactic only; same-named identifiers can collide.
4. Cross-language edges sit at apiJson/apiRequest call sites; JS→JS call
   edges link components to helpers when the callee is defined in the same
   file or has a unique repo-wide name (ambiguous names are skipped — if
   `paths` from a component is empty, query from the API-wrapper module).
   URL params normalize to `{}`. Raw-fetch callers (e.g. SSE) are
   invisible; `doctor` lists unmapped routes.
5. `dead` = unreachable in the shipped binary, no tests/reflection modeled —
   not "safe to delete". Generated files (gomock) are hidden by default
   (`--generated` shows them). Differs from golangci's `unused`, which
   counts test usage and only flags unexported symbols.
   `impact` maps the git diff to changed functions, then reports upstream
   endpoints/js/handlers — over-approximate by design ("more to test").
6. Graph commands rebuild a cache after .go edits (~3-10s, prints
   "rebuilding"); gopls commands may take seconds on first use (cold index).
   `cycles` always contains one giant interface-dispatch SCC — ignore it.
