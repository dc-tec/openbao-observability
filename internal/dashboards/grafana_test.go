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
	assertStablePanelIDs(t, document)
	assertStatPresentation(t, document.Panels[0])
	assertMetricTargetModes(t, document)
}

func assertStablePanelIDs(t *testing.T, document grafanaDashboard) {
	t.Helper()

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

func assertStatPresentation(t *testing.T, stat grafanaPanel) {
	t.Helper()

	assertStatTarget(t, stat.Targets[0])
	assertStatOptions(t, stat.Options)
	assertStatFieldConfig(t, stat.FieldConfig.Defaults)
}

func assertStatTarget(t *testing.T, statTarget grafanaTarget) {
	t.Helper()

	if !statTarget.Instant || statTarget.Range == nil || *statTarget.Range {
		t.Fatalf("stat target instant = %t and range = %v, want true and false", statTarget.Instant, statTarget.Range)
	}
}

func assertStatOptions(t *testing.T, options map[string]any) {
	t.Helper()

	if got := options["colorMode"]; got != "background" {
		t.Fatalf("stat colorMode = %v, want background", got)
	}
	if got := options["reduceOptions"].(map[string]any)["calcs"].([]string); len(got) != 1 ||
		got[0] != "lastNotNull" {
		t.Fatalf("stat reductions = %v, want lastNotNull", got)
	}
}

func assertStatFieldConfig(t *testing.T, defaults grafanaFieldDefaults) {
	t.Helper()

	if got := defaults.Color["mode"]; got != "thresholds" {
		t.Fatalf("stat field color mode = %q, want thresholds", got)
	}
	if len(defaults.Mappings) != 2 {
		t.Fatalf("stat mapping count = %d, want 2", len(defaults.Mappings))
	}
	if len(defaults.Thresholds.Steps) != 3 {
		t.Fatalf("stat threshold count = %d, want 3", len(defaults.Thresholds.Steps))
	}
	assertStatMappings(t, defaults.Mappings)
	if defaults.NoValue != "No data" {
		t.Fatalf("no-value display = %q, want No data", defaults.NoValue)
	}
}

func assertStatMappings(t *testing.T, mappings []grafanaValueMapping) {
	t.Helper()

	valueMapping, ok := mappings[0].Options.(map[string]grafanaValueMappingResult)
	if !ok {
		t.Fatalf("value mapping options have type %T", mappings[0].Options)
	}
	if got := valueMapping["0"]; got.Text != "Down" || got.Color != "red" || got.Index != 0 {
		t.Fatalf("zero value mapping = %#v", got)
	}
	special, ok := mappings[1].Options.(grafanaSpecialValueMappingOptions)
	if !ok {
		t.Fatalf("special mapping options have type %T", mappings[1].Options)
	}
	if special.Match != "null+nan" || special.Result.Text != "No data" || special.Result.Color != "gray" {
		t.Fatalf("no-data mapping = %#v", special)
	}
}

func assertMetricTargetModes(t *testing.T, document grafanaDashboard) {
	t.Helper()

	seriesTarget := document.Panels[1].Targets[0]
	if seriesTarget.Instant || seriesTarget.Range == nil || !*seriesTarget.Range {
		t.Fatalf(
			"time-series target instant = %t and range = %v, want false and true",
			seriesTarget.Instant,
			seriesTarget.Range,
		)
	}
}

func TestStatPresentationDefaultsAreNeutral(t *testing.T) {
	panel := contracts.DashboardPanel{Type: "stat"}
	options := panelOptions(panel)
	if got := options["colorMode"]; got != "none" {
		t.Fatalf("default stat colorMode = %v, want none", got)
	}
	if got := options["reduceOptions"].(map[string]any)["calcs"].([]string); len(got) != 1 || got[0] != "lastNotNull" {
		t.Fatalf("default stat reductions = %v, want lastNotNull", got)
	}
}

func TestLogStatTargetRemainsRangeQuery(t *testing.T) {
	panel := contracts.DashboardPanel{
		Type:       "stat",
		Signal:     "logs",
		Expression: `sum(count_over_time({log_stream="openbao.audit"}[$__range]))`,
	}
	target := target(panel, grafanaDatasourceRef{Type: "loki", UID: "loki"}, "A")
	if target.Instant || target.Range != nil || target.QueryType != "range" {
		t.Fatalf("log stat target = instant %t, range %v, queryType %q", target.Instant, target.Range, target.QueryType)
	}
}

func dashboardContract() string {
	return `version: v0.1
maturity:
  lifecycle: draft
  evidence:
    - generated-validated
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
        color: green
    colorMode: background
    noData: No data
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
