package contracts

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"
)

type AlertContract struct {
	Version              string   `yaml:"version"`
	Maturity             Maturity `yaml:"maturity"`
	MetricPrefixVariable string   `yaml:"metricPrefixVariable"`
	SourcePrefix         string   `yaml:"sourcePrefix"`
	Alerts               []Alert  `yaml:"alerts"`
}

type Alert struct {
	ID          string            `yaml:"id"`
	Type        string            `yaml:"type"`
	Severity    string            `yaml:"severity"`
	Signal      string            `yaml:"signal"`
	For         string            `yaml:"for"`
	Expression  string            `yaml:"expression"`
	Summary     string            `yaml:"summary"`
	Description string            `yaml:"description"`
	Runbook     string            `yaml:"runbook"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type VerifyAlertOptions struct {
	ContractPath     string
	SourcePrefix     string
	RepositoryRoot   string
	ExpectedSeverity string
}

func LoadAlertContract(contractPath string) (*AlertContract, error) {
	content, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, fmt.Errorf("read alert contract %s: %w", contractPath, err)
	}

	var contract AlertContract
	if err := decodeContractYAML(content, &contract); err != nil {
		return nil, fmt.Errorf("parse alert contract %s: %w", contractPath, err)
	}

	if err := contract.validateShape(contractPath); err != nil {
		return nil, err
	}

	return &contract, nil
}

func VerifyAlertContract(opts VerifyAlertOptions) error {
	opts = opts.withDefaults()

	contract, err := LoadAlertContract(opts.ContractPath)
	if err != nil {
		return err
	}

	sourcePrefix := opts.SourcePrefix
	if sourcePrefix == "" {
		sourcePrefix = contract.DefaultSourcePrefix()
	}

	if err := contract.ValidateExpressions(sourcePrefix); err != nil {
		return err
	}

	if err := contract.ValidateRunbooks(opts.RepositoryRoot); err != nil {
		return err
	}
	if err := contract.ValidateSeverity(opts.ExpectedSeverity); err != nil {
		return err
	}

	fmt.Printf("alert contract verified at %s\n", opts.ContractPath)
	return nil
}

func (o VerifyAlertOptions) withDefaults() VerifyAlertOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "alerts", "critical.yaml")
	}
	if o.RepositoryRoot == "" {
		o.RepositoryRoot = "."
	}
	return o
}

func (c AlertContract) DefaultSourcePrefix() string {
	if c.SourcePrefix != "" {
		return c.SourcePrefix
	}
	return defaultSourcePrefix
}

func (c AlertContract) ValidateExpressions(sourcePrefix string) error {
	if err := validateMetricSourcePrefix(sourcePrefix); err != nil {
		return fmt.Errorf("validate alert source prefix: %w", err)
	}

	promQLParser := parser.NewParser(parser.Options{})
	for _, alert := range c.Alerts {
		expr := c.RenderExpression(alert.Expression, sourcePrefix)
		switch alert.Type {
		case alertTypePrometheus:
			if _, err := promQLParser.ParseExpr(expr); err != nil {
				return fmt.Errorf("parse PromQL for alert %s: %w", alert.ID, err)
			}
		case alertTypeLoki:
			if !strings.Contains(expr, "{") || !strings.Contains(expr, "}") {
				return fmt.Errorf("loki alert %s expression must include a label selector", alert.ID)
			}
		default:
			return fmt.Errorf("alert %s has unsupported type %q", alert.ID, alert.Type)
		}
	}
	return nil
}

func (c AlertContract) RenderExpression(expression, sourcePrefix string) string {
	variable := c.MetricPrefixVariable
	if variable == "" {
		variable = "${p}"
	}
	return strings.ReplaceAll(expression, variable, sourcePrefix)
}

func (c AlertContract) ValidateRunbooks(repositoryRoot string) error {
	if repositoryRoot == "" {
		repositoryRoot = "."
	}

	for _, alert := range c.Alerts {
		if isExternalRunbook(alert.Runbook) {
			continue
		}
		if filepath.IsAbs(alert.Runbook) {
			return fmt.Errorf("alert %s runbook path must be repository-relative: %s", alert.ID, alert.Runbook)
		}

		cleaned := path.Clean(alert.Runbook)
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
			return fmt.Errorf("alert %s runbook path must stay inside the repository: %s", alert.ID, alert.Runbook)
		}
		if !strings.HasSuffix(cleaned, ".md") {
			return fmt.Errorf("alert %s runbook path must point to a markdown file: %s", alert.ID, alert.Runbook)
		}

		info, err := os.Stat(filepath.Join(repositoryRoot, filepath.FromSlash(cleaned)))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("alert %s runbook does not exist: %s", alert.ID, alert.Runbook)
			}
			return fmt.Errorf("stat runbook for alert %s: %w", alert.ID, err)
		}
		if info.IsDir() {
			return fmt.Errorf("alert %s runbook path is a directory: %s", alert.ID, alert.Runbook)
		}
	}

	return nil
}

func (c AlertContract) ValidateSeverity(expectedSeverity string) error {
	if expectedSeverity == "" {
		return nil
	}
	for _, alert := range c.Alerts {
		if alert.Severity != expectedSeverity {
			return fmt.Errorf("alert %s has severity %q, want %q", alert.ID, alert.Severity, expectedSeverity)
		}
	}
	return nil
}

func isExternalRunbook(runbook string) bool {
	return strings.HasPrefix(runbook, "https://") || strings.HasPrefix(runbook, "http://")
}

func (c AlertContract) validateShape(contractPath string) error {
	if err := c.validateHeader(contractPath); err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, alert := range c.Alerts {
		if err := validateAlertShape(contractPath, alert, seen); err != nil {
			return err
		}
	}

	return nil
}

func (c AlertContract) validateHeader(contractPath string) error {
	if err := validateContractVersion(contractPath, c.Version); err != nil {
		return err
	}
	if err := validateMaturity(contractPath, c.Maturity); err != nil {
		return err
	}
	if err := validateMetricSourcePrefix(c.DefaultSourcePrefix()); err != nil {
		return fmt.Errorf("alert contract %s has invalid sourcePrefix: %w", contractPath, err)
	}
	if len(c.Alerts) == 0 {
		return fmt.Errorf("alert contract %s has no alerts", contractPath)
	}
	return nil
}

func validateAlertShape(contractPath string, alert Alert, seen map[string]bool) error {
	if alert.ID == "" {
		return fmt.Errorf("alert contract %s has an alert without an id", contractPath)
	}
	if seen[alert.ID] {
		return fmt.Errorf("alert contract %s has duplicate alert id %q", contractPath, alert.ID)
	}
	seen[alert.ID] = true
	required := []struct {
		name  string
		value string
	}{
		{name: "type", value: alert.Type},
		{name: "severity", value: alert.Severity},
		{name: "signal", value: alert.Signal},
		{name: "expression", value: alert.Expression},
		{name: "summary", value: alert.Summary},
		{name: "description", value: alert.Description},
		{name: "runbook", value: alert.Runbook},
	}
	for _, field := range required {
		if field.value == "" {
			return fmt.Errorf("alert %s is missing %s", alert.ID, field.name)
		}
	}
	return validateAlertMetadataKeys(alert)
}

func validateAlertMetadataKeys(alert Alert) error {
	for _, key := range []string{"severity", "signal", "source_prefix"} {
		if _, ok := alert.Labels[key]; ok {
			return fmt.Errorf("alert %s uses reserved label %q", alert.ID, key)
		}
	}
	for _, key := range []string{"summary", "description", "runbook_url"} {
		if _, ok := alert.Annotations[key]; ok {
			return fmt.Errorf("alert %s uses reserved annotation %q", alert.ID, key)
		}
	}
	return nil
}
