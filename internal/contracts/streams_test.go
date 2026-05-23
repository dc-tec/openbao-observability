package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadStreamContract(t *testing.T) {
	path := writeStreamContract(t, t.TempDir(), baseStreamContract())

	contract, err := LoadStreamContract(path)
	if err != nil {
		t.Fatalf("LoadStreamContract returned error: %v", err)
	}

	if len(contract.Streams) != 2 {
		t.Fatalf("stream count = %d, want 2", len(contract.Streams))
	}
	if !stringSet(contract.AllowedLabels)["log_stream"] {
		t.Fatal("expected log_stream to be allowed")
	}
}

func TestVerifyStreamContract(t *testing.T) {
	root := writeStreamTestRepository(t, baseStreamContract(), baseAlertContract(), baseDashboardContract())

	err := VerifyStreamContract(VerifyStreamOptions{
		ContractPath:           filepath.Join(root, "contracts", "streams", "log-streams.yaml"),
		AlertContractPath:      filepath.Join(root, "contracts", "alerts", "critical.yaml"),
		DashboardContractPaths: []string{filepath.Join(root, "contracts", "dashboards", "openbao-overview.yaml")},
	})
	if err != nil {
		t.Fatalf("VerifyStreamContract returned error: %v", err)
	}
}

func TestValidateLogExpressionRejectsForbiddenSelectorLabel(t *testing.T) {
	contract := mustLoadStreamContract(t, baseStreamContract())

	err := contract.ValidateLogExpression(`{log_stream="openbao.audit", request_id="abc"}`)
	if err == nil {
		t.Fatal("expected forbidden selector label to fail")
	}
	if !strings.Contains(err.Error(), "request_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateLogExpressionRejectsForbiddenGroupingLabel(t *testing.T) {
	contract := mustLoadStreamContract(t, baseStreamContract())

	err := contract.ValidateLogExpression(`sum by (request_path) (count_over_time({log_stream="openbao.audit"}[5m]))`)
	if err == nil {
		t.Fatal("expected forbidden grouping label to fail")
	}
	if !strings.Contains(err.Error(), "request_path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateLogExpressionRejectsUnknownGroupingLabel(t *testing.T) {
	contract := mustLoadStreamContract(t, baseStreamContract())

	err := contract.ValidateLogExpression(`sum by (unexpected) (count_over_time({log_stream="openbao.audit"}[5m]))`)
	if err == nil {
		t.Fatal("expected unknown grouping label to fail")
	}
	if !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateLogExpressionAllowsQueryTimeParsing(t *testing.T) {
	contract := mustLoadStreamContract(t, baseStreamContract())

	err := contract.ValidateLogExpression(`{log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"sys/.*"`)
	if err != nil {
		t.Fatalf("ValidateLogExpression returned error: %v", err)
	}
}

func mustLoadStreamContract(t *testing.T, content string) *StreamContract {
	t.Helper()

	contract, err := LoadStreamContract(writeStreamContract(t, t.TempDir(), content))
	if err != nil {
		t.Fatalf("LoadStreamContract returned error: %v", err)
	}
	return contract
}

func writeStreamContract(t *testing.T, dir, content string) string {
	t.Helper()

	path := filepath.Join(dir, "log-streams.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write stream contract: %v", err)
	}
	return path
}

func writeStreamTestRepository(t *testing.T, streamContract, alertContract, dashboardContract string) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"contracts/streams/log-streams.yaml":         streamContract,
		"contracts/alerts/critical.yaml":             alertContract,
		"contracts/dashboards/openbao-overview.yaml": dashboardContract,
		"docs/runbooks/no-active-openbao-leader.md":  "# No active leader\n",
		"docs/runbooks/audit-log-stream-missing.md":  "# Audit stream missing\n",
	}

	for path, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	return root
}

func baseStreamContract() string {
	return `version: v0.1
status: draft
streams:
  - id: openbao.operational
    default: enabled
    source: OpenBao server logs
    format: json
    access: sre-platform
    retention: 14-30d
  - id: openbao.audit
    default: enabled
    source: OpenBao audit devices
    format: jsonl
    access: security-restricted
    retention: 7-30d
allowedLabels:
  - cluster
  - environment
  - log_stream
  - node_id
forbiddenLabels:
  - request_id
  - request_path
  - token_accessor
`
}
