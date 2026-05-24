package dashboards

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

func TestGenerateGrafanaDashboard(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "dashboard.yaml")
	outputPath := filepath.Join(dir, "openbao-overview.json")

	if err := os.WriteFile(contractPath, []byte(dashboardContract()), 0o644); err != nil {
		t.Fatalf("write dashboard contract: %v", err)
	}

	err := GenerateGrafanaDashboard(GenerateOptions{
		ContractPath: contractPath,
		OutputPath:   outputPath,
	})
	if err != nil {
		t.Fatalf("GenerateGrafanaDashboard returned error: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read dashboard output: %v", err)
	}

	var document map[string]any
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("generated dashboard is not valid JSON: %v", err)
	}

	if document["uid"] != "openbao-overview" {
		t.Fatalf("dashboard uid = %v, want openbao-overview", document["uid"])
	}

	text := string(content)
	for _, fragment := range []string{
		`"title": "OpenBao overview"`,
		`"uid": "prometheus"`,
		`"uid": "loki"`,
		`"expr": "openbao:core_active:sum"`,
		`"expr": "openbao:autopilot_node_healthy:min"`,
		`"legendFormat": "{{node_id}}"`,
		`"name": "request_id"`,
		`"type": "textbox"`,
		`"expr": "{log_stream=\"openbao.audit\"} | json request_id=\"request.id\" | request_id=~\"${request_id:raw}\""`,
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("generated dashboard missing %q:\n%s", fragment, text)
		}
	}
}

func TestBuildGrafanaDashboardUsesStablePanelIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard.yaml")
	if err := os.WriteFile(path, []byte(dashboardContract()), 0o644); err != nil {
		t.Fatalf("write dashboard contract: %v", err)
	}
	contract, err := contracts.LoadDashboardContract(path)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}

	document := buildGrafanaDashboard(*contract)
	if len(document.Panels) != 3 {
		t.Fatalf("panel count = %d, want 3", len(document.Panels))
	}
	if document.Panels[0].ID != 1 || document.Panels[1].ID != 2 || document.Panels[2].ID != 3 {
		t.Fatalf(
			"panel IDs = %d, %d, %d; want 1, 2, 3",
			document.Panels[0].ID,
			document.Panels[1].ID,
			document.Panels[2].ID,
		)
	}
}

func dashboardContract() string {
	return `version: v0.1
status: draft
uid: openbao-overview
title: OpenBao overview
refresh: 30s
tags:
  - openbao
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
  - id: active-nodes
    title: Active nodes
    type: stat
    signal: metrics
    datasource: metrics
    expression: openbao:core_active:sum
    unit: none
    grid:
      x: 0
      y: 0
      w: 6
      h: 4
  - id: autopilot-node-health
    title: Autopilot node health
    type: timeseries
    signal: metrics
    datasource: metrics
    expression: openbao:autopilot_node_healthy:min
    legend: "{{node_id}}"
    unit: none
    grid:
      x: 0
      y: 4
      w: 12
      h: 8
  - id: audit-stream
    title: Audit stream
    type: logs
    signal: logs
    datasource: logs
    expression: '{log_stream="openbao.audit"} | json request_id="request.id" | request_id=~"${request_id:raw}"'
    grid:
      x: 0
      y: 12
      w: 12
      h: 8
`
}
