package config

import "testing"

func TestPatchResolvedUpstream(t *testing.T) {
	r := &Resolved{
		UpstreamBaseURL:   "http://bifrost:8080",
		HealthUpstreamURL: "http://bifrost:8080/health",
	}
	PatchResolvedUpstream(r, "http://127.0.0.1:9090")
	if r.UpstreamBaseURL != "http://127.0.0.1:9090" || r.HealthUpstreamURL != "http://127.0.0.1:9090/health" {
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
