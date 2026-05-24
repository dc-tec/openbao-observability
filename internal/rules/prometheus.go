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
	PrometheusName         string
	LokiName               string
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

	prometheusDocument := buildAlertPrometheusRule(*contract, sourcePrefix, opts.PrometheusName)
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

	lokiDocument := buildLokiAlertDocument(*contract, sourcePrefix, opts.LokiName)
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
	if o.PrometheusName == "" {
		o.PrometheusName = "openbao-alerts"
	}
	if o.LokiName == "" {
		o.LokiName = "openbao-loki-alerts"
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
							Record: recordPrefix + ":core_handle_request:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "core_handle_request"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_handle_request:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "core_handle_request"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_handle_login_request:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "core_handle_login_request"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_handle_login_request:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "core_handle_login_request"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_check_token:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "core_check_token"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_check_token:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "core_check_token"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_in_flight_requests:max",
							Expr:   "max(" + metricName(sourcePrefix, "core_in_flight_requests") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_mount_table_num_entries:max",
							Expr: "max by (local, type) (" + metricName(
								sourcePrefix,
								"core_mount_table_num_entries",
							) + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":core_mount_table_size:max",
							Expr:   "max by (local, type) (" + metricName(sourcePrefix, "core_mount_table_size") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":raft_peers:max",
							Expr:   raftPeerCountExpression(sourcePrefix),
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
							Record: recordPrefix + ":raft_storage_commit_index:max",
							Expr:   raftStorageMaxExpression(sourcePrefix, "raft_storage_stats_commit_index"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":raft_storage_applied_index:max",
							Expr:   raftStorageMaxExpression(sourcePrefix, "raft_storage_stats_applied_index"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":raft_storage_apply_gap:max",
							Expr:   raftStorageApplyGapExpression(sourcePrefix),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":raft_storage_fsm_pending:max",
							Expr:   raftStorageMaxExpression(sourcePrefix, "raft_storage_stats_fsm_pending"),
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
							Record: recordPrefix + ":audit_log_request:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "audit_log_request"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":audit_log_response:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "audit_log_response"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":audit_log_request:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "audit_log_request"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":audit_log_response:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "audit_log_response"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":expire_num_leases:max",
							Expr:   "max(" + metricName(sourcePrefix, "expire_num_leases") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":expire_num_irrevocable_leases:max",
							Expr:   "max(" + metricName(sourcePrefix, "expire_num_irrevocable_leases") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":expire_revoke:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "expire_revoke"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":expire_revoke:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "expire_revoke"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":expire_register_auth:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "expire_register_auth"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":expire_register_auth:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "expire_register_auth"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_count:max30m",
							Expr:   "max(max_over_time(" + metricName(sourcePrefix, "token_count") + "[30m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_creation:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "token_creation") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_creation_by_auth:increase15m",
							Expr: "sum by (auth_method) (increase(" + metricName(
								sourcePrefix,
								"token_creation",
							) + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_creation_by_namespace:increase15m",
							Expr: "sum by (namespace) (increase(" + metricName(
								sourcePrefix,
								"token_creation",
							) + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_creation_by_auth_namespace:increase15m",
							Expr: "sum by (namespace, auth_method) (increase(" + metricName(
								sourcePrefix,
								"token_creation",
							) + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_create:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "token_create"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_create:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "token_create"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_lookup:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "token_lookup"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_lookup:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "token_lookup"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_store:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "token_store"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_store:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "token_store"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_revoke_tree:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "token_revoke_tree"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":token_revoke_tree:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "token_revoke_tree"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":runtime_num_goroutines:max",
							Expr:   "max(" + metricName(sourcePrefix, "runtime_num_goroutines") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":runtime_alloc_bytes:max",
							Expr:   "max(" + metricName(sourcePrefix, "runtime_alloc_bytes") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":runtime_heap_objects:max",
							Expr:   "max(" + metricName(sourcePrefix, "runtime_heap_objects") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":runtime_sys_bytes:max",
							Expr:   "max(" + metricName(sourcePrefix, "runtime_sys_bytes") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":runtime_gc_pause_ns:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "runtime_gc_pause_ns"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":runtime_total_gc_pause_ns:max",
							Expr:   "max(" + metricName(sourcePrefix, "runtime_total_gc_pause_ns") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":runtime_total_gc_runs:max",
							Expr:   "max(" + metricName(sourcePrefix, "runtime_total_gc_runs") + ")",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_get:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "barrier_get"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_get:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "barrier_get"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_put:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "barrier_put"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_put:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "barrier_put"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_list:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "barrier_list"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_list:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "barrier_list"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_list_page:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "barrier_list_page"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_list_page:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "barrier_list_page"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_delete:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "barrier_delete"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_delete:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "barrier_delete"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":barrier_estimated_encryptions:increase15m",
							Expr: "sum by (term) (increase(" + metricName(
								sourcePrefix,
								"barrier_estimated_encryptions",
							) + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":cache_hit:rate5m",
							Expr:   "sum(rate(" + metricName(sourcePrefix, "cache_hit") + "[5m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":cache_miss:rate5m",
							Expr:   "sum(rate(" + metricName(sourcePrefix, "cache_miss") + "[5m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":cache_write:rate5m",
							Expr:   "sum(rate(" + metricName(sourcePrefix, "cache_write") + "[5m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":cache_hit_ratio:ratio5m",
							Expr:   cacheHitRatioExpression(sourcePrefix),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":secret_lease_creation:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "secret_lease_creation") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":secret_lease_creation_by_engine:increase15m",
							Expr: "sum by (secret_engine) (increase(" + metricName(
								sourcePrefix,
								"secret_lease_creation",
							) + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":secret_lease_creation_by_engine_namespace:increase15m",
							Expr: "sum by (namespace, secret_engine) (increase(" + metricName(
								sourcePrefix,
								"secret_lease_creation",
							) + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":pki_issue:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "pki_issue"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":pki_issue:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "pki_issue"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":pki_revoke:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "pki_revoke"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":pki_revoke:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "pki_revoke"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":pki_issue_failure:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "pki_issue_failure") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":pki_revoke_failure:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "pki_revoke_failure") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_initialize:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "database_Initialize"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_initialize:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "database_Initialize"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_close:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "database_Close"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_close:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "database_Close"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_new_user:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "database_NewUser"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_new_user:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "database_NewUser"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_update_user:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "database_UpdateUser"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_update_user:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "database_UpdateUser"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_delete_user:rate5m",
							Expr:   summaryRateExpression(sourcePrefix, "database_DeleteUser"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_delete_user:avg5m",
							Expr:   summaryAverageExpression(sourcePrefix, "database_DeleteUser"),
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_initialize_error:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "database_Initialize_error") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_close_error:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "database_Close_error") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_new_user_error:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "database_NewUser_error") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_update_user_error:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "database_UpdateUser_error") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
						{
							Record: recordPrefix + ":database_delete_user_error:increase15m",
							Expr:   "sum(increase(" + metricName(sourcePrefix, "database_DeleteUser_error") + "[15m]))",
							Labels: ruleLabels(sourcePrefix),
						},
					},
				},
			},
		},
	}
}

