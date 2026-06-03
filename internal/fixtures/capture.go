package fixtures

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CaptureOptions struct {
	Version       string
	Image         string
	PostgresImage string
	OutputDir     string
	PortBase      int
	RootToken     string
}

type captureRun struct {
	options    CaptureOptions
	containers []string
	networks   []string
	tempDirs   []string
}

type raftNode struct {
	Index     int
	ID        string
	NonVoter  bool
	Container string
	Port      int
	Config    string
}

const (
	raftVoterCount          = 3
	raftReadReplicaCount    = 1
	raftNodeCount           = raftVoterCount + raftReadReplicaCount
	fixtureAdminToken       = "openbao-observability-fixture-token"
	fixturePostgresDB       = "openbao_app"
	fixturePostgresUser     = "openbao_admin"
	fixturePostgresPassword = "openbao_admin_password"
	fixturePromRetention    = "5m"
	fixtureUsageGaugePeriod = "10s"
	usageGaugeCaptureDelay  = 25 * time.Second
)

func Capture(ctx context.Context, opts CaptureOptions) error {
	opts = opts.withDefaults()

	if err := requireCommand("docker"); err != nil {
		return err
	}

	if err := makeFixtureDirs(opts.OutputDir); err != nil {
		return err
	}

	run := &captureRun{options: opts}
	defer run.cleanup(context.Background())

	prefixes := []string{"vault", "openbao"}
	for index, prefix := range prefixes {
		if err := run.capturePrefix(ctx, prefix, opts.PortBase+index); err != nil {
			return err
		}
	}
	if err := run.captureRaft(ctx, "vault", opts.PortBase+20); err != nil {
		return err
	}

	fmt.Printf("captured OpenBao fixtures in %s\n", opts.OutputDir)
	return nil
}

func (o CaptureOptions) withDefaults() CaptureOptions {
	if o.Version == "" {
		o.Version = "2.5.4"
	}
	if o.Image == "" {
		o.Image = "quay.io/openbao/openbao:" + o.Version
	}
	if o.PostgresImage == "" {
		o.PostgresImage = "postgres:17-alpine"
	}
	if o.OutputDir == "" {
		o.OutputDir = filepath.Join("fixtures", "captured", "openbao-"+o.Version)
	}
	if o.PortBase == 0 {
		o.PortBase = 18220
	}
	if o.RootToken == "" {
		o.RootToken = "root"
	}
	return o
}

func makeFixtureDirs(root string) error {
	for _, dir := range []string{
		filepath.Join(root, "logs", "audit"),
		filepath.Join(root, "metadata"),
		filepath.Join(root, "metrics"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create fixture directory %s: %w", dir, err)
		}
	}
	return nil
}

func (r *captureRun) capturePrefix(ctx context.Context, prefix string, port int) error {
	opts := r.options
	name := fmt.Sprintf("openbao-observability-%s-%s", strings.ReplaceAll(opts.Version, ".", "-"), prefix)

	tempDir, err := os.MkdirTemp("/tmp", "openbao-observability-"+prefix+".")
	if err != nil {
		return fmt.Errorf("create temp config directory: %w", err)
	}
	r.tempDirs = append(r.tempDirs, tempDir)

	configPath := filepath.Join(tempDir, "config.hcl")
	if err := writeConfig(prefix, configPath); err != nil {
		return err
	}

	_, _, _ = dockerCombined(ctx, "rm", "-f", name)
	r.containers = append(r.containers, name)

	_, _, err = dockerCombined(ctx, "run",
		"--detach",
		"--name", name,
		"--publish", fmt.Sprintf("127.0.0.1:%d:8200", port),
		"--volume", configPath+":/bao/config/config.hcl:ro",
		opts.Image,
		"server",
		"-dev",
		"-dev-root-token-id="+opts.RootToken,
		"-dev-listen-address=0.0.0.0:8200",
		"-config=/bao/config/config.hcl",
	)
	if err != nil {
		return fmt.Errorf("start OpenBao container for prefix %s: %w", prefix, err)
	}

	healthPath := metadataPath(opts, prefix, "health.json")
	if err := waitForHealth(ctx, port, healthPath, name); err != nil {
		return err
	}

	if err := r.captureVersion(ctx, name, prefix); err != nil {
		return err
	}
	if err := r.captureAPIAuditRejection(ctx, name, prefix); err != nil {
		return err
	}
	if err := r.exerciseOpenBao(ctx, name, prefix); err != nil {
		return err
	}
	if err := waitForUsageGaugeCollection(ctx); err != nil {
		return err
	}
	if err := r.captureMetrics(ctx, prefix, port); err != nil {
		return err
	}
	if err := r.captureAuditLog(ctx, name, prefix); err != nil {
		return err
	}

	return nil
}

