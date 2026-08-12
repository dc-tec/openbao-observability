package contracts

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

type DashboardContract struct {
	Version     string               `yaml:"version"`
	Maturity    Maturity             `yaml:"maturity"`
	UID         string               `yaml:"uid"`
	Title       string               `yaml:"title"`
	Refresh     string               `yaml:"refresh"`
	Tags        []string             `yaml:"tags"`
	TimeRange   DashboardTimeRange   `yaml:"timeRange"`
	Datasources DashboardDatasources `yaml:"datasources"`
	Variables   []DashboardVariable  `yaml:"variables"`
	Panels      []DashboardPanel     `yaml:"panels"`
}

type DashboardTimeRange struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type DashboardDatasources struct {
	Metrics DashboardDatasource `yaml:"metrics"`
	Logs    DashboardDatasource `yaml:"logs"`
}

type DashboardDatasource struct {
	Type string `yaml:"type"`
	UID  string `yaml:"uid"`
}

type DashboardVariable struct {
	Name    string   `yaml:"name"`
	Label   string   `yaml:"label"`
	Type    string   `yaml:"type"`
	Default string   `yaml:"default"`
	Options []string `yaml:"options"`
}

type DashboardPanel struct {
	ID            string                   `yaml:"id"`
	Title         string                   `yaml:"title"`
	Type          string                   `yaml:"type"`
	Signal        string                   `yaml:"signal"`
	Datasource    string                   `yaml:"datasource"`
	Expression    string                   `yaml:"expression"`
	Legend        string                   `yaml:"legend"`
	Unit          string                   `yaml:"unit"`
	Description   string                   `yaml:"description"`
	ValueMappings []DashboardValueMapping  `yaml:"valueMappings"`
	Thresholds    []DashboardThresholdStep `yaml:"thresholds"`
	ColorMode     string                   `yaml:"colorMode"`
	NoData        string                   `yaml:"noData"`
	Grid          DashboardGrid            `yaml:"grid"`
}

type DashboardValueMapping struct {
	Value string `yaml:"value"`
	Text  string `yaml:"text"`
	Color string `yaml:"color"`
}

type DashboardThresholdStep struct {
	Value *float64 `yaml:"value"`
	Color string   `yaml:"color"`
}

type DashboardGrid struct {
	X int `yaml:"x"`
	Y int `yaml:"y"`
	W int `yaml:"w"`
	H int `yaml:"h"`
}

type VerifyDashboardOptions struct {
	ContractPath string
}

var (
	dashboardVariableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	dashboardColorPattern        = regexp.MustCompile(
		`^(#[0-9A-Fa-f]{6}|transparent|text|` +
			`(?:(?:super-light|light|semi-dark|dark)-)?(?:red|orange|yellow|green|blue|purple|gray))$`,
	)
)

func LoadDashboardContract(path string) (*DashboardContract, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dashboard contract %s: %w", path, err)
	}

	var contract DashboardContract
	if err := yaml.Unmarshal(content, &contract); err != nil {
		return nil, fmt.Errorf("parse dashboard contract %s: %w", path, err)
	}

	if err := contract.validateShape(path); err != nil {
		return nil, err
	}

	return &contract, nil
}

func VerifyDashboardContract(opts VerifyDashboardOptions) error {
	opts = opts.withDefaults()

	contract, err := LoadDashboardContract(opts.ContractPath)
	if err != nil {
		return err
	}

	if err := contract.ValidateExpressions(); err != nil {
		return err
	}

	fmt.Printf("dashboard contract verified at %s\n", opts.ContractPath)
	return nil
}

func (o VerifyDashboardOptions) withDefaults() VerifyDashboardOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "dashboards", "openbao-overview.yaml")
	}
	return o
}

func (c DashboardContract) ValidateExpressions() error {
	promQLParser := parser.NewParser(parser.Options{})
	for _, panel := range c.Panels {
		expression := c.ExpressionWithDefaultVariables(panel.Expression)
		switch panel.Signal {
		case dashboardSignalMetrics:
			if _, err := promQLParser.ParseExpr(expression); err != nil {
				return fmt.Errorf("parse PromQL for dashboard panel %s: %w", panel.ID, err)
			}
		case dashboardSignalLogs:
			if !strings.Contains(expression, "{") || !strings.Contains(expression, "}") {
				return fmt.Errorf("log panel %s expression must include a label selector", panel.ID)
			}
		default:
			return fmt.Errorf("dashboard panel %s has unsupported signal %q", panel.ID, panel.Signal)
		}
	}
	return nil
}

