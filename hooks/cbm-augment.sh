#!/bin/bash
# codebase-memory-mcp search augmenter (PreToolUse). Adds graph context to
# search tool calls; NEVER blocks. Any failure is silent (exit 0, no output).
BIN="$(command -v codebase-memory-mcp || echo "$HOME/.local/bin/codebase-memory-mcp")"
[ -x "$BIN" ] || exit 0
"$BIN" hook-augment 2>/dev/null
exit 0
