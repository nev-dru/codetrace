package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nev-dru/codetrace/internal/cterr"
	"github.com/nev-dru/codetrace/internal/gopls"
	"github.com/nev-dru/codetrace/internal/jsgrep"
	"github.com/nev-dru/codetrace/internal/output"
	"github.com/nev-dru/codetrace/internal/srcfile"
	"github.com/nev-dru/codetrace/internal/symbol"
)

// resolveRef turns a symbol argument into a definition position, resolving
// bare/qualified names through gopls. Position args are absolutized against
// the cwd — gopls runs with its working directory set to modDir, so a
// cwd-relative path like backend/internal/x.go would otherwise double up.
func resolveRef(o opts, sym string) (symbol.Ref, error) {
	if ref, ok := symbol.ParsePos(sym); ok {
		if !filepath.IsAbs(ref.File) {
			if abs, err := filepath.Abs(ref.File); err == nil {
				ref.File = abs
			}
		}
		return ref, nil
	}
	return symbol.ResolveGo(o.modDir, sym)
}

// hideScaffolding drops lines whose source file is a test or generated file
// unless the matching flag includes them. If filtering would hide everything,
// the original lines are shown with a note — empty output must never imply
// "no references" when test-only references exist.
func hideScaffolding(lines, files []string, o opts) []string {
	if o.tests && o.generated {
		return lines
	}
	genCache := map[string]bool{}
	var kept []string
	hiddenT, hiddenG := 0, 0
	for i, l := range lines {
		switch {
		case !o.tests && srcfile.IsTest(files[i]):
			hiddenT++
		case !o.generated && srcfile.IsGenerated(files[i], genCache):
			hiddenG++
		default:
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 && len(lines) > 0 {
		fmt.Fprintln(os.Stderr, "note: every result is in test/generated files — showing them")
		return lines
	}
	if hiddenT > 0 {
		fmt.Fprintf(os.Stderr, "note: %d test-file result(s) hidden (--tests to show)\n", hiddenT)
	}
	if hiddenG > 0 {
		fmt.Fprintf(os.Stderr, "note: %d generated-file result(s) hidden (--generated to show)\n", hiddenG)
	}
	return kept
}

func cmdHierarchy(name string, args []string, direction string) error {
	o, pos, err := parseFlags(name, args, 1)
	if err != nil {
		return err
	}
	ref, err := resolveRef(o, pos[0])
	if err != nil {
		return err
	}
	out, err := gopls.Run(o.modDir, "call_hierarchy",
		fmt.Sprintf("%s:%d:%d", ref.File, ref.Line, ref.Col))
	if err != nil {
		return err
	}
	var lines, files []string
	for _, h := range gopls.ParseCallHierarchy(out) {
		if h.Direction == direction {
			lines = append(lines, fmt.Sprintf("%s:%d  %s", rel(h.CallFile), h.CallLine, h.Func))
			files = append(files, h.CallFile)
		}
	}
	lines = hideScaffolding(lines, files, o)
	if len(lines) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, lines, o.all, o.json)
}

func cmdRefs(args []string) error {
	o, pos, err := parseFlags("refs", args, 1)
	if err != nil {
		return err
	}
	sym := pos[0]
	if o.js {
		return jsRefs(o, sym)
	}
	ref, rerr := resolveRef(o, sym)
	if errors.Is(rerr, cterr.ErrNoResults) {
		return jsRefs(o, sym) // not a Go symbol — fall through to the JS engine
	}
	if rerr != nil {
		return rerr
	}
	if !strings.HasSuffix(ref.File, ".go") {
		// A non-.go position can't be handed to ast-grep — it takes
		// patterns, not coordinates.
		return fmt.Errorf("the JS engine takes names, not positions (got %s)", sym)
	}
	out, err := gopls.Run(o.modDir, "references",
		fmt.Sprintf("%s:%d:%d", ref.File, ref.Line, ref.Col))
	if err != nil {
		return err
	}
	var lines, files []string
	for _, l := range gopls.ParseLocations(out) {
		lines = append(lines, fmt.Sprintf("%s:%d", rel(l.File), l.Line))
		files = append(files, l.File)
	}
	lines = hideScaffolding(lines, files, o)
	if len(lines) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, lines, o.all, o.json)
}

