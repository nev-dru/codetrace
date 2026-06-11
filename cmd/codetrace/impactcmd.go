package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nev-dru/codetrace/internal/cterr"
	"github.com/nev-dru/codetrace/internal/diffspan"
	"github.com/nev-dru/codetrace/internal/output"
)

// cmdImpact maps the current git diff onto graph functions and reports the
// blast radius: which endpoints, frontend callers, and module functions sit
// upstream of the changed code. VTA over-approximates interface dispatch, so
// the report errs toward "more to test", never less.
//
//	codetrace impact          — uncommitted changes (vs HEAD)
//	codetrace impact main     — everything since main, plus working tree
func cmdImpact(args []string) error {
	o, pos, err := parseFlags("impact", args, 0)
	if err != nil {
		return err
	}
	base := "HEAD"
	if len(pos) > 0 {
		base = pos[0]
	}
	top, err := gitToplevel()
	if err != nil {
		return err
	}
	diffCmd := exec.Command("git", "diff", "-U0", base)
	diffCmd.Dir = top
	diff, err := diffCmd.Output()
	if err != nil {
		return fmt.Errorf("git diff %s: %v", base, err)
	}
	spans := diffspan.Parse(string(diff))
	if len(spans) == 0 {
		fmt.Fprintln(os.Stderr, "hint: git diff is empty — nothing changed vs "+base)
		return cterr.ErrNoResults
	}

	g, err := loadStitched(o)
	if err != nil {
		return err
	}

	// Changed nodes: graph spans (absolute paths) containing a diff span
	// (repo-relative). Non-Go files have no Go node; note them so silence
	// isn't mistaken for "no impact".
	changed := map[int32]bool{}
	matchedFile := map[string]bool{}
	for i := range g.Nodes {
		file, start, end, ok := g.Span(int32(i))
		if !ok {
			continue
		}
		for _, s := range spans {
			if !strings.HasSuffix(s.File, ".go") {
				continue
			}
			if file == filepath.Join(top, s.File) && s.Start <= int(end) && int(start) <= s.End {
				changed[int32(i)] = true
				matchedFile[s.File] = true
			}
		}
	}
	skipped := map[string]bool{}
	for _, s := range spans {
		if !strings.HasSuffix(s.File, ".go") || !matchedFile[s.File] {
			skipped[s.File] = true
		}
	}
	for f := range matchedFile {
		delete(skipped, f)
	}
	if len(skipped) > 0 {
		fmt.Fprintf(os.Stderr, "note: %d changed file(s) not mapped to graph functions (non-Go, or edits outside function bodies)\n", len(skipped))
	}
	if len(changed) == 0 {
		fmt.Fprintln(os.Stderr, "hint: no changed lines fall inside Go functions known to the graph")
		return cterr.ErrNoResults
	}

	ids := make([]int32, 0, len(changed))
	for i := range changed {
		ids = append(ids, i)
	}
	mod := modulePath(o.modDir)
	var lines []string
	for _, i := range ids {
		n := g.Name(i)
		if p := g.Position(i); p != "" {
			n += "  " + rel(p)
		}
		lines = append(lines, "changed  "+n)
	}
	for _, i := range g.Reverse(ids) {
		if changed[i] {
			continue
		}
		n := g.Name(i)
		switch {
		case strings.HasPrefix(n, "http:"):
			lines = append(lines, "endpoint "+strings.TrimPrefix(n, "http:"))
		case strings.HasPrefix(n, "js:"):
			lines = append(lines, "js       "+strings.TrimPrefix(n, "js:"))
		case mod == "" || strings.Contains(n, mod):
			if p := g.Position(i); p != "" {
				n += "  " + rel(p)
			}
			lines = append(lines, "go       "+n)
		}
	}
	return output.Render(os.Stdout, stripModule(lines, mod), o.all, o.json)
}

func gitToplevel() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}
