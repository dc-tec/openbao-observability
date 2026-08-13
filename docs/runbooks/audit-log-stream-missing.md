# Audit log stream missing

Use this runbook when the `OpenBaoAuditStreamMissing` warning fires because
Loki has not received the `openbao.audit` stream for the alert window. The
steps help you determine whether OpenBao stopped writing audit logs, the
collector stopped reading them, labels changed before ingestion, or the
environment is quiet.

## Before you begin

- Get access to Loki or the log backend that evaluates the alert.
- Get access to OpenBao audit device configuration.
- Get access to the log collector that tails or receives audit logs.
- Get security approval before changing audit device configuration.

> [!WARNING]
> Audit logs can contain sensitive request and response metadata. Do not paste
> raw audit log lines into tickets, chat, or public logs.

## Confirm the missing stream

1. Query the audit stream count for the alert window.

   ```logql
   count_over_time({log_stream="openbao.audit"}[10m])
   ```

2. Check whether the stream exists under a different label.

   ```logql
   {log_stream=~"openbao.*"}
   ```

3. If audit logs arrive under the wrong label, fix the collector labels and
   keep the `log_stream="openbao.audit"` contract stable.

## Check OpenBao audit output

1. List enabled audit devices.

   ```shell
   bao audit list -detailed -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable node.

2. Confirm that at least one expected audit device is enabled.

3. For file audit devices, check whether the audit file is updated.

   ```shell
   stat <audit_log_file>
   ```

   - `<audit_log_file>`: Full path to the audit log file on the OpenBao node.

4. If the audit file is stale, use
   [Audit request and response failures](./audit-request-response-failures.md)
   to restore audit logging.

## Check the collector

1. Check collector health and logs.

   ```shell
   journalctl -u <collector_service> --since <incident_start>
   ```

   - `<collector_service>`: System service name for your log collector.
   - `<incident_start>`: Time shortly before the alert first fired.

2. Confirm that the collector still has permission to read the audit log file or
   receive from the audit sink.

3. Confirm that log rotation did not move the file without reopening it. For
   file audit devices, send OpenBao `SIGHUP` after rotation so OpenBao reopens
   the audit file.

4. Confirm that the collector sends to the expected Loki tenant, endpoint, and
   label set.

## Restore ingestion

1. Restore the audit sink first when OpenBao is not writing audit records.

2. Restore collector file permissions, positions, endpoint credentials, or Loki
   connectivity when OpenBao writes audit records but Loki receives none.

3. Fix label changes by restoring `log_stream="openbao.audit"` on the audit
   stream.

4. Restart or reload the collector through your deployment process.

## Verify the result

1. Run a permitted audited request against a non-sensitive path in your
   environment.

2. Confirm that the audit stream receives new entries.

   ```logql
   count_over_time({log_stream="openbao.audit"}[5m])
   ```

3. Confirm that audit failure counters are not increasing.

   ```promql
   sum(
     increase(${p}_audit_log_request_failure[5m])
   )
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

4. Wait for the alert window to pass and confirm that
   `OpenBaoAuditStreamMissing` resolves.

5. If `OpenBaoAuditCanaryMissing` is also firing, use
   [Audit canary missing](./audit-canary-missing.md) to inspect the scheduled
   canary request.

## Troubleshooting

### Audit logs exist on disk but not in Loki

Check collector permissions, file path glob patterns, positions state, and Loki
write errors. Also check whether log rotation changed the inode that the
collector follows.

### Loki has audit logs under another label

Restore the contract label. Dashboards and alerts expect
`log_stream="openbao.audit"`.

### OpenBao has no audit devices

Treat this as a security incident unless the environment is explicitly exempt.
Enable or restore audit devices through the approved security change process.

## Related pages

- Use [Audit request and response failures](./audit-request-response-failures.md)
  if OpenBao reports audit write failures.
- Use [Audit canary missing](./audit-canary-missing.md) when the scheduled
  audit canary path is absent.
- Use [Run the Docker Compose stack](../docker-compose.md) to inspect the local
  Alloy, Loki, and OpenBao audit stream wiring.

Source: OpenBao documents audit devices and audit log sensitivity in the
[OpenBao audit device documentation][openbao-audit]. OpenBao documents file
audit rotation behavior in the
[OpenBao file audit device documentation][openbao-file-audit].

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-file-audit]: https://openbao.org/docs/audit/file/
