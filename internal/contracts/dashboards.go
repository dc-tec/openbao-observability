package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

type DashboardContract struct {
	Version     string               `yaml:"version"`
	Status      string               `yaml:"status"`
	UID         string               `yaml:"uid"`
	Title       string               `yaml:"title"`
	Refresh     string               `yaml:"refresh"`
	Tags        []string             `yaml:"tags"`
	TimeRange   DashboardTimeRange   `yaml:"timeRange"`
	Datasources DashboardDatasources `yaml:"datasources"`
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

type DashboardPanel struct {
	ID          string        `yaml:"id"`
	Title       string        `yaml:"title"`
	Type        string        `yaml:"type"`
	Signal      string        `yaml:"signal"`
	Datasource  string        `yaml:"datasource"`
	Expression  string        `yaml:"expression"`
	Legend      string        `yaml:"legend"`
	Unit        string        `yaml:"unit"`
	Description string        `yaml:"description"`
	Grid        DashboardGrid `yaml:"grid"`
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
		switch panel.Signal {
		case "metrics":
			if _, err := promQLParser.ParseExpr(panel.Expression); err != nil {
				return fmt.Errorf("parse PromQL for dashboard panel %s: %w", panel.ID, err)
			}
		case "logs":
			if !strings.Contains(panel.Expression, "{") || !strings.Contains(panel.Expression, "}") {
				return fmt.Errorf("log panel %s expression must include a label selector", panel.ID)
			}
		default:
			return fmt.Errorf("dashboard panel %s has unsupported signal %q", panel.ID, panel.Signal)
		}
	}
	return nil
}

func (c DashboardContract) validateShape(path string) error {
	if c.UID == "" {
		return fmt.Errorf("dashboard contract %s is missing uid", path)
	}
	if c.Title == "" {
		return fmt.Errorf("dashboard contract %s is missing title", path)
	}
	if c.TimeRange.From == "" || c.TimeRange.To == "" {
		return fmt.Errorf("dashboard contract %s is missing timeRange.from or timeRange.to", path)
	}
	if c.Datasources.Metrics.UID == "" || c.Datasources.Metrics.Type == "" {
		return fmt.Errorf("dashboard contract %s is missing metrics datasource", path)
	}
	if c.Datasources.Logs.UID == "" || c.Datasources.Logs.Type == "" {
		return fmt.Errorf("dashboard contract %s is missing logs datasource", path)
	}
	if len(c.Panels) == 0 {
		return fmt.Errorf("dashboard contract %s has no panels", path)
	}

	seen := map[string]bool{}
	for _, panel := range c.Panels {
		if panel.ID == "" {
			return fmt.Errorf("dashboard contract %s has a panel without an id", path)
		}
		if seen[panel.ID] {
			return fmt.Errorf("dashboard contract %s has duplicate panel id %q", path, panel.ID)
		}
		seen[panel.ID] = true
		if panel.Title == "" {
			return fmt.Errorf("dashboard panel %s is missing title", panel.ID)
		}
		if panel.Type == "" {
			return fmt.Errorf("dashboard panel %s is missing type", panel.ID)
		}
		switch panel.Type {
		case "logs", "stat", "timeseries":
		default:
			return fmt.Errorf("dashboard panel %s has unsupported type %q", panel.ID, panel.Type)
		}
		if panel.Datasource != "metrics" && panel.Datasource != "logs" {
			return fmt.Errorf("dashboard panel %s has unsupported datasource %q", panel.ID, panel.Datasource)
		}
		if panel.Signal == "metrics" && panel.Datasource != "metrics" {
			return fmt.Errorf("dashboard panel %s uses metrics signal with datasource %q", panel.ID, panel.Datasource)
		}
		if panel.Signal == "logs" && panel.Datasource != "logs" {
			return fmt.Errorf("dashboard panel %s uses logs signal with datasource %q", panel.ID, panel.Datasource)
		}
		if panel.Expression == "" {
			return fmt.Errorf("dashboard panel %s is missing expression", panel.ID)
		}
		if panel.Grid.W <= 0 || panel.Grid.H <= 0 {
			return fmt.Errorf("dashboard panel %s has invalid grid size", panel.ID)
		}
	}

	return nil
}
