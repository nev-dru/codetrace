// Package gopls shells out to the gopls CLI and parses its plain-text output.
package gopls

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Bin locates gopls: PATH first, then ~/go/bin (where go install puts it on
// machines that never added GOBIN to PATH — doctor reports it "ok" there, so
// commands must find it there too).
func Bin() string {
	if p, err := exec.LookPath("gopls"); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		alt := filepath.Join(home, "go", "bin", "gopls")
		if _, err := os.Stat(alt); err == nil {
			return alt
		}
	}
	return "gopls" // let exec fail with the standard not-found error
}

// Run executes a gopls subcommand in dir. First call on a cold cache can take
// seconds while gopls indexes; callers surface that in doctor, not here.
func Run(dir string, args ...string) (string, error) {
	// Args are argv (no shell), the binary is the developer's own gopls, and
	// inputs are local CLI arguments — same trust model as invoking git.
	cmd := exec.Command(Bin(), args...) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gopls %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

type Hier struct {
	Direction string // "caller" or "callee"
	Func      string
	CallFile  string
	CallLine  int
}

var hierRe = regexp.MustCompile(
	`^(caller|callee)\[\d+\]: ranges (\d+):\d+(?:-\d+)?(?:, *\d+:\d+(?:-\d+)?)* in (\S+) from/to function (\S+) in \S+:\d+:`)

func ParseCallHierarchy(out string) []Hier {
	var res []Hier
	for _, line := range strings.Split(out, "\n") {
		m := hierRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		res = append(res, Hier{Direction: m[1], Func: m[4], CallFile: m[3], CallLine: n})
	}
	return res
}

type Loc struct {
	File string
	Line int
}

var locRe = regexp.MustCompile(`^(\S+):(\d+):\d+(?:-\d+)?$`)

func ParseLocations(out string) []Loc {
	var res []Loc
	for _, line := range strings.Split(out, "\n") {
		if m := locRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			n, _ := strconv.Atoi(m[2])
			res = append(res, Loc{m[1], n})
		}
	}
	return res
}

type Sym struct {
	File string
	Line int
	Col  int
	Name string
	Kind string
}

var symRe = regexp.MustCompile(`^(\S+):(\d+):(\d+)-\d+ (\S+) (\w+)$`)

func ParseSymbols(out string) []Sym {
	var res []Sym
	for _, line := range strings.Split(out, "\n") {
		m := symRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		ln, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		res = append(res, Sym{File: m[1], Line: ln, Col: col, Name: m[4], Kind: m[5]})
	}
	return res
}
