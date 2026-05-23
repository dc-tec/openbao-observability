# OpenBao Raft and Autopilot health

Use this runbook when a Raft or Autopilot critical alert fires for an OpenBao
cluster that uses integrated storage. The steps help you identify whether the
cluster lost a peer, lost failure tolerance, or has an unhealthy Raft node.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get OpenBao CLI access to a reachable node.
- Get access to OpenBao operational logs and platform health data.
- Confirm that the deployment uses integrated storage with a three-node or
  larger HA baseline.

> [!WARNING]
> Do not remove Raft peers, restore snapshots, or rewrite `raft/peers.json`
> unless your incident commander approves the action and you have a current
> backup.

## Confirm the alert signal

1. Check the Raft peer count.

   ```promql
   openbao:raft_peers:max
   ```

2. Check cluster-level Autopilot health.

   ```promql
   openbao:autopilot_healthy:max
   ```

3. Check current Autopilot failure tolerance.

   ```promql
   openbao:autopilot_failure_tolerance:max
   ```

4. Identify unhealthy nodes when `OpenBaoAutopilotNodeUnhealthy` fires.

   ```promql
   openbao:autopilot_node_healthy:min == 0
   ```

5. Compare the alerting series with target freshness.

   ```promql
   timestamp(
     openbao:autopilot_node_healthy:min
   )
   ```

## Inspect Raft state

1. List Raft peers from a reachable node.

   ```shell
   bao operator raft list-peers -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable node, including
     scheme and port.

2. Check Autopilot state.

   ```shell
   bao operator raft autopilot state -address=<openbao_address>
   ```

3. Compare the command output with the metrics:

   - `OpenBaoRaftPeerCountLow`: fewer than three peers are visible.
   - `OpenBaoAutopilotUnhealthy`: Autopilot reports the cluster as unhealthy.
   - `OpenBaoAutopilotFailureToleranceLost`: Autopilot reports no tolerated
     voter failure while the cluster still has at least three peers.
   - `OpenBaoAutopilotNodeUnhealthy`: one or more known nodes are unhealthy.

4. Check whether the affected node is sealed, stopped, partitioned, or unable
   to access its Raft storage path.

5. Check OpenBao logs for leader election churn, failed Raft append entries,
   snapshot transfer failures, storage errors, network failures, and clock
   jumps.

   ```shell
   journalctl -u openbao --since <incident_start>
   ```

   - `<incident_start>`: Time shortly before the alert first fired.

## Restore Raft health

1. Restore failed OpenBao processes, pods, VMs, or hosts before changing Raft
   membership.

2. Restore network connectivity on both the OpenBao API address and cluster
   address.

3. Restore the affected node's persistent storage before restarting OpenBao.

4. Restart failed nodes one at a time. Confirm that each node rejoins and
   becomes healthy before restarting another node.

5. If a peer is permanently lost, follow your recovery procedure for replacing
   a Raft node. Confirm backup availability and operator approval before
   removing the old peer.

## Verify the result

1. Confirm the peer count is back at the expected baseline.

   ```promql
   openbao:raft_peers:max
   ```

2. Confirm Autopilot reports a healthy cluster.

   ```promql
   openbao:autopilot_healthy:max
   ```

3. Confirm Autopilot has failure tolerance for at least one voter.

   ```promql
   openbao:autopilot_failure_tolerance:max
   ```

4. Confirm every known node reports healthy.

   ```promql
   openbao:autopilot_node_healthy:min
   ```

5. Confirm the CLI output matches the metrics.

   ```shell
   bao operator raft list-peers -address=<openbao_address>
   bao operator raft autopilot state -address=<openbao_address>
   ```

6. Wait for the alert window to pass and confirm that the Raft or Autopilot
   alert resolves.

## Troubleshooting

### Metrics show stale peer state

Check Prometheus target freshness, federation, relabeling, and scrape
deduplication before changing OpenBao. Stale metrics can make a recovered node
look unhealthy after Autopilot has already converged.

### The raw peer gauge is missing

Use `openbao:raft_peers:max` for alert triage. The recording rule uses
the raw peer gauge, such as `vault_raft_peers` or `openbao_raft_peers`, when
OpenBao exposes it. It falls back to counting
`vault_raft_storage_stats_commit_index` or
`openbao_raft_storage_stats_commit_index` by `peer_id` when the raw peer gauge
is absent from the current scrape.

### Autopilot is healthy but failure tolerance is zero

The cluster can be healthy and still have no tolerated voter failure. Restore a
missing voter or add a replacement voter before starting planned maintenance.

### A node repeatedly rejoins as unhealthy

Check the node's cluster address, persistent storage, clock synchronization,
resource pressure, and snapshot transfer logs. A node that can reach the API
address can still fail Raft replication on the cluster address.

## What's next

- Use [No active OpenBao leader](./no-active-openbao-leader.md) if Raft health
  issues remove the active node.
- Use [OpenBao sealed unexpectedly](./openbao-sealed-unexpectedly.md) if a peer
  is sealed.
- Use [OpenBao metrics scrape failing](./openbao-metrics-scrape-failing.md) if
  Prometheus no longer scrapes one or more peers.

Source: OpenBao documents Raft peer inspection and Autopilot state in the
[OpenBao raft command documentation][openbao-raft]. OpenBao documents integrated
storage and Raft peer management in the
[OpenBao integrated storage documentation][openbao-integrated-storage].

[openbao-integrated-storage]: https://openbao.org/docs/concepts/integrated-storage/
[openbao-raft]: https://openbao.org/docs/commands/operator/raft/
