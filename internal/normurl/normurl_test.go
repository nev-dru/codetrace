package normurl

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct{ method, raw, want string }{
		// frontend template literals
		{"get", "`/api/applicants/${applicantId}`", "GET /api/applicants/{}"},
		{"GET", "'/api/credentials'", "GET /api/credentials"},
		{"post", `"/api/chat"`, "POST /api/chat"},
		// query strings stripped
		{"GET", "`/api/applicants/lookup?token=${encodeURIComponent(t)}`", "GET /api/applicants/lookup"},
		// chi route templates
		{"POST", "/api/schemas/{schemaId}/copilot", "POST /api/schemas/{}/copilot"},
		{"GET", "/api/forms/:id", "GET /api/forms/{}"},
		// segment with embedded template mid-segment
		{"DELETE", "`/api/labels/${id}/assign`", "DELETE /api/labels/{}/assign"},
		// trailing slash collapses
		{"GET", "/api/things/", "GET /api/things"},
	}
	for _, c := range cases {
		if got := Normalize(c.method, c.raw); got != c.want {
			t.Errorf("Normalize(%q,%q) = %q, want %q", c.method, c.raw, got, c.want)
		}
	}
}

func TestNormalizeDynamicUnresolvable(t *testing.T) {
	// whole path is a variable — not a literal we can normalize
	if got, ok := TryNormalize("GET", "url"); ok {
		t.Errorf("expected !ok for bare identifier, got %q", got)
	}
	if _, ok := TryNormalize("GET", "`${base}/x`"); ok {
		t.Error("path starting with a template var is unresolvable")
	}
}