func buildAlertPrometheusRule(contract contracts.AlertContract, sourcePrefix, name string) prometheusRule {
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
			"name": name,
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

func buildLokiAlertDocument(contract contracts.AlertContract, sourcePrefix, name string) lokiAlertDocument {
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
			"name": name,
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

func raftPeerCountExpression(sourcePrefix string) string {
	rawPeerCount := "max(" + metricName(sourcePrefix, "raft_peers") + ")"
	storageStatsMetric := metricName(sourcePrefix, "raft_storage_stats_commit_index")
	storageStatsPeerCount := "count(count by (peer_id) (" + storageStatsMetric + "))"
	return rawPeerCount + " or " + storageStatsPeerCount
}

func raftStorageMaxExpression(sourcePrefix, id string) string {
	return "max by (instance, peer_id) (" + metricName(sourcePrefix, id) + ")"
}

func raftStorageApplyGapExpression(sourcePrefix string) string {
	commitIndex := metricName(sourcePrefix, "raft_storage_stats_commit_index")
	appliedIndex := metricName(sourcePrefix, "raft_storage_stats_applied_index")
	return "clamp_min(max by (instance, peer_id) (" + commitIndex + " - " + appliedIndex + "), 0)"
}

func summaryRateExpression(sourcePrefix, id string) string {
	return "sum(rate(" + metricName(sourcePrefix, id+"_count") + "[5m]))"
}

func summaryAverageExpression(sourcePrefix, id string) string {
	return "sum(rate(" + metricName(
		sourcePrefix,
		id+"_sum",
	) + "[5m])) / clamp_min(sum(rate(" + metricName(
		sourcePrefix,
		id+"_count",
	) + "[5m])), 0.001)"
}

func cacheHitRatioExpression(sourcePrefix string) string {
	hits := "sum(rate(" + metricName(sourcePrefix, "cache_hit") + "[5m]))"
	misses := "sum(rate(" + metricName(sourcePrefix, "cache_miss") + "[5m]))"
	return hits + " / clamp_min(" + hits + " + " + misses + ", 0.001)"
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
