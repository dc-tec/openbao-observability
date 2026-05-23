package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dc-tec/openbao-observability/internal/contracts"
	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

type GenerateOptions struct {
	ContractPath string
	OutputPath   string
	RuleFilePath string
	SourcePrefix string
}

type GenerateAlertOptions struct {
	ContractPath           string
	PrometheusOutputPath   string
	PrometheusRuleFilePath string
	LokiOutputPath         string
	SourcePrefix           string
}

type prometheusRule struct {
	APIVersion string             `yaml:"apiVersion"`
	Kind       string             `yaml:"kind"`
	Metadata   map[string]string  `yaml:"metadata"`
	Spec       prometheusRuleSpec `yaml:"spec"`
}

type prometheusRuleSpec struct {
	Groups []prometheusRuleGroup `yaml:"groups"`
}

type prometheusRuleFile struct {
	Groups []prometheusRuleGroup `yaml:"groups"`
}

type prometheusRuleGroup struct {
	Name  string               `yaml:"name"`
	Rules []prometheusRuleItem `yaml:"rules"`
}

type prometheusRuleItem struct {
	Alert       string            `yaml:"alert,omitempty"`
	Record      string            `yaml:"record,omitempty"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

type lokiAlertDocument struct {
	APIVersion string                `yaml:"apiVersion"`
	Kind       string                `yaml:"kind"`
	Metadata   map[string]string     `yaml:"metadata"`
	Spec       lokiAlertDocumentSpec `yaml:"spec"`
}

type lokiAlertDocumentSpec struct {
	Groups []lokiAlertGroup `yaml:"groups"`
}

type lokiAlertGroup struct {
	Name  string          `yaml:"name"`
	Rules []lokiAlertRule `yaml:"rules"`
}

type lokiAlertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

func GeneratePrometheusRules(opts GenerateOptions) error {
	opts = opts.withDefaults()

	contract, err := contracts.LoadMetricContract(opts.ContractPath)
	if err != nil {
		return err
	}

	sourcePrefix := opts.SourcePrefix
	if sourcePrefix == "" {
		sourcePrefix = contract.MetricPrefixes.Default
	}

	document := buildPrometheusRule(*contract, sourcePrefix)
	if err := validatePromQL(document); err != nil {
		return err
	}
	if err := writeYAML(opts.OutputPath, document); err != nil {
		return err
	}
	fmt.Printf("generated PrometheusRule at %s\n", opts.OutputPath)

	if opts.RuleFilePath != "" {
		if err := writeYAML(opts.RuleFilePath, buildPrometheusRuleFile(document)); err != nil {
			return err
		}
		fmt.Printf("generated Prometheus rule file at %s\n", opts.RuleFilePath)
	}

	return nil
}

func GenerateAlertRules(opts GenerateAlertOptions) error {
	opts = opts.withDefaults()

	contract, err := contracts.LoadAlertContract(opts.ContractPath)
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

	prometheusDocument := buildAlertPrometheusRule(*contract, sourcePrefix)
	if err := validatePromQL(prometheusDocument); err != nil {
		return err
	}
	if err := writeYAML(opts.PrometheusOutputPath, prometheusDocument); err != nil {
		return err
	}
	fmt.Printf("generated Prometheus alert rules at %s\n", opts.PrometheusOutputPath)

	if opts.PrometheusRuleFilePath != "" {
		if err := writeYAML(opts.PrometheusRuleFilePath, buildPrometheusRuleFile(prometheusDocument)); err != nil {
			return err
		}
		fmt.Printf("generated Prometheus alert rule file at %s\n", opts.PrometheusRuleFilePath)
	}

	lokiDocument := buildLokiAlertDocument(*contract, sourcePrefix)
	if err := writeYAML(opts.LokiOutputPath, lokiDocument); err != nil {
		return err
	}
	fmt.Printf("generated Loki alert rules at %s\n", opts.LokiOutputPath)

	return nil
}

func (o GenerateOptions) withDefaults() GenerateOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "metrics", "openbao-core.yaml")
	}
	if o.OutputPath == "" {
		o.OutputPath = filepath.Join("generated", "prometheusrules", "openbao-recording-rules.yaml")
	}
	return o
}

func (o GenerateAlertOptions) withDefaults() GenerateAlertOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "alerts", "critical.yaml")
	}
	if o.PrometheusOutputPath == "" {
		o.PrometheusOutputPath = filepath.Join("generated", "prometheusrules", "openbao-alerts.yaml")
	}
	if o.LokiOutputPath == "" {
		o.LokiOutputPath = filepath.Join("generated", "loki", "openbao-alerts.yaml")
	}
	return o
}

func buildPrometheusRuleFile(document prometheusRule) prometheusRuleFile {
	return prometheusRuleFile{
		Groups: document.Spec.Groups,
	}
}

func buildPrometheusRule(contract contracts.MetricContract, sourcePrefix string) prometheusRule {
	recordPrefix := contract.Normalization.RecordingRulePrefix

	return prometheusRule{
		APIVersion: "monitoring.coreos.com/v1",
		Kind:       "PrometheusRule",
		Metadata: map[string]string{
			"name": "openbao-recording-rules",
		},
		Spec: prometheusRuleSpec{
			Groups: []prometheusRuleGroup{
				{
					Name: "openbao.recording",
					Rules: []prometheusRuleItem{
						{
							Record: recordPrefix + ":core_active:sum",
							Expr:   "sum(" + metricName(sourcePrefix, "core_active") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_unsealed:sum",
							Expr:   `sum(` + metricName(sourcePrefix, "core_unsealed") + `{cluster!=""})`,
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":raft_peers:max",
							Expr:   "max(" + metricName(sourcePrefix, "raft_peers") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":autopilot_healthy:max",
							Expr:   "max(" + metricName(sourcePrefix, "autopilot_healthy") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":autopilot_failure_tolerance:max",
							Expr:   "max(" + metricName(sourcePrefix, "autopilot_failure_tolerance") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":autopilot_node_healthy:min",
							Expr:   "min by (node_id) (" + metricName(sourcePrefix, "autopilot_node_healthy") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":audit_log_request_failure:increase5m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "audit_log_request_failure") + "[5m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":audit_log_response_failure:increase5m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "audit_log_response_failure") + "[5m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":expire_num_leases:max",
							Expr:   "max(" + metricName(sourcePrefix, "expire_num_leases") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":runtime_num_goroutines:max",
							Expr:   "max(" + metricName(sourcePrefix, "runtime_num_goroutines") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
					},
				},
			},
		},
	}
}

func buildAlertPrometheusRule(contract contracts.AlertContract, sourcePrefix string) prometheusRule {
	rules := []prometheusRuleItem{}
	for _, alert := range contract.Alerts {
		if alert.Type != "prometheus" {
			continue
		}
		rules = append(rules, prometheusRuleItem{
			Alert:       alert.ID,
			Expr:        contract.RenderExpression(alert.Expression, sourcePrefix),
			For:         alert.For,
			Labels:      alertLabels(alert, sourcePrefix),
			Annotations: alertAnnotations(alert),
		})
	}

	return prometheusRule{
		APIVersion: "monitoring.coreos.com/v1",
		Kind:       "PrometheusRule",
		Metadata: map[string]string{
			"name": "openbao-alerts",
		},
		Spec: prometheusRuleSpec{
			Groups: []prometheusRuleGroup{
				{
					Name:  "openbao.alerts",
					Rules: rules,
				},
			},
		},
	}
}

func buildLokiAlertDocument(contract contracts.AlertContract, sourcePrefix string) lokiAlertDocument {
	rules := []lokiAlertRule{}
	for _, alert := range contract.Alerts {
		if alert.Type != "loki" {
			continue
		}
		rules = append(rules, lokiAlertRule{
			Alert:       alert.ID,
			Expr:        contract.RenderExpression(alert.Expression, sourcePrefix),
			For:         alert.For,
			Labels:      alertLabels(alert, sourcePrefix),
			Annotations: alertAnnotations(alert),
		})
	}

	return lokiAlertDocument{
		APIVersion: "openbao.observability/v1alpha1",
		Kind:       "LokiAlertRules",
		Metadata: map[string]string{
			"name": "openbao-loki-alerts",
		},
		Spec: lokiAlertDocumentSpec{
			Groups: []lokiAlertGroup{
				{
					Name:  "openbao.loki.alerts",
					Rules: rules,
				},
			},
		},
	}
}

func metricName(prefix, id string) string {
	return prefix + "_" + strings.TrimPrefix(id, "_")
}

func validatePromQL(document prometheusRule) error {
	promQLParser := parser.NewParser(parser.Options{})
	for _, group := range document.Spec.Groups {
		for _, rule := range group.Rules {
			name := rule.Record
			if name == "" {
				name = rule.Alert
			}
			if _, err := promQLParser.ParseExpr(rule.Expr); err != nil {
				return fmt.Errorf("parse PromQL for rule %s: %w", name, err)
			}
		}
	}
	return nil
}

func ruleLabels(sourcePrefix string) map[string]string {
	return map[string]string{
		"source_prefix": sourcePrefix,
	}
}

func alertLabels(alert contracts.Alert, sourcePrefix string) map[string]string {
	labels := map[string]string{
		"severity":      alert.Severity,
		"signal":        alert.Signal,
		"source_prefix": sourcePrefix,
	}
	for key, value := range alert.Labels {
		labels[key] = value
	}
	return labels
}

func alertAnnotations(alert contracts.Alert) map[string]string {
	annotations := map[string]string{
		"summary":     alert.Summary,
		"description": alert.Description,
		"runbook_url": alert.Runbook,
	}
	for key, value := range alert.Annotations {
		annotations[key] = value
	}
	return annotations
}

func writeYAML(path string, document any) error {
	content, err := yaml.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal YAML for %s: %w", path, err)
	}

	header := []byte("# Code generated by openbao-observability; DO NOT EDIT.\n")
	content = append(header, content...)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
