package contracts

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type VerifyRepositoryOptions struct {
	RepositoryRoot string
}

type generatedDashboard struct {
	UID    string                    `json:"uid"`
	Title  string                    `json:"title"`
	Panels []generatedDashboardPanel `json:"panels"`
}

type generatedDashboardPanel struct {
	Title      string              `json:"title"`
	Type       string              `json:"type"`
	Datasource generatedDatasource `json:"datasource"`
	Targets    []generatedTarget   `json:"targets"`
}

type generatedDatasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

type generatedTarget struct {
	Expr       string              `json:"expr"`
	Datasource generatedDatasource `json:"datasource"`
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
	Alert  string `yaml:"alert"`
	Record string `yaml:"record"`
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
	if err := verifyGeneratedDashboards(root, dashboardContractPaths, generatedDashboardPaths, recordingRules); err != nil {
		return err
	}
	if err := verifyGeneratedAlerts(root, alertContractPaths); err != nil {
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

func verifyGeneratedDashboards(root string, contractPaths, generatedPaths []string, recordingRules map[string]bool) error {
	contractsByUID := map[string]dashboardContractFile{}
	for _, contractPath := range contractPaths {
		contract, err := LoadDashboardContract(contractPath)
		if err != nil {
			return err
		}
		if err := contract.ValidateExpressions(); err != nil {
			return fmt.Errorf("validate dashboard expressions in %s: %w", relPath(root, contractPath), err)
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
		if dashboard.UID == "" {
			return fmt.Errorf("generated dashboard %s is missing uid", relPath(root, generatedPath))
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
		if err := verifyGeneratedDashboard(root, contractFile.Path, contractFile.Contract, generatedPath, dashboard, recordingRules); err != nil {
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

func verifyGeneratedDashboard(root, contractPath string, contract *DashboardContract, generatedPath string, dashboard *generatedDashboard, recordingRules map[string]bool) error {
	if dashboard.Title != contract.Title {
		return fmt.Errorf("generated dashboard %s title %q does not match contract %s title %q", relPath(root, generatedPath), dashboard.Title, relPath(root, contractPath), contract.Title)
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
		}
		if panel.Signal == "metrics" {
			for _, ruleName := range recordingRuleReferences(panel.Expression) {
				if !recordingRules[ruleName] {
					return fmt.Errorf("dashboard contract %s panel %s references recording rule %s, but it is not generated", relPath(root, contractPath), panel.ID, ruleName)
				}
			}
		}
	}

	return nil
}

func contractDatasource(contract *DashboardContract, name string) (DashboardDatasource, error) {
	switch name {
	case "metrics":
		return contract.Datasources.Metrics, nil
	case "logs":
		return contract.Datasources.Logs, nil
	default:
		return DashboardDatasource{}, fmt.Errorf("unsupported datasource %q", name)
	}
}

func verifyGeneratedAlerts(root string, alertContractPaths []string) error {
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
			switch alert.Type {
			case "prometheus":
				if previous, ok := expectedPrometheusAlerts[alert.ID]; ok {
					return fmt.Errorf("prometheus alert %s is defined in both %s and %s", alert.ID, previous, relPath(root, contractPath))
				}
				expectedPrometheusAlerts[alert.ID] = relPath(root, contractPath)
			case "loki":
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
	return nil
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
	for _, path := range paths {
		expected = append(expected, relPath(root, path))
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

	return nil, fmt.Errorf("Makefile variable %s is not defined", variable)
}

func loadGeneratedDashboard(path string) (*generatedDashboard, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read generated dashboard %s: %w", path, err)
	}
	var dashboard generatedDashboard
	if err := json.Unmarshal(content, &dashboard); err != nil {
		return nil, fmt.Errorf("parse generated dashboard %s: %w", path, err)
	}
	return &dashboard, nil
}

func loadRecordingRuleNames(path string) (map[string]bool, error) {
	rules, err := loadRuleFile(path)
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

func loadRuleFile(path string) (*ruleFile, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rule file %s: %w", path, err)
	}
	var rules ruleFile
	if err := yaml.Unmarshal(content, &rules); err != nil {
		return nil, fmt.Errorf("parse rule file %s: %w", path, err)
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
