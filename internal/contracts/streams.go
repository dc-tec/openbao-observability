package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type StreamContract struct {
	Version         string   `yaml:"version"`
	Status          string   `yaml:"status"`
	Streams         []Stream `yaml:"streams"`
	AllowedLabels   []string `yaml:"allowedLabels"`
	ForbiddenLabels []string `yaml:"forbiddenLabels"`
}

type Stream struct {
	ID        string `yaml:"id"`
	Default   string `yaml:"default"`
	Source    string `yaml:"source"`
	Format    string `yaml:"format"`
	Access    string `yaml:"access"`
	Retention string `yaml:"retention"`
}

type VerifyStreamOptions struct {
	ContractPath           string
	AlertContractPath      string
	DashboardContractPaths []string
}

var (
	labelMatcherPattern = regexp.MustCompile(`(?:^|,)\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:=~|!~|=|!=)`)
	logGroupingPattern  = regexp.MustCompile(`(?i)\b(?:by|without)\s*\(([^)]*)\)`)
)

func LoadStreamContract(path string) (*StreamContract, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read stream contract %s: %w", path, err)
	}

	var contract StreamContract
	if err := yaml.Unmarshal(content, &contract); err != nil {
		return nil, fmt.Errorf("parse stream contract %s: %w", path, err)
	}

	if err := contract.validateShape(path); err != nil {
		return nil, err
	}

	return &contract, nil
}

func VerifyStreamContract(opts VerifyStreamOptions) error {
	opts = opts.withDefaults()

	contract, err := LoadStreamContract(opts.ContractPath)
	if err != nil {
		return err
	}

	if opts.AlertContractPath != "" {
		alertContract, err := LoadAlertContract(opts.AlertContractPath)
		if err != nil {
			return err
		}
		for _, alert := range alertContract.Alerts {
			if alert.Type != alertTypeLoki {
				continue
			}
			if err := contract.ValidateLogExpression(alert.Expression); err != nil {
				return fmt.Errorf("validate Loki labels for alert %s: %w", alert.ID, err)
			}
		}
	}

	for _, dashboardPath := range opts.DashboardContractPaths {
		dashboardContract, err := LoadDashboardContract(dashboardPath)
		if err != nil {
			return err
		}
		for _, panel := range dashboardContract.Panels {
			if panel.Signal != dashboardSignalLogs {
				continue
			}
			if err := contract.ValidateLogExpression(panel.Expression); err != nil {
				return fmt.Errorf("validate Loki labels for dashboard panel %s in %s: %w", panel.ID, dashboardPath, err)
			}
		}
	}

	fmt.Printf("stream contract verified at %s\n", opts.ContractPath)
	return nil
}

func (o VerifyStreamOptions) withDefaults() VerifyStreamOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "streams", "log-streams.yaml")
	}
	if o.AlertContractPath == "" {
		o.AlertContractPath = filepath.Join("contracts", "alerts", "critical.yaml")
	}
	if len(o.DashboardContractPaths) == 0 {
		o.DashboardContractPaths = []string{
			filepath.Join("contracts", "dashboards", "openbao-overview.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-ha-raft.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-audit-overview.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-operational-logs.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-audit-investigation.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-auth-identity.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-token-lease-lifecycle.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-database-secrets.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-transit.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-pki.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-secret-engines-mounts.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-runtime-storage.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-kubernetes-platform.yaml"),
			filepath.Join("contracts", "dashboards", "openbao-slo-availability.yaml"),
		}
	}
	return o
}

