# Understanding OpenBao logs

Use this explainer to understand the OpenBao log streams used by this
reference architecture. It is for operators who need to distinguish
operational logs, completed request logs, audit logs, and audit archives.

## Why this matters

OpenBao logs are not one stream with one audience. Operational logs help you
debug server behavior. Completed request logs help with temporary request
troubleshooting. Audit logs are restricted security records. Audit archives are
durable evidence paths.

Mixing these streams creates two problems: operators miss the signal they need,
and sensitive audit metadata reaches audiences that do not need it.

## Stream model

This project defines four OpenBao log streams.

| Stream | Default | Source | Access |
| ------ | ------- | ------ | ------ |
| `openbao.operational` | Enabled | OpenBao server logs. | SRE platform. |
| `openbao.completed_requests` | Disabled | OpenBao completed request logging. | Break-glass. |
| `openbao.audit` | Enabled | OpenBao audit devices. | Security restricted. |
| `openbao.audit_archive` | Production enabled | Security archive pipeline. | Security and compliance. |

Keep these streams separate at collection, storage, dashboard, and access
layers.

## Operational logs

Operational logs describe OpenBao server behavior. Use them for startup,
shutdown, listener, TLS, storage, Raft, seal, unseal, plugin, and process-level
troubleshooting.

OpenBao supports log levels `trace`, `debug`, `info`, `warn`, and `error`.
OpenBao also supports `standard` and `json` log formats. The reference stack
uses JSON-friendly processing because structured fields make Loki filtering
and dashboard panels more reliable.

Operational logs are not audit evidence.

## Completed request logs

Completed request logging is a separate OpenBao feature controlled by
`log_requests_level`. It is disabled by default. When enabled, OpenBao emits
completed request logs at the configured level only when the main `log_level`
includes that level.

Use completed request logs for short troubleshooting windows. Do not leave
them enabled by default, and do not use them as an audit-log replacement.

## Audit logs

Audit logs are request and response records from OpenBao audit devices. They
are security records and need restricted access.

Audit logs are useful for request ID correlation, auth activity, token
lifecycle investigation, secret-engine activity, and system mutation review.
They can expose sensitive metadata even when values are HMACed.

Use [Audit logs as security records](../concepts/audit-logs-as-security-records.md)
before you expand audit-log collection, access, retention, or dashboard scope.

## Audit archive

The audit archive stream represents the durable evidence path. It can be a
SIEM, object store, immutable archive, or another approved backend.

Do not treat Loki as the archive unless your organization has explicitly
approved Loki for that purpose. Loki is useful for short-term exploration and
correlation, but compliance retention needs separate design.

## Collection pattern

Use collection labels to describe source and routing, not request content.

Recommended stable labels include:

- `cluster`
- `environment`
- `region`
- `namespace`
- `app`
- `component`
- `log_stream`
- `node_id`
- `deployment_profile`
- `pod`
- `container`
- `instance`

Forbidden labels include request IDs, request paths, secret paths, mount paths,
token accessors, entity IDs, auth accessors, client addresses, policy names,
and user names.

## Common mistakes

- Sending audit logs to a broad operational log tenant.
- Treating completed request logs as audit logs.
- Leaving debug, trace, or completed request logging enabled after
  troubleshooting.
- Promoting request IDs or paths to Loki labels.
- Treating a quiet operational stream as proof that OpenBao is healthy.
- Treating Loki as an audit archive without retention and access approval.

## What's next

- Use [Loki label strategy for OpenBao](./loki-label-strategy.md) before you
  change log labels.
- Use [Log retention and access control](./retention-and-access-control.md) to
  separate operational, audit, and archive retention.
- Use [OpenBao operational logs dashboard](../dashboards/operational-logs.md)
  to read the generated operational log view.
- Use [OpenBao audit investigation dashboard](../dashboards/audit-investigation.md)
  for restricted audit drilldown.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) for
  repeatable audit-device setup.

Source: OpenBao documents server log settings in the
[OpenBao configuration documentation][openbao-configuration]. OpenBao
documents completed request logging in the
[OpenBao completed request logging documentation][openbao-log-requests].
OpenBao documents audit devices in the
[OpenBao audit device documentation][openbao-audit]. This page also reflects
the repository stream contract in `contracts/streams/log-streams.yaml`.

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-configuration]: https://openbao.org/docs/configuration/
[openbao-log-requests]: https://openbao.org/docs/configuration/log-requests-level/
