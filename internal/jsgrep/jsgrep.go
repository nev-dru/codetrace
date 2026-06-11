// Package jsgrep answers JS/JSX structural queries by shelling out to
// ast-grep. Matches are syntactic only — no type resolution exists for plain
// .jsx; the skill documents this precision gap.
package jsgrep

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type Loc struct {
	File string
	Line int // 1-based
}

type match struct {
	File  string `json:"file"`
	Text  string `json:"text"`
	Range struct {
		Start struct {
			Line int `json:"line"` // 0-based
		} `json:"start"`
		End struct {
			Line int `json:"line"` // 0-based
		} `json:"end"`
	} `json:"range"`
	MetaVariables struct {
		Single map[string]struct {
			Text string `json:"text"`
		} `json:"single"`
	} `json:"metaVariables"`
}

func Parse(out []byte) ([]Loc, error) {
	var ms []match
	if err := json.Unmarshal(out, &ms); err != nil {
		return nil, fmt.Errorf("ast-grep json: %w", err)
	}
	locs := make([]Loc, len(ms))
	for i, m := range ms {
		locs[i] = Loc{File: m.File, Line: m.Range.Start.Line + 1}
	}
	return locs, nil
}

// runRaw executes ast-grep with a pattern over root (the frontend source
// dir) and returns the raw JSON match array.
func runRaw(root, pattern string) ([]byte, error) {
	if _, err := exec.LookPath("ast-grep"); err != nil {
		return nil, fmt.Errorf("ast-grep not installed (see https://ast-grep.github.io for install options)")
	}
	cmd := exec.Command("ast-grep", "run", "--pattern", pattern, "--json=compact", root)
	out, err := cmd.Output()
	if err != nil {
		// ast-grep uses grep exit semantics: 1 = no matches, still valid JSON.
		ee, ok := err.(*exec.ExitError)
		if !ok || ee.ExitCode() != 1 {
			return nil, fmt.Errorf("ast-grep: %v", err)
		}
		if !json.Valid(out) {
			return nil, fmt.Errorf("ast-grep: %v\n%s", err, ee.Stderr)
		}
	}
	return out, nil
}

func run(root, pattern string) ([]Loc, error) {
	out, err := runRaw(root, pattern)
	if err != nil {
		return nil, err
	}
	return Parse(out)
}

// Root picks the frontend source directory: src/ if present, else cwd.
func Root() string {
	if st, err := os.Stat("src"); err == nil && st.IsDir() {
		return "src"
	}
	return "."
}

// Refs finds every identifier occurrence of sym.
func Refs(root, sym string) ([]Loc, error) {
	return run(root, sym)
}

type APICall struct {
	File    string
	Line    int // 1-based
	RawPath string
	Method  string
}

var methodRe = regexp.MustCompile(`method:\s*['"]([A-Za-z]+)['"]`)

func ParseAPICalls(out []byte) ([]APICall, error) {
	var ms []match
	if err := json.Unmarshal(out, &ms); err != nil {
		return nil, fmt.Errorf("ast-grep json: %w", err)
	}
	calls := make([]APICall, 0, len(ms))
	for _, m := range ms {
		method := "GET"
		if g := methodRe.FindStringSubmatch(m.Text); g != nil {
			method = strings.ToUpper(g[1])
		}
		calls = append(calls, APICall{
			File:    m.File,
			Line:    m.Range.Start.Line + 1,
			RawPath: m.MetaVariables.Single["PATH"].Text,
			Method:  method,
		})
	}
	return calls, nil
}

// APICalls finds every apiJson/apiRequest call site under root — the single
// HTTP choke point in this codebase's frontend.
func APICalls(root string) ([]APICall, error) {
	patterns := []string{
		"apiJson($PATH, $$$)", "apiJson($PATH)",
		"apiRequest($PATH, $$$)", "apiRequest($PATH)",
	}
	seen := map[string]bool{}
	var all []APICall
	for _, p := range patterns {
		out, err := runRaw(root, p)
		if err != nil {
			return nil, err
		}
		calls, err := ParseAPICalls(out)
		if err != nil {
			return nil, err
		}
		for _, c := range calls {
			key := fmt.Sprintf("%s:%d", c.File, c.Line)
			if !seen[key] {
				seen[key] = true
				all = append(all, c)
			}
		}
	}
	return all, nil
}

