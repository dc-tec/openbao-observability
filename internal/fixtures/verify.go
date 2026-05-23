package fixtures

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dc-tec/openbao-observability/internal/promtext"
)

type VerifyOptions struct {
	Version    string
	FixtureDir string
}

func Verify(opts VerifyOptions) error {
	opts = opts.withDefaults()

	for _, prefix := range []string{"vault", "openbao"} {
		if err := checkMetrics(opts, prefix); err != nil {
			return err
		}
		if err := checkAPIAuditRejection(opts, prefix); err != nil {
			return err
		}
		if err := checkAuditJSON(opts, prefix); err != nil {
			return err
		}
	}

	fmt.Printf("fixture checks passed for %s\n", opts.FixtureDir)
	return nil
}

func (o VerifyOptions) withDefaults() VerifyOptions {
	if o.Version == "" {
		o.Version = "2.5.4"
	}
	if o.FixtureDir == "" {
		o.FixtureDir = filepath.Join("fixtures", "captured", "openbao-"+o.Version)
	}
	return o
}

func checkMetrics(opts VerifyOptions, prefix string) error {
	path := filepath.Join(opts.FixtureDir, "metrics", fmt.Sprintf("openbao-%s-%s-prefix.prom", opts.Version, prefix))
	families, err := promtext.LoadFamilies(path)
	if err != nil {
		return err
	}

	metrics := []string{
		prefix + "_core_active",
		prefix + "_core_unsealed",
		prefix + "_core_handle_request",
		prefix + "_audit_log_request_failure",
		prefix + "_audit_log_response_failure",
		prefix + "_audit_local_file__log_request",
		prefix + "_expire_num_leases",
		prefix + "_runtime_num_goroutines",
	}

	for _, metric := range metrics {
		if !families.HasMetric(metric) {
			return fmt.Errorf("missing expected metric in %s: %s", path, metric)
		}
	}

	coreUnsealed := prefix + "_core_unsealed"
	if !families.HasMetricWithLabel(coreUnsealed, "cluster", "") {
		return fmt.Errorf("missing empty cluster label series in %s: %s", path, coreUnsealed)
	}
	if !hasClusterWithPrefix(families, coreUnsealed, "vault-cluster-") {
		return fmt.Errorf("missing real cluster label series in %s: %s", path, coreUnsealed)
	}

	return nil
}

func hasClusterWithPrefix(families promtext.Families, name, prefix string) bool {
	for _, metric := range families[name].GetMetric() {
		for _, label := range metric.GetLabel() {
			if label.GetName() == "cluster" && strings.HasPrefix(label.GetValue(), prefix) {
				return true
			}
		}
	}
	return false
}

func checkAPIAuditRejection(opts VerifyOptions, prefix string) error {
	path := filepath.Join(opts.FixtureDir, "metadata", fmt.Sprintf("openbao-%s-%s-api-audit-enable.txt", opts.Version, prefix))
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read API audit rejection fixture %s: %w", path, err)
	}

	text := string(content)
	if !strings.Contains(text, "cannot enable audit device via API") {
		return fmt.Errorf("missing API audit rejection message in %s", path)
	}

	if ok, err := regexp.MatchString(`(?m)^exit_code=[1-9][0-9]*$`, text); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("missing non-zero API audit rejection exit code in %s", path)
	}

	return nil
}

func checkAuditJSON(opts VerifyOptions, prefix string) error {
	path := filepath.Join(opts.FixtureDir, "logs", "audit", fmt.Sprintf("openbao-%s-%s-prefix.jsonl", opts.Version, prefix))
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open audit fixture %s: %w", path, err)
	}
	defer file.Close()

	var count int
	var seenNestedRequestID bool
	var seenTopLevelRequestID bool
	var seenRequestPath bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("parse audit JSON in %s: %w", path, err)
		}

		count++
		if _, ok := entry["request_id"]; ok {
			seenTopLevelRequestID = true
		}

		request, ok := entry["request"].(map[string]any)
		if !ok {
			continue
		}
		if value, ok := request["id"].(string); ok && value != "" {
			seenNestedRequestID = true
		}
		if value, ok := request["path"].(string); ok && value != "" {
			seenRequestPath = true
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan audit fixture %s: %w", path, err)
	}

	if count == 0 {
		return fmt.Errorf("audit fixture is empty: %s", path)
	}
	if seenTopLevelRequestID {
		return fmt.Errorf("audit fixture contains top-level request_id: %s", path)
	}
	if !seenNestedRequestID {
		return fmt.Errorf("audit fixture does not contain request.id: %s", path)
	}
	if !seenRequestPath {
		return fmt.Errorf("audit fixture does not contain request.path: %s", path)
	}

	return nil
}
