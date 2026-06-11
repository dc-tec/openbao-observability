package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dc-tec/openbao-observability/internal/promtext"
	"gopkg.in/yaml.v3"
)

type MetricContract struct {
	Version          string            `yaml:"version"`
	Maturity         Maturity          `yaml:"maturity"`
	OpenBAOVersion   string            `yaml:"openbaoVersion"`
	MetricPrefixes   MetricPrefixes    `yaml:"metricPrefixes"`
	Normalization    Normalization     `yaml:"normalization"`
	Fixtures         Fixtures          `yaml:"fixtures"`
	CoverageProfiles []CoverageProfile `yaml:"coverageProfiles"`
	Metrics          []Metric          `yaml:"metrics"`
}

type MetricPrefixes struct {
	Supported []string `yaml:"supported"`
	Default   string   `yaml:"default"`
}

type Normalization struct {
	RecordingRulePrefix string   `yaml:"recordingRulePrefix"`
	Notes               []string `yaml:"notes"`
}

type Fixtures struct {
	Required []string `yaml:"required"`
}

type CoverageProfile struct {
	ID                 string `yaml:"id"`
	Class              string `yaml:"class"`
	DefaultExpectation string `yaml:"defaultExpectation"`
}

const (
	MetricCoverageRequired      = "required"
	MetricCoverageOptional      = "optional"
	MetricCoverageNotApplicable = "not-applicable"
)

type Metric struct {
	ID                    string            `yaml:"id"`
	DocsName              string            `yaml:"docsName"`
	PrometheusName        string            `yaml:"prometheusName"`
	FixturePrometheusName string            `yaml:"fixturePrometheusName"`
	Required              bool              `yaml:"required"`
	Overview              bool              `yaml:"overview"`
	Notes                 []string          `yaml:"notes"`
	Coverage              map[string]string `yaml:"coverage"`
}

type VerifyOptions struct {
	ContractPath string
	FixtureDir   string
}

func LoadMetricContract(path string) (*MetricContract, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read metric contract %s: %w", path, err)
	}

	var contract MetricContract
	if err := yaml.Unmarshal(content, &contract); err != nil {
		return nil, fmt.Errorf("parse metric contract %s: %w", path, err)
	}

	if err := contract.validateShape(path); err != nil {
		return nil, err
	}

	return &contract, nil
}

func VerifyMetricContract(opts VerifyOptions) error {
	opts = opts.withDefaults()

	contract, err := LoadMetricContract(opts.ContractPath)
	if err != nil {
		return err
	}

	for _, fixture := range contract.Fixtures.Required {
		if _, err := os.Stat(fixture); err != nil {
			return fmt.Errorf("required fixture %s is not readable: %w", fixture, err)
		}
	}

	for _, prefix := range contract.MetricPrefixes.Supported {
		fixturePath := metricFixturePath(opts.FixtureDir, contract.OpenBAOVersion, prefix)
		families, err := promtext.LoadFamilies(fixturePath)
		if err != nil {
			return err
		}

		for _, metric := range contract.Metrics {
			if !metric.Required {
				continue
			}
			name := metric.FixtureName(prefix)
			if !families.HasMetric(name) {
				return fmt.Errorf("required metric %s (%s) missing from %s", metric.ID, name, fixturePath)
			}
		}
	}

	fmt.Printf("metric contract verified against %s\n", opts.FixtureDir)
	return nil
}

func (o VerifyOptions) withDefaults() VerifyOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "metrics", "openbao-core.yaml")
	}
	if o.FixtureDir == "" {
		o.FixtureDir = filepath.Join("fixtures", "captured", "openbao-2.5.4")
	}
	return o
}

func (c MetricContract) validateShape(path string) error {
	if err := validateMaturity(path, c.Maturity); err != nil {
		return err
	}
	if err := c.validateMetricContractShapeBasics(path); err != nil {
		return err
	}
	coverageProfiles, err := c.validateCoverageProfiles(path)
	if err != nil {
		return err
	}
	if err := c.validateMetrics(path, coverageProfiles); err != nil {
		return err
	}

	return nil
}

func (c MetricContract) validateMetricContractShapeBasics(path string) error {
	if c.OpenBAOVersion == "" {
		return fmt.Errorf("metric contract %s is missing openbaoVersion", path)
	}
	if len(c.MetricPrefixes.Supported) == 0 {
		return fmt.Errorf("metric contract %s has no supported metric prefixes", path)
	}
	if c.MetricPrefixes.Default == "" {
		return fmt.Errorf("metric contract %s is missing metricPrefixes.default", path)
	}
	if c.Normalization.RecordingRulePrefix == "" {
		return fmt.Errorf("metric contract %s is missing normalization.recordingRulePrefix", path)
	}
	if len(c.Metrics) == 0 {
		return fmt.Errorf("metric contract %s has no metrics", path)
	}
	return nil
}

func (c MetricContract) validateCoverageProfiles(path string) (map[string]bool, error) {
	coverageProfiles := map[string]bool{}
	for _, profile := range c.CoverageProfiles {
		if profile.ID == "" {
			return nil, fmt.Errorf("metric contract %s has a coverage profile without an id", path)
		}
		if coverageProfiles[profile.ID] {
			return nil, fmt.Errorf("metric contract %s has duplicate coverage profile id %q", path, profile.ID)
		}
		coverageProfiles[profile.ID] = true
		if profile.Class == "" {
			return nil, fmt.Errorf("coverage profile %s is missing class", profile.ID)
		}
		if err := validateMetricCoverageExpectation(
			profile.DefaultExpectation,
			fmt.Sprintf("coverage profile %s defaultExpectation", profile.ID),
		); err != nil {
			return nil, err
		}
	}
	return coverageProfiles, nil
}

func (c MetricContract) validateMetrics(path string, coverageProfiles map[string]bool) error {
	seen := map[string]bool{}
	for _, metric := range c.Metrics {
		if metric.ID == "" {
			return fmt.Errorf("metric contract %s has a metric without an id", path)
		}
		if seen[metric.ID] {
			return fmt.Errorf("metric contract %s has duplicate metric id %q", path, metric.ID)
		}
		seen[metric.ID] = true
		if metric.PrometheusName == "" {
			return fmt.Errorf("metric %s is missing prometheusName", metric.ID)
		}
		for profileID, expectation := range metric.Coverage {
			if !coverageProfiles[profileID] {
				return fmt.Errorf("metric %s has coverage for unknown profile %q", metric.ID, profileID)
			}
			if err := validateMetricCoverageExpectation(
				expectation,
				fmt.Sprintf("metric %s coverage for profile %s", metric.ID, profileID),
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateMetricCoverageExpectation(value, field string) error {
	switch value {
	case MetricCoverageRequired, MetricCoverageOptional, MetricCoverageNotApplicable:
		return nil
	default:
		return fmt.Errorf(
			"%s must be one of %q, %q, or %q",
			field,
			MetricCoverageRequired,
			MetricCoverageOptional,
			MetricCoverageNotApplicable,
		)
	}
}

func (m Metric) FixtureName(prefix string) string {
	name := m.FixturePrometheusName
	if name == "" {
		name = m.PrometheusName
	}
	return strings.ReplaceAll(name, "${p}", prefix)
}

func metricFixturePath(root, version, prefix string) string {
	return filepath.Join(root, "metrics", fmt.Sprintf("openbao-%s-%s-prefix.prom", version, prefix))
}
