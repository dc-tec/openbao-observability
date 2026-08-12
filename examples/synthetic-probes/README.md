# Synthetic probes

Use this example to publish optional OpenBao synthetic probe metrics for the
SLO and availability dashboard. It is for operators who already run Prometheus
and want a blackbox-style probe contract for OpenBao availability.

## Metric contract

The generated SLO dashboard and availability alerts expect these metrics:

| Metric | Meaning |
| ------ | ------- |
| `probe_success` | `1` when the selected synthetic probe succeeds, and `0` when it fails. |
| `probe_duration_seconds` | End-to-end probe duration in seconds. |

Keep labels bounded. Use labels such as `job`, `instance`, `environment`,
`region`, and `cluster`. Do not label request paths, secret paths, tenant
identifiers, token metadata, or entity identifiers.

## Probe target

Use a low-risk OpenBao endpoint for the initial probe, usually
`/v1/sys/health`.

The health endpoint proves that the selected network path can reach OpenBao and
that OpenBao reports a service state. It does not prove that every auth method,
secret engine, policy, namespace, or client path works.

## Scrape example

Use [blackbox-scrape.example.yaml](./blackbox-scrape.example.yaml) as the
starting point for Prometheus blackbox exporter scraping.

Replace these values before production:

- `openbao-active.openbao.svc:8200`: OpenBao address or load balancer used by
  the probe.
- `blackbox-exporter.monitoring.svc:9115`: Blackbox exporter address.
- `environment`, `region`, and `cluster`: Bounded routing labels for your
  deployment.

Install a `synthetic_probe` [signal expectation](../signal-expectations/) when
this probe is required and its telemetry must not disappear silently.

## Validate the result

1. Confirm Prometheus receives probe success.

   ```promql
   probe_success{job=~".*openbao.*"}
   ```

2. Confirm Prometheus receives probe duration.

   ```promql
   probe_duration_seconds{job=~".*openbao.*"}
   ```

3. Open the `OpenBao SLO and availability` dashboard.

## What's next

- Use [OpenBao SLO and availability dashboard](../../docs/dashboards/slo-availability.md)
  to read the generated SLO view.
- Use [SLO and availability](../../docs/runbooks/slo-availability.md) when
  synthetic probe or burn-rate alerts fire.
- Use [Configure a secure metrics scrape](../../docs/metrics/secure-metrics-scrape.md)
  for OpenBao telemetry scraping. Synthetic probes do not replace OpenBao
  metrics.

Source: Prometheus documents blackbox-style multi-target probing in the
[Prometheus multi-target exporter guide][prometheus-blackbox].

[prometheus-blackbox]: https://prometheus.io/docs/guides/multi-target-exporter/