func (r *captureRun) captureRaft(ctx context.Context, prefix string, portBase int) error {
	opts := r.options
	network, postgresContainer := raftFixtureNames(opts.Version)
	if err := r.prepareRaftNetwork(ctx, network); err != nil {
		return err
	}
	if err := r.startRaftPostgres(ctx, postgresContainer, network); err != nil {
		return err
	}
	tempDir, sealDir, err := r.prepareRaftTempDir()
	if err != nil {
		return err
	}
	voters, readReplicas, err := createRaftNodes(opts.Version, prefix, portBase, tempDir, postgresContainer)
	if err != nil {
		return err
	}
	nodes := appendRaftNodes(voters, readReplicas)
	r.registerRaftContainers(ctx, nodes)

	if err := r.startRaftLeader(ctx, opts, prefix, voters, network, sealDir); err != nil {
		return err
	}
	if err := r.startRaftFollowers(ctx, opts, prefix, voters, network, sealDir); err != nil {
		return err
	}
	if err := r.startRaftReadReplicas(ctx, opts, prefix, voters, readReplicas, network, sealDir); err != nil {
		return err
	}
	if err := r.runRaftWorkload(ctx, voters[0], readReplicas, prefix, postgresContainer); err != nil {
		return err
	}
	return r.captureRaftOutputs(ctx, nodes, prefix)
}

func raftFixtureNames(version string) (string, string) {
	versionID := strings.ReplaceAll(version, ".", "-")
	return fmt.Sprintf("openbao-observability-%s-raft", versionID),
		fmt.Sprintf("openbao-observability-%s-postgres", versionID)
}

func (r *captureRun) prepareRaftNetwork(ctx context.Context, network string) error {
	_, _, _ = dockerCombined(ctx, "network", "rm", network)
	if _, _, err := dockerCombined(ctx, "network", "create", network); err != nil {
		return fmt.Errorf("create Docker network for Raft fixture: %w", err)
	}
	r.networks = append(r.networks, network)
	return nil
}

func (r *captureRun) startRaftPostgres(ctx context.Context, postgresContainer, network string) error {
	_, _, _ = dockerCombined(ctx, "rm", "-f", postgresContainer)
	r.containers = append(r.containers, postgresContainer)
	if err := r.startPostgres(ctx, postgresContainer, network); err != nil {
		return err
	}
	return waitForPostgres(ctx, postgresContainer)
}

func (r *captureRun) prepareRaftTempDir() (string, string, error) {
	tempDir, err := os.MkdirTemp("/tmp", "openbao-observability-raft.")
	if err != nil {
		return "", "", fmt.Errorf("create temp Raft fixture directory: %w", err)
	}
	r.tempDirs = append(r.tempDirs, tempDir)

	sealDir := filepath.Join(tempDir, "seal")
	if err := os.MkdirAll(sealDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create static seal directory: %w", err)
	}
	if err := writeStaticSealKey(filepath.Join(sealDir, "static-unseal.key")); err != nil {
		return "", "", err
	}
	return tempDir, sealDir, nil
}

func createRaftNodes(
	version, prefix string,
	portBase int,
	tempDir, postgresContainer string,
) ([]raftNode, []raftNode, error) {
	versionID := strings.ReplaceAll(version, ".", "-")
	voters, err := createRaftVoterNodes(versionID, prefix, portBase, tempDir, postgresContainer)
	if err != nil {
		return nil, nil, err
	}
	readReplicas, err := createRaftReadReplicaNodes(versionID, prefix, portBase, tempDir, postgresContainer, voters)
	if err != nil {
		return nil, nil, err
	}
	return voters, readReplicas, nil
}

func createRaftVoterNodes(
	versionID, prefix string,
	portBase int,
	tempDir, postgresContainer string,
) ([]raftNode, error) {
	voters := make([]raftNode, 0, raftVoterCount)
	for index := 0; index < raftVoterCount; index++ {
		node := raftNode{
			Index:     index,
			ID:        fmt.Sprintf("node%d", index),
			Container: fmt.Sprintf("openbao-observability-%s-raft-node%d", versionID, index),
			Port:      portBase + index,
			Config:    filepath.Join(tempDir, fmt.Sprintf("node%d.hcl", index)),
		}
		if err := writeRaftConfig(prefix, node, index == 0, voters, postgresContainer, node.Config); err != nil {
			return nil, err
		}
		voters = append(voters, node)
	}
	return voters, nil
}

func createRaftReadReplicaNodes(
	versionID, prefix string,
	portBase int,
	tempDir, postgresContainer string,
	voters []raftNode,
) ([]raftNode, error) {
	readReplicas := make([]raftNode, 0, raftReadReplicaCount)
	for index := 0; index < raftReadReplicaCount; index++ {
		nodeIndex := raftVoterCount + index
		node := raftNode{
			Index:     nodeIndex,
			ID:        fmt.Sprintf("read-replica%d", index),
			NonVoter:  true,
			Container: fmt.Sprintf("openbao-observability-%s-raft-read-replica%d", versionID, index),
			Port:      portBase + nodeIndex,
			Config:    filepath.Join(tempDir, fmt.Sprintf("read-replica%d.hcl", index)),
		}
		if err := writeRaftConfig(prefix, node, false, voters, postgresContainer, node.Config); err != nil {
			return nil, err
		}
		readReplicas = append(readReplicas, node)
	}
	return readReplicas, nil
}

