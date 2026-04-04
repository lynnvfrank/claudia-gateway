package config

import "testing"

func TestPatchResolvedUpstream(t *testing.T) {
	r := &Resolved{
		LitellmBaseURL:   "http://bifrost:8080",
		HealthLitellmURL: "http://bifrost:8080/health",
	}
	PatchResolvedUpstream(r, "http://127.0.0.1:9090")
	if r.LitellmBaseURL != "http://127.0.0.1:9090" || r.HealthLitellmURL != "http://127.0.0.1:9090/health" {
		t.Fatalf("%+v", r)
	}
}

func TestCloneResolved_Slice(t *testing.T) {
	a := &Resolved{FallbackChain: []string{"x", "y"}}
	b := CloneResolved(a)
	b.FallbackChain[0] = "z"
	if a.FallbackChain[0] != "x" {
		t.Fatal("aliased slice")
	}
}
