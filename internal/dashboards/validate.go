package dashboards

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dc-tec/openbao-observability/internal/contracts"
	promapi "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type QueryValidationOptions struct {
	ContractPaths  []string
	GeneratedPaths []string
	PrometheusURL  string
	LokiURL        string
	Range          time.Duration
	Step           time.Duration
	Timeout        time.Duration
}

type dashboardQuery struct {
	Source     string
	Dashboard  string
	PanelID    string
	PanelTitle string
	Signal     string
	Expression string
}

type lokiResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		Result json.RawMessage `json:"result"`
	} `json:"data"`
}

func ValidateDashboardQueries(ctx context.Context, opts QueryValidationOptions) error {
	opts = opts.withDefaults()

	queries, err := loadDashboardQueries(opts)
	if err != nil {
		return err
	}
	if len(queries) == 0 {
		return fmt.Errorf("no dashboard queries found")
	}

	prometheusAPI, err := newPrometheusAPI(opts.PrometheusURL)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: opts.Timeout}
	window := v1.Range{
		Start: time.Now().Add(-opts.Range),
		End:   time.Now(),
		Step:  opts.Step,
	}

	seen := map[string]bool{}
	metricQueries := 0
	logQueries := 0
	for _, query := range queries {
		key := query.Signal + "\x00" + query.Expression
		if seen[key] {
			continue
		}
		seen[key] = true

		switch query.Signal {
		case dashboardSignalMetrics:
			metricQueries++
			if err := validatePrometheusQuery(ctx, prometheusAPI, query, window, opts.Timeout); err != nil {
				return err
			}
		case dashboardSignalLogs:
			logQueries++
			if err := validateLokiQuery(ctx, client, opts.LokiURL, query, opts.Range, opts.Step); err != nil {
				return err
			}
		default:
			return fmt.Errorf("query %s has unsupported signal %q", query.describe(), query.Signal)
		}
	}

	fmt.Printf("validated %d dashboard queries against Prometheus and Loki (%d metrics, %d logs)\n", metricQueries+logQueries, metricQueries, logQueries)
	return nil
}

func (o QueryValidationOptions) withDefaults() QueryValidationOptions {
	if len(o.ContractPaths) == 0 {
		o.ContractPaths = []string{
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
	if len(o.GeneratedPaths) == 0 {
		o.GeneratedPaths = []string{
			filepath.Join("generated", "grafana", "openbao-overview.json"),
			filepath.Join("generated", "grafana", "openbao-ha-raft.json"),
			filepath.Join("generated", "grafana", "openbao-audit-overview.json"),
			filepath.Join("generated", "grafana", "openbao-operational-logs.json"),
			filepath.Join("generated", "grafana", "openbao-audit-investigation.json"),
			filepath.Join("generated", "grafana", "openbao-auth-identity.json"),
			filepath.Join("generated", "grafana", "openbao-token-lease-lifecycle.json"),
			filepath.Join("generated", "grafana", "openbao-database-secrets.json"),
			filepath.Join("generated", "grafana", "openbao-transit.json"),
			filepath.Join("generated", "grafana", "openbao-pki.json"),
			filepath.Join("generated", "grafana", "openbao-secret-engines-mounts.json"),
			filepath.Join("generated", "grafana", "openbao-runtime-storage.json"),
			filepath.Join("generated", "grafana", "openbao-kubernetes-platform.json"),
			filepath.Join("generated", "grafana", "openbao-slo-availability.json"),
		}
	}
	if o.PrometheusURL == "" {
		o.PrometheusURL = "http://127.0.0.1:19090"
	}
	if o.LokiURL == "" {
		o.LokiURL = "http://127.0.0.1:13100"
	}
	if o.Range == 0 {
		o.Range = 15 * time.Minute
	}
	if o.Step == 0 {
		o.Step = 30 * time.Second
	}
	if o.Timeout == 0 {
		o.Timeout = 10 * time.Second
	}
	return o
}

func loadDashboardQueries(opts QueryValidationOptions) ([]dashboardQuery, error) {
	queries := []dashboardQuery{}
	for _, path := range opts.ContractPaths {
		if path == "" {
			continue
		}
		contract, err := contracts.LoadDashboardContract(path)
		if err != nil {
			return nil, err
		}
		if err := contract.ValidateExpressions(); err != nil {
			return nil, err
		}
		for _, panel := range contract.Panels {
			queries = append(queries, dashboardQuery{
				Source:     path,
				Dashboard:  contract.Title,
				PanelID:    panel.ID,
				PanelTitle: panel.Title,
				Signal:     panel.Signal,
				Expression: contract.ExpressionWithDefaultVariables(panel.Expression),
			})
		}
	}

	for _, path := range opts.GeneratedPaths {
		if path == "" {
			continue
		}
		document, err := loadGeneratedDashboard(path)
		if err != nil {
			return nil, err
		}
		variableDefaults := document.variableDefaults()
		for _, panel := range document.Panels {
			for _, target := range panel.Targets {
				signal := signalFromDatasource(target.Datasource.Type)
				if signal == "" {
					signal = signalFromDatasource(panel.Datasource.Type)
				}
				if signal == "" {
					return nil, fmt.Errorf("generated dashboard %s panel %q has unsupported datasource type %q", path, panel.Title, target.Datasource.Type)
				}
				queries = append(queries, dashboardQuery{
					Source:     path,
					Dashboard:  document.Title,
					PanelID:    fmt.Sprintf("%d", panel.ID),
					PanelTitle: panel.Title,
					Signal:     signal,
					Expression: contracts.InterpolateDashboardVariables(target.Expr, variableDefaults),
				})
			}
		}
	}

	return queries, nil
}

func loadGeneratedDashboard(path string) (*grafanaDashboard, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read generated dashboard %s: %w", path, err)
	}

	var document grafanaDashboard
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parse generated dashboard %s: %w", path, err)
	}
	if document.Title == "" {
		return nil, fmt.Errorf("generated dashboard %s is missing title", path)
	}
	if len(document.Panels) == 0 {
		return nil, fmt.Errorf("generated dashboard %s has no panels", path)
	}
	return &document, nil
}

