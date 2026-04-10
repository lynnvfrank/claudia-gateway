package routinggen

import (
	"testing"

	"github.com/lynn/claudia-gateway/internal/routing"
)

func TestExtractCatalogModelIDs(t *testing.T) {
	raw := []byte(`{"data":[{"id":"Claudia-0.1.0"},{"id":"groq/a"},{"id":"groq/b"},{"id":"groq/a"}]}`)
	ids, err := ExtractCatalogModelIDs(raw, "Claudia-0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "groq/a" || ids[1] != "groq/b" {
		t.Fatalf("%#v", ids)
	}
}

func TestOrderFallbackChain_ollamaLast(t *testing.T) {
	in := []string{"ollama/small", "groq/llama-3.3-70b-versatile", "groq/llama-3.1-8b-instant"}
	out := OrderFallbackChain(in)
	if out[0] != "groq/llama-3.3-70b-versatile" {
		t.Fatalf("want 70b first, got %v", out)
	}
	if out[len(out)-1] != "ollama/small" {
		t.Fatalf("want ollama last, got %v", out)
	}
}

func TestBuildRoutingPolicyYAML_validates(t *testing.T) {
	b, err := BuildRoutingPolicyYAML([]string{"gemini/x", "groq/y"})
	if err != nil {
		t.Fatal(err)
	}
	if err := routing.ValidatePolicyYAML(b); err != nil {
		t.Fatal(err)
	}
}
