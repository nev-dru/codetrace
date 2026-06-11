// codetrace answers code-navigation questions for LLM agents in one call:
// who calls X, what does X reach, is X dead, where is this JS symbol used.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nev-dru/codetrace/internal/cterr"
)

const usage = `codetrace — code tracing for LLM agents

Usage: codetrace <command> [flags] [args]

Commands:
  callers <sym>     who calls this Go function (precise, gopls)
  callees <sym>     what this Go function calls (precise, gopls)
  refs <sym>        all references (Go via gopls; JS via ast-grep)
  def <sym>         jump to definition (--body also prints the source, Go only)
  reachable <sym>   everything transitively callable from sym (whole-program VTA)
  reaches <sym>     everything that can transitively call sym (whole-program VTA)
  cycles            recursion cycles / SCCs in module code
  dead              functions unreachable from main
  paths <from> <to>          all edges on paths between two nodes (crosses JS/HTTP/Go)
  endpoint <METHOD> <path>   JS callers + Go handler + what it reaches
  impact [base-ref]          blast radius of the git diff (default: vs HEAD)
  doctor            check required tools and cache health

Flags (per command):
  -C <dir>   Go module directory (default: auto-detect ./go.mod, then backend/go.mod)
  --json     JSON output
  --all      disable 200-line truncation
  --js       force the JS engine (ast-grep) for refs/def

Symbols: bare name (resolveTeamMember), qualified (handler.resolveTeamMember),
or position (path/file.go:73:6). Ambiguous names exit 2 with a candidate list.

Exit codes: 0 results, 1 no results, 2 ambiguous, 3 error.
`

type opts struct {
	modDir string
	json   bool
	all    bool
	js        bool
	body      bool
	mermaid   bool
	generated bool
	tests     bool
}

func parseFlags(name string, args []string, nPos int) (opts, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var o opts
	fs.StringVar(&o.modDir, "C", "", "Go module directory")
	fs.BoolVar(&o.json, "json", false, "JSON output")
	fs.BoolVar(&o.all, "all", false, "disable truncation")
	fs.BoolVar(&o.js, "js", false, "force JS engine")
	fs.BoolVar(&o.body, "body", false, "def: also print the function source")
	fs.BoolVar(&o.mermaid, "mermaid", false, "paths: emit a Mermaid flowchart instead of edge lines")
	fs.BoolVar(&o.generated, "generated", false, "include results in generated files (gomock etc.)")
	fs.BoolVar(&o.tests, "tests", false, "include results in _test.go files")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return o, nil, err
	}
	if o.modDir == "" {
		o.modDir = detectModDir()
	}
	pos := fs.Args()
	if len(pos) < nPos {
		return o, nil, fmt.Errorf("usage: codetrace %s [flags] — expects %d argument(s), got %d (see codetrace help)", name, nPos, len(pos))
	}
	return o, pos, nil
}

// reorderFlags moves flag tokens ahead of positionals so trailing flags work
// ("codetrace refs foo --all"): Go's flag package stops at the first
// positional and would silently drop them. -C consumes the following token as
// its value unless given as -C=dir.
func reorderFlags(args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			pos = append(pos, a)
			continue
		}
		flags = append(flags, a)
		if (a == "-C" || a == "--C") && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, pos...)
}

func detectModDir() string {
	if _, err := os.Stat("go.mod"); err == nil {
		return "."
	}
	if _, err := os.Stat(filepath.Join("backend", "go.mod")); err == nil {
		return "backend"
	}
	return "."
}

// rel makes a path relative to the cwd for compact output.
func rel(p string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return p
	}
	if r, err := filepath.Rel(cwd, p); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return p
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(3)
	}
	var err error
	switch os.Args[1] {
	case "callers":
		err = cmdHierarchy("callers", os.Args[2:], "caller")
	case "callees":
		err = cmdHierarchy("callees", os.Args[2:], "callee")
	case "refs":
		err = cmdRefs(os.Args[2:])
	case "def":
		err = cmdDef(os.Args[2:])
	case "reachable":
		err = cmdGraph("reachable", os.Args[2:])
	case "reaches":
		err = cmdGraph("reaches", os.Args[2:])
	case "paths":
		err = cmdPaths(os.Args[2:])
	case "endpoint":
		err = cmdEndpoint(os.Args[2:])
	case "impact":
		err = cmdImpact(os.Args[2:])
	case "cycles":
		err = cmdCycles(os.Args[2:])
	case "dead":
		err = cmdDead(os.Args[2:])
	case "doctor":
		err = cmdDoctor(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n%s", os.Args[1], usage)
		os.Exit(3)
	}

	var amb *cterr.Ambiguous
	switch {
	case err == nil:
	case errors.Is(err, cterr.ErrNoResults):
		fmt.Fprintln(os.Stderr, "no results")
		os.Exit(1)
	case errors.As(err, &amb):
		fmt.Fprintln(os.Stderr, amb.Error())
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(3)
	}
}
