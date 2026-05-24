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

func TestVerifyRepositoryRejectsGeneratedDashboardSchemaDrift(t *testing.T) {
	root := writeRepositoryTestRepository(t, baseDashboardContract(), defaultRepositoryDocsIndex())
	dashboardPath := filepath.Join(root, "generated", "grafana", "openbao-overview.json")
	var dashboard map[string]any
	content, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("read generated dashboard: %v", err)
	}
	if err := json.Unmarshal(content, &dashboard); err != nil {
		t.Fatalf("parse generated dashboard: %v", err)
	}
	delete(dashboard, "schemaVersion")
	updated, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		t.Fatalf("marshal generated dashboard: %v", err)
	}
	if err := os.WriteFile(dashboardPath, updated, 0o644); err != nil {
		t.Fatalf("write generated dashboard: %v", err)
	}

	err = VerifyRepository(VerifyRepositoryOptions{RepositoryRoot: root})
	if err == nil {
		t.Fatal("expected generated dashboard schema drift to fail")
	}
	if !strings.Contains(err.Error(), "schemaVersion") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRepositoryRejectsForbiddenGeneratedLokiLabel(t *testing.T) {
	root := writeRepositoryTestRepository(t, baseDashboardContract(), defaultRepositoryDocsIndex())
	dashboardPath := filepath.Join(root, "generated", "grafana", "openbao-overview.json")
	dashboard := readRepositoryTestDashboard(t, dashboardPath)
	for i := range dashboard.Panels {
		for j := range dashboard.Panels[i].Targets {
			if dashboard.Panels[i].Targets[j].Datasource.Type == "loki" {
				dashboard.Panels[i].Targets[j].Expr = `{log_stream="openbao.audit", request_id="abc"}`
			}
		}
	}
	writeRepositoryTestDashboard(t, dashboardPath, dashboard)

	err := VerifyRepository(VerifyRepositoryOptions{RepositoryRoot: root})
	if err == nil {
		t.Fatal("expected forbidden generated Loki label to fail")
	}
	if !strings.Contains(err.Error(), "request_id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyRepositoryRejectsPrefixVariantDrift(t *testing.T) {
	root := writeRepositoryTestRepository(t, baseDashboardContract(), defaultRepositoryDocsIndex())
	writeRepositoryTestFile(
		t,
		root,
		"generated/prometheus/openbao-prefix/openbao-recording-rules.yaml",
		nativeRecordingRules("openbao", "vault_core_active"),
	)

	err := VerifyRepository(VerifyRepositoryOptions{RepositoryRoot: root})
	if err == nil {
		t.Fatal("expected prefix variant drift to fail")
	}
	if !strings.Contains(err.Error(), "openbao-prefix") {
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
		"contracts/streams/log-streams.yaml":         baseStreamContract(),
		"docs/README.md":                             docsIndex,
		"docs/runbooks/no-active-openbao-leader.md":  "# No active leader\n",
		"docs/runbooks/audit-canary-missing.md":      "# Audit canary missing\n",
	}

	for path, content := range files {
		writeRepositoryTestFile(t, root, path, content)
	}
	writeGeneratedDashboardFromContract(t, root)
	writeRepositoryGeneratedRuleVariants(t, root)
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
	for index, panel := range contract.Panels {
		datasource, err := contractDatasource(contract, panel.Datasource)
		if err != nil {
			t.Fatalf("resolve datasource: %v", err)
		}
		ds := generatedDatasource(datasource)
		panels = append(panels, generatedDashboardPanel{
			Title:      panel.Title,
			Type:       panel.Type,
			Datasource: ds,
			GridPos:    panel.Grid,
			ID:         index + 1,
			Targets: []generatedTarget{
				{Expr: panel.Expression, Datasource: ds, RefID: "A"},
			},
		})
	}

	variables := make([]generatedDashboardVariable, 0, len(contract.Variables))
	for _, variable := range contract.Variables {
		variables = append(variables, generatedDashboardVariable{
			Current: generatedDashboardVariableCurrent{
				Text:  variable.Default,
				Value: variable.Default,
			},
			Name:  variable.Name,
			Query: variable.Default,
			Type:  variable.Type,
		})
	}

	dashboard := generatedDashboard{
		Refresh:       contract.Refresh,
		SchemaVersion: 41,
		Templating:    generatedDashboardTemplating{List: variables},
		Time:          contract.TimeRange,
		UID:           contract.UID,
		Title:         contract.Title,
		Panels:        panels,
	}
	content, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		t.Fatalf("marshal generated dashboard: %v", err)
	}
	writeRepositoryTestFile(t, root, "generated/grafana/openbao-overview.json", string(content))
}

func readRepositoryTestDashboard(t *testing.T, path string) generatedDashboard {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated dashboard: %v", err)
	}
	var dashboard generatedDashboard
	if err := json.Unmarshal(content, &dashboard); err != nil {
		t.Fatalf("parse generated dashboard: %v", err)
	}
	return dashboard
}

