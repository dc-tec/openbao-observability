# No active OpenBao leader

Use this runbook when `OpenBaoNoActiveNode` or
`OpenBaoCoreActiveSignalMissing` fires. These alerts report no active node or
a missing required active-node signal. The steps help you identify a leader,
scrape, recording-rule, seal, or network problem.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get OpenBao CLI access to at least one reachable node.
- Get access to platform networking, storage, and pod or process health data.
- Confirm whether the deployment uses integrated storage with Raft.

## Confirm the alert signal

1. Query the active-node metric.

   ```promql
   openbao:core_active:sum
   ```

2. Check the raw series to identify which nodes Prometheus still scrapes.

   ```promql
   ${p}_core_active
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

   If this query returns no series while `up{job="openbao"}` is `1`, the
   `OpenBaoCoreActiveSignalMissing` alert fires. Check the recording rule and
   the required raw `core_active` metric.

3. If Prometheus is missing one or more OpenBao targets, switch to
   [OpenBao metrics scrape failing](./openbao-metrics-scrape-failing.md).

## Check node state

1. Query leader status on a reachable node.

   ```shell
   curl -fsS http://<openbao_address>/v1/sys/leader
   ```

   - `<openbao_address>`: OpenBao API address for a reachable node, including
     scheme and port.

2. Check seal status on each node.

   ```shell
   bao status -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for the node being checked.

3. If every reachable node is sealed, switch to
   [OpenBao sealed unexpectedly](./openbao-sealed-unexpectedly.md).

4. Check operational logs for leader election, storage, network, and Raft
   messages.

   ```shell
   journalctl -u openbao --since <incident_start>
   ```

   - `<incident_start>`: Time shortly before the alert first fired.

## Check Raft health

Use these steps when the deployment uses integrated storage.

1. List Raft peers from a reachable node.

   ```shell
   bao operator raft list-peers -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable node.

2. Check Autopilot state when Autopilot is enabled.

   ```shell
   bao operator raft autopilot state -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable node.

3. Check whether the cluster still has quorum. A Raft cluster cannot elect a
   leader without quorum.

4. Check pod, VM, or host reachability between peers on the OpenBao cluster
   address and storage paths.

## Restore leadership

1. Restore sealed nodes, failed pods, failed VMs, or broken network paths.

2. Restore storage backend availability before forcing process restarts.

3. Restart failed OpenBao processes one at a time. Confirm each node rejoins
   before moving to the next node.

4. Do not remove Raft peers, restore snapshots, or rebootstrap a cluster unless
   your incident commander approves the action and you have a current backup.

5. If an active node exists but clients still fail, check load balancer health
   checks and service selectors.

## Verify the result

1. Confirm exactly one active node.

   ```promql
   sum(
     ${p}_core_active
   )
   ```

2. Confirm that the leader endpoint identifies a leader.

   ```shell
   curl -fsS http://<openbao_address>/v1/sys/leader
   ```

   - `<openbao_address>`: OpenBao API address for a reachable node.

3. Confirm that clients can complete a permitted request through the normal
   service endpoint.

4. Wait for the alert window to pass and confirm that `OpenBaoNoActiveNode`
   resolves.

## Troubleshooting

### Metrics show no active node but the API has a leader

Prometheus probably does not scrape the active node. Fix service discovery or
the active node scrape target before changing OpenBao.

If `OpenBaoCoreActiveSignalMissing` fires, Prometheus can scrape OpenBao but
cannot evaluate the normalized active-node signal. Check the recording rule,
the selected metric prefix, and the raw `core_active` metric.

### Nodes are unsealed but no leader is elected

Check quorum, storage, and peer connectivity. For Raft, inspect peer state
before making membership changes.

### More than one node appears active

Switch to [Multiple active OpenBao nodes](./multiple-active-nodes.md). Treat the
incident as possible split brain until you prove the signal is a scrape
artifact.

## What's next

- Use [OpenBao metrics scrape failing](./openbao-metrics-scrape-failing.md) if
  Prometheus is missing the active node.
- Use [OpenBao sealed unexpectedly](./openbao-sealed-unexpectedly.md) if leader
  loss follows a seal event.

Source: OpenBao documents leader status in the
[OpenBao leader API documentation][openbao-leader]. OpenBao documents Raft peer
inspection and Autopilot state in the
[OpenBao raft command documentation][openbao-raft].

[openbao-leader]: https://openbao.org/api-docs/system/leader/
[openbao-raft]: https://openbao.org/docs/commands/operator/raft/
