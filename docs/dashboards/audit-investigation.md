# OpenBao audit investigation dashboard

Use this explainer to read the generated OpenBao audit investigation dashboard.
It is for operators who need restricted drilldown by request ID, request path,
operation, or node without turning sensitive audit fields into Loki labels.

## What this dashboard is for

Use the audit investigation dashboard when you already know an audit question
that needs filtering.

The dashboard answers these questions:

- Which audit entries match a request ID?
- Which audit entries match a request path pattern?
- Which audited create, update, or delete operations touched high-risk
  `sys/` paths?
- Which security detection category needs follow-up?
- Which audited auth method, login, user, or token events match the filters?
- Which response entries contain an error field?
- Which OpenBao node produced the matching events?

## What this dashboard is not for

Do not use this dashboard as a general-purpose search backend or compliance
archive. It is an investigation view over the Loki audit stream.

Do not use it to prove that all required audit records exist. Use audit-device
configuration, archive validation, collector health, and the audit canary for
that assurance.

## Required data source

The generated dashboard expects a Loki data source with UID `loki`. The panels
query audit logs collected with `log_stream="openbao.audit"` and a bounded
`node_id` label.

The dashboard does not require Prometheus for its current panels, even though
the contract includes the standard metrics data-source definition for
consistency with the dashboard generator.

## Investigation filters

The dashboard exposes these variables:

| Variable | Type | Default | Use |
| -------- | ---- | ------- | --- |
| Kubernetes namespace | Textbox | `.*` | Selects the OpenBao workload namespace when multiple OpenBao instances share the same observability backend. |
| Request ID | Textbox | `.*` | Narrow the stream to one request ID or a request ID pattern. |
| Request path | Textbox | `.*` | Narrow the stream to one path or a path pattern. |
| Operation | Custom | `.*` | Filter to `read`, `list`, `create`, `update`, or `delete`. |
| Node | Textbox | `.*` | Narrow the stream to one OpenBao node label. |

Treat textbox values as LogQL regular expressions. Escape special characters
when you need an exact match.

## How request ID drilldown works

The request ID drilldown panel parses `request.id`, `request.path`, and
`request.operation` from the audit JSON line at query time. It then applies the
selected variables.

This design keeps request IDs out of Loki labels. Request IDs are unbounded and
highly specific, so they are useful for investigation but poor indexing keys.

Use the request ID filter when you need to correlate request and response
entries. Clear the filter before moving back to broad event review.

## How risky system events work

The risky system panels filter audited create, update, and delete operations
against these `sys/` path families:

- `sys/auth`
- `sys/audit`
- `sys/mounts`
- `sys/policies`
- `sys/raw`
- `sys/plugins`
- `sys/storage/raft`
- `sys/rotate`

These paths can change authentication, audit behavior, mount configuration,
policy behavior, raw storage access, plugin behavior, Raft state, or rotation
state. Confirm that matching events align with an approved change or automation.

## How security detection panels work

The security detection panels summarize activity that usually needs a change
record or incident review:

| Panel | Investigation focus |
| ----- | ------------------- |
| Audit config changes | Audit device enable, disable, tune, or delete activity. |
| Privileged config changes | Policy, auth method, and mount configuration changes. |
| High-risk sys paths | Raw storage, plugin, Raft storage, and rotation endpoints. |
| Permission denied | Repeated authorization failures visible in audited responses. |

Use these panels to choose the next log table and request ID drilldown. Keep
request paths and request IDs as query-time fields.

## How auth activity works

The auth activity panels filter audit paths that match `auth/.*` or
`sys/auth/.*`. Use them to inspect audited login, auth method, user, and token
activity within the selected time range.

Auth activity is not automatically suspicious. Compare it with the expected
auth methods, automation windows, and user activity for the environment.

## How response errors work

The response error panels parse the audit entry type and error field. They
show response entries where `type="response"` and the parsed `error` field is
not empty.

Use these panels to find failed audited API responses. Then inspect nearby
request and response entries to understand whether the error came from
authorization, input validation, backend behavior, storage, or another cause.

## Label safety

The dashboard uses query-time parsing for `request.id`, `request.path`, and
`request.operation`. It does not require those values as Loki labels.

Keep this pattern when you extend the dashboard. Request IDs, paths, entity
IDs, token accessors, auth accessors, and client addresses belong in
restricted query-time filters, not ingestion labels.

## Common mistakes

- Entering an unescaped regular expression when you wanted an exact request ID.
- Leaving a request ID filter active and assuming the audit stream is empty.
- Treating risky system activity as malicious before checking approved changes.
- Treating permission denied bursts as harmless without checking the targeted
  paths.
- Promoting investigation fields to Loki labels for convenience.
- Giving the dashboard to users who do not have approval to inspect audit
  metadata.

## Known limitations

- The dashboard depends on Loki retention for `log_stream="openbao.audit"`.
- It cannot query audit records that were never collected by Loki.
- It does not replace a compliance archive or SIEM.
- It cannot show OpenBao paths that bypass the audit system.
- It interprets fields based on the current audit JSON shape.

## What's next

- Use [OpenBao audit overview dashboard](./audit-overview.md) when you need
  audit-health and volume context before drilling down.
- Use [Audit logs as security records](../concepts/audit-logs-as-security-records.md)
  to understand why audit fields need restricted access.
- Use [OpenBao auth and identity dashboard](./auth-identity.md) for focused
  auth method, token, and identity activity.
- Use [OpenBao database secrets dashboard](./database-secrets.md) for focused
  database credential and database lease activity.
- Use [OpenBao secret engines and mounts dashboard](./secret-engines-mounts.md)
  for focused secret engine and mount activity.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add new filters or labels.
- Use [Metrics, logs, and audit logs](../concepts/metrics-vs-logs-vs-audit-logs.md)
  to decide whether the question belongs in audit logs.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) to
  review audit collection and retention boundaries.
- Use [Security audit detections](../runbooks/security-audit-detections.md)
  when a security detection alert fires.

Source: OpenBao documents audit-device behavior, unaudited paths, and HMAC
limits in the [OpenBao audit device documentation][openbao-audit]. Loki
documents low-cardinality label guidance in the
[Grafana Loki label documentation][loki-labels]. This page describes the
generated dashboard contract in
`contracts/dashboards/openbao-audit-investigation.yaml`.

[loki-labels]: https://grafana.com/docs/loki/latest/get-started/labels/
[openbao-audit]: https://openbao.org/docs/audit/
