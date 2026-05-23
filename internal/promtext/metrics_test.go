package promtext

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadFamiliesParsesMetricsAndLabels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.prom")
	content := `# HELP openbao_core_unsealed OpenBao unsealed status.
# TYPE openbao_core_unsealed gauge
openbao_core_unsealed{cluster=""} 0
openbao_core_unsealed{cluster="vault-cluster-test"} 1
openbao_runtime_num_goroutines 42
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	families, err := LoadFamilies(path)
	if err != nil {
		t.Fatalf("LoadFamilies returned error: %v", err)
	}

	if !families.HasMetric("openbao_core_unsealed") {
		t.Fatal("expected openbao_core_unsealed metric")
	}
	if !families.HasMetricWithLabel("openbao_core_unsealed", "cluster", "") {
		t.Fatal("expected empty cluster label series")
	}
	if !families.HasMetricWithLabel("openbao_core_unsealed", "cluster", "vault-cluster-test") {
		t.Fatal("expected real cluster label series")
	}

	expectedNames := []string{"openbao_core_unsealed", "openbao_runtime_num_goroutines"}
	if got := families.Names(); !reflect.DeepEqual(got, expectedNames) {
		t.Fatalf("metric names mismatch:\n got: %v\nwant: %v", got, expectedNames)
	}
}

func TestLoadFamiliesRejectsInvalidPrometheusText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.prom")
	if err := os.WriteFile(path, []byte("not valid prometheus text"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := LoadFamilies(path); err == nil {
		t.Fatal("expected invalid Prometheus text to fail")
	}
}
