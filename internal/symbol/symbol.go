// Package symbol resolves user-supplied symbol arguments (bare name,
// pkg-or-receiver qualified name, or file:line:col) to a definition position.
package symbol

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nev-dru/codetrace/internal/cterr"
	"github.com/nev-dru/codetrace/internal/gopls"
	"github.com/nev-dru/codetrace/internal/srcfile"
)

type Ref struct {
	File string
	Line int
	Col  int
}

var posRe = regexp.MustCompile(`^(.+\.[a-z]+):(\d+):(\d+)$`)

func ParsePos(sym string) (Ref, bool) {
	m := posRe.FindStringSubmatch(sym)
	if m == nil {
		return Ref{}, false
	}
	line, _ := strconv.Atoi(m[2])
	col, _ := strconv.Atoi(m[3])
	return Ref{File: m[1], Line: line, Col: col}, true
}

// ResolveGo finds the definition position of a Go symbol via workspace_symbol.
// Returns cterr.ErrNoResults or *cterr.Ambiguous as appropriate.
func ResolveGo(modDir, sym string) (Ref, error) {
	qual, name := "", sym
	if i := strings.LastIndex(sym, "."); i >= 0 {
		qual, name = sym[:i], sym[i+1:]
	}
	out, err := gopls.Run(modDir, "workspace_symbol", name)
	if err != nil {
		return Ref{}, err
	}
	matches := pick(gopls.ParseSymbols(out), qual, name)
	switch len(matches) {
	case 0:
		return Ref{}, cterr.ErrNoResults
	case 1:
		return Ref{File: matches[0].File, Line: matches[0].Line, Col: matches[0].Col}, nil
	default:
		cands := make([]string, len(matches))
		for i, s := range matches {
			cands[i] = fmt.Sprintf("%s:%d:%d  %s (%s)", s.File, s.Line, s.Col, s.Name, s.Kind)
		}
		return Ref{}, &cterr.Ambiguous{Candidates: cands}
	}
}

// pick keeps symbols whose final name segment equals name. A qualifier must
// match the symbol's container (Recv.Name form, anchored at the start so
// "Client" cannot claim "MockDBClient") or the file's immediate parent
// directory (≈ the Go package name) — substring matching on the whole path
// would wrongly let "handler" claim files under handler/admin/. A two-part
// qualifier ("db.Client") requires the receiver AND the package directory to
// match. If the qualifier eliminates every name match, the unqualified
// matches are returned so the caller reports them as ambiguous candidates
// instead of nothing.
func pick(syms []gopls.Sym, qual, name string) []gopls.Sym {
	var nameMatches, out []gopls.Sym
	for _, s := range syms {
		last := s.Name
		if i := strings.LastIndex(last, "."); i >= 0 {
			last = last[i+1:]
		}
		if last != name {
			continue
		}
		nameMatches = append(nameMatches, s)
		if qual != "" && !qualMatches(s, qual) {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		out = nameMatches
	}
	// Prefer production definitions: same-named symbols in _test.go and
	// generated files (gomock) are scaffolding shadows of the real one. They
	// survive only when nothing else matches, so test helpers stay findable.
	genCache := map[string]bool{}
	var prod []gopls.Sym
	for _, s := range out {
		if !srcfile.IsTest(s.File) && !srcfile.IsGenerated(s.File, genCache) {
			prod = append(prod, s)
		}
	}
	if len(prod) > 0 {
		return prod
	}
	return out
}

func qualMatches(s gopls.Sym, qual string) bool {
	pkgDir := filepath.Base(filepath.Dir(s.File))
	if i := strings.LastIndex(qual, "."); i >= 0 {
		pkg, recv := qual[:i], qual[i+1:]
		if j := strings.LastIndex(pkg, "."); j >= 0 {
			pkg = pkg[j+1:]
		}
		return strings.HasPrefix(s.Name, recv+".") && pkgDir == pkg
	}
	return strings.HasPrefix(s.Name, qual+".") || pkgDir == qual
}
