# OpenBao HA/Raft observability

Use this explainer to understand the signals that describe OpenBao HA and Raft
health. It is for operators who need to reason about leadership, quorum,
Autopilot, replication, and failure tolerance before responding to dashboard
or alert signals.

## Why this matters

OpenBao integrated storage uses Raft to keep cluster state consistent across
servers. A healthy HA cluster is not only a set of running pods or processes.
It needs stable leadership, enough voters for quorum, followers that can keep
up, and storage that can commit and apply new log entries.

HA/Raft observability helps you detect loss of safety margin before the cluster
becomes unavailable.

## Mental model

Read HA/Raft health from four layers:

| Layer | What it tells you |
| ----- | ----------------- |
| OpenBao core state | Whether nodes are active, standby, sealed, or unsealed. |
| Raft quorum state | Whether enough voters exist to commit new log entries. |
| Replication state | Whether followers receive and apply leader log entries. |
| Operational context | Whether logs, restarts, storage, or network events explain metric changes. |

One panel rarely explains the whole incident. Compare state, quorum,
replication, and logs together.

## OpenBao behavior

OpenBao integrated storage uses a consensus protocol based on Raft. The Raft
peer set elects one leader. The leader is the active OpenBao node, and
followers are standby nodes.

Raft needs quorum to commit new log entries. For a peer set of size `n`,
quorum requires a majority. If quorum is unavailable, no new log entries can be
committed and the cluster cannot make progress.

Autopilot manages quorum health by evaluating when nodes are healthy enough to
be voters and by tracking unhealthy or dead nodes. OpenBao documents Autopilot
as enabled by default for integrated storage.

## Signals to observe

| Signal | What to inspect |
| ------ | --------------- |
| Active node count | Exactly one active node exists. |
| Unsealed node count | Expected nodes report as unsealed. |
| Raft peer count | Peer count matches the expected cluster membership. |
| Autopilot health | Autopilot reports whether the Raft cluster is healthy. |
| Failure tolerance | Remaining voter failures before quorum is at risk. |
| Node health | Per-node Autopilot health by Raft node ID. |
| Leader and candidate transitions | Leadership churn and election activity. |
| Commit and applied index | Follower progress and applied state. |
| Last-contact and heartbeat timers | Peer communication latency where exposed. |
| Raft and storage logs | Server-side explanation for metric changes. |

## How to interpret leadership

Healthy OpenBao HA has exactly one active node. Zero active nodes means the
cluster cannot serve normal active-node traffic. More than one active node is a
split-brain symptom and needs immediate response.

Leader and candidate transition counters show churn. A single transition can
happen during planned maintenance. Repeated transitions point to unstable
leadership, network problems, process restarts, or storage latency.

## How to interpret quorum and failure tolerance

Failure tolerance is a safety-margin signal. A cluster with no remaining
failure tolerance can still serve traffic, but the next voter failure can make
the cluster unavailable.

Treat loss of failure tolerance as an operational event. It is often cheaper
to fix voter health before quorum is lost than to recover after write progress
stops.

## How to interpret replication

Commit and applied index panels show Raft progress. Healthy peers tend to move
forward together. A peer that falls behind, stops applying entries, or keeps a
large pending FSM backlog needs investigation.

Replication latency can come from the network, disk, CPU pressure, storage
contention, process stalls, or overloaded followers. Use platform telemetry
and operational logs to identify the cause.

## How to interpret Autopilot

Autopilot health gives a summarized Raft-cluster signal, but it is not the only
HA signal. Combine it with peer count, failure tolerance, node health, leader
transitions, and operational logs.

Recently joined nodes can pass through stabilization before becoming voters.
During that period, a dashboard can show changing peer or health behavior
without an outage.

## Scrape profile implications

HA/Raft observability benefits from all-node scraping. Active-node scraping can
show cluster-level state, but it limits visibility into standby and follower
runtime behavior.

Use the private all-node profile when you need per-node Raft troubleshooting.
Keep the all-node metrics path isolated because standby metrics access expands
the metrics exposure surface.

## Common mistakes

- Treating pod readiness as Raft health.
- Ignoring failure tolerance while the cluster still serves requests.
- Assuming Autopilot health explains every Raft symptom.
- Reading active-node scrape output as complete follower visibility.
- Treating leader churn as harmless without checking storage and network
  context.
- Adding or removing nodes without watching quorum and index progress.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed OpenBao docs behavior | OpenBao documents Raft leaders, followers, quorum, committed entries, Autopilot behavior, and failure tolerance. |
| Observed fixture behavior | The OpenBao 2.5.5 HA fixture exercises active state, unseal state, peer metrics, Autopilot health, Raft storage stats, three voters, and one non-voter read replica. |
| Design decision | This project separates HA/Raft dashboard interpretation from recovery procedures and treats all-node scraping as the richer diagnostics profile. |
| To validate | Production Raft labels, storage latency, network policy, node replacement procedures, and alert thresholds. |

## What's next

- Use [Active-node and all-node observability](./active-node-vs-all-node-observability.md)
  to choose the right scrape profile.
- Use [OpenBao HA/Raft metrics](../metrics/ha-raft-metrics.md) to connect HA
  and Raft concepts to concrete Prometheus series.
- Use [OpenBao HA/Raft dashboard](../dashboards/ha-raft.md) to inspect the
  generated HA/Raft view.
- Use [OpenBao Raft and Autopilot health](../runbooks/raft-autopilot-health.md)
  when HA/Raft alerts fire.
- Use [No active OpenBao leader](../runbooks/no-active-openbao-leader.md) when
  active node count drops to `0`.
- Use [Multiple active OpenBao nodes](../runbooks/multiple-active-nodes.md)
  when active node count is greater than `1`.

Source: OpenBao documents integrated storage, Raft leaders, followers, quorum,
and Autopilot in the
[OpenBao integrated storage documentation][openbao-integrated-storage].
OpenBao documents Raft configuration and Autopilot intervals in the
[OpenBao Raft storage configuration documentation][openbao-raft-storage].
OpenBao documents Raft telemetry in the
[OpenBao Raft telemetry documentation][openbao-raft-telemetry].

[openbao-integrated-storage]: https://openbao.org/docs/internals/integrated-storage/
[openbao-raft-storage]: https://openbao.org/docs/configuration/storage/raft/
[openbao-raft-telemetry]: https://openbao.org/docs/internals/telemetry/metrics/raft/
