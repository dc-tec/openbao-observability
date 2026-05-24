package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAlertContract(t *testing.T) {
	root := writeAlertTestRepository(t, baseAlertContract(), []string{
		"docs/runbooks/no-active-openbao-leader.md",
		"docs/runbooks/audit-log-stream-missing.md",
	})
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	contract, err := LoadAlertContract(path)
	if err != nil {
		t.Fatalf("LoadAlertContract returned error: %v", err)
	}

	if got := contract.DefaultSourcePrefix(); got != "vault" {
		t.Fatalf("DefaultSourcePrefix = %q, want vault", got)
	}
	if got := contract.RenderExpression("sum(${p}_core_active)", "openbao"); got != "sum(openbao_core_active)" {
		t.Fatalf("RenderExpression = %q", got)
	}
}

func TestVerifyAlertContract(t *testing.T) {
	root := writeAlertTestRepository(t, baseAlertContract(), []string{
		"docs/runbooks/no-active-openbao-leader.md",
		"docs/runbooks/audit-log-stream-missing.md",
	})
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	err := VerifyAlertContract(VerifyAlertOptions{
		ContractPath:   path,
		SourcePrefix:   "openbao",
		RepositoryRoot: root,
	})
	if err != nil {
		t.Fatalf("VerifyAlertContract returned error: %v", err)
	}
}

func TestVerifyAlertContractRejectsInvalidPromQL(t *testing.T) {
	contract := strings.Replace(baseAlertContract(), "sum(${p}_core_active) == 0", "sum(", 1)
	root := writeAlertTestRepository(t, contract, []string{
		"docs/runbooks/no-active-openbao-leader.md",
		"docs/runbooks/audit-log-stream-missing.md",
	})
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	err := VerifyAlertContract(VerifyAlertOptions{ContractPath: path, RepositoryRoot: root})
	if err == nil {
		t.Fatal("expected invalid PromQL to fail")
	}
	if !strings.Contains(err.Error(), "OpenBaoNoActiveNode") {
		t.Fatalf("error does not include alert id: %v", err)
	}
}

func TestVerifyAlertContractRejectsMissingRunbook(t *testing.T) {
	root := writeAlertTestRepository(t, baseAlertContract(), []string{
		"docs/runbooks/no-active-openbao-leader.md",
	})
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	err := VerifyAlertContract(VerifyAlertOptions{ContractPath: path, RepositoryRoot: root})
	if err == nil {
		t.Fatal("expected missing runbook to fail")
	}
	if !strings.Contains(err.Error(), "OpenBaoAuditStreamMissing") {
		t.Fatalf("error does not include alert id: %v", err)
	}
}

func TestVerifyAlertContractRejectsUnexpectedSeverity(t *testing.T) {
	root := writeAlertTestRepository(t, baseAlertContract(), []string{
		"docs/runbooks/no-active-openbao-leader.md",
		"docs/runbooks/audit-log-stream-missing.md",
	})
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	err := VerifyAlertContract(VerifyAlertOptions{
		ContractPath:     path,
		RepositoryRoot:   root,
		ExpectedSeverity: "warning",
	})
	if err == nil {
		t.Fatal("expected severity mismatch to fail")
	}
	if !strings.Contains(err.Error(), "OpenBaoNoActiveNode") {
		t.Fatalf("error does not include alert id: %v", err)
	}
}

func TestLoadAlertContractRejectsDuplicateIDs(t *testing.T) {
	contract := baseAlertContract() + `
  - id: OpenBaoNoActiveNode
    type: prometheus
    severity: critical
    signal: metrics
    expression: sum(${p}_core_active) == 0
    summary: duplicate
    description: duplicate
    runbook: docs/runbooks/duplicate.md
`
	root := writeAlertTestRepository(t, contract, nil)
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	_, err := LoadAlertContract(path)
	if err == nil {
		t.Fatal("expected duplicate alert IDs to fail")
	}
	if !strings.Contains(err.Error(), "duplicate alert id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeAlertTestRepository(t *testing.T, content string, runbooks []string) string {
	t.Helper()

	root := t.TempDir()
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create alert contract directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write alert contract: %v", err)
	}
	for _, runbook := range runbooks {
		runbookPath := filepath.Join(root, filepath.FromSlash(runbook))
		if err := os.MkdirAll(filepath.Dir(runbookPath), 0o755); err != nil {
			t.Fatalf("create runbook directory: %v", err)
		}
		if err := os.WriteFile(runbookPath, []byte("# Test runbook\n"), 0o644); err != nil {
			t.Fatalf("write runbook: %v", err)
		}
	}
	return root
}

func baseAlertContract() string {
	return `version: v0.1
status: draft
metricPrefixVariable: ${p}
sourcePrefix: vault
alerts:
  - id: OpenBaoNoActiveNode
    type: prometheus
    severity: critical
    signal: metrics
    for: 2m
    expression: sum(${p}_core_active) == 0
    summary: OpenBao has no active node
    description: No active OpenBao node is visible to Prometheus.
    runbook: docs/runbooks/no-active-openbao-leader.md
  - id: OpenBaoAuditStreamMissing
    type: loki
    severity: critical
    signal: loki
    for: 10m
    expression: absent_over_time({log_stream="openbao.audit"}[10m])
    summary: OpenBao audit log stream is missing
    description: Loki has not received OpenBao audit logs for the alert window.
    runbook: docs/runbooks/audit-log-stream-missing.md
`
}
