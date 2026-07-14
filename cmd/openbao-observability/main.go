package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dc-tec/openbao-observability/internal/compatibility"
	"github.com/dc-tec/openbao-observability/internal/contracts"
	dashboardgen "github.com/dc-tec/openbao-observability/internal/dashboards"
	docverify "github.com/dc-tec/openbao-observability/internal/docs"
	"github.com/dc-tec/openbao-observability/internal/fixtures"
	releasepkg "github.com/dc-tec/openbao-observability/internal/release"
	"github.com/dc-tec/openbao-observability/internal/rules"
)

const (
	defaultOpenBAOVersion = "2.6.0"
	helpFlag              = "--help"
	helpCommand           = "help"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return usage()
	}

	switch args[0] {
	case "contracts":
		return runContracts(args[1:])
	case "fixtures":
		return runFixtures(ctx, args[1:])
	case "generate":
		return runGenerate(args[1:])
	case "release":
		return runRelease(args[1:])
	case "validate":
		return runValidate(ctx, args[1:])
	case "-h", helpFlag, helpCommand:
		return usage()
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usageText())
	}
}

func runContracts(args []string) error {
	if len(args) < 1 {
		return contractsUsage()
	}

	switch args[0] {
	case "verify":
		return runContractsVerify(args[1:])
	case "verify-alerts":
		return runContractsVerifyAlerts(args[1:])
	case "verify-dashboards":
		return runContractsVerifyDashboards(args[1:])
	case "verify-repository":
		return runContractsVerifyRepository(args[1:])
	case "verify-streams":
		return runContractsVerifyStreams(args[1:])
	case "-h", helpFlag, helpCommand:
		return contractsUsage()
	default:
		return fmt.Errorf("unknown contracts command %q\n\n%s", args[0], contractsUsageText())
	}
}

func runFixtures(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fixturesUsage()
	}

	switch args[0] {
	case "capture":
		return runFixturesCapture(ctx, args[1:])
	case "scenario":
		return runFixturesScenario(ctx, args[1:])
	case "verify":
		return runFixturesVerify(args[1:])
	case "-h", helpFlag, helpCommand:
		return fixturesUsage()
	default:
		return fmt.Errorf("unknown fixtures command %q\n\n%s", args[0], fixturesUsageText())
	}
}

func runGenerate(args []string) error {
	if len(args) < 1 {
		return generateUsage()
	}

	switch args[0] {
	case "alert-rules":
		return runGenerateAlertRules(args[1:])
	case "compatibility-matrix":
		return runGenerateCompatibilityMatrix(args[1:])
	case "grafana-dashboard":
		return runGenerateGrafanaDashboard(args[1:])
	case "prometheus-rules":
		return runGeneratePrometheusRules(args[1:])
	case "-h", helpFlag, helpCommand:
		return generateUsage()
	default:
		return fmt.Errorf("unknown generate command %q\n\n%s", args[0], generateUsageText())
	}
}

func runRelease(args []string) error {
	if len(args) < 1 {
		return releaseUsage()
	}

	switch args[0] {
	case "bundle":
		return runReleaseBundle(args[1:])
	case "checksums":
		return runReleaseChecksums(args[1:])
	case "-h", helpFlag, helpCommand:
		return releaseUsage()
	default:
		return fmt.Errorf("unknown release command %q\n\n%s", args[0], releaseUsageText())
	}
}

func runValidate(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return validateUsage()
	}

	switch args[0] {
	case "dashboard-queries":
		return runValidateDashboardQueries(ctx, args[1:])
	case "docs":
		return runValidateDocs(args[1:])
	case "site-links":
		return runValidateSiteLinks(args[1:])
	case "-h", helpFlag, helpCommand:
		return validateUsage()
	default:
		return fmt.Errorf("unknown validate command %q\n\n%s", args[0], validateUsageText())
	}
}