func (c StreamContract) ValidateLogExpression(expression string) error {
	allowed := stringSet(c.AllowedLabels)
	forbidden := stringSet(c.ForbiddenLabels)

	for _, selector := range logSelectors(expression) {
		for _, match := range labelMatcherPattern.FindAllStringSubmatch(selector, -1) {
			label := match[1]
			if forbidden[label] {
				return fmt.Errorf("selector uses forbidden Loki label %q", label)
			}
			if !allowed[label] {
				return fmt.Errorf("selector uses label %q that is not listed in allowedLabels", label)
			}
		}
	}

	for _, match := range logGroupingPattern.FindAllStringSubmatch(expression, -1) {
		for _, label := range splitLabelList(match[1]) {
			if forbidden[label] {
				return fmt.Errorf("grouping uses forbidden Loki label %q", label)
			}
			if !allowed[label] {
				return fmt.Errorf("grouping uses label %q that is not listed in allowedLabels", label)
			}
		}
	}

	return nil
}

func (c StreamContract) validateShape(path string) error {
	if err := c.validateStreamContractHeader(path); err != nil {
		return err
	}
	if err := c.validateStreams(path); err != nil {
		return err
	}
	return c.validateLabels(path)
}

func (c StreamContract) validateStreamContractHeader(path string) error {
	switch {
	case len(c.Streams) == 0:
		return fmt.Errorf("stream contract %s has no streams", path)
	case len(c.AllowedLabels) == 0:
		return fmt.Errorf("stream contract %s has no allowedLabels", path)
	case len(c.ForbiddenLabels) == 0:
		return fmt.Errorf("stream contract %s has no forbiddenLabels", path)
	default:
		return nil
	}
}

func (c StreamContract) validateStreams(path string) error {
	seenStreams := map[string]bool{}
	for _, stream := range c.Streams {
		if err := validateStream(path, stream, seenStreams); err != nil {
			return err
		}
	}
	return nil
}

func validateStream(path string, stream Stream, seen map[string]bool) error {
	if stream.ID == "" {
		return fmt.Errorf("stream contract %s has a stream without an id", path)
	}
	if seen[stream.ID] {
		return fmt.Errorf("stream contract %s has duplicate stream id %q", path, stream.ID)
	}
	seen[stream.ID] = true
	if stream.Default == "" {
		return fmt.Errorf("stream %s is missing default", stream.ID)
	}
	if stream.Source == "" {
		return fmt.Errorf("stream %s is missing source", stream.ID)
	}
	if stream.Format == "" {
		return fmt.Errorf("stream %s is missing format", stream.ID)
	}
	if stream.Access == "" {
		return fmt.Errorf("stream %s is missing access", stream.ID)
	}
	if stream.Retention == "" {
		return fmt.Errorf("stream %s is missing retention", stream.ID)
	}
	return nil
}

func (c StreamContract) validateLabels(path string) error {
	allowed := stringSet(c.AllowedLabels)
	if !allowed["log_stream"] {
		return fmt.Errorf("stream contract %s must allow the log_stream routing label", path)
	}

	if err := validateLabelList("allowedLabels", c.AllowedLabels); err != nil {
		return fmt.Errorf("stream contract %s: %w", path, err)
	}
	if err := validateLabelList("forbiddenLabels", c.ForbiddenLabels); err != nil {
		return fmt.Errorf("stream contract %s: %w", path, err)
	}

	for _, label := range c.ForbiddenLabels {
		if allowed[label] {
			return fmt.Errorf("stream contract %s lists label %q as both allowed and forbidden", path, label)
		}
	}

	return nil
}

func validateLabelList(name string, labels []string) error {
	seen := map[string]bool{}
	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("%s contains an empty label", name)
		}
		if seen[label] {
			return fmt.Errorf("%s contains duplicate label %q", name, label)
		}
		seen[label] = true
	}
	return nil
}

func logSelectors(expression string) []string {
	selectors := []string{}
	inQuote := false
	escaped := false
	start := -1

	for i, r := range expression {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch r {
		case '{':
			if start == -1 {
				start = i + 1
			}
		case '}':
			if start != -1 {
				selectors = append(selectors, expression[start:i])
				start = -1
			}
		}
	}

	return selectors
}

func splitLabelList(value string) []string {
	labels := []string{}
	for _, item := range strings.Split(value, ",") {
		label := strings.TrimSpace(item)
		if label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