func appendRaftNodes(voters, readReplicas []raftNode) []raftNode {
	return append(append([]raftNode{}, voters...), readReplicas...)
}

func (r *captureRun) registerRaftContainers(ctx context.Context, nodes []raftNode) {
	for _, node := range nodes {
		_, _, _ = dockerCombined(ctx, "rm", "-f", node.Container)
		r.containers = append(r.containers, node.Container)
	}
}

func (r *captureRun) startRaftLeader(
	ctx context.Context,
	opts CaptureOptions,
	prefix string,
	voters []raftNode,
	network, sealDir string,
) error {
	if err := r.startRaftNode(ctx, voters[0], network, sealDir); err != nil {
		return err
	}
	healthPath := raftMetadataPath(opts, prefix, voters[0].ID, "health.json")
	if err := waitForInitializedUnsealed(ctx, voters[0].Port, healthPath); err != nil {
		return err
	}
	return r.captureVersion(ctx, voters[0].Container, raftPrefix(prefix))
}

func (r *captureRun) startRaftFollowers(
	ctx context.Context,
	opts CaptureOptions,
	prefix string,
	voters []raftNode,
	network, sealDir string,
) error {
	for _, node := range voters[1:] {
		if err := r.startRaftNode(ctx, node, network, sealDir); err != nil {
			return err
		}
	}
	for _, node := range voters[1:] {
		healthPath := raftMetadataPath(opts, prefix, node.ID, "health.json")
		if err := waitForInitializedUnsealed(ctx, node.Port, healthPath); err != nil {
			return err
		}
	}
	if err := r.waitForRaftTopology(ctx, voters[0], raftVoterCount, raftVoterCount); err != nil {
		return err
	}
	if err := r.waitForAutopilotTolerance(ctx, voters[0], raftVoterCount, 1); err != nil {
		return err
	}
	return nil
}

func (r *captureRun) startRaftReadReplicas(
	ctx context.Context,
	opts CaptureOptions,
	prefix string,
	voters, readReplicas []raftNode,
	network, sealDir string,
) error {
	for _, node := range readReplicas {
		if err := r.startRaftNode(ctx, node, network, sealDir); err != nil {
			return err
		}
	}
	for _, node := range readReplicas {
		healthPath := raftMetadataPath(opts, prefix, node.ID, "health.json")
		if err := waitForInitializedUnsealed(ctx, node.Port, healthPath); err != nil {
			return err
		}
	}
	if err := r.waitForRaftTopology(ctx, voters[0], raftNodeCount, raftVoterCount); err != nil {
		return err
	}
	if err := r.waitForAutopilotTolerance(ctx, voters[0], raftNodeCount, 1); err != nil {
		return err
	}
	return nil
}

func (r *captureRun) runRaftWorkload(
	ctx context.Context,
	leader raftNode,
	readReplicas []raftNode,
	prefix, postgresContainer string,
) error {
	if err := r.exerciseRaft(ctx, leader, prefix); err != nil {
		return err
	}
	for _, node := range readReplicas {
		if err := r.exerciseRaftReadReplica(ctx, node, prefix); err != nil {
			return err
		}
	}
	if err := r.runRaftScenario(ctx, leader, prefix, postgresContainer); err != nil {
		return err
	}
	return nil
}

func (r *captureRun) captureRaftOutputs(ctx context.Context, nodes []raftNode, prefix string) error {
	if err := r.captureRaftState(ctx, nodes, prefix); err != nil {
		return err
	}
	if err := waitForUsageGaugeCollection(ctx); err != nil {
		return err
	}
	if err := r.captureRaftMetrics(ctx, nodes, prefix); err != nil {
		return err
	}
	for _, node := range nodes {
		if err := r.captureRaftAuditLog(ctx, node, prefix); err != nil {
			return err
		}
	}

	return nil
}

func writeStaticSealKey(path string) error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generate static seal key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o644); err != nil {
		return fmt.Errorf("write static seal key %s: %w", path, err)
	}
	return nil
}

func (r *captureRun) startRaftNode(ctx context.Context, node raftNode, network, sealDir string) error {
	_, _, err := dockerCombined(ctx, "run",
		"--detach",
		"--name", node.Container,
		"--network", network,
		"--publish", fmt.Sprintf("127.0.0.1:%d:8200", node.Port),
		"--mount", "type=tmpfs,destination=/bao/data",
		"--volume", node.Config+":/bao/config/config.hcl:ro",
		"--volume", sealDir+":/bao/seal:ro",
		r.options.Image,
		"server",
		"-config=/bao/config/config.hcl",
	)
	if err != nil {
		return fmt.Errorf("start OpenBao Raft container %s: %w", node.ID, err)
	}
	return nil
}

