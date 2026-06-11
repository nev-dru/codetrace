// Package routes extracts HTTP-route → handler bridges by walking chi-style
// registrations in the typed AST. It matches registration SHAPE (verb method
// + string-literal path + handler arg) rather than chi's types, and resolves
// handlers through types.Func.FullName(), which is identical to
// ssa.Function.String() — so bridge edges join the call graph exactly.
package routes

import (
	"fmt"
	"go/ast"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/packages"

	"github.com/nev-dru/codetrace/internal/normurl"
)

var verbs = map[string]string{
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE",
	"Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS",
}

type Bridge struct {
	Endpoint        string // normalized "METHOD /path"
	HandlerFullName string // types.Func.FullName(), e.g. (*pkg.ChatHandler).Handle
}

// Extract walks every syntax tree in pkgs. unmapped lists registrations the
// walker could not bridge soundly: handlers that aren't resolvable function
// values, and registrations on router PARAMETERS — those are mounted by a
// caller under a prefix this walker cannot see, so emitting a bridge would
// produce a confidently-wrong endpoint (worse than no edge).
func Extract(pkgs []*packages.Package) (bridges []Bridge, unmapped []string) {
	seen := map[string]bool{}
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.TypesInfo == nil || seen[p.ID] {
			return
		}
		seen[p.ID] = true
		for _, f := range p.Syntax {
			for _, decl := range f.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
					walk(p, fd.Body, "", paramNames(fd), &bridges, &unmapped)
				}
			}
		}
	})
	return bridges, unmapped
}

// paramNames collects the function's parameter identifiers — routers received
// as parameters have an unknown mount prefix.
func paramNames(fd *ast.FuncDecl) map[string]bool {
	ps := map[string]bool{}
	if fd.Type.Params == nil {
		return ps
	}
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			ps[n.Name] = true
		}
	}
	return ps
}

func walk(p *packages.Package, n ast.Node, prefix string, untrusted map[string]bool, bridges *[]Bridge, unmapped *[]string) {
	ast.Inspect(n, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := sel.Sel.Name

		switch {
		case name == "Route" && len(call.Args) == 2:
			path, pathOK := strLit(call.Args[0])
			fl, flOK := call.Args[1].(*ast.FuncLit)
			if pathOK && flOK {
				// The closure's router param is trusted: registrations on it
				// live under the prefix we just extended.
				walk(p, fl.Body, prefix+path, trustClosureParams(untrusted, fl), bridges, unmapped)
				return false // subtree handled with the extended prefix
			}
			pos := p.Fset.Position(call.Pos())
			*unmapped = append(*unmapped,
				fmt.Sprintf("Route at %s:%d (non-literal path or non-inline subtree — prefix unknown)", pos.Filename, pos.Line))
			return false // never visit the subtree with a dropped prefix
		case name == "Group" && len(call.Args) == 1:
			if fl, ok := call.Args[0].(*ast.FuncLit); ok {
				walk(p, fl.Body, prefix, trustClosureParams(untrusted, fl), bridges, unmapped)
				return false
			}
		default:
			if method, isVerb := verbs[name]; isVerb && len(call.Args) == 2 {
				path, ok := strLit(call.Args[0])
				if !ok {
					return true
				}
				ep, ok := normurl.TryNormalize(method, prefix+path)
				if !ok {
					return true
				}
				pos := p.Fset.Position(call.Pos())
				if id := rootIdent(sel.X); id != nil && untrusted[id.Name] {
					*unmapped = append(*unmapped,
						fmt.Sprintf("%s at %s:%d (registered on router parameter %q — mount prefix unknown)", ep, pos.Filename, pos.Line, id.Name))
					return true
				}
				if fn := handlerFunc(p, call.Args[1]); fn != nil {
					*bridges = append(*bridges, Bridge{Endpoint: ep, HandlerFullName: fn.FullName()})
				} else {
					*unmapped = append(*unmapped,
						fmt.Sprintf("%s (handler not a resolvable function value) at %s:%d", ep, pos.Filename, pos.Line))
				}
			}
		}
		return true
	})
}

// trustClosureParams returns untrusted minus the closure's own parameters —
// a Route/Group closure's router param carries the prefix we are tracking.
func trustClosureParams(untrusted map[string]bool, fl *ast.FuncLit) map[string]bool {
	if fl.Type.Params == nil || len(fl.Type.Params.List) == 0 {
		return untrusted
	}
	out := make(map[string]bool, len(untrusted))
	for k, v := range untrusted {
		out[k] = v
	}
	for _, f := range fl.Type.Params.List {
		for _, n := range f.Names {
			delete(out, n.Name)
		}
	}
	return out
}

// rootIdent unwraps an expression like r, s.r, or r.With(...) to its base
// identifier; nil when there is none.
func rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.SelectorExpr:
			e = x.X
		case *ast.CallExpr:
			e = x.Fun
		case *ast.ParenExpr:
			e = x.X
		default:
			return nil
		}
	}
}

func strLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// handlerFunc resolves method values (chatHandler.Handle) and plain function
// references (handleX) to their *types.Func. Returns nil for func literals
// and wrapped handlers.
func handlerFunc(p *packages.Package, e ast.Expr) *types.Func {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		if s, ok := p.TypesInfo.Selections[x]; ok {
			if fn, ok := s.Obj().(*types.Func); ok {
				return fn
			}
		}
		if obj, ok := p.TypesInfo.Uses[x.Sel].(*types.Func); ok {
			return obj
		}
	case *ast.Ident:
		if obj, ok := p.TypesInfo.Uses[x].(*types.Func); ok {
			return obj
		}
	}
	return nil
}
