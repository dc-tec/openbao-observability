# OpenBao metrics scrape failing

Use this runbook when the `OpenBaoUnreachable` alert fires because Prometheus
cannot scrape one or more OpenBao metrics targets. The steps help you separate
an OpenBao health problem from a scrape configuration, network, or token issue.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get network access to the OpenBao metrics listener.
- Get a metrics token when your profile protects `/v1/sys/metrics` with an
  OpenBao token.
- Know whether your deployment uses the default `vault` metric prefix or an
  explicit `openbao` prefix.

## Confirm the scrape failure

1. Query the scrape result for all OpenBao targets.

   ```promql
   up{job="openbao"}
   ```

2. Check which discovered targets are down.

   ```promql
   up{job="openbao"} == 0
   ```

   The alert requires a discovered target to stay down for 5 minutes.

   If the first query returns no series, the alert uses an `absent()` fallback.
   This fallback keeps only the `job="openbao"` label. It cannot identify a
   missing cluster, pod, or instance.

3. Open the Prometheus targets page and inspect the error for the OpenBao job.
   The target error usually identifies DNS, TCP, TLS, HTTP status, or token
   failures.

## Check OpenBao health

1. Query the health endpoint on the affected node.

   ```shell
   curl -fsS http://<openbao_address>/v1/sys/health
   ```

   - `<openbao_address>`: OpenBao API address for the affected node, including
     scheme and port.

2. Interpret the health response.

   | Status | Meaning |
   | ------ | ------- |
   | `200` | The node is initialized, unsealed, and active. |
   | `429` | The node is initialized, unsealed, and standby. |
   | `501` | The node is not initialized. |
   | `503` | The node is sealed. |

3. If the node is sealed, switch to
   [OpenBao sealed unexpectedly](./openbao-sealed-unexpectedly.md).

## Check the metrics endpoint

1. Query the metrics endpoint from the same network path that Prometheus uses.

   ```shell
   curl -fsS --header "X-Vault-Token: <metrics_token>" 'http://<openbao_address>/v1/sys/metrics?format=prometheus'
   ```

   - `<metrics_token>`: Token allowed to read metrics when the listener requires
     authentication.
   - `<openbao_address>`: OpenBao API or metrics listener address for the
     affected node.

2. If the endpoint returns `403`, check the token policy and token expiration.

3. If the endpoint returns `404` or an empty response, check the OpenBao
   listener configuration. The listener must allow metrics and must not set
   `disallow_metrics = true` on the listener that Prometheus scrapes.

4. If the endpoint times out, check security groups, firewalls, NetworkPolicy,
   service selectors, and TLS settings between Prometheus and OpenBao.

## Restore the scrape

1. Fix the failing layer identified by the previous checks.

   | Failure | Action |
   | ------- | ------ |
   | OpenBao is sealed | Unseal or restore the seal backend. |
   | Token is denied | Rotate or reissue the metrics token with the required policy. |
   | DNS or service target is wrong | Correct the scrape target, Service, or service discovery labels. |
   | TLS verification fails | Correct the CA bundle, server name, or scrape scheme. |
   | Listener blocks metrics | Enable metrics on the private metrics listener. |

2. Reload or restart Prometheus when you changed scrape configuration.

3. Avoid exposing unauthenticated metrics on a broad network. If you use
   unauthenticated metrics access, keep the listener private and enforce access
   with network controls.

## Verify the result

1. Confirm that Prometheus sees the target as up.

   ```promql
   up{job="openbao"}
   ```

2. Confirm that OpenBao metrics are present.

   ```promql
   ${p}_core_active
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

3. Wait for the alert window to pass and confirm that
   `OpenBaoUnreachable` resolves.

## Troubleshooting

### Health works but metrics fail

Check listener-specific metrics settings. OpenBao can expose health and API
paths while the scraped listener still blocks `/v1/sys/metrics`.

### Prometheus target is up but the alert still fires

Check whether another target in the same job remains down. The
`OpenBaoUnreachable` alert evaluates the canonical `openbao` job.

If the alert has no target identity labels, Prometheus found no matching
`job="openbao"` target. Check service discovery and scrape configuration before
you investigate one OpenBao node.

### Metrics work from your workstation but not Prometheus

Run the same request from the Prometheus network path. Scrape failures often
come from service discovery, NetworkPolicy, or TLS trust differences.

## What's next

- Use [Run the Docker Compose stack](../docker-compose.md) to reproduce the
  local reference stack.
- Use [No active OpenBao leader](./no-active-openbao-leader.md) if metrics
  recover but OpenBao reports no active node.

Source: OpenBao documents `/v1/sys/health` status codes in the
[OpenBao health API documentation][openbao-health]. OpenBao documents
Prometheus format metrics in the
[OpenBao metrics API documentation][openbao-metrics].

[openbao-health]: https://openbao.org/api-docs/system/health/
[openbao-metrics]: https://openbao.org/api-docs/system/metrics/
