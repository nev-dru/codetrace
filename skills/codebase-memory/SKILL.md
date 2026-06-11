---
name: codebase-memory
description: Use for code-structure questions in ANY language (who-calls, impact, architecture, cross-file relationships) — especially in projects without codetrace. Wraps the codebase-memory-mcp CLI: a local 159-language code knowledge graph. Query the graph instead of grep/Read — ~90% fewer tokens on multi-file questions.
---

# codebase-memory — cross-language code graph for agents

Binary: `~/.local/bin/codebase-memory-mcp` (single static binary, MIT, fully local).
Run `CMM cli <tool> '<json>'` from anywhere; append `2>/dev/null` — logs go to stderr, JSON to stdout. Pipe to `jq` if you need to trim.

```bash
CMM=~/.local/bin/codebase-memory-mcp
```

**In Go projects with codetrace installed, prefer codetrace** for callers/callees/paths/endpoint — it's gopls-exact (no false positives, mocks/tests excluded). Use codebase-memory for: non-Go languages, architecture overviews, Cypher queries, and as fallback when codetrace returns no results (dynamic dispatch).

## First use in a project

```bash
$CMM cli index_repository '{"repo_path": "/abs/path/to/repo"}' 2>/dev/null
```
~1s per 750 files. The **project name** is the absolute path with `/` → `-`, leading slash dropped (e.g. `/home/me/code/myapp` → `home-me-code-myapp`). Confirm with:
```bash
$CMM cli list_projects '{}' 2>/dev/null
```
Re-index after big changes: same command (incremental, uses file hashes). `detect_changes` maps a git diff to affected symbols.

## Question → command

Set `P=<project-name>` first. **`query_graph` (Cypher) is the workhorse — prefer it over `search_graph`/`trace_path`**, which miss methods.

| Question | Command |
|---|---|
| Who calls X? | `$CMM cli query_graph '{"project":"'$P'","query":"MATCH (f)-[:CALLS]->(g {name: \"X\"}) RETURN f.name, f.file_path LIMIT 100"}'` |
| What does X call? | `$CMM cli query_graph '{"project":"'$P'","query":"MATCH (h {qualified_name: \"<QN>\"})-[:CALLS]->(g) RETURN g.name, g.file_path LIMIT 60"}'` |
| Find symbol / disambiguate | `$CMM cli query_graph '{"project":"'$P'","query":"MATCH (m) WHERE m.name = \"X\" RETURN m.qualified_name, m.file_path LIMIT 10"}'` |
| Fuzzy name search | `$CMM cli search_graph '{"project":"'$P'","name_pattern":".*Handler.*"}'` |
| Architecture overview | `$CMM cli get_architecture '{"project":"'$P'","aspects":["layers"]}'` — **one aspect at a time** (`layers`, `entry_points`, `hotspots`, `communities`); `["all"]` is ~40KB |
| HTTP routes | `MATCH (r:Route) WHERE r.name =~ ".*applicants.*" RETURN r.name LIMIT 20` via query_graph |
| Source for a symbol | `$CMM cli get_code_snippet '{"project":"'$P'","qualified_name":"<QN>"}'` |
| Impact of my diff | `$CMM cli detect_changes '{"project":"'$P'"}'` |
| Semantic/text search | `$CMM cli search_code '{"project":"'$P'","query":"rate limiting"}'` |

## Conventions & gotchas

- **Methods share names across types** (handler.UpdateApplicant vs db.UpdateApplicant vs mocks). Always resolve via the disambiguation query, then use `qualified_name`. QNs look like `<project>.backend.internal.handler.applicants.UpdateApplicant`.
- Param names are inconsistent: `index_repository` wants `repo_path`; `trace_path` wants `function_name`. On "X is required" errors, the message names the right param.
- `trace_path` only sees `Function` nodes — returns empty `callers/callees` for methods. Use Cypher instead.
- Results include test/mock files — filter with `WHERE NOT f.file_path =~ ".*_test.*"` when noise matters.
- Node labels: `Function`, `Method`, `Class`, `Variable`, `File`, `Module`, `Route`, `Section`. Edges: `CALLS`, `IMPORTS`, `IMPLEMENTS` (full list: `get_graph_schema`).
- Tree-sitter syntactic, not compiler-precise: edges can over/under-approximate on dynamic dispatch and same-name collisions. For load-bearing Go answers, cross-check with codetrace.
- Graph DB lives in `~/.cache/codebase-memory-mcp/` — per-project, persists across sessions.
