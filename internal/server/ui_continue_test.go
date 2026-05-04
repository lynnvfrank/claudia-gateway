package server

import (
	"strings"
	"testing"

	"github.com/lynn/claudia-gateway/internal/indexer"
)

func TestIngestProjectForContinue_aliasWorkspace(t *testing.T) {
	got := ingestProjectForContinue("", "myws")
	if got != "myws" {
		t.Fatalf("got %q", got)
	}
	got = ingestProjectForContinue("explicit", "myws")
	if got != "explicit" {
		t.Fatalf("got %q", got)
	}
}

func TestIngestProject_matchesIndexerIngestProject(t *testing.T) {
	want := indexer.IngestProject(indexer.ScopeFragment{ProjectID: "p", WorkspaceID: "w"})
	if ingestProjectForContinue("p", "w") != want {
		t.Fatal("drift from indexer.IngestProject")
	}
}

func TestContinueConfigYAMLBytes_headersAndRoles(t *testing.T) {
	b, err := continueConfigYAMLBytes(
		"0.2.0",
		"Claudia-0.2.0",
		"http://127.0.0.1:3000/",
		"gateway-token",
		"proj-a",
		"w-ignored",
		"flav-x",
	)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, sub := range []string{
		"%YAML 1.1",
		"name: Claudia",
		`model: Claudia-0.2.0`,
		`apiKey: gateway-token`,
		`apiBase: http://127.0.0.1:3000/v1`,
		"X-Claudia-Project: proj-a",
		"X-Claudia-Flavor-Id: flav-x",
		"- chat",
		"- reasoning",
	} {
		if !strings.Contains(s, sub) {
			t.Fatalf("missing %q in:\n%s", sub, s)
		}
	}
}
