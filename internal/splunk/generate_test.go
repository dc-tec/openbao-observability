package splunk

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dc-tec/openbao-observability/internal/contracts"
)

func testOptions(t *testing.T) Options {
	t.Helper()
	return Options{
		RepositoryRoot: filepath.Join("..", ".."),
		ContractPath:   filepath.Join("..", "..", "contracts/profiles/splunk-enterprise.yaml"), OutputDir: t.TempDir(),
	}
}

func TestGeneratedProfileParityAndOrphans(t *testing.T) {
	opts := testOptions(t)
	if err := Generate(opts); err != nil {
		t.Fatal(err)
	}
	if err := Verify(opts); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(opts.OutputDir, "coverage.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(opts); err == nil {
		t.Fatal("accepted stale coverage")
	}
	if err := Generate(opts); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opts.OutputDir, "orphan.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(opts); err == nil {
		t.Fatal("accepted orphan artifact")
	}
}

func TestStudioReferencesTimeBoundsAndMetadataProjection(t *testing.T) {
	artifacts, err := build(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	var dashboard studioDashboard
	data := artifacts["dashboards/openbao_audit_investigation.json"]
	if err := json.Unmarshal(data, &dashboard); err != nil {
		t.Fatal(err)
	}
	if len(dashboard.Visualizations) != 17 {
		t.Fatalf("got %d panels", len(dashboard.Visualizations))
	}
	assertStudioReferences(t, dashboard)
	var view studioView
	if err := xml.Unmarshal(artifacts["app/openbao_observability/default/data/ui/views/"+
		"openbao_audit_investigation.xml"], &view); err != nil {
		t.Fatal(err)
	}
	if view.Version != "2" || view.Definition.Value != string(data) {
		t.Fatal("Studio XML does not wrap generated JSON")
	}
	macros := string(artifacts["app/openbao_observability/default/macros.conf"])
	for _, fragment := range []string{
		`index="openbao_audit"`, "spath input=_raw path=request.id output=request_id",
		"| fields - _raw", "coalesce(caller,\"(missing)\")",
	} {
		if !strings.Contains(macros, fragment) {
			t.Fatalf("macro missing %s", fragment)
		}
	}
	assertNoSecretFields(t, artifacts)
}

func assertNoSecretFields(t *testing.T, artifacts map[string][]byte) {
	t.Helper()
	for name, artifact := range artifacts {
		if strings.Contains(string(artifact), "request.data") || strings.Contains(string(artifact), "response.data") {
			t.Fatalf("secret payload included in %s", name)
		}
	}
}

func TestCoverageTracksUnavailablePanels(t *testing.T) {
	opts := testOptions(t)
	profile, err := contracts.LoadSplunkProfile(opts.ContractPath, opts.RepositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := buildCoverage(opts.RepositoryRoot, *profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(coverage.Dashboards) != 15 {
		t.Fatalf("coverage has %d dashboards", len(coverage.Dashboards))
	}
	partial, mapped, total := 0, 0, 0
	for _, dashboard := range coverage.Dashboards {
		total += len(dashboard.IncludedPanels)
		if len(dashboard.RequiredSources) == 0 {
			t.Fatal("dashboard lacks data source requirements")
		}
		switch dashboard.Status {
		case "partial":
			partial++
			if len(dashboard.IncludedPanels) == 0 || len(dashboard.UnavailablePanels) == 0 {
				t.Fatal("inaccurate partial coverage")
			}
		case "mapped":
			mapped++
			if len(dashboard.UnavailablePanels) != 0 {
				t.Fatal("mapped dashboard has unavailable panels")
			}
		default:
			t.Fatal("dashboard family is unmapped")
		}
	}
	if partial != 0 || mapped != 15 || total != 289 {
		t.Fatalf("got partial=%d mapped=%d panels=%d", partial, mapped, total)
	}
}

func TestCoverageStatusFollowsProfileLifecycle(t *testing.T) {
	opts := testOptions(t)
	profile, err := contracts.LoadSplunkProfile(opts.ContractPath, opts.RepositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, lifecycle := range []string{"draft", "reference", "stable", "deprecated"} {
		t.Run(lifecycle, func(t *testing.T) {
			candidate := *profile
			candidate.Maturity.Lifecycle = lifecycle
			report, err := buildCoverage(opts.RepositoryRoot, candidate)
			if err != nil {
				t.Fatal(err)
			}
			if report.Status != lifecycle {
				t.Fatalf("coverage status = %q, want contract lifecycle %q", report.Status, lifecycle)
			}
		})
	}
}

func TestProfileOverridesIndexAndJSONPaths(t *testing.T) {
	profile := contracts.SplunkProfileContract{
		AuditIndex: "test_audit", AuditSourcetype: "test:audit",
		Fields: map[string]string{},
	}
	for _, field := range contracts.SplunkAuditFields() {
		profile.Fields[field] = "event." + field
	}
	result := macros(profile)
	if !strings.Contains(result, `index="test_audit" sourcetype="test:audit"`) ||
		!strings.Contains(result, "path=event.request_id output=request_id") {
		t.Fatal("overrides were lost")
	}
}

func assertStudioReferences(t *testing.T, dashboard studioDashboard) {
	t.Helper()
	for id, viz := range dashboard.Visualizations {
		source, ok := dashboard.DataSources[viz.DataSources.Primary]
		if !ok {
			t.Fatalf("unbound visualization %s", id)
		}
		if source.Options.QueryParameters.Earliest != "$global_time.earliest$" ||
			source.Options.QueryParameters.Latest != "$global_time.latest$" {
			t.Fatal("missing time bounds")
		}
		for _, token := range []string{"request_id", "request_path", "logical_namespace", "host"} {
			if !strings.Contains(source.Options.Query, "$"+token+"|s$") {
				t.Fatalf("unquoted token %s", token)
			}
		}
	}
	for _, item := range dashboard.Layout.LayoutDefinitions["layout_audit"].Structure {
		if _, ok := dashboard.Visualizations[item.Item]; !ok {
			t.Fatalf("unbound layout item %s", item.Item)
		}
	}
}

func TestMetricBindingsKeepFiltersUnitsAndPrefix(t *testing.T) {
	opts := testOptions(t)
	profile, err := contracts.LoadSplunkProfile(opts.ContractPath, opts.RepositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(opts.RepositoryRoot, "contracts/dashboards/openbao-overview.yaml")
	canonical, err := contracts.LoadDashboardContract(path)
	if err != nil {
		t.Fatal(err)
	}
	binding := profile.Dashboards[1]
	profile.MetricsPrefix = "openbao"
	document := buildDashboard(*canonical, binding, *profile)
	login := document.Visualizations["viz_login_latency"]
	if login.Options == nil || login.Options.Unit != "ms" ||
		!strings.Contains(login.Options.MajorValue, "seriesByName('value')") {
		t.Fatal("latency value lacks an explicit field or unit")
	}
	for _, source := range document.DataSources {
		if strings.Contains(source.Options.Query, "${p}") || strings.Contains(source.Options.Query, "vault_") {
			t.Fatal("source prefix was not replaced")
		}
		if !strings.HasPrefix(source.Options.Query, "| `openbao_metric_samples(") {
			continue
		}
		for _, filter := range []string{"$cluster|s$", "$instance|s$", "$scrape_profile|s$"} {
			if !strings.Contains(source.Options.Query, filter) {
				t.Fatalf("missing quoted metric filter %s", filter)
			}
		}
	}
	if document.Inputs["input_cluster"].Options.DefaultValue != "compose" ||
		document.Inputs["input_scrape_profile"].Options.DefaultValue != "all_nodes" {
		t.Fatal("configured deployment defaults were lost")
	}
	if _, ok := document.Inputs["input_request_id"]; !ok {
		t.Fatal("mixed overview lacks audit filters")
	}
}

func TestExternalScopesAndFixedWindows(t *testing.T) {
	artifacts, err := build(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	var document studioDashboard
	if err := json.Unmarshal(artifacts["dashboards/openbao_slo_availability.json"], &document); err != nil {
		t.Fatal(err)
	}
	availability := document.DataSources["ds_availability_30d"].Options
	if availability.QueryParameters.Earliest != "-30d" || availability.QueryParameters.Latest != "now" {
		t.Fatal("fixed availability window is affected by the picker")
	}
	if strings.Contains(availability.Query, "$scrape_profile|s$") ||
		!strings.Contains(availability.Query, "$probe_job|s$") {
		t.Fatal("external probes inherit OpenBao scrape filters or lose their job filter")
	}
	if document.Inputs["input_slo_target"].Options.DefaultValue != "0.999" {
		t.Fatal("missing SLO objective default")
	}
}

func TestOperationalProjectionAndSourceFilters(t *testing.T) {
	artifacts, err := build(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	var document studioDashboard
	if err := json.Unmarshal(artifacts["dashboards/openbao_operational_logs.json"], &document); err != nil {
		t.Fatal(err)
	}
	for _, source := range document.DataSources {
		if !strings.Contains(source.Options.Query, "$host|s$") ||
			strings.Contains(source.Options.Query, "$request_id|s$") {
			t.Fatal("operational logs are filtered using audit-only fields")
		}
	}
	if _, ok := document.Inputs["input_request_id"]; ok {
		t.Fatal("operational view exposes an unused audit filter")
	}
	projection := operationalNormalization()
	for _, field := range []string{"@level", "@module", "@message", "fields - _raw"} {
		if !strings.Contains(projection, field) {
			t.Fatalf("operational projection lacks %s", field)
		}
	}
}

func TestAllStudioLayoutsAvoidOverlap(t *testing.T) {
	artifacts, err := build(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range artifacts {
		if !strings.HasPrefix(path, "dashboards/") {
			continue
		}
		var document studioDashboard
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		for _, layout := range document.Layout.LayoutDefinitions {
			for i, item := range layout.Structure {
				box := item.Position
				if box.X < 0 || box.Y < 0 || box.X+box.W > layout.Options.Width || box.Y+box.H > layout.Options.Height {
					t.Fatalf("%s: %s lies outside the canvas", path, item.Item)
				}
				for _, other := range layout.Structure[i+1:] {
					right := other.Position
					if rectanglesOverlap(box, right) {
						t.Fatalf("%s: %s overlaps %s", path, item.Item, other.Item)
					}
				}
			}
		}
	}
}

func rectanglesOverlap(left, right position) bool {
	return left.X < right.X+right.W && right.X < left.X+left.W && left.Y < right.Y+right.H && right.Y < left.Y+left.H
}
