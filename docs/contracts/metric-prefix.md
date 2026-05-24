# Understand metric prefixes and recording rules

Use this reference to map OpenBao source metrics to the normalized recording
rules used by this project. It is for operators who need to configure metric
prefixes, read generated alerts, or adapt the dashboards to an existing
Prometheus-compatible backend.

## Prefix strategy

OpenBao still documents `vault` as the default telemetry prefix. This project
therefore treats `vault_*` as the default source metric family and supports
`openbao_*` when you explicitly configure `metrics_prefix = "openbao"`.

| Layer | Example | Meaning |
| ----- | ------- | ------- |
| OpenBao docs metric name | `vault.core.active` | The upstream metric name used in OpenBao documentation. |
| Prometheus source metric | `vault_core_active` | The Prometheus exposition name when `metrics_prefix = "vault"`. |
| Alternate source metric | `openbao_core_active` | The Prometheus exposition name when `metrics_prefix = "openbao"`. |
| Recording rule | `openbao:core_active:sum` | The normalized project rule used by dashboards and alerts. |

Use source metrics for capture and validation. Use recording rules for
dashboards and critical alerts when a rule exists.

## Choose a prefix

| Prefix | Use when | Tradeoff |
| ------ | -------- | -------- |
| `vault` | You want the OpenBao default and the widest compatibility with existing examples. | The name still carries historical Vault terminology. |
| `openbao` | You intentionally want OpenBao-branded source metrics. | Existing dashboards, alerts, and filters that hardcode `vault_*` must change. |

Set the prefix in the OpenBao telemetry stanza.

```hcl
telemetry {
  prometheus_retention_time = "30s"
  disable_hostname          = true
  metrics_prefix            = "vault"
}
```

If you use `prefix_filter`, write filters for the configured OpenBao prefix.
For example, use `+openbao.core` only when you also set
`metrics_prefix = "openbao"`.

## Use normalized rules

The generated recording rules use the `openbao:` namespace even when the source
metric prefix is `vault`. This keeps Grafana dashboards and alert expressions
stable across deployments.

| Signal | Source metric with `vault` prefix | Source metric with `openbao` prefix | Recording rule |
| ------ | --------------------------------- | ----------------------------------- | -------------- |
| Active node count | `vault_core_active` | `openbao_core_active` | `openbao:core_active:sum` |
| Unsealed node count | `vault_core_unsealed` | `openbao_core_unsealed` | `openbao:core_unsealed:sum` |
| Audit request failures | `vault_audit_log_request_failure` | `openbao_audit_log_request_failure` | `openbao:audit_log_request_failure:increase5m` |
| Audit response failures | `vault_audit_log_response_failure` | `openbao_audit_log_response_failure` | `openbao:audit_log_response_failure:increase5m` |
| Lease count | `vault_expire_num_leases` | `openbao_expire_num_leases` | `openbao:expire_num_leases:max` |
| Goroutines | `vault_runtime_num_goroutines` | `openbao_runtime_num_goroutines` | `openbao:runtime_num_goroutines:max` |
| Raft peer count | `vault_raft_peers` | `openbao_raft_peers` | `openbao:raft_peers:max` |
| Autopilot health | `vault_autopilot_healthy` | `openbao_autopilot_healthy` | `openbao:autopilot_healthy:max` |

Generate rules for the source prefix that your OpenBao deployment emits.

```shell
go run ./cmd/openbao-observability generate prometheus-rules \
  --contract contracts/metrics/openbao-core.yaml \
  --source-prefix vault \
  --output generated/prometheusrules/openbao-recording-rules.yaml \
  --rule-output generated/prometheus/openbao-recording-rules.yaml
```

Use `--source-prefix openbao` when your OpenBao servers emit `openbao_*`
metrics.

## Query source metrics during validation

Use source metrics when you need to confirm what OpenBao emits.

```promql
vault_core_active
vault_core_unsealed
vault_audit_log_request_failure
vault_runtime_num_goroutines
```

For an `openbao`-prefixed deployment, replace `vault_` with `openbao_`.

Use compatibility queries only during migration or discovery. Regex selectors
over metric names are useful for exploration, but generated dashboards and
alerts should use direct source metrics or normalized recording rules.

```promql
{__name__=~"^(vault|openbao)_core_active$"}
```

## Account for label differences

Raw OpenBao metrics do not expose one uniform label set. Some core metrics
include an OpenBao `cluster` label, some development and fixture profiles emit
an empty `cluster=""` series, and some runtime or lease metrics rely on scrape
labels instead.

Recording rules in this project normalize the signals that dashboards need.
When you write custom queries, check the live label set before grouping by
`cluster`, `namespace`, `pod`, or `instance`.

## Raft peer count behavior

The HA/Raft dashboard uses `openbao:raft_peers:max` instead of raw
`vault_raft_peers`. The normalized rule prefers the raw Raft peer metric when
OpenBao exposes it and falls back to counting
`*_raft_storage_stats_commit_index` by `peer_id` in all-node scrape profiles.

This fallback exists because the current OpenBao 2.5.4 HA/Raft fixture
observed `vault_raft_peers` on the active node, while the live Docker Compose
all-node scrape exposed Raft storage stats without `vault_raft_peers`.

## Validate generated artifacts

Run the contract and generated rule checks after you change metric prefixes,
recording rules, or dashboard contracts.

```shell
make generate
make contracts-verify
make validate-generated
```

Run the full test target before you publish a change.

```shell
make test
```

## Related files

| File | Purpose |
| ---- | ------- |
| `contracts/metrics/openbao-core.yaml` | Defines source metric names, supported prefixes, fixture expectations, and normalization notes. |
| `generated/prometheus/openbao-recording-rules.yaml` | Native Prometheus rule file for the local Compose stack. |
| `generated/prometheusrules/openbao-recording-rules.yaml` | Prometheus Operator `PrometheusRule` artifact. |
| `contracts/dashboards/openbao-overview.yaml` | Overview dashboard contract that consumes normalized rules. |
| `contracts/dashboards/openbao-ha-raft.yaml` | HA/Raft dashboard contract that consumes normalized rules and validated Raft source metrics. |
| `contracts/dashboards/openbao-audit-investigation.yaml` | Audit investigation dashboard contract that uses query-time audit fields without turning them into Loki labels. |
| `contracts/dashboards/openbao-auth-identity.yaml` | Auth and identity dashboard contract that filters audit request paths at query time without turning them into Loki labels. |
| `contracts/alerts/critical.yaml` | Alert contract that maps critical alerts to runbooks. |
| `contracts/alerts/warning.yaml` | Alert contract that maps warning alerts to runbooks. |

## What's next

- Use [Configure a secure metrics scrape](../metrics/secure-metrics-scrape.md)
  to configure the authenticated active-node profile.
- Use [Configure an all-node metrics scrape](../metrics/all-node-metrics-scrape.md)
  when you need standby and Raft follower visibility.
- Use [Configure declarative audit devices](../audit/declarative-audit.md)
  when you need repeatable audit stream setup.
- Use [Run the Docker Compose stack](../docker-compose.md) to inspect live
  source metrics and normalized rules locally.

Source: OpenBao documents `metrics_prefix`, `disable_hostname`,
`prometheus_retention_time`, `prefix_filter`, and Prometheus scrape behavior in
the [OpenBao telemetry documentation][openbao-telemetry]. OpenBao documents
the `/sys/metrics` endpoint and Prometheus output examples in the
[OpenBao metrics API documentation][openbao-metrics-api].

[openbao-metrics-api]: https://openbao.org/api-docs/system/metrics/
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