func (c DashboardContract) ExpressionWithDefaultVariables(expression string) string {
	return InterpolateDashboardVariables(expression, c.variableDefaults())
}

func InterpolateDashboardVariables(expression string, values map[string]string) string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})

	result := expression
	for _, name := range names {
		value := values[name]
		result = strings.ReplaceAll(result, "${"+name+":raw}", value)
		result = strings.ReplaceAll(result, "${"+name+"}", value)
		result = strings.ReplaceAll(result, "$"+name, value)
	}
	return result
}

func (c DashboardContract) variableDefaults() map[string]string {
	defaults := map[string]string{}
	for _, variable := range c.Variables {
		defaults[variable.Name] = variable.Default
	}
	return defaults
}

func (c DashboardContract) validateShape(path string) error {
	if err := c.validateMetadata(path); err != nil {
		return err
	}
	if err := c.validateVariables(path); err != nil {
		return err
	}
	return c.validatePanels(path)
}

func (c DashboardContract) validateMetadata(path string) error {
	if err := validateMaturity(path, c.Maturity); err != nil {
		return err
	}
	switch {
	case c.UID == "":
		return fmt.Errorf("dashboard contract %s is missing uid", path)
	case c.Title == "":
		return fmt.Errorf("dashboard contract %s is missing title", path)
	case c.TimeRange.From == "" || c.TimeRange.To == "":
		return fmt.Errorf("dashboard contract %s is missing timeRange.from or timeRange.to", path)
	case c.Datasources.Metrics.UID == "" || c.Datasources.Metrics.Type == "":
		return fmt.Errorf("dashboard contract %s is missing metrics datasource", path)
	case c.Datasources.Logs.UID == "" || c.Datasources.Logs.Type == "":
		return fmt.Errorf("dashboard contract %s is missing logs datasource", path)
	case len(c.Panels) == 0:
		return fmt.Errorf("dashboard contract %s has no panels", path)
	default:
		return nil
	}
}

func (c DashboardContract) validateVariables(path string) error {
	seenVariables := map[string]bool{}
	for _, variable := range c.Variables {
		if err := validateDashboardVariable(path, variable, seenVariables); err != nil {
			return err
		}
	}
	return nil
}

func validateDashboardVariable(path string, variable DashboardVariable, seen map[string]bool) error {
	if variable.Name == "" {
		return fmt.Errorf("dashboard contract %s has a variable without a name", path)
	}
	if !dashboardVariableNamePattern.MatchString(variable.Name) {
		return fmt.Errorf("dashboard contract %s has invalid variable name %q", path, variable.Name)
	}
	if seen[variable.Name] {
		return fmt.Errorf("dashboard contract %s has duplicate variable %q", path, variable.Name)
	}
	seen[variable.Name] = true
	if variable.Type == "" {
		return fmt.Errorf("dashboard variable %s is missing type", variable.Name)
	}
	switch variable.Type {
	case dashboardVariableTypeList, dashboardVariableTypeText:
	default:
		return fmt.Errorf("dashboard variable %s has unsupported type %q", variable.Name, variable.Type)
	}
	if variable.Default == "" {
		return fmt.Errorf("dashboard variable %s is missing default", variable.Name)
	}
	if variable.Type != dashboardVariableTypeList {
		return nil
	}
	if len(variable.Options) == 0 {
		return fmt.Errorf("dashboard variable %s has no options", variable.Name)
	}
	if !stringSet(variable.Options)[variable.Default] {
		return fmt.Errorf(
			"dashboard variable %s default %q is not listed in options",
			variable.Name,
			variable.Default,
		)
	}
	return nil
}

func (c DashboardContract) validatePanels(path string) error {
	seen := map[string]bool{}
	for _, panel := range c.Panels {
		if err := validateDashboardPanel(path, panel, seen); err != nil {
			return err
		}
	}
	return nil
}

func validateDashboardPanel(path string, panel DashboardPanel, seen map[string]bool) error {
	if panel.ID == "" {
		return fmt.Errorf("dashboard contract %s has a panel without an id", path)
	}
	if seen[panel.ID] {
		return fmt.Errorf("dashboard contract %s has duplicate panel id %q", path, panel.ID)
	}
	seen[panel.ID] = true
	if err := validateDashboardPanelBasics(panel); err != nil {
		return err
	}
	if err := validateDashboardPanelSignal(panel); err != nil {
		return err
	}
	return validateDashboardPanelPresentation(panel)
}

