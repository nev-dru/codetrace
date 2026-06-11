// Package graph builds and queries a whole-program call graph. The build uses
// VTA over SSA: sound but over-approximate — an edge means "possible", not
// "observed". The skill documents this for the agent.
package graph

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/nev-dru/codetrace/internal/routes"
)

// SchemaVersion gates the gob cache: bump it whenever the Graph layout or
// edge semantics change so stale caches rebuild instead of mis-decoding.
const SchemaVersion = 4

// Edge kinds, parallel to Edges.
const (
	EdgeCall uint8 = 0 // go: → go: function call
	EdgeHTTP uint8 = 1 // js: → http: boundary, or http: → go: bridge
)

type Graph struct {
	Version  int
	Rev      string
	Nodes    []string // namespaced: "go:" / "http:" / "js:"
	Pos      []string // parallel to Nodes: "file:line" of the definition, or ""
	End      []int32  // parallel to Nodes: last line of the definition, 0 if unknown
	Edges    [][2]int32
	Kinds    []uint8 // parallel to Edges
	Roots    []int32 // main.main + main package inits
	Unmapped []string // route registrations whose handler couldn't be resolved
	index    map[string]int32
}

// initName matches SSA package initializers exactly: "init" and "init#1",
// "init#2"… — NOT user functions like initDB, which must stay eligible for
// dead-code reporting.
var initName = regexp.MustCompile(`^init(#\d+)?$`)

func Build(modDir, rev string) (*Graph, error) {
	cfg := &packages.Config{Mode: packages.LoadAllSyntax, Dir: modDir}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	var firstErr string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if firstErr == "" && len(p.Errors) > 0 {
			firstErr = p.Errors[0].Error()
		}
	})
	if firstErr != "" {
		return nil, fmt.Errorf("fix compile errors first: %s", firstErr)
	}
	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	cg := vta.CallGraph(ssautil.AllFunctions(prog), cha.CallGraph(prog))
	cg.DeleteSyntheticNodes()

	g := &Graph{Version: SchemaVersion, Rev: rev, index: map[string]int32{}}
	id := func(name string) int32 {
		if i, ok := g.index[name]; ok {
			return i
		}
		i := int32(len(g.Nodes))
		g.Nodes = append(g.Nodes, name)
		g.Pos = append(g.Pos, "")
		g.End = append(g.End, 0)
		g.index[name] = i
		return i
	}
	// idf records the definition span alongside the node so query output is
	// self-sufficient (file:line without a follow-up def call) and diff lines
	// can be mapped to their containing function (impact).
	idf := func(fn *ssa.Function) int32 {
		i := id("go:" + fn.String())
		if g.Pos[i] == "" && fn.Pos().IsValid() {
			p := prog.Fset.Position(fn.Pos())
			g.Pos[i] = fmt.Sprintf("%s:%d", p.Filename, p.Line)
			if syn := fn.Syntax(); syn != nil && syn.End().IsValid() {
				g.End[i] = int32(prog.Fset.Position(syn.End()).Line)
			}
		}
		return i
	}
	for fn, node := range cg.Nodes {
		if fn == nil {
			continue
		}
		src := idf(fn)
		for _, e := range node.Out {
			if e.Callee == nil || e.Callee.Func == nil {
				continue
			}
			g.Edges = append(g.Edges, [2]int32{src, idf(e.Callee.Func)})
			g.Kinds = append(g.Kinds, EdgeCall)
		}
		if fn.Pkg != nil && fn.Pkg.Pkg.Name() == "main" &&
			(fn.Name() == "main" || initName.MatchString(fn.Name())) {
			g.Roots = append(g.Roots, src)
		}
	}

	// Bridge edges: chi route table from the SAME typed packages the SSA
	// build consumed — handler FullNames are identical to the go: node names
	// above, so the join cannot drift.
	bridges, unmapped := routes.Extract(pkgs)
	for _, b := range bridges {
		g.Edges = append(g.Edges, [2]int32{id("http:" + b.Endpoint), id("go:" + b.HandlerFullName)})
		g.Kinds = append(g.Kinds, EdgeHTTP)
	}
	g.Unmapped = unmapped
	return g, nil
}

// Extend merges query-time nodes/edges (JS boundary edges) into the graph
// in-memory. Never persisted — the cache holds only go:+http: edges.
func (g *Graph) Extend(nodes []string, edges [][2]string, kind uint8) {
	id := func(name string) int32 {
		if i, ok := g.index[name]; ok {
			return i
		}
		i := int32(len(g.Nodes))
		g.Nodes = append(g.Nodes, name)
		g.Pos = append(g.Pos, "")
		g.End = append(g.End, 0)
		g.index[name] = i
		return i
	}
	for _, n := range nodes {
		id(n)
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, [2]int32{id(e[0]), id(e[1])})
		g.Kinds = append(g.Kinds, kind)
	}
}

// Match returns node ids whose name exactly equals "go:"+sym, or, failing
// that, all nodes containing sym as a substring.
func (g *Graph) Match(sym string) []int32 {
	if i, ok := g.index["go:"+sym]; ok {
		return []int32{i}
	}
	var ids []int32
	for i, n := range g.Nodes {
		if strings.Contains(n, sym) {
			ids = append(ids, int32(i))
		}
	}
	return ids
}

func (g *Graph) Name(i int32) string { return g.Nodes[i] }

// Position returns the node's definition site as "file:line", or "" when
// unknown (http: endpoints, js: nodes — whose names embed the file — and
// position-less SSA functions). Guarded for caches written before Pos existed.
func (g *Graph) Position(i int32) string {
	if int(i) < len(g.Pos) {
		return g.Pos[i]
	}
	return ""
}

// Span returns the node's definition file and inclusive line range, with
// ok=false when the span is unknown.
func (g *Graph) Span(i int32) (file string, start, end int32, ok bool) {
	p := g.Position(i)
	if p == "" || int(i) >= len(g.End) || g.End[i] == 0 {
		return "", 0, 0, false
	}
	c := strings.LastIndex(p, ":")
	n, err := strconv.Atoi(p[c+1:])
	if c < 0 || err != nil {
		return "", 0, 0, false
	}
	return p[:c], int32(n), g.End[i], true
}

func (g *Graph) reindex() {
	g.index = make(map[string]int32, len(g.Nodes))
	for i, n := range g.Nodes {
		g.index[n] = int32(i)
	}
}
