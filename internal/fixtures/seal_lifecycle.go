package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dc-tec/openbao-observability/internal/promtext"
	baoapi "github.com/openbao/openbao/api/v2"
)

type sealPhase struct {
	name        string
	initialized bool
	sealed      bool
}

var sealPhases = []sealPhase{
	{"startup", false, true},
	{"unsealed", true, false},
	{"resealed", true, true},
	{"restarted", true, true},
	{"recovered", true, false},
}

func (r *captureRun) captureSealLifecycle(ctx context.Context) (retErr error) {
	fmt.Printf("capturing seal lifecycle for OpenBao %s\n", r.options.Version)
	name := "openbao-observability-seal-" + strings.ReplaceAll(r.options.Version, ".", "-")
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("%w%s", retErr, dockerDiagnostics(ctx, name))
		}
	}()
	port := r.options.PortBase + 40
	if err := r.startSealFixture(ctx, name, port); err != nil {
		return err
	}
	client, err := fixtureClient(port, "")
	if err != nil {
		return err
	}
	if err := r.captureSealPhase(ctx, client, port, sealPhases[0]); err != nil {
		return err
	}
	init, err := client.Sys().InitWithContext(ctx, &baoapi.InitRequest{SecretShares: 1, SecretThreshold: 1})
	if err != nil {
		return fmt.Errorf("initialize seal fixture: %w", err)
	}
	client.SetToken(init.RootToken)
	if len(init.Keys) != 1 {
		return fmt.Errorf("expected one local fixture unseal key")
	}
	if _, err := client.Sys().UnsealWithContext(ctx, init.Keys[0]); err != nil {
		return err
	}
	if err := r.captureSealPhase(ctx, client, port, sealPhases[1]); err != nil {
		return err
	}
	if err := client.Sys().SealWithContext(ctx); err != nil {
		return err
	}
	if err := r.captureSealPhase(ctx, client, port, sealPhases[2]); err != nil {
		return err
	}
	if _, _, err := dockerCombined(ctx, "restart", name); err != nil {
		return err
	}
	if err := r.captureSealPhase(ctx, client, port, sealPhases[3]); err != nil {
		return err
	}
	if _, err := client.Sys().UnsealWithContext(ctx, init.Keys[0]); err != nil {
		return err
	}
	return r.captureSealPhase(ctx, client, port, sealPhases[4])
}

func fixtureClient(port int, token string) (*baoapi.Client, error) {
	config := baoapi.DefaultConfig()
	config.Address = fmt.Sprintf("http://127.0.0.1:%d", port)
	config.MaxRetries = 0
	client, err := baoapi.NewClient(config)
	if err != nil {
		return nil, err
	}
	client.SetToken(token)
	return client, nil
}

func (r *captureRun) captureSealPhase(ctx context.Context, client *baoapi.Client, port int, phase sealPhase) error {
	var status *baoapi.SealStatusResponse
	err := awaitFixture(ctx, func() (bool, error) {
		var err error
		status, err = client.Sys().SealStatusWithContext(ctx)
		if err != nil {
			return false, err
		}
		return status.Initialized == phase.initialized && status.Sealed == phase.sealed, nil
	})
	if err != nil {
		return fmt.Errorf("seal phase %s: %w", phase.name, err)
	}
	// Wait beyond the configured five-second retention so stale values cannot pass.
	if err := fixtureDelay(ctx, 16*time.Second); err != nil {
		return err
	}
	path := sealPhasePath(r.options.OutputDir, phase.name)
	if err := captureMetricsFromPort(ctx, phase.name, port, "", path+".prom"); err != nil {
		return err
	}
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return writeFile(path+".json", data)
}

func awaitFixture(ctx context.Context, check func() (bool, error)) error {
	var lastErr error
	for range 60 {
		ok, err := check()
		if ok && err == nil {
			return nil
		}
		lastErr = err
		if err := fixtureDelay(ctx, time.Second); err != nil {
			return err
		}
	}
	return fmt.Errorf("fixture state did not converge: %v", lastErr)
}

func fixtureDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func sealPhasePath(dir, phase string) string {
	return filepath.Join(dir, "lifecycle", "seal-"+phase)
}

func checkSealLifecycle(opts VerifyOptions) error {
	for _, phase := range sealPhases {
		if err := checkSealPhase(opts, phase); err != nil {
			return err
		}
	}
	return nil
}

func checkSealPhase(opts VerifyOptions, phase sealPhase) error {
	path := sealPhasePath(opts.FixtureDir, phase.name)
	data, err := os.ReadFile(path + ".json") // #nosec G304 -- Read a local lifecycle fixture.
	if err != nil {
		return err
	}
	var status baoapi.SealStatusResponse
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}
	if status.Initialized != phase.initialized || status.Sealed != phase.sealed {
		return fmt.Errorf("unexpected API state for seal phase %s", phase.name)
	}
	families, err := promtext.LoadFamilies(path + ".prom")
	if err != nil {
		return err
	}
	family := families["vault_core_unsealed"]
	if phase.sealed && strings.HasPrefix(opts.Version, "2.6.") {
		if family != nil {
			return fmt.Errorf("expected expired 2.6 seal gauge in %s", path)
		}
		return nil
	}
	want := float64(1)
	if phase.sealed {
		want = 0
	}
	if family == nil || len(family.GetMetric()) == 0 {
		return fmt.Errorf("missing seal gauge in %s", path)
	}
	for _, metric := range family.GetMetric() {
		if metric.GetGauge() == nil || metric.GetGauge().GetValue() != want {
			return fmt.Errorf("seal gauge in %s does not match API state", path)
		}
	}
	return nil
}

const sealLifecycleConfig = `disable_mlock = true
api_addr = "http://127.0.0.1:8200"
cluster_addr = "http://127.0.0.1:8201"
storage "raft" {
  path = "/tmp/openbao-seal-data"
  node_id = "seal-fixture"
}
listener "tcp" {
  address = "0.0.0.0:8200"
  tls_disable = true
  telemetry {
    unauthenticated_metrics_access = true
  }
}
telemetry {
  disable_hostname = true
  prometheus_retention_time = "5s"
}
`

func (r *captureRun) startSealFixture(ctx context.Context, name string, port int) error {
	dir, err := os.MkdirTemp("/tmp", "openbao-seal-lifecycle-")
	if err != nil {
		return err
	}
	r.tempDirs = append(r.tempDirs, dir)
	config := filepath.Join(dir, "config.hcl")
	if err := writeFile(config, []byte(sealLifecycleConfig)); err != nil {
		return err
	}
	r.containers = append(r.containers, name)
	// Container-local storage survives docker restart and uses the image user's ownership.
	// A host-owned 0700 bind mount prevents the unprivileged server from starting on Linux.
	_, _, err = dockerCombined(ctx, "run", "--detach", "--name", name,
		"--publish", fmt.Sprintf("127.0.0.1:%d:8200", port),
		"--volume", config+":/bao/config/config.hcl:ro",
		r.options.Image, "server", "-config=/bao/config/config.hcl")
	if err != nil {
		return fmt.Errorf("start seal lifecycle fixture: %w", err)
	}

	return nil
}
