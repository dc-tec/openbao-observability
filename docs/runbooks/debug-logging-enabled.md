# Debug logging enabled

Use this runbook when the `OpenBaoDebugTraceLoggingEnabled` alert fires because
OpenBao operational logs include debug or trace entries. Debug and trace logs
can increase log cost and expose sensitive operational metadata.

## Before you begin

- Get access to Loki or the log backend that evaluates the alert.
- Get access to OpenBao server configuration.
- Get permission to inspect or change OpenBao runtime logger levels.
- Check whether an approved troubleshooting window is active.

## Confirm the logging level

1. Query debug and trace log volume.

   ```logql
   sum(
     count_over_time(
       {log_stream="openbao.operational"} |~ "\"@level\":\"(debug|trace)\"" [5m]
     )
   )
   ```

2. Inspect recent debug and trace log entries.

   ```logql
   {log_stream="openbao.operational"} |~ "\"@level\":\"(debug|trace)\""
   ```

3. Confirm whether the logs come from all OpenBao nodes or one node.

   ```logql
   sum by (node_id) (
     count_over_time(
       {log_stream="openbao.operational"} |~ "\"@level\":\"(debug|trace)\"" [5m]
     )
   )
   ```

## Find the source

1. Check the configured server log level.

   ```shell
   grep -n 'log_level' <openbao_config_file>
   ```

   - `<openbao_config_file>`: OpenBao server configuration file.

2. Check whether runtime logger levels were changed through the OpenBao logger
   API.

   ```shell
   bao read -address=<openbao_address> sys/loggers
   ```

3. Check change records for an approved troubleshooting window.

4. Check whether the collector is ingesting a temporary completed-request or
   debug stream under `openbao.operational`.

## Restore normal logging

1. If the server configuration sets `log_level = "debug"` or
   `log_level = "trace"`, change it back to your approved level, usually
   `info`.

2. If runtime logger levels were changed through the API, revert the changed
   logger level or reload the approved configuration.

3. Reload or restart OpenBao through your deployment process.

4. Keep any retained debug or trace logs restricted according to your incident
   and retention policy.

## Verify the result

1. Confirm that debug and trace logs stop increasing.

   ```logql
   sum(
     count_over_time(
       {log_stream="openbao.operational"} |~ "\"@level\":\"(debug|trace)\"" [5m]
     )
   )
   ```

2. Confirm that normal operational logs still arrive.

   ```logql
   count_over_time({log_stream="openbao.operational"}[5m])
   ```

3. Wait for the alert window to pass and confirm that
   `OpenBaoDebugTraceLoggingEnabled` resolves.

## Troubleshooting

### The API shows normal levels but debug logs continue

Check static server configuration and container arguments. Runtime logger
changes are not the only way to enable debug or trace logging.

### Only one node emits debug logs

Compare that node's configuration, runtime logger state, and deployment
revision with the other OpenBao nodes.

### Debug logging is approved

Add a maintenance annotation or silence in your alerting system for the
approved troubleshooting window. Do not change the dashboard or alert contract
to hide the signal.

## What's next

- Use [Operational log stream missing](./operational-log-stream-missing.md) if
  operational logs disappear after a logging change.
- Use [Run the Docker Compose stack](../docker-compose.md) to inspect the local
  operational log stream.

Source: OpenBao documents server logging options in the
[OpenBao configuration documentation][openbao-configuration]. OpenBao documents
runtime logger inspection and changes in the
[OpenBao loggers API documentation][openbao-loggers].

[openbao-configuration]: https://openbao.org/docs/configuration/
[openbao-loggers]: https://openbao.org/api-docs/system/loggers/
