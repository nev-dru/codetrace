// Package output renders result lines for LLM consumption: deduped, sorted,
// truncated at MaxLines by default to protect the agent's context window.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

const MaxLines = 200

func Render(w io.Writer, lines []string, all, asJSON bool) error {
	seen := make(map[string]bool, len(lines))
	uniq := make([]string, 0, len(lines))
	for _, l := range lines {
		if !seen[l] {
			seen[l] = true
			uniq = append(uniq, l)
		}
	}
	sort.Strings(uniq)
	out, more := uniq, 0
	if !all && len(uniq) > MaxLines {
		out, more = uniq[:MaxLines], len(uniq)-MaxLines
	}
	if asJSON {
		// JSON honors the same cap — it feeds the same context window.
		doc := map[string]any{"results": out}
		if more > 0 {
			doc["truncated"] = more
		}
		return json.NewEncoder(w).Encode(doc)
	}
	for _, l := range out {
		fmt.Fprintln(w, l)
	}
	if more > 0 {
		fmt.Fprintf(w, "... %d more (use --all)\n", more)
	}
	return nil
}