func writeRepositoryTestDashboard(t *testing.T, path string, dashboard generatedDashboard) {
	t.Helper()

	content, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		t.Fatalf("marshal generated dashboard: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write generated dashboard: %v", err)
	}
}

func writeRepositoryGeneratedRuleVariants(t *testing.T, root string) {
	t.Helper()

	prometheusFiles := []string{
		"openbao-recording-rules.yaml",
		"openbao-alerts.yaml",
		"openbao-warning-alerts.yaml",
		"openbao-security-alerts.yaml",
	}
	for _, fileName := range prometheusFiles {
		vaultContent := nativeEmptyRules()
		openbaoContent := nativeEmptyRules()
		switch fileName {
		case "openbao-recording-rules.yaml":
			vaultContent = nativeRecordingRules("vault", "vault_core_active")
			openbaoContent = nativeRecordingRules("openbao", "openbao_core_active")
		case "openbao-alerts.yaml":
			vaultContent = nativePrometheusAlerts("vault", "vault_core_active")
			openbaoContent = nativePrometheusAlerts("openbao", "openbao_core_active")
		}
		writeRepositoryTestFile(
			t,
			root,
			filepath.ToSlash(filepath.Join("generated/prometheus", fileName)),
			vaultContent,
		)
		writeRepositoryTestFile(
			t,
			root,
			filepath.ToSlash(filepath.Join("generated/prometheus/vault-prefix", fileName)),
			vaultContent,
		)
		writeRepositoryTestFile(
			t,
			root,
			filepath.ToSlash(filepath.Join("generated/prometheus/openbao-prefix", fileName)),
			openbaoContent,
		)

		writeRepositoryTestFile(
			t,
			root,
			filepath.ToSlash(filepath.Join("generated/prometheusrules", fileName)),
			prometheusRuleObject("openbao-test", vaultContent),
		)
		writeRepositoryTestFile(
			t,
			root,
			filepath.ToSlash(filepath.Join("generated/prometheusrules/vault-prefix", fileName)),
			prometheusRuleObject("openbao-test", vaultContent),
		)
		writeRepositoryTestFile(
			t,
			root,
			filepath.ToSlash(filepath.Join("generated/prometheusrules/openbao-prefix", fileName)),
			prometheusRuleObject("openbao-test", openbaoContent),
		)
	}

	lokiFiles := []string{
		"openbao-alerts.yaml",
		"openbao-warning-alerts.yaml",
		"openbao-security-alerts.yaml",
	}
	for _, fileName := range lokiFiles {
		vaultContent := lokiEmptyRules()
		openbaoContent := lokiEmptyRules()
		if fileName == "openbao-alerts.yaml" {
			vaultContent = lokiAlerts("vault")
			openbaoContent = lokiAlerts("openbao")
		}
		writeRepositoryTestFile(t, root, filepath.ToSlash(filepath.Join("generated/loki", fileName)), vaultContent)
		writeRepositoryTestFile(
			t,
			root,
			filepath.ToSlash(filepath.Join("generated/loki/vault-prefix", fileName)),
			vaultContent,
		)
		writeRepositoryTestFile(
			t,
			root,
			filepath.ToSlash(filepath.Join("generated/loki/openbao-prefix", fileName)),
			openbaoContent,
		)
	}
}

func nativeRecordingRules(prefix, metric string) string {
	return `groups:
  - name: openbao.recording
    rules:
      - record: openbao:core_active:sum
        expr: sum(` + metric + `)
        labels:
          source_prefix: ` + prefix + `
`
}

func nativePrometheusAlerts(prefix, metric string) string {
	return `groups:
  - name: openbao.alerts
    rules:
      - alert: OpenBaoNoActiveNode
        expr: sum(` + metric + `) == 0
        labels:
          source_prefix: ` + prefix + `
`
}

func nativeEmptyRules() string {
	return `groups:
  - name: openbao.empty
    rules: []
`
}

func prometheusRuleObject(name, nativeRules string) string {
	return `apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: ` + name + `
spec:
` + indentYAML(nativeRules, "  ")
}

func lokiAlerts(prefix string) string {
	return `apiVersion: openbao.observability/v1alpha1
kind: LokiAlertRules
metadata:
  name: openbao-loki-alerts
spec:
  groups:
    - name: openbao.loki.alerts
      rules:
        - alert: OpenBaoAuditCanaryMissing
          expr: >-
            absent_over_time({log_stream="openbao.audit"} | json request_path="request.path" |
            request_path="secret/data/observability/audit-canary" [15m])
          labels:
            source_prefix: ` + prefix + `
`
}

func lokiEmptyRules() string {
	return `apiVersion: openbao.observability/v1alpha1
kind: LokiAlertRules
metadata:
  name: openbao-loki-empty
spec:
  groups:
    - name: openbao.loki.empty
      rules: []
`
}

func indentYAML(value, prefix string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
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
- [Audit canary missing](./runbooks/audit-canary-missing.md)
`
}
