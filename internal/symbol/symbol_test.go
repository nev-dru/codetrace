package symbol

import (
	"testing"

	"github.com/nev-dru/codetrace/internal/gopls"
)

func TestParsePos(t *testing.T) {
	ref, ok := ParsePos("internal/handler/common.go:73:6")
	if !ok || ref.File != "internal/handler/common.go" || ref.Line != 73 || ref.Col != 6 {
		t.Errorf("got %+v ok=%v", ref, ok)
	}
	if _, ok := ParsePos("resolveTeamMember"); ok {
		t.Error("bare name should not parse as position")
	}
	if _, ok := ParsePos("handler.resolveTeamMember"); ok {
		t.Error("qualified name should not parse as position")
	}
}

func TestPickExactName(t *testing.T) {
	syms := []gopls.Sym{
		{File: "/r/a.go", Line: 1, Col: 1, Name: "resolveTeamMember", Kind: "Function"},
		{File: "/r/b.go", Line: 2, Col: 1, Name: "resolveTeamMemberFast", Kind: "Function"},
	}
	got := pick(syms, "", "resolveTeamMember")
	if len(got) != 1 || got[0].File != "/r/a.go" {
		t.Errorf("got %+v", got)
	}
}

func TestPickQualified(t *testing.T) {
	syms := []gopls.Sym{
		{File: "/r/internal/handler/labels.go", Line: 21, Col: 24, Name: "Handler.ListLabels", Kind: "Method"},
		{File: "/r/internal/admin/labels.go", Line: 9, Col: 1, Name: "ListLabels", Kind: "Function"},
	}
	got := pick(syms, "Handler", "ListLabels")
	if len(got) != 1 || got[0].Name != "Handler.ListLabels" {
		t.Errorf("got %+v", got)
	}
	// qualifier can also match the file path (package-style qualifier)
	got = pick(syms, "admin", "ListLabels")
	if len(got) != 1 || got[0].File != "/r/internal/admin/labels.go" {
		t.Errorf("got %+v", got)
	}
}

func TestPickQualifierMatchesPackageDirExactly(t *testing.T) {
	// "handler" must not claim files under handler/admin/ — the package
	// qualifier matches the immediate parent directory only.
	syms := []gopls.Sym{
		{File: "/r/internal/handler/documents.go", Line: 743, Col: 6, Name: "jsonError", Kind: "Function"},
		{File: "/r/internal/handler/admin/service.go", Line: 201, Col: 6, Name: "jsonError", Kind: "Function"},
	}
	got := pick(syms, "handler", "jsonError")
	if len(got) != 1 || got[0].File != "/r/internal/handler/documents.go" {
		t.Errorf("got %+v", got)
	}
	got = pick(syms, "admin", "jsonError")
	if len(got) != 1 || got[0].File != "/r/internal/handler/admin/service.go" {
		t.Errorf("got %+v", got)
	}
}

func TestPickUnmatchedQualifierFallsBackToNameMatches(t *testing.T) {
	syms := []gopls.Sym{
		{File: "/r/internal/handler/a.go", Line: 1, Col: 1, Name: "jsonError", Kind: "Function"},
		{File: "/r/internal/admin/b.go", Line: 2, Col: 1, Name: "jsonError", Kind: "Function"},
	}
	// wrong qualifier: better to surface both candidates than "no results"
	got := pick(syms, "nosuchpkg", "jsonError")
	if len(got) != 2 {
		t.Errorf("got %+v, want both name matches", got)
	}
}
