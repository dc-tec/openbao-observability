# Token and lease observability

Use this explainer to understand OpenBao token and lease signals. It is for
operators who need to distinguish authentication pressure, token lifecycle
activity, dynamic-secret lease behavior, and cleanup problems.

## Why this matters

Tokens and leases sit on critical OpenBao workflows. Tokens represent
authenticated access. Leases represent time-bound validity for service tokens
and dynamic secrets. If token or lease behavior changes unexpectedly, the cause
can be load growth, auth method behavior, application lifecycle changes,
dynamic secret consumers, storage pressure, or cleanup failure.

Good observability separates inventory, rate, latency, and audit evidence.

## Mental model

Read token and lease behavior from four signal types:

| Signal | What it tells you |
| ------ | ----------------- |
| Inventory gauges | How many tokens or leases OpenBao reports. |
| Operation rates | How often lifecycle operations occur. |
| Operation latency | How long lifecycle operations take. |
| Audit events | Which audited token or lease paths were used. |

Do not treat one signal as the whole story. Inventory tells you volume. Rates
tell you activity. Latency tells you pressure. Audit logs give request context.

## OpenBao behavior

OpenBao service tokens and dynamic secrets use leases. A lease includes
metadata such as duration and renewability. When a lease expires, OpenBao can
automatically revoke it. When a token is revoked, OpenBao revokes leases that
were created by that token.

OpenBao token types matter. Service tokens are tracked, renewable when
configured, manually revocable, and can create child tokens. Batch tokens are
lighter-weight encrypted blobs and do not have the same storage or revocation
properties.

OpenBao high-cardinality usage gauges, such as token counts and secret counts,
update on `usage_gauge_period`. The default interval is 10 minutes.

## Inventory signals

Inventory panels include token count, lease count, and irrevocable leases.

Use inventory signals to detect:

- Token growth.
- Lease growth.
- Irrevocable leases.
- Changes after auth method or application rollouts.
- Cleanup patterns after revocation or shutdown.

Inventory gauges can lag. A token count panel that uses a 30-minute window can
be more reliable than a single scrape because high-cardinality gauges update
less frequently.

## Operation-rate signals

Token operation rates show lifecycle activity such as create, lookup, store,
and revoke-tree operations. Lease operation rates show revocation and lease
registration behavior.

Use rate signals to distinguish a larger inventory from active churn. For
example, a rising token count with a rising create rate points to new access
activity. A rising token count without matching creation activity can point to
slow gauge updates, long TTLs, or retention of existing tokens.

## Latency signals

Token and lease latency panels show how long lifecycle operations take.

Rising latency can indicate:

- Storage pressure.
- Auth method pressure.
- Large token trees.
- Slow revocation paths.
- Lease cleanup pressure.
- Downstream dynamic-secret backend behavior.

Compare latency with operation rate and operational logs before assigning a
cause.

## Audit signals

Audit panels filter paths such as `auth/token/.*` and `sys/leases/.*`. Use
them to inspect token create, renew, revoke, lookup, lease lookup, lease
revoke, and lease tidy activity.

Audit logs provide request context, but they are not inventory. A token can
exist without appearing in the current audit retention window. A lease can
exist because of older dynamic secret activity outside the dashboard time
range.

## Auth method grouping

Token creation by auth method is useful because `auth_method` is a bounded
metric label compared with request path, entity ID, token accessor, or client
address.

Use auth method grouping to identify which auth method contributes to token
creation. Do not treat it as user attribution, application attribution, or
policy attribution.

## Irrevocable leases

Irrevocable leases represent operational debt. OpenBao reports leases it cannot
automatically revoke. This can happen when a backend, plugin, downstream
system, or stored revocation path prevents cleanup.

Treat nonzero irrevocable leases as an issue to investigate. They can represent
credentials or resources that outlive the intended lifecycle.

## Common mistakes

- Reading high-cardinality gauges as per-scrape real-time inventory.
- Treating token creation by auth method as user attribution.
- Ignoring nonzero irrevocable leases.
- Using token accessors, entity IDs, client addresses, or request paths as
  broad labels.
- Treating audit activity as complete token or lease inventory.
- Looking only at rate without checking latency and inventory.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed OpenBao docs behavior | OpenBao documents token types, lease renewal, revocation, token-linked leases, and `usage_gauge_period` behavior. |
| Observed fixture behavior | The OpenBao 2.6.0 fixture emits token and lease metrics plus token and lease audit events from the demo workload. |
| Design decision | This project combines metrics for inventory, rate, and latency with query-time audit filters for request context. |
| To validate | Auth method mix, token TTL policy, dynamic-secret backends, lease cleanup behavior, and retention windows in your deployment. |

## What's next

- Use [OpenBao token and lease lifecycle dashboard](../dashboards/token-lease-lifecycle.md)
  to inspect the generated token and lease view.
- Use [OpenBao token and lease metrics](../metrics/token-and-lease-metrics.md)
  to connect token and lease concepts to recording rules.
- Use [OpenBao auth and identity dashboard](../dashboards/auth-identity.md)
  when token signals point to auth method behavior.
- Use [Metrics, logs, and audit logs](./metrics-vs-logs-vs-audit-logs.md) to
  separate inventory, rate, latency, and audit evidence.
- Use [High-cardinality and label safety](./high-cardinality-and-label-safety.md)
  before you add token or lease fields to labels.
- Use [Irrevocable leases present](../runbooks/irrevocable-leases.md) when
  OpenBao reports irrevocable leases.

Source: OpenBao documents leases, renewal, revocation, and token-linked lease
behavior in the [OpenBao lease documentation][openbao-lease]. OpenBao
documents service tokens, batch tokens, and token lease handling in the
[OpenBao token documentation][openbao-tokens]. OpenBao documents telemetry
metric types, labels, and `usage_gauge_period` behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics].

[openbao-lease]: https://openbao.org/docs/concepts/lease/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
[openbao-tokens]: https://openbao.org/docs/concepts/tokens/
