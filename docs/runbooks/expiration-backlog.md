# Expiration backlog and slow requests

Use this runbook when `OpenBaoExpirationBacklogGrowing` or
`OpenBaoExpirationBacklogWithHighLatency` fires, or when expiration jobs
accumulate while authentication and writes slow down.

## Before you begin

- Get access to the token and lease lifecycle and runtime and storage dashboards.
- Get read access to metrics, operational logs, and restricted audit logs.
- Identify the active node, scrape profile, affected consumers, and recent changes.
- Establish the incident's requirements for availability and valid credentials.

The reference high-latency alert uses a five-minute mean above 1000 milliseconds
for 15 minutes. Set this limit and the alert duration to match your service
objectives in `contracts/alerts/warning.yaml`, then regenerate the rules. It is
a starting value, not an OpenBao capacity limit. The existing baseline-relative
storage alert can become less sensitive as its rolling baseline rises.

## Confirm the backlog

1. Select one cluster and scrape profile in the dashboards. Use the same UTC
   interval for application measurements, metrics, and logs.

2. Check the active node's waiting jobs and net backlog change.

   ```promql
   openbao:expire_job_manager_total_jobs:max
   openbao:expire_job_manager_backlog:net_change_per_second5m
   openbao:expire_job_manager_queue_length:max
   ```

   A positive slope means net growth; a negative slope means net draining.
   `total_jobs` decreases on dispatch, before a job finishes. The slope is not
   successful revocation throughput. Ignore slope changes around process
   restarts and leadership changes. Normalization excludes gauges on standby
   nodes, but a leadership transition still changes the observed queue.

3. Check the time spent waiting by jobs that started execution.

   ```promql
   openbao:expire_lease_expiration_time_in_queue:avg5m
   ```

   This mean is in milliseconds. It does not measure the oldest waiting job or
   predict remaining drain time. An old job starting now can produce a long
   wait observation even while recovery progresses.

4. Compare waiting jobs with tracked leases and irrevocable leases.

   ```promql
   openbao:expire_num_leases:max
   openbao:expire_num_irrevocable_leases:max
   ```

   These are different inventories. Subtracting waiting jobs from tracked
   leases does not count valid or unexpired credentials. In OpenBao 2.5.5,
   the 256000-lease warning is a logging threshold, not a performance switch.

Expiration queue gauges update on job activity. A missing series is unknown
state, not proof of an empty queue. Check scrape health and the active node's
metrics directly before declaring recovery. Zero waiting jobs can coexist with
jobs still executing.

`queue_id` normally identifies a mount accessor in the expiration scheduler.
The `mount-accessor-not-found` queue groups jobs with an unresolved accessor.
Queue IDs are not token accessors, consumer identities, or prefixes accepted
by a revocation command. Resolve a queue through mount metadata and restricted
audit records.
Review mount churn and label exposure before enabling this advanced drilldown.

## Locate the limiting operation

1. Read the runtime and storage dashboard's write-path panels together.

   | Signal | Interpretation |
   | ------ | -------------- |
   | Raft backend PUT and DELETE means | Include storage permits and waiting for Raft application. |
   | Quorum commit mean | Measures Raft commitment, not full request completion. |
   | Local batch apply mean | Measures local finite-state-machine database application. |
   | FSM busy wall time | A fraction of thread wall time, not CPU utilization. |
   | Pending Raft apply work | Local application work waiting by target and peer. |
   | Logs per applied batch | Batch size, not an operation rate or latency. |

   These timers cover different operation boundaries. Do not add their means
   or subtract them to calculate exact lock-wait time. Preserve node identity;
   a cluster mean can conceal a busy leader among idle followers.

2. Measure a representative login, read, and write through the affected
   application path. Exclude password-entry time. The generic login-handler
   metric includes multiple auth methods and helper requests; it is not an
   LDAP-specific measurement.

3. Correlate issuance, lease registration, renewal, wrapping, and cleanup
   activity with the start of degradation. Use audit records for caller
   attribution. Check effective token lifetimes, use limits, and reuse rather
   than relying only on configured caching. Renewal activity alone does not
   establish new token creation.

4. Confirm which device backs the configured Raft data path before inspecting
   disk metrics. Compare device latency and platform throttling signals in
   the same interval. Provisioned bandwidth or IOPS can remain available while
   serialized synchronous database commits limit application throughput.
   NVMe utilization alone is not a capacity percentage; zero block-flush
   counters do not establish an absence of application `fdatasync` calls.

5. If authorized profiling access is available, capture a bounded debug bundle
   from the active node during a known slow request.

   ```shell
   bao debug -duration=30s -interval=10s -target=pprof -target=metrics
   ```

   Treat the bundle as restricted diagnostic data. Record the node, version,
   workload, and request time alongside it. Distinguish a capture deadline
   from a production request timeout. Compare with a healthy capture under
   the same workload when possible.

## Select a mitigation

1. Reduce unnecessary token replacement, repeated authentication, or retry
   bursts in the identified workload. Apply changes through the workload's
   incident procedure and verify the properties of newly issued tokens.

2. If storage is already busy, evaluate cleanup concurrency before increasing
   it. In OpenBao 2.5.5, `BAO_LEASE_REVOCATION_WORKERS` is read when the
   expiration manager is constructed; changing the service environment
   requires a process restart. A lower value might improve foreground latency
   and prolong cleanup. Its benefit is not guaranteed. A higher value can add
   contention. Test changes with rollback criteria and HA impact accounted for.

3. Preserve valid credentials when the incident requires it. Prefix revocation
   still performs revocation work and can invalidate active credentials. Force
   revocation can leave downstream credentials behind. Tidy and an unchanged
   restart are not queue-purge operations. Do not delete raw lease storage or
   disable durable synchronization to remove the backlog.

4. Check release fixes against the observed stacks. OpenBao 2.7.0 includes
   [PR #3844](https://github.com/openbao/openbao/pull/3844), which removes an
   unnecessary second database transaction during Raft batch application.
   Qualify the upgrade, authentication plugins, and HA behavior before rollout.
   Transaction count alone does not predict application speedup.

## Verify the result

1. Confirm waiting jobs remain low under the normal workload. If the series
   disappears, verify telemetry instead of interpreting absence as zero.
2. Confirm representative authentication, reads, and writes meet the service
   objective. A draining queue can continue to keep cleanup workers busy.
3. Confirm downstream worker queues and affected regions recover. Separate
   client-side waiting and network delay from OpenBao's processing time.
4. Observe normal peak traffic and credential expiration cycles. Record the
   recovery time and a healthy baseline for comparison.

## Related pages

- [Token and lease metrics](../metrics/token-and-lease-metrics.md)
- [HA/Raft metrics](../metrics/ha-raft-metrics.md)
- [Runtime and storage warnings](./runtime-storage-warnings.md)
- [Irrevocable leases present](./irrevocable-leases.md)

Source: queue semantics follow the OpenBao
[job manager](https://github.com/openbao/openbao/blob/v2.7.2/internal/helper/fairshare/jobmanager.go)
and [expiration manager](https://github.com/openbao/openbao/blob/v2.7.2/internal/vault/expiration.go).
Storage timing follows the
[Raft backend](https://github.com/openbao/openbao/blob/v2.7.2/internal/physical/raft/raft.go).
