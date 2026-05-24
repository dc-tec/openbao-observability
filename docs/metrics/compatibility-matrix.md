# Metric compatibility matrix

This reference explains how to use the generated metric compatibility matrix.
Use it to see which captured OpenBao fixture profiles expose each contracted
metric, source prefix, metric type, and label set.

## Matrix artifact

The generated matrix lives at
[Metric compatibility matrix](../../generated/docs/metric-compatibility-matrix.md).
Regenerate it with `make generate` after changing metric contracts or fixture
captures.

The matrix is derived from
[OpenBao core metrics](../../contracts/metrics/openbao-core.yaml) and the
captured fixtures under `fixtures/captured/`. It is not a hand-authored support
promise for every OpenBao deployment shape.

## How to read coverage

| Field | Meaning |
| ----- | ------- |
| OpenBao version | Version declared by the metric contract. |
| Profile | Captured fixture profile, such as prefix fixtures or HA/Raft node fixtures. |
| Prefix | Raw source prefix used by the fixture, such as `vault` or `openbao`. |
| Metric ID | Stable contract identifier used by this repository. |
| Docs metric | Upstream OpenBao metric name from documentation-style notation. |
| Source metric | Prometheus exposition name expected in that fixture. |
| Status | `observed` means the fixture contains the metric. `missing` means the fixture did not contain it. |
| Type | Prometheus metric family type observed in the fixture. |
| Labels | Label names observed in the fixture. Label values are omitted. |
| Required | Whether contract verification requires the metric in prefix fixtures. |
| Overview | Whether the metric contributes to the overview dashboard layer. |
| Notes | Contract notes and validation caveats. |

## Evidence limits

An observed metric proves fixture coverage for that profile. It does not prove
that every OpenBao deployment exposes the metric with the same label set under
all configurations.

A missing optional metric does not prove that OpenBao lacks the signal. Usage
gauges, feature-specific metrics, and route-derived metrics depend on workload,
enabled features, scrape timing, and telemetry configuration.

Use label names as compatibility evidence, not as permission to create
high-cardinality labels. Keep paths, request IDs, token accessors, entity IDs,
and client addresses out of Prometheus labels.

## Related pages

- [Understanding OpenBao metrics](./understanding-openbao-metrics.md)
- [OpenBao HA/Raft metrics](./ha-raft-metrics.md)
- [OpenBao token and lease metrics](./token-and-lease-metrics.md)
- [Understand metric prefixes and recording rules](../contracts/metric-prefix.md)
