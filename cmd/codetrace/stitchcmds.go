package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/nev-dru/codetrace/internal/cterr"
	"github.com/nev-dru/codetrace/internal/graph"
	"github.com/nev-dru/codetrace/internal/jsgrep"
	"github.com/nev-dru/codetrace/internal/normurl"
	"github.com/nev-dru/codetrace/internal/output"
)

// loadStitched returns the cached go:+http: graph with fresh js: boundary
// edges merged in-memory.
func loadStitched(o opts) (*graph.Graph, error) {
	g, err := graph.Load(o.modDir)
	if err != nil {
		return nil, err
	}
	root := jsgrep.Root()
	calls, err := jsgrep.APICalls(root)
	var ranges []jsgrep.FuncRange
	if err == nil {
		ranges, err = jsgrep.FuncRanges(root)
	}
	if err != nil {
		// JS edges are additive — a missing ast-grep shouldn't break Go-only paths.
		fmt.Fprintf(os.Stderr, "warning: JS edges unavailable: %v\n", err)
		return g, nil
	}
	var nodes []string
	var edges [][2]string
	skipped := 0
	for _, c := range calls {
		ep, ok := normurl.TryNormalize(c.Method, c.RawPath)
		if !ok {
			skipped++
			continue
		}
		fn := jsgrep.EnclosingFunc(ranges, c.File, c.Line)
		src := fmt.Sprintf("js:%s:%s", c.File, fn)
		dst := "http:" + ep
		nodes = append(nodes, src, dst)
		edges = append(edges, [2]string{src, dst})
	}
	g.Extend(nodes, edges, graph.EdgeHTTP)
	g.Extend(jsCallEdges(ranges))
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "note: %d dynamic URL(s) skipped (not literal paths)\n", skipped)
	}
	return g, nil
}

// jsCallEdges links JS functions to the JS functions they call, so paths can
// start at a React component instead of only the src/lib/api.js wrappers.
// Resolution is syntactic: a callee binds to a definition in the same file,
// or to the single repo-wide definition of that name; ambiguous names are
// skipped (absent beats confidently-wrong).
func jsCallEdges(ranges []jsgrep.FuncRange) ([]string, [][2]string, uint8) {
	calls, err := jsgrep.Calls(jsgrep.Root())
	if err != nil {
		// Same degradation contract as the HTTP edges — Go-only still works.
		fmt.Fprintf(os.Stderr, "warning: JS call edges unavailable: %v\n", err)
		return nil, nil, graph.EdgeCall
	}
	defFiles := map[string][]string{}
	for _, r := range ranges {
		defFiles[r.Name] = append(defFiles[r.Name], r.File)
	}
	var nodes []string
	var edges [][2]string
	seen := map[string]bool{}
	for _, c := range calls {
		files := defFiles[c.Callee]
		if len(files) == 0 {
			continue // builtin, import from node_modules, etc.
		}
		target := ""
		for _, f := range files {
			if f == c.File {
				target = f
				break
			}
		}
		if target == "" {
			uniq := map[string]bool{}
			for _, f := range files {
				uniq[f] = true
			}
			if len(uniq) != 1 {
				continue
			}
			target = files[0]
		}
		src := fmt.Sprintf("js:%s:%s", c.File, jsgrep.EnclosingFunc(ranges, c.File, c.Line))
		dst := fmt.Sprintf("js:%s:%s", target, c.Callee)
		if src == dst || seen[src+"|"+dst] {
			continue
		}
		seen[src+"|"+dst] = true
		nodes = append(nodes, src, dst)
		edges = append(edges, [2]string{src, dst})
	}
	return nodes, edges, graph.EdgeCall
}

