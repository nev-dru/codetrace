// Package srcfile classifies source files as test or generated code so
// commands can keep scaffolding out of default output. Both kinds are real
// code, so filters always pair with an opt-in flag and a hidden-count note.
package srcfile

import (
	"os"
	"regexp"
	"strings"
)

func IsTest(path string) bool { return strings.HasSuffix(path, "_test.go") }

// genRe matches the Go convention for generated files (golang.org/s/generatedcode).
var genRe = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.?$`)

// IsGenerated reports whether the file carries the standard generated-code
// header before its package clause. Results are memoized in cache (one map
// per command invocation).
func IsGenerated(path string, cache map[string]bool) bool {
	if v, ok := cache[path]; ok {
		return v
	}
	gen := false
	if b, err := os.ReadFile(path); err == nil {
		for i, l := range strings.SplitN(string(b), "\n", 40) {
			if i >= 30 || strings.HasPrefix(l, "package ") {
				break
			}
			if genRe.MatchString(strings.TrimRight(l, "\r")) {
				gen = true
				break
			}
		}
	}
	cache[path] = gen
	return gen
}
