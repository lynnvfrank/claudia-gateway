package providerfreetier

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndMatch(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "p.yaml")
	raw := `format_version: 1
effective_date: "2026-01-01"
models:
  - groq/a
  - gemini/b
patterns:
  - "gemini/c*"
`
	if err := os.WriteFile(p, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Match("groq/a") || !s.Match("gemini/b") || !s.Match("gemini/c-foo") {
		t.Fatalf("match failed")
	}
	if s.Match("groq/b") || s.Match("ollama/x") {
		t.Fatal("unexpected match")
	}
	got := s.Filter([]string{"groq/a", "groq/a", "groq/z", "gemini/c-foo"})
	if len(got) != 2 || got[0] != "groq/a" || got[1] != "gemini/c-foo" {
		t.Fatalf("filter: %#v", got)
	}
}

func TestLoad_badVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "p.yaml")
	if err := os.WriteFile(p, []byte("format_version: 99\nmodels: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected error")
	}
}
