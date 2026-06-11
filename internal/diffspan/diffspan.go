// Package diffspan parses unified diff output into changed line spans of the
// post-change files — the input impact needs to map edits onto functions.
package diffspan

import (
	"regexp"
	"strconv"
	"strings"
)

type Span struct {
	File  string // path as printed by git (repo-relative)
	Start int    // 1-based, inclusive
	End   int    // inclusive; for pure deletions Start==End marks the seam
}

var (
	fileRe = regexp.MustCompile(`^\+\+\+ b/(.+)$`)
	hunkRe = regexp.MustCompile(`^@@ .* \+(\d+)(?:,(\d+))? @@`)
)

// Parse reads `git diff -U0` output. Each hunk contributes one span in the
// new-file coordinate system. A zero-count hunk (pure deletion) still yields
// a one-line span: the deletion site sits inside some function, and that
// function changed.
func Parse(out string) []Span {
	var spans []Span
	file := ""
	for l := range strings.SplitSeq(out, "\n") {
		if m := fileRe.FindStringSubmatch(l); m != nil {
			file = m[1]
			continue
		}
		m := hunkRe.FindStringSubmatch(l)
		if m == nil || file == "" || file == "/dev/null" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		end := start + count - 1
		if count == 0 {
			end = start
			if start == 0 { // deletion at top of file
				start = 1
				end = 1
			}
		}
		spans = append(spans, Span{File: file, Start: start, End: end})
	}
	return spans
}
