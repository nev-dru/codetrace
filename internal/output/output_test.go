package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestRenderDedupesAndSorts(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []string{"b.go:2  f", "a.go:1  g", "b.go:2  f"}, false, false); err != nil {
		t.Fatal(err)
	}
	want := "a.go:1  g\nb.go:2  f\n"
	if b.String() != want {
		t.Errorf("got %q want %q", b.String(), want)
	}
}

func TestRenderTruncatesAt200(t *testing.T) {
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, fmt.Sprintf("f.go:%03d  x", i))
	}
	var b bytes.Buffer
	if err := Render(&b, lines, false, false); err != nil {
		t.Fatal(err)
	}
	out := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if len(out) != 201 {
		t.Fatalf("got %d lines, want 201", len(out))
	}
	if out[200] != "... 50 more (use --all)" {
		t.Errorf("tail = %q", out[200])
	}
}

func TestRenderAllDisablesTruncation(t *testing.T) {
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, fmt.Sprintf("f.go:%03d  x", i))
	}
	var b bytes.Buffer
	if err := Render(&b, lines, true, false); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(b.String(), "\n"); n != 250 {
		t.Errorf("got %d lines, want 250", n)
	}
}

func TestRenderJSONTruncates(t *testing.T) {
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, fmt.Sprintf("f.go:%03d  x", i))
	}
	var b bytes.Buffer
	if err := Render(&b, lines, false, true); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Results   []string `json:"results"`
		Truncated int      `json:"truncated"`
	}
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 200 || got.Truncated != 50 {
		t.Errorf("got %d results, truncated=%d; want 200/50", len(got.Results), got.Truncated)
	}
}

func TestRenderJSON(t *testing.T) {
	var b bytes.Buffer
	if err := Render(&b, []string{"z", "a", "z"}, false, true); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Results []string `json:"results"`
	}
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 2 || got.Results[0] != "a" {
		t.Errorf("got %v", got.Results)
	}
}
