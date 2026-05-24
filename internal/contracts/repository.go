package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

type VerifyRepositoryOptions struct {
	RepositoryRoot string
}

type generatedDashboard struct {
	Refresh       string                       `json:"refresh"`
	SchemaVersion int                          `json:"schemaVersion"`
	Templating    generatedDashboardTemplating `json:"templating"`
	Time          DashboardTimeRange           `json:"time"`
	Title         string                       `json:"title"`
	UID           string                       `json:"uid"`
	Panels        []generatedDashboardPanel    `json:"panels"`
}

type generatedDashboardTemplating struct {
	List []generatedDashboardVariable `json:"list"`
}

type generatedDashboardVariable struct {
	Current generatedDashboardVariableCurrent  `json:"current"`
	Name    string                             `json:"name"`
	Options []generatedDashboardVariableOption `json:"options"`
	Query   string                             `json:"query"`
	Type    string                             `json:"type"`
}

type generatedDashboardVariableCurrent struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

type generatedDashboardVariableOption struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

type generatedDashboardPanel struct {
	Title      string              `json:"title"`
	Type       string              `json:"type"`
	Datasource generatedDatasource `json:"datasource"`
	GridPos    DashboardGrid       `json:"gridPos"`
	ID         int                 `json:"id"`
	Targets    []generatedTarget   `json:"targets"`
}

type generatedDatasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

type generatedTarget struct {
	Expr       string              `json:"expr"`
	Datasource generatedDatasource `json:"datasource"`
	RefID      string              `json:"refId"`
}

type ruleFile struct {
	Groups []ruleGroup `yaml:"groups"`
	Spec   struct {
		Groups []ruleGroup `yaml:"groups"`
	} `yaml:"spec"`
}

type ruleGroup struct {
	Rules []rule `yaml:"rules"`
}

type rule struct {
	Alert  string            `yaml:"alert"`
	Expr   string            `yaml:"expr"`
	Labels map[string]string `yaml:"labels"`
	Record string            `yaml:"record"`
}

var (
	recordingRuleReferencePattern = regexp.MustCompile(`\bopenbao:[A-Za-z0-9_:]+\b`)
	rawOpenBAOMetricPattern       = regexp.MustCompile(`\bopenbao_[A-Za-z0-9_:]+\b`)
	rawVaultMetricPattern         = regexp.MustCompile(`\bvault_[A-Za-z0-9_:]+\b`)
)

func VerifyRepository(opts VerifyRepositoryOptions) error {
	opts = opts.withDefaults()
	root := opts.RepositoryRoot

	dashboardContractPaths, err := globRequired(root, "contracts/dashboards/*.yaml")
	if err != nil {
		return err
	}
	generatedDashboardPaths, err := globRequired(root, "generated/grafana/*.json")
	if err != nil {
		return err
	}
	alertContractPaths, err := globRequired(root, "contracts/alerts/*.yaml")
	if err != nil {
		return err
	}
	streamContract, err := LoadStreamContract(filepath.Join(root, "contracts", "streams", "log-streams.yaml"))
	if err != nil {
		return err
	}

	if err := verifyMakefileList(root, "DASHBOARD_CONTRACTS", dashboardContractPaths); err != nil {
		return err
	}
	if err := verifyMakefileList(root, "GENERATED_DASHBOARDS", generatedDashboardPaths); err != nil {
		return err
	}

	recordingRules, err := loadRecordingRuleNames(filepath.Join(root, "generated", "prometheus", "openbao-recording-rules.yaml"))
	if err != nil {
		return err
	}
	if err := verifyGeneratedDashboards(root, dashboardContractPaths, generatedDashboardPaths, recordingRules, streamContract); err != nil {
		return err
	}
	if err := verifyGeneratedAlerts(root, alertContractPaths, streamContract); err != nil {
		return err
	}
	if err := verifyPrefixVariants(root, streamContract); err != nil {
		return err
	}
	if err := verifyRunbookIndex(root, alertContractPaths); err != nil {
		return err
	}

	fmt.Printf("repository contracts verified at %s\n", root)
	return nil
}

func (o VerifyRepositoryOptions) withDefaults() VerifyRepositoryOptions {
	if o.RepositoryRoot == "" {
		o.RepositoryRoot = "."
	}
	o.RepositoryRoot = filepath.Clean(o.RepositoryRoot)
	return o
}

