# SLO and availability

Use this runbook when an OpenBao synthetic probe, availability burn-rate, or
synthetic latency alert fires. These alerts point to user-facing availability
or latency symptoms from a selected probe path.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get access to the OpenBao SLO and availability dashboard.
- Get access to the OpenBao overview and Kubernetes platform dashboards.
- Know which synthetic probe target, network location, and OpenBao endpoint the
  alert represents.
- Know the approved availability SLO target for the affected environment.

## Confirm the alert

1. Check which availability alert fired.

   ```promql
   ALERTS{alertstate="firing", alertname=~"OpenBao(SyntheticProbeFailing|AvailabilityFastBurn|AvailabilitySlowBurn|SyntheticProbeLatencyElevated)"}
   ```

2. Open the `OpenBao SLO and availability` dashboard.

3. Confirm the selected synthetic probe target.

   ```promql
   probe_success{job=~".*openbao.*"}
   ```

4. Confirm the probe duration.

   ```promql
   probe_duration_seconds{job=~".*openbao.*"}
   ```

## Investigate probe failure

1. Check whether every OpenBao probe target fails or only one target fails.

   ```promql
   min_over_time(probe_success{job=~".*openbao.*"}[5m])
   ```

2. Check whether Prometheus can still scrape OpenBao metrics.

   ```promql
   up{job=~"openbao.*"}
   ```

3. Check OpenBao cluster state.

   ```promql
   openbao:core_active:sum
   openbao:core_unsealed:sum
   openbao:autopilot_healthy:max
   ```

4. Query the same health path from a comparable network location.

   ```shell
   curl -fsS http://<openbao_address>/v1/sys/health
   ```

   - `<openbao_address>`: Address used by the affected synthetic probe, when
     you can safely reach it from the same network path.

## Investigate error-budget burn

1. Check short-window burn.

   ```promql
   (1 - avg_over_time(probe_success{job=~".*openbao.*"}[5m])) / 0.001
   ```

2. Check one-hour burn.

   ```promql
   (1 - avg_over_time(probe_success{job=~".*openbao.*"}[1h])) / 0.001
   ```

3. Check six-hour burn.

   ```promql
   (1 - avg_over_time(probe_success{job=~".*openbao.*"}[6h])) / 0.001
   ```

4. Confirm that the 99.9 percent target in the generated alert matches the
   approved SLO for the affected environment. If it does not match, treat the
   generated alert as a reference signal and use the local SLO policy for
   incident severity.

## Investigate synthetic latency

1. Compare current probe duration with the recent baseline.

   ```promql
   avg_over_time(probe_duration_seconds{job=~".*openbao.*"}[5m])
   avg_over_time(probe_duration_seconds{job=~".*openbao.*"}[1h])
   ```

2. Compare with OpenBao request latency.

   ```promql
   openbao:core_handle_request:avg5m
   openbao:core_handle_login_request:avg5m
   openbao:core_check_token:avg5m
   ```

3. Check storage, audit, and runtime context.

   ```promql
   openbao:barrier_get:avg5m
   openbao:audit_log_request:avg5m
   openbao:runtime_sys_bytes:max
   ```

4. Search operational logs for correlated symptoms.

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(timeout|unavailable|latency|slow|error|failed|connection refused)"
   ```

## Restore the baseline

1. If all probes fail and OpenBao metrics are also unavailable, use the metrics
   scrape and cluster-health runbooks before you tune SLO alerts.

2. If probes fail but OpenBao metrics and health are normal, investigate the
   load balancer, DNS, TLS, route, NetworkPolicy, proxy, and probe location.

3. If probe latency correlates with OpenBao request latency, investigate
   storage, audit, auth backend, token checks, runtime, and Raft health.

4. If only one probe location fails, route the incident through the owner of
   that network path or region while continuing to watch the global budget.

5. If the alert target does not represent the approved SLO path, update the
   synthetic probe contract and alert selector after the incident.

## Verify the result

1. Confirm probes return to success.

   ```promql
   min_over_time(probe_success{job=~".*openbao.*"}[5m])
   ```

2. Confirm short-window burn returns toward zero.

   ```promql
   (1 - avg_over_time(probe_success{job=~".*openbao.*"}[5m])) / 0.001
   ```

3. Confirm probe duration returns toward baseline.

   ```promql
   avg_over_time(probe_duration_seconds{job=~".*openbao.*"}[5m])
   ```

4. Confirm OpenBao internal latency is stable.

   ```promql
   openbao:core_handle_request:avg5m
   openbao:core_handle_login_request:avg5m
   openbao:core_check_token:avg5m
   ```

5. Wait for the alert window to pass and confirm the alert resolves.

## Troubleshooting

### Synthetic probe metrics are empty

Confirm that the synthetic probe job exists and that Prometheus scrapes it.
The generated dashboard and alerts expect `probe_success` and
`probe_duration_seconds`.

### The probe succeeds but users still report failures

The probe path may be too narrow or the probe location may not match the user
network path. Add probes for the affected route, region, or load balancer
path, but keep labels bounded.

### Burn-rate alerts do not match local SLO policy

The generated alerts assume a 99.9 percent availability target. Update the
alert contract when your approved target is different.

### Probe latency is high but OpenBao latency is normal

Investigate network, DNS, TLS, load balancer, proxy, and probe-location
dependencies before changing OpenBao.

## What's next

- Use [OpenBao SLO and availability dashboard](../dashboards/slo-availability.md)
  to inspect availability, burn rate, and latency context.
- Use [OpenBao overview dashboard](../dashboards/overview-dashboard.md) for
  cluster, request, audit, and runtime context.
- Use [OpenBao Kubernetes platform dashboard](../dashboards/kubernetes-platform.md)
  when the symptom may come from Kubernetes, PVCs, nodes, or collectors.
- Use [OpenBao metrics scrape failing](./openbao-metrics-scrape-failing.md)
  when the observability path is also degraded.
- Use [Synthetic probe example](../../examples/synthetic-probes/) to configure
  the optional probe metric surface.

Source: Prometheus documents blackbox-style multi-target probes with
`probe_success` and `probe_duration_seconds` in the
[Prometheus multi-target exporter guide][prometheus-blackbox]. Prometheus
documents alerting rules and the `for` clause in the
[Prometheus alerting rules documentation][prometheus-alerting].

[prometheus-alerting]: https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/
[prometheus-blackbox]: https://prometheus.io/docs/guides/multi-target-exporter/
