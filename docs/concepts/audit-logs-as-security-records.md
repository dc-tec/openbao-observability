# Audit logs as security records

Use this explainer to understand how OpenBao audit logs fit into a security
observability design. It is for operators who need to protect audit records,
interpret audit metrics, and avoid confusing audit logs with operational logs.

## Why this matters

Audit logs are security records. They describe audited OpenBao API request and
response activity, including errors. They can help you investigate access,
configuration changes, auth activity, token activity, and secret-engine use.

That value also makes audit logs sensitive. Even when OpenBao HMACs sensitive
strings, audit records can reveal request paths, identities, methods, timing,
and access patterns.

## Mental model

Treat audit logging as a security pipeline.

```text
OpenBao API request
  -> audit device request entry
  -> OpenBao request handling
  -> audit device response entry
  -> restricted collection
  -> short-term exploration and long-term archive
```

Metrics tell you whether audit logging succeeds. Audit logs tell you what
audited activity occurred. Both signals matter.

## What OpenBao audit logs represent

OpenBao audit devices log request and response entries for audited API paths.
Request and response entries share a request identifier that lets you correlate
the two sides of an audited interaction.

Audit logs are useful for:

- Confirming audited API activity.
- Investigating request paths and operations.
- Correlating request and response entries.
- Reviewing auth, identity, token, policy, mount, and secret-engine changes.
- Checking whether an expected audit canary reached the log stream.

## What OpenBao does not audit

OpenBao documents several paths that bypass the audit system:

- `sys/init`
- `sys/seal-status`
- `sys/seal`
- `sys/unseal`
- `sys/leader`
- `sys/health`
- `sys/storage/raft/bootstrap`
- `sys/storage/raft/join`

When listener configuration allows unauthenticated access, these paths also
bypass audit:

- `sys/metrics`
- `sys/pprof/*`
- `sys/in-flight-req`

Do not use these paths as audit canaries. Use a known audited path with a
non-sensitive request instead.

## HMAC behavior and limits

OpenBao HMACs sensitive strings by default. This protects many values while
still allowing operators with the audit device salt to compare expected values
through audit hash workflows.

HMAC protection does not make audit logs safe for broad access. Audit records
still contain metadata, paths, operations, timestamps, response status, and
some non-string values. OpenBao documents that non-string JSON values such as
integers and booleans pass through in plaintext.

Do not enable raw audit logging for the reference profile. Raw audit logging
can expose values that OpenBao normally protects.

## Audit metrics

Audit metrics are the health layer for audit logging.

Use audit metrics to detect:

- Request audit logging failures.
- Response audit logging failures.
- Audit logging latency.
- Audit logging rate.

Treat request and response failure metrics as critical. OpenBao can block or
fail requests when audit devices cannot write. Audit failure metrics therefore
represent both security-record risk and availability risk.

## Multiple audit devices

Multiple audit devices provide redundancy and independent copies. OpenBao
attempts to write audit records to all enabled devices. A complete security
review needs the aggregate of all audit paths that your deployment treats as
authoritative.

Design the paths deliberately:

- Keep a local or node-adjacent device for resilience.
- Send one stream to a restricted exploration backend when needed.
- Send the durable evidence stream to an approved archive or SIEM.
- Test sink-failure behavior before production rollout.

## Loki and retention

Loki is useful for short-term exploration, dashboard correlation, and
investigation workflows. It is not automatically a compliance archive.

Use Loki when you need:

- Fast query-time filtering.
- Dashboard drilldowns.
- Request ID correlation.
- Short-term incident investigation.

Use an approved archive or SIEM when you need:

- Long-term retention.
- Evidence preservation.
- Legal hold.
- Tamper-resistance.
- Separation from operational users.

## Missing audit logs

A quiet audit stream does not prove failure. Quiet clusters can produce no
audited events, and some system paths are not audited.

The reference architecture uses a canary pattern: perform a safe request
against a known audited path, then alert when that canary event is missing.
This reduces false positives while still detecting collector, Loki, or audit
pipeline failures.

## Access control

Restrict audit logs separately from operational logs. Audit metadata can reveal
security-sensitive usage patterns even when values are HMACed.

Keep these fields out of Loki labels:

- Request paths.
- Request IDs.
- Entity IDs.
- Token accessors.
- Auth accessors.
- Client addresses.
- User names.
- Policy names.

Parse them at query time in restricted dashboards instead.

## Common mistakes

- Treating audit logs as ordinary application logs.
- Giving broad operational viewers access to audit dashboards.
- Using health, leader, seal, unseal, or metrics paths as audit canaries.
- Treating Loki as a compliance archive without approval.
- Ignoring audit failure metrics because OpenBao still serves some traffic.
- Enabling `log_raw` without a security exception and compensating controls.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed OpenBao docs behavior | OpenBao documents audited request and response entries, unaudited paths, HMAC behavior, multiple audit devices, and audit failure behavior. |
| Observed fixture behavior | The OpenBao 2.5.4 fixture emits audit JSON lines and audit telemetry metrics, and the demo stack separates audit logs from operational logs. |
| Design decision | This project treats audit logs as restricted security records and uses canary-backed missing-audit alerts. |
| To validate | Archive retention, SIEM forwarding, access controls, tenant boundaries, and failure behavior in your deployment. |

## What's next

- Use [Configure declarative audit devices](../audit/declarative-audit.md) to
  configure repeatable audit devices.
- Use [OpenBao audit overview dashboard](../dashboards/audit-overview.md) to
  inspect audit health and volume.
- Use [OpenBao audit investigation dashboard](../dashboards/audit-investigation.md)
  for request ID, path, operation, and node drilldown.
- Use [High-cardinality and label safety](./high-cardinality-and-label-safety.md)
  before you change audit labels.
- Use [Audit request and response failures](../runbooks/audit-request-response-failures.md)
  when audit failure metrics increase.
- Use [Audit canary missing](../runbooks/audit-canary-missing.md) when the
  canary-backed missing-audit alert fires.

Source: OpenBao documents audit-device behavior, unaudited paths, HMAC
behavior, multiple audit devices, and blocking behavior in the
[OpenBao audit device documentation][openbao-audit]. OpenBao documents
configuration-managed audit devices in the
[OpenBao declarative audit documentation][openbao-declarative-audit]. OpenBao
documents audit telemetry in the
[OpenBao audit telemetry documentation][openbao-audit-telemetry].

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-audit-telemetry]: https://openbao.org/docs/internals/telemetry/metrics/audit/
[openbao-declarative-audit]: https://openbao.org/docs/configuration/audit/