func verifyGeneratedDashboards(root string, contractPaths, generatedPaths []string, recordingRules map[string]bool, streamContract *StreamContract) error {
	contractsByUID := map[string]dashboardContractFile{}
	for _, contractPath := range contractPaths {
		contract, err := LoadDashboardContract(contractPath)
		if err != nil {
			return err
		}
		if err := contract.ValidateExpressions(); err != nil {
			return fmt.Errorf("validate dashboard expressions in %s: %w", relPath(root, contractPath), err)
		}
		if err := validateDashboardContractLabelSafety(relPath(root, contractPath), contract, streamContract.ForbiddenLabels); err != nil {
			return err
		}
		if previous, ok := contractsByUID[contract.UID]; ok {
			return fmt.Errorf("dashboard uid %q is used by both %s and %s", contract.UID, relPath(root, previous.Path), relPath(root, contractPath))
		}
		contractsByUID[contract.UID] = dashboardContractFile{Path: contractPath, Contract: contract}
	}

	generatedByUID := map[string]string{}
	for _, generatedPath := range generatedPaths {
		dashboard, err := loadGeneratedDashboard(generatedPath)
		if err != nil {
			return err
		}
		if err := validateGeneratedDashboardSchema(root, generatedPath, dashboard, streamContract); err != nil {
			return err
		}
		if previous, ok := generatedByUID[dashboard.UID]; ok {
			return fmt.Errorf("generated dashboard uid %q is used by both %s and %s", dashboard.UID, relPath(root, previous), relPath(root, generatedPath))
		}
		if filepath.Base(generatedPath) != dashboard.UID+".json" {
			return fmt.Errorf("generated dashboard %s filename must match uid %q", relPath(root, generatedPath), dashboard.UID)
		}
		generatedByUID[dashboard.UID] = generatedPath
	}

	for uid, contractFile := range contractsByUID {
		generatedPath, ok := generatedByUID[uid]
		if !ok {
			return fmt.Errorf("dashboard contract %s has no generated dashboard generated/grafana/%s.json", relPath(root, contractFile.Path), uid)
		}
		dashboard, err := loadGeneratedDashboard(generatedPath)
		if err != nil {
			return err
		}
		if err := verifyGeneratedDashboard(root, contractFile.Path, contractFile.Contract, generatedPath, dashboard, recordingRules, streamContract); err != nil {
			return err
		}
	}

	for uid, generatedPath := range generatedByUID {
		if _, ok := contractsByUID[uid]; !ok {
			return fmt.Errorf("generated dashboard %s has no source dashboard contract with uid %q", relPath(root, generatedPath), uid)
		}
	}

	return nil
}

type dashboardContractFile struct {
	Path     string
	Contract *DashboardContract
}

