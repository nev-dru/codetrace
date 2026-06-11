package graph

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CacheDir is .codetrace/ at the git toplevel (falls back to modDir).
func CacheDir(modDir string) string {
	out, err := exec.Command("git", "-C", modDir, "rev-parse", "--show-toplevel").Output()
	root := strings.TrimSpace(string(out))
	if err != nil || root == "" {
		root = modDir
	}
	return filepath.Join(root, ".codetrace")
}

// CachePath keys the cache file by the module directory: a repo can hold
// several Go modules (backend and this CLI itself), and a shared file would
// silently serve one module's graph for the other's queries.
func CachePath(modDir string) string {
	abs, err := filepath.Abs(modDir)
	if err != nil {
		abs = modDir
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(CacheDir(modDir), fmt.Sprintf("graph-%x.gob", sum[:6]))
}

func gitRev(modDir string) string {
	out, err := exec.Command("git", "-C", modDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// newerGoFiles reports whether any .go file under modDir is newer than t.
// vendor, node_modules, and dot-dirs are skipped.
func newerGoFiles(modDir string, t time.Time) bool {
	newer := false
	filepath.WalkDir(modDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || newer {
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() && (name == "vendor" || name == "node_modules" || (strings.HasPrefix(name, ".") && path != modDir)) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(name, ".go") {
			if info, err := d.Info(); err == nil && info.ModTime().After(t) {
				newer = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return newer
}

// Load returns the cached graph if fresh, else rebuilds (printing a notice to
// stderr) and refreshes the cache.
func Load(modDir string) (*Graph, error) {
	g, _, err := load(modDir)
	return g, err
}

func load(modDir string) (*Graph, bool, error) {
	rev := gitRev(modDir)
	path := CachePath(modDir)
	if info, err := os.Stat(path); err == nil {
		if g, err := read(path); err == nil && g.Version == SchemaVersion && g.Rev == rev && !newerGoFiles(modDir, info.ModTime()) {
			g.reindex()
			return g, true, nil
		}
		// Unreadable/corrupt/stale cache: fall through and rebuild.
	}
	fmt.Fprintln(os.Stderr, "rebuilding graph cache...")
	g, err := Build(modDir, rev)
	if err != nil {
		return nil, false, err
	}
	if err := write(path, g); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write cache: %v\n", err)
	}
	return g, false, nil
}

func read(path string) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var g Graph
	if err := gob.NewDecoder(f).Decode(&g); err != nil {
		return nil, err
	}
	return &g, nil
}

func write(path string, g *Graph) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "graph-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := gob.NewEncoder(f).Encode(g); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
