# Loki label strategy for OpenBao

Use this explainer to choose safe Loki labels for OpenBao logs. It is for
operators who need useful log routing and dashboard filters without indexing
sensitive or high-cardinality audit fields.

## Why this matters

Loki indexes labels, not full log line content. Good labels make log streams
easy to route and query. Poor labels create too many streams, make queries
expensive, and expose metadata that belongs in restricted investigation views.

OpenBao audit logs are especially sensitive because paths, request IDs,
identity fields, token metadata, and client metadata can reveal how the
secrets platform is used.

## Label model

Use labels for stable source identity and routing. Use query-time parsing or
structured metadata for investigation fields.

```text
Stable labels
  -> choose stream and source
Log body or structured metadata
  -> inspect request-specific fields
Query-time parsing
  -> filter sensitive fields in restricted dashboards
```

## Allowed labels

The stream contract allows these labels:

| Label | Use |
| ----- | --- |
| `cluster` | OpenBao or platform cluster identity. |
| `environment` | Environment such as production or staging. |
| `region` | Region or location. |
| `namespace` | Platform namespace when bounded and approved. |
| `app` | Application or workload source. |
| `component` | Component such as `openbao`. |
| `log_stream` | Project stream such as `openbao.audit`. |
| `node_id` | OpenBao node identifier when bounded. |
| `deployment_profile` | Demo, Kubernetes, VM, or production profile. |
| `pod` | Pod identity when your Loki design allows it. |
| `container` | Container identity. |
| `instance` | Scrape or collector instance identity. |

Keep the label set small. Do not add a label because one dashboard needs a
temporary filter.

## Forbidden labels

The stream contract forbids these labels:

| Label | Risk |
| ----- | ---- |
| `request_id` | Unbounded and investigation-specific. |
| `request_path` | Reveals API, secret, auth, and mount usage. |
| `secret_path` | Reveals secret naming and business context. |
| `mount_path` | Reveals mount layout and can grow over time. |
| `namespace_path` | Reveals tenancy structure. |
| `client_token` | Security-sensitive token material. |
| `token_accessor` | Security-sensitive token metadata. |
| `entity_id` | Security-sensitive identity metadata. |
| `auth_accessor` | Security-sensitive auth mount metadata. |
| `client_ip` | High-cardinality and privacy-sensitive value. |
| `remote_address` | High-cardinality and privacy-sensitive value. |
| `policy` | Reveals authorization model details. |
| `user_name` | Identity metadata. |
| `display_name` | Identity metadata. |

Parse these fields at query time in restricted dashboards instead.

## Structured metadata

Loki supports structured metadata for data that you need alongside log entries
without indexing it as labels. Use it only when your Loki version, schema, and
tenant settings support it.

Structured metadata can help with metadata that is too high-cardinality for
labels and too expensive to parse repeatedly. It does not remove the need for
access control. Sensitive OpenBao fields still need restricted tenants,
folders, and dashboards.

## Query-time parsing

The generated audit dashboards parse audit JSON fields at query time.

Example pattern:

```logql
{log_stream="openbao.audit"}
  | json request_id="request.id", request_path="request.path"
  | request_id=~"${request_id:raw}"
  | request_path=~"${request_path:raw}"
```

This pattern keeps labels stable while still enabling request ID and path
drilldown for approved users.

## Label review checklist

Before you add a Loki label, confirm all of these conditions:

- The value set is bounded for the full retention window.
- The value does not reveal sensitive OpenBao usage patterns.
- Multiple dashboards or alerts need the label.
- Routing or access control benefits from indexing the value.
- The value does not grow with requests, tokens, entities, clients, or paths.
- The label is allowed by the stream contract.

If any condition fails, keep the value out of labels.

## Common mistakes

- Labeling every parsed JSON field.
- Labeling `request.path` to make one dashboard faster.
- Using token accessors or entity IDs in alert labels.
- Treating demo cardinality as production cardinality.
- Forgetting that labels can leak metadata even when audit values are HMACed.
- Mixing audit and operational logs in the same `log_stream`.

## What's next

- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  for the broader Prometheus and Loki label model.
- Use [Understanding OpenBao logs](./understanding-openbao-logs.md) to choose
  the right log stream.
- Use [Log retention and access control](./retention-and-access-control.md)
  before you expose audit logs to dashboards.
- Use [OpenBao audit investigation dashboard](../dashboards/audit-investigation.md)
  to see query-time parsing in generated panels.

Source: Loki documents labels and low-cardinality guidance in the
[Grafana Loki label documentation][loki-labels]. Loki documents structured
metadata in the [Grafana Loki structured metadata documentation][loki-structured-metadata].
This page also reflects the repository stream contract in
`contracts/streams/log-streams.yaml`.

[loki-labels]: https://grafana.com/docs/loki/latest/get-started/labels/
[loki-structured-metadata]: https://grafana.com/docs/loki/latest/get-started/labels/structured-metadata/
