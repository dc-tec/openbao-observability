package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRepository(t *testing.T) {
	root := writeRepositoryTestRepository(t, baseDashboardContract(), defaultRepositoryDocsIndex())

	if err := VerifyRepository(VerifyRepositoryOptions{RepositoryRoot: root}); err != nil {
		t.Fatalf("VerifyRepository returned error: %v", err)
	}
}

func TestVerifyRepositoryRejectsMakefileDashboardDrift(t *testing.T) {
	root := writeRepositoryTestRepository(t, baseDashboardContract(), defaultRepositoryDocsIndex())
	makefile := "DASHBOARD_CONTRACTS ?=\nGENERATED_DASHBOARDS ?= generated/grafana/openbao-overview.json\n"
	writeRepositoryTestFile(t, root, "Makefile", makefile)

	err := VerifyRepository(VerifyRepositoryOptions{RepositoryRoot: root})
	if err == nil {
		t.Fatal("expected Makefile dashboard drift to fail")
	}
	if !strings.Contains(err.Error(), "DASHBOARD_CONTRACTS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRepositoryRejectsUnknownRecordingRuleReference(t *testing.T) {
	dashboardContract := strings.Replace(baseDashboardContract(), `min(up{job="openbao"})`, "openbao:missing:sum", 1)
	root := writeRepositoryTestRepository(t, dashboardContract, defaultRepositoryDocsIndex())

	err := VerifyRepository(VerifyRepositoryOptions{RepositoryRoot: root})
	if err == nil {
		t.Fatal("expected unknown recording rule reference to fail")
	}
	if !strings.Contains(err.Error(), "openbao:missing:sum") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRepositoryRejectsUnindexedRunbook(t *testing.T) {
	root := writeRepositoryTestRepository(t, baseDashboardContract(), "# Documentation\n")

	err := VerifyRepository(VerifyRepositoryOptions{RepositoryRoot: root})
	if err == nil {
		t.Fatal("expected unindexed runbook to fail")
	}
	if !strings.Contains(err.Error(), "docs/README.md") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeRepositoryTestRepository(t *testing.T, dashboardContract, docsIndex string) string {
	t.Helper()

	root := t.TempDir()
	files := map[string]string{
		"Makefile": `DASHBOARD_CONTRACTS ?= contracts/dashboards/openbao-overview.yaml
GENERATED_DASHBOARDS ?= generated/grafana/openbao-overview.json
`,
		"contracts/dashboards/openbao-overview.yaml": dashboardContract,
		"contracts/alerts/critical.yaml":             baseAlertContract(),
		"docs/README.md":                             docsIndex,
		"docs/runbooks/no-active-openbao-leader.md":  "# No active leader\n",
		"docs/runbooks/audit-log-stream-missing.md":  "# Audit stream missing\n",
		"generated/prometheus/openbao-recording-rules.yaml": `groups:
  - name: openbao.recording
    rules:
      - record: openbao:core_active:sum
        expr: sum(vault_core_active)
`,
		"generated/prometheus/openbao-alerts.yaml": `groups:
  - name: openbao.alerts
    rules:
      - alert: OpenBaoNoActiveNode
        expr: sum(vault_core_active) == 0
`,
		"generated/loki/openbao-alerts.yaml": `spec:
  groups:
    - name: openbao.loki.alerts
      rules:
        - alert: OpenBaoAuditStreamMissing
          expr: absent_over_time({log_stream="openbao.audit"}[10m])
`,
	}

	for path, content := range files {
		writeRepositoryTestFile(t, root, path, content)
	}
	writeGeneratedDashboardFromContract(t, root)
	return root
}

func writeGeneratedDashboardFromContract(t *testing.T, root string) {
	t.Helper()

	contractPath := filepath.Join(root, "contracts", "dashboards", "openbao-overview.yaml")
	contract, err := LoadDashboardContract(contractPath)
	if err != nil {
		t.Fatalf("load dashboard contract: %v", err)
	}

	panels := make([]generatedDashboardPanel, 0, len(contract.Panels))
	for _, panel := range contract.Panels {
		datasource, err := contractDatasource(contract, panel.Datasource)
		if err != nil {
			t.Fatalf("resolve datasource: %v", err)
		}
		ds := generatedDatasource{Type: datasource.Type, UID: datasource.UID}
		panels = append(panels, generatedDashboardPanel{
			Title:      panel.Title,
			Type:       panel.Type,
			Datasource: ds,
			Targets: []generatedTarget{
				{Expr: panel.Expression, Datasource: ds},
			},
		})
	}

	dashboard := generatedDashboard{
		UID:    contract.UID,
		Title:  contract.Title,
		Panels: panels,
	}
	content, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		t.Fatalf("marshal generated dashboard: %v", err)
	}
	writeRepositoryTestFile(t, root, "generated/grafana/openbao-overview.json", string(content))
}

func writeRepositoryTestFile(t *testing.T, root, name, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func defaultRepositoryDocsIndex() string {
	return `# Documentation

## Runbooks

- [No active OpenBao leader](./runbooks/no-active-openbao-leader.md)
- [Audit log stream missing](./runbooks/audit-log-stream-missing.md)
`
}
