package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const testTarget = "host.docker.internal:18200"

func TestFixturePreservesActualMetricsPipeline(t *testing.T) {
	source, err := os.ReadFile("../../" + profilePath)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := fixtureConfig(source, testTarget, "http://capture/candidate/services/collector")
	if err != nil {
		t.Fatal(err)
	}
	var original, fixture yaml.Node
	if err := yaml.Unmarshal(source, &original); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(generated, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, path := range [][]string{
		{"processors", "transform/metric_identity"},
		{"processors", "batch"},
		{"service", "pipelines", "metrics"},
	} {
		before := nodeAt(t, original.Content[0], path...)
		after := nodeAt(t, fixture.Content[0], path...)
		assertNodeEqual(t, before, after)
	}
	beforeJobs := nodeAt(t, original.Content[0], "receivers", "prometheus", "config", "scrape_configs")
	afterJobs := nodeAt(t, fixture.Content[0], "receivers", "prometheus", "config", "scrape_configs")
	if len(afterJobs.Content) != 1 {
		t.Fatalf("fixture has %d scrape jobs", len(afterJobs.Content))
	}
	for _, key := range []string{"job_name", "metrics_path", "params", "relabel_configs", "metric_relabel_configs"} {
		assertNodeEqual(t, nodeAt(t, beforeJobs.Content[0], key), nodeAt(t, afterJobs.Content[0], key))
	}
	pipelines := nodeAt(t, fixture.Content[0], "service", "pipelines")
	if len(pipelines.Content) != 2 {
		t.Fatal("fixture must not run log pipelines or the licensed Splunk service")
	}
	if _, err := lookup(fixture.Content[0], "extensions"); err == nil {
		t.Fatal("fixture must not initialize file-storage extensions")
	}
	missing := bytes.Replace(source, []byte("job_name: openbao\n"), []byte("job_name: renamed\n"), 1)
	if _, err := fixtureConfig(missing, testTarget, "http://capture"); err == nil {
		t.Fatal("renamed source job must fail instead of testing a replacement pipeline")
	}
}

func nodeAt(t *testing.T, node *yaml.Node, path ...string) *yaml.Node {
	t.Helper()
	result, err := lookup(node, path...)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertNodeEqual(t *testing.T, before, after *yaml.Node) {
	t.Helper()
	a, err := yaml.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	b, err := yaml.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("source component changed:\n%s\nfixture:\n%s", a, b)
	}
}

func baselineSeries() map[string]metricSample {
	rows := []struct {
		metric, namespace string
		value             float64
	}{
		{"vault_core_active", "team-a", 1},
		{"vault_core_active", "team-b", 0},
		{"vault_core_unsealed", "team-a", 1},
		{"vault_core_unsealed", "team-b", 1},
		{"vault_runtime_num_goroutines", "team-a", 7},
		{"vault_runtime_num_goroutines", "team-b", 11},
		{"vault_raft_peers", "team-a", 3},
		{"vault_raft_peers", "team-b", 3},
	}
	series := make(map[string]metricSample)
	for _, row := range rows {
		series[row.metric+"/"+row.namespace] = metricSample{
			Name: row.metric, Value: row.value, Host: testTarget, Index: "openbao_metrics", SourceType: "openbao:metrics",
			Dimensions: map[string]string{
				"cluster": "compose", "deployment_profile": "docker-compose", "environment": "local",
				"instance": testTarget, "job": "openbao", "pod": "host.docker.internal", "scrape_profile": "all_nodes",
				"openbao_namespace": row.namespace, "kubernetes_namespace": "workloads",
			},
		}
	}
	return series
}

func candidateSeries() map[string]metricSample {
	series := baselineSeries()
	for _, sample := range series {
		sample.Dimensions["__scheme__"] = "http"
		sample.Dimensions["__metrics_path__"] = "/v1/sys/metrics"
	}
	return series
}

func TestNamespaceIdentityAndApprovedDimensions(t *testing.T) {
	baseline, candidate := baselineSeries(), candidateSeries()
	if err := validateSeries(baseline, testTarget, false); err != nil {
		t.Fatal(err)
	}
	if err := validateSeries(candidate, testTarget, true); err != nil {
		t.Fatal(err)
	}
	if err := compareSeries(baseline, candidate); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]metricSample){
		"logical namespace removed": func(s map[string]metricSample) {
			delete(s["vault_core_active/team-a"].Dimensions, "openbao_namespace")
		},
		"deployment namespace replaced": func(s map[string]metricSample) {
			s["vault_core_active/team-a"].Dimensions["kubernetes_namespace"] = "team-a"
		},
		"source cluster overrides target": func(s map[string]metricSample) {
			s["vault_core_active/team-a"].Dimensions["cluster"] = "untrusted"
		},
		"raw namespace retained": func(s map[string]metricSample) {
			s["vault_core_active/team-a"].Dimensions["namespace"] = "team-a"
		},
		"wrong metadata value": func(s map[string]metricSample) {
			s["vault_core_active/team-a"].Dimensions["__metrics_path__"] = "/other"
		},
		"changed metric value": func(s map[string]metricSample) {
			v := s["vault_core_active/team-a"]
			v.Value = 0
			s["vault_core_active/team-a"] = v
		},
	} {
		t.Run(name, func(t *testing.T) {
			series := candidateSeries()
			mutate(series)
			if err := validateSeries(series, testTarget, true); err == nil {
				t.Fatal("invalid namespace, value or dimension passed qualification")
			}
		})
	}
	candidate["vault_core_active/team-a"].Dimensions["unexpected"] = "extra"
	if err := compareSeries(baseline, candidate); err == nil {
		t.Fatal("a third new dimension must fail baseline comparison")
	}
}

