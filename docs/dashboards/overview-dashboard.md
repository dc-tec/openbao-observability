# OpenBao overview dashboard

Use this explainer to read the generated OpenBao overview dashboard. It is for
operators who need a fast health view before they move into HA/Raft, audit,
auth, token, lease, log, or secret-engine drilldowns.

## What this dashboard is for

Use the overview dashboard as the first stop during routine checks and
incidents. It combines scrape health, cluster state, request health, HA/Raft
state, runtime pressure, audit-log presence, and operational errors.

The dashboard answers these questions:

- Can Prometheus scrape OpenBao?
- Does the cluster have exactly one active node?
- How many nodes report as unsealed?
- Are regular requests, login requests, or token checks getting slower?
- Does Raft report expected peer and Autopilot health?
- Are audit events and operational errors visible in Loki?

## What this dashboard is not for

Do not use the overview dashboard as the only investigation surface. It is a
triage view, not a forensic record.

Use a more specific dashboard when you need:

- Per-node Raft detail.
- Audit request ID drilldown.
- Auth and identity activity.
- Token and lease lifecycle detail.
- Secret engine and mount activity.
- Full operational log exploration.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | Expected UID | Used for |
| ----------- | ------------ | -------- |
| Prometheus | `prometheus` | OpenBao source metrics and normalized recording rules. |
| Loki | `loki` | Operational logs and audit logs. |

Deploy the generated Prometheus recording rules before you rely on this
dashboard. Most metric panels use normalized `openbao:` rules instead of raw
`vault_*` or `openbao_*` source metrics.

## Scrape profile assumptions

The dashboard works best when the scrape profile matches the question you ask.

| Scrape profile | What works well | Limitation |
| -------------- | --------------- | ---------- |
| Authenticated active-node scrape | Cluster-level health, active state, request latency, audit metrics, and runtime pressure on the active node. | Standby and follower node runtime state is limited. |
| Private all-node scrape | Per-node HA, Raft, Autopilot, runtime, and standby visibility. | Requires stronger network isolation and label review. |
| Local Docker Compose scrape | Reference-stack validation and dashboard development. | Not a production security model. |

The scrape health panel currently queries `up{job="openbao"}`. If your
Prometheus job label is different, update the dashboard contract or add a
compatible recording rule before you treat scrape health as authoritative.

## How to read cluster health

Start with the top row:

| Panel | Healthy interpretation |
| ----- | ---------------------- |
| Scrape health | All expected OpenBao scrape targets return `up=1`. |
| Active nodes | Exactly one active node exists. |
| Unsealed nodes | The expected number of nodes report as unsealed. |
| Audit request failures | The value stays at `0` over the five-minute window. |

Investigate immediately when active nodes is `0` or greater than `1`. A sealed
or unreachable active node changes the meaning of every other panel because
the dashboard can only show what Prometheus and Loki still receive.

## How to read request health

The request-health row shows:

- In-flight requests.
- Non-login request rate.
- Non-login request latency.
- Login request latency.
- Token check latency.

Read these panels together. Higher latency with normal request rate points to
server-side or dependency pressure. Higher request rate with stable latency
often points to load growth. Rising token-check latency can affect many paths
because token checks sit on the request path for authenticated operations.

Do not compare request latency across environments without checking OpenBao
version, storage backend, auth methods, scrape interval, and load profile.

## How to read HA and Raft health

The HA/Raft row shows peer count, Autopilot health, failure tolerance, and
unhealthy Raft nodes. The Autopilot node health panel shows node-level health
by Raft node ID.

Use these panels to decide whether to open the HA/Raft dashboard. The overview
does not replace the dedicated HA/Raft dashboard because it does not show all
Raft timing, contact, and peer-detail signals.

## How to read runtime pressure

Lease count and goroutines are trend panels. Use them to spot changes in
shape, not to declare an incident from one sample.

Lease count can lag real activity because OpenBao high-cardinality usage
gauges update on `usage_gauge_period`. Treat sudden growth as a prompt to
open the token and lease lifecycle dashboard.

## How to read audit and logs

The bottom row shows the recent audit stream and operational log entries that
contain `error`.

Use the audit stream panel as a presence check only. A quiet audit stream does
not prove that audit logging works, because quiet clusters can have no audited
activity. Use the canary-backed audit alert and the audit overview dashboard
when you need audit-pipeline confidence.

Use the operational errors panel to find process-level symptoms. Do not treat
operational logs as audit evidence.

## Known limitations

- The dashboard depends on generated recording rules for most metrics.
- The scrape health panel assumes `job="openbao"`.
- Active-node scraping does not provide complete standby visibility.
- Loki panels depend on `log_stream="openbao.audit"` and
  `log_stream="openbao.operational"`.
- Audit stream presence is not the same as audit canary success.
- Inventory-style gauges can update on `usage_gauge_period` rather than every
  scrape.

## What's next

- Use [OpenBao observability model](../concepts/openbao-observability-model.md)
  to understand the source and derived signals behind the dashboard.
- Use [Metrics, logs, and audit logs](../concepts/metrics-vs-logs-vs-audit-logs.md)
  to choose the right follow-up signal.
- Use [OpenBao audit overview dashboard](./audit-overview.md) when audit
  metrics or audit streams look unhealthy.
- Use [OpenBao audit investigation dashboard](./audit-investigation.md) when
  you need request ID, path, operation, or node drilldown.
- Use [Understand metric prefixes and recording rules](../contracts/metric-prefix.md)
  when a metric panel is empty.

Source: OpenBao documents telemetry behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. OpenBao
documents audit-device behavior and unaudited paths in the
[OpenBao audit device documentation][openbao-audit]. This page describes the
generated dashboard contract in
`contracts/dashboards/openbao-overview.yaml`.

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
