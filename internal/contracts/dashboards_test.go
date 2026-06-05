package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDashboardContract(t *testing.T) {
	path := writeDashboardContract(t, t.TempDir(), baseDashboardContract())

	contract, err := LoadDashboardContract(path)
	if err != nil {
		t.Fatalf("LoadDashboardContract returned error: %v", err)
	}

	if contract.UID != "openbao-overview" {
		t.Fatalf("UID = %q, want openbao-overview", contract.UID)
	}
	if len(contract.Panels) != 2 {
		t.Fatalf("panel count = %d, want 2", len(contract.Panels))
	}
	if len(contract.Variables) != 1 {
		t.Fatalf("variable count = %d, want 1", len(contract.Variables))
	}
}

func TestVerifyDashboardContract(t *testing.T) {
	path := writeDashboardContract(t, t.TempDir(), baseDashboardContract())

	err := VerifyDashboardContract(VerifyDashboardOptions{ContractPath: path})
	if err != nil {
		t.Fatalf("VerifyDashboardContract returned error: %v", err)
	}
}

func TestVerifyDashboardContractRejectsInvalidPromQL(t *testing.T) {
	contract := strings.Replace(baseDashboardContract(), "min(up{job=\"openbao\"})", "sum(", 1)
	path := writeDashboardContract(t, t.TempDir(), contract)

	err := VerifyDashboardContract(VerifyDashboardOptions{ContractPath: path})
	if err == nil {
		t.Fatal("expected invalid PromQL to fail")
	}
	if !strings.Contains(err.Error(), "scrape-health") {
		t.Fatalf("error does not include panel id: %v", err)
	}
}

func TestLoadDashboardContractRejectsDuplicatePanelIDs(t *testing.T) {
	contract := baseDashboardContract() + `
  - id: scrape-health
    title: Duplicate
    type: stat
    signal: metrics
    datasource: metrics
    expression: up
    grid:
      x: 0
      y: 8
      w: 6
      h: 4
`
	path := writeDashboardContract(t, t.TempDir(), contract)

	_, err := LoadDashboardContract(path)
	if err == nil {
		t.Fatal("expected duplicate panel IDs to fail")
	}
	if !strings.Contains(err.Error(), "duplicate panel id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadDashboardContractRejectsDuplicateVariables(t *testing.T) {
	contract := strings.Replace(baseDashboardContract(), "variables:", `variables:
  - name: request_id
    label: Duplicate request ID
    type: textbox
    default: .*
`, 1)
	path := writeDashboardContract(t, t.TempDir(), contract)

	_, err := LoadDashboardContract(path)
	if err == nil {
		t.Fatal("expected duplicate variables to fail")
	}
	if !strings.Contains(err.Error(), "duplicate variable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyDashboardContractAppliesVariableDefaults(t *testing.T) {
	path := writeDashboardContract(t, t.TempDir(), baseDashboardContract())

	contract, err := LoadDashboardContract(path)
	if err != nil {
		t.Fatalf("LoadDashboardContract returned error: %v", err)
	}

	expression := contract.ExpressionWithDefaultVariables(
		`{log_stream="openbao.audit"} | json request_id="request.id" | ` +
			`request_id=~"${request_id:raw}"`,
	)
	if !strings.Contains(expression, `request_id=~".*"`) {
		t.Fatalf("expression = %q, want default variable interpolation", expression)
	}

	if err := contract.ValidateExpressions(); err != nil {
		t.Fatalf("ValidateExpressions returned error: %v", err)
	}
}

func writeDashboardContract(t *testing.T, dir, content string) string {
	t.Helper()

	path := filepath.Join(dir, "dashboard.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write dashboard contract: %v", err)
	}
	return path
}

func baseDashboardContract() string {
	return `version: v0.1
maturity:
  lifecycle: draft
  evidence:
    - generated-validated
uid: openbao-overview
title: OpenBao overview
refresh: 30s
timeRange:
  from: now-1h
  to: now
datasources:
  metrics:
    type: prometheus
    uid: prometheus
  logs:
    type: loki
    uid: loki
variables:
  - name: request_id
    label: Request ID
    type: textbox
    default: .*
panels:
  - id: scrape-health
    title: Scrape health
    type: stat
    signal: metrics
    datasource: metrics
    expression: min(up{job="openbao"})
    grid:
      x: 0
      y: 0
      w: 6
      h: 4
  - id: audit-stream
    title: Audit stream
    type: logs
    signal: logs
    datasource: logs
    expression: '{log_stream="openbao.audit"} | json request_id="request.id" | request_id=~"${request_id:raw}"'
    grid:
      x: 0
      y: 4
      w: 12
      h: 8
`
}
