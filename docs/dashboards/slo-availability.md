# OpenBao SLO and availability dashboard

Use this explainer to read the generated OpenBao SLO and availability
dashboard. It is for SRE and platform teams who need user-facing availability,
synthetic probe, latency, scrape, and error-budget context for OpenBao.

## What this dashboard is for

Use the SLO and availability dashboard when you need to separate user-facing
availability from internal OpenBao health.

The dashboard answers these questions:

- Are synthetic OpenBao probes succeeding?
- What is the 30-day probe availability?
- How much availability error budget remains?
- Is the error budget burning at a high rate?
- Are synthetic probe durations increasing?
- Do OpenBao request, login, or token-check latency signals correlate?
- Did Prometheus scrape availability change at the same time?

## What this dashboard is not for

Do not use this dashboard as a compliance report. It is an operational SLO
view over the metrics you collect.

Do not treat OpenBao scrape health as the same thing as user-facing
availability. Scrape health tells you whether Prometheus can collect metrics.
Synthetic probes tell you whether a selected user path works from a selected
network location.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | Expected UID | Used for |
| ----------- | ------------ | -------- |
| Prometheus | `prometheus` | Synthetic probe metrics, OpenBao recording rules, and scrape target health. |
| Loki | `loki` | Operational log context for availability and latency symptoms. |

Synthetic panels depend on `probe_success` and `probe_duration_seconds`, which
are commonly exposed by Prometheus blackbox-style probes. If you do not deploy
synthetic probes, those panels remain empty and synthetic-probe alerts do not
fire. `OpenBaoSyntheticProbeSignalMissing` fires only when you install a
matching [signal expectation](../../examples/signal-expectations/).

## Dashboard variables

The dashboard exposes these variables:

| Variable | Default | Purpose |
| -------- | ------- | ------- |
| Cluster | `.*` | Selects the stable OpenBao cluster identity. |
| Kubernetes namespace | `.*` | Selects the OpenBao workload namespace when multiple OpenBao instances share the same observability backend. |
| Scrape profile | `.*` | Selects the active or all-node metrics profile. |
| Synthetic probe job | `.*openbao.*` | Selects synthetic probe scrape jobs. |
| Synthetic probe target | `.*` | Selects probe targets by `instance`. |
| OpenBao scrape job | `openbao` | Selects the canonical OpenBao metrics scrape job. |
| Availability SLO target | `0.999` | Sets the SLO target used for error-budget calculations. |

Keep probe labels bounded. Do not put request paths, secret paths, token
metadata, entity identifiers, or tenant identifiers into probe labels.

Prometheus stat panels use instant queries. Probe panels show `Probe missing`
when the current query returns no series. They do not keep a prior successful
sample from the dashboard time range.

The 30-day availability panel uses a neutral color. Its selected SLO target is
a variable, and Grafana thresholds cannot follow that variable. Error-budget
and burn-rate panels calculate their values from the selected target.

## How to read availability

Start with probe success, 30-day availability, and error budget remaining.

`probe_success` is a binary signal: `1` means the probe succeeded, and `0`
means it failed. Availability over a window is the average of this binary
signal over that window.

The dashboard calculates approximate error budget remaining as:

```text
1 - ((1 - observed availability) / (1 - SLO target))
```

A value near `1` means most of the error budget remains. A value near `0`
means the selected target has consumed the 30-day budget for the configured
SLO target. Negative values mean the target is outside budget.

## How to read burn rate

Burn rate compares the current failure ratio with the allowed failure ratio.
With the default `0.999` target, the allowed failure ratio is `0.001`.

Use short and medium windows together. A high five-minute burn rate can be a
small transient failure. A high one-hour burn rate means the symptom persisted
long enough to threaten the budget.

The generated warning alerts use fixed 99.9 percent target math. If your
production SLO target differs, update the alert contract before you use the
alerts for paging.

## How to read latency context

Probe duration shows end-to-end synthetic latency from the probe location.
OpenBao request, login, and token-check latency show server-side internal
timing from OpenBao telemetry.

Read them together:

- Probe latency high and OpenBao latency normal usually points to network,
  DNS, load balancer, TLS, or probe-location issues.
- Probe latency high and OpenBao request latency high points to OpenBao,
  storage, audit, auth, or runtime pressure.
- OpenBao latency high while probes succeed can still be user-impacting for
  workloads that are not covered by the synthetic probe.

## How to read scrape context

Scrape availability is an observability-path signal. If probes fail and scrape
availability is healthy, Prometheus can still observe OpenBao during the
incident. If scrape availability is also degraded, you may have both a service
problem and an observability problem.

Use scrape availability with the metrics scrape runbook before you assume the
SLO dashboard is complete.

## Common mistakes

- Treating Prometheus scrape health as user-facing availability.
- Probing unaudited or overly narrow paths and calling the result an OpenBao
  availability SLO.
- Using one probe location to represent every client network.
- Putting request paths or tenant identifiers into probe labels.
- Using the default 99.9 percent alert math for an environment with a
  different approved SLO target.
- Treating error-budget dashboards as compliance evidence without an approved
  SLO policy.

## Known limitations

- The synthetic probe contract is optional.
- The generated warning alerts assume a 99.9 percent availability target.
- The dashboard calculates availability from probe success, not from every
  OpenBao client request.
- Probe labels and target names are deployment-specific.
- The dashboard does not define incident severity policy. Your organization
  owns paging thresholds and error-budget policy.

## Related pages

- Use [SLO and availability](../runbooks/slo-availability.md) when synthetic
  probe or burn-rate alerts fire.
- Use [Synthetic probe example](../../examples/synthetic-probes/) to map a
  blackbox-style probe into the expected metrics.
- Use [OpenBao overview dashboard](./overview-dashboard.md) when availability
  symptoms need OpenBao health context.
- Use [OpenBao Kubernetes platform dashboard](./kubernetes-platform.md) when
  availability symptoms may have a platform cause.
- Use [OpenBao metrics scrape failing](../runbooks/openbao-metrics-scrape-failing.md)
  when scrape availability drops.

Source: Prometheus documents blackbox-style multi-target probes with
`probe_success` and `probe_duration_seconds` in the
[Prometheus multi-target exporter guide][prometheus-blackbox]. Prometheus
documents alerting rules and the `for` clause in the
[Prometheus alerting rules documentation][prometheus-alerting]. The generated
dashboard contract is
`contracts/dashboards/openbao-slo-availability.yaml`.

[prometheus-alerting]: https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/
[prometheus-blackbox]: https://prometheus.io/docs/guides/multi-target-exporter/
