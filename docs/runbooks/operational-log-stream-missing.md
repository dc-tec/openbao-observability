# Operational log stream missing

Use this runbook when the `OpenBaoOperationalLogStreamMissing` alert fires
because Loki has not received the `openbao.operational` stream for the alert
window. The steps help you separate an OpenBao logging issue from a collector,
Loki, or label-routing issue.

## Before you begin

- Get access to Loki or the log backend that evaluates the alert.
- Get access to OpenBao server logs on the affected nodes.
- Get access to the log collector that tails or receives operational logs.
- Know whether OpenBao writes logs to stdout, journald, or a configured log
  file.

## Confirm the missing stream

1. Query the operational stream count for the alert window.

   ```logql
   count_over_time({log_stream="openbao.operational"}[10m])
   ```

2. Check whether OpenBao logs arrive under a different stream label.

   ```logql
   {log_stream=~"openbao.*"}
   ```

3. If operational logs arrive under the wrong label, restore the collector
   label `log_stream="openbao.operational"`.

## Check OpenBao log output

1. Check whether the OpenBao process still writes operational logs.

   ```shell
   journalctl -u openbao --since <incident_start>
   ```

   - `<incident_start>`: Time shortly before the alert first fired.

2. If OpenBao writes to a configured log file, check the file timestamp and
   size.

   ```shell
   stat <openbao_log_file>
   ```

   - `<openbao_log_file>`: Full path to the OpenBao operational log file.

3. Confirm that `log_level` and `log_format` still match your deployment
   profile.

4. If the process is not writing logs and OpenBao is unhealthy, use the
   relevant availability runbook before changing collector settings.

## Check the collector

1. Check collector health and logs.

   ```shell
   journalctl -u <collector_service> --since <incident_start>
   ```

   - `<collector_service>`: System service name for your log collector.

2. Confirm that the collector still has permission to read the OpenBao log
   source.

3. Confirm that file rotation, container restart, or a path change did not move
   the log source away from the collector target.

4. Confirm that the collector sends to the expected Loki tenant, endpoint, and
   label set.

## Restore ingestion

1. Restore OpenBao logging first when the server no longer writes logs.

2. Restore collector file permissions, positions, endpoint credentials, or Loki
   connectivity when OpenBao writes logs but Loki receives none.

3. Restore the `log_stream="openbao.operational"` label on OpenBao operational
   logs.

4. Restart or reload the collector through your deployment process.

## Verify the result

1. Confirm that Loki receives operational logs.

   ```logql
   count_over_time({log_stream="openbao.operational"}[5m])
   ```

2. Confirm that recent operational warning and error logs are visible.

   ```logql
   {log_stream="openbao.operational"} |~ "\"@level\":\"(warn|error)\""
   ```

3. Wait for the alert window to pass and confirm that
   `OpenBaoOperationalLogStreamMissing` resolves.

## Troubleshooting

### OpenBao logs exist on disk but not in Loki

Check collector path glob patterns, file permissions, positions state, and Loki
write errors. Also check whether log rotation changed the inode that the
collector follows.

### Logs arrive under another label

Restore the contract label. Dashboards and alerts expect
`log_stream="openbao.operational"`.

### No logs are emitted because OpenBao is quiet

Check whether the alert window is too short for your environment. Keep the
stream alert conservative enough to catch collector outages without paging on a
quiet but healthy server.

## Related pages

- Use [Run the Docker Compose stack](../docker-compose.md) to inspect the local
  Alloy, Loki, and OpenBao operational log wiring.
- Use [OpenBao metrics scrape failing](./openbao-metrics-scrape-failing.md) if
  logs and metrics both disappear.

Source: OpenBao documents server logging options in the
[OpenBao configuration documentation][openbao-configuration]. Grafana documents
local file collection in the
[Alloy file source documentation][alloy-file-source].

[alloy-file-source]: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.file/
[openbao-configuration]: https://openbao.org/docs/configuration/