func verifyGeneratedDashboard(root, contractPath string, contract *DashboardContract, generatedPath string, dashboard *generatedDashboard, recordingRules map[string]bool, streamContract *StreamContract) error {
	if dashboard.Title != contract.Title {
		return fmt.Errorf("generated dashboard %s title %q does not match contract %s title %q", relPath(root, generatedPath), dashboard.Title, relPath(root, contractPath), contract.Title)
	}
	if dashboard.Refresh != contract.Refresh {
		return fmt.Errorf("generated dashboard %s refresh %q does not match contract %s refresh %q", relPath(root, generatedPath), dashboard.Refresh, relPath(root, contractPath), contract.Refresh)
	}
	if dashboard.Time.From != contract.TimeRange.From || dashboard.Time.To != contract.TimeRange.To {
		return fmt.Errorf("generated dashboard %s time range %s..%s does not match contract %s time range %s..%s", relPath(root, generatedPath), dashboard.Time.From, dashboard.Time.To, relPath(root, contractPath), contract.TimeRange.From, contract.TimeRange.To)
	}
	if err := verifyGeneratedDashboardVariables(root, contractPath, contract, generatedPath, dashboard); err != nil {
		return err
	}
	if len(dashboard.Panels) != len(contract.Panels) {
		return fmt.Errorf("generated dashboard %s has %d panels, want %d from %s", relPath(root, generatedPath), len(dashboard.Panels), len(contract.Panels), relPath(root, contractPath))
	}

	for i, panel := range contract.Panels {
		generatedPanel := dashboard.Panels[i]
		if generatedPanel.Title != panel.Title {
			return fmt.Errorf("generated dashboard %s panel %d title %q does not match contract panel %s title %q", relPath(root, generatedPath), i+1, generatedPanel.Title, panel.ID, panel.Title)
		}
		if generatedPanel.Type != panel.Type {
			return fmt.Errorf("generated dashboard %s panel %s type %q does not match contract type %q", relPath(root, generatedPath), panel.ID, generatedPanel.Type, panel.Type)
		}
		if generatedPanel.GridPos != panel.Grid {
			return fmt.Errorf("generated dashboard %s panel %s grid does not match contract", relPath(root, generatedPath), panel.ID)
		}

		expectedDatasource, err := contractDatasource(contract, panel.Datasource)
		if err != nil {
			return fmt.Errorf("dashboard contract %s panel %s: %w", relPath(root, contractPath), panel.ID, err)
		}
		if generatedPanel.Datasource.Type != expectedDatasource.Type || generatedPanel.Datasource.UID != expectedDatasource.UID {
			return fmt.Errorf("generated dashboard %s panel %s datasource %s/%s does not match contract %s/%s", relPath(root, generatedPath), panel.ID, generatedPanel.Datasource.Type, generatedPanel.Datasource.UID, expectedDatasource.Type, expectedDatasource.UID)
		}
		if len(generatedPanel.Targets) == 0 {
			return fmt.Errorf("generated dashboard %s panel %s has no targets", relPath(root, generatedPath), panel.ID)
		}
		for _, target := range generatedPanel.Targets {
			if target.Expr != panel.Expression {
				return fmt.Errorf("generated dashboard %s panel %s target expression does not match contract", relPath(root, generatedPath), panel.ID)
			}
			if target.Datasource.Type != expectedDatasource.Type || target.Datasource.UID != expectedDatasource.UID {
				return fmt.Errorf("generated dashboard %s panel %s target datasource %s/%s does not match contract %s/%s", relPath(root, generatedPath), panel.ID, target.Datasource.Type, target.Datasource.UID, expectedDatasource.Type, expectedDatasource.UID)
			}
			switch panel.Signal {
			case dashboardSignalMetrics:
				if err := validatePromQLLabelSafety(contract.ExpressionWithDefaultVariables(target.Expr), streamContract.ForbiddenLabels); err != nil {
					return fmt.Errorf("generated dashboard %s panel %s target labels: %w", relPath(root, generatedPath), panel.ID, err)
				}
			case dashboardSignalLogs:
				if err := streamContract.ValidateLogExpression(target.Expr); err != nil {
					return fmt.Errorf("generated dashboard %s panel %s target labels: %w", relPath(root, generatedPath), panel.ID, err)
				}
			}
		}
		if panel.Signal == dashboardSignalMetrics {
			for _, ruleName := range recordingRuleReferences(panel.Expression) {
				if !recordingRules[ruleName] {
					return fmt.Errorf("dashboard contract %s panel %s references recording rule %s, but it is not generated", relPath(root, contractPath), panel.ID, ruleName)
				}
			}
		}
	}

	return nil
}

func verifyGeneratedDashboardVariables(root, contractPath string, contract *DashboardContract, generatedPath string, dashboard *generatedDashboard) error {
	if len(dashboard.Templating.List) != len(contract.Variables) {
		return fmt.Errorf("generated dashboard %s has %d variables, want %d from %s", relPath(root, generatedPath), len(dashboard.Templating.List), len(contract.Variables), relPath(root, contractPath))
	}
	for i, variable := range contract.Variables {
		generatedVariable := dashboard.Templating.List[i]
		if generatedVariable.Name != variable.Name {
			return fmt.Errorf("generated dashboard %s variable %d name %q does not match contract variable %q", relPath(root, generatedPath), i+1, generatedVariable.Name, variable.Name)
		}
		if generatedVariable.Type != variable.Type {
			return fmt.Errorf("generated dashboard %s variable %s type %q does not match contract type %q", relPath(root, generatedPath), variable.Name, generatedVariable.Type, variable.Type)
		}
		if generatedVariable.Current.Value != variable.Default {
			return fmt.Errorf("generated dashboard %s variable %s default %q does not match contract default %q", relPath(root, generatedPath), variable.Name, generatedVariable.Current.Value, variable.Default)
		}
	}
	return nil
}

func contractDatasource(contract *DashboardContract, name string) (DashboardDatasource, error) {
	switch name {
	case dashboardSignalMetrics:
		return contract.Datasources.Metrics, nil
	case dashboardSignalLogs:
		return contract.Datasources.Logs, nil
	default:
		return DashboardDatasource{}, fmt.Errorf("unsupported datasource %q", name)
	}
}

