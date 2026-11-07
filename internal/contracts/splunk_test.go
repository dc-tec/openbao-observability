package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplunkProfileRejectsInvalidBindings(t *testing.T) {
	root := filepath.Join("..", "..")
	source, err := os.ReadFile(filepath.Join(root, "contracts/profiles/splunk-enterprise.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, old, replacement string }{
		{"unknown field", "appID:", "unexpected: value\nappID:"},
		{"version", "version: v0.1", "version: v99"},
		{"unsafe index", "auditIndex: openbao_audit", `auditIndex: 'openbao_audit | delete'`},
		{"unsafe metrics index", "metricsIndex: openbao_metrics", `metricsIndex: '*'`},
		{"unknown metric macro", "`openbao_metric_fresh`", "`openbao_metric_unknown`"},
		{"unknown metrics prefix", "metricsPrefix: vault", "metricsPrefix: other"},
		{"unsafe sourcetype", "auditSourcetype: openbao:audit", `auditSourcetype: '*'`},
		{"unsafe operational index", "operationalIndex: openbao_operational", `operationalIndex: '*'`},
		{"unsafe operational sourcetype", "operationalSourcetype: openbao:operational", `operationalSourcetype: '*'`},
		{"unsafe completed index", "completedIndex: openbao_completed", `completedIndex: '*'`},
		{"unsafe completed sourcetype", "completedSourcetype: openbao:completed", `completedSourcetype: '*'`},
		{"unsafe platform index", "platformIndex: openbao_platform", `platformIndex: '*'`},
		{"unsafe platform sourcetype", "platformSourcetype: openbao:platform", `platformSourcetype: '*'`},
		{"unsafe scope", "scope: probe", "scope: other"},
		{"unsafe lookback", "earliest: \"-30d\"", "earliest: '0 | delete'"},
		{"extra exposed field", "  caller: auth.display_name", "  secret: response.data\n  caller: auth.display_name"},
		{"unsafe field path", "  caller: auth.display_name", `  caller: 'auth.display_name | delete'`},
		{"unknown panel", "sourcePanel: filtered-audit-events", "sourcePanel: nonexistent"},
		{"unsafe source", "source: contracts/dashboards/openbao-audit-investigation.yaml", "source: ../outside.yaml"},
		{"unnormalized search", "`openbao_audit_events`", "index=*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "profile.yaml")
			content := strings.Replace(string(source), tc.old, tc.replacement, 1)
			if content == string(source) {
				t.Fatal("test did not modify source")
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadSplunkProfile(path, root); err == nil {
				t.Fatal("accepted invalid profile")
			}
		})
	}
}

func TestSplunkProfileRejectsMismatchedSignalSource(t *testing.T) {
	root := filepath.Join("..", "..")
	err := validateSplunkDashboard(root, SplunkDashboardBinding{
		ID: "test", Source: "contracts/dashboards/openbao-overview.yaml", Description: "test",
		Panels: []SplunkPanelBinding{{SourcePanel: "active-nodes", Description: "test", Search: "`openbao_audit_events`"}},
	})
	if err == nil {
		t.Fatal("accepted an audit search for a metric panel")
	}
}
