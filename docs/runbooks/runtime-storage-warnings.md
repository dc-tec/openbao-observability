# Runtime and storage warnings

Use this runbook when a runtime or storage warning fires for OpenBao. These
alerts point to storage barrier, cache, runtime memory, or mount table changes
that need correlation with request latency, logs, and recent deployment
activity.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get access to the OpenBao runtime and storage dashboard.
- Get access to OpenBao operational logs and audit logs.
- Get access to deployment, platform, and change history for the affected
  OpenBao cluster.

## Confirm the warning

1. Check which warning fired.

   ```promql
   ALERTS{alertname=~"OpenBao(StorageBarrierLatencyElevated|StorageCacheHitRatioLow|RuntimeMemoryGrowth|MountTableGrowth)", alertstate="firing"}
   ```

2. Open the `OpenBao runtime and storage` dashboard.

3. Compare the warning time with request latency and token-check latency.

   ```promql
   openbao:core_handle_request:avg5m
   openbao:core_check_token:avg5m
   ```

4. Check whether Raft, audit, token, or lease alerts fired at the same time.

   ```promql
   ALERTS{alertstate="firing", alertname=~"OpenBao.*"}
   ```

## Investigate barrier latency

1. Check barrier operation latency.

   ```promql
   openbao:barrier_get:avg5m
   openbao:barrier_put:avg5m
   openbao:barrier_list:avg5m
   openbao:barrier_delete:avg5m
   ```

2. Check barrier operation rates.

   ```promql
   openbao:barrier_get:rate5m
   openbao:barrier_put:rate5m
   openbao:barrier_list:rate5m
   openbao:barrier_delete:rate5m
   ```

3. Check HA/Raft health before you tune clients or storage.

   ```promql
   openbao:autopilot_healthy:max
   openbao:autopilot_node_healthy:min
   openbao:raft_peers:max
   ```

4. Search operational logs for storage and Raft symptoms.

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(storage|barrier|raft|autopilot|timeout|slow|error|failed)"
   ```

## Investigate cache changes

1. Check cache hit ratio and traffic volume together.

   ```promql
   openbao:cache_hit_ratio:ratio5m
   openbao:cache_hit:rate5m
   openbao:cache_miss:rate5m
   ```

2. Check whether request mix changed around the same time.

   ```logql
   sum by (request_path) (
     count_over_time(
       {log_stream="openbao.audit"} | json request_path="request.path" [15m]
     )
   )
   ```

3. Check for new mounts, remounts, or policy changes in audit logs.

   ```logql
   {log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"sys/(mounts|auth).*"
   ```

## Investigate runtime memory

1. Check runtime memory and heap signals.

   ```promql
   openbao:runtime_alloc_bytes:max
   openbao:runtime_sys_bytes:max
   openbao:runtime_heap_objects:max
   ```

2. Check garbage collection signals.

   ```promql
   openbao:runtime_gc_pause_ns:avg5m
   openbao:runtime_total_gc_runs:max
   openbao:runtime_total_gc_pause_ns:max
   ```

3. Compare runtime growth with request throughput and operational logs.

   ```promql
   openbao:core_handle_request:rate5m
   ```

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(runtime|memory|gc|heap|allocation|error|failed)"
   ```

4. Check container or host memory metrics in your platform monitoring system.
   OpenBao runtime memory does not show the full container or node memory
   picture.

## Investigate mount table growth

1. Check mount table entries by bounded labels.

   ```promql
   openbao:core_mount_table_num_entries:max
   ```

2. Check mount table size.

   ```promql
   openbao:core_mount_table_size:max
   ```

3. Inspect recent mount and auth method changes.

   ```shell
   bao secrets list -detailed -address=<openbao_address>
   bao auth list -detailed -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable active node.

4. Check audit logs for mount and auth configuration changes.

   ```logql
   {log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"sys/(mounts|auth).*"
   ```

## Restore the baseline

1. If latency correlates with Raft health, use the HA/Raft runbook before you
   change clients or secret engines.

2. If cache ratio changed because of a planned workload or mount change, record
   the expected baseline and alert duration in your change record.

3. If runtime memory grows with request latency or platform memory pressure,
   reduce the triggering workload or roll back the related deployment change.

4. If mount table growth is unplanned, identify the change owner before you
   disable mounts, auth methods, plugins, or policies.

5. If operational logs show storage backend errors, restore the storage backend
   or platform dependency first.

## Verify the result

1. Confirm that request latency returns toward baseline.

   ```promql
   openbao:core_handle_request:avg5m
   ```

2. Confirm that the warning-specific signal returns toward baseline.

   ```promql
   openbao:barrier_get:avg5m
   openbao:cache_hit_ratio:ratio5m
   openbao:runtime_sys_bytes:max
   sum(openbao:core_mount_table_num_entries:max)
   ```

3. Confirm that operational logs no longer show correlated storage, cache,
   runtime, or mount errors.

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(storage|barrier|cache|runtime|memory|mount)" |~ "(?i)(error|failed|timeout)"
   ```

4. Wait for the alert window to pass and confirm that the warning resolves.

## Troubleshooting

### The alert fires after a planned change

Record the new baseline and expected duration. Silence the alert only for the
approved change window.

### Metrics are empty

Confirm that generated recording rules are loaded and that Prometheus scrapes
OpenBao source metrics with the expected `vault_*` or `openbao_*` prefix.

### Runtime memory grows but platform memory is stable

Check whether Go runtime memory has stabilized at a higher allocation target.
Use platform memory, request latency, and GC pause together before you treat
the warning as a leak.

### Cache hit ratio is low in a quiet cluster

The alert requires active cache traffic. If it fires in a quiet cluster, check
recording rule freshness and scrape timestamps.

## What's next

- Use [OpenBao runtime and storage dashboard](../dashboards/runtime-storage.md)
  to inspect the correlated signals.
- Use [OpenBao HA/Raft dashboard](../dashboards/ha-raft.md) when storage
  symptoms correlate with Raft health.
- Use [OpenBao secret engines and mounts dashboard](../dashboards/secret-engines-mounts.md)
  when mount table growth needs audit context.
- Use [OpenBao Raft and Autopilot health](./raft-autopilot-health.md) if a
  Raft alert fires with the storage warning.

Source: OpenBao documents telemetry metric behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. OpenBao
documents runtime, barrier, cache, and mount table metric names in the
[OpenBao telemetry metrics reference][openbao-telemetry-all].

[openbao-telemetry-all]: https://openbao.org/docs/internals/telemetry/metrics/all/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
