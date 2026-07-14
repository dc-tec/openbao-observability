# OpenBao HA/Raft metrics

Use this explainer to understand the HA, Raft, and Autopilot metrics used by
the generated dashboards and alerts. It is for operators who need to connect
Raft concepts to concrete Prometheus series and recording rules.

## Why this matters

HA/Raft metrics show whether OpenBao has stable leadership, enough unsealed
nodes, enough Raft peers, healthy Autopilot state, and follower progress. They
help you detect loss of failure tolerance before the cluster becomes
unavailable.

These metrics are most useful with all-node scraping. Active-node scraping can
show some cluster-level state, but it limits standby and follower visibility.

## Metric groups

The HA/Raft dashboard uses three metric groups:

| Group | Examples | Purpose |
| ----- | -------- | ------- |
| Core HA state | `core_active`, `core_unsealed` | Show active and unsealed node state. |
| Normalized Raft health | `raft_peers`, `autopilot_healthy`, `autopilot_failure_tolerance`, `autopilot_node_healthy` | Show quorum and Autopilot health. |
| Raw Raft internals | `raft_storage_stats_*`, `raft_state_*`, `raft_leader_lastContact`, `raft_replication_*` | Show replication, leadership churn, and peer progress where available. |

## Core HA state

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_core_active` | `openbao:core_active:sum` | Number of nodes that report active. Healthy HA has exactly one. |
| `${p}_core_unsealed` | `openbao:core_unsealed:sum` | Number of nodes that report unsealed. Healthy value depends on expected cluster size. |

`${p}` is the source prefix. Use `vault` for the OpenBao default or `openbao`
when you configure `metrics_prefix = "openbao"`.

Do not use unsealed node count as voter count. The value counts scraped
OpenBao nodes that report unsealed state, which can include voters,
non-voters, and nodes that are catching up.

## Autopilot and peer health

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_raft_peers` | `openbao:raft_peers:max` | Maximum Raft peer count where OpenBao exposes the source metric. |
| `${p}_autopilot_healthy` | `openbao:autopilot_healthy:max` | Whether Autopilot reports the cluster healthy. |
| `${p}_autopilot_failure_tolerance` | `openbao:autopilot_failure_tolerance:max` | Number of voter failures Autopilot reports as tolerable. |
| `${p}_autopilot_node_healthy` | `openbao:autopilot_node_healthy:min` | Per-node Autopilot health by `node_id`. |

The normalized peer rule falls back to counting
`${p}_raft_storage_stats_commit_index` by `peer_id` when `${p}_raft_peers` is
not present. This keeps the dashboard useful across the OpenBao 2.6.0 fixture
and live all-node scrape behavior observed in this repository.

The current fixture validates a topology with three voters plus one non-voter
read replica. It observes `${p}_raft_peers` as `4`, Autopilot failure tolerance
as `1`, and `autopilot_node_healthy` with a `node_id` value for the read
replica.

## Raw Raft internals

Some detailed HA/Raft dashboard panels query raw `vault_raft_*` source metrics
directly. These panels are intentionally more advanced because the current
metric contract does not normalize every Raft detail series.

| Raw metric family | Interpretation |
| ----------------- | -------------- |
| `vault_raft_state_leader` | Leader transition activity over a time window. |
| `vault_raft_state_candidate` | Candidate transition activity over a time window. |
| `vault_raft_storage_stats_commit_index` | Commit progress by peer. |
| `vault_raft_storage_stats_applied_index` | Applied progress by peer. |
| `vault_raft_storage_stats_fsm_pending` | Pending finite-state-machine work by peer. |
| `vault_raft_storage_stats_term` | Current Raft term by peer. |
| `vault_raft_leader_lastContact` | Leader last-contact timer where exposed. |
| `vault_raft_replication_heartbeat` | Replication heartbeat timer by peer. |
| `vault_raft_replication_appendEntries_logs` | Append-entry log activity by peer. |

If your deployment emits `openbao_*` source metrics, update these raw panels or
add additional normalized recording rules before you rely on them.

## Read replicas and non-voters

OpenBao documents Raft non-voters as nodes that receive the data replication
stream without participating in quorum. They can add read scalability, but they
do not increase voter failure tolerance.

Read non-voter health with all-node scraping and Autopilot or peer-state
signals. Keep quorum alerts voter-aware, and keep read-capacity alerts separate
from voter quorum alerts.

The OpenBao 2.6.0 HA/Raft fixture starts one non-voter with
`retry_join_as_non_voter = true`. The fixture verifies that the node can read a
sample KV value, list mounts and auth methods, emit read-replica audit entries,
and appear in Raft peer and Autopilot state as a non-voter.

Treat role-specific production thresholds as environment-specific. The fixture
validates the label shape and basic read behavior, not your production traffic
split or read-capacity target.

## How to interpret the signals

Start with active node count and unsealed node count. If either value is wrong,
other metrics can be stale, partial, or misleading.

Then check Autopilot health and failure tolerance. A cluster can still serve
requests while failure tolerance is already exhausted.

Use commit index, applied index, and pending FSM work to inspect follower
progress. A follower that stops moving with the rest of the cluster needs
investigation.

Use leader and candidate transition metrics as churn signals. One transition
during planned maintenance can be normal. Repeated transitions usually need
network, storage, process, or platform investigation.

## Scrape profile

Use all-node scraping when you need the strongest HA/Raft view. Active-node
scraping is useful for secure cluster-level health, but it cannot fully
describe follower runtime behavior.

The all-node scrape path needs isolation because standby metrics access
depends on unauthenticated metrics access.

## Common mistakes

- Treating `openbao:autopilot_healthy:max` as the only Raft health signal.
- Ignoring failure tolerance because the cluster still serves traffic.
- Expecting raw `vault_raft_*` panels to work on an `openbao_*` source-prefix
  deployment without adaptation.
- Reading active-node scrape output as complete follower visibility.
- Counting all unsealed nodes as Raft voters.
- Treating read-replica capacity alerts as voter quorum alerts.
- Alerting on raw peer IDs without reviewing label cardinality and metadata
  exposure.

## What's next

- Use [OpenBao HA/Raft observability](../concepts/openbao-ha-raft-observability.md)
  for the operational mental model.
- Use [Namespaces and scale observability](../concepts/namespaces-and-scale-observability.md)
  before you add read-replica or namespace-aware HA/Raft panels.
- Use [OpenBao HA/Raft dashboard](../dashboards/ha-raft.md) to read the
  generated dashboard.
- Use [Configure an all-node metrics scrape](./all-node-metrics-scrape.md) for
  the private all-node profile.
- Use [OpenBao Raft and Autopilot health](../runbooks/raft-autopilot-health.md)
  when HA/Raft alerts fire.

Source: OpenBao documents Raft telemetry in the
[OpenBao Raft telemetry documentation][openbao-raft-telemetry]. OpenBao
documents integrated storage, quorum, and Autopilot in the
[OpenBao integrated storage documentation][openbao-integrated-storage]. This
page also reflects the repository metric contract in
`contracts/metrics/openbao-core.yaml`.

[openbao-integrated-storage]: https://openbao.org/docs/internals/integrated-storage/
[openbao-raft-telemetry]: https://openbao.org/docs/internals/telemetry/metrics/raft/