func verifyGeneratedAlerts(root string, alertContractPaths []string, streamContract *StreamContract) error {
	expectedPrometheusAlerts := map[string]string{}
	expectedLokiAlerts := map[string]string{}
	for _, contractPath := range alertContractPaths {
		contract, err := LoadAlertContract(contractPath)
		if err != nil {
			return err
		}
		if err := contract.ValidateExpressions(contract.DefaultSourcePrefix()); err != nil {
			return fmt.Errorf("validate alert expressions in %s: %w", relPath(root, contractPath), err)
		}
		if err := contract.ValidateRunbooks(root); err != nil {
			return err
		}
		for _, alert := range contract.Alerts {
			if err := validateAlertContractLabelSafety(relPath(root, contractPath), contract, alert, streamContract); err != nil {
				return err
			}
		}
		for _, alert := range contract.Alerts {
			switch alert.Type {
			case alertTypePrometheus:
				if previous, ok := expectedPrometheusAlerts[alert.ID]; ok {
					return fmt.Errorf("prometheus alert %s is defined in both %s and %s", alert.ID, previous, relPath(root, contractPath))
				}
				expectedPrometheusAlerts[alert.ID] = relPath(root, contractPath)
			case alertTypeLoki:
				if previous, ok := expectedLokiAlerts[alert.ID]; ok {
					return fmt.Errorf("loki alert %s is defined in both %s and %s", alert.ID, previous, relPath(root, contractPath))
				}
				expectedLokiAlerts[alert.ID] = relPath(root, contractPath)
			}
		}
	}

	generatedPrometheusAlerts, err := loadGeneratedAlertNames(root, filepath.Join("generated", "prometheus", "*.yaml"), false)
	if err != nil {
		return err
	}
	generatedLokiAlerts, err := loadGeneratedAlertNames(root, filepath.Join("generated", "loki", "*.yaml"), true)
	if err != nil {
		return err
	}
	if err := compareNamedSets("generated Prometheus alerts", generatedPrometheusAlerts, expectedPrometheusAlerts); err != nil {
		return err
	}
	if err := compareNamedSets("generated Loki alerts", generatedLokiAlerts, expectedLokiAlerts); err != nil {
		return err
	}
	if err := verifyGeneratedRuleLabels(root, filepath.Join("generated", "prometheus", "*.yaml"), false, streamContract); err != nil {
		return err
	}
	if err := verifyGeneratedRuleLabels(root, filepath.Join("generated", "loki", "*.yaml"), true, streamContract); err != nil {
		return err
	}
	return nil
}

func validateAlertContractLabelSafety(source string, contract *AlertContract, alert Alert, streamContract *StreamContract) error {
	for label := range alert.Labels {
		if stringSet(streamContract.ForbiddenLabels)[label] {
			return fmt.Errorf("alert %s in %s uses forbidden alert label %q", alert.ID, source, label)
		}
	}
	switch alert.Type {
	case alertTypePrometheus:
		return validatePromQLLabelSafety(contract.RenderExpression(alert.Expression, contract.DefaultSourcePrefix()), streamContract.ForbiddenLabels)
	case alertTypeLoki:
		return streamContract.ValidateLogExpression(alert.Expression)
	default:
		return nil
	}
}

func validateDashboardContractLabelSafety(source string, contract *DashboardContract, forbiddenLabels []string) error {
	for _, panel := range contract.Panels {
		if panel.Signal != dashboardSignalMetrics {
			continue
		}
		if err := validatePromQLLabelSafety(contract.ExpressionWithDefaultVariables(panel.Expression), forbiddenLabels); err != nil {
			return fmt.Errorf("dashboard %s panel %s labels: %w", source, panel.ID, err)
		}
	}
	return nil
}

func validatePromQLLabelSafety(expression string, forbiddenLabels []string) error {
	expr, err := parser.NewParser(parser.Options{}).ParseExpr(expression)
	if err != nil {
		return err
	}
	forbidden := stringSet(forbiddenLabels)
	var foundErr error
	parser.Inspect(expr, func(node parser.Node, _ []parser.Node) error {
		if foundErr != nil {
			return foundErr
		}
		switch n := node.(type) {
		case *parser.VectorSelector:
			for _, matcher := range n.LabelMatchers {
				if forbidden[matcher.Name] {
					foundErr = fmt.Errorf("selector uses forbidden Prometheus label %q", matcher.Name)
					return foundErr
				}
			}
		case *parser.AggregateExpr:
			for _, label := range n.Grouping {
				if forbidden[label] {
					foundErr = fmt.Errorf("grouping uses forbidden Prometheus label %q", label)
					return foundErr
				}
			}
		}
		return nil
	})
	return foundErr
}

