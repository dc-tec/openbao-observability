package compatibility

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

func TestGenerateMatrixReportsFixtureCoverage(t *testing.T) {
	root := t.TempDir()
	contractPath := filepath.Join(root, "contracts", "metrics", "openbao-core.yaml")
	fixtureDir := filepath.Join(root, "fixtures", "captured", "openbao-2.5.5")
	outputPath := filepath.Join(root, "generated", "docs", "metric-compatibility-matrix.md")
	metricPath := func(name string) string {
		return filepath.Join(fixtureDir, "metrics", name)
	}

	writeFile(t, contractPath, `version: v0.1
maturity:
  lifecycle: draft
  evidence:
    - documented
    - fixture-validated
    - generated-validated
openbaoVersion: "2.5.5"
metricPrefixes:
  supported:
    - vault
    - openbao
  default: vault
normalization:
  recordingRulePrefix: openbao
coverageProfiles:
  - id: vault-prefix
    class: prefix-smoke
    defaultExpectation: optional
  - id: openbao-prefix
    class: prefix-smoke
    defaultExpectation: optional
  - id: raft-vault-node0
    class: ha-raft-active
    defaultExpectation: optional
metrics:
  - id: core_active
    docsName: vault.core.active
    prometheusName: ${p}_core_active
    required: true
    overview: true
    notes:
      - Observed in test fixtures.
  - id: token_count
    docsName: vault.token.count
    prometheusName: ${p}_token_count
    required: false
    overview: true
    coverage:
      raft-vault-node0: variable
`)
	writeFile(t, metricPath("openbao-2.5.5-vault-prefix.prom"), `# HELP vault_core_active Active node status.
# TYPE vault_core_active gauge
vault_core_active{cluster="test"} 1
`)
	writeFile(t, metricPath("openbao-2.5.5-openbao-prefix.prom"), `# HELP openbao_core_active Active node status.
# TYPE openbao_core_active gauge
openbao_core_active{cluster="test"} 1
# HELP openbao_token_count Token count.
# TYPE openbao_token_count gauge
openbao_token_count{namespace="root"} 4
`)
	writeFile(t, metricPath("openbao-2.5.5-raft-vault-node0.prom"), `# HELP vault_core_active Active node status.
# TYPE vault_core_active gauge
vault_core_active{cluster="test",instance="node0"} 1
# HELP vault_token_count Token count.
# TYPE vault_token_count gauge
vault_token_count{namespace="root"} 2
`)

	if err := GenerateMatrix(MatrixOptions{
		ContractPath: contractPath,
		FixtureDir:   fixtureDir,
		OutputPath:   outputPath,
	}); err != nil {
		t.Fatalf("GenerateMatrix returned error: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read generated matrix: %v", err)
	}
	got := string(content)
	for _, want := range []string{
		"# OpenBao metric compatibility matrix",
		"`vault-prefix`",
		"`openbao-prefix`",
		"`raft-vault-node0`",
		"| `raft-vault-node0` | `ha-raft-active` | `vault` |",
		"| Maturity lifecycle | `draft` |",
		"| Maturity evidence | `documented`, `fixture-validated`, `generated-validated` |",
		"`vault_core_active` | required | observed | gauge | `cluster`",
		"`openbao_token_count` | optional | observed | gauge | `namespace`",
		"`vault_token_count` | optional | optional-missing | - | none",
		"`vault_token_count` | variable | variable | - | none",
		"Observed in test fixtures.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated matrix did not contain %q:\n%s", want, got)
		}
	}
}

func TestMissingStatusClassifiesExpectations(t *testing.T) {
	tests := []struct {
		name        string
		expectation string
		want        string
	}{
		{
			name:        "required",
			expectation: contracts.MetricCoverageRequired,
			want:        statusMissingRequired,
		},
		{
			name:        "optional",
			expectation: contracts.MetricCoverageOptional,
			want:        statusOptionalMissing,
		},
		{
			name:        "not applicable",
			expectation: contracts.MetricCoverageNotApplicable,
			want:        statusNotApplicable,
		},
		{
			name:        "unclassified",
			expectation: coverageUnclassified,
			want:        statusMissingUnclassified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := missingStatus(tt.expectation); got != tt.want {
				t.Fatalf("missingStatus(%q) = %q, want %q", tt.expectation, got, tt.want)
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
