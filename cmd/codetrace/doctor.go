package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nev-dru/codetrace/internal/graph"
)

type tool struct {
	name     string
	required bool
	install  string
}

var tools = []tool{
	{"gopls", true, "go install golang.org/x/tools/gopls@latest"},
	{"ast-grep", false, "see https://ast-grep.github.io — needed only for JS refs/def and stitched queries"},
	{"ctags", false, "see https://ctags.io — optional symbol fallback"},
	{"git", true, "install git"},
	{"codebase-memory-mcp", false, "optional companion for non-Go/JS languages — install its binary to ~/.local/bin (codebase-memory skill is disabled without it)"},
}

func cmdDoctor(args []string) error {
	o, _, err := parseFlags("doctor", args, 0)
	if err != nil {
		return err
	}
	ok := true
	for _, t := range tools {
		if p, err := exec.LookPath(t.name); err == nil {
			fmt.Printf("ok       %-10s %s\n", t.name, p)
			continue
		}
		// go-installed binaries often live outside PATH
		home, _ := os.UserHomeDir()
		found := false
		for _, alt := range []string{
			filepath.Join(home, "go", "bin", t.name),
			filepath.Join(home, ".local", "bin", t.name),
		} {
			if _, err := os.Stat(alt); err == nil {
				fmt.Printf("ok       %-10s %s (not on PATH)\n", t.name, alt)
				found = true
				break
			}
		}
		if found {
			continue
		}
		status := "missing "
		if t.required {
			status = "MISSING "
			ok = false
		}
		fmt.Printf("%s %-10s install: %s\n", status, t.name, t.install)
	}

	fmt.Printf("moddir   %s\n", o.modDir)
	cache := graph.CachePath(o.modDir)
	if info, err := os.Stat(cache); err == nil {
		fmt.Printf("cache    %s (%d KB, built %s)\n", cache, info.Size()/1024,
			info.ModTime().Format("2006-01-02 15:04"))
	} else {
		fmt.Printf("cache    none yet — first reachable/reaches/cycles/dead call builds it (slow once)\n")
	}
	if g, err := graph.Load(o.modDir); err == nil && len(g.Unmapped) > 0 {
		fmt.Printf("unmapped %d route(s) lack a method-value handler (paths/endpoint blind to them):\n", len(g.Unmapped))
		for i, u := range g.Unmapped {
			if i == 5 {
				fmt.Printf("         … %d more\n", len(g.Unmapped)-5)
				break
			}
			fmt.Printf("         %s\n", u)
		}
	}
	if !ok {
		return fmt.Errorf("required tools missing")
	}
	fmt.Println("ready")
	return nil
}