func validateGeneratedDashboardSchema(root, generatedPath string, dashboard *generatedDashboard, streamContract *StreamContract) error {
	source := relPath(root, generatedPath)
	if dashboard.UID == "" {
		return fmt.Errorf("generated dashboard %s is missing uid", source)
	}
	if dashboard.Title == "" {
		return fmt.Errorf("generated dashboard %s is missing title", source)
	}
	if dashboard.SchemaVersion <= 0 {
		return fmt.Errorf("generated dashboard %s is missing schemaVersion", source)
	}
	if dashboard.Refresh == "" {
		return fmt.Errorf("generated dashboard %s is missing refresh", source)
	}
	if dashboard.Time.From == "" || dashboard.Time.To == "" {
		return fmt.Errorf("generated dashboard %s is missing time.from or time.to", source)
	}
	if len(dashboard.Panels) == 0 {
		return fmt.Errorf("generated dashboard %s has no panels", source)
	}

	seenVariables := map[string]bool{}
	for _, variable := range dashboard.Templating.List {
		if variable.Name == "" {
			return fmt.Errorf("generated dashboard %s has a variable without a name", source)
		}
		if seenVariables[variable.Name] {
			return fmt.Errorf("generated dashboard %s has duplicate variable %q", source, variable.Name)
		}
		seenVariables[variable.Name] = true
		if variable.Type == "" {
			return fmt.Errorf("generated dashboard %s variable %s is missing type", source, variable.Name)
		}
		if variable.Current.Value == "" && variable.Current.Text == "" {
			return fmt.Errorf("generated dashboard %s variable %s is missing current value", source, variable.Name)
		}
		if variable.Type == dashboardVariableTypeList && len(variable.Options) == 0 {
			return fmt.Errorf("generated dashboard %s custom variable %s has no options", source, variable.Name)
		}
	}

	seenPanels := map[int]bool{}
	for _, panel := range dashboard.Panels {
		if panel.ID <= 0 {
			return fmt.Errorf("generated dashboard %s has panel with invalid id %d", source, panel.ID)
		}
		if seenPanels[panel.ID] {
			return fmt.Errorf("generated dashboard %s has duplicate panel id %d", source, panel.ID)
		}
		seenPanels[panel.ID] = true
		if panel.Title == "" {
			return fmt.Errorf("generated dashboard %s panel %d is missing title", source, panel.ID)
		}
		if panel.Type == "" {
			return fmt.Errorf("generated dashboard %s panel %d is missing type", source, panel.ID)
		}
		if err := validateGeneratedDatasource(source, fmt.Sprintf("panel %d", panel.ID), panel.Datasource); err != nil {
			return err
		}
		if err := validateGeneratedGrid(source, panel.ID, panel.GridPos); err != nil {
			return err
		}
		if len(panel.Targets) == 0 {
			return fmt.Errorf("generated dashboard %s panel %d has no targets", source, panel.ID)
		}
		seenTargets := map[string]bool{}
		for _, target := range panel.Targets {
			if target.RefID == "" {
				return fmt.Errorf("generated dashboard %s panel %d has target without refId", source, panel.ID)
			}
			if seenTargets[target.RefID] {
				return fmt.Errorf("generated dashboard %s panel %d has duplicate target refId %q", source, panel.ID, target.RefID)
			}
			seenTargets[target.RefID] = true
			if target.Expr == "" {
				return fmt.Errorf("generated dashboard %s panel %d target %s is missing expr", source, panel.ID, target.RefID)
			}
			if err := validateGeneratedDatasource(source, fmt.Sprintf("panel %d target %s", panel.ID, target.RefID), target.Datasource); err != nil {
				return err
			}
			expression := target.Expr
			if target.Datasource.Type == alertTypePrometheus {
				expression = InterpolateDashboardVariables(target.Expr, generatedDashboardVariableDefaults(dashboard))
			}
			switch target.Datasource.Type {
			case alertTypePrometheus:
				if err := validatePromQLLabelSafety(expression, streamContract.ForbiddenLabels); err != nil {
					return fmt.Errorf("generated dashboard %s panel %d target %s labels: %w", source, panel.ID, target.RefID, err)
				}
			case alertTypeLoki:
				if err := streamContract.ValidateLogExpression(target.Expr); err != nil {
					return fmt.Errorf("generated dashboard %s panel %d target %s labels: %w", source, panel.ID, target.RefID, err)
				}
			default:
				return fmt.Errorf("generated dashboard %s panel %d target %s has unsupported datasource type %q", source, panel.ID, target.RefID, target.Datasource.Type)
			}
		}
	}
	return nil
}

