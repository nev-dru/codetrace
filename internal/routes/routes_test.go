package routes

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func loadFix(t *testing.T) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{Mode: packages.LoadAllSyntax, Dir: "testdata/routesfix"}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatal(err)
	}
	return pkgs
}

func TestExtractRoutes(t *testing.T) {
	bridges, unmapped := Extract(loadFix(t))
	want := map[string]string{
		"POST /api/chat":           "(*routesfix.ChatHandler).Handle",
		"GET /api/chat/{}/history": "(*routesfix.ChatHandler).History",
		"GET /ping":                "routesfix.ping",
	}
	got := map[string]string{}
	for _, b := range bridges {
		got[b.Endpoint] = b.HandlerFullName
	}
	for ep, h := range want {
		if got[ep] != h {
			t.Errorf("endpoint %q → %q, want %q", ep, got[ep], h)
		}
	}
	// /organizations must NOT be bridged — its prefix is unknown.
	if _, bad := got["GET /organizations"]; bad {
		t.Error("router-parameter registration was bridged with a guessed prefix")
	}
	// unmapped: the /health func literal + the router-param registration
	if len(unmapped) != 2 {
		t.Errorf("unmapped = %d, want 2: %v", len(unmapped), unmapped)
	}
	foundParamReason := false
	for _, u := range unmapped {
		if strings.Contains(u, "mount prefix unknown") {
			foundParamReason = true
		}
	}
	if !foundParamReason {
		t.Errorf("router-param registration lacks the prefix-unknown reason: %v", unmapped)
	}
}
