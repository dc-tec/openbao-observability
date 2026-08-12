package dashboards

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

const (
	dashboardPanelTypeStat = "stat"
	grafanaValueNone       = "none"
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
	List []grafanaVariable `json:"list"`
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

type grafanaVariable struct {
	Current     grafanaVariableCurrent  `json:"current"`
	Hide        int                     `json:"hide"`
	Label       string                  `json:"label,omitempty"`
	Name        string                  `json:"name"`
	Options     []grafanaVariableOption `json:"options"`
	Query       string                  `json:"query"`
	SkipURLSync bool                    `json:"skipUrlSync"`
	Type        string                  `json:"type"`
}

type grafanaVariableCurrent struct {
	Selected bool   `json:"selected"`
	Text     string `json:"text"`
	Value    string `json:"value"`
}

type grafanaVariableOption struct {
	Selected bool   `json:"selected"`
	Text     string `json:"text"`
	Value    string `json:"value"`
}

type grafanaFieldConfig struct {
	Defaults  grafanaFieldDefaults `json:"defaults"`
	Overrides []any                `json:"overrides"`
}

type grafanaFieldDefaults struct {
	Color      map[string]string     `json:"color,omitempty"`
	Mappings   []grafanaValueMapping `json:"mappings,omitempty"`
	NoValue    string                `json:"noValue,omitempty"`
	Thresholds grafanaThresholds     `json:"thresholds,omitempty"`
	Unit       string                `json:"unit,omitempty"`
}

type grafanaValueMapping struct {
	Options any    `json:"options"`
	Type    string `json:"type"`
}

type grafanaValueMappingResult struct {
	Color string `json:"color,omitempty"`
	Index int    `json:"index"`
	Text  string `json:"text"`
}

type grafanaSpecialValueMappingOptions struct {
	Match  string                    `json:"match"`
	Result grafanaValueMappingResult `json:"result"`
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
	Instant      bool                 `json:"instant,omitempty"`
	LegendFormat string               `json:"legendFormat,omitempty"`
	QueryType    string               `json:"queryType,omitempty"`
	Range        *bool                `json:"range,omitempty"`
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
		Templating:           grafanaTemplating{List: buildGrafanaVariables(contract.Variables)},
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

func buildGrafanaVariables(variables []contracts.DashboardVariable) []grafanaVariable {
	grafanaVariables := make([]grafanaVariable, 0, len(variables))
	for _, variable := range variables {
		label := variable.Label
		if label == "" {
			label = variable.Name
		}

		grafanaVariable := grafanaVariable{
			Current: grafanaVariableCurrent{
				Selected: true,
				Text:     variable.Default,
				Value:    variable.Default,
			},
			Hide:        0,
			Label:       label,
			Name:        variable.Name,
			Options:     []grafanaVariableOption{},
			Query:       variable.Default,
			SkipURLSync: false,
			Type:        variable.Type,
		}

		if variable.Type == dashboardVariableCustom {
			grafanaVariable.Query = strings.Join(variable.Options, ",")
			grafanaVariable.Options = make([]grafanaVariableOption, 0, len(variable.Options))
			for _, option := range variable.Options {
				grafanaVariable.Options = append(grafanaVariable.Options, grafanaVariableOption{
					Selected: option == variable.Default,
					Text:     option,
					Value:    option,
				})
			}
		}

		grafanaVariables = append(grafanaVariables, grafanaVariable)
	}
	return grafanaVariables
}

func buildGrafanaPanel(
	contract contracts.DashboardContract,
	panel contracts.DashboardPanel,
	numericID int,
) grafanaPanel {
	datasource := datasourceRef(contract, panel.Datasource)
	return grafanaPanel{
		Datasource:  datasource,
		Description: panel.Description,
		FieldConfig: fieldConfig(panel),
		GridPos: grafanaGridPos{
			H: panel.Grid.H,
			W: panel.Grid.W,
			X: panel.Grid.X,
			Y: panel.Grid.Y,
		},
		ID:      numericID,
		Options: panelOptions(panel),
		Targets: []grafanaTarget{
			target(panel, datasource, "A"),
		},
		Title: panel.Title,
		Type:  panel.Type,
	}
}

func (d grafanaDashboard) variableDefaults() map[string]string {
	defaults := map[string]string{}
	for _, variable := range d.Templating.List {
		value := variable.Current.Value
		if value == "" {
			value = variable.Current.Text
		}
		if value != "" {
			defaults[variable.Name] = value
		}
	}
	return defaults
}

func datasourceRef(contract contracts.DashboardContract, name string) grafanaDatasourceRef {
	if name == dashboardSignalLogs {
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

func fieldConfig(panel contracts.DashboardPanel) grafanaFieldConfig {
	unit := panel.Unit
	if unit == "" {
		unit = grafanaValueNone
	}
	colorMode := "palette-classic"
	if panel.Type == dashboardPanelTypeStat && len(panel.Thresholds) > 0 {
		colorMode = "thresholds"
	}

	thresholds := []grafanaThreshold{{Color: "green"}}
	if len(panel.Thresholds) > 0 {
		thresholds = make([]grafanaThreshold, 0, len(panel.Thresholds))
		for _, threshold := range panel.Thresholds {
			thresholds = append(thresholds, grafanaThreshold{
				Color: threshold.Color,
				Value: threshold.Value,
			})
		}
	}
	return grafanaFieldConfig{
		Defaults: grafanaFieldDefaults{
			Color: map[string]string{
				"mode": colorMode,
			},
			Mappings: valueMappings(panel),
			NoValue:  panel.NoData,
			Thresholds: grafanaThresholds{
				Mode:  "absolute",
				Steps: thresholds,
			},
			Unit: unit,
		},
		Overrides: []any{},
	}
}

func valueMappings(panel contracts.DashboardPanel) []grafanaValueMapping {
	mappings := make([]grafanaValueMapping, 0, 2)
	if len(panel.ValueMappings) > 0 {
		options := make(map[string]grafanaValueMappingResult, len(panel.ValueMappings))
		for index, mapping := range panel.ValueMappings {
			options[mapping.Value] = grafanaValueMappingResult{
				Color: mapping.Color,
				Index: index,
				Text:  mapping.Text,
			}
		}
		mappings = append(mappings, grafanaValueMapping{Options: options, Type: "value"})
	}
	if panel.NoData != "" {
		mappings = append(mappings, grafanaValueMapping{
			Options: grafanaSpecialValueMappingOptions{
				Match: "null+nan",
				Result: grafanaValueMappingResult{
					Color: "gray",
					Index: len(panel.ValueMappings),
					Text:  panel.NoData,
				},
			},
			Type: "special",
		})
	}
	return mappings
}

func panelOptions(panel contracts.DashboardPanel) map[string]any {
	switch panel.Type {
	case dashboardSignalLogs:
		return map[string]any{
			"dedupStrategy":      grafanaValueNone,
			"enableLogDetails":   true,
			"prettifyLogMessage": false,
			"showLabels":         false,
			"showTime":           true,
			"sortOrder":          "Descending",
			"wrapLogMessage":     false,
		}
	case dashboardPanelTypeStat:
		colorMode := panel.ColorMode
		if colorMode == "" {
			colorMode = grafanaValueNone
			if len(panel.Thresholds) > 0 || hasColoredValueMapping(panel.ValueMappings) {
				colorMode = "value"
			}
		}
		return map[string]any{
			"colorMode":   colorMode,
			"graphMode":   grafanaValueNone,
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
				"sort":      grafanaValueNone,
			},
		}
	}
}

func hasColoredValueMapping(mappings []contracts.DashboardValueMapping) bool {
	for _, mapping := range mappings {
		if mapping.Color != "" {
			return true
		}
	}
	return false
}

func target(panel contracts.DashboardPanel, datasource grafanaDatasourceRef, refID string) grafanaTarget {
	result := grafanaTarget{
		Datasource: datasource,
		Expr:       panel.Expression,
		RefID:      refID,
	}
	if panel.Signal == dashboardSignalMetrics {
		result.Format = "time_series"
		result.LegendFormat = panel.Title
		if panel.Legend != "" {
			result.LegendFormat = panel.Legend
		}
		if panel.Type == dashboardPanelTypeStat {
			result.Instant = true
			result.Range = boolPointer(false)
			return result
		}
		result.Range = boolPointer(true)
		return result
	}

	result.QueryType = "range"
	return result
}

func boolPointer(value bool) *bool {
	return &value
}
