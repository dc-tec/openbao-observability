# Audit request and response failures

Use this runbook when an audit failure alert fires because OpenBao reports
request or response audit logging failures. Treat the alert as both an
availability risk and an evidence integrity risk.

## Before you begin

- Get OpenBao CLI access with permission to list audit devices.
- Get access to operational logs and audit sink health data.
- Get access to file system, syslog, socket, or HTTP sink owners for the
  configured audit devices.
- Get security approval before disabling, replacing, or moving an audit device.

> [!WARNING]
> Do not disable all audit devices to clear the alert. OpenBao audit logs are
> security evidence, and disabling an audit device changes future audit
> coverage.

## Confirm the failure

Use the `cluster`, `kubernetes_namespace`, and `scrape_profile` alert labels to
select the affected OpenBao deployment.

1. Check request audit failures.

   ```promql
   openbao:audit_log_request_failure:increase5m{
     cluster="<cluster>",
     kubernetes_namespace="<kubernetes_namespace>",
     scrape_profile="<scrape_profile>"
   }
   ```

2. Check response audit failures.

   ```promql
   openbao:audit_log_response_failure:increase5m{
     cluster="<cluster>",
     kubernetes_namespace="<kubernetes_namespace>",
     scrape_profile="<scrape_profile>"
   }
   ```

3. Identify when the counter first increased. Use that timestamp for log and
   sink checks.

## Inspect audit devices

1. List enabled audit devices.

   ```shell
   bao audit list -detailed -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable node.

2. Confirm the expected audit devices are present on the cluster.

3. For file audit devices, check disk space and directory permissions.

   ```shell
   df -h <audit_log_directory>
   ```

   - `<audit_log_directory>`: Directory that contains the audit log file.

   ```shell
   ls -ld <audit_log_directory>
   ```

   - `<audit_log_directory>`: Directory that contains the audit log file.

4. For network audit devices, check the sink endpoint, DNS, TLS, packet loss,
   firewall rules, and collector status.

## Inspect OpenBao logs

1. Check OpenBao operational logs around the first failure.

   ```shell
   journalctl -u openbao --since <incident_start>
   ```

   - `<incident_start>`: Time shortly before the failure counter first
     increased.

2. Look for audit device errors, write failures, permission errors, disk
   pressure, sink timeouts, and blocked requests.

3. If requests hang while audit failures increase, treat the incident as a
   blocked audit device. Restore the blocked sink or remove the blocking path
   only through an approved change.

## Restore audit logging

1. Restore at least one reliable audit sink before changing other audit
   devices.

2. Fix file audit failures by restoring disk space, ownership, permissions, or
   the mounted volume.

3. Fix syslog, socket, and HTTP audit failures by restoring the local agent,
   network path, endpoint availability, or TLS trust.

4. If you use declarative audit configuration, update the server configuration
   consistently across all OpenBao nodes and reload or restart through the
   approved deployment process.

5. If you use API-managed audit devices, make any audit device change through
   the approved security change process.

## Verify the result

1. Run a permitted audited request against a non-sensitive path in your
   environment.

2. Confirm that the request and response entries reach the expected audit sink.
   Do not paste raw audit entries into shared systems.

3. Confirm that failure counters stop increasing.

   ```promql
   openbao:audit_log_request_failure:increase5m{
     cluster="<cluster>",
     kubernetes_namespace="<kubernetes_namespace>",
     scrape_profile="<scrape_profile>"
   }
   ```

   ```promql
   openbao:audit_log_response_failure:increase5m{
     cluster="<cluster>",
     kubernetes_namespace="<kubernetes_namespace>",
     scrape_profile="<scrape_profile>"
   }
   ```

4. Confirm that the audit log collector still sends the audit stream to Loki or
   your log backend.

## Troubleshooting

### The alert fires after an audit device change

Confirm that every OpenBao node can write to the configured device. Audit
device configuration is replicated by default, and a path that works on one
node can fail on another node.

### Requests hang during the incident

Investigate blocked audit devices. A network audit sink that accepts a
connection but never completes writes can block OpenBao requests.

### The counter increased once and stopped

Keep the incident open until you identify the failed sink and confirm audit
coverage. A single failed audit write still creates an evidence gap.

## What's next

- Use [Audit log stream missing](./audit-log-stream-missing.md) if metrics
  recover but Loki or the log backend has no audit stream.
- Use [OpenBao metrics scrape failing](./openbao-metrics-scrape-failing.md) if
  audit metrics disappear from Prometheus.

Source: OpenBao documents audit failure and blocking behavior in the
[OpenBao audit device documentation][openbao-audit]. OpenBao documents
configuration-managed audit devices in the
[OpenBao declarative audit documentation][openbao-declarative-audit].

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-declarative-audit]: https://openbao.org/docs/configuration/audit/
