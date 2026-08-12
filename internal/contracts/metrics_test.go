package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMetricContract(t *testing.T) {
	path := writeContract(t, t.TempDir(), nil)

	contract, err := LoadMetricContract(path)
	if err != nil {
		t.Fatalf("LoadMetricContract returned error: %v", err)
	}

	if contract.OpenBAOVersion != "2.6.0" {
		t.Fatalf("OpenBAOVersion = %q, want 2.6.0", contract.OpenBAOVersion)
	}
	if got := contract.Metrics[0].FixtureName("openbao"); got != "openbao_core_active" {
		t.Fatalf("FixtureName = %q, want openbao_core_active", got)
	}
	if got := contract.Metrics[1].FixtureName("vault"); got != "vault_audit_local_file__log_request" {
		t.Fatalf("FixtureName with fixture override = %q", got)
	}
}

func TestLoadMetricContractRejectsInvalidSchema(t *testing.T) {
	base := baseContract(nil)
	tests := []struct {
		name      string
		content   string
		errorText string
	}{
		{
			name:      "unknown top-level field",
			content:   strings.Replace(base, "maturity:", "unexpected: true\nmaturity:", 1),
			errorText: "field unexpected not found",
		},
		{
			name: "unknown nested field",
			content: strings.Replace(
				base,
				"  recordingRulePrefix: openbao",
				"  recordingRulePrefix: openbao\n  unexpected: true",
				1,
			),
			errorText: "field unexpected not found",
		},
		{
			name:      "missing version",
			content:   strings.TrimPrefix(base, "version: v0.1\n"),
			errorText: "missing version",
		},
		{
			name:      "unsupported version",
			content:   strings.Replace(base, "version: v0.1", "version: v9", 1),
			errorText: `unsupported version "v9"`,
		},
		{
			name:      "second document",
			content:   base + "---\nversion: v0.1\n",
			errorText: "exactly one YAML document",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "contract.yaml")
			if err := os.WriteFile(path, []byte(test.content), 0o644); err != nil {
				t.Fatalf("write contract: %v", err)
			}
			_, err := LoadMetricContract(path)
			if err == nil {
				t.Fatal("expected invalid metric contract to fail")
			}
			if !strings.Contains(err.Error(), test.errorText) {
				t.Fatalf("error = %v, want text %q", err, test.errorText)
			}
		})
	}
}

func TestLoadMetricContractRejectsInvalidPrefixes(t *testing.T) {
	base := baseContract(nil)
	tests := []struct {
		name      string
		content   string
		errorText string
	}{
		{
			name:      "duplicate supported prefix",
			content:   strings.Replace(base, "    - openbao", "    - vault", 1),
			errorText: `duplicate supported metric prefix "vault"`,
		},
		{
			name:      "unsupported default prefix",
			content:   strings.Replace(base, "  default: vault", "  default: other", 1),
			errorText: "is not listed in metricPrefixes.supported",
		},
		{
			name:      "invalid supported prefix",
			content:   strings.Replace(base, "    - openbao", "    - open-bao", 1),
			errorText: "not a valid Prometheus metric prefix",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "contract.yaml")
			if err := os.WriteFile(path, []byte(test.content), 0o644); err != nil {
				t.Fatalf("write contract: %v", err)
			}
			_, err := LoadMetricContract(path)
			if err == nil {
				t.Fatal("expected invalid metric prefix declaration to fail")
			}
			if !strings.Contains(err.Error(), test.errorText) {
				t.Fatalf("error = %v, want text %q", err, test.errorText)
			}
		})
	}
}

func TestMetricContractValidateSourcePrefix(t *testing.T) {
	path := writeContract(t, t.TempDir(), nil)
	contract, err := LoadMetricContract(path)
	if err != nil {
		t.Fatalf("LoadMetricContract returned error: %v", err)
	}

	if err := contract.ValidateSourcePrefix("openbao"); err != nil {
		t.Fatalf("ValidateSourcePrefix rejected supported prefix: %v", err)
	}
	if err := contract.ValidateSourcePrefix("other"); err == nil {
		t.Fatal("expected unsupported source prefix to fail")
	}
	if err := contract.ValidateSourcePrefix("open-bao"); err == nil {
		t.Fatal("expected invalid source prefix to fail")
	}
}

func TestLoadMetricContractRejectsDuplicateMetricIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contract.yaml")
	content := baseContract(nil) + `
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

func TestLoadMetricContractRejectsInvalidMaturity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contract.yaml")
	content := strings.Replace(baseContract(nil), "lifecycle: draft", "lifecycle: preview", 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write contract: %v", err)
	}

	_, err := LoadMetricContract(path)
	if err == nil {
		t.Fatal("expected invalid maturity lifecycle to fail")
	}
	if !strings.Contains(err.Error(), "maturity.lifecycle") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyMetricContract(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join(dir, "fixtures")
	writeMetricFixture(t, fixtureDir, "vault")
	writeMetricFixture(t, fixtureDir, "openbao")

	required := []string{
		filepath.Join(fixtureDir, "metrics", "openbao-2.6.0-vault-prefix.prom"),
		filepath.Join(fixtureDir, "metrics", "openbao-2.6.0-openbao-prefix.prom"),
	}
	contractPath := writeContract(t, dir, required)

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

	openbaoFixture := filepath.Join(fixtureDir, "metrics", "openbao-2.6.0-openbao-prefix.prom")
	content, err := os.ReadFile(openbaoFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	content = []byte(strings.ReplaceAll(string(content), "openbao_runtime_num_goroutines 42\n", ""))
	if err := os.WriteFile(openbaoFixture, content, 0o644); err != nil {
		t.Fatalf("rewrite fixture: %v", err)
	}

	required := []string{
		filepath.Join(fixtureDir, "metrics", "openbao-2.6.0-vault-prefix.prom"),
		filepath.Join(fixtureDir, "metrics", "openbao-2.6.0-openbao-prefix.prom"),
	}
	contractPath := writeContract(t, dir, required)

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

func writeContract(t *testing.T, dir string, required []string) string {
	t.Helper()

	path := filepath.Join(dir, "contract.yaml")
	if err := os.WriteFile(path, []byte(baseContract(required)), 0o644); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	return path
}

func baseContract(required []string) string {
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
maturity:
  lifecycle: draft
  evidence:
    - documented
    - fixture-validated
    - generated-validated
openbaoVersion: "2.6.0"
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

	path := filepath.Join(root, "metrics", "openbao-2.6.0-"+prefix+"-prefix.prom")
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