func validateDashboardPanelBasics(panel DashboardPanel) error {
	if panel.Title == "" {
		return fmt.Errorf("dashboard panel %s is missing title", panel.ID)
	}
	if panel.Type == "" {
		return fmt.Errorf("dashboard panel %s is missing type", panel.ID)
	}
	switch panel.Type {
	case dashboardSignalLogs, dashboardPanelTypeStat, dashboardPanelTypeSeries:
	default:
		return fmt.Errorf("dashboard panel %s has unsupported type %q", panel.ID, panel.Type)
	}
	if panel.Expression == "" {
		return fmt.Errorf("dashboard panel %s is missing expression", panel.ID)
	}
	if panel.Grid.W <= 0 || panel.Grid.H <= 0 {
		return fmt.Errorf("dashboard panel %s has invalid grid size", panel.ID)
	}
	return nil
}

func validateDashboardPanelSignal(panel DashboardPanel) error {
	if panel.Datasource != dashboardSignalMetrics && panel.Datasource != dashboardSignalLogs {
		return fmt.Errorf("dashboard panel %s has unsupported datasource %q", panel.ID, panel.Datasource)
	}
	if panel.Signal == dashboardSignalMetrics && panel.Datasource != dashboardSignalMetrics {
		return fmt.Errorf("dashboard panel %s uses metrics signal with datasource %q", panel.ID, panel.Datasource)
	}
	if panel.Signal == dashboardSignalLogs && panel.Datasource != dashboardSignalLogs {
		return fmt.Errorf("dashboard panel %s uses logs signal with datasource %q", panel.ID, panel.Datasource)
	}
	return nil
}

func validateDashboardPanelPresentation(panel DashboardPanel) error {
	hasStatPresentation := len(panel.ValueMappings) > 0 || len(panel.Thresholds) > 0 ||
		panel.ColorMode != "" || panel.NoData != ""
	if panel.Type != dashboardPanelTypeStat {
		if hasStatPresentation {
			return fmt.Errorf("dashboard panel %s uses stat presentation fields with type %q", panel.ID, panel.Type)
		}
		return nil
	}

	if err := validateDashboardColorMode(panel); err != nil {
		return err
	}
	if err := validateDashboardValueMappings(panel); err != nil {
		return err
	}
	if err := validateDashboardThresholds(panel); err != nil {
		return err
	}
	if panel.NoData != "" && strings.TrimSpace(panel.NoData) == "" {
		return fmt.Errorf("dashboard panel %s has an empty noData display", panel.ID)
	}
	return nil
}

func validateDashboardColorMode(panel DashboardPanel) error {
	if panel.ColorMode == "" {
		return nil
	}
	switch panel.ColorMode {
	case "none", "value", "background", "background_solid":
		return nil
	default:
		return fmt.Errorf("dashboard panel %s has unsupported colorMode %q", panel.ID, panel.ColorMode)
	}
}

func validateDashboardValueMappings(panel DashboardPanel) error {
	seen := map[string]bool{}
	for _, mapping := range panel.ValueMappings {
		if mapping.Value == "" {
			return fmt.Errorf("dashboard panel %s has a value mapping without a value", panel.ID)
		}
		if strings.TrimSpace(mapping.Text) == "" {
			return fmt.Errorf("dashboard panel %s value mapping %q has no text", panel.ID, mapping.Value)
		}
		if seen[mapping.Value] {
			return fmt.Errorf("dashboard panel %s has duplicate value mapping %q", panel.ID, mapping.Value)
		}
		seen[mapping.Value] = true
		if mapping.Color != "" && !dashboardColorPattern.MatchString(mapping.Color) {
			return fmt.Errorf(
				"dashboard panel %s value mapping %q has invalid color %q",
				panel.ID,
				mapping.Value,
				mapping.Color,
			)
		}
	}
	return nil
}

func validateDashboardThresholds(panel DashboardPanel) error {
	if len(panel.Thresholds) == 0 {
		return nil
	}
	for index, threshold := range panel.Thresholds {
		if !dashboardColorPattern.MatchString(threshold.Color) {
			return fmt.Errorf(
				"dashboard panel %s threshold %d has invalid color %q",
				panel.ID,
				index,
				threshold.Color,
			)
		}
		if index == 0 {
			if threshold.Value != nil {
				return fmt.Errorf("dashboard panel %s first threshold must not have a value", panel.ID)
			}
			continue
		}
		if threshold.Value == nil || math.IsNaN(*threshold.Value) || math.IsInf(*threshold.Value, 0) {
			return fmt.Errorf("dashboard panel %s threshold %d must have a finite value", panel.ID, index)
		}
		if index > 1 && *threshold.Value <= *panel.Thresholds[index-1].Value {
			return fmt.Errorf("dashboard panel %s threshold values must be strictly increasing", panel.ID)
		}
	}
	return nil
}
