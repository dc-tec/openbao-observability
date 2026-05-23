package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

type AlertContract struct {
	Version              string  `yaml:"version"`
	Status               string  `yaml:"status"`
	MetricPrefixVariable string  `yaml:"metricPrefixVariable"`
	SourcePrefix         string  `yaml:"sourcePrefix"`
	Alerts               []Alert `yaml:"alerts"`
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
	ContractPath string
	SourcePrefix string
}

func LoadAlertContract(path string) (*AlertContract, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read alert contract %s: %w", path, err)
	}

	var contract AlertContract
	if err := yaml.Unmarshal(content, &contract); err != nil {
		return nil, fmt.Errorf("parse alert contract %s: %w", path, err)
	}

	if err := contract.validateShape(path); err != nil {
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

	fmt.Printf("alert contract verified at %s\n", opts.ContractPath)
	return nil
}

func (o VerifyAlertOptions) withDefaults() VerifyAlertOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "alerts", "critical.yaml")
	}
	return o
}

func (c AlertContract) DefaultSourcePrefix() string {
	if c.SourcePrefix != "" {
		return c.SourcePrefix
	}
	return "vault"
}

func (c AlertContract) ValidateExpressions(sourcePrefix string) error {
	promQLParser := parser.NewParser(parser.Options{})
	for _, alert := range c.Alerts {
		expr := c.RenderExpression(alert.Expression, sourcePrefix)
		switch alert.Type {
		case "prometheus":
			if _, err := promQLParser.ParseExpr(expr); err != nil {
				return fmt.Errorf("parse PromQL for alert %s: %w", alert.ID, err)
			}
		case "loki":
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

func (c AlertContract) validateShape(path string) error {
	if len(c.Alerts) == 0 {
		return fmt.Errorf("alert contract %s has no alerts", path)
	}

	seen := map[string]bool{}
	for _, alert := range c.Alerts {
		if alert.ID == "" {
			return fmt.Errorf("alert contract %s has an alert without an id", path)
		}
		if seen[alert.ID] {
			return fmt.Errorf("alert contract %s has duplicate alert id %q", path, alert.ID)
		}
		seen[alert.ID] = true
		if alert.Type == "" {
			return fmt.Errorf("alert %s is missing type", alert.ID)
		}
		if alert.Severity == "" {
			return fmt.Errorf("alert %s is missing severity", alert.ID)
		}
		if alert.Signal == "" {
			return fmt.Errorf("alert %s is missing signal", alert.ID)
		}
		if alert.Expression == "" {
			return fmt.Errorf("alert %s is missing expression", alert.ID)
		}
		if alert.Summary == "" {
			return fmt.Errorf("alert %s is missing summary", alert.ID)
		}
		if alert.Description == "" {
			return fmt.Errorf("alert %s is missing description", alert.ID)
		}
		if alert.Runbook == "" {
			return fmt.Errorf("alert %s is missing runbook", alert.ID)
		}
	}

	return nil
}