func (r *captureRun) startPostgres(ctx context.Context, name, network string) error {
	_, _, err := dockerCombined(ctx, "run",
		"--detach",
		"--name", name,
		"--network", network,
		"-e", "POSTGRES_DB="+fixturePostgresDB,
		"-e", "POSTGRES_USER="+fixturePostgresUser,
		"-e", "POSTGRES_PASSWORD="+fixturePostgresPassword,
		r.options.PostgresImage,
	)
	if err != nil {
		return fmt.Errorf("start PostgreSQL container for Raft fixture: %w", err)
	}
	return nil
}

func waitForPostgres(ctx context.Context, container string) error {
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		_, _, err := dockerCombined(
			ctx,
			"exec",
			container,
			"pg_isready",
			"-U",
			fixturePostgresUser,
			"-d",
			fixturePostgresDB,
		)
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return fmt.Errorf("PostgreSQL fixture did not become ready: %w", lastErr)
}

func waitForUsageGaugeCollection(ctx context.Context) error {
	timer := time.NewTimer(usageGaugeCaptureDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func writeRaftConfig(
	prefix string,
	node raftNode,
	initialize bool,
	existingNodes []raftNode,
	databaseHost, path string,
) error {
	var retryJoin strings.Builder
	if !initialize {
		for _, leader := range existingNodes {
			fmt.Fprintf(&retryJoin, `

  retry_join {
    leader_api_addr = "http://%s:8200"
  }`, leader.Container)
		}
	}
	nonVoterConfig := ""
	if !initialize && node.NonVoter {
		nonVoterConfig = "\n  retry_join_as_non_voter = true"
	}

	config := fmt.Sprintf(`ui = true
cluster_name = "openbao-observability-fixture"
api_addr     = "http://%s:8200"
cluster_addr = "http://%s:8201"

storage "raft" {
  path                   = "/bao/data"
  node_id                = "%s"
  performance_multiplier = 1%s%s
}

listener "tcp" {
  address         = "0.0.0.0:8200"
  cluster_address = "0.0.0.0:8201"
  tls_disable     = true

  telemetry {
    unauthenticated_metrics_access = true
  }
}

seal "static" {
  current_key_id = "fixture-1"
  current_key    = "file:///bao/seal/static-unseal.key"
}

telemetry {
  prometheus_retention_time = "%s"
  disable_hostname          = true
  metrics_prefix            = "%s"
  usage_gauge_period        = "%s"
}

audit "file" "local-file" {
  description = "Validation audit file."

  options {
    file_path     = "/tmp/openbao-audit.jsonl"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}
%s`,
		node.Container,
		node.Container,
		node.ID,
		nonVoterConfig,
		retryJoin.String(),
		fixturePromRetention,
		prefix,
		fixtureUsageGaugePeriod,
		raftInitializeBlock(initialize, databaseHost),
	)

	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		return fmt.Errorf("write OpenBao Raft config %s: %w", path, err)
	}
	return nil
}

func raftInitializeBlock(enabled bool, databaseHost string) string {
	if !enabled {
		return ""
	}

	return fmt.Sprintf(`

initialize "fixture-foundation" {
  request "enable-userpass-auth" {
    operation = "update"
    path      = "sys/auth/userpass"

    data = {
      type = "userpass"
    }
  }

  request "enable-approle-auth" {
    operation = "update"
    path      = "sys/auth/approle"

    data = {
      type        = "approle"
      description = "Fixture AppRole auth method for observability reference captures."
    }
  }

  request "enable-secret-kv-v2" {
    operation = "update"
    path      = "sys/mounts/secret"

    data = {
      type        = "kv"
      description = "Fixture KV engine for observability reference captures."

      options = {
        version = "2"
      }
    }
  }

  request "enable-kv-v1-secrets" {
    operation = "update"
    path      = "sys/mounts/kv-v1"

    data = {
      type        = "kv"
      description = "Fixture KV v1 engine for observability reference captures."

      options = {
        version = "1"
      }
    }
  }

  request "enable-database-secrets" {
    operation = "update"
    path      = "sys/mounts/database"

    data = {
      type        = "database"
      description = "Fixture PostgreSQL dynamic secrets engine."
    }
  }

  request "enable-transit-secrets" {
    operation = "update"
    path      = "sys/mounts/transit"

    data = {
      type        = "transit"
      description = "Fixture Transit engine for observability reference captures."
    }
  }

  request "enable-pki-secrets" {
    operation = "update"
    path      = "sys/mounts/pki"

    data = {
      type        = "pki"
      description = "Fixture PKI engine for observability reference captures."
    }
  }

  request "generate-pki-root" {
    operation = "update"
    path      = "pki/root/generate/internal"

    data = {
      common_name = "OpenBao Observability Fixture Root CA"
      ttl         = "24h"
    }
  }

  request "create-pki-observability-role" {
    operation = "update"
    path      = "pki/roles/observability-dot-local"

    data = {
      allowed_domains    = ["observability.local"]
      allow_bare_domains = true
      allow_subdomains   = true
      max_ttl            = "1h"
    }
  }

  request "create-transit-payments-key" {
    operation = "update"
    path      = "transit/keys/payments"

    data = {
      type                   = "aes256-gcm96"
      derived                = false
      exportable             = false
      allow_plaintext_backup = false
    }
  }

  request "configure-postgres-database" {
    operation = "update"
    path      = "database/config/postgres"

    data = {
      plugin_name     = "postgresql-database-plugin"
      allowed_roles   = ["readonly", "failure-*"]
      connection_url  = "postgresql://{{username}}:{{password}}@%s:5432/openbao_app?sslmode=disable"
      username        = "%s"
      password        = "%s"
    }
  }

  request "create-postgres-readonly-role" {
    operation = "update"
    path      = "database/roles/readonly"

    data = {
      db_name             = "postgres"
      default_ttl         = "5m"
      max_ttl             = "30m"
      creation_statements = [
        "CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';",
        "GRANT CONNECT ON DATABASE openbao_app TO \"{{name}}\";"
      ]
    }
  }

  request "create-fixture-admin-policy" {
    operation = "update"
    path      = "sys/policies/acl/fixture-admin"

    data = {
      policy = <<EOT
path "*" {
  capabilities = ["create", "read", "update", "delete", "list", "sudo"]
}
EOT
    }
  }

  request "create-app-reader-policy" {
    operation = "update"
    path      = "sys/policies/acl/app-reader"

    data = {
      policy = <<EOT
path "secret/data/apps/*" {
  capabilities = ["read"]
}

path "secret/metadata/apps/*" {
  capabilities = ["list", "read"]
}
EOT
    }
  }

  request "create-identity-auditor-policy" {
    operation = "update"
    path      = "sys/policies/acl/identity-auditor"

    data = {
      policy = <<EOT
path "identity/*" {
  capabilities = ["read", "list"]
}

path "sys/auth" {
  capabilities = ["read", "list"]
}
EOT
    }
  }

  request "create-demo-admin-user" {
    operation = "update"
    path      = "auth/userpass/users/demo-admin"

    data = {
      password       = "openbao-observability"
      token_policies = ["fixture-admin"]
    }
  }

  request "create-demo-reader-user" {
    operation = "update"
    path      = "auth/userpass/users/demo-reader"

    data = {
      password       = "openbao-observability"
      token_policies = ["app-reader"]
    }
  }

  request "create-observability-approle" {
    operation = "update"
    path      = "auth/approle/role/observability-app"

    data = {
      token_policies    = ["app-reader"]
      token_ttl         = "15m"
      token_max_ttl     = "1h"
      secret_id_ttl     = "30m"
      secret_id_num_uses = 5
    }
  }

  request "create-fixture-token" {
    operation = "update"
    path      = "auth/token/create-orphan"

    data = {
      id       = "%s"
      policies = ["fixture-admin"]
    }
  }
}`, databaseHost, fixturePostgresUser, fixturePostgresPassword, fixtureAdminToken)
}

func writeConfig(prefix, path string) error {
	config := fmt.Sprintf(`telemetry {
  prometheus_retention_time = "%s"
  disable_hostname          = true
  metrics_prefix            = "%s"
  usage_gauge_period        = "%s"
}

audit "file" "local-file" {
  description = "Validation audit file."

  options {
    file_path     = "/tmp/openbao-audit.jsonl"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}
`, fixturePromRetention, prefix, fixtureUsageGaugePeriod)

	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		return fmt.Errorf("write OpenBao config %s: %w", path, err)
	}
	return nil
}

