package dashboards

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

type GenerateOptions struct {
	ContractPath string
	OutputPath   string
}

type grafanaDashboard struct {
	Annotations          grafanaAnnotations `json:"annotations"`
	Editable             bool               `json:"editable"`
	FiscalYearStartMonth int                `json:"fiscalYearStartMonth"`
	GraphTooltip         int                `json:"graphTooltip"`
	ID                   *int               `json:"id"`
	Links                []any              `json:"links"`
	Panels               []grafanaPanel     `json:"panels"`
	Refresh              string             `json:"refresh"`
	SchemaVersion        int                `json:"schemaVersion"`
	Tags                 []string           `json:"tags"`
	Templating           grafanaTemplating  `json:"templating"`
	Time                 grafanaTimeRange   `json:"time"`
	Timezone             string             `json:"timezone"`
	Title                string             `json:"title"`
	UID                  string             `json:"uid"`
	Version              int                `json:"version"`
}

type grafanaAnnotations struct {
	List []any `json:"list"`
}

type grafanaTemplating struct {
	List []any `json:"list"`
}

type grafanaTimeRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type grafanaPanel struct {
	Datasource  grafanaDatasourceRef `json:"datasource"`
	Description string               `json:"description,omitempty"`
	FieldConfig grafanaFieldConfig   `json:"fieldConfig"`
	GridPos     grafanaGridPos       `json:"gridPos"`
	ID          int                  `json:"id"`
	Options     map[string]any       `json:"options"`
	Targets     []grafanaTarget      `json:"targets"`
	Title       string               `json:"title"`
	Type        string               `json:"type"`
}

type grafanaDatasourceRef struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

type grafanaFieldConfig struct {
	Defaults  grafanaFieldDefaults `json:"defaults"`
	Overrides []any                `json:"overrides"`
}

type grafanaFieldDefaults struct {
	Color      map[string]string `json:"color,omitempty"`
	Thresholds grafanaThresholds `json:"thresholds,omitempty"`
	Unit       string            `json:"unit,omitempty"`
}

type grafanaThresholds struct {
	Mode  string             `json:"mode"`
	Steps []grafanaThreshold `json:"steps"`
}

type grafanaThreshold struct {
	Color string   `json:"color"`
	Value *float64 `json:"value"`
}

type grafanaGridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

type grafanaTarget struct {
	Datasource   grafanaDatasourceRef `json:"datasource"`
	Expr         string               `json:"expr"`
	Format       string               `json:"format,omitempty"`
	LegendFormat string               `json:"legendFormat,omitempty"`
	QueryType    string               `json:"queryType,omitempty"`
	Range        bool                 `json:"range,omitempty"`
	RefID        string               `json:"refId"`
}

func GenerateGrafanaDashboard(opts GenerateOptions) error {
	opts = opts.withDefaults()

	contract, err := contracts.LoadDashboardContract(opts.ContractPath)
	if err != nil {
		return err
	}
	if err := contract.ValidateExpressions(); err != nil {
		return err
	}

	document := buildGrafanaDashboard(*contract)
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal Grafana dashboard for %s: %w", opts.OutputPath, err)
	}
	content = append(content, '\n')

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory for %s: %w", opts.OutputPath, err)
	}
	if err := os.WriteFile(opts.OutputPath, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", opts.OutputPath, err)
	}

	fmt.Printf("generated Grafana dashboard at %s\n", opts.OutputPath)
	return nil
}

func (o GenerateOptions) withDefaults() GenerateOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "dashboards", "openbao-overview.yaml")
	}
	if o.OutputPath == "" {
		o.OutputPath = filepath.Join("generated", "grafana", "openbao-overview.json")
	}
	return o
}

