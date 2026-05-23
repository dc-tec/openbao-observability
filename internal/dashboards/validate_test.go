package dashboards

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

func TestValidateDashboardQueries(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "dashboard.yaml")
	generatedPath := filepath.Join(dir, "dashboard.json")

	if err := os.WriteFile(contractPath, []byte(dashboardContract()), 0o644); err != nil {
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
		if r.URL.Path != "/api/v1/query_range" {
			t.Fatalf("unexpected Prometheus path %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse Prometheus form: %v", err)
		}
		prometheusQueries[r.Form.Get("query")]++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
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

	if prometheusQueries["openbao:core_active:sum"] != 1 {
		t.Fatalf("Prometheus query count = %d, want 1", prometheusQueries["openbao:core_active:sum"])
	}
	if lokiQueries[`{log_stream="openbao.audit"}`] != 1 {
		t.Fatalf("Loki query count = %d, want 1", lokiQueries[`{log_stream="openbao.audit"}`])
	}
}

func TestValidateDashboardQueriesReturnsLokiErrors(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "dashboard.yaml")
	if err := os.WriteFile(contractPath, []byte(dashboardContract()), 0o644); err != nil {
		t.Fatalf("write dashboard contract: %v", err)
	}

	prometheus := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
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
