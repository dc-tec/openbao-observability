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

Aggregate recording rules preserve the canonical identity labels that their
source signals provide. Use `kubernetes_namespace` to distinguish workload
placement. Use `openbao_namespace` only for OpenBao logical namespace context.
This separation lets one observability backend distinguish deployments without
overwriting OpenBao namespace data.

| Signal | Source metric with `vault` prefix | Source metric with `openbao` prefix | Recording rule |
| ------ | --------------------------------- | ----------------------------------- | -------------- |
| Active node count | `vault_core_active` | `openbao_core_active` | `openbao:core_active:sum` |
| Unsealed node count | `vault_core_unsealed` | `openbao_core_unsealed` | `openbao:core_unsealed:sum` |
| Audit request failures | `vault_audit_log_request_failure` | `openbao_audit_log_request_failure` | `openbao:audit_log_request_failure:increase5m` |
| Audit response failures | `vault_audit_log_response_failure` | `openbao_audit_log_response_failure` | `openbao:audit_log_response_failure:increase5m` |
| Token count | `vault_token_count` | `openbao_token_count` | `openbao:token_count:max30m` |
| Token creation | `vault_token_creation` | `openbao_token_creation` | `openbao:token_creation:increase15m` |
| Lease count | `vault_expire_num_leases` | `openbao_expire_num_leases` | `openbao:expire_num_leases:max` |
| Irrevocable leases | `vault_expire_num_irrevocable_leases` | `openbao_expire_num_irrevocable_leases` | `openbao:expire_num_irrevocable_leases:max` |
| Goroutines | `vault_runtime_num_goroutines` | `openbao_runtime_num_goroutines` | `openbao:runtime_num_goroutines:max` |
| Heap objects | `vault_runtime_heap_objects` | `openbao_runtime_heap_objects` | `openbao:runtime_heap_objects:max` |
| System bytes | `vault_runtime_sys_bytes` | `openbao_runtime_sys_bytes` | `openbao:runtime_sys_bytes:max` |
| Barrier GET latency | `vault_barrier_get` | `openbao_barrier_get` | `openbao:barrier_get:avg5m` |
| Barrier PUT latency | `vault_barrier_put` | `openbao_barrier_put` | `openbao:barrier_put:avg5m` |
| Cache hit ratio | `vault_cache_hit`, `vault_cache_miss` | `openbao_cache_hit`, `openbao_cache_miss` | `openbao:cache_hit_ratio:ratio5m` |
| Mount table entries | `vault_core_mount_table_num_entries` | `openbao_core_mount_table_num_entries` | `openbao:core_mount_table_num_entries:max` |
| Raft peer count | `vault_raft_peers` | `openbao_raft_peers` | `openbao:raft_peers:max` |
| Autopilot health | `vault_autopilot_healthy` | `openbao_autopilot_healthy` | `openbao:autopilot_healthy:max` |

The repository ships generated rule variants for both supported source
prefixes. Use the variant that matches the metrics your OpenBao deployment
emits.

| Source prefix | Native Prometheus rules | Prometheus Operator rules |
| ------------- | ----------------------- | ------------------------- |
| `vault` | `generated/prometheus/vault-prefix/` | `generated/prometheusrules/vault-prefix/` |
| `openbao` | `generated/prometheus/openbao-prefix/` | `generated/prometheusrules/openbao-prefix/` |

The top-level `generated/prometheus/openbao-recording-rules.yaml` and
`generated/prometheusrules/openbao-recording-rules.yaml` files remain the
default `vault` variant for the local Docker Compose stack.

Regenerate all variants after you change the metric contract, alert contracts,
or generator code.

```shell
make generate
```

You can still generate a single custom output with `--source-prefix` when you
need a one-off path.

```shell
go run ./cmd/openbao-observability generate prometheus-rules \
  --contract contracts/metrics/openbao-core.yaml \
  --source-prefix openbao \
  --output /tmp/openbao-recording-rules.yaml \
  --rule-output /tmp/openbao-native-rules.yaml
```

## Query source metrics during validation

Use source metrics when you need to confirm what OpenBao emits.

