package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMetricContract(t *testing.T) {
	path := writeContract(t, t.TempDir(), "2.5.4", nil)

	contract, err := LoadMetricContract(path)
	if err != nil {
		t.Fatalf("LoadMetricContract returned error: %v", err)
	}

	if contract.OpenBAOVersion != "2.5.4" {
		t.Fatalf("OpenBAOVersion = %q, want 2.5.4", contract.OpenBAOVersion)
	}
	if got := contract.Metrics[0].FixtureName("openbao"); got != "openbao_core_active" {
		t.Fatalf("FixtureName = %q, want openbao_core_active", got)
	}
	if got := contract.Metrics[1].FixtureName("vault"); got != "vault_audit_local_file__log_request" {
		t.Fatalf("FixtureName with fixture override = %q", got)
	}
}

func TestLoadMetricContractRejectsDuplicateMetricIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contract.yaml")
	content := baseContract("2.5.4", nil) + `
  - id: core_active
    prometheusName: ${p}_core_active_duplicate
    required: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write contract: %v", err)
	}

	_, err := LoadMetricContract(path)
	if err == nil {
		t.Fatal("expected duplicate metric IDs to fail")
	}
	if !strings.Contains(err.Error(), "duplicate metric id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyMetricContract(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join(dir, "fixtures")
	writeMetricFixture(t, fixtureDir, "vault")
	writeMetricFixture(t, fixtureDir, "openbao")

	required := []string{
		filepath.Join(fixtureDir, "metrics", "openbao-2.5.4-vault-prefix.prom"),
		filepath.Join(fixtureDir, "metrics", "openbao-2.5.4-openbao-prefix.prom"),
	}
	contractPath := writeContract(t, dir, "2.5.4", required)

	err := VerifyMetricContract(VerifyOptions{
		ContractPath: contractPath,
		FixtureDir:   fixtureDir,
	})
	if err != nil {
		t.Fatalf("VerifyMetricContract returned error: %v", err)
	}
}

func TestVerifyMetricContractFailsWhenMetricMissing(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join(dir, "fixtures")
	writeMetricFixture(t, fixtureDir, "vault")
	writeMetricFixture(t, fixtureDir, "openbao")

	openbaoFixture := filepath.Join(fixtureDir, "metrics", "openbao-2.5.4-openbao-prefix.prom")
	content, err := os.ReadFile(openbaoFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	content = []byte(strings.ReplaceAll(string(content), "openbao_runtime_num_goroutines 42\n", ""))
	if err := os.WriteFile(openbaoFixture, content, 0o644); err != nil {
		t.Fatalf("rewrite fixture: %v", err)
	}

	required := []string{
		filepath.Join(fixtureDir, "metrics", "openbao-2.5.4-vault-prefix.prom"),
		filepath.Join(fixtureDir, "metrics", "openbao-2.5.4-openbao-prefix.prom"),
	}
	contractPath := writeContract(t, dir, "2.5.4", required)

	err = VerifyMetricContract(VerifyOptions{
		ContractPath: contractPath,
		FixtureDir:   fixtureDir,
	})
	if err == nil {
		t.Fatal("expected missing required metric to fail")
	}
	if !strings.Contains(err.Error(), "runtime_num_goroutines") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeContract(t *testing.T, dir, version string, required []string) string {
	t.Helper()

	path := filepath.Join(dir, "contract.yaml")
	if err := os.WriteFile(path, []byte(baseContract(version, required)), 0o644); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	return path
}

func baseContract(version string, required []string) string {
	var fixtureLines strings.Builder
	for _, fixture := range required {
		fixtureLines.WriteString("    - ")
		fixtureLines.WriteString(fixture)
		fixtureLines.WriteByte('\n')
	}
	if fixtureLines.Len() == 0 {
		fixtureLines.WriteString("    []\n")
	}

	return `version: v0.1
status: draft
openbaoVersion: "` + version + `"
metricPrefixes:
  supported:
    - vault
    - openbao
  default: vault
normalization:
  recordingRulePrefix: openbao
fixtures:
  required:
` + fixtureLines.String() + `metrics:
  - id: core_active
    prometheusName: ${p}_core_active
    required: true
  - id: audit_device_log_request
    prometheusName: ${p}_audit_<device>__log_request
    fixturePrometheusName: ${p}_audit_local_file__log_request
    required: true
  - id: runtime_num_goroutines
    prometheusName: ${p}_runtime_num_goroutines
    required: true
`
}

func writeMetricFixture(t *testing.T, root, prefix string) {
	t.Helper()

	path := filepath.Join(root, "metrics", "openbao-2.5.4-"+prefix+"-prefix.prom")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}

	content := prefix + `_core_active{cluster="vault-cluster-test"} 1
` + prefix + `_audit_local_file__log_request{quantile="0.5"} 0.1
` + prefix + `_runtime_num_goroutines 42
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write metrics fixture: %v", err)
	}
}
