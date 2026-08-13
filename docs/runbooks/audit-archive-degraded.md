# Audit archive degraded

Use this runbook when `OpenBaoAuditArchiveDegraded` or
`OpenBaoAuditArchiveSignalMissing` fires. These alerts report a degraded
archive pipeline or a missing required health signal. The steps help you
protect audit evidence while you restore archive delivery.

## Before you begin

- Get access to Prometheus or the alert evaluator that reads archive health
  metrics.
- Get access to the collector, gateway, SIEM, object-store writer, or security
  pipeline that emits archive health metrics.
- Get access to the local OpenBao audit file or replay source.
- Get security approval before changing archive retention, archive writer
  credentials, dead-letter handling, or audit-device configuration.

> [!WARNING]
> Audit records are security evidence. Do not paste raw audit records into
> tickets, chat, or public logs while you investigate archive delivery.

## Understand the alert metrics

The alert uses reference health metrics from the archive delivery pipeline, not
OpenBao server metrics. Publish these metrics from your collector, archive
gateway, SIEM forwarder, object-store writer, or another controlled component.

| Metric | Type | Meaning |
| ------ | ---- | ------- |
| `openbao_audit_archive_enabled` | Gauge | Set to `1` only when this environment expects archive delivery. Leave absent or set to `0` for local and exempt environments. |
| `openbao_audit_archive_delivery_success` | Gauge | Set to `1` when the archive path is currently healthy and `0` when delivery is degraded. |
| `openbao_audit_archive_last_success_timestamp_seconds` | Gauge | Unix timestamp for the last successful archive delivery or acknowledgement. |
| `openbao_audit_archive_delivery_failures_total` | Counter | Count of failed archive writes, rejected batches, or failed delivery acknowledgements. |
| `openbao_audit_archive_dead_letter_records_total` | Counter | Count of records sent to a dead-letter path instead of the durable archive. |

If your backend exposes different metric names, add recording rules that map
them to these reference names before you enable this alert.
The [audit archive health example](../../examples/audit-archive-health/) shows
both a small exporter and a recording-rule mapping pattern.

## Confirm the degradation

1. Confirm that archive delivery is enabled for this environment.

   ```promql
   max by (cluster, environment, backend, pipeline) (
     openbao_audit_archive_enabled
   )
   ```

   If this query returns no series, check whether the deployment installs an
   `audit_archive` [signal expectation](../../examples/signal-expectations/).
   `OpenBaoAuditArchiveSignalMissing` fires only for an expected archive
   pipeline identity.

2. Check the current archive delivery status.

   ```promql
   min by (cluster, environment, backend, pipeline) (
     openbao_audit_archive_delivery_success
   )
   ```

3. Check how long it has been since the last successful archive delivery.

   ```promql
   time() - max by (cluster, environment, backend, pipeline) (
     openbao_audit_archive_last_success_timestamp_seconds
   )
   ```

4. Check delivery failures over the alert window.

   ```promql
   sum by (cluster, environment, backend, pipeline) (
     increase(openbao_audit_archive_delivery_failures_total[15m])
   )
   ```

5. Check dead-lettered records over the alert window.

   ```promql
   sum by (cluster, environment, backend, pipeline) (
     increase(openbao_audit_archive_dead_letter_records_total[15m])
   )
   ```

6. Record whether the alert is caused by missing health metrics, stale
   delivery, failed delivery, or dead-lettered records.

## Check OpenBao audit health

