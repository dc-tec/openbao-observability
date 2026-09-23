package examples

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"gopkg.in/yaml.v3"
)

func TestSealRelabelingPreservesBothSources(t *testing.T) {
	paths := []string{
		"../../examples/docker-compose/prometheus/prometheus.yml",
		"../../examples/docker-compose/prometheus/prometheus.audit-archive.yml",
		"../../examples/kubernetes/all-node-metrics-scrape.yaml",
		"../../examples/kubernetes/secure-metrics-scrape.yaml",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			configs := metricRelabelConfigs(t, path)
			for _, prefix := range []string{"vault", "openbao"} {
				for _, cluster := range []string{"", "upstream-cluster"} {
					input := labels.FromMap(map[string]string{
						"__name__": prefix + "_core_unsealed", "cluster": "deployment",
						"exported_cluster": cluster, "instance": "node:8200", "namespace": "team-a",
					})
					builder := labels.NewBuilder(input)
					keep := relabel.ProcessBuilder(builder, configs...)
					got := builder.Labels()
					want := "startup"
					if cluster != "" {
						want = "cluster"
					}
					if !keep || got.Get("seal_state_source") != want || got.Get("cluster") != "deployment" {
						t.Fatalf("lost seal source or deployment identity: %v, keep=%v", got, keep)
					}
					if got.Has("exported_cluster") || got.Has("namespace") || got.Get("openbao_namespace") != "team-a" {
						t.Fatalf("incorrect source-label normalization: %v", got)
					}
				}
			}
		})
	}
}

func metricRelabelConfigs(t *testing.T, path string) []*relabel.Config {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	for {
		var doc map[string]any
		if err := decoder.Decode(&doc); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		var raw any
		if jobs, ok := doc["scrape_configs"].([]any); ok {
			raw = jobs[0].(map[string]any)["metric_relabel_configs"]
		} else if doc["kind"] == "ServiceMonitor" {
			spec := doc["spec"].(map[string]any)
			raw = spec["endpoints"].([]any)[0].(map[string]any)["metricRelabelings"]
		}
		if raw == nil {
			continue
		}
		data, err := yaml.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		replacer := strings.NewReplacer("sourceLabels:", "source_labels:", "targetLabel:", "target_label:")
		normalized := replacer.Replace(string(data))
		var configs []*relabel.Config
		if err := yaml.Unmarshal([]byte(normalized), &configs); err != nil {
			t.Fatal(err)
		}
		for _, config := range configs {
			if err := config.Validate(model.UTF8Validation); err != nil {
				t.Fatal(err)
			}
		}
		return configs
	}
	t.Fatalf("no metrics relabel configuration in %s", path)
	return nil
}
