# OpenBao token and lease metrics

Use this explainer to understand the token and lease metrics used by the
generated dashboards and alerts. It is for operators who need to distinguish
inventory, operation rate, latency, auth method pressure, and irrevocable
lease signals.

## Why this matters

Token and lease metrics show authentication pressure and dynamic-secret
lifecycle pressure. They help you detect growth, churn, slow lifecycle
operations, and cleanup debt.

The most common mistake is to treat every token or lease metric as real-time
inventory. Some OpenBao usage gauges update on `usage_gauge_period`, which
defaults to 10 minutes.

## Inventory metrics

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_token_count` | `openbao:token_count:max30m` | Maximum token count observed over 30 minutes. |
| `${p}_expire_num_leases` | `openbao:expire_num_leases:max` | Maximum observed lease count. |
| `${p}_expire_num_irrevocable_leases` | `openbao:expire_num_irrevocable_leases:max` | Leases OpenBao reports it cannot automatically revoke. |

`${p}` is the source prefix. Use `vault` for the OpenBao default or `openbao`
when you configure `metrics_prefix = "openbao"`.

Use inventory metrics for trends and thresholds. Do not use them as exact
request-time state.

## Token operation metrics

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_token_creation` | `openbao:token_creation:increase15m` | Token creation events over 15 minutes. |
| `${p}_token_creation` by `auth_method` | `openbao:token_creation_by_auth:increase15m` | Token creation grouped by auth method. |
| `${p}_token_create` | `openbao:token_create:rate5m` and `openbao:token_create:avg5m` | Token create rate and average latency. |
| `${p}_token_lookup` | `openbao:token_lookup:rate5m` and `openbao:token_lookup:avg5m` | Token lookup rate and average latency. |
| `${p}_token_store` | `openbao:token_store:rate5m` and `openbao:token_store:avg5m` | Token store rate and average latency. |
| `${p}_token_revoke_tree` | `openbao:token_revoke_tree:rate5m` and `openbao:token_revoke_tree:avg5m` | Token revoke-tree rate and average latency. |

Read rates and latency together. Higher rate with stable latency often points
to workload growth. Higher latency without higher rate can point to storage,
auth method, token tree, or internal lifecycle pressure.

## Lease operation metrics

| Source metric | Recording rule | Interpretation |
| ------------- | -------------- | -------------- |
| `${p}_expire_revoke` | `openbao:expire_revoke:rate5m` and `openbao:expire_revoke:avg5m` | Lease revoke rate and average latency. |
| `${p}_expire_register_auth` | `openbao:expire_register_auth:rate5m` and `openbao:expire_register_auth:avg5m` | Auth lease registration rate and average latency. |

Use lease operation metrics with inventory metrics. Lease growth with low
revoke activity can indicate consumers that are not cleaning up. Lease latency
growth can indicate storage pressure or backend cleanup pressure.

## Auth method labels

`vault.token.creation` can include labels such as `auth_method`,
`creation_ttl`, `mount_point`, `namespace`, and `token_type`.

This project groups token creation by `auth_method` only. That gives a useful
view of auth method pressure without grouping by mount point, policy, token
accessor, entity ID, or client identity.

Treat `auth_method` as a trend dimension, not a full attribution model.

## Irrevocable leases

Nonzero irrevocable leases need response. They represent leases OpenBao reports
it cannot automatically revoke. That can leave downstream credentials or
resources outside the intended lifecycle.

Use the runbook when the value is nonzero, and correlate with operational logs,
secret-engine behavior, and recent changes.

## Common mistakes

- Reading `openbao:token_count:max30m` as exact real-time inventory.
- Grouping token metrics by `mount_point`, `policy`, entity ID, token accessor,
  or client address without a label review.
- Treating auth method grouping as user or application attribution.
- Ignoring irrevocable leases because the lease count is otherwise stable.
- Looking only at token creation while ignoring lookup, store, and revoke-tree
  latency.

## What's next

- Use [Token and lease observability](../concepts/token-and-lease-observability.md)
  for the operational mental model.
- Use [OpenBao token and lease lifecycle dashboard](../dashboards/token-lease-lifecycle.md)
  to read the generated dashboard.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add token or lease groupings.
- Use [Irrevocable leases present](../runbooks/irrevocable-leases.md) when
  OpenBao reports irrevocable leases.

Source: OpenBao documents leases in the
[OpenBao lease documentation][openbao-lease]. OpenBao documents tokens in the
[OpenBao token documentation][openbao-tokens]. OpenBao documents telemetry
metric types, labels, and `usage_gauge_period` behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. This page
also reflects the repository metric contract in
`contracts/metrics/openbao-core.yaml`.

[openbao-lease]: https://openbao.org/docs/concepts/lease/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
[openbao-tokens]: https://openbao.org/docs/concepts/tokens/
