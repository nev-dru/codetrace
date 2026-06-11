#!/bin/bash
# SessionStart: code-discovery routing for the codetrace plugin.
cat << 'REMINDER'
Code Discovery Protocol (codetrace plugin):
1. For Go/JS code-navigation questions (who-calls, refs, def, reachability,
   UI->API->DB paths, impact, dead code): use the codetrace skill FIRST —
   gopls-exact, one CLI call.
2. For other languages, architecture overviews, Cypher graph queries, or as
   fallback when codetrace has no results (dynamic dispatch): use the
   codebase-memory skill (index_repository first if the project is new).
3. Use Grep/Glob/Read freely for text, configs, and non-code files, and
   always Read a file before editing it.
REMINDER
