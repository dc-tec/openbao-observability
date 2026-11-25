# Collector HEC regression fixture

This fixture compares the actual digest-pinned Collector Contrib 0.164.0 image
with the collector image in `examples/splunk/compose.yaml`. The hosted
`OTel HEC qualification` job runs both images sequentially on an isolated
Linux runner. It uses synthetic Prometheus input and a local HEC capture
sink. It does not start Splunk or contact an external test provider.

Run the fixture's unit tests from the repository root:

```shell
go test -race ./hack/otelhec
```

The runtime command is restricted to Linux with `CI=true`. The workflow runs
`make qualify-otel-hec`; local unit tests do not start Docker or listeners.
Do not set `CI=true` on a workstation to bypass that restriction.

## Pipeline and assertions

The fixture derives its collector configuration from
`examples/splunk/local/otel.yaml`. It retains the OpenBao scrape job's metric
selection and relabeling, the metric-identity transform, batching, and HEC
metrics exporter. It replaces the static scrape target, scrape cadence, HEC
endpoint, and token with fixture inputs, and adds a synthetic Kubernetes
namespace. Other scrape jobs, log pipelines, and storage extensions are not
part of this comparison.

Four gauges produce eight series across `team-a` and `team-b`. Assertions
check metric names and values, HEC index and sourcetype, target/job/pod
identity, deployment labels, namespace separation, and repeated-scrape
stability. A conflicting source `cluster` label must not override target
identity. An unselected metric must not reach HEC.

The baseline must omit `__scheme__` and `__metrics_path__`. The candidate must
include exactly the expected values, `http` and `/v1/sys/metrics`. After
removing only those two approved additions, all captured input series must
match the baseline. Malformed HEC requests and unexpected dimensions fail
qualification. Scrape-health metrics remain in the captured evidence but
are not inputs to the value comparison.

## Isolation and evidence

The driver has a four-minute deadline. It binds its source and sink to the
runner's Docker bridge gateway and starts containers with a read-only root
filesystem, no Linux capabilities, no new privileges, and a read-only
configuration mount. It mounts no Docker socket, supplies no real secrets,
and stops and removes only its uniquely named collector containers.

The workflow retains `tmp/otel-hec-qualification/` as an artifact, including
failed-run evidence when available:

- `qualification.json`: image references, binary versions, configuration
  hashes, captured series, and comparison outcome.
- `baseline-*` and `candidate-*`: image inspection, collector configuration,
  logs, and captured HEC events.

This test qualifies the synthetic metrics pipeline through its HEC wire
payload. It does not qualify Splunk ingestion, indexed searches, dashboards,
historical series continuity, TLS, logs, external probes, or production
throughput. The recorded Enterprise validation retains its own version and
environment limits.
