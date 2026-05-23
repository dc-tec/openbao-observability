# Multiple active OpenBao nodes

Use this runbook when the `OpenBaoMultipleActiveNodes` alert fires because
Prometheus sees more than one active OpenBao node. Treat this as a possible
split-brain incident until you prove that the signal is a scrape artifact.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get OpenBao CLI access to each node that reports active.
- Get access to OpenBao operational logs, platform networking data, and storage
  backend health data.
- Get an incident commander before isolating nodes or making Raft membership
  changes.

> [!WARNING]
> Do not run automated restarts, Raft peer removal, snapshot restore, or storage
> repair until you know whether multiple nodes can accept writes.

## Confirm the active-node count

1. Query the active-node count.

   ```promql
   sum(
     ${p}_core_active
   )
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

2. Identify the reporting targets.

   ```promql
   ${p}_core_active == 1
   ```

3. Check whether duplicate scrape labels, stale series, or federation duplicate
   the same node. Compare the `instance`, `pod`, `job`, and `cluster` labels.

4. Check target freshness.

   ```promql
   timestamp(
     ${p}_core_active == 1
   )
   ```

## Check leader state

1. Query the leader endpoint on each node that reports active.

   ```shell
   curl -fsS http://<openbao_address>/v1/sys/leader
   ```

   - `<openbao_address>`: OpenBao API address for the node being checked,
     including scheme and port.

2. Compare the `is_self`, `leader_address`, and `leader_cluster_address` values
   across nodes.

3. Check whether more than one node accepts writes through the normal service
   endpoint and through direct node endpoints. Use a non-sensitive test path
   that your incident process allows.

4. If only one node reports itself as leader through `/sys/leader`, fix the
   scrape or label problem before changing OpenBao.

## Check cluster and network health

1. Inspect operational logs on the nodes that report active.

   ```shell
   journalctl -u openbao --since <incident_start>
   ```

   - `<incident_start>`: Time shortly before the alert first fired.

2. Look for leader election churn, storage lock errors, Raft messages, network
   partitions, restarts, and clock jumps.

3. If the deployment uses integrated storage, list Raft peers from a reachable
   node.

   ```shell
   bao operator raft list-peers -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable node.

4. Check load balancer, service, and DNS routing. Confirm that active and
   standby services do not select the same pods incorrectly.

## Restore a safe state

1. If the signal is a scrape artifact, fix duplicate scrape targets, stale
   federation, relabeling, or service discovery. Do not restart OpenBao to fix
   a metrics-only problem.

2. If more than one node can accept writes, isolate the affected nodes through
   the incident process. Preserve logs and storage state for investigation.

3. Restore network and storage consistency before restarting nodes.

4. Restart nodes one at a time only after you have confirmed the intended
   leader and storage state.

5. Do not remove peers or restore snapshots unless the incident commander
   approves the action and you have a current backup.

## Verify the result

1. Confirm exactly one active node.

   ```promql
   sum(
     ${p}_core_active
   )
   ```

2. Confirm that only one node reports `is_self: true` from `/v1/sys/leader`.

3. Confirm that clients can complete permitted requests through the normal
   service endpoint.

4. Wait for the alert window to pass and confirm that
   `OpenBaoMultipleActiveNodes` resolves.

## Troubleshooting

### Two Prometheus series point to the same node

Fix relabeling, federation, or duplicate scrape jobs. The alert should only
count each OpenBao node once.

### The alert fires after a restart

Check whether Prometheus retained a stale active series while a new active node
was elected. If the stale series is the only extra active series, fix scrape
freshness or alert deduplication rather than changing OpenBao.

### More than one node accepts writes

Treat the incident as split brain. Isolate nodes through your incident process
and preserve evidence before remediation.

## What's next

- Use [No active OpenBao leader](./no-active-openbao-leader.md) if remediation
  leaves the cluster without an active node.
- Use [OpenBao metrics scrape failing](./openbao-metrics-scrape-failing.md) if
  target freshness or duplicate scrape jobs caused the alert.

Source: OpenBao documents leader status in the
[OpenBao leader API documentation][openbao-leader]. OpenBao documents Raft peer
inspection in the [OpenBao raft command documentation][openbao-raft].

[openbao-leader]: https://openbao.org/api-docs/system/leader/
[openbao-raft]: https://openbao.org/docs/commands/operator/raft/
