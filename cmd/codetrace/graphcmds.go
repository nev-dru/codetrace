package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nev-dru/codetrace/internal/cterr"
	"github.com/nev-dru/codetrace/internal/graph"
	"github.com/nev-dru/codetrace/internal/output"
	"github.com/nev-dru/codetrace/internal/srcfile"
)

// modulePath reads the module line from modDir/go.mod; used to filter output
// to this repo's code (dependencies stay in the graph but out of results).
func modulePath(modDir string) string {
	b, err := os.ReadFile(filepath.Join(modDir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(l, "module "))
		}
	}
	return ""
}

// stripModule removes the module path from display lines — it repeats on
// every go: node and carries no information. Stripped names remain valid
// query inputs (Match falls back to substring containment).
func stripModule(lines []string, mod string) []string {
	if mod == "" {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		l = strings.ReplaceAll(l, mod+"/", "")
		out[i] = strings.ReplaceAll(l, mod+".", "")
	}
	return out
}

// matchOrAmbiguous resolves sym against graph nodes. Multiple matches are
// fine up to a small bound (same-named helpers); past it the agent must
// disambiguate.
func matchOrAmbiguous(g *graph.Graph, sym string) ([]int32, error) {
	ids := g.Match(sym)
	switch {
	case len(ids) == 0:
		return nil, cterr.ErrNoResults
	case len(ids) > 5:
		// Substring matching on a vague symbol can hit thousands of nodes;
		// cap the dump so the ambiguity error itself honors the bounded-
		// output contract.
		const maxCands = 20
		shown := ids
		if len(shown) > maxCands {
			shown = shown[:maxCands]
		}
		cands := make([]string, 0, len(shown)+1)
		for _, i := range shown {
			cands = append(cands, g.Name(i))
		}
		if len(ids) > maxCands {
			cands = append(cands, fmt.Sprintf("… %d more — refine the symbol", len(ids)-maxCands))
		}
		return nil, &cterr.Ambiguous{Candidates: cands}
	}
	return ids, nil
}

// moduleLines filters ids to this module's code. With annotate, each line
// carries the definition's file:line so the agent needs no follow-up def
// call (cycles passes false — positions would bloat the <-> chains).
func moduleLines(g *graph.Graph, ids []int32, mod string, annotate bool) []string {
	var lines []string
	for _, i := range ids {
		n := g.Name(i)
		// The module filter only applies to go: nodes — http:/js: nodes are
		// boundary nodes of this repo by construction, never dependencies.
		if mod == "" || !strings.HasPrefix(n, "go:") || strings.Contains(n, mod) {
			if p := g.Position(i); annotate && p != "" {
				n += "  " + rel(p)
			}
			lines = append(lines, n)
		}
	}
	return lines
}

func cmdGraph(name string, args []string) error {
	o, pos, err := parseFlags(name, args, 1)
	if err != nil {
		return err
	}
	g, err := graph.Load(o.modDir)
	if err != nil {
		return err
	}
	ids, err := matchOrAmbiguous(g, pos[0])
	if err != nil {
		return err
	}
	var result []int32
	if name == "reachable" {
		result = g.Forward(ids)
	} else {
		result = g.Reverse(ids)
	}
	// Generated-file nodes (gomock) are graph noise for reachability
	// questions; tests are never in the graph (program loads without them).
	if !o.generated {
		genCache := map[string]bool{}
		hidden := 0
		kept := result[:0]
		for _, i := range result {
			if file, _, _, ok := g.Span(i); ok && srcfile.IsGenerated(file, genCache) {
				hidden++
				continue
			}
			kept = append(kept, i)
		}
		result = kept
		if hidden > 0 {
			fmt.Fprintf(os.Stderr, "note: %d generated-file result(s) hidden (--generated to show)\n", hidden)
		}
	}
	mod := modulePath(o.modDir)
	lines := stripModule(moduleLines(g, result, mod, true), mod)
	if len(lines) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, lines, o.all, o.json)
}

func cmdCycles(args []string) error {
	o, pos, err := parseFlags("cycles", args, 0)
	if err != nil {
		return err
	}
	g, err := graph.Load(o.modDir)
	if err != nil {
		return err
	}
	mod := modulePath(o.modDir)
	pkgFilter := ""
	if len(pos) > 0 {
		pkgFilter = pos[0]
	}
	// Cap members shown per cycle: whole-program graphs always contain one
	// giant interface-dispatch SCC (thousands of nodes) that would otherwise
	// print as a single enormous line.
	const maxMembers = 8
	var lines []string
	for _, scc := range g.SCCs() {
		names := stripModule(moduleLines(g, scc, mod, false), mod)
		if len(names) == 0 {
			continue
		}
		if pkgFilter != "" && !anyContains(names, pkgFilter) {
			continue
		}
		shown := names
		if !o.all && len(shown) > maxMembers {
			shown = append(shown[:maxMembers:maxMembers],
				fmt.Sprintf("… +%d more", len(names)-maxMembers))
		}
		lines = append(lines, fmt.Sprintf("[%d] %s", len(scc), strings.Join(shown, " <-> ")))
	}
	if len(lines) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, lines, o.all, o.json)
}

func anyContains(names []string, sub string) bool {
	for _, n := range names {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

func cmdDead(args []string) error {
	o, _, err := parseFlags("dead", args, 0)
	if err != nil {
		return err
	}
	g, err := graph.Load(o.modDir)
	if err != nil {
		return err
	}
	if len(g.Roots) == 0 {
		fmt.Fprintln(os.Stderr, "warning: no main package found — every function will appear dead")
	}
	// http: endpoint nodes have no callers by construction (requests come
	// from outside the binary) — they are not dead code.
	// Generated files (gomock etc.) are test scaffolding: dead-in-the-binary
	// by definition, so reporting them buries the real findings.
	genCache := map[string]bool{}
	hidden := 0
	var goOnly []int32
	for _, i := range g.Dead() {
		if !strings.HasPrefix(g.Name(i), "go:") {
			continue
		}
		if !o.generated {
			if file, _, _, ok := g.Span(i); ok && srcfile.IsGenerated(file, genCache) {
				hidden++
				continue
			}
		}
		goOnly = append(goOnly, i)
	}
	if hidden > 0 {
		fmt.Fprintf(os.Stderr, "note: %d function(s) in generated files hidden (--generated to show)\n", hidden)
	}
	mod := modulePath(o.modDir)
	lines := stripModule(moduleLines(g, goOnly, mod, true), mod)
	if len(lines) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, lines, o.all, o.json)
}
