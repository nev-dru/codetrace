// Package cterr defines sentinel errors that map to codetrace exit codes.
package cterr

import (
	"errors"
	"strings"
)

// ErrNoResults maps to exit code 1.
var ErrNoResults = errors.New("no results")

// Ambiguous maps to exit code 2; main prints the candidates.
type Ambiguous struct{ Candidates []string }

func (a *Ambiguous) Error() string {
	return "ambiguous symbol; candidates:\n" + strings.Join(a.Candidates, "\n")
}
