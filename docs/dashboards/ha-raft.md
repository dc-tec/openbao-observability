# OpenBao HA/Raft dashboard

Use this explainer to read the generated OpenBao HA/Raft dashboard. It is for
operators who need to inspect active-node state, Raft peer health, Autopilot
signals, replication behavior, and Raft-related operational logs.

## What this dashboard is for

Use the HA/Raft dashboard when the overview dashboard shows leader, seal,
Autopilot, Raft peer, or storage symptoms.

The dashboard answers these questions:

- How many Raft peers does OpenBao report?
- Does Autopilot report the Raft cluster as healthy?
- How many voter failures does Autopilot report as tolerable?
- Which Raft nodes look unhealthy?
- Did active-node or candidate transitions increase?
- Are commit and applied indexes moving together?
- Do operational logs mention Raft, storage, or Autopilot problems?

## What this dashboard is not for

Do not use this dashboard as a Raft tuning guide. It shows symptoms and
directional health signals. Use OpenBao configuration, server logs, and
operational change history to explain why the signals changed.

Do not use this dashboard as a backup or recovery workflow. Use the relevant
OpenBao integrated storage and snapshot procedures for those tasks.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | Expected UID | Used for |
| ----------- | ------------ | -------- |
| Prometheus | `prometheus` | OpenBao Raft, Autopilot, active, and seal metrics. |
| Loki | `loki` | Raft, storage, and Autopilot operational logs. |

The dashboard works best with a private all-node scrape. Active-node scraping
can show cluster-level health, but it limits standby and follower visibility.

## Metric-prefix assumptions

Some panels use normalized `openbao:` recording rules. Other detailed Raft
panels still query raw `vault_raft_*` source metrics because OpenBao exposes
Raft internals as source metrics and the current contract has not normalized
every detail series.

If your deployment emits `openbao_*` source metrics, adapt the dashboard
contract or add compatible recording rules before you treat the raw Raft panels
as complete.

## How to read quorum health

Start with the first row:

| Panel | Healthy interpretation |
| ----- | ---------------------- |
| Raft peers | The value matches the expected cluster peer count. |
| Autopilot health | The value reports healthy. |
| Failure tolerance | The value matches the number of voter failures your cluster can tolerate. |
| Unhealthy Raft nodes | The value stays at `0`. |

Loss of failure tolerance changes incident severity. A cluster can remain
available while losing the ability to tolerate another voter failure.

## How to read active and transition signals

Active nodes must remain exactly `1`. Unsealed nodes must match the expected
running cluster size.

Leader and candidate transitions are churn signals. A nonzero value is not
automatically an outage, but repeated transitions in a short window usually
mean the cluster is losing stable leadership or detecting leader failures.

Compare transition spikes with operational logs, node restarts, network
events, and storage latency.

## How to read replication and storage panels

Commit index and applied index show Raft progress by peer. Healthy peers tend
to move forward together. A peer that stops progressing, falls behind, or keeps
large pending FSM work needs investigation.

Leader last-contact and replication heartbeat timers are latency signals where
OpenBao exposes them. Rising values can point to network, disk, CPU, or peer
health problems.

Append entries logs show Raft append-entry activity by peer. Use the panel as
context for replication behavior, not as a standalone health check.

## How to read Raft and storage logs

The bottom panel filters operational logs for Raft, storage, and Autopilot
terms. Use it to correlate metric changes with server-side messages.

Operational logs are troubleshooting context. They are not audit records.

## Common mistakes

- Reading active-node scrape results as complete follower visibility.
- Treating Autopilot health as the only Raft signal.
- Ignoring loss of failure tolerance while the cluster still serves traffic.
- Treating one leader transition as an outage without checking the surrounding
  time window.
- Forgetting that detailed Raft panels currently query raw `vault_raft_*`
  source metrics.

## Known limitations

- The dashboard needs all-node scraping for the strongest per-node view.
- Some detailed Raft panels use raw `vault_raft_*` metric names.
- Missing raw Raft internals can make detail panels empty while normalized
  health panels still work.
- Operational logs depend on `log_stream="openbao.operational"`.
- The dashboard does not replace OpenBao storage recovery procedures.

## What's next

- Use [Configure an all-node metrics scrape](../metrics/all-node-metrics-scrape.md)
  when you need standby and follower visibility.
- Use [OpenBao operational logs dashboard](./operational-logs.md) when the log
  panels need deeper process context.
- Use [OpenBao Raft and Autopilot health](../runbooks/raft-autopilot-health.md)
  when Raft or Autopilot alerts fire.
- Use [No active OpenBao leader](../runbooks/no-active-openbao-leader.md) when
  the cluster has no active node.
- Use [Multiple active OpenBao nodes](../runbooks/multiple-active-nodes.md)
  when more than one node reports active.

Source: OpenBao documents integrated storage, Raft, and Autopilot behavior in
the [OpenBao integrated storage documentation][openbao-integrated-storage] and
[OpenBao Raft storage configuration documentation][openbao-raft-storage]. This
page describes the generated dashboard contract in
`contracts/dashboards/openbao-ha-raft.yaml`.

[openbao-integrated-storage]: https://openbao.org/docs/internals/integrated-storage/
[openbao-raft-storage]: https://openbao.org/docs/configuration/storage/raft/
