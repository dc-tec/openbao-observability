package compatibility

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dc-tec/openbao-observability/internal/contracts"
	"github.com/dc-tec/openbao-observability/internal/promtext"
	dto "github.com/prometheus/client_model/go"
)

type MatrixOptions struct {
	ContractPath string
	FixtureDir   string
	OutputPath   string
}

type fixtureProfile struct {
	Name               string
	Class              string
	Prefix             string
	DefaultExpectation string
	Path               string
}

type metricObservation struct {
	Status string
	Type   string
	Labels []string
}

type profileCoverageCounts struct {
	Observed            int
	MissingRequired     int
	OptionalMissing     int
	Variable            int
	NotApplicable       int
	MissingUnclassified int
}

const (
	profileClassPrefixSmoke = "prefix-smoke"

	coverageUnclassified = "unclassified"

	statusObserved            = "observed"
	statusMissingRequired     = "missing-required"
	statusOptionalMissing     = "optional-missing"
	statusVariable            = "variable"
	statusNotApplicable       = "not-applicable"
	statusMissingUnclassified = "missing-unclassified"
)

func GenerateMatrix(opts MatrixOptions) error {
	opts = opts.withDefaults()

	contract, err := contracts.LoadMetricContract(opts.ContractPath)
	if err != nil {
		return err
	}

	profiles, err := fixtureProfiles(opts.FixtureDir, contract)
	if err != nil {
		return err
	}

	familiesByProfile := make(map[string]promtext.Families, len(profiles))
	for _, profile := range profiles {
		families, err := promtext.LoadFamilies(profile.Path)
		if err != nil {
			return err
		}
		familiesByProfile[profile.Name] = families
	}

	content := renderMatrix(opts, contract, profiles, familiesByProfile)
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory for %s: %w", opts.OutputPath, err)
	}
	if err := os.WriteFile(opts.OutputPath, content, 0o644); err != nil {
		return fmt.Errorf("write compatibility matrix %s: %w", opts.OutputPath, err)
	}

	fmt.Printf("generated compatibility matrix %s\n", opts.OutputPath)
	return nil
}

func (o MatrixOptions) withDefaults() MatrixOptions {
	if o.ContractPath == "" {
		o.ContractPath = filepath.Join("contracts", "metrics", "openbao-core.yaml")
	}
	if o.FixtureDir == "" {
		o.FixtureDir = filepath.Join("fixtures", "captured", "openbao-2.5.4")
	}
	if o.OutputPath == "" {
		o.OutputPath = filepath.Join("generated", "docs", "metric-compatibility-matrix.md")
	}
	return o
}