func waitForHealth(ctx context.Context, port int, outputPath, container string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/sys/health", port)
	deadline := time.Now().Add(90 * time.Second)

	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req) //nolint:bodyclose // readAndClose closes the response body.
		if err == nil {
			body, readErr := readAndClose(resp.Body)
			if readErr != nil {
				return readErr
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				if err := os.WriteFile(outputPath, body, 0o644); err != nil {
					return fmt.Errorf("write health fixture %s: %w", outputPath, err)
				}
				return nil
			}
			lastErr = fmt.Errorf("health endpoint returned %s", resp.Status)
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return fmt.Errorf("OpenBao did not become healthy on %s: %w%s", url, lastErr, dockerDiagnostics(ctx, container))
}

func dockerDiagnostics(ctx context.Context, container string) string {
	if container == "" {
		return ""
	}

	inspect, _, _ := dockerCombined(
		ctx,
		"inspect",
		"--format",
		"state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}}",
		container,
	)
	logs, _, _ := dockerCombined(ctx, "logs", "--tail", "120", container)

	var builder strings.Builder
	builder.WriteString("\nDocker diagnostics for ")
	builder.WriteString(container)
	builder.WriteString(":\n")
	if len(inspect) > 0 {
		builder.WriteString(strings.TrimSpace(string(inspect)))
		builder.WriteString("\n")
	}
	if len(logs) > 0 {
		builder.WriteString(strings.TrimSpace(string(logs)))
		builder.WriteString("\n")
	}
	return builder.String()
}