func cmdPaths(args []string) error {
	o, pos, err := parseFlags("paths", args, 2)
	if err != nil {
		return err
	}
	g, err := loadStitched(o)
	if err != nil {
		return err
	}
	from, err := matchOrAmbiguous(g, pos[0])
	if err != nil {
		return err
	}
	to, err := matchOrAmbiguous(g, pos[1])
	if err != nil {
		return err
	}
	// nodes on some from→to path = forward(from) ∩ reverse(to)
	inFwd := map[int32]bool{}
	for _, i := range g.Forward(from) {
		inFwd[i] = true
	}
	onPath := map[int32]bool{}
	for _, i := range g.Reverse(to) {
		if inFwd[i] {
			onPath[i] = true
		}
	}
	if o.mermaid {
		return renderMermaid(g, onPath, modulePath(o.modDir))
	}
	var lines []string
	for ei, e := range g.Edges {
		if onPath[e[0]] && onPath[e[1]] {
			arrow := "->"
			if g.Kinds[ei] == graph.EdgeHTTP {
				arrow = "=http=>"
			}
			lines = append(lines, fmt.Sprintf("%s %s %s", g.Name(e[0]), arrow, g.Name(e[1])))
		}
	}
	// Legend: definition site of every hop ("@" sorts these above the edges).
	// With positions inline the chain needs no follow-up def/read calls.
	for i := range onPath {
		if p := g.Position(i); p != "" {
			lines = append(lines, fmt.Sprintf("@ %s  %s", g.Name(i), rel(p)))
		}
	}
	if len(lines) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, stripModule(lines, modulePath(o.modDir)), o.all, o.json)
}

// renderMermaid prints the on-path subgraph as a Mermaid flowchart — paste
// into GitHub markdown, PR descriptions, or mermaid.live. Node shape encodes
// the layer: stadium = JS, double-rect = HTTP endpoint, rect = Go.
func renderMermaid(g *graph.Graph, onPath map[int32]bool, mod string) error {
	fmt.Println("flowchart LR")
	ids := map[int32]string{}
	n := 0
	declare := func(i int32) string {
		if s, ok := ids[i]; ok {
			return s
		}
		s := fmt.Sprintf("n%d", n)
		n++
		ids[i] = s
		name := g.Name(i)
		if mod != "" {
			name = strings.ReplaceAll(strings.ReplaceAll(name, mod+"/", ""), mod+".", "")
		}
		label := strings.ReplaceAll(name, `"`, `'`)
		if p := g.Position(i); p != "" {
			label += "<br/>" + rel(p)
		}
		switch {
		case strings.HasPrefix(name, "js:"):
			fmt.Printf("    %s([\"%s\"])\n", s, label)
		case strings.HasPrefix(name, "http:"):
			fmt.Printf("    %s[[\"%s\"]]\n", s, label)
		default:
			fmt.Printf("    %s[\"%s\"]\n", s, label)
		}
		return s
	}
	for ei, e := range g.Edges {
		if !onPath[e[0]] || !onPath[e[1]] {
			continue
		}
		src, dst := declare(e[0]), declare(e[1])
		arrow := "-->"
		if g.Kinds[ei] == graph.EdgeHTTP {
			arrow = "==>|http|"
		}
		fmt.Printf("    %s %s %s\n", src, arrow, dst)
	}
	if n == 0 {
		return cterr.ErrNoResults
	}
	return nil
}

func cmdEndpoint(args []string) error {
	o, pos, err := parseFlags("endpoint", args, 2)
	if err != nil {
		return err
	}
	ep, ok := normurl.TryNormalize(pos[0], pos[1])
	if !ok {
		return fmt.Errorf("not a path literal: %s", pos[1])
	}
	g, err := loadStitched(o)
	if err != nil {
		return err
	}
	// The input is already normalized — require an exact endpoint node.
	// Substring fallback would silently union distinct endpoints
	// (/api/applicants and /api/applicants/{}).
	ids := g.Match("http:" + ep)
	switch {
	case len(ids) == 0:
		fmt.Fprintln(os.Stderr, "hint: endpoint not in graph — the route may be unmapped (see `codetrace doctor`) or the path needs its {} template form")
		return cterr.ErrNoResults
	case len(ids) > 1 || g.Name(ids[0]) != "http:"+ep:
		cands := make([]string, len(ids))
		for i, id := range ids {
			cands[i] = g.Name(id)
		}
		return &cterr.Ambiguous{Candidates: cands}
	}
	var lines []string
	// JS call sites: transitive predecessors; Go handler + reach: successors.
	for _, i := range g.Reverse(ids) {
		if n := g.Name(i); strings.HasPrefix(n, "js:") {
			lines = append(lines, "caller  "+n)
		}
	}
	mod := modulePath(o.modDir)
	for _, i := range g.Forward(ids) {
		n := g.Name(i)
		if strings.HasPrefix(n, "go:") && strings.Contains(n, mod) {
			if p := g.Position(i); p != "" {
				n += "  " + rel(p)
			}
			lines = append(lines, "reaches "+n)
		}
	}
	if len(lines) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, stripModule(lines, mod), o.all, o.json)
}