func validateGeneratedDatasource(source, field string, datasource generatedDatasource) error {
	if datasource.Type == "" || datasource.UID == "" {
		return fmt.Errorf("generated dashboard %s %s is missing datasource type or uid", source, field)
	}
	return nil
}

func generatedDashboardVariableDefaults(dashboard *generatedDashboard) map[string]string {
	defaults := map[string]string{}
	for _, variable := range dashboard.Templating.List {
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

func validateGeneratedGrid(source string, panelID int, grid DashboardGrid) error {
	if grid.W <= 0 || grid.H <= 0 {
		return fmt.Errorf("generated dashboard %s panel %d has invalid grid size", source, panelID)
	}
	if grid.X < 0 || grid.Y < 0 {
		return fmt.Errorf("generated dashboard %s panel %d has invalid grid position", source, panelID)
	}
	if grid.W > 24 || grid.X+grid.W > 24 {
		return fmt.Errorf("generated dashboard %s panel %d grid exceeds 24-column Grafana width", source, panelID)
	}
	return nil
}

func verifyGeneratedRuleLabels(root, pattern string, specGroups bool, streamContract *StreamContract) error {
	paths, err := globRequired(root, filepath.ToSlash(pattern))
	if err != nil {
		return err
	}
	for _, filePath := range paths {
		rules, err := loadRuleFile(filePath)
		if err != nil {
			return err
		}
		for _, group := range ruleGroups(rules, specGroups) {
			for _, rule := range group.Rules {
				if rule.Alert == "" && rule.Record == "" {
					continue
				}
				if err := validateGeneratedRuleLabelSafety(root, filePath, rule, specGroups, streamContract); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateGeneratedRuleLabelSafety(root, filePath string, rule rule, loki bool, streamContract *StreamContract) error {
	for label := range rule.Labels {
		if stringSet(streamContract.ForbiddenLabels)[label] {
			return fmt.Errorf("generated rule %s in %s uses forbidden label %q", generatedRuleName(rule), relPath(root, filePath), label)
		}
	}
	if rule.Expr == "" {
		return nil
	}
	if loki {
		if err := streamContract.ValidateLogExpression(rule.Expr); err != nil {
			return fmt.Errorf("generated Loki rule %s in %s labels: %w", generatedRuleName(rule), relPath(root, filePath), err)
		}
		return nil
	}
	if err := validatePromQLLabelSafety(rule.Expr, streamContract.ForbiddenLabels); err != nil {
		return fmt.Errorf("generated Prometheus rule %s in %s labels: %w", generatedRuleName(rule), relPath(root, filePath), err)
	}
	return nil
}

func verifyPrefixVariants(root string, streamContract *StreamContract) error {
	artifacts := []struct {
		dir       string
		fileNames []string
		loki      bool
		spec      bool
	}{
		{
			dir: "generated/prometheus",
			fileNames: []string{
				"openbao-recording-rules.yaml",
				"openbao-alerts.yaml",
				"openbao-warning-alerts.yaml",
				"openbao-security-alerts.yaml",
			},
		},
		{
			dir:  "generated/prometheusrules",
			spec: true,
			fileNames: []string{
				"openbao-recording-rules.yaml",
				"openbao-alerts.yaml",
				"openbao-warning-alerts.yaml",
				"openbao-security-alerts.yaml",
			},
		},
		{
			dir:  "generated/loki",
			loki: true,
			spec: true,
			fileNames: []string{
				"openbao-alerts.yaml",
				"openbao-warning-alerts.yaml",
				"openbao-security-alerts.yaml",
			},
		},
	}

	for _, artifact := range artifacts {
		for _, fileName := range artifact.fileNames {
			defaultPath := filepath.Join(root, filepath.FromSlash(artifact.dir), fileName)
			vaultPath := filepath.Join(root, filepath.FromSlash(artifact.dir), "vault-prefix", fileName)
			openbaoPath := filepath.Join(root, filepath.FromSlash(artifact.dir), "openbao-prefix", fileName)

			if err := verifySameFile(root, defaultPath, vaultPath); err != nil {
				return err
			}
			if err := verifyRuleFileSourcePrefix(root, defaultPath, artifact.spec, artifact.loki, defaultSourcePrefix, streamContract); err != nil {
				return err
			}
			if err := verifyRuleFileSourcePrefix(root, vaultPath, artifact.spec, artifact.loki, defaultSourcePrefix, streamContract); err != nil {
				return err
			}
			if err := verifyRuleFileSourcePrefix(root, openbaoPath, artifact.spec, artifact.loki, "openbao", streamContract); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifySameFile(root, leftPath, rightPath string) error {
	left, err := os.ReadFile(leftPath)
	if err != nil {
		return fmt.Errorf("read generated artifact %s: %w", relPath(root, leftPath), err)
	}
	right, err := os.ReadFile(rightPath)
	if err != nil {
		return fmt.Errorf("read generated artifact %s: %w", relPath(root, rightPath), err)
	}
	if !bytes.Equal(left, right) {
		return fmt.Errorf("generated default artifact %s must match vault-prefix artifact %s", relPath(root, leftPath), relPath(root, rightPath))
	}
	return nil
}

func verifyRuleFileSourcePrefix(root, filePath string, specGroups, loki bool, expectedPrefix string, streamContract *StreamContract) error {
	rules, err := loadRuleFile(filePath)
	if err != nil {
		return err
	}
	for _, group := range ruleGroups(rules, specGroups) {
		for _, rule := range group.Rules {
			if rule.Alert == "" && rule.Record == "" {
				continue
			}
			if got := rule.Labels["source_prefix"]; got != expectedPrefix {
				return fmt.Errorf("generated rule %s in %s has source_prefix %q, want %q", generatedRuleName(rule), relPath(root, filePath), got, expectedPrefix)
			}
			if err := validateGeneratedRuleLabelSafety(root, filePath, rule, loki, streamContract); err != nil {
				return err
			}
			if expectedPrefix == "openbao" && rawVaultMetricPattern.MatchString(rule.Expr) {
				return fmt.Errorf("generated rule %s in %s uses vault_* metric under openbao-prefix profile", generatedRuleName(rule), relPath(root, filePath))
			}
			if expectedPrefix == defaultSourcePrefix && hasDisallowedRawOpenBAOMetric(rule.Expr) {
				return fmt.Errorf("generated rule %s in %s uses openbao_* metric under vault-prefix profile", generatedRuleName(rule), relPath(root, filePath))
			}
		}
	}
	return nil
}

func hasDisallowedRawOpenBAOMetric(expression string) bool {
	for _, match := range rawOpenBAOMetricPattern.FindAllString(expression, -1) {
		if strings.HasPrefix(match, "openbao_audit_archive_") {
			continue
		}
		return true
	}
	return false
}

func ruleGroups(rules *ruleFile, specGroups bool) []ruleGroup {
	if specGroups {
		return rules.Spec.Groups
	}
	return rules.Groups
}

func generatedRuleName(rule rule) string {
	if rule.Alert != "" {
		return rule.Alert
	}
	if rule.Record != "" {
		return rule.Record
	}
	return "<unnamed>"
}

func verifyRunbookIndex(root string, alertContractPaths []string) error {
	indexPath := filepath.Join(root, "docs", "README.md")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("read docs runbook index %s: %w", relPath(root, indexPath), err)
	}
	index := string(content)

	seenRunbooks := map[string]bool{}
	for _, contractPath := range alertContractPaths {
		contract, err := LoadAlertContract(contractPath)
		if err != nil {
			return err
		}
		for _, alert := range contract.Alerts {
			if isExternalRunbook(alert.Runbook) {
				continue
			}
			cleaned := path.Clean(alert.Runbook)
			if !strings.HasPrefix(cleaned, "docs/runbooks/") {
				return fmt.Errorf("alert %s runbook %s must live under docs/runbooks", alert.ID, alert.Runbook)
			}
			if seenRunbooks[cleaned] {
				continue
			}
			seenRunbooks[cleaned] = true
			link := "./" + strings.TrimPrefix(cleaned, "docs/")
			if !strings.Contains(index, "]("+link+")") {
				return fmt.Errorf("runbook %s referenced by alert %s is not linked from %s", cleaned, alert.ID, relPath(root, indexPath))
			}
		}
	}
	return nil
}

func verifyMakefileList(root, variable string, paths []string) error {
	listed, err := makefileCSVVariable(root, variable)
	if err != nil {
		return err
	}
	expected := make([]string, 0, len(paths))
	for _, itemPath := range paths {
		expected = append(expected, relPath(root, itemPath))
	}
	return compareStringSets("Makefile "+variable, listed, expected)
}

func makefileCSVVariable(root, variable string) ([]string, error) {
	makefilePath := filepath.Join(root, "Makefile")
	content, err := os.ReadFile(makefilePath)
	if err != nil {
		return nil, fmt.Errorf("read Makefile: %w", err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, variable) {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(trimmed, variable)), "?=") &&
			!strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(trimmed, variable)), "=") {
			continue
		}
		_, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			break
		}
		return csvList(value), nil
	}

	return nil, fmt.Errorf("makefile variable %s is not defined", variable)
}

func loadGeneratedDashboard(dashboardPath string) (*generatedDashboard, error) {
	content, err := os.ReadFile(dashboardPath)
	if err != nil {
		return nil, fmt.Errorf("read generated dashboard %s: %w", dashboardPath, err)
	}
	var dashboard generatedDashboard
	if err := json.Unmarshal(content, &dashboard); err != nil {
		return nil, fmt.Errorf("parse generated dashboard %s: %w", dashboardPath, err)
	}
	return &dashboard, nil
}

func loadRecordingRuleNames(rulePath string) (map[string]bool, error) {
	rules, err := loadRuleFile(rulePath)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, group := range rules.Groups {
		for _, rule := range group.Rules {
			if rule.Record != "" {
				names[rule.Record] = true
			}
		}
	}
	return names, nil
}

func loadGeneratedAlertNames(root, pattern string, loki bool) (map[string]string, error) {
	paths, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	alerts := map[string]string{}
	for _, filePath := range paths {
		rules, err := loadRuleFile(filePath)
		if err != nil {
			return nil, err
		}
		groups := rules.Groups
		if loki {
			groups = rules.Spec.Groups
		}
		for _, group := range groups {
			for _, rule := range group.Rules {
				if rule.Alert == "" {
					continue
				}
				if previous, ok := alerts[rule.Alert]; ok {
					return nil, fmt.Errorf("generated alert %s is defined in both %s and %s", rule.Alert, previous, relPath(root, filePath))
				}
				alerts[rule.Alert] = relPath(root, filePath)
			}
		}
	}
	return alerts, nil
}

func loadRuleFile(rulePath string) (*ruleFile, error) {
	content, err := os.ReadFile(rulePath)
	if err != nil {
		return nil, fmt.Errorf("read rule file %s: %w", rulePath, err)
	}
	var rules ruleFile
	if err := yaml.Unmarshal(content, &rules); err != nil {
		return nil, fmt.Errorf("parse rule file %s: %w", rulePath, err)
	}
	return &rules, nil
}

func recordingRuleReferences(expression string) []string {
	matches := recordingRuleReferencePattern.FindAllString(expression, -1)
	sort.Strings(matches)
	return compactStrings(matches)
}

func globRequired(root, pattern string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no files matched %s", pattern)
	}
	sort.Strings(paths)
	return paths, nil
}

func compareNamedSets(name string, actual map[string]string, expected map[string]string) error {
	actualNames := make([]string, 0, len(actual))
	for name := range actual {
		actualNames = append(actualNames, name)
	}
	expectedNames := make([]string, 0, len(expected))
	for name := range expected {
		expectedNames = append(expectedNames, name)
	}
	return compareStringSets(name, actualNames, expectedNames)
}

func compareStringSets(name string, actual, expected []string) error {
	actualSet := stringSet(actual)
	expectedSet := stringSet(expected)

	missing := []string{}
	for _, item := range expected {
		if !actualSet[item] {
			missing = append(missing, item)
		}
	}
	extra := []string{}
	for _, item := range actual {
		if !expectedSet[item] {
			extra = append(extra, item)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 || len(extra) > 0 {
		parts := []string{}
		if len(missing) > 0 {
			parts = append(parts, "missing "+strings.Join(missing, ", "))
		}
		if len(extra) > 0 {
			parts = append(parts, "extra "+strings.Join(extra, ", "))
		}
		return fmt.Errorf("%s mismatch: %s", name, strings.Join(parts, "; "))
	}
	return nil
}

func csvList(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, filepath.ToSlash(trimmed))
		}
	}
	return values
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	compact := values[:1]
	for _, value := range values[1:] {
		if value != compact[len(compact)-1] {
			compact = append(compact, value)
		}
	}
	return compact
}

func relPath(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}
