package gopls

import "testing"

const hierOut = `identifier: function resolveTeamMember in /r/backend/internal/handler/common.go:73:6-23
caller[0]: ranges 434:12-29 in /r/backend/internal/handler/documents.go from/to function uploadFormTemplate in /r/backend/internal/handler/documents.go:398:27-45
caller[1]: ranges 92:8-25, 105:8-25 in /r/backend/internal/handler/form_library.go from/to function requireOwner in /r/backend/internal/handler/form_library.go:86:30-42
callee[0]: ranges 75:14-21 in /r/backend/internal/handler/common.go from/to function GetUser in /r/backend/internal/middleware/auth.go:120:6-13
`

func TestParseCallHierarchy(t *testing.T) {
	hs := ParseCallHierarchy(hierOut)
	if len(hs) != 3 {
		t.Fatalf("got %d entries, want 3", len(hs))
	}
	h := hs[0]
	if h.Direction != "caller" || h.Func != "uploadFormTemplate" ||
		h.CallFile != "/r/backend/internal/handler/documents.go" || h.CallLine != 434 {
		t.Errorf("got %+v", h)
	}
	if hs[1].CallLine != 92 { // first of multiple ranges
		t.Errorf("multi-range: got line %d, want 92", hs[1].CallLine)
	}
	if hs[2].Direction != "callee" || hs[2].Func != "GetUser" {
		t.Errorf("callee: got %+v", hs[2])
	}
}

func TestParseLocations(t *testing.T) {
	out := "/r/a.go:92:9-26\n/r/b.go:12:1-5\nnot a location\n"
	locs := ParseLocations(out)
	if len(locs) != 2 || locs[0].File != "/r/a.go" || locs[0].Line != 92 {
		t.Errorf("got %+v", locs)
	}
}

func TestParseSymbols(t *testing.T) {
	out := "/r/common.go:73:6-23 resolveTeamMember Function\n/r/labels.go:21:24-34 Handler.ListLabels Method\n"
	syms := ParseSymbols(out)
	if len(syms) != 2 {
		t.Fatalf("got %d", len(syms))
	}
	if syms[0].Name != "resolveTeamMember" || syms[0].Line != 73 || syms[0].Col != 6 {
		t.Errorf("got %+v", syms[0])
	}
	if syms[1].Name != "Handler.ListLabels" || syms[1].Kind != "Method" {
		t.Errorf("got %+v", syms[1])
	}
}