func fixtureProfiles(root string, contract *contracts.MetricContract) ([]fixtureProfile, error) {
	seen := map[string]bool{}
	profiles := []fixtureProfile{}
	coverageProfiles := coverageProfilesByID(contract)

	for _, prefix := range contract.MetricPrefixes.Supported {
		profile := fixtureProfile{
			Name:   prefix + "-prefix",
			Prefix: prefix,
			Path: filepath.Join(
				root,
				"metrics",
				fmt.Sprintf("openbao-%s-%s-prefix.prom", contract.OpenBAOVersion, prefix),
			),
		}
		applyCoverageProfile(&profile, coverageProfiles)
		if _, err := os.Stat(profile.Path); err != nil {
			return nil, fmt.Errorf("required compatibility fixture %s is not readable: %w", profile.Path, err)
		}
		profiles = append(profiles, profile)
		seen[filepath.Clean(profile.Path)] = true
	}

	paths, err := filepath.Glob(
		filepath.Join(root, "metrics", fmt.Sprintf("openbao-%s-*.prom", contract.OpenBAOVersion)),
	)
	if err != nil {
		return nil, fmt.Errorf("glob compatibility fixtures: %w", err)
	}
	slices.Sort(paths)
	for _, path := range paths {
		cleaned := filepath.Clean(path)
		if seen[cleaned] {
			continue
		}
		profile := fixtureProfile{
			Name:   fixtureProfileName(filepath.Base(path), contract.OpenBAOVersion),
			Prefix: fixturePrefix(filepath.Base(path), contract),
			Path:   path,
		}
		applyCoverageProfile(&profile, coverageProfiles)
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

func coverageProfilesByID(contract *contracts.MetricContract) map[string]contracts.CoverageProfile {
	profiles := make(map[string]contracts.CoverageProfile, len(contract.CoverageProfiles))
	for _, profile := range contract.CoverageProfiles {
		profiles[profile.ID] = profile
	}
	return profiles
}

func applyCoverageProfile(profile *fixtureProfile, coverageProfiles map[string]contracts.CoverageProfile) {
	coverageProfile, ok := coverageProfiles[profile.Name]
	if !ok {
		return
	}
	profile.Class = coverageProfile.Class
	profile.DefaultExpectation = coverageProfile.DefaultExpectation
}

func fixtureProfileName(base, version string) string {
	name := strings.TrimSuffix(base, ".prom")
	name = strings.TrimPrefix(name, "openbao-"+version+"-")
	return name
}

func fixturePrefix(base string, contract *contracts.MetricContract) string {
	for _, prefix := range contract.MetricPrefixes.Supported {
		if strings.Contains(base, "-"+prefix+"-") || strings.Contains(base, "-"+prefix+".") {
			return prefix
		}
	}
	return contract.MetricPrefixes.Default
}

func renderMatrix(
	opts MatrixOptions,
	contract *contracts.MetricContract,
	profiles []fixtureProfile,
	familiesByProfile map[string]promtext.Families,
) []byte {
	var buf bytes.Buffer

	fmt.Fprintln(&buf, "# OpenBao metric compatibility matrix")
	fmt.Fprintln(&buf)
	fmt.Fprintln(
		&buf,
		"This generated reference lists each metric contract entry against captured OpenBao metric fixtures. "+
			"Regenerate it with `make generate` after changing metric contracts or fixture captures.",
	)
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "> [!WARNING]")
	fmt.Fprintln(
		&buf,
		"> Do not edit this file by hand. "+
			"Edit the metric contract or captured fixtures and regenerate the matrix.",
	)
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "## Source")
	fmt.Fprintln(&buf)
	fmt.Fprintln(&buf, "| Field | Value |")
	fmt.Fprintln(&buf, "| ----- | ----- |")
	fmt.Fprintf(&buf, "| Contract | `%s` |\n", markdownCell(filepath.ToSlash(opts.ContractPath)))
	fmt.Fprintf(&buf, "| Fixture directory | `%s` |\n", markdownCell(filepath.ToSlash(opts.FixtureDir)))
	fmt.Fprintf(&buf, "| OpenBao version | `%s` |\n", markdownCell(contract.OpenBAOVersion))
	fmt.Fprintf(&buf, "| Maturity lifecycle | `%s` |\n", markdownCell(contract.Maturity.Lifecycle))
	fmt.Fprintf(
		&buf,
		"| Maturity evidence | `%s` |\n",
		markdownCell(strings.Join(contract.Maturity.Evidence, "`, `")),
	)
	fmt.Fprintf(
		&buf,
		"| Supported prefixes | `%s` |\n",
		markdownCell(strings.Join(contract.MetricPrefixes.Supported, "`, `")),
	)
	fmt.Fprintln(&buf)

	fmt.Fprintln(&buf, "## Fixture profiles")
	fmt.Fprintln(&buf)
	fmt.Fprintln(
		&buf,
		"| Profile | Class | Prefix | Fixture | Observed | Missing required | "+
			"Optional missing | Variable | Not applicable | Unclassified missing |",
	)
	fmt.Fprintln(
		&buf,
		"| ------- | ----- | ------ | ------- | -------- | ---------------- | "+
			"---------------- | -------- | -------------- | -------------------- |",
	)
	for _, profile := range profiles {
		counts := profileCounts(contract, profile, familiesByProfile[profile.Name])
		fmt.Fprintf(
			&buf,
			"| `%s` | `%s` | `%s` | `%s` | %d | %d | %d | %d | %d | %d |\n",
			markdownCell(profile.Name),
			markdownCell(defaultDash(profile.Class)),
			markdownCell(profile.Prefix),
			markdownCell(filepath.ToSlash(profile.Path)),
			counts.Observed,
			counts.MissingRequired,
			counts.OptionalMissing,
			counts.Variable,
			counts.NotApplicable,
			counts.MissingUnclassified,
		)
	}
	fmt.Fprintln(&buf)

	fmt.Fprintln(&buf, "## Metric coverage")
	fmt.Fprintln(&buf)
	fmt.Fprintln(
		&buf,
		"| OpenBao version | Profile | Profile class | Prefix | Metric ID | Docs metric | Source metric | "+
			"Expectation | Status | Type | Labels | Required | Overview | Notes |",
	)
	fmt.Fprintln(
		&buf,
		"| --------------- | ------- | ------------- | ------ | --------- | ----------- | ------------- | "+
			"----------- | ------ | ---- | ------ | -------- | -------- | ----- |",
	)
	for _, metric := range contract.Metrics {
		for _, profile := range profiles {
			sourceName := metric.FixtureName(profile.Prefix)
			expectation := metricExpectation(metric, profile)
			obs := observeMetric(familiesByProfile[profile.Name], sourceName, expectation)
			fmt.Fprintf(
				&buf,
				"| `%s` | `%s` | `%s` | `%s` | `%s` | `%s` | `%s` | %s | %s | %s | %s | %s | %s | %s |\n",
				markdownCell(contract.OpenBAOVersion),
				markdownCell(profile.Name),
				markdownCell(defaultDash(profile.Class)),
				markdownCell(profile.Prefix),
				markdownCell(metric.ID),
				markdownCell(metric.DocsName),
				markdownCell(sourceName),
				markdownCell(expectation),
				markdownCell(obs.Status),
				markdownCell(defaultDash(obs.Type)),
				markdownCell(labelText(obs.Labels)),
				boolText(metric.Required),
				boolText(metric.Overview),
				markdownCell(notesText(metric.Notes)),
			)
		}
	}

	return buf.Bytes()
}

