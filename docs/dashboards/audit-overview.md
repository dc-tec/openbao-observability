# OpenBao audit overview dashboard

Use this explainer to read the generated OpenBao audit overview dashboard. It
is for operators who need to separate audit-device health, audit event volume,
and high-risk audited activity before opening a deeper investigation view.

## What this dashboard is for

Use the audit overview dashboard when the OpenBao overview dashboard shows
audit failures, a missing audit stream, unexpected audit volume, or risky
system-path activity.

The dashboard answers these questions:

- Are audit request or response logging failures increasing?
- How many audited request and response entries does OpenBao report?
- How much audit latency does OpenBao report?
- Does Loki receive audit entries by node?
- Are request and response entries broadly balanced?
- Are audited mutations touching high-risk system paths?
- Is the durable audit archive expected, healthy, and current?
- Did security detection summary panels change recently?

## What this dashboard is not for

Do not use this dashboard as a full audit investigation tool. It summarizes
audit health and volume, but it does not give a focused workflow for a specific
request ID, path, operation, or node.

Use the audit investigation dashboard when you need filtered event streams or
request ID drilldown.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | Expected UID | Used for |
| ----------- | ------------ | -------- |
| Prometheus | `prometheus` | Audit telemetry metrics and normalized recording rules. |
| Loki | `loki` | Audit event volume, request/response balance, and event logs. |

Prometheus panels depend on generated `openbao:` recording rules. Loki panels
depend on audit logs collected with `log_stream="openbao.audit"`.

The generated dashboard exposes a Kubernetes namespace selector for
deployments where multiple OpenBao instances share the same observability
backend.

## How to read audit health

Start with request and response failures. Healthy values remain `0`.

Audit request failures mean OpenBao failed while logging request entries.
Audit response failures mean OpenBao failed while logging response entries.
Treat either condition as a security and availability issue because OpenBao
audit-device failures can affect whether requests complete.

Use the audit request rate, response rate, and audit latency panels as context.
An increase in latency without failures can indicate that an audit sink is
slow. A failure increase needs runbook response, not dashboard tuning.

## How to read archive health

Use the archive-health row when your environment has a durable audit archive,
SIEM path, or object-store evidence path outside Loki.

| Panel | What it means |
| ----- | ------------- |
| Archive expected | The archive health exporter reports that this environment expects durable audit archive delivery. |
| Archive delivery | The archive health exporter reports whether the archive path is currently healthy. |
| Archive success age | The age of the most recent successful archive delivery acknowledgement. |
| Archive failures | Failed archive writes, rejected batches, or failed acknowledgements over 15 minutes. |
| Archive dead letters | Records sent to a dead-letter path instead of the durable archive over 15 minutes. |

Read these panels together. `Archive expected` is the guardrail. If it is `0`,
the environment has not declared archive delivery as required through the
reference exporter metrics. If it is `1`, then delivery health, success age,
failures, and dead letters describe the durable evidence path.

These panels do not come from OpenBao itself. They come from the archive
pipeline or from the reference archive health exporter. Use
[Audit archive reference design](../audit/audit-archive-reference-design.md)
before you rely on these metrics for production evidence handling.

## How to read audit volume

The audit event volume panel counts audit log entries received by Loki over
five minutes, grouped by node. Use it to check whether the collector sees
expected audit activity from the nodes that your deployment expects to produce
audit events.

Volume alone does not prove that the audit path is healthy. A quiet cluster can
produce no audited events, and some system paths bypass audit. Use the audit
canary alert to prove that a known audited request reaches the audit stream.

## How to read request and response balance

The request/response balance panel subtracts audited response entries from
audited request entries over a ten-minute window.

Short-lived imbalance can happen while requests are in flight or when the time
window cuts through active traffic. Persistent imbalance deserves
investigation because it can point to collection gaps, parsing issues, request
failures, or audit sink behavior.

Use this panel as a directional signal. Do not treat a nonzero value by itself
as proof of lost audit records.

## How to read system mutations

The system mutations panel filters audited create, update, and delete
operations against high-risk `sys/` paths, including auth, audit, mounts,
policies, raw, plugins, Raft storage, and rotation paths.

High-risk does not mean malicious. It means the operation can change the
security, storage, or plugin behavior of the OpenBao deployment. Confirm that
each event matches an approved change or a known automation.

## How to read security detection summaries

Use the security detection summary panels as a triage layer before you open the
audit investigation dashboard.

| Panel | What it means |
| ----- | ------------- |
| Audit config changes | Audit device configuration changed through an audited `sys/audit` path. |
| Privileged config changes | Policy, auth method, or mount configuration changed. |
| Permission denied | Audited responses included permission denied errors. |
| Completed request logs | Completed request logging produced entries and needs an approved troubleshooting window. |

These panels are not proof of malicious activity. They show events that should
have an owner, change record, or incident context.

## Common mistakes

- Treating a quiet audit volume panel as proof that the audit pipeline works.
- Ignoring audit request or response failures because API traffic still works.
- Using health, leader, seal, unseal, or metrics paths as audit-pipeline tests.
- Giving broad operational users access to audit dashboards.
- Treating system mutation activity as malicious without checking the approved
  change window.
- Ignoring completed request log entries after a troubleshooting window ends.
- Treating absent archive-health metrics as proof that audit archiving is not
  required.

## Known limitations

- The dashboard assumes `log_stream="openbao.audit"`.
- It summarizes audit events but does not replace restricted audit storage.
- It does not prove compliance retention.
- Archive-health panels depend on `openbao_audit_archive_*` metrics from an
  archive pipeline or the reference archive health exporter.
- It does not show every OpenBao system path because some paths bypass audit.
- Request/response balance is directional and depends on the selected time
  window.

## What's next

- Use [OpenBao audit investigation dashboard](./audit-investigation.md) for
  request ID, path, operation, or node drilldown.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) to
  review the audit collection pattern.
- Use [Audit logs as security records](../concepts/audit-logs-as-security-records.md)
  to understand audit-log access, retention, and canary design.
- Use [Metrics, logs, and audit logs](../concepts/metrics-vs-logs-vs-audit-logs.md)
  to separate audit records from operational logs.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you change audit labels.
- Use [Audit request and response failures](../runbooks/audit-request-response-failures.md)
  when audit failure metrics increase.
- Use [Audit canary missing](../runbooks/audit-canary-missing.md) when the
  canary-backed audit alert fires.
- Use [Audit archive degraded](../runbooks/audit-archive-degraded.md) when the
  durable archive path is expected but stale, failing, or dead-lettering.
- Use [Security audit detections](../runbooks/security-audit-detections.md)
  when a security detection alert fires.

Source: OpenBao documents audit-device behavior, unaudited paths, and HMAC
limits in the [OpenBao audit device documentation][openbao-audit]. OpenBao
documents audit telemetry metrics in the
[OpenBao audit telemetry documentation][openbao-audit-telemetry]. This page
describes the generated dashboard contract in
`contracts/dashboards/openbao-audit-overview.yaml`.

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-audit-telemetry]: https://openbao.org/docs/internals/telemetry/metrics/audit/
