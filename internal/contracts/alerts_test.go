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
		"docs/runbooks/audit-canary-missing.md",
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

func TestLoadAlertContractRejectsUnknownField(t *testing.T) {
	content := strings.Replace(baseAlertContract(), "alerts:", "unexpected: true\nalerts:", 1)
	root := writeAlertTestRepository(t, content, nil)
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	_, err := LoadAlertContract(path)
	if err == nil {
		t.Fatal("expected unknown alert contract field to fail")
	}
	if !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAlertContractRejectsUnsupportedVersion(t *testing.T) {
	content := strings.Replace(baseAlertContract(), "version: v0.1", "version: v9", 1)
	root := writeAlertTestRepository(t, content, nil)
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	_, err := LoadAlertContract(path)
	if err == nil {
		t.Fatal("expected unsupported alert contract version to fail")
	}
	if !strings.Contains(err.Error(), `unsupported version "v9"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAlertContractRejectsMissingVersion(t *testing.T) {
	content := strings.TrimPrefix(baseAlertContract(), "version: v0.1\n")
	root := writeAlertTestRepository(t, content, nil)
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	_, err := LoadAlertContract(path)
	if err == nil {
		t.Fatal("expected missing alert contract version to fail")
	}
	if !strings.Contains(err.Error(), "missing version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAlertContractRejectsInvalidSourcePrefix(t *testing.T) {
	content := strings.Replace(baseAlertContract(), "sourcePrefix: vault", "sourcePrefix: open-bao", 1)
	root := writeAlertTestRepository(t, content, nil)
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	_, err := LoadAlertContract(path)
	if err == nil {
		t.Fatal("expected invalid alert source prefix to fail")
	}
	if !strings.Contains(err.Error(), "not a valid Prometheus metric prefix") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyAlertContractRejectsInvalidSourcePrefixOverride(t *testing.T) {
	root := writeAlertTestRepository(t, baseAlertContract(), nil)
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	err := VerifyAlertContract(VerifyAlertOptions{
		ContractPath:   path,
		SourcePrefix:   "open-bao",
		RepositoryRoot: root,
	})
	if err == nil {
		t.Fatal("expected invalid alert source prefix override to fail")
	}
	if !strings.Contains(err.Error(), "not a valid Prometheus metric prefix") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAlertContractRejectsReservedLabels(t *testing.T) {
	for _, key := range []string{"severity", "signal", "source_prefix"} {
		t.Run(key, func(t *testing.T) {
			content := strings.Replace(
				baseAlertContract(),
				"    runbook: docs/runbooks/no-active-openbao-leader.md",
				"    runbook: docs/runbooks/no-active-openbao-leader.md\n    labels:\n      "+key+": overridden",
				1,
			)
			root := writeAlertTestRepository(t, content, nil)
			path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

			_, err := LoadAlertContract(path)
			if err == nil {
				t.Fatal("expected reserved alert label to fail")
			}
			if !strings.Contains(err.Error(), `reserved label "`+key+`"`) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadAlertContractRejectsReservedAnnotations(t *testing.T) {
	for _, key := range []string{"summary", "description", "runbook_url"} {
		t.Run(key, func(t *testing.T) {
			content := strings.Replace(
				baseAlertContract(),
				"    runbook: docs/runbooks/no-active-openbao-leader.md",
				"    runbook: docs/runbooks/no-active-openbao-leader.md\n    annotations:\n      "+key+": overridden",
				1,
			)
			root := writeAlertTestRepository(t, content, nil)
			path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

			_, err := LoadAlertContract(path)
			if err == nil {
				t.Fatal("expected reserved alert annotation to fail")
			}
			if !strings.Contains(err.Error(), `reserved annotation "`+key+`"`) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadAlertContractAllowsCustomMetadata(t *testing.T) {
	content := strings.Replace(
		baseAlertContract(),
		"    runbook: docs/runbooks/no-active-openbao-leader.md",
		"    runbook: docs/runbooks/no-active-openbao-leader.md\n"+
			"    labels:\n      component: core\n"+
			"    annotations:\n      owner: platform",
		1,
	)
	root := writeAlertTestRepository(t, content, nil)
	path := filepath.Join(root, "contracts", "alerts", "critical.yaml")

	if _, err := LoadAlertContract(path); err != nil {
		t.Fatalf("LoadAlertContract rejected custom metadata: %v", err)
	}
}

func TestVerifyAlertContract(t *testing.T) {
	root := writeAlertTestRepository(t, baseAlertContract(), []string{
		"docs/runbooks/no-active-openbao-leader.md",
		"docs/runbooks/audit-canary-missing.md",
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
		"docs/runbooks/audit-canary-missing.md",
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
	if !strings.Contains(err.Error(), "OpenBaoAuditCanaryMissing") {
		t.Fatalf("error does not include alert id: %v", err)
	}
}

func TestVerifyAlertContractRejectsUnexpectedSeverity(t *testing.T) {
	root := writeAlertTestRepository(t, baseAlertContract(), []string{
		"docs/runbooks/no-active-openbao-leader.md",
		"docs/runbooks/audit-canary-missing.md",
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
maturity:
  lifecycle: draft
  evidence:
    - documented
    - generated-validated
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
  - id: OpenBaoAuditCanaryMissing
    type: loki
    severity: critical
    signal: loki
    for: 15m
    expression: >-
      absent_over_time({log_stream="openbao.audit"} | json request_path="request.path" |
      request_path="secret/data/observability/audit-canary" [15m])
    summary: OpenBao audit canary is missing
    description: Loki has not received the expected OpenBao audit canary request for the alert window.
    runbook: docs/runbooks/audit-canary-missing.md
`
}
