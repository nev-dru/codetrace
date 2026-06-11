package graph

import (
	"strings"
	"testing"
)

func buildFixture(t *testing.T) *Graph {
	t.Helper()
	g, err := Build("testdata/fixture", "testrev")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func (g *Graph) mustFind(t *testing.T, substr string) int32 {
	t.Helper()
	ids := g.Match(substr)
	if len(ids) != 1 {
		t.Fatalf("Match(%q) = %d hits, want 1", substr, len(ids))
	}
	return ids[0]
}

func TestBuildProducesNodesAndRoots(t *testing.T) {
	g := buildFixture(t)
	if len(g.Nodes) == 0 || len(g.Edges) == 0 {
		t.Fatalf("empty graph: %d nodes %d edges", len(g.Nodes), len(g.Edges))
	}
	if len(g.Roots) == 0 {
		t.Fatal("no roots (main/init) found")
	}
	g.mustFind(t, "fixture.helperA") // node naming sanity
}

func TestSchemaVersionAndKinds(t *testing.T) {
	g := buildFixture(t)
	if g.Version != SchemaVersion {
		t.Errorf("Version = %d, want %d", g.Version, SchemaVersion)
	}
	if len(g.Kinds) != len(g.Edges) {
		t.Errorf("Kinds len %d != Edges len %d", len(g.Kinds), len(g.Edges))
	}
}

func TestExtend(t *testing.T) {
	g := buildFixture(t)
	nEdges := len(g.Edges)
	g.Extend(
		[]string{"js:src/a.jsx:send", "http:POST /api/chat"},
		[][2]string{{"js:src/a.jsx:send", "http:POST /api/chat"}},
		EdgeHTTP,
	)
	if len(g.Edges) != nEdges+1 || len(g.Kinds) != len(g.Edges) {
		t.Fatalf("edge not added consistently")
	}
	ids := g.Match("js:src/a.jsx:send")
	if len(ids) != 1 {
		t.Fatalf("extended node not matchable")
	}
	fwd := names(g, g.Forward(ids))
	if !fwd["http:POST /api/chat"] {
		t.Error("extended edge not traversable")
	}
}

func TestNodesAreNamespaced(t *testing.T) {
	g := buildFixture(t)
	for _, n := range g.Nodes {
		if !strings.HasPrefix(n, "go:") && !strings.HasPrefix(n, "http:") {
			t.Fatalf("node %q missing namespace prefix", n)
		}
	}
}
