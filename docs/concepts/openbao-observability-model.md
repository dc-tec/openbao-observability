# OpenBao observability model

Use this explainer to understand how the reference architecture turns OpenBao
metrics, logs, audit logs, and platform state into dashboards, alerts, and
runbooks. It is for operators who need to reason about what each signal can
prove before they depend on it.

## Why this matters

OpenBao observability is not one dashboard or one scrape job. You operate a
security-critical service by combining source signals, derived signals, and
response guidance.

Each signal answers a different class of question. Metrics show health, rate,
latency, saturation, and trends. Operational logs show server behavior. Audit
logs show API activity that passed through the audit system. Platform signals
show whether the runtime environment is healthy enough for OpenBao to operate.

## Mental model

Read the reference architecture as a signal pipeline.

```text
OpenBao deployment
  -> source signals
  -> collectors and scrapes
  -> recording rules and log queries
  -> dashboards, alerts, and runbooks
  -> operator decisions
```

The project keeps source signals and derived signals separate. You validate
source metrics and log streams first. You then use recording rules, alert
rules, and generated dashboards to keep operator-facing views stable.

## Source signals

| Signal | Use it for | Do not use it for |
| ------ | ---------- | ----------------- |
| Metrics | Health, request latency, HA state, Raft state, runtime pressure, token counts, lease counts, and audit-device failures. | Full request reconstruction or compliance evidence. |
| Operational logs | Startup, shutdown, listener, storage, Raft, plugin, and process troubleshooting. | Security audit trails or durable evidence by themselves. |
| Audit logs | API request and response security records for audited paths. | General application debugging or platform troubleshooting. |
| Platform signals | Pod, container, host, volume, network, and service discovery state. | OpenBao internal semantics such as seal state or audit-device health. |

Dashboards combine these signals for interpretation. They do not become the
source of truth themselves.

## Derived signals

The repository uses contracts and generators so dashboards and alerts depend on
explicit signal definitions.

| Derived signal | Purpose |
| -------------- | ------- |
| Recording rules | Normalize OpenBao source metrics into stable `openbao:` series. |
| Alert rules | Turn critical and warning conditions into named operational events. |
| Dashboard contracts | Define the panels, queries, variables, and data-source expectations for generated Grafana dashboards. |
| Runbooks | Describe how you respond when an alert fires. |

This separation matters because OpenBao deployments can emit either `vault_*`
or `openbao_*` Prometheus metrics, and live label sets vary by scrape profile.
The dashboards consume normalized rules where possible, while validation still
checks the raw source signals.

## OpenBao behavior

OpenBao telemetry exposes counters, gauges, and summaries. High-cardinality
usage gauges, such as token, entity, and secret counts, update on the
`usage_gauge_period` interval.

OpenBao audit devices write request and response entries for audited API paths.
Some system paths bypass the audit system, including health, seal, unseal,
leader, and initialization paths. `sys/metrics`, `sys/pprof/*`, and
`sys/in-flight-req` also bypass audit when listener configuration allows
unauthenticated access.

OpenBao completed request logging is separate from audit devices. It is
disabled by default and depends on both `log_requests_level` and the main
OpenBao `log_level`.

## Design recommendations

Use metrics for fast health and trend detection. Alert on source metrics or
normalized recording rules when the condition has clear operational meaning.

Use operational logs for server behavior and troubleshooting. Keep them
separate from audit logs so operational dashboards do not require broad access
to security records.

Use audit logs for security investigation and canary validation. Keep sensitive
fields out of Loki labels and parse them at query time in restricted dashboards.

Use platform signals to explain why OpenBao cannot serve traffic, write audit
files, or expose metrics. Do not replace OpenBao metrics with platform health
checks.

Use all-node metrics scraping when you need standby, follower, or per-node Raft
visibility. Use authenticated active-node scraping as the secure baseline when
you only need cluster-level health.

## Common mistakes

- Treating an imported dashboard as the observability design.
- Treating Loki as a compliance archive without an approved retention and
  access-control design.
- Labeling request paths, secret paths, request IDs, entity IDs, token
  accessors, or client addresses.
- Expecting active-node scraping to provide complete standby and follower
  visibility.
- Using `sys/health`, `sys/leader`, or `sys/metrics` as an audit canary path.
- Reading token and lease inventory gauges as real-time values without checking
  `usage_gauge_period`.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed OpenBao docs behavior | OpenBao documents telemetry metric types, the `/sys/metrics` scrape endpoint, audit-device behavior, unaudited paths, and completed request logging. |
| Observed fixture behavior | The OpenBao 2.6.0 fixtures in this repository exercise HA/Raft metrics, audit streams, and both supported metric-prefix variants. |
| Design decision | This project normalizes source metrics into `openbao:` recording rules and keeps audit fields out of Loki labels. |
| To validate | Deployment-specific labels, scrape identities, retention controls, and OpenBao versions outside the validated fixture set. |

## Related pages

- Use [Metrics, logs, and audit logs](./metrics-vs-logs-vs-audit-logs.md) to
  choose the right signal for a question.
- Use [High-cardinality and label safety](./high-cardinality-and-label-safety.md)
  before you change Prometheus or Loki labels.
- Use [Active-node and all-node observability](./active-node-vs-all-node-observability.md)
  to choose a metrics scrape profile.
- Use [Audit logs as security records](./audit-logs-as-security-records.md)
  before you expand audit-log access or retention.
- Use [Token and lease observability](./token-and-lease-observability.md) to
  interpret token and lease panels.
- Use [OpenBao overview dashboard](../dashboards/overview-dashboard.md) to
  read the generated first-stop dashboard.
- Use [Configure a secure metrics scrape](../metrics/secure-metrics-scrape.md)
  for the authenticated active-node baseline.
- Use [Configure an all-node metrics scrape](../metrics/all-node-metrics-scrape.md)
  when you need per-node HA and Raft visibility.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) for
  repeatable audit-log setup.
- Use [Understand metric prefixes and recording rules](../contracts/metric-prefix.md)
  to map source metrics to generated rules.

Source: OpenBao documents telemetry behavior in the
[OpenBao telemetry documentation][openbao-telemetry] and
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. OpenBao
documents audit-device behavior and unaudited paths in the
[OpenBao audit device documentation][openbao-audit]. OpenBao documents
completed request logging in the
[OpenBao completed request logging documentation][openbao-log-requests].

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-log-requests]: https://openbao.org/docs/configuration/log-requests-level/
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
