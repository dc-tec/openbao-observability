package main

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

const samples = `# TYPE vault_core_active gauge
vault_core_active{namespace="team-a"} 1
vault_core_active{namespace="team-b"} 0
# TYPE vault_core_unsealed gauge
vault_core_unsealed{namespace="team-a",cluster="untrusted"} 1
vault_core_unsealed{namespace="team-b",cluster="untrusted"} 1
# TYPE vault_runtime_num_goroutines gauge
vault_runtime_num_goroutines{namespace="team-a"} 7
vault_runtime_num_goroutines{namespace="team-b"} 11
# TYPE vault_raft_peers gauge
vault_raft_peers{namespace="team-a"} 3
vault_raft_peers{namespace="team-b"} 3
# TYPE qualification_unselected gauge
qualification_unselected{namespace="team-a"} 999
`

var expectedMetrics = map[string][2]float64{
	"vault_core_active":            {1, 0},
	"vault_core_unsealed":          {1, 1},
	"vault_runtime_num_goroutines": {7, 11},
	"vault_raft_peers":             {3, 3},
}

type hecEvent struct {
	Event      string                     `json:"event"`
	Host       string                     `json:"host"`
	Index      string                     `json:"index"`
	SourceType string                     `json:"sourcetype"`
	Fields     map[string]json.RawMessage `json:"fields"`
}

type metricSample struct {
	Name       string            `json:"name"`
	Value      float64           `json:"value"`
	Host       string            `json:"host"`
	Index      string            `json:"index"`
	SourceType string            `json:"sourcetype"`
	Dimensions map[string]string `json:"dimensions"`
}

type capture struct {
	mu      sync.Mutex
	events  map[string][]hecEvent
	failure map[string]string
	scrapes atomic.Int64
}

func newCapture() *capture {
	return &capture{events: make(map[string][]hecEvent), failure: make(map[string]string)}
}