func runContractsVerify(args []string) error {
	version := envString("OPENBAO_VERSION", defaultOpenBAOVersion)
	defaultFixtureDir := filepath.Join("fixtures", "captured", "openbao-"+version)

	fs := flag.NewFlagSet("contracts verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := contracts.VerifyOptions{}
	fs.StringVar(
		&opts.ContractPath,
		"contract",
		filepath.Join("contracts", "metrics", "openbao-core.yaml"),
		"metric contract path",
	)
	fs.StringVar(&opts.FixtureDir, "fixtures", envString("FIXTURE_DIR", defaultFixtureDir), "fixture directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return contracts.VerifyMetricContract(opts)
}

func runContractsVerifyAlerts(args []string) error {
	fs := flag.NewFlagSet("contracts verify-alerts", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := contracts.VerifyAlertOptions{}
	fs.StringVar(
		&opts.ContractPath,
		"contract",
		filepath.Join("contracts", "alerts", "critical.yaml"),
		"alert contract path",
	)
	fs.StringVar(
		&opts.SourcePrefix,
		"source-prefix",
		"",
		"source metric prefix; defaults to sourcePrefix from contract",
	)
	fs.StringVar(
		&opts.RepositoryRoot,
		"repository-root",
		".",
		"repository root used to resolve local runbook paths",
	)
	fs.StringVar(&opts.ExpectedSeverity, "severity", "", "optional expected severity for every alert in the contract")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return contracts.VerifyAlertContract(opts)
}

func runContractsVerifyDashboards(args []string) error {
	fs := flag.NewFlagSet("contracts verify-dashboards", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := contracts.VerifyDashboardOptions{}
	fs.StringVar(
		&opts.ContractPath,
		"contract",
		filepath.Join("contracts", "dashboards", "openbao-overview.yaml"),
		"dashboard contract path",
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	return contracts.VerifyDashboardContract(opts)
}

func runContractsVerifyRepository(args []string) error {
	fs := flag.NewFlagSet("contracts verify-repository", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := contracts.VerifyRepositoryOptions{}
	fs.StringVar(&opts.RepositoryRoot, "repository-root", ".", "repository root")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return contracts.VerifyRepository(opts)
}

func runContractsVerifyStreams(args []string) error {
	defaultDashboardContracts := strings.Join(defaultDashboardContractPaths(), ",")

	fs := flag.NewFlagSet("contracts verify-streams", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := contracts.VerifyStreamOptions{}
	var dashboardContracts string
	fs.StringVar(
		&opts.ContractPath,
		"contract",
		filepath.Join("contracts", "streams", "log-streams.yaml"),
		"stream contract path",
	)
	fs.StringVar(
		&opts.AlertContractPath,
		"alert-contract",
		filepath.Join("contracts", "alerts", "critical.yaml"),
		"alert contract path",
	)
	fs.StringVar(
		&dashboardContracts,
		"dashboard-contracts",
		defaultDashboardContracts,
		"comma-separated dashboard contract paths",
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	opts.DashboardContractPaths = splitCSV(dashboardContracts)
	return contracts.VerifyStreamContract(opts)
}

func runFixturesCapture(ctx context.Context, args []string) error {
	version := envString("OPENBAO_VERSION", defaultOpenBAOVersion)
	defaultImage := "quay.io/openbao/openbao:" + version
	defaultOutput := filepath.Join("fixtures", "captured", "openbao-"+version)

	fs := flag.NewFlagSet("fixtures capture", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := fixtures.CaptureOptions{}
	fs.StringVar(&opts.Version, "version", version, "OpenBao version to capture")
	fs.StringVar(&opts.Image, "image", envString("OPENBAO_IMAGE", defaultImage), "OpenBao container image")
	fs.StringVar(
		&opts.PostgresImage,
		"postgres-image",
		envString("POSTGRES_IMAGE", "postgres:17-alpine"),
		"PostgreSQL container image for dynamic secret fixtures",
	)
	fs.StringVar(&opts.OutputDir, "output", envString("OUTPUT_DIR", defaultOutput), "fixture output directory")
	fs.IntVar(&opts.PortBase, "port-base", envInt("OPENBAO_PORT_BASE", 18220), "first localhost port to use")
	fs.StringVar(&opts.RootToken, "root-token", envString("OPENBAO_ROOT_TOKEN", "root"), "OpenBao dev root token")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return fixtures.Capture(ctx, opts)
}

func runFixturesScenario(ctx context.Context, args []string) error {
	version := envString("OPENBAO_VERSION", defaultOpenBAOVersion)
	defaultOutput := filepath.Join(
		"fixtures",
		"captured",
		"openbao-"+version,
		"metadata",
		"openbao-"+version+"-compose-scenario.json",
	)

	fs := flag.NewFlagSet("fixtures scenario", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := fixtures.ScenarioOptions{}
	fs.StringVar(&opts.Address, "address", envString("BAO_ADDR", "http://127.0.0.1:18200"), "OpenBao API address")
	fs.StringVar(&opts.Token, "token", envString("BAO_TOKEN", ""), "optional OpenBao token; defaults to userpass login")
	fs.StringVar(&opts.Username, "username", "demo-admin", "userpass username when --token is empty")
	fs.StringVar(&opts.Password, "password", "openbao-observability", "userpass password when --token is empty")
	fs.StringVar(
		&opts.PostgresHost,
		"postgres-host",
		envString("POSTGRES_HOST", "postgres"),
		"PostgreSQL host reachable from OpenBao for namespace database fixtures",
	)
	fs.StringVar(&opts.OutputPath, "output", defaultOutput, "scenario report output path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return fixtures.RunScenario(ctx, opts)
}

func runFixturesVerify(args []string) error {
	version := envString("OPENBAO_VERSION", defaultOpenBAOVersion)
	defaultDir := filepath.Join("fixtures", "captured", "openbao-"+version)

	fs := flag.NewFlagSet("fixtures verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := fixtures.VerifyOptions{}
	fs.StringVar(&opts.Version, "version", version, "OpenBao fixture version")
	fs.StringVar(&opts.FixtureDir, "dir", envString("FIXTURE_DIR", defaultDir), "fixture directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if opts.Version == "" {
		opts.Version = inferVersion(opts.FixtureDir)
	}

	return fixtures.Verify(opts)
}

func runGeneratePrometheusRules(args []string) error {
	fs := flag.NewFlagSet("generate prometheus-rules", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := rules.GenerateOptions{}
	fs.StringVar(
		&opts.ContractPath,
		"contract",
		filepath.Join("contracts", "metrics", "openbao-core.yaml"),
		"metric contract path",
	)
	fs.StringVar(
		&opts.OutputPath,
		"output",
		filepath.Join("generated", "prometheusrules", "openbao-recording-rules.yaml"),
		"output PrometheusRule path",
	)
	fs.StringVar(&opts.RuleFilePath, "rule-output", "", "optional native Prometheus rule file path")
	fs.StringVar(&opts.SourcePrefix, "source-prefix", "", "source metric prefix; defaults to metricPrefixes.default")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return rules.GeneratePrometheusRules(opts)
}

func runGenerateAlertRules(args []string) error {
	fs := flag.NewFlagSet("generate alert-rules", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := rules.GenerateAlertOptions{}
	fs.StringVar(
		&opts.ContractPath,
		"contract",
		filepath.Join("contracts", "alerts", "critical.yaml"),
		"alert contract path",
	)
	fs.StringVar(
		&opts.PrometheusOutputPath,
		"prometheus-output",
		filepath.Join("generated", "prometheusrules", "openbao-alerts.yaml"),
		"output PrometheusRule path",
	)
	fs.StringVar(
		&opts.PrometheusRuleFilePath,
		"prometheus-rule-output",
		"",
		"optional native Prometheus alert rule file path",
	)
	fs.StringVar(
		&opts.LokiOutputPath,
		"loki-output",
		filepath.Join("generated", "loki", "openbao-alerts.yaml"),
		"output Loki alert path",
	)
	fs.StringVar(
		&opts.SourcePrefix,
		"source-prefix",
		"",
		"source metric prefix; defaults to sourcePrefix from contract",
	)
	fs.StringVar(
		&opts.PrometheusName,
		"prometheus-name",
		"openbao-alerts",
		"metadata name for generated Prometheus alert artifacts",
	)
	fs.StringVar(&opts.LokiName, "loki-name", "openbao-loki-alerts", "metadata name for generated Loki alert artifacts")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return rules.GenerateAlertRules(opts)
}

func runGenerateCompatibilityMatrix(args []string) error {
	version := envString("OPENBAO_VERSION", defaultOpenBAOVersion)
	defaultFixtureDir := filepath.Join("fixtures", "captured", "openbao-"+version)

	fs := flag.NewFlagSet("generate compatibility-matrix", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := compatibility.MatrixOptions{}
	fs.StringVar(
		&opts.ContractPath,
		"contract",
		filepath.Join("contracts", "metrics", "openbao-core.yaml"),
		"metric contract path",
	)
	fs.StringVar(&opts.FixtureDir, "fixtures", envString("FIXTURE_DIR", defaultFixtureDir), "fixture directory")
	fs.StringVar(
		&opts.OutputPath,
		"output",
		filepath.Join("generated", "docs", "metric-compatibility-matrix.md"),
		"output compatibility matrix path",
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	return compatibility.GenerateMatrix(opts)
}

func runGenerateGrafanaDashboard(args []string) error {
	fs := flag.NewFlagSet("generate grafana-dashboard", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := dashboardgen.GenerateOptions{}
	fs.StringVar(
		&opts.ContractPath,
		"contract",
		filepath.Join("contracts", "dashboards", "openbao-overview.yaml"),
		"dashboard contract path",
	)
	fs.StringVar(
		&opts.OutputPath,
		"output",
		filepath.Join("generated", "grafana", "openbao-overview.json"),
		"output Grafana dashboard JSON path",
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	return dashboardgen.GenerateGrafanaDashboard(opts)
}

func runReleaseBundle(args []string) error {
	fs := flag.NewFlagSet("release bundle", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := releasepkg.BundleOptions{}
	var sourceDateEpoch string
	fs.StringVar(&opts.RepositoryRoot, "repository-root", ".", "repository root")
	fs.StringVar(&opts.Version, "version", envString("VERSION", "0.0.0-dev"), "release version without a leading v")
	fs.StringVar(&opts.OutputPath, "output", "", "output tar.gz path")
	fs.StringVar(
		&sourceDateEpoch,
		"source-date-epoch",
		envString("SOURCE_DATE_EPOCH", "0"),
		"deterministic source timestamp as Unix epoch seconds",
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	epoch, err := strconv.ParseInt(sourceDateEpoch, 10, 64)
	if err != nil {
		return fmt.Errorf("parse --source-date-epoch: %w", err)
	}
	opts.SourceDateEpoch = epoch
	return releasepkg.Bundle(opts)
}

func runReleaseChecksums(args []string) error {
	fs := flag.NewFlagSet("release checksums", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := releasepkg.ChecksumOptions{}
	fs.StringVar(&opts.Directory, "dir", filepath.Join("dist", "release"), "release asset directory")
	fs.StringVar(&opts.OutputPath, "output", filepath.Join("dist", "release", "checksums.txt"), "checksum output path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return releasepkg.Checksums(opts)
}

func runValidateDashboardQueries(ctx context.Context, args []string) error {
	defaultDashboardContracts := strings.Join(defaultDashboardContractPaths(), ",")
	defaultGeneratedDashboards := strings.Join([]string{
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
	}, ",")

	fs := flag.NewFlagSet("validate dashboard-queries", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := dashboardgen.QueryValidationOptions{}
	var contractsCSV string
	var generatedCSV string
	var queryRange string
	var step string
	var timeout string
	fs.StringVar(&contractsCSV, "contracts", defaultDashboardContracts, "comma-separated dashboard contract paths")
	fs.StringVar(
		&generatedCSV,
		"generated",
		defaultGeneratedDashboards,
		"comma-separated generated Grafana dashboard JSON paths",
	)
	fs.StringVar(
		&opts.PrometheusURL,
		"prometheus-url",
		envString("PROMETHEUS_URL", "http://127.0.0.1:19090"),
		"Prometheus base URL",
	)
	fs.StringVar(&opts.LokiURL, "loki-url", envString("LOKI_URL", "http://127.0.0.1:13100"), "Loki base URL")
	fs.StringVar(&queryRange, "range", "15m", "query range duration")
	fs.StringVar(&step, "step", "30s", "query step duration")
	fs.StringVar(&timeout, "timeout", "10s", "per-query timeout")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var err error
	opts.Range, err = time.ParseDuration(queryRange)
	if err != nil {
		return fmt.Errorf("parse --range: %w", err)
	}
	opts.Step, err = time.ParseDuration(step)
	if err != nil {
		return fmt.Errorf("parse --step: %w", err)
	}
	opts.Timeout, err = time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("parse --timeout: %w", err)
	}
	opts.ContractPaths = splitCSV(contractsCSV)
	opts.GeneratedPaths = splitCSV(generatedCSV)

	return dashboardgen.ValidateDashboardQueries(ctx, opts)
}

func runValidateDocs(args []string) error {
	fs := flag.NewFlagSet("validate docs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := docverify.VerifyOptions{}
	fs.StringVar(&opts.RepositoryRoot, "repository-root", ".", "repository root")
	fs.StringVar(&opts.DocsRoot, "docs-root", filepath.Join("docs"), "documentation root")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return docverify.Verify(opts)
}

func runValidateSiteLinks(args []string) error {
	fs := flag.NewFlagSet("validate site-links", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := docverify.SiteLinkOptions{}
	fs.StringVar(&opts.SiteRoot, "site-root", "public", "generated site root")
	fs.StringVar(
		&opts.BaseURL,
		"base-url",
		envString("DOCS_BASE_URL", ""),
		"site base URL used to derive the base path",
	)
	fs.StringVar(&opts.BasePath, "base-path", "", "optional site base path override")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return docverify.VerifySiteLinks(opts)
}

func inferVersion(dir string) string {
	base := filepath.Base(filepath.Clean(dir))
	if version, ok := strings.CutPrefix(base, "openbao-"); ok {
		return version
	}
	return defaultOpenBAOVersion
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func defaultDashboardContractPaths() []string {
	return []string{
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

func usage() error {
	return fmt.Errorf("%s", usageText())
}

func fixturesUsage() error {
	return fmt.Errorf("%s", fixturesUsageText())
}

func usageText() string {
	return `usage:
  openbao-observability contracts <command>
  openbao-observability fixtures <command>
  openbao-observability generate <command>
  openbao-observability release <command>
  openbao-observability validate <command>

commands:
  contracts verify     verify metric contracts against captured fixtures
  contracts verify-alerts
                      verify alert contracts
  contracts verify-dashboards
                      verify dashboard contracts
  contracts verify-repository
                      verify repository contract and generated artifact wiring
  contracts verify-streams
                      verify log stream contracts
  fixtures capture    capture OpenBao Docker fixtures
  fixtures scenario   run production-like fixture activity against OpenBao
  fixtures verify     verify captured OpenBao fixtures
  generate compatibility-matrix
                      generate metric fixture compatibility Markdown
  generate grafana-dashboard
                      generate Grafana dashboard JSON
  generate prometheus-rules
                      generate Prometheus recording rules
  release bundle     build deterministic release tarball
  release checksums  write SHA256 checksums for release assets
  validate dashboard-queries
                      validate dashboard queries against Prometheus and Loki
  validate docs       validate user-facing Markdown documentation
  validate site-links validate generated documentation site links`
}

func contractsUsage() error {
	return fmt.Errorf("%s", contractsUsageText())
}

func contractsUsageText() string {
	return `usage:
  openbao-observability contracts verify [flags]
  openbao-observability contracts verify-alerts [flags]
  openbao-observability contracts verify-dashboards [flags]
  openbao-observability contracts verify-repository [flags]
  openbao-observability contracts verify-streams [flags]`
}

func fixturesUsageText() string {
	return `usage:
  openbao-observability fixtures capture [flags]
  openbao-observability fixtures scenario [flags]
  openbao-observability fixtures verify [flags]`
}

func generateUsage() error {
	return fmt.Errorf("%s", generateUsageText())
}

func releaseUsage() error {
	return fmt.Errorf("%s", releaseUsageText())
}

func validateUsage() error {
	return fmt.Errorf("%s", validateUsageText())
}

func generateUsageText() string {
	return `usage:
  openbao-observability generate alert-rules [flags]
  openbao-observability generate compatibility-matrix [flags]
  openbao-observability generate grafana-dashboard [flags]
  openbao-observability generate prometheus-rules [flags]`
}

func releaseUsageText() string {
	return `usage:
  openbao-observability release bundle [flags]
  openbao-observability release checksums [flags]`
}

func validateUsageText() string {
	return `usage:
  openbao-observability validate dashboard-queries [flags]
  openbao-observability validate docs [flags]
  openbao-observability validate site-links [flags]`
}