func signalFromDatasource(datasourceType string) string {
	switch datasourceType {
	case datasourceTypePrometheus:
		return dashboardSignalMetrics
	case datasourceTypeLoki:
		return dashboardSignalLogs
	default:
		return ""
	}
}

func newPrometheusAPI(address string) (v1.API, error) {
	client, err := promapi.NewClient(promapi.Config{Address: strings.TrimRight(address, "/")})
	if err != nil {
		return nil, fmt.Errorf("create Prometheus client for %s: %w", address, err)
	}
	return v1.NewAPI(client), nil
}

func validatePrometheusQuery(ctx context.Context, api v1.API, query dashboardQuery, window v1.Range, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, warnings, err := api.QueryRange(ctx, query.Expression, window, v1.WithTimeout(timeout))
	if err != nil {
		return fmt.Errorf("validate PromQL for %s: %w", query.describe(), err)
	}
	if len(warnings) > 0 {
		fmt.Printf("Prometheus warnings for %s: %s\n", query.describe(), strings.Join(warnings, "; "))
	}
	return nil
}

func validateLokiQuery(ctx context.Context, client *http.Client, baseURL string, query dashboardQuery, queryRange, step time.Duration) error {
	now := time.Now()
	end := now
	start := now.Add(-queryRange)

	endpoint, err := url.Parse(strings.TrimRight(baseURL, "/") + "/loki/api/v1/query_range")
	if err != nil {
		return fmt.Errorf("parse Loki URL %s: %w", baseURL, err)
	}
	values := endpoint.Query()
	values.Set("query", query.Expression)
	values.Set("start", fmt.Sprintf("%d", start.UnixNano()))
	values.Set("end", fmt.Sprintf("%d", end.UnixNano()))
	values.Set("step", step.String())
	values.Set("limit", "1")
	endpoint.RawQuery = values.Encode()

	ctx, cancel := context.WithTimeout(ctx, client.Timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create Loki request for %s: %w", query.describe(), err)
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("query Loki for %s: %w", query.describe(), err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	var body lokiResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return fmt.Errorf("decode Loki response for %s: %w", query.describe(), err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if body.Error != "" {
			return fmt.Errorf("validate LogQL for %s: Loki returned HTTP %d: %s", query.describe(), response.StatusCode, body.Error)
		}
		return fmt.Errorf("validate LogQL for %s: Loki returned HTTP %d", query.describe(), response.StatusCode)
	}
	if body.Status != "success" {
		if body.Error != "" {
			return fmt.Errorf("validate LogQL for %s: Loki status %q: %s", query.describe(), body.Status, body.Error)
		}
		return fmt.Errorf("validate LogQL for %s: Loki status %q", query.describe(), body.Status)
	}
	return nil
}

func (q dashboardQuery) describe() string {
	panel := q.PanelTitle
	if q.PanelID != "" {
		panel = panel + " (" + q.PanelID + ")"
	}
	return fmt.Sprintf("%s panel %s from %s", q.Dashboard, panel, q.Source)
}
