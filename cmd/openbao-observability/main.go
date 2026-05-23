package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dc-tec/openbao-observability/internal/contracts"
	dashboardgen "github.com/dc-tec/openbao-observability/internal/dashboards"
	"github.com/dc-tec/openbao-observability/internal/fixtures"
	"github.com/dc-tec/openbao-observability/internal/rules"
)

const defaultOpenBAOVersion = "2.5.4"

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
	case "-h", "--help", "help":
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
	case "-h", "--help", "help":
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
	case "verify":
		return runFixturesVerify(args[1:])
	case "-h", "--help", "help":
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
	case "grafana-dashboard":
		return runGenerateGrafanaDashboard(args[1:])
	case "prometheus-rules":
		return runGeneratePrometheusRules(args[1:])
	case "-h", "--help", "help":
		return generateUsage()
	default:
		return fmt.Errorf("unknown generate command %q\n\n%s", args[0], generateUsageText())
	}
}

func runContractsVerify(args []string) error {
	version := envString("OPENBAO_VERSION", defaultOpenBAOVersion)
	defaultFixtureDir := filepath.Join("fixtures", "captured", "openbao-"+version)

	fs := flag.NewFlagSet("contracts verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := contracts.VerifyOptions{}
	fs.StringVar(&opts.ContractPath, "contract", filepath.Join("contracts", "metrics", "openbao-core.yaml"), "metric contract path")
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
	fs.StringVar(&opts.ContractPath, "contract", filepath.Join("contracts", "alerts", "critical.yaml"), "alert contract path")
	fs.StringVar(&opts.SourcePrefix, "source-prefix", "", "source metric prefix; defaults to sourcePrefix from contract")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return contracts.VerifyAlertContract(opts)
}

func runContractsVerifyDashboards(args []string) error {
	fs := flag.NewFlagSet("contracts verify-dashboards", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := contracts.VerifyDashboardOptions{}
	fs.StringVar(&opts.ContractPath, "contract", filepath.Join("contracts", "dashboards", "openbao-overview.yaml"), "dashboard contract path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return contracts.VerifyDashboardContract(opts)
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
	fs.StringVar(&opts.OutputDir, "output", envString("OUTPUT_DIR", defaultOutput), "fixture output directory")
	fs.IntVar(&opts.PortBase, "port-base", envInt("OPENBAO_PORT_BASE", 18220), "first localhost port to use")
	fs.StringVar(&opts.RootToken, "root-token", envString("OPENBAO_ROOT_TOKEN", "root"), "OpenBao dev root token")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return fixtures.Capture(ctx, opts)
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
	fs.StringVar(&opts.ContractPath, "contract", filepath.Join("contracts", "metrics", "openbao-core.yaml"), "metric contract path")
	fs.StringVar(&opts.OutputPath, "output", filepath.Join("generated", "prometheusrules", "openbao-recording-rules.yaml"), "output PrometheusRule path")
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
	fs.StringVar(&opts.ContractPath, "contract", filepath.Join("contracts", "alerts", "critical.yaml"), "alert contract path")
	fs.StringVar(&opts.PrometheusOutputPath, "prometheus-output", filepath.Join("generated", "prometheusrules", "openbao-alerts.yaml"), "output PrometheusRule path")
	fs.StringVar(&opts.PrometheusRuleFilePath, "prometheus-rule-output", "", "optional native Prometheus alert rule file path")
	fs.StringVar(&opts.LokiOutputPath, "loki-output", filepath.Join("generated", "loki", "openbao-alerts.yaml"), "output Loki alert path")
	fs.StringVar(&opts.SourcePrefix, "source-prefix", "", "source metric prefix; defaults to sourcePrefix from contract")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return rules.GenerateAlertRules(opts)
}

func runGenerateGrafanaDashboard(args []string) error {
	fs := flag.NewFlagSet("generate grafana-dashboard", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	opts := dashboardgen.GenerateOptions{}
	fs.StringVar(&opts.ContractPath, "contract", filepath.Join("contracts", "dashboards", "openbao-overview.yaml"), "dashboard contract path")
	fs.StringVar(&opts.OutputPath, "output", filepath.Join("generated", "grafana", "openbao-overview.json"), "output Grafana dashboard JSON path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return dashboardgen.GenerateGrafanaDashboard(opts)
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

commands:
  contracts verify     verify contracts against captured fixtures
  fixtures capture    capture OpenBao Docker fixtures
  fixtures verify     verify captured OpenBao fixtures
  generate grafana-dashboard
                      generate Grafana dashboard JSON
  generate prometheus-rules
                      generate Prometheus recording rules`
}

func contractsUsage() error {
	return fmt.Errorf("%s", contractsUsageText())
}

func contractsUsageText() string {
	return `usage:
  openbao-observability contracts verify [flags]
  openbao-observability contracts verify-alerts [flags]
  openbao-observability contracts verify-dashboards [flags]`
}

func fixturesUsageText() string {
	return `usage:
  openbao-observability fixtures capture [flags]
  openbao-observability fixtures verify [flags]`
}

func generateUsage() error {
	return fmt.Errorf("%s", generateUsageText())
}

func generateUsageText() string {
	return `usage:
  openbao-observability generate alert-rules [flags]
  openbao-observability generate grafana-dashboard [flags]
  openbao-observability generate prometheus-rules [flags]`
}