func (c *capture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/v1/sys/metrics" {
		if r.URL.Query().Get("format") != "prometheus" {
			http.Error(w, "expected configured Prometheus format", http.StatusBadRequest)
			return
		}
		c.scrapes.Add(1)
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = io.WriteString(w, samples)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(parts) != 2 || (parts[0] != "baseline" && parts[0] != "candidate") {
		http.NotFound(w, r)
		return
	}
	name := parts[0]
	err := readHEC(r, func(event hecEvent) {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.events[name] = append(c.events[name], event)
	})
	if err != nil {
		c.mu.Lock()
		c.failure[name] = err.Error()
		c.mu.Unlock()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"text":"Success","code":0}`)
}

func readHEC(r *http.Request, consume func(hecEvent)) error {
	if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Splunk "+fixtureToken {
		return errors.New("unexpected HEC method or fixture token")
	}
	if !strings.HasSuffix(r.URL.Path, "/services/collector") &&
		!strings.HasSuffix(r.URL.Path, "/services/collector/event") {
		return errors.New("unexpected HEC path")
	}
	body := io.LimitReader(r.Body, 16<<20)
	if encoding := r.Header.Get("Content-Encoding"); encoding == "gzip" {
		reader, err := gzip.NewReader(body)
		if err != nil {
			return err
		}
		defer func() { _ = reader.Close() }() // Decoding reports gzip checksum errors before EOF.
		body = io.LimitReader(reader, 16<<20)
	} else if encoding != "" {
		return fmt.Errorf("unsupported HEC content encoding %q", encoding)
	}
	decoder := json.NewDecoder(body)
	for {
		var event hecEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		consume(event)
	}
}

func parseSample(event hecEvent) (metricSample, error) {
	result := metricSample{Host: event.Host, Index: event.Index, SourceType: event.SourceType}
	result.Dimensions = make(map[string]string)
	if event.Event != "metric" {
		return result, errors.New("expected a HEC metric event")
	}
	for key, value := range event.Fields {
		if strings.HasPrefix(key, "metric_name:") {
			if result.Name != "" {
				return result, errors.New("profile unexpectedly uses multi-metric format")
			}
			result.Name = strings.TrimPrefix(key, "metric_name:")
			if err := json.Unmarshal(value, &result.Value); err != nil {
				return result, err
			}
			continue
		}
		var dimension string
		if err := json.Unmarshal(value, &dimension); err != nil {
			return result, fmt.Errorf("non-string dimension %q: %w", key, err)
		}
		result.Dimensions[key] = dimension
	}
	if result.Name == "" {
		return result, errors.New("metric event has no numeric metric field")
	}
	return result, nil
}

func (c *capture) snapshot(name string) (map[string]metricSample, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failure[name] != "" {
		return nil, errors.New(c.failure[name])
	}
	result := make(map[string]metricSample)
	for _, event := range c.events[name] {
		sample, err := parseSample(event)
		if err != nil {
			return nil, err
		}
		if sample.Name == "qualification_unselected" {
			return nil, errors.New("profile metric allowlist did not filter the negative control")
		}
		if _, selected := expectedMetrics[sample.Name]; !selected {
			continue // Scrape-health metrics are retained in raw evidence but are not test inputs.
		}
		key := sample.Name + "/" + sample.Dimensions["openbao_namespace"]
		if previous, exists := result[key]; exists && !reflect.DeepEqual(previous, sample) {
			return nil, fmt.Errorf("metric identity or values changed between scrapes: %s", key)
		}
		result[key] = sample
	}
	return result, nil
}

func validateSeries(series map[string]metricSample, target string, added bool) error {
	if len(series) != len(expectedMetrics)*2 {
		return fmt.Errorf("got %d series, want %d", len(series), len(expectedMetrics)*2)
	}
	for metric, values := range expectedMetrics {
		for i, namespace := range []string{"team-a", "team-b"} {
			key := metric + "/" + namespace
			sample, exists := series[key]
			if !exists || sample.Value != values[i] || sample.Host != target ||
				sample.Index != "openbao_metrics" || sample.SourceType != "openbao:metrics" {
				return fmt.Errorf("metric value or HEC identity mismatch: %s", key)
			}
			if err := validateDimensions(sample.Dimensions, target, namespace, added); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
	}
	return nil
}

func validateDimensions(dimensions map[string]string, target, namespace string, added bool) error {
	want := map[string]string{
		"cluster": "compose", "deployment_profile": "docker-compose", "environment": "local",
		"scrape_profile": "all_nodes", "instance": target, "job": "openbao",
		"pod": "host.docker.internal", "kubernetes_namespace": "workloads", "openbao_namespace": namespace,
	}
	for key, value := range want {
		if dimensions[key] != value {
			return fmt.Errorf("dimension %s = %q, want %q", key, dimensions[key], value)
		}
	}
	for _, key := range []string{"namespace", "exported_cluster"} {
		if _, present := dimensions[key]; present {
			return fmt.Errorf("unmapped or conflicting namespace/deployment label %q", key)
		}
	}
	for key, value := range map[string]string{"__scheme__": "http", "__metrics_path__": "/v1/sys/metrics"} {
		got, present := dimensions[key]
		if present != added || (added && got != value) {
			return fmt.Errorf("unexpected receiver metadata %s = %q (present=%t)", key, got, present)
		}
	}
	return nil
}

func compareSeries(baseline, candidate map[string]metricSample) error {
	for key, actual := range candidate {
		dimensions := make(map[string]string, len(actual.Dimensions))
		for name, value := range actual.Dimensions {
			if name != "__scheme__" && name != "__metrics_path__" {
				dimensions[name] = value
			}
		}
		actual.Dimensions = dimensions
		if !reflect.DeepEqual(baseline[key], actual) {
			return fmt.Errorf("dimensions, metric identity or values differ beyond approved metadata: %s", key)
		}
	}
	if len(baseline) != len(candidate) {
		return errors.New("baseline and candidate series sets differ")
	}
	return nil
}
