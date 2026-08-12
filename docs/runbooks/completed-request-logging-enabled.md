# Completed request logging enabled

Use this runbook when the `OpenBaoCompletedRequestLoggingObserved` alert fires
because Loki has received OpenBao completed request logs. Completed request
logging is a temporary troubleshooting stream and should not remain enabled
outside an approved window.

## Before you begin

- Get access to Loki or the log backend that evaluates the alert.
- Get access to OpenBao server configuration for all affected nodes.
- Get access to OpenBao deployment and reload procedures.
- Check whether an approved troubleshooting window is active.

> [!WARNING]
> Completed request logs can expose request metadata. Do not treat them as an
> audit-log replacement, and keep access restricted while you investigate.

## Confirm the stream

1. Check completed request log volume.

   ```logql
   sum by (cluster, kubernetes_namespace) (count_over_time({cluster="<cluster>",kubernetes_namespace="<kubernetes_namespace>",log_stream="openbao.completed_requests"}[5m]))
   ```

2. Inspect recent completed request log entries.

   ```logql
   {cluster="<cluster>",kubernetes_namespace="<kubernetes_namespace>",log_stream="openbao.completed_requests"}
   ```

3. Check whether one node or all nodes emit the stream.

   ```logql
   sum by (cluster, kubernetes_namespace, node_id) (
     count_over_time({cluster="<cluster>",kubernetes_namespace="<kubernetes_namespace>",log_stream="openbao.completed_requests"}[5m])
   )
   ```

4. Confirm whether the entries match an approved troubleshooting window.

## Find where logging is enabled

1. Check OpenBao configuration for `log_requests_level`.

   ```shell
   grep -n 'log_requests_level' <openbao_config_file>
   ```

   - `<openbao_config_file>`: OpenBao server configuration file.

2. Check the main OpenBao `log_level`.

   ```shell
   grep -n 'log_level' <openbao_config_file>
   ```

3. Compare configuration across all OpenBao nodes.

4. Check recent deployment changes, configuration reloads, and runtime logging
   changes for the affected node.

## Disable completed request logging

1. If no approved troubleshooting window exists, set completed request logging
   to `off`.

   ```hcl
   log_requests_level = "off"
   ```

2. Apply the configuration through your deployment process.

3. Reload or restart OpenBao according to your operational procedure.

4. Keep retained completed request logs restricted according to your incident
   and retention policy.

## Verify the result

1. Confirm that completed request log volume stops increasing.

   ```logql
   sum by (cluster, kubernetes_namespace) (count_over_time({cluster="<cluster>",kubernetes_namespace="<kubernetes_namespace>",log_stream="openbao.completed_requests"}[5m]))
   ```

2. Confirm that normal operational logs still arrive.

   ```logql
   count_over_time({cluster="<cluster>",kubernetes_namespace="<kubernetes_namespace>",log_stream="openbao.operational"}[5m])
   ```

3. Confirm that audit logs still arrive for the canary request.

   ```logql
   {cluster="<cluster>",kubernetes_namespace="<kubernetes_namespace>",log_stream="openbao.audit"} | json request_path="request.path" | request_path="secret/data/observability/audit-canary"
   ```

4. Wait for the alert window to pass and confirm that
   `OpenBaoCompletedRequestLoggingObserved` resolves.

## Troubleshooting

### The stream is approved

Record the approved troubleshooting window and expected end time. Silence the
alert only for that window.

### Only one node emits completed request logs

Compare that node's configuration, deployment revision, and reload history with
the rest of the cluster.

### The configuration is off but logs continue

Check whether the running process has reloaded the updated configuration. Then
check collector routing to confirm that old files or another source are not
being labeled as `openbao.completed_requests`.

### Logs stop but the alert keeps firing

Check the alert evaluation window and Loki query time range. The alert clears
after the configured `for` period and query window no longer contain completed
request entries.

## What's next

- Use [OpenBao operational logs dashboard](../dashboards/operational-logs.md)
  to inspect completed request stream context.
- Use [Security audit detections](./security-audit-detections.md) when
  completed request logging appears with other privileged activity.
- Use [Understanding OpenBao logs](../logging/understanding-openbao-logs.md)
  to understand operational, completed request, audit, and archive streams.
- Use [Log retention and access control](../logging/retention-and-access-control.md)
  before you change retention or access for completed request logs.
- Use [Debug logging enabled](./debug-logging-enabled.md) when debug or trace
  logging is active at the same time.

Source: OpenBao documents completed request logging in the
[OpenBao completed request logging documentation][openbao-log-requests].

[openbao-log-requests]: https://openbao.org/docs/configuration/log-requests-level/
