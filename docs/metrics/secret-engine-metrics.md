# OpenBao secret engine metrics

Use this reference to understand the secret-engine metrics used by generated
recording rules and dashboards. It is for operators who need to distinguish KV
inventory, dynamic-secret lease creation, and engine-specific operation
signals without turning secret paths into labels.

## Why this matters

Secret engines carry application-facing secret workflows. Metrics help you see
aggregate mount inventory, KV inventory, dynamic credential churn, PKI
activity, database backend behavior, and plugin failures.

Metrics should stay aggregate. Use audit logs for restricted request-level
investigation, and keep mount paths and secret paths out of Prometheus labels
unless you have reviewed the metadata exposure.

## Mount inventory metrics

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_core_mount_table_num_entries` | `openbao:core_mount_table_num_entries:max` | Mount table entries grouped by bounded mount metadata. |
| `${p}_core_mount_table_size` | `openbao:core_mount_table_size:max` | Mount table size grouped by bounded mount metadata. |

Use these to detect inventory growth or configuration churn. They are not a
replacement for the OpenBao mount table API when you need exact current
configuration.

## KV inventory metrics

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_secret_kv_count` | `openbao:secret_kv_count:max30m` | Total KV secret count observed over 30 minutes. |
| `${p}_secret_kv_count` by `namespace` | `openbao:secret_kv_count_by_namespace:max30m` | KV secret count grouped by namespace over 30 minutes. |

`${p}` is the source prefix. Use `vault` for the OpenBao default or `openbao`
when you configure `metrics_prefix = "openbao"`.

OpenBao emits `vault.secret.kv.count` as a usage gauge. The gauge updates on
`usage_gauge_period`, which defaults to 10 minutes. Read it as trend and
inventory evidence, not as exact request-time state.

The raw source metric can include `mount_point`. The generated rules do not
preserve mount-point grouping by default because mount names can reveal
deployment-specific context. Use mount-point drilldowns only after a label and
access review.

## KV route metrics

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_route_create_kv_v1_` | `openbao:route_create_kv_v1:rate5m` and `openbao:route_create_kv_v1:avg5m` | KV v1 create rate and average latency for the reference `kv-v1/` mount. |
| `${p}_route_read_kv_v1_` | `openbao:route_read_kv_v1:rate5m` and `openbao:route_read_kv_v1:avg5m` | KV v1 read rate and average latency for the reference `kv-v1/` mount. |
| `${p}_route_list_kv_v1_` | `openbao:route_list_kv_v1:rate5m` and `openbao:route_list_kv_v1:avg5m` | KV v1 list rate and average latency for the reference `kv-v1/` mount. |
| `${p}_route_delete_kv_v1_` | `openbao:route_delete_kv_v1:rate5m` and `openbao:route_delete_kv_v1:avg5m` | KV v1 delete rate and average latency for the reference `kv-v1/` mount. |
| `${p}_route_create_secret_` | `openbao:route_create_kv_v2:rate5m` and `openbao:route_create_kv_v2:avg5m` | KV v2 create rate and average latency for the default `secret/` mount. |
| `${p}_route_read_secret_` | `openbao:route_read_kv_v2:rate5m` and `openbao:route_read_kv_v2:avg5m` | KV v2 read rate and average latency for the default `secret/` mount. |
| `${p}_route_list_secret_` | `openbao:route_list_kv_v2:rate5m` and `openbao:route_list_kv_v2:avg5m` | KV v2 list rate and average latency for the default `secret/` mount. |
| `${p}_route_delete_secret_` | `openbao:route_delete_kv_v2:rate5m` and `openbao:route_delete_kv_v2:avg5m` | KV v2 delete rate and average latency for the default `secret/` mount. |

OpenBao derives route metric names from operation and mount path. The generated
KV route rules intentionally cover the fixture-backed `kv-v1/` reference mount
and the default `secret/` KV v2 mount. Use audit logs for broad mount-path
exploration, and add route rules for custom mounts only after reviewing the
naming and label-safety tradeoff.

## Dynamic secret metrics

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_secret_lease_creation` | `openbao:secret_lease_creation:increase15m` | Dynamic secret lease creation events over 15 minutes. |
| `${p}_secret_lease_creation` by `secret_engine` | `openbao:secret_lease_creation_by_engine:increase15m` | Lease creation grouped by bounded engine type. |
| `${p}_secret_lease_creation` by `namespace` and `secret_engine` | `openbao:secret_lease_creation_by_engine_namespace:increase15m` | Namespace-level dynamic secret activity by engine. |

Use these with lease inventory metrics. A rising lease creation rate with a
rising lease count often points to dynamic secret consumer growth or cleanup
changes.

## Engine operation metrics

Secret-engine-specific dashboards use additional normalized rules when OpenBao
emits the source metrics.

| Engine | Example recording rules | Use |
| ------ | ----------------------- | --- |
| Database | `openbao:database_new_user:rate5m`, `openbao:database_delete_user:rate5m`, `openbao:database_new_user_error:increase15m` | Credential creation, renewal, revocation, and backend failure signals. |
| PKI | `openbao:pki_issue:rate5m`, `openbao:pki_revoke:rate5m`, `openbao:pki_issue_failure:increase15m` | Certificate issue and revoke activity and failures. |

Read operation rates, latency, and failure counters together. A single metric
rarely proves whether the issue is workload growth, role configuration,
storage pressure, or a downstream backend problem.

## Label safety

Allowed metric grouping should be intentionally narrow:

- `namespace` is useful when deployments use OpenBao namespaces.
- `secret_engine` is useful for dynamic leases because it is a bounded engine
  type.
- `mount_point` is drilldown data, not a default aggregation label.

Avoid request paths, secret paths, token accessors, entity identifiers,
policies, and client addresses as labels.

## Common mistakes

- Reading `openbao:secret_kv_count:max30m` as exact real-time inventory.
- Grouping broad dashboards by `mount_point` without reviewing metadata
  exposure.
- Treating audit event volume as current secret inventory.
- Expecting the `kv-v1/` or `secret/` route rules to cover custom KV mount
  paths.
- Using dynamic lease creation as a proxy for static KV usage.
- Looking only at operation rate while ignoring latency, errors, and audit
  context.

## Related pages

- Use [OpenBao secret engines and mounts dashboard](../dashboards/secret-engines-mounts.md)
  to combine aggregate metrics with restricted audit investigation.
- Use [OpenBao database secrets dashboard](../dashboards/database-secrets.md)
  for focused database dynamic secret behavior.
- Use [OpenBao PKI dashboard](../dashboards/pki.md) for certificate lifecycle
  metrics and audit streams.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before adding mount or path dimensions.

Source: OpenBao documents secret engine telemetry, including
`vault.secret.kv.count`, in the
[OpenBao secrets telemetry documentation][openbao-secrets-telemetry]. OpenBao
documents metric types and `usage_gauge_period` behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. The metric
model also follows
`contracts/metrics/openbao-core.yaml`.

[openbao-secrets-telemetry]: https://openbao.org/docs/internals/telemetry/metrics/secrets/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
