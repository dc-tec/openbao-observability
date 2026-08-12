package dashboards

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

func TestValidateDashboardQueries(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "dashboard.yaml")
	generatedPath := filepath.Join(dir, "dashboard.json")

	contractContent := strings.Replace(
		dashboardContract(),
		"openbao:autopilot_node_healthy:min",
		"openbao:core_active:sum",
		1,
	)
	if err := os.WriteFile(contractPath, []byte(contractContent), 0o644); err != nil {
		t.Fatalf("write dashboard contract: %v", err)
	}
	contract, err := contracts.LoadDashboardContract(contractPath)
	if err != nil {
		t.Fatalf("load dashboard contract: %v", err)
	}
	generated := buildGrafanaDashboard(*contract)
	generatedContent, err := json.Marshal(generated)
	if err != nil {
		t.Fatalf("marshal generated dashboard: %v", err)
	}
	if err := os.WriteFile(generatedPath, generatedContent, 0o644); err != nil {
		t.Fatalf("write generated dashboard: %v", err)
	}

	prometheusQueries := map[string]int{}
	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse Prometheus form: %v", err)
		}
		prometheusQueries[r.URL.Path+"\x00"+r.Form.Get("query")]++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/query":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		case "/api/v1/query_range":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		default:
			t.Fatalf("unexpected Prometheus path %s", r.URL.Path)
		}
	}))
	defer prometheus.Close()

	lokiQueries := map[string]int{}
	loki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/query_range" {
			t.Fatalf("unexpected Loki path %s", r.URL.Path)
		}
		lokiQueries[r.URL.Query().Get("query")]++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[]}}`))
	}))
	defer loki.Close()

	err = ValidateDashboardQueries(context.Background(), QueryValidationOptions{
		ContractPaths:  []string{contractPath},
		GeneratedPaths: []string{generatedPath},
		PrometheusURL:  prometheus.URL,
		LokiURL:        loki.URL,
		Range:          time.Minute,
		Step:           15 * time.Second,
		Timeout:        time.Second,
	})
	if err != nil {
		t.Fatalf("ValidateDashboardQueries returned error: %v", err)
	}

	instantKey := "/api/v1/query\x00openbao:core_active:sum"
	if prometheusQueries[instantKey] != 1 {
		t.Fatalf("Prometheus instant query count = %d, want 1", prometheusQueries[instantKey])
	}
	rangeKey := "/api/v1/query_range\x00openbao:core_active:sum"
	if prometheusQueries[rangeKey] != 1 {
		t.Fatalf("Prometheus range query count = %d, want 1", prometheusQueries[rangeKey])
	}
	auditQuery := `{log_stream="openbao.audit"} | json request_id="request.id" | request_id=~".*"`
	if lokiQueries[auditQuery] != 1 {
		t.Fatalf("Loki query count = %d, want 1", lokiQueries[auditQuery])
	}
}

func TestValidateDashboardQueriesReturnsLokiErrors(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "dashboard.yaml")
	if err := os.WriteFile(contractPath, []byte(dashboardContract()), 0o644); err != nil {
		t.Fatalf("write dashboard contract: %v", err)
	}

	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/query" {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer prometheus.Close()

	loki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":"error","error":"parse error"}`))
	}))
	defer loki.Close()

	err := ValidateDashboardQueries(context.Background(), QueryValidationOptions{
		ContractPaths:  []string{contractPath},
		GeneratedPaths: []string{},
		PrometheusURL:  prometheus.URL,
		LokiURL:        loki.URL,
		Range:          time.Minute,
		Step:           15 * time.Second,
		Timeout:        time.Second,
	})
	if err == nil {
		t.Fatal("expected Loki error to fail validation")
	}
}

func TestGeneratedQueryModeRejectsInvalidPrometheusModes(t *testing.T) {
	tests := []struct {
		name      string
		panelType string
		target    grafanaTarget
		want      dashboardQueryMode
		wantError string
	}{
		{
			name:      "stat instant",
			panelType: "stat",
			target:    grafanaTarget{Instant: true, Range: boolPointer(false)},
			want:      dashboardQueryModeInstant,
		},
		{
			name:      "time series range",
			panelType: "timeseries",
			target:    grafanaTarget{Range: boolPointer(true)},
			want:      dashboardQueryModeRange,
		},
		{
			name:      "both modes",
			panelType: "stat",
			target:    grafanaTarget{Instant: true, Range: boolPointer(true)},
			wantError: "enable exactly one query mode",
		},
		{
			name:      "neither mode",
			panelType: "stat",
			target:    grafanaTarget{Range: boolPointer(false)},
			wantError: "enable exactly one query mode",
		},
		{
			name:      "missing range flag",
			panelType: "stat",
			target:    grafanaTarget{Instant: true},
			wantError: "set range explicitly",
		},
		{
			name:      "stat range",
			panelType: "stat",
			target:    grafanaTarget{Range: boolPointer(true)},
			wantError: "stat target must use instant mode",
		},
		{
			name:      "time series instant",
			panelType: "timeseries",
			target:    grafanaTarget{Instant: true, Range: boolPointer(false)},
			wantError: "timeseries target must use range mode",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mode, err := generatedQueryMode(dashboardSignalMetrics, test.panelType, test.target)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want text %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("generatedQueryMode returned error: %v", err)
			}
			if mode != test.want {
				t.Fatalf("mode = %q, want %q", mode, test.want)
			}
		})
	}
}
