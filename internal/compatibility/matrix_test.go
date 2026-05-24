package compatibility

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateMatrixReportsFixtureCoverage(t *testing.T) {
	root := t.TempDir()
	contractPath := filepath.Join(root, "contracts", "metrics", "openbao-core.yaml")
	fixtureDir := filepath.Join(root, "fixtures", "captured", "openbao-2.5.4")
	outputPath := filepath.Join(root, "generated", "docs", "metric-compatibility-matrix.md")

	writeFile(t, contractPath, `version: v0.1
status: draft
openbaoVersion: "2.5.4"
metricPrefixes:
  supported:
    - vault
    - openbao
  default: vault
normalization:
  recordingRulePrefix: openbao
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
`)
	writeFile(t, filepath.Join(fixtureDir, "metrics", "openbao-2.5.4-vault-prefix.prom"), `# HELP vault_core_active Active node status.
# TYPE vault_core_active gauge
vault_core_active{cluster="test"} 1
`)
	writeFile(t, filepath.Join(fixtureDir, "metrics", "openbao-2.5.4-openbao-prefix.prom"), `# HELP openbao_core_active Active node status.
# TYPE openbao_core_active gauge
openbao_core_active{cluster="test"} 1
# HELP openbao_token_count Token count.
# TYPE openbao_token_count gauge
openbao_token_count{namespace="root"} 4
`)
	writeFile(t, filepath.Join(fixtureDir, "metrics", "openbao-2.5.4-raft-vault-node0.prom"), `# HELP vault_core_active Active node status.
# TYPE vault_core_active gauge
vault_core_active{cluster="test",instance="node0"} 1
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
		"`vault_core_active` | observed | gauge | `cluster`",
		"`openbao_token_count` | observed | gauge | `namespace`",
		"`vault_token_count` | missing | - | none",
		"Observed in test fixtures.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated matrix did not contain %q:\n%s", want, got)
		}
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