1. Confirm that OpenBao is still writing audit records.

   ```promql
   sum(
     increase(${p}_audit_log_request_failure[5m])
   )
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

2. Check response audit failures.

   ```promql
   sum(
     increase(${p}_audit_log_response_failure[5m])
   )
   ```

3. If either counter increases, use
   [Audit request and response failures](./audit-request-response-failures.md)
   before you focus on downstream archive delivery.

4. For file audit devices, confirm that the local audit file is still growing
   and that the volume has enough space to buffer records while archive
   delivery is degraded.

   ```shell
   stat <audit_log_file>
   ```

   - `<audit_log_file>`: Full path to the OpenBao audit log file.

   ```shell
   df -h <audit_log_directory>
   ```

   - `<audit_log_directory>`: Directory that contains the audit log file.

## Restore archive delivery

1. Keep the local audit file or replay source intact. Do not delete collector
   positions, buffered files, queues, or dead-letter records until security
   responders approve the recovery plan.

2. Restore collector health when the collector cannot read the local audit file
   or send batches to the archive path.

3. Restore archive backend connectivity when object storage, SIEM ingestion, or
   the archive gateway is unavailable.

4. Rotate or restore archive writer credentials when authentication failures
   cause delivery errors.

5. Fix parser, schema, size, or policy errors when records are rejected or
   dead-lettered.

6. Replay records from the local audit file, collector queue, or dead-letter
   path after the delivery path is healthy.

7. Increase local buffer capacity or collector throughput when backlog grows
   faster than the archive path can drain.

## Verify the result

1. Confirm that the archive path reports healthy delivery.

   ```promql
   min by (cluster, environment, backend, pipeline) (
     openbao_audit_archive_delivery_success
   ) == 1
   ```

2. Confirm that the last successful delivery is recent.

   ```promql
   time() - max by (cluster, environment, backend, pipeline) (
     openbao_audit_archive_last_success_timestamp_seconds
   ) < 300
   ```

3. Confirm that delivery failures and dead-lettered records stop increasing.

   ```promql
   sum by (cluster, environment, backend, pipeline) (
     increase(openbao_audit_archive_delivery_failures_total[15m])
   )
   ```

   ```promql
   sum by (cluster, environment, backend, pipeline) (
     increase(openbao_audit_archive_dead_letter_records_total[15m])
   )
   ```

4. Confirm that the local backlog drains and that replayed records reach the
   archive backend.

5. Confirm that OpenBao audit failure counters are not increasing.

6. Wait for the alert window to pass and confirm that
   `OpenBaoAuditArchiveDegraded` resolves.

## Troubleshooting

### The alert fires in a local or exempt environment

Do not publish `openbao_audit_archive_enabled=1` for environments that do not
require durable archive delivery. Leave the metric absent or publish `0`. Do
not install an `audit_archive` expectation marker for an exempt environment.

### The archive health signal is missing

`OpenBaoAuditArchiveSignalMissing` means that an expectation marker has no
matching `openbao_audit_archive_enabled` series. Check the exporter target,
scrape configuration, and identity labels. The expectation and observed
series must have equal `cluster`, `environment`, `backend`, and `pipeline`
labels.

### Delivery success is healthy but the timestamp is stale

Check whether the archive writer updates the timestamp only on successful
batches. If the environment is quiet, add an audited archive canary or update
the writer to report explicit heartbeat delivery.

### Dead-letter records increase

Inspect dead-letter metadata without exposing raw audit records. Common causes
include parser changes, schema changes, record-size limits, SIEM policy
rejections, and object-store permission errors.

### Loki is healthy but archive delivery is degraded

Treat this as an evidence-retention incident. Loki exploration does not replace
the durable archive path.

### The archive backend is in maintenance

Silence the alert only when the security owner confirms the maintenance window,
local backlog capacity, replay plan, and evidence handling.

## Related pages

- Use [Audit archive reference design](../audit/audit-archive-reference-design.md)
  to validate the archive pattern and failure modes.
- Use the [audit archive health example](../../examples/audit-archive-health/)
  to publish the reference metrics expected by this alert.
- Use [Audit log stream missing](./audit-log-stream-missing.md) when the
  short-term audit exploration stream is missing.
- Use [Audit request and response failures](./audit-request-response-failures.md)
  when OpenBao reports audit-device failures.
- Use [Audit canary missing](./audit-canary-missing.md) when the Loki-backed
  audit canary is absent.

Source: OpenBao documents audit devices and audit blocking behavior in the
[OpenBao audit device documentation][openbao-audit]. The reference archive
metrics and failure model come from the
[Audit archive reference design][audit-archive-reference-design].

[audit-archive-reference-design]: ../audit/audit-archive-reference-design.md
[openbao-audit]: https://openbao.org/docs/audit/
