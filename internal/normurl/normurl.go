// Package normurl canonicalizes URL path templates from both sides of the
// HTTP boundary so they join on equality: frontend `/api/x/${id}` and chi
// "/api/x/{id}" both become "METHOD /api/x/{}".
package normurl

import (
	"regexp"
	"strings"
)

var paramSegment = regexp.MustCompile(`\$\{[^}]*\}|\{[^}]*\}|^:[A-Za-z_]\w*$`)

// Normalize returns "METHOD /path" with parameter segments collapsed to {}.
// The raw path may carry JS quotes/backticks and a query string.
func Normalize(method, raw string) string {
	s, _ := TryNormalize(method, raw)
	return s
}

// TryNormalize reports ok=false when the path is not a resolvable literal
// (a bare identifier, or starts with a template expression) — callers report
// those for manual mapping instead of joining on garbage.
func TryNormalize(method, raw string) (string, bool) {
	p := strings.TrimSpace(raw)
	p = strings.Trim(p, "'\"`")
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	if p == "" || !strings.HasPrefix(p, "/") {
		return "", false
	}
	segs := strings.Split(p, "/")
	for i, seg := range segs {
		if paramSegment.MatchString(seg) {
			segs[i] = "{}"
		}
	}
	p = strings.Join(segs, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		p = "/"
	}
	return strings.ToUpper(method) + " " + p, true
}
