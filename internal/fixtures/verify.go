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
	if err := checkRaftMetrics(opts, "vault"); err != nil {
		return err
	}
	if err := checkRaftMetadata(opts, "vault"); err != nil {
		return err
	}
	if err := checkRaftScenarioReport(opts, "vault"); err != nil {
		return err
	}
	if err := checkRaftAuditJSON(opts, "vault"); err != nil {
		return err
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
		prefix + "_core_handle_login_request",
		prefix + "_core_check_token",
		prefix + "_core_in_flight_requests",
		prefix + "_audit_log_request",
		prefix + "_audit_log_request_failure",
		prefix + "_audit_log_response",
		prefix + "_audit_log_response_failure",
		prefix + "_audit_local_file__log_request",
		prefix + "_expire_num_leases",
		prefix + "_expire_num_irrevocable_leases",
		prefix + "_expire_register_auth",
		prefix + "_expire_revoke",
		prefix + "_token_creation",
		prefix + "_token_create",
		prefix + "_token_lookup",
		prefix + "_token_store",
		prefix + "_token_revoke_tree",
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
	tokenCreation := prefix + "_token_creation"
	for _, label := range []string{"auth_method", "creation_ttl", "mount_point", "namespace", "token_type"} {
		if !families.HasMetricWithLabelName(tokenCreation, label) {
			return fmt.Errorf("missing %s label on %s in %s", label, tokenCreation, path)
		}
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
	return checkAuditJSONFile(path)
}

func checkRaftMetrics(opts VerifyOptions, prefix string) error {
	var leaderMetrics promtext.Families
	for index := 0; index < raftNodeCount; index++ {
		nodeID := fmt.Sprintf("node%d", index)
		path := filepath.Join(opts.FixtureDir, "metrics", fmt.Sprintf("openbao-%s-raft-%s-%s.prom", opts.Version, prefix, nodeID))
		families, err := promtext.LoadFamilies(path)
		if err != nil {
			return err
		}

		for _, metric := range []string{
			prefix + "_core_active",
			prefix + "_core_unsealed",
			prefix + "_core_in_flight_requests",
			prefix + "_raft_get",
			prefix + "_runtime_num_goroutines",
		} {
			if !families.HasMetric(metric) {
				return fmt.Errorf("missing expected Raft metric in %s: %s", path, metric)
			}
		}

		if index == 0 {
			leaderMetrics = families
		}
	}

	for _, metric := range []string{
		prefix + "_autopilot_failure_tolerance",
		prefix + "_autopilot_healthy",
		prefix + "_autopilot_node_healthy",
		prefix + "_database_Initialize_error",
		prefix + "_database_NewUser_error",
		prefix + "_database_UpdateUser_error",
		prefix + "_database_DeleteUser_error",
		prefix + "_pki_issue_failure",
		prefix + "_pki_revoke_failure",
	} {
		if !leaderMetrics.HasMetric(metric) {
			return fmt.Errorf("missing expected Raft leader metric: %s", metric)
		}
	}
	if !hasAnyGaugeValue(leaderMetrics, prefix+"_autopilot_healthy", 1) {
		return fmt.Errorf("missing healthy Autopilot gauge value")
	}
	if !hasAnyGaugeValue(leaderMetrics, prefix+"_autopilot_failure_tolerance", 1) {
		return fmt.Errorf("missing Autopilot failure tolerance of one voter")
	}
	if !hasAnyGaugeValue(leaderMetrics, prefix+"_core_active", 1) {
		return fmt.Errorf("missing active leader gauge value in Raft fixture")
	}
	for index := 0; index < raftNodeCount; index++ {
		nodeID := fmt.Sprintf("node%d", index)
		if !leaderMetrics.HasMetricWithLabel(prefix+"_autopilot_node_healthy", "node_id", nodeID) {
			return fmt.Errorf("missing Autopilot healthy node series for %s", nodeID)
		}
	}

	return nil
}

func hasAnyGaugeValue(families promtext.Families, name string, value float64) bool {
	family, ok := families[name]
	if !ok {
		return false
	}
	for _, metric := range family.GetMetric() {
		if metric.GetGauge().GetValue() == value {
			return true
		}
	}
	return false
}

func checkRaftMetadata(opts VerifyOptions, prefix string) error {
	peersPath := filepath.Join(opts.FixtureDir, "metadata", fmt.Sprintf("openbao-%s-raft-%s-peers.json", opts.Version, prefix))
	content, err := os.ReadFile(peersPath)
	if err != nil {
		return fmt.Errorf("read Raft peers fixture %s: %w", peersPath, err)
	}
	servers, err := parseRaftServers(content)
	if err != nil {
		return err
	}
	if len(servers) != raftNodeCount {
		return fmt.Errorf("expected %d Raft peers in %s, found %d", raftNodeCount, peersPath, len(servers))
	}

	var leaders int
	seen := map[string]bool{}
	for _, server := range servers {
		if server.Leader {
			leaders++
		}
		if !server.Voter {
			return fmt.Errorf("Raft peer %s is not a voter in %s", server.NodeID, peersPath)
		}
		seen[server.NodeID] = true
	}
	if leaders != 1 {
		return fmt.Errorf("expected exactly one Raft leader in %s, found %d", peersPath, leaders)
	}
	for index := 0; index < raftNodeCount; index++ {
		nodeID := fmt.Sprintf("node%d", index)
		if !seen[nodeID] {
			return fmt.Errorf("missing Raft peer %s in %s", nodeID, peersPath)
		}
	}

	autopilotPath := filepath.Join(opts.FixtureDir, "metadata", fmt.Sprintf("openbao-%s-raft-%s-autopilot-state.json", opts.Version, prefix))
	content, err = os.ReadFile(autopilotPath)
	if err != nil {
		return fmt.Errorf("read Autopilot fixture %s: %w", autopilotPath, err)
	}
	state, err := parseAutopilotState(content)
	if err != nil {
		return err
	}
	if !state.Healthy {
		return fmt.Errorf("Autopilot reports unhealthy state in %s", autopilotPath)
	}
	if state.FailureTolerance < 1 {
		return fmt.Errorf("expected Autopilot failure tolerance >= 1 in %s, found %d", autopilotPath, state.FailureTolerance)
	}
	if len(state.Servers) != raftNodeCount {
		return fmt.Errorf("expected %d Autopilot servers in %s, found %d", raftNodeCount, autopilotPath, len(state.Servers))
	}
	for id, server := range state.Servers {
		if !server.Healthy {
			return fmt.Errorf("Autopilot server %s is unhealthy in %s", id, autopilotPath)
		}
		if server.NodeType != "voter" {
			return fmt.Errorf("Autopilot server %s has node type %q in %s", id, server.NodeType, autopilotPath)
		}
	}

	return nil
}

func checkRaftAuditJSON(opts VerifyOptions, prefix string) error {
	path := filepath.Join(opts.FixtureDir, "logs", "audit", fmt.Sprintf("openbao-%s-raft-%s-node0.jsonl", opts.Version, prefix))
	if err := checkAuditJSONFile(path); err != nil {
		return err
	}
	return checkAuditRequestPaths(path, []string{
		"auth/approle/login",
		"auth/token/create",
		"auth/token/lookup",
		"auth/token/renew",
		"auth/token/revoke",
		"auth/userpass/login/demo-admin",
		"auth/userpass/login/demo-reader",
		"database/config/postgres-bad",
		"database/creds/failure-create",
		"database/creds/failure-renew",
		"database/creds/failure-revoke",
		"database/creds/readonly",
		"database/roles/failure-create",
		"database/roles/failure-renew",
		"database/roles/failure-revoke",
		"identity/entity/name/scenario-service",
		"identity/group/name/scenario-services",
		"kv-v1/apps/payments/",
		"kv-v1/apps/payments/config",
		"pki/cert/ca",
		"pki/issue/observability-dot-local",
		"pki/root/generate/internal",
		"pki/revoke",
		"pki/roles/observability-dot-local",
		"secret/data/apps/payments/denied",
		"secret/data/apps/payments/scenario",
		"sys/mounts",
		"sys/mounts/kv-v1",
		"sys/mounts/pki",
		"sys/mounts/transit",
		"sys/leases/lookup",
		"sys/leases/renew",
		"sys/leases/revoke",
		"sys/policies/acl/scenario-app",
		"transit/decrypt/payments",
		"transit/encrypt/payments",
		"transit/keys/payments",
	})
}

func checkRaftScenarioReport(opts VerifyOptions, prefix string) error {
	path := filepath.Join(opts.FixtureDir, "metadata", fmt.Sprintf("openbao-%s-raft-%s-scenario.json", opts.Version, prefix))
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read scenario fixture %s: %w", path, err)
	}

	var report ScenarioReport
	if err := json.Unmarshal(content, &report); err != nil {
		return fmt.Errorf("parse scenario fixture %s: %w", path, err)
	}

	expected := map[string]string{
		"create-token":                                "success",
		"delete-kv-v1":                                "success",
		"denied-reader-kv-write":                      "expected_error",
		"decrypt-transit":                             "success",
		"encrypt-transit":                             "success",
		"ensure-kv-v1-mount":                          "success",
		"ensure-pki-mount":                            "success",
		"ensure-pki-root":                             "success",
		"ensure-transit-key":                          "success",
		"ensure-transit-mount":                        "success",
		"failed-userpass-login":                       "expected_error",
		"failed-database-delete-user":                 "expected_error",
		"failed-database-initialize":                  "expected_error",
		"failed-database-new-user":                    "expected_error",
		"failed-database-update-user":                 "expected_error",
		"failed-pki-issue-invalid-domain":             "expected_error",
		"failed-pki-revoke-invalid-serial":            "expected_error",
		"issue-pki-certificate":                       "success",
		"list-kv-v1":                                  "success",
		"list-kv-metadata":                            "success",
		"login-approle":                               "success",
		"login-userpass-admin":                        "success",
		"lookup-database-lease":                       "success",
		"lookup-token":                                "success",
		"read-database-credentials":                   "success",
		"read-kv":                                     "success",
		"read-kv-v1":                                  "success",
		"read-transit-key":                            "success",
		"renew-database-lease":                        "success",
		"renew-token":                                 "success",
		"revoke-pki-certificate":                      "success",
		"revoke-database-lease":                       "success",
		"revoke-database-failure-renew-lease":         "success",
		"revoke-database-failure-revoke-lease":        "success",
		"revoke-token":                                "success",
		"read-database-failure-renew-credentials":     "success",
		"read-database-failure-revoke-credentials":    "success",
		"write-identity-entity":                       "success",
		"write-identity-group":                        "success",
		"write-kv":                                    "success",
		"write-kv-v1":                                 "success",
		"write-database-failure-create-role":          "success",
		"write-database-failure-renew-role":           "success",
		"write-database-failure-renew-invalid-role":   "success",
		"write-database-failure-renew-recovery-role":  "success",
		"write-database-failure-revoke-role":          "success",
		"write-database-failure-revoke-invalid-role":  "success",
		"write-database-failure-revoke-recovery-role": "success",
		"write-pki-role":                              "success",
	}
	seen := map[string]string{}
	for _, step := range report.Steps {
		seen[step.Name] = step.Status
	}
	for name, status := range expected {
		got, ok := seen[name]
		if !ok {
			return fmt.Errorf("scenario fixture %s is missing step %s", path, name)
		}
		if got != status {
			return fmt.Errorf("scenario fixture %s step %s status %q, want %q", path, name, got, status)
		}
	}
	return nil
}

func checkAuditJSONFile(path string) error {
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

func checkAuditRequestPaths(path string, required []string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open audit fixture %s: %w", path, err)
	}
	defer file.Close()

	seen := map[string]bool{}
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

		request, ok := entry["request"].(map[string]any)
		if !ok {
			continue
		}
		if value, ok := request["path"].(string); ok && value != "" {
			seen[value] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan audit fixture %s: %w", path, err)
	}

	for _, requestPath := range required {
		if !seen[requestPath] {
			return fmt.Errorf("audit fixture %s is missing request.path %s", path, requestPath)
		}
	}
	return nil
}