func profileCounts(
	contract *contracts.MetricContract,
	profile fixtureProfile,
	families promtext.Families,
) profileCoverageCounts {
	var counts profileCoverageCounts
	for _, metric := range contract.Metrics {
		expectation := metricExpectation(metric, profile)
		obs := observeMetric(families, metric.FixtureName(profile.Prefix), expectation)
		switch obs.Status {
		case statusObserved:
			counts.Observed++
		case statusMissingRequired:
			counts.MissingRequired++
		case statusOptionalMissing:
			counts.OptionalMissing++
		case statusVariable:
			counts.Variable++
		case statusNotApplicable:
			counts.NotApplicable++
		case statusMissingUnclassified:
			counts.MissingUnclassified++
		}
	}
	return counts
}

func metricExpectation(metric contracts.Metric, profile fixtureProfile) string {
	if expectation := metric.Coverage[profile.Name]; expectation != "" {
		return expectation
	}
	if profile.Class == profileClassPrefixSmoke && metric.Required {
		return contracts.MetricCoverageRequired
	}
	if profile.DefaultExpectation != "" {
		return profile.DefaultExpectation
	}
	return coverageUnclassified
}

func observeMetric(families promtext.Families, name, expectation string) metricObservation {
	if expectation == contracts.MetricCoverageVariable {
		return metricObservation{Status: statusVariable}
	}

	family, ok := families[name]
	if !ok {
		return metricObservation{Status: missingStatus(expectation)}
	}
	return metricObservation{
		Status: statusObserved,
		Type:   metricType(family.GetType()),
		Labels: labelNames(family),
	}
}

func missingStatus(expectation string) string {
	switch expectation {
	case contracts.MetricCoverageRequired:
		return statusMissingRequired
	case contracts.MetricCoverageOptional:
		return statusOptionalMissing
	case contracts.MetricCoverageNotApplicable:
		return statusNotApplicable
	default:
		return statusMissingUnclassified
	}
}

func metricType(metricType dto.MetricType) string {
	value := strings.ToLower(metricType.String())
	if value == "" {
		return "untyped"
	}
	return value
}

func labelNames(family *dto.MetricFamily) []string {
	set := map[string]bool{}
	for _, metric := range family.GetMetric() {
		for _, label := range metric.GetLabel() {
			set[label.GetName()] = true
		}
	}
	labels := make([]string, 0, len(set))
	for label := range set {
		labels = append(labels, label)
	}
	slices.Sort(labels)
	return labels
}

func labelText(labels []string) string {
	if len(labels) == 0 {
		return "none"
	}
	quoted := make([]string, 0, len(labels))
	for _, label := range labels {
		quoted = append(quoted, "`"+label+"`")
	}
	return strings.Join(quoted, ", ")
}

func notesText(notes []string) string {
	if len(notes) == 0 {
		return "-"
	}
	return strings.Join(notes, "<br>")
}

func defaultDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func boolText(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", `\|`)
	return value
}
