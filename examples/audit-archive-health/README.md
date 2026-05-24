# Audit archive health example

Use this example to publish the reference audit archive health metrics consumed
by the `OpenBaoAuditArchiveDegraded` alert. It is for teams that already have
an audit archive pipeline and need a small, predictable Prometheus surface for
delivery health.

## What this example provides

- A small Go exporter that reads archive delivery state from a JSON status
  file and exposes Prometheus metrics.
- A sample JSON status file that shows the expected writer contract.
- A recording-rule mapping example for environments that already expose
  archive or SIEM delivery metrics under different names.

The exporter is intentionally backend-neutral. Your archive pipeline owns the
real SIEM, object-store, or WORM delivery logic. The exporter only turns that
pipeline's current state into the five reference metrics.

## Reference metrics

| Metric | Meaning |
| ------ | ------- |
| `openbao_audit_archive_enabled` | `1` when this environment expects durable audit archive delivery. |
| `openbao_audit_archive_delivery_success` | `1` when the archive path is healthy and `0` when delivery is degraded. |
| `openbao_audit_archive_last_success_timestamp_seconds` | Unix timestamp for the last successful archive delivery or acknowledgement. |
| `openbao_audit_archive_delivery_failures_total` | Count of failed archive writes, rejected batches, or failed acknowledgements. |
| `openbao_audit_archive_dead_letter_records_total` | Count of records sent to a dead-letter path instead of the durable archive. |

## Run the exporter

1. Create or update a status file from your archive delivery pipeline.

   ```json
   {
     "enabled": true,
     "delivery_success": true,
     "last_success_timestamp": "2026-05-24T12:00:00Z",
     "delivery_failures_total": 0,
     "dead_letter_records_total": 0,
     "backend": "object-store",
     "pipeline": "openbao-audit-archive"
   }
   ```

   The timestamp is illustrative. Your archive pipeline must update it after
   each successful delivery or acknowledgement.

2. Run the exporter.

   ```shell
   go run ./examples/audit-archive-health/exporter --enabled --status-file examples/audit-archive-health/status.example.json
   ```

3. Scrape the metrics endpoint.

   ```shell
   curl -fsS http://127.0.0.1:19110/metrics
   ```

4. Configure Prometheus to scrape the exporter.

   ```yaml
   scrape_configs:
     - job_name: openbao-audit-archive-health
       static_configs:
         - targets:
             - openbao-audit-archive-health:19110
           labels:
             cluster: <cluster_name>
             environment: <environment_name>
   ```

   - `<cluster_name>`: OpenBao cluster name.
   - `<environment_name>`: Environment name such as `production`.

## Run with Docker Compose

Use the optional Compose profile when you want the local reference stack to
scrape this exporter and evaluate the generated archive alert.

```shell
make compose-audit-archive-up
```

The profile starts a local status writer that updates the demo status file
every 30 seconds. Prometheus uses
`examples/docker-compose/prometheus/prometheus.audit-archive.yml` so the
default stack does not show a down target when the profile is not enabled.

Stop the profile with:

```shell
make compose-audit-archive-down
```

## Scrape in Kubernetes

Use `examples/kubernetes/audit-archive-health-scrape.yaml` when your platform
deploys the exporter and you want Prometheus Operator to scrape it.

Before you apply the example:

- Replace the namespace if your monitoring stack does not use `monitoring`.
- Replace the `ServiceMonitor` labels so your Prometheus resource selects it.
- Deploy the exporter image and make sure its Pod uses the
  `app.kubernetes.io/name=openbao-audit-archive-health` label.

## Status file contract

Write the status file atomically from the archive pipeline. For example, write
to a temporary file and rename it over the previous status file.

| Field | Required | Meaning |
| ----- | -------- | ------- |
| `enabled` | Optional | Overrides the exporter `--enabled` flag when present. |
| `delivery_success` | Required when enabled. | `true` only when the archive path is currently healthy. |
| `last_success_timestamp` | Required when enabled. | RFC3339 timestamp or Unix timestamp string for the last successful delivery. |
| `last_success_timestamp_seconds` | Optional | Unix timestamp as a number. Takes precedence over `last_success_timestamp`. |
| `delivery_failures_total` | Required when enabled. | Monotonic count of failed archive deliveries. |
| `dead_letter_records_total` | Required when enabled. | Monotonic count of records sent to a dead-letter path. |
| `backend` | Optional | Stable backend label, such as `s3-object-lock` or `siem`. |
| `pipeline` | Optional | Stable archive pipeline label. |

Use stable labels only. Do not include request IDs, request paths, entity IDs,
token accessors, client addresses, or other request-derived values.

## Map existing metrics

Use `recording-rules.example.yaml` when your archive pipeline already exposes
health metrics. Replace the example source metric names with the names from
your SIEM forwarder, object-store writer, or archive gateway.

The generated critical alert reads the reference metric names, so recording
rules are enough when you do not need the example exporter.

## Failure behavior

When the exporter runs with `--enabled` and cannot read or parse the status
file, it emits:

- `openbao_audit_archive_enabled` as `1`.
- `openbao_audit_archive_delivery_success` as `0`.

That makes the `OpenBaoAuditArchiveDegraded` alert fire instead of hiding a
broken status writer.

When the exporter is not enabled and the status file does not enable archive
delivery, it emits only `openbao_audit_archive_enabled` as `0`. This keeps local
and exempt environments quiet.

## What's next

- Use [Audit archive reference design](../../docs/audit/audit-archive-reference-design.md)
  to choose the archive pattern and failure tests.
- Use [Audit archive degraded](../../docs/runbooks/audit-archive-degraded.md)
  to respond when archive health fails.