func waitForInitializedUnsealed(ctx context.Context, port int, outputPath string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/sys/health", port)
	deadline := time.Now().Add(90 * time.Second)

	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req) //nolint:bodyclose // readAndClose closes the response body.
		if err == nil {
			body, readErr := readAndClose(resp.Body)
			if readErr != nil {
				return readErr
			}
			var health struct {
				Initialized bool `json:"initialized"`
				Sealed      bool `json:"sealed"`
			}
			if err := json.Unmarshal(body, &health); err != nil {
				lastErr = fmt.Errorf("parse health response from %s: %w", url, err)
			} else if health.Initialized && !health.Sealed {
				if err := os.WriteFile(outputPath, body, 0o644); err != nil {
					return fmt.Errorf("write Raft health fixture %s: %w", outputPath, err)
				}
				return nil
			} else {
				lastErr = fmt.Errorf("health endpoint returned initialized=%v sealed=%v", health.Initialized, health.Sealed)
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return fmt.Errorf("OpenBao did not become initialized and unsealed on %s: %w", url, lastErr)
}

func (r *captureRun) captureVersion(ctx context.Context, container, prefix string) error {
	out, _, err := dockerCombined(ctx, "exec", container, "bao", "version")
	if err != nil {
		return fmt.Errorf("capture OpenBao version for %s: %w", prefix, err)
	}
	return writeFile(metadataPath(r.options, prefix, "version.txt"), out)
}

func (r *captureRun) captureAPIAuditRejection(ctx context.Context, container, prefix string) error {
	out, code, err := dockerCombined(ctx, "exec",
		"-e", "BAO_ADDR=http://127.0.0.1:8200",
		"-e", "BAO_TOKEN="+r.options.RootToken,
		container,
		"bao",
		"audit",
		"enable",
		"file",
		"file_path=/tmp/api-audit.jsonl",
	)
	if err != nil && code < 0 {
		return fmt.Errorf("run API audit rejection check for %s: %w", prefix, err)
	}

	var content bytes.Buffer
	content.Write(out)
	if !bytes.HasSuffix(out, []byte("\n")) {
		content.WriteByte('\n')
	}
	fmt.Fprintf(&content, "exit_code=%d\n", code)

	return writeFile(metadataPath(r.options, prefix, "api-audit-enable.txt"), content.Bytes())
}

func (r *captureRun) exerciseOpenBao(ctx context.Context, container, prefix string) error {
	var output bytes.Buffer
	commands := [][]string{
		{"bao", "secrets", "enable", "-path=demo", "kv-v2"},
		{"bao", "kv", "put", "demo/example", "value=test"},
		{"bao", "auth", "enable", "userpass"},
		{"bao", "token", "create", "-ttl=1m"},
	}

	for _, command := range commands {
		args := []string{
			"exec",
			"-e", "BAO_ADDR=http://127.0.0.1:8200",
			"-e", "BAO_TOKEN=" + r.options.RootToken,
			container,
		}
		args = append(args, command...)

		out, _, err := dockerCombined(ctx, args...)
		output.Write(out)
		if !bytes.HasSuffix(out, []byte("\n")) {
			output.WriteByte('\n')
		}
		if err != nil {
			_ = writeFile(metadataPath(r.options, prefix, "exercise.txt"), output.Bytes())
			return fmt.Errorf("exercise OpenBao command %q for %s: %w", strings.Join(command, " "), prefix, err)
		}
	}

	return writeFile(metadataPath(r.options, prefix, "exercise.txt"), output.Bytes())
}

func (r *captureRun) exerciseRaft(ctx context.Context, node raftNode, prefix string) error {
	var output bytes.Buffer
	commands := [][]string{
		{"bao", "kv", "put", "secret/observability", "sample=raft"},
		{"bao", "kv", "get", "-format=json", "secret/observability"},
		{"bao", "auth", "list", "-format=json"},
		{"bao", "secrets", "list", "-format=json"},
		{"bao", "token", "lookup", "-format=json"},
		{"bao", "token", "create", "-policy=fixture-admin", "-ttl=1m"},
	}

	for _, command := range commands {
		out, _, err := r.baoExec(ctx, node, command...)
		output.Write(out)
		if !bytes.HasSuffix(out, []byte("\n")) {
			output.WriteByte('\n')
		}
		if err != nil {
			_ = writeFile(raftClusterMetadataPath(r.options, prefix, "exercise.txt"), output.Bytes())
			return fmt.Errorf("exercise OpenBao Raft command %q: %w", strings.Join(command, " "), err)
		}
	}

	return writeFile(raftClusterMetadataPath(r.options, prefix, "exercise.txt"), output.Bytes())
}

func (r *captureRun) exerciseRaftReadReplica(ctx context.Context, node raftNode, prefix string) error {
	var output bytes.Buffer
	commands := [][]string{
		{"bao", "status", "-format=json"},
		{"bao", "kv", "get", "-format=json", "secret/observability"},
		{"bao", "auth", "list", "-format=json"},
		{"bao", "secrets", "list", "-format=json"},
		{"bao", "operator", "raft", "list-peers", "-format=json"},
		{"bao", "operator", "raft", "autopilot", "state", "-format=json"},
	}

	for _, command := range commands {
		out, _, err := r.baoExec(ctx, node, command...)
		output.Write(out)
		if !bytes.HasSuffix(out, []byte("\n")) {
			output.WriteByte('\n')
		}
		if err != nil {
			_ = writeFile(raftClusterMetadataPath(r.options, prefix, node.ID+"-exercise.txt"), output.Bytes())
			return fmt.Errorf(
				"exercise OpenBao Raft read replica command %q on %s: %w",
				strings.Join(command, " "),
				node.ID,
				err,
			)
		}
	}

	return writeFile(raftClusterMetadataPath(r.options, prefix, node.ID+"-exercise.txt"), output.Bytes())
}

func (r *captureRun) runRaftScenario(ctx context.Context, node raftNode, prefix, postgresHost string) error {
	return RunScenario(ctx, ScenarioOptions{
		Address:      fmt.Sprintf("http://127.0.0.1:%d", node.Port),
		PostgresHost: postgresHost,
		OutputPath:   raftClusterMetadataPath(r.options, prefix, "scenario.json"),
	})
}

func (r *captureRun) captureRaftState(ctx context.Context, nodes []raftNode, prefix string) error {
	leader := nodes[0]
	if err := r.captureRaftCommand(
		ctx,
		leader,
		raftClusterMetadataPath(r.options, prefix, "peers.json"),
		"bao",
		"operator",
		"raft",
		"list-peers",
		"-format=json",
	); err != nil {
		return err
	}
	if err := r.captureRaftCommand(
		ctx,
		leader,
		raftClusterMetadataPath(r.options, prefix, "autopilot-state.json"),
		"bao",
		"operator",
		"raft",
		"autopilot",
		"state",
		"-format=json",
	); err != nil {
		return err
	}
	for _, node := range nodes {
		if err := r.captureRaftCommand(
			ctx,
			node,
			raftMetadataPath(r.options, prefix, node.ID, "status.json"),
			"bao",
			"status",
			"-format=json",
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *captureRun) captureRaftCommand(
	ctx context.Context,
	node raftNode,
	outputPath string,
	command ...string,
) error {
	out, _, err := r.baoExec(ctx, node, command...)
	if err != nil {
		return fmt.Errorf("capture OpenBao Raft command %q from %s: %w", strings.Join(command, " "), node.ID, err)
	}
	return writeFile(outputPath, out)
}

func (r *captureRun) waitForRaftTopology(
	ctx context.Context,
	leader raftNode,
	expectedPeers, expectedVoters int,
) error {
	deadline := time.Now().Add(90 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		out, _, err := r.baoExec(ctx, leader, "bao", "operator", "raft", "list-peers", "-format=json")
		if err == nil {
			servers, parseErr := parseRaftServers(out)
			if parseErr != nil {
				lastErr = parseErr
			} else if len(servers) == expectedPeers && countVoters(servers) == expectedVoters {
				return nil
			} else {
				lastErr = fmt.Errorf(
					"expected %d Raft peers with %d voters, found %d peers with %d voters",
					expectedPeers,
					expectedVoters,
					len(servers),
					countVoters(servers),
				)
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	return fmt.Errorf(
		"OpenBao Raft peers did not converge to %d peers with %d voters: %w",
		expectedPeers,
		expectedVoters,
		lastErr,
	)
}

func (r *captureRun) waitForAutopilotTolerance(
	ctx context.Context,
	leader raftNode,
	expectedServers, minFailureTolerance int,
) error {
	deadline := time.Now().Add(120 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		out, _, err := r.baoExec(ctx, leader, "bao", "operator", "raft", "autopilot", "state", "-format=json")
		if err == nil {
			state, parseErr := parseAutopilotState(out)
			if parseErr != nil {
				lastErr = parseErr
			} else if state.Healthy && len(state.Servers) == expectedServers && state.FailureTolerance >= minFailureTolerance {
				return nil
			} else {
				lastErr = fmt.Errorf(
					"autopilot healthy=%v servers=%d failure_tolerance=%d",
					state.Healthy,
					len(state.Servers),
					state.FailureTolerance,
				)
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}

	return fmt.Errorf("OpenBao Autopilot did not reach failure tolerance %d: %w", minFailureTolerance, lastErr)
}

func (r *captureRun) baoExec(ctx context.Context, node raftNode, command ...string) ([]byte, int, error) {
	args := make([]string, 0, 6+len(command))
	args = append(args,
		"exec",
		"-e", "BAO_ADDR=http://127.0.0.1:8200",
		"-e", "BAO_TOKEN="+fixtureAdminToken,
		node.Container,
	)
	args = append(args, command...)
	return dockerCombined(ctx, args...)
}

func (r *captureRun) captureMetrics(ctx context.Context, prefix string, port int) error {
	return captureMetricsFromPort(ctx, prefix, port, r.options.RootToken, metricsPath(r.options, prefix))
}

func (r *captureRun) captureRaftMetrics(ctx context.Context, nodes []raftNode, prefix string) error {
	for _, node := range nodes {
		if err := captureMetricsFromPort(
			ctx,
			raftPrefix(prefix)+"-"+node.ID,
			node.Port,
			fixtureAdminToken,
			raftMetricsPath(r.options, prefix, node.ID),
		); err != nil {
			return err
		}
	}
	return nil
}

func captureMetricsFromPort(ctx context.Context, label string, port int, token, outputPath string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/sys/metrics?format=prometheus", port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}

	resp, err := client.Do(req) //nolint:bodyclose // readAndClose closes the response body.
	if err != nil {
		return fmt.Errorf("fetch metrics for %s: %w", label, err)
	}

	body, err := readAndClose(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metrics endpoint for %s returned %s: %s", label, resp.Status, string(body))
	}

	return writeFile(outputPath, body)
}

func (r *captureRun) captureAuditLog(ctx context.Context, container, prefix string) error {
	out, _, err := dockerCombined(ctx, "exec", container, "sh", "-c", "cat /tmp/openbao-audit.jsonl")
	if err != nil {
		return fmt.Errorf("capture audit log for %s: %w", prefix, err)
	}
	return writeFile(auditPath(r.options, prefix), out)
}

func (r *captureRun) captureRaftAuditLog(ctx context.Context, node raftNode, prefix string) error {
	out, _, err := dockerCombined(ctx, "exec", node.Container, "sh", "-c", "cat /tmp/openbao-audit.jsonl")
	if err != nil {
		return fmt.Errorf("capture Raft audit log for %s: %w", prefix, err)
	}
	return writeFile(raftAuditPath(r.options, prefix, node.ID), out)
}

func (r *captureRun) cleanup(ctx context.Context) {
	for _, container := range r.containers {
		_, _, _ = dockerCombined(ctx, "rm", "-f", container)
	}
	for _, network := range r.networks {
		_, _, _ = dockerCombined(ctx, "network", "rm", network)
	}
	for _, dir := range r.tempDirs {
		_ = os.RemoveAll(dir)
	}
}

func metadataPath(opts CaptureOptions, prefix, suffix string) string {
	return filepath.Join(opts.OutputDir, "metadata", fmt.Sprintf("openbao-%s-%s-%s", opts.Version, prefix, suffix))
}

func metricsPath(opts CaptureOptions, prefix string) string {
	return filepath.Join(opts.OutputDir, "metrics", fmt.Sprintf("openbao-%s-%s-prefix.prom", opts.Version, prefix))
}

func auditPath(opts CaptureOptions, prefix string) string {
	return filepath.Join(
		opts.OutputDir,
		"logs",
		"audit",
		fmt.Sprintf("openbao-%s-%s-prefix.jsonl", opts.Version, prefix),
	)
}

func raftPrefix(prefix string) string {
	return "raft-" + prefix
}

func raftClusterMetadataPath(opts CaptureOptions, prefix, suffix string) string {
	return filepath.Join(opts.OutputDir, "metadata", fmt.Sprintf("openbao-%s-raft-%s-%s", opts.Version, prefix, suffix))
}

func raftMetadataPath(opts CaptureOptions, prefix, nodeID, suffix string) string {
	return filepath.Join(
		opts.OutputDir,
		"metadata",
		fmt.Sprintf("openbao-%s-raft-%s-%s-%s", opts.Version, prefix, nodeID, suffix),
	)
}

func raftMetricsPath(opts CaptureOptions, prefix, nodeID string) string {
	return filepath.Join(
		opts.OutputDir,
		"metrics",
		fmt.Sprintf("openbao-%s-raft-%s-%s.prom", opts.Version, prefix, nodeID),
	)
}

func raftAuditPath(opts CaptureOptions, prefix, nodeID string) string {
	return filepath.Join(
		opts.OutputDir,
		"logs",
		"audit",
		fmt.Sprintf("openbao-%s-raft-%s-%s.jsonl", opts.Version, prefix, nodeID),
	)
}
