# Log retention and access control

Use this explainer to design retention and access boundaries for OpenBao logs.
It is for operators who need different handling for operational logs,
completed request logs, audit logs, and audit archives.

## Why this matters

OpenBao log streams have different risk and retention requirements. A single
retention period or access policy is usually wrong.

Operational logs support troubleshooting. Completed request logs are
temporary. Audit logs are restricted security records. Audit archives are
durable evidence paths governed by organizational policy.

## Retention model

Use retention based on stream purpose.

| Stream | Reference retention | Reason |
| ------ | ------------------- | ------ |
| `openbao.operational` | 14-30 days. | Enough for incident diagnosis and change correlation. |
| `openbao.completed_requests` | 24-72 hours. | Short troubleshooting windows with break-glass access. |
| `openbao.audit` | 7-30 days in Loki. | Short-term security exploration and dashboard drilldown. |
| `openbao.audit_archive` | Organization policy. | Long-term evidence, compliance, and legal requirements. |

The exact values are deployment decisions. Keep them documented beside the
collector, Loki tenant, archive destination, and Grafana access model.

## Access model

Use access based on data sensitivity.

| Stream | Access boundary |
| ------ | --------------- |
| `openbao.operational` | SRE platform users who operate OpenBao. |
| `openbao.completed_requests` | Break-glass users during approved troubleshooting windows. |
| `openbao.audit` | Security-restricted users and approved incident responders. |
| `openbao.audit_archive` | Security and compliance users under evidence-handling policy. |

Do not use one Grafana folder, one Loki tenant, or one role for every OpenBao
log stream unless your organization explicitly accepts the exposure.

## Loki retention

Loki retention is an operational setting, not a compliance guarantee by
itself. Loki documents retention through the Compactor, and retention can be
configured per tenant or per stream depending on the deployment.

If you rely on object storage lifecycle policies, keep them aligned with Loki
retention. The object store must not remove data earlier than the Loki
retention design expects.

## Audit archive retention

The archive path is the durable evidence path. It can be a SIEM, immutable
object store, write-once storage, or another approved backend.

Archive retention depends on security, compliance, legal, and business
requirements. Record these decisions outside the dashboard repository when
they include organization-specific policy, owner names, or exception details.

## Completed request log retention

Completed request logs are disabled by default in OpenBao. When enabled, they
can include request metadata that does not belong in broad operational
dashboards.

Keep retention short. Require a reason, owner, start time, end time, and
deactivation step for each troubleshooting window.

## Dashboard access

Align dashboard access with the most sensitive data on the page.

Examples:

- Operational logs dashboard: SRE platform access.
- Audit overview dashboard: security-restricted access.
- Audit investigation dashboard: restricted incident-response access.
- Auth and identity dashboard: restricted security and platform access.
- Database secrets dashboard: restricted security and platform access.
- Secret engines and mounts dashboard: restricted security and platform
  access.

If a dashboard mixes operational and audit data, use the audit access boundary.

## Common mistakes

- Keeping audit logs only in a short-lived Loki tenant.
- Giving operational users access to audit dashboards by default.
- Enabling completed request logs without an end time.
- Treating Loki retention as an archive policy.
- Letting object storage lifecycle delete Loki data earlier than expected.
- Putting request paths, token metadata, or identity metadata into labels and
  then trying to fix exposure with dashboard permissions.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed upstream behavior | Loki documents retention through the Compactor. OpenBao documents completed request logging as disabled by default and audit logs as security-sensitive records. |
| Design decision | This project separates operational logs, completed request logs, audit logs, and audit archives into different streams and access boundaries. |
| To validate | Your Loki tenant model, archive backend, legal retention, object-store lifecycle policy, and Grafana folder permissions. |

## What's next

- Use [Understanding OpenBao logs](./understanding-openbao-logs.md) to choose
  the right log stream.
- Use [Loki label strategy for OpenBao](./loki-label-strategy.md) before you
  add labels or structured metadata.
- Use [Audit logs as security records](../concepts/audit-logs-as-security-records.md)
  before you broaden audit-log access.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) to
  configure repeatable audit devices and collection paths.

Source: Loki documents retention in the
[Grafana Loki retention documentation][loki-retention]. OpenBao documents
completed request logging in the
[OpenBao completed request logging documentation][openbao-log-requests].
OpenBao documents audit-device behavior in the
[OpenBao audit device documentation][openbao-audit]. This page also reflects
the repository stream contract in `contracts/streams/log-streams.yaml`.

[loki-retention]: https://grafana.com/docs/loki/latest/operations/storage/retention/
[openbao-audit]: https://openbao.org/docs/audit/
[openbao-log-requests]: https://openbao.org/docs/configuration/log-requests-level/
