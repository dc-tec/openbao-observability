package fixtures

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CaptureOptions struct {
	Version   string
	Image     string
	OutputDir string
	PortBase  int
	RootToken string
}

type captureRun struct {
	options    CaptureOptions
	containers []string
	tempDirs   []string
}

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

	_, _, _ = combined(ctx, "docker", "rm", "-f", name)
	r.containers = append(r.containers, name)

	_, _, err = combined(ctx,
		"docker",
		"run",
		"--rm",
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
	if err := waitForHealth(ctx, port, healthPath); err != nil {
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
	if err := r.captureMetrics(ctx, prefix, port); err != nil {
		return err
	}
	if err := r.captureAuditLog(ctx, name, prefix); err != nil {
		return err
	}

	return nil
}

func writeConfig(prefix, path string) error {
	config := fmt.Sprintf(`telemetry {
  prometheus_retention_time = "30s"
  disable_hostname          = true
  metrics_prefix           = "%s"
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
`, prefix)

	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		return fmt.Errorf("write OpenBao config %s: %w", path, err)
	}
	return nil
}

func waitForHealth(ctx context.Context, port int, outputPath string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/sys/health", port)
	deadline := time.Now().Add(30 * time.Second)

	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
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

	return fmt.Errorf("OpenBao did not become healthy on %s: %w", url, lastErr)
}

func (r *captureRun) captureVersion(ctx context.Context, container, prefix string) error {
	out, _, err := combined(ctx, "docker", "exec", container, "bao", "version")
	if err != nil {
		return fmt.Errorf("capture OpenBao version for %s: %w", prefix, err)
	}
	return writeFile(metadataPath(r.options, prefix, "version.txt"), out)
}

func (r *captureRun) captureAPIAuditRejection(ctx context.Context, container, prefix string) error {
	out, code, err := combined(ctx,
		"docker",
		"exec",
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

		out, _, err := combined(ctx, "docker", args...)
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

func (r *captureRun) captureMetrics(ctx context.Context, prefix string, port int) error {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/sys/metrics?format=prometheus", port)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Vault-Token", r.options.RootToken)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch metrics for %s: %w", prefix, err)
	}

	body, err := readAndClose(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("metrics endpoint for %s returned %s: %s", prefix, resp.Status, string(body))
	}

	return writeFile(metricsPath(r.options, prefix), body)
}

func (r *captureRun) captureAuditLog(ctx context.Context, container, prefix string) error {
	out, _, err := combined(ctx, "docker", "exec", container, "sh", "-c", "cat /tmp/openbao-audit.jsonl")
	if err != nil {
		return fmt.Errorf("capture audit log for %s: %w", prefix, err)
	}
	return writeFile(auditPath(r.options, prefix), out)
}

func (r *captureRun) cleanup(ctx context.Context) {
	for _, container := range r.containers {
		_, _, _ = combined(ctx, "docker", "rm", "-f", container)
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
	return filepath.Join(opts.OutputDir, "logs", "audit", fmt.Sprintf("openbao-%s-%s-prefix.jsonl", opts.Version, prefix))
}
