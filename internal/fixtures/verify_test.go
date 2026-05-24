package fixtures

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditNonRootNamespaceCountCountsDistinctNamespaceIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	content := `{"request":{"namespace":{"id":"root"},"path":"sys/mounts"}}
{"request":{"namespace":{"id":"ns-team-a"},"path":"secret/data/apps/team-a/scenario"}}
{"request":{"namespace":{"id":"ns-team-a"},"path":"secret/data/apps/team-a/scenario"}}
{"request":{"namespace":{"id":"ns-team-a-payments"},"path":"secret/data/apps/payments/scenario"}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write audit fixture: %v", err)
	}

	count, err := auditNonRootNamespaceCount(path)
	if err != nil {
		t.Fatalf("auditNonRootNamespaceCount returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("non-root namespace count = %d, want 2", count)
	}
	if err := checkAuditNonRootNamespaceCount(path, 2); err != nil {
		t.Fatalf("checkAuditNonRootNamespaceCount returned error: %v", err)
	}
}

func TestPostgresConnectionURLUsesConfiguredHost(t *testing.T) {
	got := postgresConnectionURL("postgres.internal")
	want := "postgresql://{{username}}:{{password}}@postgres.internal:5432/openbao_app?sslmode=disable"
	if got != want {
		t.Fatalf("postgresConnectionURL() = %q, want %q", got, want)
	}
}