func buildGrafanaDashboard(contract contracts.DashboardContract) grafanaDashboard {
	panels := make([]grafanaPanel, 0, len(contract.Panels))
	for index, panel := range contract.Panels {
		panels = append(panels, buildGrafanaPanel(contract, panel, index+1))
	}

	return grafanaDashboard{
		Annotations:          grafanaAnnotations{List: []any{}},
		Editable:             true,
		FiscalYearStartMonth: 0,
		GraphTooltip:         0,
		ID:                   nil,
		Links:                []any{},
		Panels:               panels,
		Refresh:              contract.Refresh,
		SchemaVersion:        41,
		Tags:                 contract.Tags,
		Templating:           grafanaTemplating{List: []any{}},
		Time: grafanaTimeRange{
			From: contract.TimeRange.From,
			To:   contract.TimeRange.To,
		},
		Timezone: "browser",
		Title:    contract.Title,
		UID:      contract.UID,
		Version:  1,
	}
}

func buildGrafanaPanel(contract contracts.DashboardContract, panel contracts.DashboardPanel, numericID int) grafanaPanel {
	datasource := datasourceRef(contract, panel.Datasource)
	return grafanaPanel{
		Datasource:  datasource,
		Description: panel.Description,
		FieldConfig: fieldConfig(panel.Unit),
		GridPos: grafanaGridPos{
			H: panel.Grid.H,
			W: panel.Grid.W,
			X: panel.Grid.X,
			Y: panel.Grid.Y,
		},
		ID:      numericID,
		Options: panelOptions(panel.Type),
		Targets: []grafanaTarget{
			target(panel, datasource, "A"),
		},
		Title: panel.Title,
		Type:  panel.Type,
	}
}

func datasourceRef(contract contracts.DashboardContract, name string) grafanaDatasourceRef {
	if name == "logs" {
		return grafanaDatasourceRef{
			Type: contract.Datasources.Logs.Type,
			UID:  contract.Datasources.Logs.UID,
		}
	}
	return grafanaDatasourceRef{
		Type: contract.Datasources.Metrics.Type,
		UID:  contract.Datasources.Metrics.UID,
	}
}

func fieldConfig(unit string) grafanaFieldConfig {
	if unit == "" {
		unit = "none"
	}
	return grafanaFieldConfig{
		Defaults: grafanaFieldDefaults{
			Color: map[string]string{
				"mode": "palette-classic",
			},
			Thresholds: grafanaThresholds{
				Mode: "absolute",
				Steps: []grafanaThreshold{
					{Color: "green"},
				},
			},
			Unit: unit,
		},
		Overrides: []any{},
	}
}

func panelOptions(panelType string) map[string]any {
	switch panelType {
	case "logs":
		return map[string]any{
			"dedupStrategy":      "none",
			"enableLogDetails":   true,
			"prettifyLogMessage": false,
			"showLabels":         false,
			"showTime":           true,
			"sortOrder":          "Descending",
			"wrapLogMessage":     false,
		}
	case "stat":
		return map[string]any{
			"colorMode":   "value",
			"graphMode":   "none",
			"justifyMode": "auto",
			"orientation": "auto",
			"reduceOptions": map[string]any{
				"calcs":  []string{"lastNotNull"},
				"fields": "",
				"values": false,
			},
			"textMode": "auto",
		}
	default:
		return map[string]any{
			"legend": map[string]any{
				"calcs":       []string{},
				"displayMode": "list",
				"placement":   "bottom",
				"showLegend":  true,
			},
			"tooltip": map[string]any{
				"hideZeros": false,
				"mode":      "single",
				"sort":      "none",
			},
		}
	}
}

func target(panel contracts.DashboardPanel, datasource grafanaDatasourceRef, refID string) grafanaTarget {
	result := grafanaTarget{
		Datasource: datasource,
		Expr:       panel.Expression,
		RefID:      refID,
	}
	if panel.Signal == "metrics" {
		result.Format = "time_series"
		result.LegendFormat = panel.Title
		if panel.Legend != "" {
			result.LegendFormat = panel.Legend
		}
		result.Range = true
		return result
	}

	result.QueryType = "range"
	return result
}