type Call struct {
	File   string
	Line   int // 1-based
	Callee string
}

func ParseCalls(out []byte) ([]Call, error) {
	var ms []match
	if err := json.Unmarshal(out, &ms); err != nil {
		return nil, fmt.Errorf("ast-grep json: %w", err)
	}
	calls := make([]Call, 0, len(ms))
	for _, m := range ms {
		fn := m.MetaVariables.Single["FN"].Text
		if fn == "" {
			continue
		}
		calls = append(calls, Call{File: m.File, Line: m.Range.Start.Line + 1, Callee: fn})
	}
	return calls, nil
}

// Calls finds every bare-identifier call site under root ($FN is a single
// node, so member calls like obj.method() are excluded by construction).
// Callers filter the callee names against known definitions — builtins and
// imports from node_modules fall out of the join.
func Calls(root string) ([]Call, error) {
	out, err := runRaw(root, "$FN($$$)")
	if err != nil {
		return nil, err
	}
	return ParseCalls(out)
}

type FuncRange struct {
	File  string
	Name  string
	Start int // 1-based, inclusive
	End   int
}

func ParseFuncRanges(out []byte) ([]FuncRange, error) {
	var ms []match
	if err := json.Unmarshal(out, &ms); err != nil {
		return nil, fmt.Errorf("ast-grep json: %w", err)
	}
	rs := make([]FuncRange, 0, len(ms))
	for _, m := range ms {
		name := m.MetaVariables.Single["NAME"].Text
		if name == "" {
			continue
		}
		rs = append(rs, FuncRange{File: m.File, Name: name, Start: m.Range.Start.Line + 1, End: m.Range.End.Line + 1})
	}
	return rs, nil
}

// FuncRanges finds named function declarations and function-valued bindings —
// the candidates for "which function encloses this call site". Bindings must
// be arrow functions: a bare `const $NAME = $$$` would match value bindings
// like `const data = await apiJson(...)` and steal the enclosing-function
// title from the real component function.
func FuncRanges(root string) ([]FuncRange, error) {
	patterns := []string{
		"function $NAME($$$) { $$$ }",
		"async function $NAME($$$) { $$$ }",
		"const $NAME = ($$$) => { $$$ }",
		"const $NAME = async ($$$) => { $$$ }",
		"const $NAME = ($$$) => $$$",
		"const $NAME = async ($$$) => $$$",
	}
	var all []FuncRange
	for _, p := range patterns {
		out, err := runRaw(root, p)
		if err != nil {
			return nil, err
		}
		rs, err := ParseFuncRanges(out)
		if err != nil {
			return nil, err
		}
		all = append(all, rs...)
	}
	return all, nil
}

// EnclosingFunc returns the name of the smallest range containing file:line,
// or "top-level".
func EnclosingFunc(rs []FuncRange, file string, line int) string {
	best, bestSize := "top-level", 1<<31-1
	for _, r := range rs {
		if r.File == file && r.Start <= line && line <= r.End {
			if size := r.End - r.Start; size < bestSize {
				best, bestSize = r.Name, size
			}
		}
	}
	return best
}

// Defs finds likely definition sites of sym (function decls and const/let
// bindings — the dominant patterns in this repo's plain-React code).
func Defs(root, sym string) ([]Loc, error) {
	patterns := []string{
		fmt.Sprintf("function %s($$$) { $$$ }", sym),
		fmt.Sprintf("const %s = $$$", sym),
		fmt.Sprintf("let %s = $$$", sym),
	}
	var all []Loc
	for _, p := range patterns {
		locs, err := run(root, p)
		if err != nil {
			return nil, err
		}
		all = append(all, locs...)
	}
	return all, nil
}