func eventFor(t *testing.T, sample metricSample) hecEvent {
	t.Helper()
	fields := make(map[string]json.RawMessage)
	for key, value := range sample.Dimensions {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		fields[key] = raw
	}
	value, err := json.Marshal(sample.Value)
	if err != nil {
		t.Fatal(err)
	}
	fields["metric_name:"+sample.Name] = value
	return hecEvent{Event: "metric", Host: sample.Host, Index: sample.Index, SourceType: sample.SourceType, Fields: fields}
}

func TestHECGzipStreamsAndNegativeControls(t *testing.T) {
	captured := newCapture()
	sample := baselineSeries()["vault_core_active/team-a"]
	event := eventFor(t, sample)
	var body bytes.Buffer
	compressed := gzip.NewWriter(&body)
	encoder := json.NewEncoder(compressed)
	for range 2 {
		if err := encoder.Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/baseline/services/collector", &body)
	request.Header.Set("Authorization", "Splunk "+fixtureToken)
	request.Header.Set("Content-Encoding", "gzip")
	response := httptest.NewRecorder()
	captured.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("HEC capture failed: %s", response.Body)
	}
	series, err := captured.snapshot("baseline")
	if err != nil || len(series) != 1 || series["vault_core_active/team-a"].Value != 1 {
		t.Fatalf("gzip event stream was not decoded: %v %v", series, err)
	}
	changed := event
	changed.Fields = maps.Clone(event.Fields)
	changed.Fields["metric_name:vault_core_active"] = json.RawMessage("0")
	captured.events["baseline"] = append(captured.events["baseline"], changed)
	if _, err := captured.snapshot("baseline"); err == nil {
		t.Fatal("unstable values across repeated scrapes must fail")
	}
	negative := eventFor(t, metricSample{Name: "qualification_unselected", Value: 999})
	captured.events["candidate"] = []hecEvent{negative}
	if _, err := captured.snapshot("candidate"); err == nil {
		t.Fatal("an unfiltered negative-control metric must fail")
	}
}

func TestMalformedOrUnauthenticatedHECDoesNotPass(t *testing.T) {
	for _, testCase := range []struct{ name, token, encoding, body string }{
		{"wrong token", "invalid", "", `{}`},
		{"bad encoding", fixtureToken, "br", `{}`},
		{"malformed JSON", fixtureToken, "", `{`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			captured := newCapture()
			request := httptest.NewRequest(http.MethodPost, "/candidate/services/collector", strings.NewReader(testCase.body))
			request.Header.Set("Authorization", "Splunk "+testCase.token)
			request.Header.Set("Content-Encoding", testCase.encoding)
			response := httptest.NewRecorder()
			captured.ServeHTTP(response, request)
			if response.Code == http.StatusOK {
				t.Fatal("bad HEC request was acknowledged")
			}
			if _, err := captured.snapshot("candidate"); err == nil {
				t.Fatal("bad request did not block qualification")
			}
		})
	}
}

func TestHostedOnlyGuardAndPinnedImage(t *testing.T) {
	t.Setenv("CI", "false")
	if err := run(context.Background(), t.TempDir()); err == nil {
		t.Fatal("local invocation must not start Docker or listeners")
	}
	compose, err := os.ReadFile("../../examples/splunk/compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	image, err := candidateImage(compose)
	if err != nil || !imagePattern.MatchString(image) || image == baselineImage {
		t.Fatalf("candidate image is not a new immutable pin: %s: %v", image, err)
	}
}
