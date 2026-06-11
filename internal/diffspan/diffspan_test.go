package diffspan

import "testing"

const sample = `diff --git a/backend/internal/db/applicants.go b/backend/internal/db/applicants.go
index 111..222 100644
--- a/backend/internal/db/applicants.go
+++ b/backend/internal/db/applicants.go
@@ -185,2 +185,3 @@ func (c *Client) GetApplicantByMagicToken(
+	// changed
+	// lines
+	// here
@@ -300 +301,0 @@ func other(
-	removed := true
diff --git a/src/lib/api.js b/src/lib/api.js
index 333..444 100644
--- a/src/lib/api.js
+++ b/src/lib/api.js
@@ -10 +10 @@ export async function apiRequest(
+const API_BASE = '/api';
diff --git a/gone.go b/gone.go
deleted file mode 100644
--- a/gone.go
+++ /dev/null
`

func TestParse(t *testing.T) {
	spans := Parse(sample)
	want := []Span{
		{File: "backend/internal/db/applicants.go", Start: 185, End: 187},
		{File: "backend/internal/db/applicants.go", Start: 301, End: 301}, // pure deletion → seam line
		{File: "src/lib/api.js", Start: 10, End: 10},
	}
	if len(spans) != len(want) {
		t.Fatalf("got %d spans: %+v", len(spans), spans)
	}
	for i, w := range want {
		if spans[i] != w {
			t.Errorf("span %d: got %+v want %+v", i, spans[i], w)
		}
	}
}
