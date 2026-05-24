# OpenBao token and lease lifecycle dashboard

Use this explainer to read the generated OpenBao token and lease lifecycle
dashboard. It is for operators who need to inspect token inventory, lease
inventory, lifecycle operation rates, latency, and audited token or lease
management activity.

## What this dashboard is for

Use the token and lease lifecycle dashboard when token creation, token
operation latency, lease count, or irrevocable lease alerts need investigation.

The dashboard answers these questions:

- How many tokens and leases does OpenBao report?
- Does OpenBao report irrevocable leases?
- Are token create, lookup, store, or revoke-tree rates changing?
- Are token or lease lifecycle operations getting slower?
- Which auth methods create tokens?
- Which audited token and lease management requests match the filters?

## What this dashboard is not for

Do not use this dashboard as a complete session inventory. Some panels use
usage gauges that update on a slower interval. Audit panels show activity that
Loki collected, not every token or lease currently present in storage.

Do not use token or lease audit fields as broad Loki labels. Treat them as
restricted investigation fields.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | Expected UID | Used for |
| ----------- | ------------ | -------- |
| Prometheus | `prometheus` | Token, lease, and normalized lifecycle recording rules. |
| Loki | `loki` | Audited token and lease management requests. |

Prometheus panels depend on generated `openbao:` recording rules. Loki panels
depend on audit logs collected with `log_stream="openbao.audit"`.

## Investigation filters

The dashboard exposes these variables:

| Variable | Type | Default | Use |
| -------- | ---- | ------- | --- |
| Auth method | Textbox | `.*` | Filter token creation metrics by auth method. |
| Request ID | Textbox | `.*` | Narrow audit panels to one request ID or pattern. |
| Request path | Textbox | `.*` | Narrow audit panels to token or lease paths. |
| Operation | Custom | `.*` | Filter audit panels by operation. |
| Node | Textbox | `.*` | Narrow audit panels to one OpenBao node label. |

Treat textbox values as regular expressions.

## How to read inventory panels

Token count, lease count, and irrevocable leases are inventory-style gauges.
Use them to detect trends and outliers.

OpenBao high-cardinality usage gauges update on `usage_gauge_period`, which
defaults to 10 minutes. The token count panel uses a 30-minute maximum to avoid
missing slow gauge updates. Do not read it as a per-scrape real-time value.

Irrevocable leases need attention because OpenBao reports that it cannot
automatically revoke them. Use the runbook when this value is nonzero.

## How to read token operation panels

Token create, lookup, store, and revoke-tree panels show rates and latency.
Read rate and latency together.

Higher token create rate with stable latency often points to expected load
growth. Higher latency without higher rate can point to storage, auth method,
or internal lifecycle pressure. Revoke-tree activity can indicate cleanup,
application shutdown, or token hierarchy changes.

## How to read lease operation panels

Lease revoke rate and lease operation latency show lifecycle pressure around
lease revocation and auth lease registration. Use these panels with lease
count and irrevocable lease signals.

Unexpected lease growth can point to dynamic secret consumers that do not
renew, revoke, or rotate as expected.

## How to read token creation by auth method

The token creation by auth method panel groups token creation by the
`auth_method` label. This is a bounded view compared with grouping by mount
point, policy, token accessor, entity ID, or client address.

Use it to identify which auth method contributes to token growth. Do not treat
auth method grouping as a full user or application attribution model.

## How to read audit panels

The audit panels filter `auth/token/.*` and `sys/leases/.*` paths. Use them to
inspect token lifecycle requests and lease lookup, revoke, or tidy requests
within the selected time range.

Audit panels give request context. Metric panels give rate, latency, and
inventory context. Use both views before you decide whether the issue is load,
storage, policy, application behavior, or cleanup behavior.

## Common mistakes

- Reading usage gauges as real-time inventory.
- Treating token creation by auth method as user attribution.
- Ignoring nonzero irrevocable leases.
- Grouping token or lease dashboards by accessors, entity IDs, or client
  addresses.
- Treating audit activity as complete inventory.

## Known limitations

- Token count can update on `usage_gauge_period`.
- The dashboard depends on normalized `openbao:` recording rules.
- Audit panels depend on `log_stream="openbao.audit"`.
- Audit panels cannot show token or lease records that were never collected by
  Loki.
- Auth method grouping is useful for trend analysis, not full attribution.

## What's next

- Use [Irrevocable leases present](../runbooks/irrevocable-leases.md) when
  OpenBao reports irrevocable leases.
- Use [OpenBao auth and identity dashboard](./auth-identity.md) when token
  lifecycle panels point to auth method activity.
- Use [Metrics, logs, and audit logs](../concepts/metrics-vs-logs-vs-audit-logs.md)
  to separate inventory, rate, latency, and audit evidence.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add token or lease fields to labels.

Source: OpenBao documents leases, renewal, and revocation in the
[OpenBao lease documentation][openbao-lease]. OpenBao documents telemetry
metric types, labels, and `usage_gauge_period` behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. This page
describes the generated dashboard contract in
`contracts/dashboards/openbao-token-lease-lifecycle.yaml`.

[openbao-lease]: https://openbao.org/docs/concepts/lease/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
