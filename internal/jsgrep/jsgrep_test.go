package jsgrep

import "testing"

const astGrepOut = `[{"text":"sendMessage","range":{"byteOffset":{"start":10,"end":21},"start":{"line":41,"column":8},"end":{"line":41,"column":19}},"file":"src/components/ChatPanel.jsx","language":"Tsx"},
{"text":"sendMessage","range":{"byteOffset":{"start":99,"end":110},"start":{"line":7,"column":2},"end":{"line":7,"column":13}},"file":"src/lib/api.js","language":"JavaScript"}]`

func TestParse(t *testing.T) {
	locs, err := Parse([]byte(astGrepOut))
	if err != nil {
		t.Fatal(err)
	}
	if len(locs) != 2 {
		t.Fatalf("got %d", len(locs))
	}
	// 0-based line 41 → 1-based 42
	if locs[0].File != "src/components/ChatPanel.jsx" || locs[0].Line != 42 {
		t.Errorf("got %+v", locs[0])
	}
}

func TestParseEmpty(t *testing.T) {
	locs, err := Parse([]byte("[]"))
	if err != nil || len(locs) != 0 {
		t.Errorf("got %v, %v", locs, err)
	}
}

const apiCallOut = `[{"text":"apiJson(` + "`" + `/api/applicants/${id}` + "`" + `, {\n    method: 'PATCH',\n    body: x,\n  }, token)","range":{"byteOffset":{"start":1,"end":2},"start":{"line":78,"column":9},"end":{"line":83,"column":3}},"file":"src/lib/api.js","language":"JavaScript","metaVariables":{"single":{"PATH":{"text":"` + "`" + `/api/applicants/${id}` + "`" + `","range":{"start":{"line":78,"column":17},"end":{"line":78,"column":38}}}},"multi":{},"transformed":{}}}]`

func TestParseAPICalls(t *testing.T) {
	calls, err := ParseAPICalls([]byte(apiCallOut))
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("got %d", len(calls))
	}
	c := calls[0]
	if c.File != "src/lib/api.js" || c.Line != 79 { // 0-based 78 → 79
		t.Errorf("loc: %+v", c)
	}
	if c.RawPath != "`/api/applicants/${id}`" {
		t.Errorf("raw path: %q", c.RawPath)
	}
	if c.Method != "PATCH" {
		t.Errorf("method: %q", c.Method)
	}
}

const funcRangeOut = `[{"text":"function updateApplicant(id) { }","range":{"byteOffset":{"start":0,"end":1},"start":{"line":70,"column":0},"end":{"line":90,"column":1}},"file":"src/lib/api.js","language":"JavaScript","metaVariables":{"single":{"NAME":{"text":"updateApplicant","range":{"start":{"line":70,"column":9},"end":{"line":70,"column":24}}}},"multi":{},"transformed":{}}}]`

func TestEnclosingFunc(t *testing.T) {
	rs, err := ParseFuncRanges([]byte(funcRangeOut))
	if err != nil {
		t.Fatal(err)
	}
	if got := EnclosingFunc(rs, "src/lib/api.js", 79); got != "updateApplicant" {
		t.Errorf("got %q", got)
	}
	if got := EnclosingFunc(rs, "src/lib/api.js", 5); got != "top-level" {
		t.Errorf("outside any range: got %q", got)
	}
}

const callOut = `[{"text":"getApplicantByToken(token)","range":{"byteOffset":{"start":0,"end":1},"start":{"line":50,"column":27},"end":{"line":50,"column":53}},"file":"src/context/ApplicantContext.jsx","language":"JavaScript","metaVariables":{"single":{"FN":{"text":"getApplicantByToken","range":{"start":{"line":50,"column":27},"end":{"line":50,"column":46}}}},"multi":{},"transformed":{}}},{"text":"weird()","range":{"byteOffset":{"start":0,"end":1},"start":{"line":3,"column":0},"end":{"line":3,"column":7}},"file":"src/x.js","language":"JavaScript","metaVariables":{"single":{},"multi":{},"transformed":{}}}]`

func TestParseCalls(t *testing.T) {
	calls, err := ParseCalls([]byte(callOut))
	if err != nil {
		t.Fatal(err)
	}
	// The second match has no FN metavariable and must be dropped.
	if len(calls) != 1 {
		t.Fatalf("got %d calls", len(calls))
	}
	c := calls[0]
	if c.File != "src/context/ApplicantContext.jsx" || c.Line != 51 { // 0-based 50 → 51
		t.Errorf("loc: %+v", c)
	}
	if c.Callee != "getApplicantByToken" {
		t.Errorf("callee: %q", c.Callee)
	}
}
