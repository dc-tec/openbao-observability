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

var recordingRuleReferencePattern = regexp.MustCompile(`\bopenbao:[A-Za-z0-9_:]+\b`)

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

	recordingRules, err := loadRecordingRuleNames(
		filepath.Join(root, "generated", "prometheus", "openbao-recording-rules.yaml"),
	)
	if err != nil {
		return err
	}
	if err := verifyGeneratedDashboards(
		root,
		dashboardContractPaths,
		generatedDashboardPaths,
		recordingRules,
		streamContract,
	); err != nil {
		return err
	}
	if err := verifyGeneratedAlerts(root, alertContractPaths, recordingRules, streamContract); err != nil {
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

func verifyGeneratedDashboards(
	root string,
	contractPaths, generatedPaths []string,
	recordingRules map[string]bool,
	streamContract *StreamContract,
) error {
	contractsByUID, err := loadDashboardContractFiles(root, contractPaths, streamContract)
	if err != nil {
		return err
	}
	generatedByUID, err := loadGeneratedDashboardFiles(root, generatedPaths, streamContract)
	if err != nil {
		return err
	}
	if err := verifyGeneratedDashboardPairs(
		root,
		contractsByUID,
		generatedByUID,
		recordingRules,
		streamContract,
	); err != nil {
		return err
	}
	return verifyGeneratedDashboardOrphans(root, contractsByUID, generatedByUID)
}

func loadDashboardContractFiles(
	root string,
	contractPaths []string,
	streamContract *StreamContract,
) (map[string]dashboardContractFile, error) {
	contractsByUID := map[string]dashboardContractFile{}
	for _, contractPath := range contractPaths {
		contract, err := LoadDashboardContract(contractPath)
		if err != nil {
			return nil, err
		}
		if err := contract.ValidateExpressions(); err != nil {
			return nil, fmt.Errorf("validate dashboard expressions in %s: %w", relPath(root, contractPath), err)
		}
		if err := validateDashboardContractLabelSafety(
			relPath(root, contractPath),
			contract,
			streamContract.ForbiddenLabels,
		); err != nil {
			return nil, err
		}
		if previous, ok := contractsByUID[contract.UID]; ok {
			return nil, fmt.Errorf(
				"dashboard uid %q is used by both %s and %s",
				contract.UID,
				relPath(root, previous.Path),
				relPath(root, contractPath),
			)
		}
		contractsByUID[contract.UID] = dashboardContractFile{Path: contractPath, Contract: contract}
	}
	return contractsByUID, nil
}

func loadGeneratedDashboardFiles(
	root string,
	generatedPaths []string,
	streamContract *StreamContract,
) (map[string]string, error) {
	generatedByUID := map[string]string{}
	for _, generatedPath := range generatedPaths {
		dashboard, err := loadGeneratedDashboard(generatedPath)
		if err != nil {
			return nil, err
		}
		if err := validateGeneratedDashboardSchema(root, generatedPath, dashboard, streamContract); err != nil {
			return nil, err
		}
		if previous, ok := generatedByUID[dashboard.UID]; ok {
			return nil, fmt.Errorf(
				"generated dashboard uid %q is used by both %s and %s",
				dashboard.UID,
				relPath(root, previous),
				relPath(root, generatedPath),
			)
		}
		if filepath.Base(generatedPath) != dashboard.UID+".json" {
			return nil, fmt.Errorf(
				"generated dashboard %s filename must match uid %q",
				relPath(root, generatedPath),
				dashboard.UID,
			)
		}
		generatedByUID[dashboard.UID] = generatedPath
	}
	return generatedByUID, nil
}

func verifyGeneratedDashboardPairs(
	root string,
	contractsByUID map[string]dashboardContractFile,
	generatedByUID map[string]string,
	recordingRules map[string]bool,
	streamContract *StreamContract,
) error {
	for uid, contractFile := range contractsByUID {
		generatedPath, ok := generatedByUID[uid]
		if !ok {
			return fmt.Errorf(
				"dashboard contract %s has no generated dashboard generated/grafana/%s.json",
				relPath(root, contractFile.Path),
				uid,
			)
		}
		dashboard, err := loadGeneratedDashboard(generatedPath)
		if err != nil {
			return err
		}
		if err := verifyGeneratedDashboard(
			root,
			contractFile.Path,
			contractFile.Contract,
			generatedPath,
			dashboard,
			recordingRules,
			streamContract,
		); err != nil {
			return err
		}
	}
	return nil
}

func verifyGeneratedDashboardOrphans(
	root string,
	contractsByUID map[string]dashboardContractFile,
	generatedByUID map[string]string,
) error {
	for uid, generatedPath := range generatedByUID {
		if _, ok := contractsByUID[uid]; !ok {
			return fmt.Errorf(
				"generated dashboard %s has no source dashboard contract with uid %q",
				relPath(root, generatedPath),
				uid,
			)
		}
	}
	return nil
}

type dashboardContractFile struct {
	Path     string
	Contract *DashboardContract
}

func verifyGeneratedDashboard(
	root, contractPath string,
	contract *DashboardContract,
	generatedPath string,
	dashboard *generatedDashboard,
	recordingRules map[string]bool,
	streamContract *StreamContract,
) error {
	if err := verifyGeneratedDashboardHeader(root, contractPath, contract, generatedPath, dashboard); err != nil {
		return err
	}
	if err := verifyGeneratedDashboardVariables(root, contractPath, contract, generatedPath, dashboard); err != nil {
		return err
	}
	return verifyGeneratedDashboardPanels(
		root,
		contractPath,
		contract,
		generatedPath,
		dashboard,
		recordingRules,
		streamContract,
	)
}

func verifyGeneratedDashboardHeader(
	root, contractPath string,
	contract *DashboardContract,
	generatedPath string,
	dashboard *generatedDashboard,
) error {
	if dashboard.Title != contract.Title {
		return fmt.Errorf(
			"generated dashboard %s title %q does not match contract %s title %q",
			relPath(root, generatedPath),
			dashboard.Title,
			relPath(root, contractPath),
			contract.Title,
		)
	}
	if dashboard.Refresh != contract.Refresh {
		return fmt.Errorf(
			"generated dashboard %s refresh %q does not match contract %s refresh %q",
			relPath(root, generatedPath),
			dashboard.Refresh,
			relPath(root, contractPath),
			contract.Refresh,
		)
	}
	if dashboard.Time.From != contract.TimeRange.From || dashboard.Time.To != contract.TimeRange.To {
		return fmt.Errorf(
			"generated dashboard %s time range %s..%s does not match contract %s time range %s..%s",
			relPath(root, generatedPath),
			dashboard.Time.From,
			dashboard.Time.To,
			relPath(root, contractPath),
			contract.TimeRange.From,
			contract.TimeRange.To,
		)
	}
	return nil
}

func verifyGeneratedDashboardPanels(
	root, contractPath string,
	contract *DashboardContract,
	generatedPath string,
	dashboard *generatedDashboard,
	recordingRules map[string]bool,
	streamContract *StreamContract,
) error {
	if len(dashboard.Panels) != len(contract.Panels) {
		return fmt.Errorf(
			"generated dashboard %s has %d panels, want %d from %s",
			relPath(root, generatedPath),
			len(dashboard.Panels),
			len(contract.Panels),
			relPath(root, contractPath),
		)
	}

	for i, panel := range contract.Panels {
		if err := verifyGeneratedDashboardPanel(
			root,
			contractPath,
			contract,
			generatedPath,
			panel,
			dashboard.Panels[i],
			i,
			recordingRules,
			streamContract,
		); err != nil {
			return err
		}
	}

	return nil
}

func verifyGeneratedDashboardPanel(
	root, contractPath string,
	contract *DashboardContract,
	generatedPath string,
	panel DashboardPanel,
	generatedPanel generatedDashboardPanel,
	index int,
	recordingRules map[string]bool,
	streamContract *StreamContract,
) error {
	if err := verifyGeneratedDashboardPanelShape(root, generatedPath, panel, generatedPanel, index); err != nil {
		return err
	}
	expectedDatasource, err := contractDatasource(contract, panel.Datasource)
	if err != nil {
		return fmt.Errorf("dashboard contract %s panel %s: %w", relPath(root, contractPath), panel.ID, err)
	}
	if err := verifyGeneratedPanelDatasource(
		root,
		generatedPath,
		panel.ID,
		generatedPanel.Datasource,
		expectedDatasource,
	); err != nil {
		return err
	}
	if len(generatedPanel.Targets) == 0 {
		return fmt.Errorf("generated dashboard %s panel %s has no targets", relPath(root, generatedPath), panel.ID)
	}
	for _, target := range generatedPanel.Targets {
		if err := verifyGeneratedDashboardTarget(
			root,
			contract,
			generatedPath,
			panel,
			target,
			expectedDatasource,
			streamContract,
		); err != nil {
			return err
		}
	}
	return verifyGeneratedPanelRecordingRules(root, contractPath, panel, recordingRules)
}

func verifyGeneratedDashboardPanelShape(
	root, generatedPath string,
	panel DashboardPanel,
	generatedPanel generatedDashboardPanel,
	index int,
) error {
	if generatedPanel.Title != panel.Title {
		return fmt.Errorf(
			"generated dashboard %s panel %d title %q does not match contract panel %s title %q",
			relPath(root, generatedPath),
			index+1,
			generatedPanel.Title,
			panel.ID,
			panel.Title,
		)
	}
	if generatedPanel.Type != panel.Type {
		return fmt.Errorf(
			"generated dashboard %s panel %s type %q does not match contract type %q",
			relPath(root, generatedPath),
			panel.ID,
			generatedPanel.Type,
			panel.Type,
		)
	}
	if generatedPanel.GridPos != panel.Grid {
		return fmt.Errorf(
			"generated dashboard %s panel %s grid does not match contract",
			relPath(root, generatedPath),
			panel.ID,
		)
	}
	return nil
}

func verifyGeneratedPanelDatasource(
	root, generatedPath, panelID string,
	datasource generatedDatasource,
	expected DashboardDatasource,
) error {
	if datasource.Type == expected.Type && datasource.UID == expected.UID {
		return nil
	}
	return fmt.Errorf(
		"generated dashboard %s panel %s datasource %s/%s does not match contract %s/%s",
		relPath(root, generatedPath),
		panelID,
		datasource.Type,
		datasource.UID,
		expected.Type,
		expected.UID,
	)
}

func verifyGeneratedDashboardTarget(
	root string,
	contract *DashboardContract,
	generatedPath string,
	panel DashboardPanel,
	target generatedTarget,
	expectedDatasource DashboardDatasource,
	streamContract *StreamContract,
) error {
	if target.Expr != panel.Expression {
		return fmt.Errorf(
			"generated dashboard %s panel %s target expression does not match contract",
			relPath(root, generatedPath),
			panel.ID,
		)
	}
	if target.Datasource.Type != expectedDatasource.Type || target.Datasource.UID != expectedDatasource.UID {
		return fmt.Errorf(
			"generated dashboard %s panel %s target datasource %s/%s does not match contract %s/%s",
			relPath(root, generatedPath),
			panel.ID,
			target.Datasource.Type,
			target.Datasource.UID,
			expectedDatasource.Type,
			expectedDatasource.UID,
		)
	}
	return verifyGeneratedDashboardTargetLabels(root, contract, generatedPath, panel, target, streamContract)
}

func verifyGeneratedDashboardTargetLabels(
	root string,
	contract *DashboardContract,
	generatedPath string,
	panel DashboardPanel,
	target generatedTarget,
	streamContract *StreamContract,
) error {
	switch panel.Signal {
	case dashboardSignalMetrics:
		expression := contract.ExpressionWithDefaultVariables(target.Expr)
		if err := validatePromQLLabelSafety(expression, streamContract.ForbiddenLabels); err != nil {
			return fmt.Errorf("generated dashboard %s panel %s target labels: %w", relPath(root, generatedPath), panel.ID, err)
		}
	case dashboardSignalLogs:
		if err := streamContract.ValidateLogExpression(target.Expr); err != nil {
			return fmt.Errorf("generated dashboard %s panel %s target labels: %w", relPath(root, generatedPath), panel.ID, err)
		}
	}
	return nil
}

func verifyGeneratedPanelRecordingRules(
	root, contractPath string,
	panel DashboardPanel,
	recordingRules map[string]bool,
) error {
	if panel.Signal != dashboardSignalMetrics {
		return nil
	}
	for _, ruleName := range recordingRuleReferences(panel.Expression) {
		if !recordingRules[ruleName] {
			return fmt.Errorf(
				"dashboard contract %s panel %s references recording rule %s, but it is not generated",
				relPath(root, contractPath),
				panel.ID,
				ruleName,
			)
		}
	}
	return nil
}

func verifyGeneratedDashboardVariables(
	root, contractPath string,
	contract *DashboardContract,
	generatedPath string,
	dashboard *generatedDashboard,
) error {
	if len(dashboard.Templating.List) != len(contract.Variables) {
		return fmt.Errorf(
			"generated dashboard %s has %d variables, want %d from %s",
			relPath(root, generatedPath),
			len(dashboard.Templating.List),
			len(contract.Variables),
			relPath(root, contractPath),
		)
	}
	for i, variable := range contract.Variables {
		generatedVariable := dashboard.Templating.List[i]
		if generatedVariable.Name != variable.Name {
			return fmt.Errorf(
				"generated dashboard %s variable %d name %q does not match contract variable %q",
				relPath(root, generatedPath),
				i+1,
				generatedVariable.Name,
				variable.Name,
			)
		}
		if generatedVariable.Type != variable.Type {
			return fmt.Errorf(
				"generated dashboard %s variable %s type %q does not match contract type %q",
				relPath(root, generatedPath),
				variable.Name,
				generatedVariable.Type,
				variable.Type,
			)
		}
		if generatedVariable.Current.Value != variable.Default {
			return fmt.Errorf(
				"generated dashboard %s variable %s default %q does not match contract default %q",
				relPath(root, generatedPath),
				variable.Name,
				generatedVariable.Current.Value,
				variable.Default,
			)
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

func verifyGeneratedAlerts(
	root string,
	alertContractPaths []string,
	recordingRules map[string]bool,
	streamContract *StreamContract,
) error {
	expectedPrometheusAlerts, expectedLokiAlerts, err := loadExpectedGeneratedAlerts(
		root,
		alertContractPaths,
		recordingRules,
		streamContract,
	)
	if err != nil {
		return err
	}
	return compareGeneratedAlertFiles(root, expectedPrometheusAlerts, expectedLokiAlerts, streamContract)
}

func loadExpectedGeneratedAlerts(
	root string,
	alertContractPaths []string,
	recordingRules map[string]bool,
	streamContract *StreamContract,
) (map[string]string, map[string]string, error) {
	expectedPrometheusAlerts := map[string]string{}
	expectedLokiAlerts := map[string]string{}
	for _, contractPath := range alertContractPaths {
		contract, err := LoadAlertContract(contractPath)
		if err != nil {
			return nil, nil, err
		}
		if err := contract.ValidateExpressions(contract.DefaultSourcePrefix()); err != nil {
			return nil, nil, fmt.Errorf("validate alert expressions in %s: %w", relPath(root, contractPath), err)
		}
		if err := contract.ValidateRunbooks(root); err != nil {
			return nil, nil, err
		}
		for _, alert := range contract.Alerts {
			if err := verifyAlertContractRecordingRules(
				root,
				contractPath,
				contract,
				alert,
				recordingRules,
			); err != nil {
				return nil, nil, err
			}
			if err := validateAlertContractLabelSafety(
				relPath(root, contractPath),
				contract,
				alert,
				streamContract,
			); err != nil {
				return nil, nil, err
			}
		}
		for _, alert := range contract.Alerts {
			if err := registerExpectedGeneratedAlert(
				relPath(root, contractPath),
				alert,
				expectedPrometheusAlerts,
				expectedLokiAlerts,
			); err != nil {
				return nil, nil, err
			}
		}
	}
	return expectedPrometheusAlerts, expectedLokiAlerts, nil
}

func verifyAlertContractRecordingRules(
	root, contractPath string,
	contract *AlertContract,
	alert Alert,
	recordingRules map[string]bool,
) error {
	if alert.Type != alertTypePrometheus {
		return nil
	}
	expression := contract.RenderExpression(alert.Expression, contract.DefaultSourcePrefix())
	for _, reference := range recordingRuleReferences(expression) {
		if recordingRules[reference] {
			continue
		}
		return fmt.Errorf(
			"alert contract %s alert %s references recording rule %s, but it is not generated",
			relPath(root, contractPath),
			alert.ID,
			reference,
		)
	}
	return nil
}

func registerExpectedGeneratedAlert(
	contractPath string,
	alert Alert,
	expectedPrometheusAlerts map[string]string,
	expectedLokiAlerts map[string]string,
) error {
	switch alert.Type {
	case alertTypePrometheus:
		if previous, ok := expectedPrometheusAlerts[alert.ID]; ok {
			return fmt.Errorf("prometheus alert %s is defined in both %s and %s", alert.ID, previous, contractPath)
		}
		expectedPrometheusAlerts[alert.ID] = contractPath
	case alertTypeLoki:
		if previous, ok := expectedLokiAlerts[alert.ID]; ok {
			return fmt.Errorf("loki alert %s is defined in both %s and %s", alert.ID, previous, contractPath)
		}
		expectedLokiAlerts[alert.ID] = contractPath
	}
	return nil
}

func compareGeneratedAlertFiles(
	root string,
	expectedPrometheusAlerts map[string]string,
	expectedLokiAlerts map[string]string,
	streamContract *StreamContract,
) error {
	generatedPrometheusAlerts, err := loadGeneratedAlertNames(
		root,
		filepath.Join("generated", "prometheus", "*.yaml"),
		false,
	)
	if err != nil {
		return err
	}
	generatedLokiAlerts, err := loadGeneratedAlertNames(root, filepath.Join("generated", "loki", "*.yaml"), true)
	if err != nil {
		return err
	}
	if err := compareNamedSets(
		"generated Prometheus alerts",
		generatedPrometheusAlerts,
		expectedPrometheusAlerts,
	); err != nil {
		return err
	}
	if err := compareNamedSets("generated Loki alerts", generatedLokiAlerts, expectedLokiAlerts); err != nil {
		return err
	}
	if err := verifyGeneratedRuleLabels(
		root,
		filepath.Join("generated", "prometheus", "*.yaml"),
		false,
		streamContract,
	); err != nil {
		return err
	}
	if err := verifyGeneratedRuleLabels(
		root,
		filepath.Join("generated", "loki", "*.yaml"),
		true,
		streamContract,
	); err != nil {
		return err
	}
	return nil
}

func validateAlertContractLabelSafety(
	source string,
	contract *AlertContract,
	alert Alert,
	streamContract *StreamContract,
) error {
	for label := range alert.Labels {
		if stringSet(streamContract.ForbiddenLabels)[label] {
			return fmt.Errorf("alert %s in %s uses forbidden alert label %q", alert.ID, source, label)
		}
	}
	switch alert.Type {
	case alertTypePrometheus:
		return validatePromQLLabelSafety(
			contract.RenderExpression(alert.Expression, contract.DefaultSourcePrefix()),
			streamContract.ForbiddenLabels,
		)
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
		expression := contract.ExpressionWithDefaultVariables(panel.Expression)
		if err := validatePromQLLabelSafety(expression, forbiddenLabels); err != nil {
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

func validateGeneratedDashboardSchema(
	root, generatedPath string,
	dashboard *generatedDashboard,
	streamContract *StreamContract,
) error {
	source := relPath(root, generatedPath)
	if err := validateGeneratedDashboardHeader(source, dashboard); err != nil {
		return err
	}
	if err := validateGeneratedDashboardVariableSchema(source, dashboard); err != nil {
		return err
	}
	return validateGeneratedDashboardPanelSchema(source, dashboard, streamContract)
}

func validateGeneratedDashboardHeader(source string, dashboard *generatedDashboard) error {
	switch {
	case dashboard.UID == "":
		return fmt.Errorf("generated dashboard %s is missing uid", source)
	case dashboard.Title == "":
		return fmt.Errorf("generated dashboard %s is missing title", source)
	case dashboard.SchemaVersion <= 0:
		return fmt.Errorf("generated dashboard %s is missing schemaVersion", source)
	case dashboard.Refresh == "":
		return fmt.Errorf("generated dashboard %s is missing refresh", source)
	case dashboard.Time.From == "" || dashboard.Time.To == "":
		return fmt.Errorf("generated dashboard %s is missing time.from or time.to", source)
	case len(dashboard.Panels) == 0:
		return fmt.Errorf("generated dashboard %s has no panels", source)
	default:
		return nil
	}
}

func validateGeneratedDashboardVariableSchema(source string, dashboard *generatedDashboard) error {
	seenVariables := map[string]bool{}
	for _, variable := range dashboard.Templating.List {
		if err := validateGeneratedDashboardVariable(source, variable, seenVariables); err != nil {
			return err
		}
	}
	return nil
}

func validateGeneratedDashboardVariable(
	source string,
	variable generatedDashboardVariable,
	seen map[string]bool,
) error {
	if variable.Name == "" {
		return fmt.Errorf("generated dashboard %s has a variable without a name", source)
	}
	if seen[variable.Name] {
		return fmt.Errorf("generated dashboard %s has duplicate variable %q", source, variable.Name)
	}
	seen[variable.Name] = true
	if variable.Type == "" {
		return fmt.Errorf("generated dashboard %s variable %s is missing type", source, variable.Name)
	}
	if variable.Current.Value == "" && variable.Current.Text == "" {
		return fmt.Errorf("generated dashboard %s variable %s is missing current value", source, variable.Name)
	}
	if variable.Type == dashboardVariableTypeList && len(variable.Options) == 0 {
		return fmt.Errorf("generated dashboard %s custom variable %s has no options", source, variable.Name)
	}
	return nil
}

func validateGeneratedDashboardPanelSchema(
	source string,
	dashboard *generatedDashboard,
	streamContract *StreamContract,
) error {
	seenPanels := map[int]bool{}
	for _, panel := range dashboard.Panels {
		if err := validateGeneratedDashboardPanel(source, dashboard, panel, seenPanels, streamContract); err != nil {
			return err
		}
	}
	return nil
}

func validateGeneratedDashboardPanel(
	source string,
	dashboard *generatedDashboard,
	panel generatedDashboardPanel,
	seenPanels map[int]bool,
	streamContract *StreamContract,
) error {
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
	return validateGeneratedDashboardTargets(source, dashboard, panel, streamContract)
}

func validateGeneratedDashboardTargets(
	source string,
	dashboard *generatedDashboard,
	panel generatedDashboardPanel,
	streamContract *StreamContract,
) error {
	seenTargets := map[string]bool{}
	for _, target := range panel.Targets {
		if err := validateGeneratedDashboardTarget(
			source,
			dashboard,
			panel.ID,
			target,
			seenTargets,
			streamContract,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateGeneratedDashboardTarget(
	source string,
	dashboard *generatedDashboard,
	panelID int,
	target generatedTarget,
	seenTargets map[string]bool,
	streamContract *StreamContract,
) error {
	if target.RefID == "" {
		return fmt.Errorf("generated dashboard %s panel %d has target without refId", source, panelID)
	}
	if seenTargets[target.RefID] {
		return fmt.Errorf("generated dashboard %s panel %d has duplicate target refId %q", source, panelID, target.RefID)
	}
	seenTargets[target.RefID] = true
	if target.Expr == "" {
		return fmt.Errorf("generated dashboard %s panel %d target %s is missing expr", source, panelID, target.RefID)
	}
	field := fmt.Sprintf("panel %d target %s", panelID, target.RefID)
	if err := validateGeneratedDatasource(source, field, target.Datasource); err != nil {
		return err
	}
	return validateGeneratedTargetExpression(source, dashboard, panelID, target, streamContract)
}

func validateGeneratedTargetExpression(
	source string,
	dashboard *generatedDashboard,
	panelID int,
	target generatedTarget,
	streamContract *StreamContract,
) error {
	switch target.Datasource.Type {
	case alertTypePrometheus:
		expression := InterpolateDashboardVariables(target.Expr, generatedDashboardVariableDefaults(dashboard))
		if err := validatePromQLLabelSafety(expression, streamContract.ForbiddenLabels); err != nil {
			return fmt.Errorf("generated dashboard %s panel %d target %s labels: %w", source, panelID, target.RefID, err)
		}
	case alertTypeLoki:
		if err := streamContract.ValidateLogExpression(target.Expr); err != nil {
			return fmt.Errorf("generated dashboard %s panel %d target %s labels: %w", source, panelID, target.RefID, err)
		}
	default:
		return fmt.Errorf(
			"generated dashboard %s panel %d target %s has unsupported datasource type %q",
			source,
			panelID,
			target.RefID,
			target.Datasource.Type,
		)
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

func validateGeneratedRuleLabelSafety(
	root, filePath string,
	rule rule,
	loki bool,
	streamContract *StreamContract,
) error {
	for label := range rule.Labels {
		if stringSet(streamContract.ForbiddenLabels)[label] {
			return fmt.Errorf(
				"generated rule %s in %s uses forbidden label %q",
				generatedRuleName(rule),
				relPath(root, filePath),
				label,
			)
		}
	}
	if rule.Expr == "" {
		return nil
	}
	if loki {
		if err := streamContract.ValidateLogExpression(rule.Expr); err != nil {
			return fmt.Errorf(
				"generated Loki rule %s in %s labels: %w",
				generatedRuleName(rule),
				relPath(root, filePath),
				err,
			)
		}
		return nil
	}
	if err := validatePromQLLabelSafety(rule.Expr, streamContract.ForbiddenLabels); err != nil {
		return fmt.Errorf(
			"generated Prometheus rule %s in %s labels: %w",
			generatedRuleName(rule),
			relPath(root, filePath),
			err,
		)
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
			if err := verifyRuleFileSourcePrefix(
				root,
				defaultPath,
				artifact.spec,
				artifact.loki,
				defaultSourcePrefix,
				streamContract,
			); err != nil {
				return err
			}
			if err := verifyRuleFileSourcePrefix(
				root,
				vaultPath,
				artifact.spec,
				artifact.loki,
				defaultSourcePrefix,
				streamContract,
			); err != nil {
				return err
			}
			if err := verifyRuleFileSourcePrefix(
				root,
				openbaoPath,
				artifact.spec,
				artifact.loki,
				"openbao",
				streamContract,
			); err != nil {
				return err
			}
		}
	}
	return verifyPrometheusRuleProfiles(root)
}

func verifyPrometheusRuleProfiles(root string) error {
	profiles := []string{"", "vault-prefix", "openbao-prefix"}
	fileNames := []string{
		"openbao-recording-rules.yaml",
		"openbao-alerts.yaml",
		"openbao-warning-alerts.yaml",
		"openbao-security-alerts.yaml",
	}
	alertFileNames := fileNames[1:]

	for _, profile := range profiles {
		nativeDir := filepath.Join(root, "generated", "prometheus", profile)
		prometheusRuleDir := filepath.Join(root, "generated", "prometheusrules", profile)
		for _, fileName := range fileNames {
			if err := verifyPrometheusRuleParity(
				root,
				filepath.Join(nativeDir, fileName),
				filepath.Join(prometheusRuleDir, fileName),
			); err != nil {
				return err
			}
		}
		if err := verifyAlertRecordingRuleReferences(
			root,
			filepath.Join(nativeDir, fileNames[0]),
			nativeDir,
			alertFileNames,
		); err != nil {
			return err
		}
	}
	return nil
}

func verifyPrometheusRuleParity(root, nativePath, prometheusRulePath string) error {
	nativeGroups, err := loadGenericRuleGroups(nativePath, false)
	if err != nil {
		return err
	}
	operatorGroups, err := loadGenericRuleGroups(prometheusRulePath, true)
	if err != nil {
		return err
	}
	if difference := firstSemanticDifference(nativeGroups, operatorGroups, "groups"); difference != "" {
		return fmt.Errorf(
			"PrometheusRule %s semantic field %s does not match native rules %s",
			relPath(root, prometheusRulePath),
			difference,
			relPath(root, nativePath),
		)
	}
	return nil
}

func loadGenericRuleGroups(rulePath string, specGroups bool) (*yaml.Node, error) {
	content, err := os.ReadFile(rulePath)
	if err != nil {
		return nil, fmt.Errorf("read rule file %s: %w", rulePath, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parse rule file %s: %w", rulePath, err)
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("rule file %s has no YAML document", rulePath)
	}
	container := document.Content[0]
	if specGroups {
		spec := yamlMappingValue(container, "spec")
		if spec == nil {
			return nil, fmt.Errorf("PrometheusRule %s is missing spec", rulePath)
		}
		container = spec
	}
	groups := yamlMappingValue(container, "groups")
	if groups == nil || groups.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("rule file %s is missing groups", rulePath)
	}
	return groups, nil
}

func yamlMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func firstSemanticDifference(expected, actual *yaml.Node, location string) string {
	if expected.Kind != actual.Kind || expected.Tag != actual.Tag {
		return location
	}
	switch expected.Kind {
	case yaml.ScalarNode:
		if expected.Value != actual.Value {
			return location
		}
		return ""
	case yaml.SequenceNode:
		return firstYAMLSequenceDifference(expected, actual, location)
	case yaml.MappingNode:
		return firstYAMLMappingDifference(expected, actual, location)
	default:
		if expected.Value != actual.Value || len(expected.Content) != len(actual.Content) {
			return location
		}
		return firstYAMLSequenceDifference(expected, actual, location)
	}
}

func firstYAMLSequenceDifference(expected, actual *yaml.Node, location string) string {
	if len(expected.Content) != len(actual.Content) {
		return location
	}
	for index := range expected.Content {
		childLocation := fmt.Sprintf("%s[%d]", location, index)
		difference := firstSemanticDifference(expected.Content[index], actual.Content[index], childLocation)
		if difference != "" {
			return difference
		}
	}
	return ""
}

func firstYAMLMappingDifference(expected, actual *yaml.Node, location string) string {
	expectedValues := yamlMappingValues(expected)
	actualValues := yamlMappingValues(actual)
	keys := make([]string, 0, len(expectedValues)+len(actualValues))
	seen := map[string]bool{}
	for key := range expectedValues {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range actualValues {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		expectedChild, expectedOK := expectedValues[key]
		actualChild, actualOK := actualValues[key]
		childLocation := location + "." + key
		if !expectedOK || !actualOK {
			return childLocation
		}
		if difference := firstSemanticDifference(expectedChild, actualChild, childLocation); difference != "" {
			return difference
		}
	}
	return ""
}

func yamlMappingValues(node *yaml.Node) map[string]*yaml.Node {
	values := make(map[string]*yaml.Node, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		values[node.Content[index].Value] = node.Content[index+1]
	}
	return values
}

func verifyAlertRecordingRuleReferences(
	root, recordingRulePath, alertDir string,
	alertFileNames []string,
) error {
	recordingRules, err := loadRecordingRuleNames(recordingRulePath)
	if err != nil {
		return err
	}
	for _, fileName := range alertFileNames {
		alertPath := filepath.Join(alertDir, fileName)
		alertRules, err := loadRuleFile(alertPath)
		if err != nil {
			return err
		}
		for _, group := range alertRules.Groups {
			for _, alert := range group.Rules {
				if alert.Alert == "" {
					continue
				}
				for _, reference := range recordingRuleReferences(alert.Expr) {
					if recordingRules[reference] {
						continue
					}
					return fmt.Errorf(
						"generated alert %s in %s references recording rule %s, but it is not generated in %s",
						alert.Alert,
						relPath(root, alertPath),
						reference,
						relPath(root, recordingRulePath),
					)
				}
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
		return fmt.Errorf(
			"generated default artifact %s must match vault-prefix artifact %s",
			relPath(root, leftPath),
			relPath(root, rightPath),
		)
	}
	return nil
}

func verifyRuleFileSourcePrefix(
	root, filePath string,
	specGroups, loki bool,
	expectedPrefix string,
	streamContract *StreamContract,
) error {
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
				return fmt.Errorf(
					"generated rule %s in %s has source_prefix %q, want %q",
					generatedRuleName(rule),
					relPath(root, filePath),
					got,
					expectedPrefix,
				)
			}
			if err := validateGeneratedRuleLabelSafety(root, filePath, rule, loki, streamContract); err != nil {
				return err
			}
			var metricNames []string
			if !loki {
				metricNames, err = promQLMetricNames(rule.Expr)
				if err != nil {
					return fmt.Errorf(
						"parse generated rule %s in %s: %w",
						generatedRuleName(rule),
						relPath(root, filePath),
						err,
					)
				}
			}
			if expectedPrefix == "openbao" && metricNameHasPrefix(metricNames, "vault_") {
				return fmt.Errorf(
					"generated rule %s in %s uses vault_* metric under openbao-prefix profile",
					generatedRuleName(rule),
					relPath(root, filePath),
				)
			}
			if expectedPrefix == defaultSourcePrefix && hasDisallowedRawOpenBAOMetric(metricNames) {
				return fmt.Errorf(
					"generated rule %s in %s uses openbao_* metric under vault-prefix profile",
					generatedRuleName(rule),
					relPath(root, filePath),
				)
			}
		}
	}
	return nil
}

func promQLMetricNames(expression string) ([]string, error) {
	expr, err := parser.NewParser(parser.Options{}).ParseExpr(expression)
	if err != nil {
		return nil, err
	}
	var names []string
	parser.Inspect(expr, func(node parser.Node, _ []parser.Node) error {
		selector, ok := node.(*parser.VectorSelector)
		if ok && selector.Name != "" {
			names = append(names, selector.Name)
		}
		return nil
	})
	return names, nil
}

func metricNameHasPrefix(metricNames []string, prefix string) bool {
	for _, name := range metricNames {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func hasDisallowedRawOpenBAOMetric(metricNames []string) bool {
	for _, name := range metricNames {
		if !strings.HasPrefix(name, "openbao_") ||
			strings.HasPrefix(name, "openbao_audit_archive_") ||
			name == "openbao_observability_signal_expected" {
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
				return fmt.Errorf(
					"runbook %s referenced by alert %s is not linked from %s",
					cleaned,
					alert.ID,
					relPath(root, indexPath),
				)
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
					return nil, fmt.Errorf(
						"generated alert %s is defined in both %s and %s",
						rule.Alert,
						previous,
						relPath(root, filePath),
					)
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
