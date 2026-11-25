// Command otelhec qualifies the example's metrics pipeline without a Splunk server.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type caseResult struct {
	Image        string                  `json:"image"`
	Version      string                  `json:"version_output"`
	ConfigSHA256 string                  `json:"config_sha256"`
	Samples      map[string]metricSample `json:"samples"`
}

type qualification struct {
	ProfileSHA256 string     `json:"profile_sha256"`
	Baseline      caseResult `json:"baseline"`
	Candidate     caseResult `json:"candidate"`
	Added         []string   `json:"added_dimensions"`
	Outcome       string     `json:"outcome"`
}

type fixture struct {
	directory string
	gateway   string
	target    string
	capture   *capture
	name      string
	source    []byte
}

func main() {
	artifacts := flag.String("artifacts", "tmp/otel-hec-qualification", "qualification evidence directory")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := run(ctx, *artifacts); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, artifacts string) (runErr error) {
	if runtime.GOOS != "linux" || os.Getenv("CI") != "true" {
		return errors.New("collector qualification runs only on a dedicated Linux CI runner")
	}
	directory, err := filepath.Abs(artifacts)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	// Paths are fixed repository inputs, not paths received from a scrape or HEC request.
	source, err := os.ReadFile(profilePath)
	if err != nil {
		return err
	}
	compose, err := os.ReadFile("examples/splunk/compose.yaml")
	if err != nil {
		return err
	}
	image, err := candidateImage(compose)
	if err != nil {
		return err
	}
	if !imagePattern.MatchString(image) || image == baselineImage {
		return fmt.Errorf("candidate must be a new digest-pinned collector image: %s", image)
	}
	state, server, err := startFixture(ctx, directory, source)
	if err != nil {
		return err
	}
	defer server.Close()
	report := qualification{ProfileSHA256: digest(source), Outcome: "failed"}
	defer func() {
		runErr = errors.Join(runErr, state.writeReport(report))
	}()
	report.Baseline, err = state.runCase(ctx, "baseline", baselineImage, false)
	if err != nil {
		return fmt.Errorf("baseline qualification: %w", err)
	}
	report.Candidate, err = state.runCase(ctx, "candidate", image, true)
	if err != nil {
		return fmt.Errorf("candidate qualification: %w", err)
	}
	if err := compareSeries(report.Baseline.Samples, report.Candidate.Samples); err != nil {
		return err
	}
	report.Added = []string{"__scheme__", "__metrics_path__"}
	report.Outcome = "passed"
	log.Printf("Qualified %d metric series with only the two approved dimensions added", len(report.Candidate.Samples))
	return nil
}

func startFixture(ctx context.Context, directory string, source []byte) (*fixture, *httptest.Server, error) {
	output, err := docker(ctx, "network", "inspect", "bridge", "--format", "{{(index .IPAM.Config 0).Gateway}}")
	if err != nil {
		return nil, nil, err
	}
	gateway := strings.TrimSpace(string(output))
	if net.ParseIP(gateway) == nil {
		return nil, nil, errors.New("bridge gateway is not an IP address")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(gateway, "0"))
	if err != nil {
		return nil, nil, err
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		_ = listener.Close()
		return nil, nil, err
	}
	captured := newCapture()
	server := httptest.NewUnstartedServer(captured)
	if err := server.Listener.Close(); err != nil {
		_ = listener.Close()
		return nil, nil, err
	}
	server.Listener = listener
	server.Config.ReadHeaderTimeout = 2 * time.Second
	server.Config.ReadTimeout = 5 * time.Second
	server.Config.WriteTimeout = 5 * time.Second
	server.Start()
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		server.Close()
		return nil, nil, err
	}
	return &fixture{
		directory: directory, gateway: gateway, target: net.JoinHostPort("host.docker.internal", port),
		capture: captured, name: "otel-hec-" + hex.EncodeToString(random), source: source,
	}, server, nil
}

func (f *fixture) runCase(ctx context.Context, name, image string, added bool) (result caseResult, caseErr error) {
	result.Image = image
	if _, err := docker(ctx, "pull", image); err != nil {
		return result, err
	}
	inspect, err := docker(ctx, "image", "inspect", image)
	if err != nil {
		return result, err
	}
	if err := f.write(name+"-image.json", inspect); err != nil {
		return result, err
	}
	version, err := docker(ctx, "run", "--rm", "--network=none", image, "--version")
	if err != nil {
		return result, err
	}
	match := imagePattern.FindStringSubmatch(image)
	if len(match) != 2 || !strings.Contains(string(version), "version "+match[1]) {
		return result, fmt.Errorf("collector binary version does not match image: %s", version)
	}
	result.Version = strings.TrimSpace(string(version))
	config, err := fixtureConfig(f.source, f.target, "http://"+f.target+"/"+name+"/services/collector")
	if err != nil {
		return result, err
	}
	result.ConfigSHA256 = digest(config)
	if err := f.write(name+"-collector.yaml", config); err != nil {
		return result, err
	}
	container := f.name + "-" + name
	defer func() {
		caseErr = errors.Join(caseErr, f.finishCase(container, name, &result, added))
	}()
	uid := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	_, err = docker(ctx, "run", "--detach", "--pull=never", "--name", container,
		"--network=bridge", "--add-host", "host.docker.internal:"+f.gateway,
		"--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--user", uid,
		"--volume", filepath.Join(f.directory, name+"-collector.yaml")+":/qualification.yaml:ro",
		image, "--config=/qualification.yaml")
	if err != nil {
		return result, err
	}
	result.Samples, err = f.awaitSamples(ctx, name, added)
	return result, err
}

func (f *fixture) awaitSamples(ctx context.Context, name string, added bool) (map[string]metricSample, error) {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	firstScrape := f.capture.scrapes.Load()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, errors.New("timed out waiting for all synthetic namespace series and repeated scrapes")
		case <-ticker.C:
			series, err := f.capture.snapshot(name)
			if err != nil {
				return nil, err
			}
			if len(series) == len(expectedMetrics)*2 && f.capture.scrapes.Load()-firstScrape >= 2 {
				return series, validateSeries(series, f.target, added)
			}
		}
	}
}

func (f *fixture) finishCase(container, name string, result *caseResult, added bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, stopErr := docker(ctx, "stop", "--time=5", container)
	logs, logErr := docker(ctx, "logs", container)
	_, removeErr := docker(ctx, "rm", "--force", container)
	f.capture.mu.Lock()
	evidenceErr := f.writeEvents(name+"-hec.json", f.capture.events[name])
	f.capture.mu.Unlock()
	var validationErr error
	if result.Samples != nil {
		result.Samples, validationErr = f.capture.snapshot(name)
		if validationErr == nil {
			validationErr = validateSeries(result.Samples, f.target, added)
		}
	}
	return errors.Join(stopErr, logErr, removeErr, f.write(name+".log", logs), evidenceErr, validationErr)
}

func docker(ctx context.Context, args ...string) ([]byte, error) {
	// #nosec G204 -- The executable is fixed; arguments are separate values, and collector images are digest-pinned.
	output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

func (f *fixture) write(name string, data []byte) error {
	root, err := os.OpenRoot(f.directory)
	if err != nil {
		return err
	}
	return errors.Join(root.WriteFile(name, data, 0o600), root.Close())
}

func (f *fixture) writeReport(report qualification) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return f.write("qualification.json", append(data, '\n'))
}

func (f *fixture) writeEvents(name string, events []hecEvent) error {
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	return f.write(name, append(data, '\n'))
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
