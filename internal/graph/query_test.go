package graph

import "testing"

func names(g *Graph, ids []int32) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, i := range ids {
		m[g.Name(i)] = true
	}
	return m
}

func TestForwardReachableFromMain(t *testing.T) {
	g := buildFixture(t)
	got := names(g, g.Forward(g.Roots))
	for _, want := range []string{"go:fixture.helperB", "go:(fixture.English).Greet"} {
		if !got[want] {
			t.Errorf("missing %s in forward set", want)
		}
	}
	// VTA precision: French is never instantiated, so French.Greet must NOT be reachable.
	if got["go:(fixture.French).Greet"] {
		t.Error("French.Greet reachable — VTA should have excluded it")
	}
	if got["go:fixture.unused"] {
		t.Error("unused reachable — should be dead")
	}
}

func TestReverse(t *testing.T) {
	g := buildFixture(t)
	got := names(g, g.Reverse([]int32{g.mustFind(t, "fixture.helperB")}))
	if !got["go:fixture.helperA"] || !got["go:fixture.main"] {
		t.Errorf("reverse(helperB) missing callers: %v", got)
	}
}

func TestSCCsFindRecursion(t *testing.T) {
	g := buildFixture(t)
	for _, scc := range g.SCCs() {
		n := names(g, scc)
		if n["go:fixture.recurseX"] && n["go:fixture.recurseY"] {
			return
		}
	}
	t.Error("recurseX/recurseY cycle not found")
}

func TestDead(t *testing.T) {
	g := buildFixture(t)
	dead := names(g, g.Dead())
	if !dead["go:fixture.unused"] {
		t.Error("unused not reported dead")
	}
	if dead["go:fixture.helperB"] {
		t.Error("helperB wrongly reported dead")
	}
}
