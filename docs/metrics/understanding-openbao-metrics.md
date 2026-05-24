# Understanding OpenBao metrics

Use this explainer to understand how OpenBao metrics become recording rules,
dashboards, and alerts in this reference architecture. It is for operators who
need to read metric names, prefixes, labels, and scrape-profile assumptions
without reverse-engineering PromQL.

## Why this matters

OpenBao metrics are the fastest way to detect health, rate, latency,
saturation, runtime pressure, audit-device failures, token pressure, lease
pressure, and HA/Raft state.

They are also easy to misread. Raw source metrics can use historical `vault`
naming, labels vary by metric family, some gauges update slowly, and scrape
profile determines which nodes Prometheus can see.

## Mental model

Read the metrics pipeline in layers.

```text
OpenBao telemetry
  -> /v1/sys/metrics?format=prometheus
  -> Prometheus source metrics
  -> normalized openbao: recording rules
  -> dashboards and alerts
```

Validate raw source metrics when you troubleshoot scraping or OpenBao
telemetry. Use normalized recording rules for dashboards and alerts when a
rule exists.

## Metric types

OpenBao documents three telemetry metric types.

| Type | Meaning | How to read it |
| ---- | ------- | -------------- |
| Counter | An event count that increases when something happens. | Use `rate()` or `increase()` over a time window. |
| Gauge | A current value. | Read current value or a max/min over a time window. |
| Summary | Observations for discrete work, often duration. | Use `_sum` and `_count` to calculate average latency. |

High-cardinality usage gauges, such as token counts and secret counts, update
on `usage_gauge_period`. The OpenBao default is 10 minutes. Do not read those
gauges as per-scrape real-time inventory.

## Source metric names

OpenBao documentation uses dot-separated metric names such as
`vault.core.active`. Prometheus exposes those names with underscores, such as
`vault_core_active`.

OpenBao still documents `vault` as the default telemetry prefix. You can set
`metrics_prefix = "openbao"` in the telemetry stanza when you want source
metrics such as `openbao_core_active`.

This project supports both source prefixes:

| Source prefix | Example source metric | Use when |
| ------------- | --------------------- | -------- |
| `vault` | `vault_core_active` | You use the OpenBao default prefix. |
| `openbao` | `openbao_core_active` | You explicitly configure `metrics_prefix = "openbao"`. |

## Normalized recording rules

Generated recording rules use the `openbao:` namespace, even when source
metrics use the `vault_*` prefix. This keeps dashboards and alerts stable
across both source-prefix profiles.

Examples:

| Source signal | Recording rule |
| ------------- | -------------- |
| Active node count | `openbao:core_active:sum` |
| Request rate | `openbao:core_handle_request:rate5m` |
| Request latency | `openbao:core_handle_request:avg5m` |
| Audit request failures | `openbao:audit_log_request_failure:increase5m` |
| Lease count | `openbao:expire_num_leases:max` |
| Token count | `openbao:token_count:max30m` |
| Raft peer count | `openbao:raft_peers:max` |

Use [Understand metric prefixes and recording rules](../contracts/metric-prefix.md)
when you need the full prefix and artifact mapping.

## Labels

OpenBao metric labels are not uniform. Some metrics include `cluster`, some
runtime metrics rely mostly on scrape labels, and token metrics can include
labels such as `auth_method`, `creation_ttl`, `mount_point`, `namespace`, and
`token_type`.

Treat labels as part of the metric contract, not as free dimensions. Before you
group by a label:

- Check whether the label exists on the live series.
- Check whether the label can expose sensitive metadata.
- Check whether the label has bounded cardinality.
- Check whether the dashboard or alert still works across both source prefixes.

## Scrape profiles

Metrics interpretation depends on the scrape profile.

| Profile | Strength | Limitation |
| ------- | -------- | ---------- |
| Authenticated active-node scrape | Strong secure baseline for cluster-level health and active request behavior. | Limited standby and follower visibility. |
| Private all-node scrape | Better HA/Raft, standby, follower, and per-node runtime visibility. | Requires isolated metrics access and label review. |
| Local Docker Compose scrape | Useful for reference-stack validation. | Not a production security model. |

Use the active-node profile as the secure baseline. Add all-node scraping when
you need HA/Raft diagnostics or per-node visibility.

## Validation

This project validates metrics at three layers:

- Captured OpenBao 2.5.4 fixtures under `fixtures/captured/openbao-2.5.4/`.
- Metric contracts under `contracts/metrics/`.
- Generated Prometheus rules under `generated/prometheus/` and
  `generated/prometheusrules/`.

Run the full verification target after you change metrics, rules, dashboards,
or alerts.

```shell
make verify
```

## Common mistakes

- Querying raw `vault_*` metrics from a deployment that emits `openbao_*`.
- Treating high-cardinality usage gauges as real-time inventory.
- Grouping by `mount_point`, `policy`, request path, or token metadata without
  a label review.
- Expecting active-node scraping to show every standby or follower signal.
- Treating an empty panel as an incident before checking source prefix, scrape
  profile, and recording rule deployment.
- Writing dashboards directly against source metrics when a normalized rule
  exists.

## What's next

- Use [Active-node and all-node observability](../concepts/active-node-vs-all-node-observability.md)
  to choose the right scrape profile.
- Use [OpenBao HA/Raft metrics](./ha-raft-metrics.md) for HA and Raft metric
  interpretation.
- Use [OpenBao token and lease metrics](./token-and-lease-metrics.md) for
  token and lease metric interpretation.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add labels or groupings.
- Use [Understand metric prefixes and recording rules](../contracts/metric-prefix.md)
  for generated rule artifacts.

Source: OpenBao documents telemetry collection in the
[OpenBao telemetry documentation][openbao-telemetry]. OpenBao documents metric
types, labels, and high-cardinality gauge behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. OpenBao
documents the Prometheus metrics endpoint in the
[OpenBao metrics API documentation][openbao-metrics-api].

[openbao-metrics-api]: https://openbao.org/api-docs/system/metrics/
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