```promql
vault_core_active
vault_core_unsealed
vault_audit_log_request_failure
vault_runtime_num_goroutines
vault_barrier_get
vault_cache_hit
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
include an OpenBao-generated `cluster` label. Some development and fixture
profiles also emit an empty `cluster=""` series. Some runtime or lease metrics
rely on scrape labels instead.

Use these canonical identity labels in collected metrics:

| Label | Meaning |
| ----- | ------- |
| `cluster` | Platform-assigned OpenBao deployment identity. |
| `kubernetes_namespace` | Kubernetes workload namespace. Omit it outside Kubernetes. |
| `openbao_namespace` | Logical OpenBao namespace when the source metric includes one. |
| `pod` | Kubernetes pod or equivalent workload identity. |
| `instance` | Prometheus scrape endpoint identity. |
| `scrape_profile` | `active` or `all_nodes`. |

Keep `honorLabels` disabled. Target relabeling must set deployment identity.
Metric relabeling must copy the OpenBao-generated `namespace` label to
`openbao_namespace` and remove the ambiguous source label. Do not assign a
synthetic `openbao_namespace` value to cluster-scoped metrics.

When a target label replaces the OpenBao-generated `cluster` label, Prometheus
preserves the source value as `exported_cluster`. Drop the empty
`*_core_unsealed` source series before you remove `exported_cluster`. This keeps
the empty fixture series from becoming a duplicate of the platform-labeled
series.

Recording rules in this project normalize the signals that dashboards need.
When you write custom queries, check the live label set before grouping by
`cluster`, `kubernetes_namespace`, `openbao_namespace`, `pod`, `instance`, or
`scrape_profile`.

## Raft peer count behavior

The HA/Raft dashboard uses `openbao:raft_peers:max` instead of raw
`vault_raft_peers`. The normalized rule prefers the raw Raft peer metric when
OpenBao exposes it and falls back to counting
`*_raft_storage_stats_commit_index` by `peer_id` in all-node scrape profiles.

This fallback exists because the current OpenBao 2.6.0 HA/Raft fixture
observed `vault_raft_peers` on the active node, while the live Docker Compose
all-node scrape exposed Raft storage stats without `vault_raft_peers`.

## Validate generated artifacts

Run the repository verification target after you change metric prefixes,
recording rules, alert contracts, or dashboard contracts.

```shell
make verify
```

When the Docker Compose stack is already running, validate dashboard queries
against Prometheus and Loki.

```shell
make verify-live
```

## Related files

| File | Purpose |
| ---- | ------- |
| `contracts/metrics/openbao-core.yaml` | Defines source metric names, supported prefixes, fixture expectations, and normalization notes. |
| `generated/prometheus/openbao-recording-rules.yaml` | Native Prometheus rule file for the local Compose stack. |
| `generated/prometheusrules/openbao-recording-rules.yaml` | Prometheus Operator `PrometheusRule` artifact. |
| `generated/prometheus/vault-prefix/` | Native Prometheus rules for OpenBao deployments that emit `vault_*` metrics. |
| `generated/prometheus/openbao-prefix/` | Native Prometheus rules for OpenBao deployments that emit `openbao_*` metrics. |
| `generated/prometheusrules/vault-prefix/` | Prometheus Operator rules for OpenBao deployments that emit `vault_*` metrics. |
| `generated/prometheusrules/openbao-prefix/` | Prometheus Operator rules for OpenBao deployments that emit `openbao_*` metrics. |
| `contracts/dashboards/openbao-overview.yaml` | Overview dashboard contract that consumes normalized rules. |
| `contracts/dashboards/openbao-ha-raft.yaml` | HA/Raft dashboard contract that consumes normalized rules and validated Raft source metrics. |
| `contracts/dashboards/openbao-audit-investigation.yaml` | Audit investigation dashboard contract that uses query-time audit fields without turning them into Loki labels. |
| `contracts/dashboards/openbao-auth-identity.yaml` | Auth and identity dashboard contract that filters audit request paths at query time without turning them into Loki labels. |
| `contracts/dashboards/openbao-token-lease-lifecycle.yaml` | Token and lease lifecycle dashboard contract that consumes normalized token and lease rules plus query-time audit fields. |
| `contracts/dashboards/openbao-secret-engines-mounts.yaml` | Secret engines and mounts dashboard contract that filters engine paths at query time without turning mount paths into Loki labels. |
| `contracts/dashboards/openbao-transit.yaml` | Transit dashboard contract that filters key management and cryptographic audit paths at query time without turning Transit key names into Loki labels. |
| `contracts/dashboards/openbao-pki.yaml` | PKI dashboard contract that consumes normalized PKI rules and filters certificate lifecycle audit paths at query time. |
| `contracts/dashboards/openbao-runtime-storage.yaml` | Runtime and storage dashboard contract that consumes normalized runtime, barrier, cache, and mount-table rules. |
| `contracts/dashboards/openbao-namespaces-scale.yaml` | Namespaces and scale dashboard contract that consumes namespace-aware token and lease rules plus Raft read-replica diagnostics. |
| `contracts/dashboards/openbao-kubernetes-platform.yaml` | Kubernetes platform dashboard contract that consumes kube-state-metrics, kubelet, cAdvisor, scrape target, and platform event signals. |
| `contracts/dashboards/openbao-slo-availability.yaml` | SLO and availability dashboard contract that consumes optional synthetic probe metrics, OpenBao latency rules, and scrape availability signals. |
| `contracts/alerts/critical.yaml` | Alert contract that maps critical alerts to runbooks. |
| `contracts/alerts/warning.yaml` | Alert contract that maps warning alerts to runbooks. |

## Related pages

- Use [OpenBao observability model](../concepts/openbao-observability-model.md)
  to understand why the project separates source and derived signals.
- Use [Understanding OpenBao metrics](../metrics/understanding-openbao-metrics.md)
  to understand metric types, labels, scrape profiles, and validation.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you group source metrics by optional OpenBao labels.
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