func cmdDef(args []string) error {
	o, pos, err := parseFlags("def", args, 1)
	if err != nil {
		return err
	}
	sym := pos[0]
	if o.js {
		return jsDef(o, sym)
	}
	if ref, ok := symbol.ParsePos(sym); ok {
		// A position is a USE site — ask gopls to jump from it to the
		// definition rather than echoing the input back.
		if !strings.HasSuffix(ref.File, ".go") {
			return fmt.Errorf("the JS engine takes names, not positions (got %s)", sym)
		}
		ref, err := resolveRef(o, sym) // absolutizes the path
		if err != nil {
			return err
		}
		out, err := gopls.Run(o.modDir, "definition",
			fmt.Sprintf("%s:%d:%d", ref.File, ref.Line, ref.Col))
		if err != nil {
			return err
		}
		line := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
		if line == "" {
			return cterr.ErrNoResults
		}
		if o.body {
			// gopls prints "file.go:line:col-endcol: defined here as …" —
			// extract the position prefix.
			if m := defLocRe.FindStringSubmatch(line); m != nil {
				l, _ := strconv.Atoi(m[2])
				c, _ := strconv.Atoi(m[3])
				return printDef(symbol.Ref{File: m[1], Line: l, Col: c}, o)
			}
		}
		return output.Render(os.Stdout, []string{line}, o.all, o.json)
	}
	ref, rerr := resolveRef(o, sym)
	if errors.Is(rerr, cterr.ErrNoResults) {
		return jsDef(o, sym)
	}
	if rerr != nil {
		return rerr
	}
	// For name lookups resolveRef already found the definition position.
	return printDef(ref, o)
}

var defLocRe = regexp.MustCompile(`^(.+\.go):(\d+):(\d+)`)

// printDef emits the definition position, plus the source of the declaration
// when --body is set. Returning the body in the same call merges the
// locate-then-read round-trip the agent would otherwise make.
func printDef(ref symbol.Ref, o opts) error {
	loc := fmt.Sprintf("%s:%d:%d", rel(ref.File), ref.Line, ref.Col)
	if !o.body {
		return output.Render(os.Stdout, []string{loc}, o.all, o.json)
	}
	if !strings.HasSuffix(ref.File, ".go") {
		return fmt.Errorf("--body is Go-only (got %s)", ref.File)
	}
	body, err := goBody(ref.File, ref.Line)
	if err != nil {
		return err
	}
	// Raw print, not output.Render — sorting/deduping would scramble source.
	fmt.Println(loc)
	for _, l := range body {
		fmt.Println(l)
	}
	return nil
}

// goBody returns the source lines of the declaration starting at line,
// tracking brace depth until it closes. Capped so a misparse can't dump a
// whole file.
func goBody(file string, line int) ([]string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	all := strings.Split(string(b), "\n")
	if line < 1 || line > len(all) {
		return nil, fmt.Errorf("line %d out of range for %s", line, file)
	}
	const maxLines = 150
	depth, opened := 0, false
	var out []string
	for i := line - 1; i < len(all); i++ {
		l := all[i]
		out = append(out, l)
		for _, r := range l {
			switch r {
			case '{':
				depth++
				opened = true
			case '}':
				depth--
			}
		}
		if opened && depth <= 0 {
			return out, nil
		}
		if len(out) >= maxLines {
			return append(out, fmt.Sprintf("… truncated at %d lines", maxLines)), nil
		}
		// Declarations with no braces on the first line (e.g. var x = y)
		// end immediately.
		if !opened && i == line-1 && !strings.Contains(l, "{") {
			return out, nil
		}
	}
	return out, nil
}

func jsLines(locs []jsgrep.Loc) []string {
	lines := make([]string, len(locs))
	for i, l := range locs {
		lines[i] = fmt.Sprintf("%s:%d", l.File, l.Line)
	}
	return lines
}

func jsRefs(o opts, sym string) error {
	locs, err := jsgrep.Refs(jsgrep.Root(), sym)
	if err != nil {
		return err
	}
	if len(locs) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, jsLines(locs), o.all, o.json)
}

func jsDef(o opts, sym string) error {
	locs, err := jsgrep.Defs(jsgrep.Root(), sym)
	if err != nil {
		return err
	}
	if len(locs) == 0 {
		return cterr.ErrNoResults
	}
	return output.Render(os.Stdout, jsLines(locs), o.all, o.json)
}
