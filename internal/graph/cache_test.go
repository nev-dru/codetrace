package graph

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// copyFixture copies the fixture module into a temp dir so tests can touch files.
func copyFixture(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	for _, f := range []string{"go.mod", "main.go"} {
		b, err := os.ReadFile(filepath.Join("testdata/fixture", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func TestLoadBuildsThenHitsCache(t *testing.T) {
	dir := copyFixture(t)
	_, hit, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Fatal("first load reported cache hit")
	}
	_, hit, err = load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Fatal("second load missed cache")
	}
}

func TestLoadRebuildsWhenGoFileTouched(t *testing.T) {
	dir := copyFixture(t)
	if _, _, err := load(dir); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(dir, "main.go"), future, future); err != nil {
		t.Fatal(err)
	}
	_, hit, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Fatal("load hit stale cache after .go file changed")
	}
}
