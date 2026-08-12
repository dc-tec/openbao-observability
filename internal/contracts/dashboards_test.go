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
	stat := contract.Panels[0]
	if stat.ColorMode != "background" || stat.NoData != "No data" {
		t.Fatalf("stat presentation = colorMode %q, noData %q", stat.ColorMode, stat.NoData)
	}
	if len(stat.ValueMappings) != 2 || len(stat.Thresholds) != 3 {
		t.Fatalf("stat mappings = %d and thresholds = %d, want 2 and 3", len(stat.ValueMappings), len(stat.Thresholds))
	}
}

func TestLoadDashboardContractRejectsUnknownField(t *testing.T) {
	content := strings.Replace(
		baseDashboardContract(),
		"title: OpenBao overview",
		"title: OpenBao overview\nunexpected: true",
		1,
	)
	path := writeDashboardContract(t, t.TempDir(), content)

	_, err := LoadDashboardContract(path)
	if err == nil {
		t.Fatal("expected unknown dashboard contract field to fail")
	}
	if !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadDashboardContractRejectsUnsupportedVersion(t *testing.T) {
	content := strings.Replace(baseDashboardContract(), "version: v0.1", "version: v9", 1)
	path := writeDashboardContract(t, t.TempDir(), content)

	_, err := LoadDashboardContract(path)
	if err == nil {
		t.Fatal("expected unsupported dashboard contract version to fail")
	}
	if !strings.Contains(err.Error(), `unsupported version "v9"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadDashboardContractRejectsMissingVersion(t *testing.T) {
	content := strings.TrimPrefix(baseDashboardContract(), "version: v0.1\n")
	path := writeDashboardContract(t, t.TempDir(), content)

	_, err := LoadDashboardContract(path)
	if err == nil {
		t.Fatal("expected missing dashboard contract version to fail")
	}
	if !strings.Contains(err.Error(), "missing version") {
		t.Fatalf("unexpected error: %v", err)
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

func TestLoadDashboardContractRejectsInvalidStatPresentation(t *testing.T) {
	tests := []struct {
		name      string
		old       string
		new       string
		errorText string
	}{
		{
			name:      "first threshold value",
			old:       "thresholds:\n      - color: red",
			new:       "thresholds:\n      - value: 0\n        color: red",
			errorText: "first threshold must not have a value",
		},
		{
			name:      "threshold order",
			old:       "- value: 2\n        color: red",
			new:       "- value: 0\n        color: red",
			errorText: "threshold values must be strictly increasing",
		},
		{
			name:      "threshold color",
			old:       "- color: red",
			new:       "- color: vermilion",
			errorText: "invalid color",
		},
		{
			name:      "mapping value",
			old:       `value: "0"`,
			new:       `value: ""`,
			errorText: "mapping without a value",
		},
		{
			name:      "mapping text",
			old:       "text: Down",
			new:       `text: ""`,
			errorText: "has no text",
		},
		{
			name:      "mapping duplicate",
			old:       `value: "1"`,
			new:       `value: "0"`,
			errorText: "duplicate value mapping",
		},
		{
			name:      "mapping color",
			old:       "color: green",
			new:       "color: chartreuse",
			errorText: "invalid color",
		},
		{
			name:      "color mode",
			old:       "colorMode: background",
			new:       "colorMode: rainbow",
			errorText: "unsupported colorMode",
		},
		{
			name:      "no data display",
			old:       "noData: No data",
			new:       `noData: "   "`,
			errorText: "empty noData display",
		},
		{
			name: "presentation on logs panel",
			old:  "type: logs\n    signal: logs",
			new: "type: logs\n    colorMode: value\n" +
				"    signal: logs",
			errorText: "uses stat presentation fields",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract := strings.Replace(baseDashboardContract(), test.old, test.new, 1)
			path := writeDashboardContract(t, t.TempDir(), contract)
			_, err := LoadDashboardContract(path)
			if err == nil {
				t.Fatal("expected invalid stat presentation to fail")
			}
			if !strings.Contains(err.Error(), test.errorText) {
				t.Fatalf("error = %v, want text %q", err, test.errorText)
			}
		})
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
    valueMappings:
      - value: "0"
        text: Down
        color: red
      - value: "1"
        text: Up
        color: green
    thresholds:
      - color: red
      - value: 1
        color: yellow
      - value: 2
        color: red
    colorMode: background
    noData: No data
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
