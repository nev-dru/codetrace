#!/bin/bash
# SessionStart: auto-index the current project into codebase-memory if it
# isn't already. Indexing runs detached so session start never blocks.
# Silent no-op without the binary, outside git repos, or when already indexed.
BIN="$(command -v codebase-memory-mcp || echo "$HOME/.local/bin/codebase-memory-mcp")"
[ -x "$BIN" ] || exit 0
TOP="$(git rev-parse --show-toplevel 2>/dev/null)"
[ -n "$TOP" ] || exit 0
NAME="$(printf '%s' "${TOP#/}" | tr '/' '-')"
[ -f "$HOME/.cache/codebase-memory-mcp/$NAME.db" ] && exit 0

# Resume/compact re-fire SessionStart; don't stack indexers while one runs.
LOCK="${TMPDIR:-/tmp}/cbm-index-$NAME.lock"
if [ -f "$LOCK" ] && [ -n "$(find "$LOCK" -mmin -10 2>/dev/null)" ]; then
  exit 0
fi
touch "$LOCK"
( "$BIN" cli index_repository "{\"repo_path\": \"$TOP\"}" >/dev/null 2>&1; rm -f "$LOCK" ) &

echo "codebase-memory: '$NAME' is not indexed yet — indexing started in the background; graph queries may be incomplete for the first minute."
exit 0
